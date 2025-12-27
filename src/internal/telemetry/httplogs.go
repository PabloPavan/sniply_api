package telemetry

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	otelLog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

type logWriter struct {
	http.ResponseWriter
	status int
}

func (w *logWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func ChiLogMiddleware(serviceName string) func(http.Handler) http.Handler {
	logger := global.Logger(serviceName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			lw := &logWriter{ResponseWriter: w, status: 200}

			next.ServeHTTP(lw, r)

			route := ""
			if rc := chi.RouteContext(r.Context()); rc != nil {
				route = rc.RoutePattern()
			}
			if strings.TrimSpace(route) == "" {
				route = "unknown_route"
			}

			severity, severityText := severityForStatus(lw.status)
			var rec otelLog.Record
			rec.SetEventName("http.request")
			rec.SetTimestamp(time.Now())
			rec.SetSeverity(severity)
			rec.SetSeverityText(severityText)
			rec.SetBody(otelLog.StringValue("request completed"))
			rec.AddAttributes(
				otelLog.String("http.method", r.Method),
				otelLog.String("http.route", route),
				otelLog.String("http.target", r.URL.Path),
				otelLog.Int("http.status_code", lw.status),
				otelLog.Int64("http.duration_ms", time.Since(start).Milliseconds()),
			)

			logger.Emit(r.Context(), rec)
		})
	}
}

func severityForStatus(status int) (otelLog.Severity, string) {
	switch {
	case status >= 500:
		return otelLog.SeverityError, "ERROR"
	case status >= 400:
		return otelLog.SeverityWarn, "WARN"
	default:
		return otelLog.SeverityInfo, "INFO"
	}
}
