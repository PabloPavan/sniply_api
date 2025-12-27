package telemetry

import (
	"context"

	otelLog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"
)

func TraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}

func LogString(key, value string) otelLog.KeyValue {
	return otelLog.String(key, value)
}

func LogInt(key string, value int) otelLog.KeyValue {
	return otelLog.Int(key, value)
}

func LogInt64(key string, value int64) otelLog.KeyValue {
	return otelLog.Int64(key, value)
}

func LogBool(key string, value bool) otelLog.KeyValue {
	return otelLog.Bool(key, value)
}
