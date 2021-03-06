package command

import (
	"context"

	"github.com/rinq/rinq-go/src/internal/service"
	"github.com/rinq/rinq-go/src/rinq"
	"github.com/rinq/rinq-go/src/rinq/ident"
)

// Invoker is a low-level RPC interface, it is used to implement the
// "command subsystem", as well as internal peer-to-peer requests.
//
// The terminology "call" refers to an invocation that expects a response,
// whereas "execute" is an invocation where no response is required.
type Invoker interface {
	service.Service

	// CallUnicast sends a unicast command request to a specific peer and blocks
	// until a response is received or the context deadline is met.
	CallUnicast(
		ctx context.Context,
		msgID ident.MessageID,
		traceID string,
		target ident.PeerID,
		namespace string,
		command string,
		payload *rinq.Payload,
	) (*rinq.Payload, error)

	// CallBalanced sends a load-balanced command request to the first available
	// peer and blocks until a response is received or the context deadline is met.
	CallBalanced(
		ctx context.Context,
		msgID ident.MessageID,
		traceID string,
		namespace string,
		command string,
		payload *rinq.Payload,
	) (*rinq.Payload, error)

	// CallBalancedAsync sends a load-balanced command request to the first
	// available peer, instructs it to send a response, but does not block.
	CallBalancedAsync(
		ctx context.Context,
		msgID ident.MessageID,
		traceID string,
		namespace string,
		command string,
		payload *rinq.Payload,
	) error

	// SetAsyncHandler sets the asynchronous handler to use for a specific
	// session.
	SetAsyncHandler(sessID ident.SessionID, h rinq.AsyncHandler)

	// ExecuteBalanced sends a load-balanced command request to the first
	// available peer and returns immediately.
	ExecuteBalanced(
		ctx context.Context,
		msgID ident.MessageID,
		traceID string,
		namespace string,
		command string,
		payload *rinq.Payload,
	) error

	// ExecuteMulticast sends a multicast command request to the all available
	// peers and returns immediately.
	ExecuteMulticast(
		ctx context.Context,
		msgID ident.MessageID,
		traceID string,
		namespace string,
		command string,
		payload *rinq.Payload,
	) error
}
