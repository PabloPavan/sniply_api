package telemetry

import (
	"context"
	"log"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func InitLogger(serviceName string) func(context.Context) error {
	ctx := context.Background()
	endpoint := otlpEndpoint("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT")

	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithEndpoint(endpoint),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal(err)
	}

	processor := sdklog.NewBatchProcessor(exporter)
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(processor),
		sdklog.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
		)),
	)

	global.SetLoggerProvider(lp)
	return lp.Shutdown
}
