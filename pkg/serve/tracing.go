package serve

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// propagator is W3C trace context (plus baggage). Every trace-aware wrapper in this package names it explicitly, so
// behaviour does not depend on which global propagator a process happened to install.
var propagator = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

// InitTracing installs the process-wide tracer. Every inbound request becomes a span that continues the
// caller's W3C trace context, every outbound call from Outbound carries it on, and access logs record the
// trace ID. Spans are exported over OTLP/HTTP to the endpoint in OTEL_EXPORTER_OTLP_ENDPOINT (the standard
// OpenTelemetry variable). With no endpoint set, spans are still created and propagated but not exported.
//
// The returned function flushes and stops the provider; call it on exit.
func InitTracing(ctx context.Context, service string) (func(context.Context) error, error) {
	if service == "" {
		return nil, fmt.Errorf("tracing needs a service name")
	}
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", service))),
	}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		exp, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("otlp exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	}
	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagator)
	return tp.Shutdown, nil
}
