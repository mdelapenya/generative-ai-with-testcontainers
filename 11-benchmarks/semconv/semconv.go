package semconv

import "strings"

// Semantic conventions for LLM benchmark metrics and attributes
// This file defines constants for metric names and attribute keys to ensure consistency

const (
	// Metric names - using dot notation which OpenTelemetry will convert to underscores in Prometheus
	MetricLLMLatency               = "llm.latency"
	MetricLLMLatencyP50            = "llm.latency.p50"
	MetricLLMLatencyP95            = "llm.latency.p95"
	MetricLLMTTFT                  = "llm.ttft"
	MetricLLMTTFTP50               = "llm.ttft.p50"
	MetricLLMTTFTP95               = "llm.ttft.p95"
	MetricLLMPromptEvalTime        = "llm.prompt_eval_time"
	MetricLLMPromptEvalTimeP50     = "llm.prompt_eval_time.p50"
	MetricLLMPromptEvalTimeP95     = "llm.prompt_eval_time.p95"
	MetricLLMSuccessRate           = "llm.success_rate"
	MetricLLMTokensPerOp           = "llm.tokens_per_op"
	MetricLLMEvalScore             = "llm.eval_score"
	MetricLLMEvalPassRate          = "llm.eval_pass_rate"
	MetricLLMTokensPerSecond       = "llm.tokens_per_second"
	MetricLLMOutputTokensPerSecond = "llm.output_tokens_per_second"
	MetricLLMNsPerOp               = "llm.ns_per_op"
	MetricGPUUtilization           = "gpu.utilization"
	MetricGPUMemory                = "gpu.memory"

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
	AttrTTFTMs           = "ttft_ms"
	AttrPromptEvalTimeMs = "prompt_eval_time_ms"

	// Metric units
	UnitMilliseconds = "ms"
	UnitPercent      = "%"
	UnitMegabytes    = "MB"

	// Metric descriptions
	DescLLMLatency               = "Total latency of LLM requests in seconds"
	DescLLMLatencyP50            = "50th percentile total latency in seconds"
	DescLLMLatencyP95            = "95th percentile total latency in seconds"
	DescLLMTTFT                  = "Time To First Token (measured via streaming) in seconds"
	DescLLMTTFTP50               = "50th percentile TTFT in seconds"
	DescLLMTTFTP95               = "95th percentile TTFT in seconds"
	DescLLMPromptEvalTime        = "Prompt evaluation time from model metadata in seconds"
	DescLLMPromptEvalTimeP50     = "50th percentile prompt evaluation time in seconds"
	DescLLMPromptEvalTimeP95     = "95th percentile prompt evaluation time in seconds"
	DescLLMSuccessRate           = "Success rate of LLM requests"
	DescLLMTokensPerOp           = "Total tokens per operation"
	DescLLMEvalScore             = "Average evaluator score (0.0-1.0) per operation"
	DescLLMEvalPassRate          = "Percentage of responses marked as 'yes' by evaluator"
	DescLLMTokensPerSecond       = "Total tokens per second (input + output / TAT)"
	DescLLMOutputTokensPerSecond = "Output tokens per second (generation speed only)"
	DescLLMNsPerOp               = "Nanoseconds per operation (Go benchmark metric)"
	DescGPUUtilization           = "GPU utilization percentage"
	DescGPUMemory                = "GPU memory usage in MB"
)

// ToPrometheusMetricName converts an OpenTelemetry metric name to Prometheus format
// OpenTelemetry uses dots (.) but Prometheus converts them to underscores (_)
// We're not using metric.WithUnit() to avoid automatic unit conversion by OTel,
// so we don't append unit suffixes here. Units are handled by Grafana dashboards.
func ToPrometheusMetricName(otelMetricName string) string {
	promName := strings.ReplaceAll(otelMetricName, ".", "_")

	// Special cases for GPU metrics that still use WithUnit()
	switch otelMetricName {
	case MetricGPUMemory:
		return promName + "_MB" // OTel uses the literal unit string "MB"
	case MetricGPUUtilization:
		return promName + "_percent" // OTel converts "%" to "_percent"
	default:
		return promName
	}
}

// Langchaingo Callback Spans - Attribute keys
const (
	// LLM lifecycle attributes
	AttrLLMMessagesCount         = "llm.messages.count"
	AttrLLMMessageRole           = "llm.message.role"
	AttrLLMMessageContent        = "llm.message.content"
	AttrLLMResponseContent       = "llm.response.content"
	AttrLLMResponseChoices       = "llm.response.choices.count"
	AttrLLMUsagePromptTokens     = "llm.usage.prompt_tokens"
	AttrLLMUsageCompletionTokens = "llm.usage.completion_tokens"
	AttrLLMUsageTotalTokens      = "llm.usage.total_tokens"

	// Tool calling attributes (KEY FOR TOOL OBSERVABILITY)
	AttrToolName       = "tool.name"
	AttrToolInput      = "tool.input"
	AttrToolOutput     = "tool.output"
	AttrToolDurationMs = "tool.duration_ms"
	AttrToolCallID     = "tool.call_id"

	// Error attributes
	AttrErrorType    = "error.type"
	AttrErrorMessage = "error.message"
)

// Additional metric names for tool calling
const (
	MetricLLMToolCallCount         = "llm.tool_call.count"
	MetricLLMToolCallLatency       = "llm.tool_call.latency"
	MetricLLMIterationCount        = "llm.iteration.count"
	MetricLLMToolSuccessRate       = "llm.tool.success_rate"
	MetricLLMToolParamAccuracy     = "llm.tool.param_accuracy"
	MetricLLMToolSelectionAccuracy = "llm.tool.selection_accuracy"
)
