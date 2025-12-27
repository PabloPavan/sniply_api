package telemetry

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	otelMetricsEnabled     bool
	otelHTTPRequestsTotal  metric.Int64Counter
	otelHTTPRequestSeconds metric.Float64Histogram
)

func initHTTPMetricsInstruments(serviceName string) {
	meter := otel.Meter(serviceName)

	var err error
	otelHTTPRequestsTotal, err = meter.Int64Counter(
		"sniply_http_requests_total",
		metric.WithDescription("Total de requisicoes HTTP"),
	)
	if err != nil {
		return
	}

	otelHTTPRequestSeconds, err = meter.Float64Histogram(
		"sniply_http_request_duration_seconds",
		metric.WithDescription("Latencia das requisicoes HTTP"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return
	}

	otelMetricsEnabled = true
}

type metricsWriter struct {
	http.ResponseWriter
	status int
}

func (w *metricsWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func ChiMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		mw := &metricsWriter{ResponseWriter: w, status: 200}

		next.ServeHTTP(mw, r)

		route := "unknown_route"
		if rc := chi.RouteContext(r.Context()); rc != nil {
			if rp := rc.RoutePattern(); rp != "" {
				route = rp
			}
		}

		if otelMetricsEnabled {
			attrs := []attribute.KeyValue{
				attribute.String("http.method", r.Method),
				attribute.String("http.route", route),
				attribute.Int("http.status_code", mw.status),
			}
			otelHTTPRequestsTotal.Add(r.Context(), 1, metric.WithAttributes(attrs...))
			otelHTTPRequestSeconds.Record(
				r.Context(),
				time.Since(start).Seconds(),
				metric.WithAttributes(attrs...),
			)
		}
	})
}
