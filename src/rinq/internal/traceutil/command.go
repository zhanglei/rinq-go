package traceutil

import (
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/opentracing/opentracing-go/log"
	"github.com/rinq/rinq-go/src/rinq"
	"github.com/rinq/rinq-go/src/rinq/ident"
	"github.com/rinq/rinq-go/src/rinq/internal/attrmeta"
)

var (
	invokerCallEvent      = log.String("event", "call")
	invokerCallAsyncEvent = log.String("event", "call-async")
	invokerExecuteEvent   = log.String("event", "execute")

	invokerErrorSourceClient = log.String("error.source", "client")
	invokerErrorSourceServer = log.String("error.source", "server")

	invokerFailureEvent = log.String("event", "failure")

	serverRequestEvent  = log.String("event", "request")
	serverResponseEvent = log.String("event", "response")
)

// SetupCommand configures span as a command-related span.
func SetupCommand(
	s opentracing.Span,
	id ident.MessageID,
	ns string,
	cmd string,
) {
	s.SetOperationName(ns + "::" + cmd + " command")

	s.SetTag("subsystem", "command")
	s.SetTag("message_id", id.String())
	s.SetTag("namespace", ns)
	s.SetTag("command", cmd)
}

// LogInvokerCall logs information about a "call" style invocation to s.
func LogInvokerCall(
	s opentracing.Span,
	attrs attrmeta.Table,
	p *rinq.Payload,
) {
	fields := []log.Field{
		invokerCallEvent,
		log.Int("size", p.Len()),
	}

	if len(attrs) > 0 {
		fields = append(fields, lazyString("attributes", attrs.String))
	}

	s.LogFields(fields...)
}

// LogInvokerCallAsync logs information about a "call-sync" style invocation to s.
func LogInvokerCallAsync(
	span opentracing.Span,
	attrs attrmeta.Table,
	p *rinq.Payload,
) {
	fields := []log.Field{
		invokerCallAsyncEvent,
		log.Int("size", p.Len()),
	}

	if len(attrs) > 0 {
		fields = append(fields, lazyString("attributes", attrs.String))
	}

	span.LogFields(fields...)
}

// LogInvokerExecute logs information about an "execute" style invoation to s.
func LogInvokerExecute(
	span opentracing.Span,
	attrs attrmeta.Table,
	p *rinq.Payload,
) {
	fields := []log.Field{
		invokerExecuteEvent,
		log.Int("size", p.Len()),
	}

	if len(attrs) > 0 {
		fields = append(fields, lazyString("attributes", attrs.String))
	}

	span.LogFields(fields...)
}

// LogInvokerSuccess logs information about a successful command response to s.
func LogInvokerSuccess(span opentracing.Span, p *rinq.Payload) {
	span.LogFields(
		successEvent,
		log.Int("size", p.Len()),
	)
}

// LogInvokerError logs information about err to s.
func LogInvokerError(s opentracing.Span, err error) {
	ext.Error.Set(s, true)

	switch e := err.(type) {
	case rinq.Failure:
		s.LogFields(
			invokerFailureEvent,
			log.String("error.kind", e.Type),
			log.String("message", e.Message),
			invokerErrorSourceServer,
			log.Int("size", e.Payload.Len()),
		)

	case rinq.CommandError:
		s.LogFields(
			errorEvent,
			log.String("message", e.Error()),
			invokerErrorSourceServer,
		)

	default:
		s.LogFields(
			errorEvent,
			log.String("message", e.Error()),
			invokerErrorSourceClient,
		)
	}
}

// LogServerRequest logs information about an incoming command request to s.
func LogServerRequest(s opentracing.Span, peerID ident.PeerID, p *rinq.Payload) {
	s.LogFields(
		serverRequestEvent,
		log.String("server", peerID.String()),
		log.Int("size", p.Len()),
	)
}

// LogServerSuccess logs information about a successful command response to s.
func LogServerSuccess(span opentracing.Span, p *rinq.Payload) {
	span.LogFields(
		serverResponseEvent,
		log.Int("size", p.Len()),
	)
}

// LogServerError logs information about err to s.
func LogServerError(s opentracing.Span, err error) {
	switch e := err.(type) {
	case rinq.Failure:
		s.LogFields(
			serverResponseEvent,
			log.String("error.kind", e.Type),
			log.String("message", e.Message),
			log.Int("size", e.Payload.Len()),
		)

	default:
		ext.Error.Set(s, true)

		s.LogFields(
			serverResponseEvent,
			log.String("message", e.Error()),
		)
	}
}