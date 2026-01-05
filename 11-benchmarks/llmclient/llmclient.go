package llmclient

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mdelapenya/genai-testcontainers-go/benchmarks/callbacks"
	"github.com/mdelapenya/genai-testcontainers-go/benchmarks/semconv"
	"github.com/mdelapenya/genai-testcontainers-go/benchmarks/tools"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/trace"
)

// Client wraps an LLM client with observability
type Client struct {
	llm    llms.Model
	model  string
	tracer trace.Tracer
}

// Response contains the LLM response and metadata
type Response struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Latency          time.Duration
	PromptEvalTime   time.Duration // Time to evaluate prompt (from model metadata if available)
	TTFT             time.Duration // Time To First Token (actual measured via streaming)
}

// NewClient creates a new LLM client
func NewClient(endpoint, model string) (*Client, error) {
	// Determine if this is an external OpenAI API or local Docker Model Runner
	apiKey := "foo" // Default for Docker Model Runner
	if strings.Contains(endpoint, "api.openai.com") {
		// Use OpenAI API key for external API
		if key := os.Getenv("OPENAI_API_KEY"); key != "" {
			apiKey = key
		} else {
			return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required for OpenAI API endpoint")
		}
	}

	opts := []openai.Option{
		openai.WithBaseURL(endpoint),
		openai.WithModel(model),
		openai.WithToken(apiKey),
		openai.WithCallback(callbacks.NewOTelCallbackHandler()),
	}

	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create openai client: %w", err)
	}

	return &Client{
		llm:    llm,
		model:  model,
		tracer: otel.Tracer("llmclient"),
	}, nil
}

// GenerateWithTemp sends a prompt to the LLM with a specific temperature and returns the response with metadata
func (c *Client) GenerateWithTemp(ctx context.Context, testCase string, systemPrompt, userPrompt string, temperature float64) (*Response, error) {
	spanAttrs := []attribute.KeyValue{
		attribute.String(semconv.AttrModel, c.model),
		attribute.String(semconv.AttrSystemPrompt, systemPrompt),
		attribute.String(semconv.AttrUserPrompt, userPrompt),
		attribute.Float64(semconv.AttrTemperature, temperature),
	}
	if testCase != "" {
		spanAttrs = append(spanAttrs, attribute.String(semconv.AttrCase, testCase))
	}

	ctx, span := c.tracer.Start(ctx, "llm.generate",
		trace.WithAttributes(spanAttrs...),
	)
	defer span.End()

	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	start := time.Now()
	var ttft time.Duration
	firstTokenReceived := false
	var fullContent strings.Builder

	// Use streaming to capture real TTFT
	completion, err := c.llm.GenerateContent(ctx, content,
		llms.WithTemperature(temperature),
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			if !firstTokenReceived {
				ttft = time.Since(start)
				firstTokenReceived = true
			}
			fullContent.Write(chunk)
			return nil
		}),
	)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("generate content: %w", err)
	}

	latency := time.Since(start)

	// Get content from streaming or from response
	responseContent := fullContent.String()
	if responseContent == "" && len(completion.Choices) > 0 {
		// Fallback to non-streaming response if streaming didn't work
		responseContent = completion.Choices[0].Content
	}

	if responseContent == "" {
		return nil, fmt.Errorf("no content returned from model")
	}

	// Extract token usage from GenerationInfo if available
	promptTokens := 0
	completionTokens := 0
	totalTokens := 0
	promptEvalTime := time.Duration(0)

	if len(completion.Choices) > 0 && completion.Choices[0].GenerationInfo != nil {
		genInfo := completion.Choices[0].GenerationInfo
		if pt, ok := genInfo["PromptTokens"].(int); ok {
			promptTokens = pt
		} else {
			promptTokens = llms.CountTokens(c.model, systemPrompt+userPrompt)
		}
		if ct, ok := genInfo["CompletionTokens"].(int); ok {
			completionTokens = ct
		} else {
			completionTokens = llms.CountTokens(c.model, responseContent)
		}
		if tt, ok := genInfo["TotalTokens"].(int); ok {
			totalTokens = tt
		} else {
			totalTokens = promptTokens + completionTokens
		}

		// Try to extract prompt evaluation time from GenerationInfo
		// Some models provide this as "prompt_eval_duration" (in nanoseconds) or similar fields
		if evalDuration, ok := genInfo["prompt_eval_duration"].(int64); ok {
			promptEvalTime = time.Duration(evalDuration) * time.Nanosecond
		} else if evalDuration, ok := genInfo["prompt_eval_duration"].(float64); ok {
			promptEvalTime = time.Duration(evalDuration) * time.Nanosecond
		}
	}

	// Fallback to estimation if token counts not provided by model
	if totalTokens == 0 {
		promptTokens = estimateTokens(systemPrompt + userPrompt)
		completionTokens = estimateTokens(responseContent)
		totalTokens = promptTokens + completionTokens
	}

	// Estimate prompt eval time if not provided by model metadata
	// This is different from TTFT - it's the model's internal prompt processing time
	// Typical models process prompts at ~100-500 tokens/sec for evaluation
	// We'll use a conservative estimate of 200 tokens/sec
	if promptEvalTime == 0 && promptTokens > 0 {
		promptEvalTime = time.Duration(float64(promptTokens)/200.0*1000) * time.Millisecond
	}

	// If TTFT wasn't captured (streaming might not be supported), use latency as fallback
	if ttft == 0 {
		ttft = latency
	}

	resp := &Response{
		Content:          responseContent,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		Latency:          latency,
		PromptEvalTime:   promptEvalTime,
		TTFT:             ttft,
	}

	// Add response metadata to span
	span.SetAttributes(
		attribute.Int(semconv.AttrPromptTokens, resp.PromptTokens),
		attribute.Int(semconv.AttrCompletionTokens, resp.CompletionTokens),
		attribute.Int(semconv.AttrTotalTokens, resp.TotalTokens),
		attribute.Int64(semconv.AttrLatencyMs, latency.Milliseconds()),
		attribute.Int64(semconv.AttrPromptEvalTimeMs, promptEvalTime.Milliseconds()),
		attribute.Int64(semconv.AttrTTFTMs, ttft.Milliseconds()),
	)

	// Log the model response
	logger := global.GetLoggerProvider().Logger("llmclient")
	var record log.Record
	record.SetSeverity(log.SeverityInfo)
	record.SetBody(log.StringValue("Model response"))

	logAttrs := []log.KeyValue{
		log.String("model", sanitizeUTF8(c.model)),
		log.String("system_prompt", truncateString(systemPrompt, 100)),
		log.String("user_prompt", truncateString(userPrompt, 200)),
		log.Float64("temperature", temperature),
		log.String("response_content", truncateString(responseContent, 500)),
		log.Int("prompt_tokens", resp.PromptTokens),
		log.Int("completion_tokens", resp.CompletionTokens),
		log.Int("total_tokens", resp.TotalTokens),
		log.Int64("latency_ms", latency.Milliseconds()),
		log.Int64("ttft_ms", ttft.Milliseconds()),
	}
	if testCase != "" {
		logAttrs = append(logAttrs, log.String("test_case", sanitizeUTF8(testCase)))
	}

	record.AddAttributes(logAttrs...)
	logger.Emit(ctx, record)

	return resp, nil
}

// truncateString truncates a string to a maximum length and ensures valid UTF-8
func truncateString(s string, maxLen int) string {
	// First, sanitize to valid UTF-8
	s = sanitizeUTF8(s)

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// sanitizeUTF8 replaces invalid UTF-8 sequences with the replacement character
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	// Replace invalid UTF-8 with valid replacement characters
	return strings.ToValidUTF8(s, "�")
}

// estimateTokens provides a rough estimate of token count based on character count.
// This is used as a last-resort fallback when neither GenerationInfo nor llms.CountTokens provides token counts.
// Preference order: 1) GenerationInfo, 2) llms.CountTokens, 3) estimateTokens
func estimateTokens(text string) int {
	// Rough approximation: 1 token ≈ 4 characters for English text
	return len(text) / 4
}

// ToolResult contains information about a tool call execution
type ToolResult struct {
	ToolName string
	Input    string
	Output   string
	Duration time.Duration
	Error    error
}

// ResponseWithTools contains the LLM response and metadata including tool execution info
type ResponseWithTools struct {
	*Response                // Embed the base Response
	ToolCalls       []ToolResult
	Iterations      int           // Number of LLM-tool roundtrips
	TotalLatency    time.Duration // Total time including all tool executions
	LLMLatency      time.Duration // LLM inference time only
	ToolLatency     time.Duration // Tool execution time only
	FinalContent    string        // Final synthesized response
}

// GenerateWithTools sends a prompt to the LLM with tools and iteratively executes tool calls
// until the model provides a final answer or reaches maxIterations
func (c *Client) GenerateWithTools(ctx context.Context, testCase string, systemPrompt, userPrompt string, temperature float64, tools []llms.Tool, maxIterations int) (*ResponseWithTools, error) {
	spanAttrs := []attribute.KeyValue{
		attribute.String(semconv.AttrModel, c.model),
		attribute.String(semconv.AttrSystemPrompt, systemPrompt),
		attribute.String(semconv.AttrUserPrompt, userPrompt),
		attribute.Float64(semconv.AttrTemperature, temperature),
	}
	if testCase != "" {
		spanAttrs = append(spanAttrs, attribute.String(semconv.AttrCase, testCase))
	}

	ctx, span := c.tracer.Start(ctx, "llm.generate",
		trace.WithAttributes(spanAttrs...),
	)
	defer span.End()

	totalStart := time.Now()
	var llmLatency, toolLatency time.Duration
	var toolResults []ToolResult
	iterations := 0

	// Build initial message history
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	// Iterative loop for tool calling
	for iterations < maxIterations {
		iterations++
		llmStart := time.Now()

		// Generate content with tools
		completion, err := c.llm.GenerateContent(ctx, messages,
			llms.WithTemperature(temperature),
			llms.WithTools(tools),
		)
		if err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("generate content with tools: %w", err)
		}

		llmLatency += time.Since(llmStart)

		// Check if there are tool calls in the response
		if len(completion.Choices) == 0 {
			return nil, fmt.Errorf("no response choices returned from model")
		}

		choice := completion.Choices[0]

		// If there are no tool calls, we have the final answer
		if len(choice.ToolCalls) == 0 {
			// Extract final content
			finalContent := choice.Content

			// Build response with tool metadata
			totalLatency := time.Since(totalStart)

			// Get token usage from first LLM call (approximation)
			promptTokens := 0
			completionTokens := 0
			totalTokens := 0

			if choice.GenerationInfo != nil {
				if pt, ok := choice.GenerationInfo["PromptTokens"].(int); ok {
					promptTokens = pt
				}
				if ct, ok := choice.GenerationInfo["CompletionTokens"].(int); ok {
					completionTokens = ct
				}
				if tt, ok := choice.GenerationInfo["TotalTokens"].(int); ok {
					totalTokens = tt
				}
			}

			if totalTokens == 0 {
				promptTokens = estimateTokens(systemPrompt + userPrompt)
				completionTokens = estimateTokens(finalContent)
				totalTokens = promptTokens + completionTokens
			}

			// Add response metadata to span
			span.SetAttributes(
				attribute.Int(semconv.AttrPromptTokens, promptTokens),
				attribute.Int(semconv.AttrCompletionTokens, completionTokens),
				attribute.Int(semconv.AttrTotalTokens, totalTokens),
				attribute.Int64(semconv.AttrLatencyMs, totalLatency.Milliseconds()),
				attribute.Int("tool_call_count", len(toolResults)),
				attribute.Int("iterations", iterations),
			)

			return &ResponseWithTools{
				Response: &Response{
					Content:          finalContent,
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      totalTokens,
					Latency:          totalLatency,
				},
				ToolCalls:    toolResults,
				Iterations:   iterations,
				TotalLatency: totalLatency,
				LLMLatency:   llmLatency,
				ToolLatency:  toolLatency,
				FinalContent: finalContent,
			}, nil
		}

		// Execute tool calls
		for _, toolCall := range choice.ToolCalls {
			toolStart := time.Now()

			// Execute the tool
			output, err := executeToolCall(ctx, toolCall)

			toolDuration := time.Since(toolStart)
			toolLatency += toolDuration

			// Record tool execution result
			toolResult := ToolResult{
				ToolName: toolCall.FunctionCall.Name,
				Input:    toolCall.FunctionCall.Arguments,
				Output:   output,
				Duration: toolDuration,
				Error:    err,
			}
			toolResults = append(toolResults, toolResult)

			// Build tool response message
			messages = append(messages, llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: toolCall.ID,
						Name:       toolCall.FunctionCall.Name,
						Content:    output,
					},
				},
			})
		}
	}

	// If we reach maxIterations without a final answer, return what we have
	totalLatency := time.Since(totalStart)
	span.SetAttributes(
		attribute.Int("tool_call_count", len(toolResults)),
		attribute.Int("iterations", iterations),
		attribute.Int64(semconv.AttrLatencyMs, totalLatency.Milliseconds()),
	)

	return &ResponseWithTools{
		Response: &Response{
			Content:  "Maximum iterations reached without final answer",
			Latency:  totalLatency,
		},
		ToolCalls:    toolResults,
		Iterations:   iterations,
		TotalLatency: totalLatency,
		LLMLatency:   llmLatency,
		ToolLatency:  toolLatency,
		FinalContent: "Maximum iterations reached without final answer",
	}, fmt.Errorf("maximum iterations (%d) reached without final answer", maxIterations)
}

// executeToolCall routes a tool call to the appropriate tool implementation
func executeToolCall(ctx context.Context, toolCall llms.ToolCall) (string, error) {
	switch toolCall.FunctionCall.Name {
	case "calculator":
		calc := tools.NewCalculator()
		return calc.Execute(toolCall.FunctionCall.Arguments)

	case "execute_python":
		executor := tools.NewCodeExecutor()
		return executor.Execute(toolCall.FunctionCall.Arguments)

	case "http_get":
		httpClient := tools.NewHTTPClient()
		return httpClient.Execute(toolCall.FunctionCall.Arguments)

	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.FunctionCall.Name)
	}
}
