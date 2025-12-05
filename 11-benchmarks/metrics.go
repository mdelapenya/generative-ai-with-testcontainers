package main

import (
	"context"
	"fmt"
	"time"

	"github.com/mdelapenya/genai-testcontainers-go/benchmarks/semconv"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// AggregateMetrics stores aggregate metrics for a specific model/case/temp combination
type AggregateMetrics struct {
	Model              string
	TestCase           string
	Temp               float64
	LatencyP50         float64
	LatencyP95         float64
	TTFTP50            float64
	TTFTP95            float64
	PromptEvalTimeP50  float64
	PromptEvalTimeP95  float64
	SuccessRate        float64
	TokensPerOp        float64
	Score              float64
	TokensPerSec       float64 // Total TPS: (input + output) / TAT
	OutputTokensPerSec float64 // Output TPS: output tokens / generation time
	NsPerOp            float64 // Nanoseconds per operation (Go benchmark metric)
}

// MetricsCollector collects and records LLM benchmark metrics
type MetricsCollector struct {
	meter metric.Meter

	// Histograms
	latencyHistogram        metric.Float64Histogram
	ttftHistogram           metric.Float64Histogram
	promptEvalTimeHistogram metric.Float64Histogram

	// Store aggregate metrics per model/case/temp combination
	aggregates map[string]*AggregateMetrics

	// GPU metrics
	gpuUtilization float64
	gpuMemory      float64

	// Counters
	totalRequests      int64
	successfulRequests int64
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() (*MetricsCollector, error) {
	meter := otel.Meter("llm-benchmark")

	// Define histogram buckets for millisecond-scale latencies
	// Buckets: 10ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s, 30s
	latencyBuckets := []float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000}

	latencyHistogram, err := meter.Float64Histogram(
		semconv.MetricLLMLatency,
		metric.WithDescription(semconv.DescLLMLatency),
		metric.WithExplicitBucketBoundaries(latencyBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create latency histogram: %w", err)
	}

	ttftHistogram, err := meter.Float64Histogram(
		semconv.MetricLLMTTFT,
		metric.WithDescription(semconv.DescLLMTTFT),
		metric.WithExplicitBucketBoundaries(latencyBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ttft histogram: %w", err)
	}

	promptEvalTimeHistogram, err := meter.Float64Histogram(
		semconv.MetricLLMPromptEvalTime,
		metric.WithDescription(semconv.DescLLMPromptEvalTime),
		metric.WithExplicitBucketBoundaries(latencyBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt eval time histogram: %w", err)
	}

	mc := &MetricsCollector{
		meter:                   meter,
		latencyHistogram:        latencyHistogram,
		ttftHistogram:           ttftHistogram,
		promptEvalTimeHistogram: promptEvalTimeHistogram,
		aggregates:              make(map[string]*AggregateMetrics),
	}

	// Register observable gauges with callbacks that emit metrics with labels
	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMLatencyP50,
		metric.WithDescription(semconv.DescLLMLatencyP50),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.LatencyP50, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create p50 gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMLatencyP95,
		metric.WithDescription(semconv.DescLLMLatencyP95),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.LatencyP95, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create p95 gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMTTFTP50,
		metric.WithDescription(semconv.DescLLMTTFTP50),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.TTFTP50, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create ttft p50 gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMTTFTP95,
		metric.WithDescription(semconv.DescLLMTTFTP95),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.TTFTP95, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create ttft p95 gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMPromptEvalTimeP50,
		metric.WithDescription(semconv.DescLLMPromptEvalTimeP50),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.PromptEvalTimeP50, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create prompt eval time p50 gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMPromptEvalTimeP95,
		metric.WithDescription(semconv.DescLLMPromptEvalTimeP95),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.PromptEvalTimeP95, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create prompt eval time p95 gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMSuccessRate,
		metric.WithDescription(semconv.DescLLMSuccessRate),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.SuccessRate, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create success rate gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMTokensPerOp,
		metric.WithDescription(semconv.DescLLMTokensPerOp),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.TokensPerOp, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create tokens per op gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMScore,
		metric.WithDescription(semconv.DescLLMScore),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.Score, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create score gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMTokensPerSecond,
		metric.WithDescription(semconv.DescLLMTokensPerSecond),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.TokensPerSec, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create tokens per second gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMOutputTokensPerSecond,
		metric.WithDescription(semconv.DescLLMOutputTokensPerSecond),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.OutputTokensPerSec, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create output tokens per second gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMNsPerOp,
		metric.WithDescription(semconv.DescLLMNsPerOp),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.NsPerOp, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create ns per op gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricGPUUtilization,
		metric.WithDescription(semconv.DescGPUUtilization),
		metric.WithUnit(semconv.UnitPercent),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			o.Observe(mc.gpuUtilization)
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create gpu utilization gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricGPUMemory,
		metric.WithDescription(semconv.DescGPUMemory),
		metric.WithUnit(semconv.UnitMegabytes),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			o.Observe(mc.gpuMemory)
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create gpu memory gauge: %w", err)
	}

	return mc, nil
}

// RecordLatency records a latency measurement with exemplar support
func (mc *MetricsCollector) RecordLatency(ctx context.Context, latency time.Duration, model, testCase string, temp float64) {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	// Record in milliseconds for better readability in dashboards
	latencyMs := float64(latency.Milliseconds())

	attrs := []attribute.KeyValue{
		attribute.String(semconv.AttrModel, model),
		attribute.String(semconv.AttrCase, testCase),
		attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", temp)),
		attribute.String(semconv.AttrTraceID, traceID),
		attribute.String(semconv.AttrSpanID, spanID),
	}

	mc.latencyHistogram.Record(ctx, latencyMs, metric.WithAttributes(attrs...))
	mc.totalRequests++
}

// RecordTTFT records a Time To First Token measurement with exemplar support
func (mc *MetricsCollector) RecordTTFT(ctx context.Context, ttft time.Duration, model, testCase string, temp float64) {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	// Record in milliseconds for better readability in dashboards
	ttftMs := float64(ttft.Milliseconds())

	attrs := []attribute.KeyValue{
		attribute.String(semconv.AttrModel, model),
		attribute.String(semconv.AttrCase, testCase),
		attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", temp)),
		attribute.String(semconv.AttrTraceID, traceID),
		attribute.String(semconv.AttrSpanID, spanID),
	}

	mc.ttftHistogram.Record(ctx, ttftMs, metric.WithAttributes(attrs...))
}

// RecordPromptEvalTime records a prompt evaluation time measurement with exemplar support
func (mc *MetricsCollector) RecordPromptEvalTime(ctx context.Context, promptEvalTime time.Duration, model, testCase string, temp float64) {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	// Record in milliseconds for better readability in dashboards
	promptEvalTimeMs := float64(promptEvalTime.Milliseconds())

	attrs := []attribute.KeyValue{
		attribute.String(semconv.AttrModel, model),
		attribute.String(semconv.AttrCase, testCase),
		attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", temp)),
		attribute.String(semconv.AttrTraceID, traceID),
		attribute.String(semconv.AttrSpanID, spanID),
	}

	mc.promptEvalTimeHistogram.Record(ctx, promptEvalTimeMs, metric.WithAttributes(attrs...))
}

// UpdateAggregates updates the aggregate metrics (percentiles, success rate, etc.) for a specific model/case/temp combination
func (mc *MetricsCollector) UpdateAggregates(model, testCase string, temp, p50, p95, ttftP50, ttftP95, promptEvalP50, promptEvalP95, successRate, tokensPerOp, score, tokensPerSec, outputTokensPerSec, nsPerOp float64) {
	key := fmt.Sprintf("%s|%s|%.1f", model, testCase, temp)

	mc.aggregates[key] = &AggregateMetrics{
		Model:              model,
		TestCase:           testCase,
		Temp:               temp,
		LatencyP50:         p50,
		LatencyP95:         p95,
		TTFTP50:            ttftP50,
		TTFTP95:            ttftP95,
		PromptEvalTimeP50:  promptEvalP50,
		PromptEvalTimeP95:  promptEvalP95,
		SuccessRate:        successRate,
		TokensPerOp:        tokensPerOp,
		Score:              score,
		TokensPerSec:       tokensPerSec,
		OutputTokensPerSec: outputTokensPerSec,
		NsPerOp:            nsPerOp,
	}
}

// UpdateGPUMetrics updates GPU utilization and memory metrics
func (mc *MetricsCollector) UpdateGPUMetrics(utilization, memory float64) {
	mc.gpuUtilization = utilization
	mc.gpuMemory = memory
}

// IncrementSuccess increments the successful request counter
func (mc *MetricsCollector) IncrementSuccess() {
	mc.successfulRequests++
}
