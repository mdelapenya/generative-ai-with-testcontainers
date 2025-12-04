package main

import (
	"context"
	"fmt"
	"time"

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
	PromptEvalTimeP50  float64
	PromptEvalTimeP95  float64
	SuccessRate        float64
	TokensPerOp        float64
	Score              float64
	TokensPerSec       float64 // Total TPS: (input + output) / TAT
	OutputTokensPerSec float64 // Output TPS: output tokens / generation time
}

// MetricsCollector collects and records LLM benchmark metrics
type MetricsCollector struct {
	meter metric.Meter

	// Histograms
	latencyHistogram         metric.Float64Histogram
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

	latencyHistogram, err := meter.Float64Histogram(
		MetricLLMLatency,
		metric.WithDescription(DescLLMLatency),
		metric.WithUnit(UnitMilliseconds),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create latency histogram: %w", err)
	}

	promptEvalTimeHistogram, err := meter.Float64Histogram(
		MetricLLMPromptEvalTime,
		metric.WithDescription(DescLLMPromptEvalTime),
		metric.WithUnit(UnitMilliseconds),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt eval time histogram: %w", err)
	}

	mc := &MetricsCollector{
		meter:                   meter,
		latencyHistogram:        latencyHistogram,
		promptEvalTimeHistogram: promptEvalTimeHistogram,
		aggregates:              make(map[string]*AggregateMetrics),
	}

	// Register observable gauges with callbacks that emit metrics with labels
	if _, err := meter.Float64ObservableGauge(
		MetricLLMLatencyP50,
		metric.WithDescription(DescLLMLatencyP50),
		metric.WithUnit(UnitMilliseconds),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(AttrModel, agg.Model),
					attribute.String(AttrCase, agg.TestCase),
					attribute.String(AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.LatencyP50, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create p50 gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		MetricLLMLatencyP95,
		metric.WithDescription(DescLLMLatencyP95),
		metric.WithUnit(UnitMilliseconds),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(AttrModel, agg.Model),
					attribute.String(AttrCase, agg.TestCase),
					attribute.String(AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.LatencyP95, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create p95 gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		MetricLLMPromptEvalTimeP50,
		metric.WithDescription(DescLLMPromptEvalTimeP50),
		metric.WithUnit(UnitMilliseconds),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(AttrModel, agg.Model),
					attribute.String(AttrCase, agg.TestCase),
					attribute.String(AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.PromptEvalTimeP50, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create prompt eval time p50 gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		MetricLLMPromptEvalTimeP95,
		metric.WithDescription(DescLLMPromptEvalTimeP95),
		metric.WithUnit(UnitMilliseconds),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(AttrModel, agg.Model),
					attribute.String(AttrCase, agg.TestCase),
					attribute.String(AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.PromptEvalTimeP95, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create prompt eval time p95 gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		MetricLLMSuccessRate,
		metric.WithDescription(DescLLMSuccessRate),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(AttrModel, agg.Model),
					attribute.String(AttrCase, agg.TestCase),
					attribute.String(AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.SuccessRate, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create success rate gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		MetricLLMTokensPerOp,
		metric.WithDescription(DescLLMTokensPerOp),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(AttrModel, agg.Model),
					attribute.String(AttrCase, agg.TestCase),
					attribute.String(AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.TokensPerOp, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create tokens per op gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		MetricLLMScore,
		metric.WithDescription(DescLLMScore),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(AttrModel, agg.Model),
					attribute.String(AttrCase, agg.TestCase),
					attribute.String(AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.Score, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create score gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		MetricLLMTokensPerSecond,
		metric.WithDescription(DescLLMTokensPerSecond),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(AttrModel, agg.Model),
					attribute.String(AttrCase, agg.TestCase),
					attribute.String(AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.TokensPerSec, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create tokens per second gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		MetricLLMOutputTokensPerSecond,
		metric.WithDescription(DescLLMOutputTokensPerSecond),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			for _, agg := range mc.aggregates {
				attrs := []attribute.KeyValue{
					attribute.String(AttrModel, agg.Model),
					attribute.String(AttrCase, agg.TestCase),
					attribute.String(AttrTemp, fmt.Sprintf("%.1f", agg.Temp)),
				}
				o.Observe(agg.OutputTokensPerSec, metric.WithAttributes(attrs...))
			}
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create output tokens per second gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		MetricGPUUtilization,
		metric.WithDescription(DescGPUUtilization),
		metric.WithUnit(UnitPercent),
		metric.WithFloat64Callback(func(ctx context.Context, o metric.Float64Observer) error {
			o.Observe(mc.gpuUtilization)
			return nil
		}),
	); err != nil {
		return nil, fmt.Errorf("failed to create gpu utilization gauge: %w", err)
	}

	if _, err := meter.Float64ObservableGauge(
		MetricGPUMemory,
		metric.WithDescription(DescGPUMemory),
		metric.WithUnit(UnitMegabytes),
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

	latencyMs := float64(latency.Milliseconds())

	attrs := []attribute.KeyValue{
		attribute.String(AttrModel, model),
		attribute.String(AttrCase, testCase),
		attribute.String(AttrTemp, fmt.Sprintf("%.1f", temp)),
		attribute.String(AttrTraceID, traceID),
		attribute.String(AttrSpanID, spanID),
	}

	mc.latencyHistogram.Record(ctx, latencyMs, metric.WithAttributes(attrs...))
	mc.totalRequests++
}

// RecordPromptEvalTime records a prompt evaluation time measurement with exemplar support
func (mc *MetricsCollector) RecordPromptEvalTime(ctx context.Context, promptEvalTime time.Duration, model, testCase string, temp float64) {
	span := trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	promptEvalTimeMs := float64(promptEvalTime.Milliseconds())

	attrs := []attribute.KeyValue{
		attribute.String(AttrModel, model),
		attribute.String(AttrCase, testCase),
		attribute.String(AttrTemp, fmt.Sprintf("%.1f", temp)),
		attribute.String(AttrTraceID, traceID),
		attribute.String(AttrSpanID, spanID),
	}

	mc.promptEvalTimeHistogram.Record(ctx, promptEvalTimeMs, metric.WithAttributes(attrs...))
}

// UpdateAggregates updates the aggregate metrics (percentiles, success rate, etc.) for a specific model/case/temp combination
func (mc *MetricsCollector) UpdateAggregates(model, testCase string, temp, p50, p95, promptEvalP50, promptEvalP95, successRate, tokensPerOp, score, tokensPerSec, outputTokensPerSec float64) {
	key := fmt.Sprintf("%s|%s|%.1f", model, testCase, temp)

	mc.aggregates[key] = &AggregateMetrics{
		Model:              model,
		TestCase:           testCase,
		Temp:               temp,
		LatencyP50:         p50,
		LatencyP95:         p95,
		PromptEvalTimeP50:  promptEvalP50,
		PromptEvalTimeP95:  promptEvalP95,
		SuccessRate:        successRate,
		TokensPerOp:        tokensPerOp,
		Score:              score,
		TokensPerSec:       tokensPerSec,
		OutputTokensPerSec: outputTokensPerSec,
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
