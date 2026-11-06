// Package tracing initializes OpenTelemetry trace IDs and W3C propagation.
package tracing

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Config struct {
	ServiceName string
}

// Setup is called once by the composition root, before starting servers/clients.
// No exporter is installed: spans correlate logs without an external collector.
func Setup(cfg Config) (*sdktrace.TracerProvider, error) {
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("tracing service name is required")
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", cfg.ServiceName))))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return provider, nil
}
