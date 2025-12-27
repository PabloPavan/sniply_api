package telemetry

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func ChiTraceMiddleware(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w, status: 200}

			spanName := "HTTP " + r.Method + " " + r.URL.Path
			ctx, span := tracer.Start(r.Context(), spanName)
			defer span.End()

			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.target", r.URL.Path),
			)

			next.ServeHTTP(sw, r.WithContext(ctx))

			route := ""
			if rc := chi.RouteContext(r.Context()); rc != nil {
				route = rc.RoutePattern()
			}
			if strings.TrimSpace(route) == "" {
				route = "unknown_route"
			}

			span.SetAttributes(attribute.String("http.route", route))
			span.SetName("HTTP " + r.Method + " " + route)
			span.SetAttributes(attribute.Int("http.status_code", sw.status))
			if sw.status >= 500 {
				span.SetStatus(codes.Error, "server_error")
			}
		})
	}
}
