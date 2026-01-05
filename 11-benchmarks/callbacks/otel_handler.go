package callbacks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mdelapenya/genai-testcontainers-go/benchmarks/semconv"
	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// OTelCallbackHandler implements callbacks.Handler for OpenTelemetry tracing
type OTelCallbackHandler struct {
	tracer trace.Tracer
}

// NewOTelCallbackHandler creates a new OpenTelemetry callback handler
func NewOTelCallbackHandler() *OTelCallbackHandler {
	return &OTelCallbackHandler{
		tracer: otel.Tracer("langchaingo-callbacks"),
	}
}

// truncateString limits string length to prevent span explosion
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// HandleLLMGenerateContentStart is called when LLM generation starts
func (h *OTelCallbackHandler) HandleLLMGenerateContentStart(ctx context.Context, ms []llms.MessageContent) context.Context {
	ctx, _ = h.tracer.Start(ctx, "langchaingo.llm.generate.start",
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.Int(semconv.AttrLLMMessagesCount, len(ms)),
	)

	// Add message details (truncated to avoid excessive data)
	for i, msg := range ms {
		if i >= 3 { // Limit to first 3 messages
			break
		}
		// Extract role and content from MessageContent
		role := string(msg.Role)
		var content string
		for _, part := range msg.Parts {
			if textPart, ok := part.(llms.TextContent); ok {
				content += textPart.Text
			}
		}
		span.SetAttributes(
			attribute.String(fmt.Sprintf("llm.message.%d.role", i), role),
			attribute.String(fmt.Sprintf("llm.message.%d.content", i), truncateString(content, 500)),
		)
	}

	return ctx
}

// HandleLLMGenerateContentEnd is called when LLM generation completes
func (h *OTelCallbackHandler) HandleLLMGenerateContentEnd(ctx context.Context, res *llms.ContentResponse) context.Context {
	ctx, _ = h.tracer.Start(ctx, "langchaingo.llm.generate.end",
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	span := trace.SpanFromContext(ctx)

	// Add response details
	if len(res.Choices) > 0 {
		span.SetAttributes(
			attribute.Int(semconv.AttrLLMResponseChoices, len(res.Choices)),
			attribute.String(semconv.AttrLLMResponseContent, truncateString(res.Choices[0].Content, 500)),
		)
	}

	// Add token usage if available from GenerationInfo
	if len(res.Choices) > 0 && res.Choices[0].GenerationInfo != nil {
		genInfo := res.Choices[0].GenerationInfo
		if promptTokens, ok := genInfo["prompt_tokens"].(int); ok {
			span.SetAttributes(attribute.Int(semconv.AttrLLMUsagePromptTokens, promptTokens))
		}
		if completionTokens, ok := genInfo["completion_tokens"].(int); ok {
			span.SetAttributes(attribute.Int(semconv.AttrLLMUsageCompletionTokens, completionTokens))
		}
		if totalTokens, ok := genInfo["total_tokens"].(int); ok {
			span.SetAttributes(attribute.Int(semconv.AttrLLMUsageTotalTokens, totalTokens))
		}
	}

	return ctx
}

// HandleLLMError is called when LLM generation fails
func (h *OTelCallbackHandler) HandleLLMError(ctx context.Context, err error) context.Context {
	ctx, span := h.tracer.Start(ctx, "langchaingo.llm.error",
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	span.SetAttributes(
		attribute.String(semconv.AttrErrorType, fmt.Sprintf("%T", err)),
		attribute.String(semconv.AttrErrorMessage, truncateString(err.Error(), 500)),
	)
	span.SetStatus(codes.Error, err.Error())
	span.End()

	return ctx
}

// HandleToolStart is called when a tool execution starts
// This is KEY for tool calling observability
func (h *OTelCallbackHandler) HandleToolStart(ctx context.Context, input string) context.Context {
	startTime := time.Now()
	ctx = context.WithValue(ctx, "tool_start_time", startTime)

	ctx, _ = h.tracer.Start(ctx, "langchaingo.tool.start",
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	span := trace.SpanFromContext(ctx)

	// Try to parse tool input as JSON to extract tool name and parameters
	var toolInput map[string]interface{}
	if err := json.Unmarshal([]byte(input), &toolInput); err == nil {
		// Successfully parsed as JSON
		if toolName, ok := toolInput["tool"].(string); ok {
			span.SetAttributes(attribute.String(semconv.AttrToolName, toolName))
		} else if toolName, ok := toolInput["name"].(string); ok {
			span.SetAttributes(attribute.String(semconv.AttrToolName, toolName))
		}
	}

	span.SetAttributes(
		attribute.String(semconv.AttrToolInput, truncateString(input, 500)),
	)

	return ctx
}

// HandleToolEnd is called when a tool execution completes
// This is KEY for tool calling observability
func (h *OTelCallbackHandler) HandleToolEnd(ctx context.Context, output string) context.Context {
	ctx, _ = h.tracer.Start(ctx, "langchaingo.tool.end",
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	span := trace.SpanFromContext(ctx)

	// Calculate tool execution duration if start time is available
	if startTime, ok := ctx.Value("tool_start_time").(time.Time); ok {
		duration := time.Since(startTime)
		span.SetAttributes(
			attribute.Float64(semconv.AttrToolDurationMs, float64(duration.Milliseconds())),
		)
	}

	span.SetAttributes(
		attribute.String(semconv.AttrToolOutput, truncateString(output, 500)),
	)

	return ctx
}

// HandleToolError is called when a tool execution fails
// This is KEY for tool calling observability
func (h *OTelCallbackHandler) HandleToolError(ctx context.Context, err error) context.Context {
	ctx, span := h.tracer.Start(ctx, "langchaingo.tool.error",
		trace.WithSpanKind(trace.SpanKindInternal),
	)

	span.SetAttributes(
		attribute.String(semconv.AttrErrorType, fmt.Sprintf("%T", err)),
		attribute.String(semconv.AttrErrorMessage, truncateString(err.Error(), 500)),
	)
	span.SetStatus(codes.Error, err.Error())
	span.End()

	return ctx
}

// HandleStreamingFunc is called for streaming responses
// We skip this per user preference (no span per streaming chunk)
func (h *OTelCallbackHandler) HandleStreamingFunc(ctx context.Context, chunk []byte) context.Context {
	// No-op: Skip streaming chunks to avoid span explosion
	return ctx
}

// Stub methods for chains/agents/text/retriever (9 methods)
// These are required by the callbacks.Handler interface but not used in this implementation

func (h *OTelCallbackHandler) HandleChainStart(ctx context.Context, inputs map[string]any) context.Context {
	return ctx
}

func (h *OTelCallbackHandler) HandleChainEnd(ctx context.Context, outputs map[string]any) context.Context {
	return ctx
}

func (h *OTelCallbackHandler) HandleChainError(ctx context.Context, err error) context.Context {
	return ctx
}

func (h *OTelCallbackHandler) HandleAgentAction(ctx context.Context, action schema.AgentAction) context.Context {
	return ctx
}

func (h *OTelCallbackHandler) HandleAgentFinish(ctx context.Context, finish schema.AgentFinish) context.Context {
	return ctx
}

func (h *OTelCallbackHandler) HandleText(ctx context.Context, text string) context.Context {
	return ctx
}

func (h *OTelCallbackHandler) HandleRetrieverStart(ctx context.Context, query string) context.Context {
	return ctx
}

func (h *OTelCallbackHandler) HandleRetrieverEnd(ctx context.Context, documents []schema.Document) context.Context {
	return ctx
}

func (h *OTelCallbackHandler) HandleRetrieverError(ctx context.Context, err error) context.Context {
	return ctx
}

// Ensure OTelCallbackHandler implements callbacks.Handler
var _ callbacks.Handler = (*OTelCallbackHandler)(nil)
