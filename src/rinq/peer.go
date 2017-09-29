package rinq

import "github.com/rinq/rinq-go/src/rinq/ident"

// Peer represents a connection to a Rinq network.
//
// Peers can act as a server, responding to application-defined commands.
// Use Peer.Listen() to start accepting incoming command requests.
//
// Command request are sent by sessions, represented by the Session interface.
// Sessions can also send notifications to other sessions. Sessions are created
// by calling Peer.Session().
//
// Each peer is assigned a unique ID, which is represented by the PeerID struct.
// All IDs generated by the peer, such as session IDs and message IDs contain
// the peer ID, so that they can be traced to their origin easily.
type Peer interface {
	// ID returns the peer's unique identifier.
	ID() ident.PeerID

	// Session returns a new session owned by this peer.
	//
	// Creating a session does not perform any network IO. The only limit to the
	// number of sessions is the memory required to store them.
	//
	// Sessions created after the peer has been stopped are unusable. Any
	// operation will fail immediately.
	Session() Session

	// Listen starts listening for command requests in the given namespace.
	//
	// When a command request is received with a namespace equal to ns, the
	// handler h is invoked.
	//
	// Repeated calls to Listen() with the same namespace simply changes the
	// handler associated with that namespace.
	//
	// h is invoked on its own goroutine for each command request.
	Listen(ns string, h CommandHandler) error

	// Unlisten stops listening for command requests in the given namepsace.
	//
	// If the peer is not currently listening to ns, nil is returned immediately.
	Unlisten(ns string) error

	// Done returns a channel that is closed when the peer is stopped.
	//
	// Err() may be called to obtain the error that caused the peer to stop, if
	// any occurred.
	Done() <-chan struct{}

	// Err returns the error that caused the Done() channel to close.
	//
	// A nil return value indicates that the peer was stopped because Stop() or
	// GracefulStop() has been called.
	Err() error

	// Stop instructs the peer to disconnect from the network immediately.
	//
	// Stop does NOT block until the peer is disconnected. Use the Done()
	// channel to wait for the peer to disconnect.
	Stop()

	// GracefulStop() instructs the peer to disconnect from the network once
	// all pending operations have completed.
	//
	// Any calls to Session.Call(), command handlers or notification handlers
	// must return before the peer has stopped.
	//
	// GracefulStop does NOT block until the peer is disconnected. Use the
	// Done() channel to wait for the peer to disconnect.
	GracefulStop()
}
