package commandamqp

import (
	"context"

	"github.com/over-pass/overpass-go/src/internals/amqputil"
	"github.com/over-pass/overpass-go/src/overpass"
	"github.com/streadway/amqp"
)

func logInvalidMessageID(
	logger overpass.Logger,
	peerID overpass.PeerID,
	msg amqp.Delivery,
) {
	if !logger.IsDebug() {
		return
	}

	logger.Log(
		"%s command server ignored AMQP message, '%s' is not a valid message ID",
		peerID.ShortString(),
		msg.MessageId,
	)
}

func logIgnoredMessage(
	logger overpass.Logger,
	peerID overpass.PeerID,
	msgID overpass.MessageID,
	err error,
) {
	if !logger.IsDebug() {
		return
	}

	logger.Log(
		"%s command server ignored AMQP message %s, %s",
		peerID.ShortString(),
		msgID.ShortString(),
		err,
	)
}

func logRequestBegin(
	ctx context.Context,
	logger overpass.Logger,
	peerID overpass.PeerID,
	msgID overpass.MessageID,
	request overpass.Command,
) {
	if !logger.IsDebug() {
		return
	}

	logger.Log(
		"%s began '%s::%s' command request %s [%s] >>> %s",
		peerID.ShortString(),
		request.Namespace,
		request.Command,
		msgID.ShortString(),
		amqputil.GetCorrelationID(ctx),
		request.Payload,
	)
}

func logRequestEnd(
	ctx context.Context,
	logger overpass.Logger,
	peerID overpass.PeerID,
	msgID overpass.MessageID,
	request overpass.Command,
	payload *overpass.Payload,
	err error,
) {
	if !logger.IsDebug() {
		return
	}

	switch e := err.(type) {
	case nil:
		logger.Log(
			"%s completed '%s::%s' command request %s successfully [%s] <<< %s",
			peerID.ShortString(),
			request.Namespace,
			request.Command,
			msgID.ShortString(),
			amqputil.GetCorrelationID(ctx),
			payload,
		)
	case overpass.Failure:
		logger.Log(
			"%s completed '%s::%s' command request %s with '%s' failure: %s [%s] <<< %s",
			peerID.ShortString(),
			request.Namespace,
			request.Command,
			msgID.ShortString(),
			e.Type,
			e.Message,
			amqputil.GetCorrelationID(ctx),
			payload,
		)
	default:
		logger.Log(
			"%s completed '%s::%s' command request %s with error [%s] <<< %s",
			peerID.ShortString(),
			request.Namespace,
			request.Command,
			msgID.ShortString(),
			amqputil.GetCorrelationID(ctx),
			err,
		)
	}
}

func logNoLongerListening(
	logger overpass.Logger,
	peerID overpass.PeerID,
	msgID overpass.MessageID,
	namespace string,
) {
	if !logger.IsDebug() {
		return
	}

	logger.Log(
		"%s is no longer listening to '%s' namespace, request %s has been re-queued",
		peerID.ShortString(),
		namespace,
		msgID.ShortString(),
	)
}

func logRequestRequeued(
	ctx context.Context,
	logger overpass.Logger,
	peerID overpass.PeerID,
	msgID overpass.MessageID,
	request overpass.Command,
) {
	if !logger.IsDebug() {
		return
	}

	logger.Log(
		"%s did not write a response for '%s::%s' command request, request %s has been re-queued [%s]",
		peerID.ShortString(),
		request.Namespace,
		request.Command,
		msgID.ShortString(),
		amqputil.GetCorrelationID(ctx),
	)
}

func logRequestRejected(
	ctx context.Context,
	logger overpass.Logger,
	peerID overpass.PeerID,
	msgID overpass.MessageID,
	request overpass.Command,
) {
	logger.Log(
		"%s did not write a response for '%s::%s' command request %s, request has been abandoned [%s]",
		peerID.ShortString(),
		request.Namespace,
		request.Command,
		msgID.ShortString(),
		amqputil.GetCorrelationID(ctx),
	)
}

func logServerStart(
	logger overpass.Logger,
	peerID overpass.PeerID,
	preFetch int,
) {
	if !logger.IsDebug() {
		return
	}

	logger.Log(
		"%s command server started with pre-fetch of %d message(s)",
		peerID.ShortString(),
		preFetch,
	)
}

func logServerStopping(
	logger overpass.Logger,
	peerID overpass.PeerID,
) {
	if !logger.IsDebug() {
		return
	}

	logger.Log(
		"%s command server is stopping gracefully",
		peerID.ShortString(),
	)
}

func logServerStop(
	logger overpass.Logger,
	peerID overpass.PeerID,
	err error,
) {
	if !logger.IsDebug() {
		return
	}

	if err == nil {
		logger.Log(
			"%s command server stopped",
			peerID.ShortString(),
		)
	} else {
		logger.Log(
			"%s command server stopped with error: %s",
			peerID.ShortString(),
			err,
		)
	}
}