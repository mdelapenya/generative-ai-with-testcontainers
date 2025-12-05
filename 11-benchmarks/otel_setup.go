package main

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
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
	LoggerProvider *log.LoggerProvider
}

// getCPUModel attempts to get the CPU model name for the current platform
func getCPUModel() string {
	switch runtime.GOOS {
	case "darwin":
		// macOS: use sysctl to get CPU brand string
		cmd := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
		if output, err := cmd.Output(); err == nil {
			return strings.TrimSpace(string(output))
		}
	case "linux":
		// Try x86/x64 first
		cmd := exec.Command("grep", "-m", "1", "model name", "/proc/cpuinfo")
		if output, err := cmd.Output(); err == nil {
			parts := strings.SplitN(string(output), ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
		// Fallback: just return the architecture
		return runtime.GOARCH
	case "windows":
		cmd := exec.Command("wmic", "cpu", "get", "name")
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) > 1 {
				return strings.TrimSpace(lines[1])
			}
		}
	}
	return runtime.GOARCH
}

// InitOTel initializes OpenTelemetry with OTLP exporters for traces and metrics
func InitOTel(ctx context.Context, otlpEndpoint string) (*OtelSetup, error) {
	// Get CPU model info
	cpuModel := getCPUModel()

	// Create resource with service and runtime information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			// Add Go runtime information (matching Go benchmark output)
			attribute.String("go.os", runtime.GOOS),           // goos: darwin
			attribute.String("go.arch", runtime.GOARCH),       // goarch: arm64
			attribute.String("go.version", runtime.Version()), // go version
			attribute.Int("go.numcpu", runtime.NumCPU()),      // GOMAXPROCS
			attribute.String("cpu.model", cpuModel),           // cpu: Apple M4 Max
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

	// Setup log exporter
	logExporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpoint(otlpEndpoint),
		otlploghttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create log exporter: %w", err)
	}

	// Setup log provider with batch processor
	loggerProvider := log.NewLoggerProvider(
		log.WithProcessor(log.NewBatchProcessor(logExporter,
			log.WithExportInterval(time.Second),
		)),
		log.WithResource(res),
	)

	// Set global providers
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	global.SetLoggerProvider(loggerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return &OtelSetup{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
		LoggerProvider: loggerProvider,
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

	if err := o.LoggerProvider.Shutdown(ctx); err != nil {
		errs = append(errs, fmt.Errorf("logger provider shutdown: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}
