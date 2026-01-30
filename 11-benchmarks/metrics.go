package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mdelapenya/genai-testcontainers-go/benchmarks/semconv"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
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
	EvalScore          float64 // Average evaluator score (0.0-1.0)
	EvalPassRate       float64 // Percentage of "yes" responses from evaluator
	TokensPerSec       float64 // Total TPS: (input + output) / TAT
	OutputTokensPerSec float64 // Output TPS: output tokens / generation time
	NsPerOp            float64 // Nanoseconds per operation (Go benchmark metric)
	// Tool calling metrics
	ToolCallCount         float64 // Average tool calls per operation
	ToolIterationCount    float64 // Average LLM-tool iterations per operation
	ToolSuccessRate       float64 // Tool call success rate (0.0-1.0)
	ToolParamAccuracy     float64 // Tool parameter extraction accuracy (0.0-1.0)
	ToolSelectionAccuracy float64 // Correct tool selection rate (0.0-1.0)
	ToolConvergence       float64 // Path convergence score (1.0 = optimal path)
	// GPU metrics (sampled during benchmark execution)
	GPUUtilization float64 // GPU utilization percentage
	GPUMemory      float64 // GPU memory usage in MB
}

// MetricsCollector collects and records LLM benchmark metrics
type MetricsCollector struct {
	meter metric.Meter

	// Histograms
	latencyHistogram         metric.Float64Histogram
	ttftHistogram            metric.Float64Histogram
	promptEvalTimeHistogram  metric.Float64Histogram
	toolCallLatencyHistogram metric.Float64Histogram

	// Store aggregate metrics per model/case/temp combination
	aggregates   map[string]*AggregateMetrics
	aggregatesMu sync.RWMutex // Protects aggregates map for concurrent access

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

	// Tool call latency histogram (smaller buckets for faster tool calls)
	toolCallBuckets := []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500}
	toolCallLatencyHistogram, err := meter.Float64Histogram(
		semconv.MetricLLMToolCallLatency,
		metric.WithDescription("Tool call execution latency"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(toolCallBuckets...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool call latency histogram: %w", err)
	}

	mc := &MetricsCollector{
		meter:                    meter,
		latencyHistogram:         latencyHistogram,
		ttftHistogram:            ttftHistogram,
		promptEvalTimeHistogram:  promptEvalTimeHistogram,
		toolCallLatencyHistogram: toolCallLatencyHistogram,
		aggregates:               make(map[string]*AggregateMetrics),
	}

	// Register observable gauges with callbacks that emit metrics with labels
	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMLatencyP50,
		metric.WithDescription(semconv.DescLLMLatencyP50),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
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
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
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
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
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
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
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
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
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
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
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
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
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
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
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
		semconv.MetricLLMEvalScore,
		metric.WithDescription(semconv.DescLLMEvalScore),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.EvalScore, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create eval score gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMEvalPassRate,
		metric.WithDescription(semconv.DescLLMEvalPassRate),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.EvalPassRate, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create eval pass rate gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMTokensPerSecond,
		metric.WithDescription(semconv.DescLLMTokensPerSecond),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
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
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
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
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
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
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.GPUUtilization, metric.WithAttributes(attrs...))
			}
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
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.GPUMemory, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create gpu memory gauge: %w", err)
	}

	// Tool call metrics gauges
	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMToolCallCount,
		metric.WithDescription("Average tool calls per operation"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.ToolCallCount, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create tool call count gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMIterationCount,
		metric.WithDescription("Average LLM-tool iterations per operation"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.ToolIterationCount, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create tool iteration count gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMToolSuccessRate,
		metric.WithDescription("Tool call success rate"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.ToolSuccessRate, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create tool success rate gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMToolParamAccuracy,
		metric.WithDescription("Tool parameter extraction accuracy (0.0-1.0)"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.ToolParamAccuracy, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create tool param accuracy gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMToolSelectionAccuracy,
		metric.WithDescription("Correct tool selection rate (0.0-1.0)"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.ToolSelectionAccuracy, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create tool selection accuracy gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		semconv.MetricLLMToolConvergence,
		metric.WithDescription("Tool calling path convergence score (1.0 = optimal path)"),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			mc.aggregatesMu.RLock()
			defer mc.aggregatesMu.RUnlock()
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(semconv.AttrModel, agg.Model),
					attribute.String(semconv.AttrCase, agg.TestCase),
					attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.ToolConvergence, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create tool convergence gauge: %w", err)
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

// RecordToolCallLatency records a tool call latency measurement with exemplar support
func (mc *MetricsCollector) RecordToolCallLatency(ctx context.Context, latency time.Duration, toolName, model, testCase string, temp float64) {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	// Record in milliseconds for better readability in dashboards
	latencyMs := float64(latency.Milliseconds())

	attrs := []attribute.KeyValue{
		attribute.String(semconv.AttrToolName, toolName),
		attribute.String(semconv.AttrModel, model),
		attribute.String(semconv.AttrCase, testCase),
		attribute.String(semconv.AttrTemp, fmt.Sprintf("%.1f", temp)),
		attribute.String(semconv.AttrTraceID, traceID),
		attribute.String(semconv.AttrSpanID, spanID),
	}

	mc.toolCallLatencyHistogram.Record(ctx, latencyMs, metric.WithAttributes(attrs...))
}

// UpdateAggregates updates the aggregate metrics (percentiles, success rate, etc.) for a specific model/case/temp combination
func (mc *MetricsCollector) UpdateAggregates(model, testCase string, temp, p50, p95, ttftP50, ttftP95, promptEvalP50, promptEvalP95, successRate, tokensPerOp, evalScore, evalPassRate, tokensPerSec, outputTokensPerSec, nsPerOp float64) {
	key := fmt.Sprintf("%s|%s|%.1f", model, testCase, temp)

	mc.aggregatesMu.Lock()
	defer mc.aggregatesMu.Unlock()
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
		EvalScore:          evalScore,
		EvalPassRate:       evalPassRate,
		TokensPerSec:       tokensPerSec,
		OutputTokensPerSec: outputTokensPerSec,
		NsPerOp:            nsPerOp,
		// Tool metrics (will be set to 0 for non-tool cases)
		ToolCallCount:         0,
		ToolIterationCount:    0,
		ToolSuccessRate:       0,
		ToolParamAccuracy:     0,
		ToolSelectionAccuracy: 0,
	}
}

// UpdateAggregatesWithToolMetrics updates the aggregate metrics including tool-specific metrics
func (mc *MetricsCollector) UpdateAggregatesWithToolMetrics(model, testCase string, temp, p50, p95, ttftP50, ttftP95, promptEvalP50, promptEvalP95, successRate, tokensPerOp, evalScore, evalPassRate, tokensPerSec, outputTokensPerSec, nsPerOp, toolCallCount, toolIterationCount, toolSuccessRate, toolParamAccuracy, toolSelectionAccuracy, toolConvergence float64) {
	key := fmt.Sprintf("%s|%s|%.1f", model, testCase, temp)

	mc.aggregatesMu.Lock()
	defer mc.aggregatesMu.Unlock()
	mc.aggregates[key] = &AggregateMetrics{
		Model:                 model,
		TestCase:              testCase,
		Temp:                  temp,
		LatencyP50:            p50,
		LatencyP95:            p95,
		TTFTP50:               ttftP50,
		TTFTP95:               ttftP95,
		PromptEvalTimeP50:     promptEvalP50,
		PromptEvalTimeP95:     promptEvalP95,
		SuccessRate:           successRate,
		TokensPerOp:           tokensPerOp,
		EvalScore:             evalScore,
		EvalPassRate:          evalPassRate,
		TokensPerSec:          tokensPerSec,
		OutputTokensPerSec:    outputTokensPerSec,
		NsPerOp:               nsPerOp,
		ToolCallCount:         toolCallCount,
		ToolIterationCount:    toolIterationCount,
		ToolSuccessRate:       toolSuccessRate,
		ToolParamAccuracy:     toolParamAccuracy,
		ToolSelectionAccuracy: toolSelectionAccuracy,
		ToolConvergence:       toolConvergence,
	}
}

// UpdateGPUMetrics updates GPU utilization and memory metrics for a specific model/case/temp
func (mc *MetricsCollector) UpdateGPUMetrics(model, testCase string, temp float64, utilization, memory float64) {
	mc.aggregatesMu.Lock()
	defer mc.aggregatesMu.Unlock()

	key := fmt.Sprintf("%s/%s/%.1f", model, testCase, temp)
	if agg, ok := mc.aggregates[key]; ok {
		agg.GPUUtilization = utilization
		agg.GPUMemory = memory
	}
}

// IncrementSuccess increments the successful request counter
func (mc *MetricsCollector) IncrementSuccess() {
	mc.successfulRequests++
}

// LogEvaluationError logs evaluation errors to OTel backend
func (mc *MetricsCollector) LogEvaluationError(ctx context.Context, model, testCase string, temp float64, err error) {
	logger := global.GetLoggerProvider().Logger("benchmark")
	var record log.Record
	record.SetSeverity(log.SeverityWarn)
	record.SetBody(log.StringValue(fmt.Sprintf("Evaluation error for %s/%s/temp%.1f", model, testCase, temp)))
	record.AddAttributes(
		log.String(semconv.AttrModel, model),
		log.String(semconv.AttrCase, testCase),
		log.String(semconv.AttrTemp, fmt.Sprintf("%.1f", temp)),
		log.String("error_type", "evaluation_error"),
		log.String("error", err.Error()),
	)
	logger.Emit(ctx, record)
}

// LogToolEvaluationError logs tool evaluation errors to OTel backend
func (mc *MetricsCollector) LogToolEvaluationError(ctx context.Context, model, testCase string, temp float64, err error) {
	logger := global.GetLoggerProvider().Logger("benchmark")
	var record log.Record
	record.SetSeverity(log.SeverityWarn)
	record.SetBody(log.StringValue(fmt.Sprintf("Tool evaluation error for %s/%s/temp%.1f", model, testCase, temp)))
	record.AddAttributes(
		log.String(semconv.AttrModel, model),
		log.String(semconv.AttrCase, testCase),
		log.String(semconv.AttrTemp, fmt.Sprintf("%.1f", temp)),
		log.String("error_type", "tool_evaluation_error"),
		log.String("error", err.Error()),
	)
	logger.Emit(ctx, record)
}

// LogBenchmarkError logs benchmark execution errors to OTel backend
func (mc *MetricsCollector) LogBenchmarkError(ctx context.Context, model, testCase string, temp float64, err error) {
	logger := global.GetLoggerProvider().Logger("benchmark")
	var record log.Record
	record.SetSeverity(log.SeverityError)
	record.SetBody(log.StringValue(fmt.Sprintf("Benchmark error for %s/%s/temp%.1f", model, testCase, temp)))
	record.AddAttributes(
		log.String(semconv.AttrModel, model),
		log.String(semconv.AttrCase, testCase),
		log.String(semconv.AttrTemp, fmt.Sprintf("%.1f", temp)),
		log.String("error_type", "benchmark_error"),
		log.String("error", err.Error()),
	)
	logger.Emit(ctx, record)
}
