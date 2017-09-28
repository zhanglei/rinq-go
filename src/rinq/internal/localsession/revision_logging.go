package localsession

import (
	"bytes"
	"context"

	"github.com/rinq/rinq-go/src/rinq"
	"github.com/rinq/rinq-go/src/rinq/ident"
	"github.com/rinq/rinq-go/src/rinq/trace"
)

func logUpdate(
	ctx context.Context,
	logger rinq.Logger,
	ref ident.Ref,
	ns string,
	diff *bytes.Buffer,
) {
	if traceID := trace.Get(ctx); traceID != "" {
		logger.Log(
			"%s session updated {%s::%s} [%s]",
			ref.ShortString(),
			ns,
			diff.String(),
			traceID,
		)
	} else {
		logger.Log(
			"%s session updated {%s::%s}",
			ref.ShortString(),
			ns,
			diff.String(),
		)
	}
}

// TODO: should this be called logDestroy ?
func logClose(
	ctx context.Context,
	logger rinq.Logger,
	cat Catalog,
) {
	logSessionDestroy(logger, cat, trace.Get(ctx))
}
