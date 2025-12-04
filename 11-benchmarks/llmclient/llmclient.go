package llmclient

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Semantic convention constants for span attributes
// These match the parent package's semconv.go to ensure consistency
const (
	AttrModel            = "model"
	AttrSystemPrompt     = "system_prompt"
	AttrUserPrompt       = "user_prompt"
	AttrTemperature      = "temperature"
	AttrPromptTokens     = "prompt_tokens"
	AttrCompletionTokens = "completion_tokens"
	AttrTotalTokens      = "total_tokens"
	AttrLatencyMs        = "latency_ms"
	AttrPromptEvalTimeMs = "prompt_eval_time_ms"
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
	PromptEvalTime   time.Duration // Time to evaluate prompt (time to first token)
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

// Generate sends a prompt to the LLM and returns the response with metadata
func (c *Client) Generate(ctx context.Context, systemPrompt, userPrompt string) (*Response, error) {
	return c.GenerateWithTemp(ctx, systemPrompt, userPrompt, 0.7)
}

// GenerateWithTemp sends a prompt to the LLM with a specific temperature and returns the response with metadata
func (c *Client) GenerateWithTemp(ctx context.Context, systemPrompt, userPrompt string, temperature float64) (*Response, error) {
	ctx, span := c.tracer.Start(ctx, "llm.generate",
		trace.WithAttributes(
			attribute.String(AttrModel, c.model),
			attribute.String(AttrSystemPrompt, systemPrompt),
			attribute.String(AttrUserPrompt, userPrompt),
			attribute.Float64(AttrTemperature, temperature),
		),
	)
	defer span.End()

	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, userPrompt),
	}

	start := time.Now()

	completion, err := c.llm.GenerateContent(ctx, content,
		llms.WithTemperature(temperature),
	)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("generate content: %w", err)
	}

	latency := time.Since(start)

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from model")
	}

	// Extract token usage from GenerationInfo if available
	promptTokens := 0
	completionTokens := 0
	totalTokens := 0
	promptEvalTime := time.Duration(0)

	if genInfo := completion.Choices[0].GenerationInfo; genInfo != nil {
		if pt, ok := genInfo["PromptTokens"].(int); ok {
			promptTokens = pt
		} else {
			promptTokens = llms.CountTokens(c.model, systemPrompt+userPrompt)
		}
		if ct, ok := genInfo["CompletionTokens"].(int); ok {
			completionTokens = ct
		} else {
			completionTokens = llms.CountTokens(c.model, completion.Choices[0].Content)
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
		completionTokens = estimateTokens(completion.Choices[0].Content)
		totalTokens = promptTokens + completionTokens
	}

	// Estimate prompt eval time if not provided
	// Typical models process prompts at ~100-500 tokens/sec for evaluation
	// We'll use a conservative estimate of 200 tokens/sec
	if promptEvalTime == 0 && promptTokens > 0 {
		promptEvalTime = time.Duration(float64(promptTokens)/200.0*1000) * time.Millisecond
	}

	resp := &Response{
		Content:          completion.Choices[0].Content,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		Latency:          latency,
		PromptEvalTime:   promptEvalTime,
	}

	// Add response metadata to span
	span.SetAttributes(
		attribute.Int(AttrPromptTokens, resp.PromptTokens),
		attribute.Int(AttrCompletionTokens, resp.CompletionTokens),
		attribute.Int(AttrTotalTokens, resp.TotalTokens),
		attribute.Int64(AttrLatencyMs, latency.Milliseconds()),
		attribute.Int64(AttrPromptEvalTimeMs, promptEvalTime.Milliseconds()),
	)

	return resp, nil
}

// estimateTokens provides a rough estimate of token count based on character count.
// This is used as a last-resort fallback when neither GenerationInfo nor llms.CountTokens provides token counts.
// Preference order: 1) GenerationInfo, 2) llms.CountTokens, 3) estimateTokens
func estimateTokens(text string) int {
	// Rough approximation: 1 token ≈ 4 characters for English text
	return len(text) / 4
}
