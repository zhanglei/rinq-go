package localsession

import (
	"context"
	"fmt"
	"sync"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/rinq/rinq-go/src/rinq"
	"github.com/rinq/rinq-go/src/rinq/constraint"
	"github.com/rinq/rinq-go/src/rinq/ident"
	"github.com/rinq/rinq-go/src/rinq/internal/command"
	"github.com/rinq/rinq-go/src/rinq/internal/notify"
	"github.com/rinq/rinq-go/src/rinq/internal/nsutil"
	"github.com/rinq/rinq-go/src/rinq/internal/traceutil"
	"github.com/rinq/rinq-go/src/rinq/trace"
)

type session struct {
	id       ident.SessionID
	catalog  Catalog
	invoker  command.Invoker
	notifier notify.Notifier
	listener notify.Listener
	logger   rinq.Logger
	tracer   opentracing.Tracer
	done     chan struct{}

	// mutex guards Call(), Listen(), Unlisten() and Close() so that Close()
	// waits for pending calls to complete or timeout, and to ensure that it's
	// call to listener.Unlisten() is not "undone" by the user.
	mutex sync.RWMutex
}

// NewSession returns a new local session.
func NewSession(
	id ident.SessionID,
	catalog Catalog,
	invoker command.Invoker,
	notifier notify.Notifier,
	listener notify.Listener,
	logger rinq.Logger,
	tracer opentracing.Tracer,
) rinq.Session {
	sess := &session{
		id:       id,
		catalog:  catalog,
		invoker:  invoker,
		notifier: notifier,
		logger:   logger,
		tracer:   tracer,
		listener: listener,
		done:     make(chan struct{}),
	}

	logCreated(logger, catalog.Ref())

	go func() {
		<-catalog.Done()
		sess.destroy()
	}()

	return sess
}

func (s *session) ID() ident.SessionID {
	return s.id
}

func (s *session) CurrentRevision() (rinq.Revision, error) {
	select {
	case <-s.done:
		return nil, rinq.NotFoundError{ID: s.id}
	default:
		return s.catalog.Head(), nil
	}
}

func (s *session) Call(ctx context.Context, ns, cmd string, out *rinq.Payload) (*rinq.Payload, error) {
	if err := nsutil.Validate(ns); err != nil {
		return nil, err
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	select {
	case <-s.done:
		return nil, rinq.NotFoundError{ID: s.id}
	default:
	}

	msgID, attrs := s.catalog.NextMessageID()

	span, ctx := traceutil.ChildOf(ctx, s.tracer, ext.SpanKindRPCClient)
	defer span.Finish()

	traceutil.SetupCommand(span, msgID, ns, cmd)
	traceutil.LogInvokerCall(span, attrs, out)

	start := time.Now()
	traceID, in, err := s.invoker.CallBalanced(ctx, msgID, ns, cmd, out)
	elapsed := time.Since(start) / time.Millisecond

	if err == nil {
		traceutil.LogInvokerSuccess(span, in)
	} else {
		traceutil.LogInvokerError(span, err)
	}

	logCall(s.logger, msgID, ns, cmd, elapsed, out, in, err, traceID)

	return in, err
}

func (s *session) CallAsync(ctx context.Context, ns, cmd string, out *rinq.Payload) (ident.MessageID, error) {
	var msgID ident.MessageID

	if err := nsutil.Validate(ns); err != nil {
		return msgID, err
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	select {
	case <-s.done:
		return msgID, rinq.NotFoundError{ID: s.id}
	default:
	}

	msgID, attrs := s.catalog.NextMessageID()

	span, ctx := traceutil.ChildOf(ctx, s.tracer, ext.SpanKindRPCClient)
	defer span.Finish()

	traceutil.SetupCommand(span, msgID, ns, cmd)
	traceutil.LogInvokerCallAsync(span, attrs, out)

	traceID, err := s.invoker.CallBalancedAsync(ctx, msgID, ns, cmd, out)

	if err != nil {
		traceutil.LogInvokerError(span, err)
	}

	logAsyncRequest(s.logger, msgID, ns, cmd, out, err, traceID)

	return msgID, err
}

// SetAsyncHandler sets the asynchronous call handler.
//
// h is invoked for each command response received to a command request made
// with CallAsync().
func (s *session) SetAsyncHandler(h rinq.AsyncHandler) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	select {
	case <-s.done:
		return rinq.NotFoundError{ID: s.id}
	default:
	}

	s.invoker.SetAsyncHandler(
		s.id,
		func(
			ctx context.Context,
			sess rinq.Session,
			msgID ident.MessageID,
			ns string,
			cmd string,
			in *rinq.Payload,
			err error,
		) {
			span := opentracing.SpanFromContext(ctx)
			traceutil.SetupCommand(span, msgID, ns, cmd)

			if err == nil {
				traceutil.LogInvokerSuccess(span, in)
			} else {
				traceutil.LogInvokerError(span, err)
			}

			logAsyncResponse(ctx, s.logger, msgID, ns, cmd, in, err)

			h(ctx, sess, msgID, ns, cmd, in, err)
		},
	)

	return nil
}

func (s *session) Execute(ctx context.Context, ns, cmd string, p *rinq.Payload) error {
	if err := nsutil.Validate(ns); err != nil {
		return err
	}

	select {
	case <-s.done:
		return rinq.NotFoundError{ID: s.id}
	default:
	}

	msgID, attrs := s.catalog.NextMessageID()

	span, ctx := traceutil.ChildOf(ctx, s.tracer, ext.SpanKindRPCClient)
	defer span.Finish()

	traceutil.SetupCommand(span, msgID, ns, cmd)
	traceutil.LogInvokerCallAsync(span, attrs, p)

	traceID, err := s.invoker.ExecuteBalanced(ctx, msgID, ns, cmd, p)

	if err != nil {
		traceutil.LogInvokerError(span, err)
	}

	// TODO: move to function
	if err == nil {
		s.logger.Log(
			"%s executed '%s::%s' command (%d/o) [%s]",
			msgID.ShortString(),
			ns,
			cmd,
			p.Len(),
			traceID,
		)
	}

	return err
}

func (s *session) Notify(ctx context.Context, ns, t string, target ident.SessionID, p *rinq.Payload) error {
	if err := target.Validate(); err != nil || target.Seq == 0 {
		return fmt.Errorf("session ID %s is invalid", target)
	}

	if err := nsutil.Validate(ns); err != nil {
		return err
	}

	select {
	case <-s.done:
		return rinq.NotFoundError{ID: s.id}
	default:
	}

	msgID, attrs := s.catalog.NextMessageID()

	span, ctx := traceutil.ChildOf(ctx, s.tracer, ext.SpanKindProducer)
	defer span.Finish()

	traceutil.SetupNotification(span, msgID, ns, t)
	traceutil.LogNotifierUnicast(span, attrs, target, p)

	traceID, err := s.notifier.NotifyUnicast(ctx, msgID, target, ns, t, p)

	if err != nil {
		traceutil.LogNotifierError(span, err)
	}

	// TODO: move to function
	if err == nil {
		s.logger.Log(
			"%s sent '%s::%s' notification to %s (%d/o) [%s]",
			msgID.ShortString(),
			ns,
			t,
			target.ShortString(),
			p.Len(),
			traceID,
		)
	}

	return err
}

func (s *session) NotifyMany(ctx context.Context, ns, t string, con constraint.Constraint, p *rinq.Payload) error {
	if err := nsutil.Validate(ns); err != nil {
		return err
	}

	select {
	case <-s.done:
		return rinq.NotFoundError{ID: s.id}
	default:
	}

	msgID, attrs := s.catalog.NextMessageID()

	span, ctx := traceutil.ChildOf(ctx, s.tracer, ext.SpanKindProducer)
	defer span.Finish()

	traceutil.SetupNotification(span, msgID, ns, t)
	traceutil.LogNotifierMulticast(span, attrs, con, p)

	traceID, err := s.notifier.NotifyMulticast(ctx, msgID, con, ns, t, p)

	if err != nil {
		traceutil.LogNotifierError(span, err)
	}

	// TODO: move to function
	if err == nil {
		s.logger.Log(
			"%s sent '%s::%s' notification to sessions matching %s (%d/o) [%s]",
			msgID.ShortString(),
			ns,
			t,
			con,
			p.Len(),
			traceID,
		)
	}

	return err
}

func (s *session) Listen(ns string, handler rinq.NotificationHandler) error {
	if err := nsutil.Validate(ns); err != nil {
		return err
	}

	if handler == nil {
		panic("handler must not be nil")
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	select {
	case <-s.done:
		return rinq.NotFoundError{ID: s.id}
	default:
	}

	changed, err := s.listener.Listen(
		s.id,
		ns,
		func(
			ctx context.Context,
			target rinq.Session,
			n rinq.Notification,
		) {
			rev := s.catalog.Head()
			ref := rev.Ref()

			span := opentracing.SpanFromContext(ctx)
			traceutil.SetupNotification(span, n.ID, n.Namespace, n.Type)
			traceutil.LogListenerReceived(span, ref, n)

			// TODO: move to function
			s.logger.Log(
				"%s received '%s::%s' notification from %s (%d/i) [%s]",
				ref.ShortString(),
				n.Namespace,
				n.Type,
				n.Source.Ref().ShortString(),
				n.Payload.Len(),
				trace.Get(ctx),
			)

			handler(ctx, target, n)
		},
	)

	if err != nil {
		return err
	} else if changed && s.logger.IsDebug() {
		s.logger.Log(
			"%s started listening for notifications in '%s' namespace",
			s.catalog.Ref().ShortString(),
			ns,
		)
	}

	return nil
}

func (s *session) Unlisten(ns string) error {
	if err := nsutil.Validate(ns); err != nil {
		return err
	}

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	select {
	case <-s.done:
		return rinq.NotFoundError{ID: s.id}
	default:
	}

	changed, err := s.listener.Unlisten(s.id, ns)

	if err != nil {
		return err
	} else if changed && s.logger.IsDebug() {
		s.logger.Log(
			"%s stopped listening for notifications in '%s' namespace",
			s.catalog.Ref().ShortString(),
			ns,
		)
	}

	return nil
}

func (s *session) Destroy() {
	if s.destroy() {
		logSessionDestroy(s.logger, s.catalog, "")
	}
}

func (s *session) destroy() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	select {
	case <-s.done:
		return false
	default:
		close(s.done)
		s.catalog.Close()
		s.invoker.SetAsyncHandler(s.id, nil)
		_ = s.listener.UnlistenAll(s.id)
		return true
	}
}

func (s *session) Done() <-chan struct{} {
	return s.done
}
