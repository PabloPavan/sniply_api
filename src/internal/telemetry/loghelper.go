package telemetry

import (
	"context"
	"time"

	otelLog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

const defaultLogScope = "sniply-api"

// Log emits a structured log event with optional attributes.
func Log(ctx context.Context, severity otelLog.Severity, msg string, attrs ...otelLog.KeyValue) {
	logger := global.Logger(defaultLogScope)

	var rec otelLog.Record
	rec.SetEventName("app.log")
	rec.SetTimestamp(time.Now())
	rec.SetSeverity(severity)
	rec.SetSeverityText(severityText(severity))
	rec.SetBody(otelLog.StringValue(msg))
	rec.AddAttributes(attrs...)

	logger.Emit(ctx, rec)
}

func LogInfo(ctx context.Context, msg string, attrs ...otelLog.KeyValue) {
	Log(ctx, otelLog.SeverityInfo, msg, attrs...)
}

func LogWarn(ctx context.Context, msg string, attrs ...otelLog.KeyValue) {
	Log(ctx, otelLog.SeverityWarn, msg, attrs...)
}

func LogError(ctx context.Context, msg string, attrs ...otelLog.KeyValue) {
	Log(ctx, otelLog.SeverityError, msg, attrs...)
}

func severityText(sev otelLog.Severity) string {
	switch {
	case sev >= otelLog.SeverityError:
		return "ERROR"
	case sev >= otelLog.SeverityWarn:
		return "WARN"
	default:
		return "INFO"
	}
}
