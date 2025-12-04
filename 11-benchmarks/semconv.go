package main

import "strings"

// Semantic conventions for LLM benchmark metrics and attributes
// This file defines constants for metric names and attribute keys to ensure consistency

const (
	// Metric names - using dot notation which OpenTelemetry will convert to underscores in Prometheus
	MetricLLMLatency              = "llm.latency"
	MetricLLMLatencyP50           = "llm.latency.p50"
	MetricLLMLatencyP95           = "llm.latency.p95"
	MetricLLMPromptEvalTime       = "llm.prompt_eval_time"
	MetricLLMPromptEvalTimeP50    = "llm.prompt_eval_time.p50"
	MetricLLMPromptEvalTimeP95    = "llm.prompt_eval_time.p95"
	MetricLLMSuccessRate          = "llm.success_rate"
	MetricLLMTokensPerOp              = "llm.tokens_per_op"
	MetricLLMScore                    = "llm.score"
	MetricLLMTokensPerSecond          = "llm.tokens_per_second"
	MetricLLMOutputTokensPerSecond    = "llm.output_tokens_per_second"
	MetricGPUUtilization              = "gpu.utilization"
	MetricGPUMemory                   = "gpu.memory"

	// Attribute keys - Metrics
	AttrModel   = "model"
	AttrCase    = "case"
	AttrTemp    = "temp"
	AttrTraceID = "trace_id"
	AttrSpanID  = "span_id"

	// Attribute keys - Spans (OpenTelemetry tracing)
	AttrSystemPrompt     = "system_prompt"
	AttrUserPrompt       = "user_prompt"
	AttrTemperature      = "temperature"
	AttrPromptTokens     = "prompt_tokens"
	AttrCompletionTokens = "completion_tokens"
	AttrTotalTokens      = "total_tokens"
	AttrLatencyMs        = "latency_ms"
	AttrPromptEvalTimeMs = "prompt_eval_time_ms"

	// Metric units
	UnitMilliseconds = "ms"
	UnitPercent      = "%"
	UnitMegabytes    = "MB"

	// Metric descriptions
	DescLLMLatency              = "Latency of LLM requests in milliseconds"
	DescLLMLatencyP50           = "50th percentile latency"
	DescLLMLatencyP95           = "95th percentile latency"
	DescLLMPromptEvalTime       = "Time for model to evaluate the prompt (time to first token) in milliseconds"
	DescLLMPromptEvalTimeP50    = "50th percentile prompt evaluation time"
	DescLLMPromptEvalTimeP95    = "95th percentile prompt evaluation time"
	DescLLMSuccessRate          = "Success rate of LLM requests"
	DescLLMTokensPerOp              = "Total tokens per operation"
	DescLLMScore                    = "Average score per operation"
	DescLLMTokensPerSecond          = "Total tokens per second (input + output / TAT)"
	DescLLMOutputTokensPerSecond    = "Output tokens per second (generation speed only)"
	DescGPUUtilization              = "GPU utilization percentage"
	DescGPUMemory                   = "GPU memory usage in MB"
)

// ToPrometheusMetricName converts an OpenTelemetry metric name to Prometheus format
// OpenTelemetry uses dots (.) but Prometheus converts them to underscores (_)
// Additionally, OTel appends unit suffixes to metric names based on the unit specified
func ToPrometheusMetricName(otelMetricName string) string {
	promName := strings.ReplaceAll(otelMetricName, ".", "_")

	// OTel Prometheus exporter appends unit suffixes to metric names
	// when a unit is specified via metric.WithUnit()
	// See: https://opentelemetry.io/docs/specs/otel/compatibility/prometheus_and_openmetrics/
	switch otelMetricName {
	case MetricLLMLatency:
		return promName + "_milliseconds"
	case MetricLLMLatencyP50:
		return promName + "_milliseconds"
	case MetricLLMLatencyP95:
		return promName + "_milliseconds"
	case MetricLLMPromptEvalTime:
		return promName + "_milliseconds"
	case MetricLLMPromptEvalTimeP50:
		return promName + "_milliseconds"
	case MetricLLMPromptEvalTimeP95:
		return promName + "_milliseconds"
	case MetricGPUMemory:
		return promName + "_MB" // OTel uses the literal unit string "MB"
	case MetricGPUUtilization:
		return promName + "_percent" // OTel converts "%" to "_percent"
	default:
		return promName
	}
}
