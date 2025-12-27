package telemetry

import (
	"os"
	"strings"
)

func otlpEndpoint(signalEnv string) string {
	endpoint := strings.TrimSpace(os.Getenv(signalEnv))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	}
	if endpoint == "" {
		endpoint = "localhost:4317"
	}
	return endpoint
}
