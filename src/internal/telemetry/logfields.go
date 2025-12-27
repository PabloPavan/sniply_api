package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

func TraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}
