package commandamqp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/over-pass/overpass-go/src/internals"
	"github.com/over-pass/overpass-go/src/internals/amqputil"
	"github.com/over-pass/overpass-go/src/internals/command"
	"github.com/over-pass/overpass-go/src/internals/deferutil"
	"github.com/over-pass/overpass-go/src/overpass"
	"github.com/streadway/amqp"
)

type server struct {
	peerID    overpass.PeerID
	preFetch  int
	revisions internals.RevisionStore
	queues    *queueSet
	channels  amqputil.ChannelPool
	logger    *log.Logger

	mutex    sync.RWMutex
	channel  *amqp.Channel
	handlers map[string]overpass.CommandHandler

	done chan struct{}
	err  atomic.Value
}

// newServer creates, starts and returns a new server.
func newServer(
	peerID overpass.PeerID,
	preFetch int,
	revisions internals.RevisionStore,
	queues *queueSet,
	channels amqputil.ChannelPool,
	logger *log.Logger,
) (command.Server, error) {
	s := &server{
		peerID:    peerID,
		preFetch:  preFetch,
		revisions: revisions,
		queues:    queues,
		channels:  channels,
		logger:    logger,
		handlers:  map[string]overpass.CommandHandler{},
		done:      make(chan struct{}),
	}

	if err := s.initialize(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *server) Listen(namespace string, handler overpass.CommandHandler) (bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// we're already listening, just swap the handler
	if _, ok := s.handlers[namespace]; ok {
		s.handlers[namespace] = handler
		fmt.Println("already listening")
		return false, nil
	}

	if err := s.channel.QueueBind(
		requestQueue(s.peerID),
		namespace,
		multicastExchange,
		false, // noWait
		nil,   //  args
	); err != nil {
		fmt.Println(err)
		return false, err
	}

	queue, err := s.queues.Get(s.channel, namespace)
	if err != nil {
		fmt.Println(err)
		return false, err
	}

	messages, err := s.channel.Consume(
		queue,
		queue, // use queue name as consumer tag
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		fmt.Println(err)
		return false, err
	}

	s.handlers[namespace] = handler
	go s.dispatchEach(messages)

	return true, nil
}

func (s *server) Unlisten(namespace string) (bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, ok := s.handlers[namespace]; !ok {
		return false, nil
	}

	if err := s.channel.QueueUnbind(
		requestQueue(s.peerID),
		namespace,
		multicastExchange,
		nil, //  args
	); err != nil {
		return false, err
	}

	if err := s.channel.Cancel(
		balancedRequestQueue(namespace), // use queue name as consumer tag
		false, // noWait
	); err != nil {
		return false, err
	}

	delete(s.handlers, namespace)

	return true, nil
}

func (s *server) Done() <-chan struct{} {
	return s.done
}

func (s *server) Error() error {
	if err, ok := s.err.Load().(error); ok {
		return err
	}

	return nil
}

func (s *server) dispatchEach(messages <-chan amqp.Delivery) {
	for msg := range messages {
		fmt.Println(msg)
		go s.dispatch(msg)
	}
}

func (s *server) dispatch(msg amqp.Delivery) {
	msgID, err := overpass.ParseMessageID(msg.MessageId)
	if err != nil {
		msg.Reject(false)
		s.logger.Printf(
			"%s ignored AMQP message, '%s' is not a valid message ID",
			s.peerID.ShortString(),
			msg.MessageId,
		)
		return
	}

	switch msg.Exchange {
	case balancedExchange, multicastExchange:
		err = s.handle(msgID, msg.RoutingKey, msg)
	case unicastExchange:
		if namespace, ok := msg.Headers[namespaceHeader].(string); ok {
			err = s.handle(msgID, namespace, msg)
		} else {
			err = errors.New("malformed request, namespace is not a string")
		}
	default:
		err = fmt.Errorf("delivery via '%s' exchange is not expected", msg.Exchange)
	}

	if err != nil {
		msg.Reject(false)
		s.logger.Printf(
			"%s ignored AMQP message %s, %s",
			s.peerID.ShortString(),
			msgID.ShortString(),
			err,
		)
	}
}

func (s *server) handle(msgID overpass.MessageID, namespace string, msg amqp.Delivery) error {
	var handler overpass.CommandHandler
	deferutil.RWith(&s.mutex, func() {
		handler = s.handlers[namespace]
	})

	if handler == nil {
		msg.Reject(true)
		// TODO: log - request was probably in network buffer before unlisten was called
		fmt.Println("no handler ", msg)
		return nil
	}

	source, err := s.revisions.GetRevision(msgID.Session)
	if err != nil {
		return err
	}

	ctx := amqputil.WithCorrelationID(context.Background(), msg)
	ctx, cancel := amqputil.WithExpiration(ctx, msg)
	defer cancel()

	cmd := overpass.Command{
		Source:      source,
		Namespace:   msg.RoutingKey,
		Command:     msg.Type,
		Payload:     overpass.NewPayloadFromBytes(msg.Body),
		IsMulticast: msg.Exchange == multicastExchange,
	}

	res := &responder{
		channels:   s.channels,
		context:    ctx,
		msgID:      msgID,
		isRequired: msg.ReplyTo != "",
		logger:     s.logger,
	}

	handler(ctx, cmd, res)

	if res.IsClosed() {
		msg.Ack(true)
	} else if msg.Exchange == balancedExchange {
		// requeue in the hopes another peer can handle it properly
		msg.Reject(true)
	} else {
		msg.Reject(false) // TODO: panic?
	}

	return nil
}

func (s *server) initialize() error {
	channel, err := s.channels.Get() // do not return to pool, used for consume
	if err != nil {
		return err
	}

	if err = channel.Qos(s.preFetch, 0, true); err != nil {
		return err
	}

	queue := requestQueue(s.peerID)

	if _, err = channel.QueueDeclare(
		queue,
		false, // durable
		false, // autoDelete
		true,  // exclusive,
		false, // noWait
		nil,   // args
	); err != nil {
		return err
	}

	if err = channel.QueueBind(
		queue,
		s.peerID.String(),
		unicastExchange,
		false, // noWait
		nil,   // args
	); err != nil {
		return err
	}

	messages, err := channel.Consume(
		queue,
		queue, // use queue name as consumer tag
		false, // autoAck
		true,  // exclusive
		false, // noLocal
		false, // noWait
		nil,   // args
	)
	if err != nil {
		return err
	}

	s.channel = channel

	go s.dispatchEach(messages)
	go s.waitForChannel()

	return nil
}

func (s *server) waitForChannel() {
	done := s.channel.NotifyClose(make(chan *amqp.Error))

	if amqpErr := <-done; amqpErr != nil {
		// we can't just return err when it's nil, because it will be a nil
		// *amqp.Error, as opposed to a nil "error" interface.
		s.close(amqpErr)
	} else {
		s.close(nil)
	}
}

func (s *server) close(err error) {
	if err != nil {
		s.err.Store(err)
	}
	close(s.done)
	s.channel.Close() // TODO lock mutes?
}
