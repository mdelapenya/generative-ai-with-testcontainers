package main

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const (
	serviceName    = "llm-benchmark"
	serviceVersion = "0.1.0"
)

// OtelSetup holds the OpenTelemetry providers and exporters
type OtelSetup struct {
	TracerProvider *trace.TracerProvider
	MeterProvider  *metric.MeterProvider
}

// InitOTel initializes OpenTelemetry with OTLP exporters for traces and metrics
func InitOTel(ctx context.Context, otlpEndpoint string) (*OtelSetup, error) {
	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Setup trace exporter
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Setup trace provider with batch processor
	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter,
			trace.WithBatchTimeout(time.Second),
		),
		trace.WithResource(res),
	)

	// Setup metric exporter
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(otlpEndpoint),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Setup metric provider with periodic reader
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(
			metric.NewPeriodicReader(metricExporter,
				metric.WithInterval(5*time.Second),
			),
		),
		metric.WithResource(res),
	)

	// Set global providers
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return &OtelSetup{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
	}, nil
}

// Shutdown gracefully shuts down the OpenTelemetry providers
func (o *OtelSetup) Shutdown(ctx context.Context) error {
	var errs []error

	if err := o.TracerProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("tracer provider shutdown: %w", err))
	}

	if err := o.MeterProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}
