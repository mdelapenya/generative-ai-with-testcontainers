package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/mdelapenya/genai-testcontainers-go/benchmarks/evaluator"
	"github.com/mdelapenya/genai-testcontainers-go/benchmarks/llmclient"
	dmr "github.com/testcontainers/testcontainers-go/modules/dockermodelrunner"
)

//go:embed testdata/fibonacci.go
var fibonacciCode string

// getDMRContainer returns the typed DMR container
func getDMRContainer() *dmr.Container {
	return dmrContainer.(*dmr.Container)
}

// ModelConfig defines a model to benchmark
type ModelConfig struct {
	Namespace   string
	Name        string
	Tag         string
	FQName      string
	IsExternal  bool   // True if using external API (not Docker Model Runner)
	ExternalURL string // External API endpoint (e.g., https://api.openai.com/v1)
}

// TestCase defines a prompt evaluation test case
type TestCase struct {
	Name         string
	SystemPrompt string
	UserPrompt   string
}

var (
	// Models to benchmark
	models []ModelConfig

	// Test cases for evaluation (without temperature)
	testCases = []TestCase{
		{
			Name:         "code-explanation",
			SystemPrompt: "You are a helpful coding assistant.",
			UserPrompt:   "Explain what this Go code does:\n\n```go\n" + fibonacciCode + "\n```",
		},
		{
			Name:         "mathematical-operations",
			SystemPrompt: "You are a mathematics expert.",
			UserPrompt:   "What is the result of sum of all numbers between 1 and 100, both inclusive?",
		},
		{
			Name:         "creative-writing",
			SystemPrompt: "You are a creative writer with a great sense of humor.",
			UserPrompt:   "Write an hilarious joke about the Fibonacci sequence.",
		},
		{
			Name:         "factual-question",
			SystemPrompt: "You are a knowledgeable history expert.",
			UserPrompt:   "What was the significance of Toledo, Spain during the medieval period, particularly regarding the translation movement?",
		},
		{
			Name:         "code-generation",
			SystemPrompt: "You are a Go programming expert.",
			UserPrompt:   "Write a Go function that calculates the Fibonacci sequence using recursion.",
		},
		// Tool-assisted test cases
		{
			Name:         "calculator-reasoning",
			SystemPrompt: "You are a helpful assistant with access to a calculator tool. Use the calculator for all arithmetic operations.",
			UserPrompt:   "Calculate (125 * 47) + (980 / 20) - 156. Break down each step and use the calculator tool for each operation. Then explain the final result.",
		},
		{
			Name:         "code-validation",
			SystemPrompt: "You are a helpful coding assistant with access to a Python code executor. Always execute code to verify correctness.",
			UserPrompt:   "Write Python code to generate the first 10 Fibonacci numbers, then execute it to verify correctness.",
		},
		{
			Name:         "api-data-retrieval",
			SystemPrompt: "You are a helpful assistant with access to web APIs. Use the HTTP client to fetch real-time data.",
			UserPrompt:   "Use the HTTP client to fetch information about repository 'testcontainers-go' from GitHub API (https://api.github.com/repos/testcontainers/testcontainers-go) and summarize the key details.",
		},
	}

	// Temperatures to test with each test case
	temperatures = []float64{0.1, 0.3, 0.5, 0.7, 0.9}
)

// getModelsToTest returns the list of models to benchmark
// If OPENAI_API_KEY is set, it includes OpenAI models at the beginning
func getModelsToTest() []ModelConfig {
	var allModels []ModelConfig

	// Check if OpenAI API key is available and add OpenAI model first
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		fmt.Println("🔑 OpenAI API key detected - adding OpenAI models to benchmark suite")
		allModels = append(allModels, ModelConfig{
			Namespace:   "openai",
			Name:        "gpt-5.1",
			Tag:         "",
			FQName:      "gpt-5.1",
			IsExternal:  true,
			ExternalURL: "https://api.openai.com/v1",
		})
	} else {
		fmt.Println("ℹ️  No OPENAI_API_KEY found - skipping OpenAI models (set OPENAI_API_KEY to include OpenAI models)")
	}

	// Add local models after OpenAI
	localModels := []ModelConfig{
		{
			Namespace: "ai",
			Name:      "llama3.2",
			Tag:       "1B-Q4_0",
			FQName:    "ai/llama3.2:1B-Q4_0",
		},
		{
			Namespace: "ai",
			Name:      "llama3.2",
			Tag:       "3B-Q4_K_M",
			FQName:    "ai/llama3.2:3B-Q4_K_M",
		},
		{
			Namespace: "ai",
			Name:      "qwen3",
			Tag:       "0.6B-Q4_0",
			FQName:    "ai/qwen3:0.6B-Q4_0",
		},
		{
			Namespace: "hf.co/bartowski",
			Name:      "Llama-3.2-1B-Instruct-GGUF",
			Tag:       "",
			FQName:    "hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF",
		},
	}

	allModels = append(allModels, localModels...)
	return allModels
}

// BenchmarkResult stores benchmark results for a single test
type BenchmarkResult struct {
	Model            string
	TestCase         string
	Temp             float64
	Latency          time.Duration // Total turnaround time (TAT)
	TTFT             time.Duration // Time To First Token (measured via streaming)
	PromptEvalTime   time.Duration // Time to evaluate prompt (from model metadata if available)
	PromptTokens     int           // Input tokens
	CompletionTokens int           // Output tokens generated
	TotalTokens      int           // Total tokens (prompt + completion)
	Success          bool
	EvalScore        float64 // Score from evaluator agent (0.0-1.0)
	EvalResponse     string  // "yes", "no", or "unsure"
	EvalReason       string  // Reasoning from evaluator
	ResponseContent  string  // The actual LLM response content
}

// BenchmarkLLMs runs benchmarks for all models and test cases
func BenchmarkLLMs(b *testing.B) {
	ctx := context.Background()

	for _, model := range models {
		modelName := model.FQName

		// Only pull models that are not external APIs
		if !model.IsExternal {
			// Pull the model before benchmarking
			b.Run(fmt.Sprintf("Pull/%s", model.Name), func(b *testing.B) {
				b.ResetTimer()
				if err := getDMRContainer().PullModel(ctx, modelName); err != nil {
					b.Fatalf("Failed to pull model %s: %v", modelName, err)
				}
			})
		}

		// Create client for this model
		var endpoint string
		if model.IsExternal {
			endpoint = model.ExternalURL
		} else {
			endpoint = getDMRContainer().OpenAIEndpoint()
		}

		client, err := llmclient.NewClient(endpoint, modelName)
		if err != nil {
			b.Fatalf("Failed to create client for %s: %v", modelName, err)
		}

		// Benchmark each test case with each temperature
		for _, tc := range testCases {
			for _, temp := range temperatures {
				benchName := fmt.Sprintf("%s/%s/temp%.1f", model.Name, tc.Name, temp)

				b.Run(benchName, func(b *testing.B) {
					results := make([]BenchmarkResult, 0, b.N)

					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						var result BenchmarkResult
						// Route to appropriate function based on test case type
						if isToolAssistedCase(tc.Name) {
							result = runSingleBenchmarkWithTools(ctx, client, modelName, tc, temp)
						} else {
							result = runSingleBenchmark(ctx, client, modelName, tc, temp)
						}
						results = append(results, result)

						// Record latency with OpenTelemetry
						metricsCollector.RecordLatency(ctx, result.Latency, modelName, tc.Name, temp)

						// Record TTFT with OpenTelemetry
						if result.TTFT > 0 {
							metricsCollector.RecordTTFT(ctx, result.TTFT, modelName, tc.Name, temp)
						}

						// Record prompt evaluation time with OpenTelemetry
						if result.PromptEvalTime > 0 {
							metricsCollector.RecordPromptEvalTime(ctx, result.PromptEvalTime, modelName, tc.Name, temp)
						}

						if result.Success {
							metricsCollector.IncrementSuccess()
						}

						// Sample GPU metrics periodically
						if i%5 == 0 {
							gpuMetrics, _ := SampleGPU()
							if gpuMetrics != nil && gpuMetrics.Available {
								metricsCollector.UpdateGPUMetrics(gpuMetrics.Utilization, gpuMetrics.MemoryUsed)
							}
						}
					}
					b.StopTimer()

					// Calculate and report aggregate metrics
					reportAggregateMetrics(b, results)

					// Calculate ns/op from Go benchmark framework
					nsPerOp := float64(b.Elapsed().Nanoseconds()) / float64(b.N)

					// Update OpenTelemetry gauges with model/case/temp labels
					updateGauges(modelName, tc.Name, temp, results, nsPerOp)
				})
			}
		}
	}
}

// runSingleBenchmark executes a single benchmark iteration
func runSingleBenchmark(ctx context.Context, client *llmclient.Client, model string, tc TestCase, temp float64) BenchmarkResult {
	resp, err := client.GenerateWithTemp(ctx, tc.Name, tc.SystemPrompt, tc.UserPrompt, temp)

	result := BenchmarkResult{
		Model:    model,
		TestCase: tc.Name,
		Temp:     temp,
		Success:  err == nil,
	}

	if err == nil {
		result.Latency = resp.Latency
		result.TTFT = resp.TTFT
		result.PromptEvalTime = resp.PromptEvalTime
		result.PromptTokens = resp.PromptTokens
		result.CompletionTokens = resp.CompletionTokens
		result.TotalTokens = resp.TotalTokens
		result.ResponseContent = resp.Content

		// Evaluate the response using the evaluator agent
		if evaluatorAgent != nil {
			evalResult, evalErr := evaluateResponse(ctx, tc.Name, tc.UserPrompt, resp.Content)
			if evalErr == nil {
				result.EvalScore = evalResult.Score
				result.EvalResponse = evalResult.Response
				result.EvalReason = evalResult.Reason
			} else {
				fmt.Printf("⚠️  Evaluation error for %s/%s/temp%.1f: %v\n", model, tc.Name, temp, evalErr)
			}
		}
	} else {
		// Log error for debugging (will appear in benchmark output)
		fmt.Printf("❌ Error in %s/%s/temp%.1f: %v\n", model, tc.Name, temp, err)
	}

	return result
}

// isToolAssistedCase checks if a test case requires tool calling
func isToolAssistedCase(name string) bool {
	toolCases := []string{"calculator-reasoning", "code-validation", "api-data-retrieval"}
	for _, tc := range toolCases {
		if tc == name {
			return true
		}
	}
	return false
}

// getToolsForCase returns the tools available for a specific test case
func getToolsForCase(name string) []interface{} {
	switch name {
	case "calculator-reasoning":
		return []interface{}{llmclient.GetCalculatorTool()}
	case "code-validation":
		return []interface{}{llmclient.GetCodeExecutorTool()}
	case "api-data-retrieval":
		return []interface{}{llmclient.GetHTTPClientTool()}
	default:
		return nil
	}
}

// runSingleBenchmarkWithTools executes a single benchmark iteration with tool calling
func runSingleBenchmarkWithTools(ctx context.Context, client *llmclient.Client, model string, tc TestCase, temp float64) BenchmarkResult {
	tools := getToolsForCase(tc.Name)
	maxIterations := 10 // Maximum LLM-tool iterations

	resp, err := client.GenerateWithTools(ctx, tc.Name, tc.SystemPrompt, tc.UserPrompt, temp, tools, maxIterations)

	result := BenchmarkResult{
		Model:    model,
		TestCase: tc.Name,
		Temp:     temp,
		Success:  err == nil,
	}

	if err == nil {
		result.Latency = resp.Latency
		result.TTFT = resp.TTFT
		result.PromptEvalTime = resp.PromptEvalTime
		result.PromptTokens = resp.PromptTokens
		result.CompletionTokens = resp.CompletionTokens
		result.TotalTokens = resp.TotalTokens
		result.ResponseContent = resp.Content

		// Evaluate the response using the evaluator agent
		if evaluatorAgent != nil {
			evalResult, evalErr := evaluateResponse(ctx, tc.Name, tc.UserPrompt, resp.Content)
			if evalErr == nil {
				result.EvalScore = evalResult.Score
				result.EvalResponse = evalResult.Response
				result.EvalReason = evalResult.Reason
			} else {
				fmt.Printf("⚠️  Evaluation error for %s/%s/temp%.1f: %v\n", model, tc.Name, temp, evalErr)
			}
		}
	} else {
		// Log error for debugging (will appear in benchmark output)
		fmt.Printf("❌ Error in %s/%s/temp%.1f: %v\n", model, tc.Name, temp, err)
	}

	return result
}

// evaluateResponse uses the evaluator agent to assess response quality
func evaluateResponse(ctx context.Context, testCaseName string, question string, answer string) (*evaluator.EvaluationResult, error) {
	criteria := evaluator.GetCriteria()
	evalCriteria, ok := criteria[testCaseName]
	if !ok {
		return nil, fmt.Errorf("no evaluation criteria found for test case: %s", testCaseName)
	}

	// Create evaluator agent with test-case-specific system prompt
	agent := evaluator.NewAgent(evaluatorAgent, evalCriteria.SystemPrompt)

	// Evaluate the response
	return agent.Evaluate(ctx, testCaseName, question, answer, evalCriteria.Reference)
}

// reportAggregateMetrics calculates and reports aggregate metrics
func reportAggregateMetrics(b *testing.B, results []BenchmarkResult) {
	if len(results) == 0 {
		return
	}

	// Calculate latency percentiles
	latencies := make([]float64, 0, len(results))
	ttfts := make([]float64, 0, len(results))
	promptEvalTimes := make([]float64, 0, len(results))
	totalPromptTokens := 0
	totalCompletionTokens := 0
	totalTurnaroundTimeMs := 0.0
	totalGenerationTimeMs := 0.0
	successCount := 0
	totalEvalScore := 0.0
	evalCount := 0
	evalYesCount := 0
	evalNoCount := 0
	evalUnsureCount := 0

	for _, r := range results {
		if r.Success {
			// Store in milliseconds to match histogram metrics
			latencies = append(latencies, float64(r.Latency.Milliseconds()))
			if r.TTFT > 0 {
				ttfts = append(ttfts, float64(r.TTFT.Milliseconds()))
			}
			if r.PromptEvalTime > 0 {
				promptEvalTimes = append(promptEvalTimes, float64(r.PromptEvalTime.Milliseconds()))
			}
			totalPromptTokens += r.PromptTokens
			totalCompletionTokens += r.CompletionTokens
			totalTurnaroundTimeMs += float64(r.Latency.Milliseconds())

			// Generation time = Total time - TTFT (more accurate than using PromptEvalTime)
			generationTime := r.Latency - r.TTFT
			if generationTime > 0 {
				totalGenerationTimeMs += float64(generationTime.Milliseconds())
			}

			successCount++

			// Track evaluation scores if available
			if r.EvalResponse != "" {
				totalEvalScore += r.EvalScore
				evalCount++

				// Count response types
				switch r.EvalResponse {
				case "yes":
					evalYesCount++
				case "no":
					evalNoCount++
				case "unsure":
					evalUnsureCount++
				}
			}
		}
	}

	if len(latencies) == 0 {
		// No successful results - report zeros for metrics except success_rate
		successRate := float64(successCount) / float64(len(results))
		b.ReportMetric(0, "latency_p50_ms")
		b.ReportMetric(0, "latency_p95_ms")
		b.ReportMetric(0, "prompt_eval_p50_ms")
		b.ReportMetric(0, "prompt_eval_p95_ms")
		b.ReportMetric(0, "tokens_per_op")
		b.ReportMetric(successRate, "success_rate")
		b.ReportMetric(0, "eval_score")
		b.ReportMetric(0, "eval_pass_rate")
		b.ReportMetric(0, "tokens_per_sec")
		b.ReportMetric(0, "output_tokens_per_sec")
		return
	}

	sort.Float64s(latencies)

	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)

	// Calculate TTFT percentiles
	ttftP50 := 0.0
	ttftP95 := 0.0
	if len(ttfts) > 0 {
		sort.Float64s(ttfts)
		ttftP50 = percentile(ttfts, 50)
		ttftP95 = percentile(ttfts, 95)
	}

	// Calculate prompt eval time percentiles
	promptEvalP50 := 0.0
	promptEvalP95 := 0.0
	if len(promptEvalTimes) > 0 {
		sort.Float64s(promptEvalTimes)
		promptEvalP50 = percentile(promptEvalTimes, 50)
		promptEvalP95 = percentile(promptEvalTimes, 95)
	}

	avgTotalTokens := float64(totalPromptTokens+totalCompletionTokens) / float64(successCount)
	successRate := float64(successCount) / float64(len(results))

	// Calculate evaluator metrics
	avgEvalScore := 0.0
	evalPassRate := 0.0
	if evalCount > 0 {
		avgEvalScore = totalEvalScore / float64(evalCount)
		evalPassRate = float64(evalYesCount) / float64(evalCount)
	}

	// Calculate TPS = (Input Tokens + Output Tokens) / Total Turnaround Time (TAT in seconds)
	// This represents average TPS accounting for both input and output tokens
	avgTurnaroundTimeSec := (totalTurnaroundTimeMs / float64(successCount)) / 1000.0
	tokensPerSec := 0.0
	if avgTurnaroundTimeSec > 0 {
		tokensPerSec = avgTotalTokens / avgTurnaroundTimeSec
	}

	// Calculate Output TPS = Output Tokens / Time to Generate Output Tokens
	// This specifically measures generation speed, excluding input processing
	avgGenerationTimeSec := (totalGenerationTimeMs / float64(successCount)) / 1000.0
	avgOutputTokens := float64(totalCompletionTokens) / float64(successCount)
	outputTokensPerSec := 0.0
	if avgGenerationTimeSec > 0 {
		outputTokensPerSec = avgOutputTokens / avgGenerationTimeSec
	}

	// Report custom metrics in milliseconds
	b.ReportMetric(p50, "latency_p50_ms")
	b.ReportMetric(p95, "latency_p95_ms")
	b.ReportMetric(ttftP50, "ttft_p50_ms")
	b.ReportMetric(ttftP95, "ttft_p95_ms")
	b.ReportMetric(promptEvalP50, "prompt_eval_p50_ms")
	b.ReportMetric(promptEvalP95, "prompt_eval_p95_ms")
	b.ReportMetric(avgTotalTokens, "tokens_per_op")
	b.ReportMetric(successRate, "success_rate")
	b.ReportMetric(avgEvalScore, "eval_score")
	b.ReportMetric(evalPassRate, "eval_pass_rate")
	b.ReportMetric(tokensPerSec, "tokens_per_sec")
	b.ReportMetric(outputTokensPerSec, "output_tokens_per_sec")
}

// updateGauges updates OpenTelemetry gauge metrics with model/case/temp labels
func updateGauges(model, testCase string, temp float64, results []BenchmarkResult, nsPerOp float64) {
	if len(results) == 0 {
		return
	}

	latencies := make([]float64, 0, len(results))
	ttfts := make([]float64, 0, len(results))
	promptEvalTimes := make([]float64, 0, len(results))
	totalPromptTokens := 0
	totalCompletionTokens := 0
	totalTurnaroundTimeMs := 0.0
	totalGenerationTimeMs := 0.0
	successCount := 0
	totalEvalScore := 0.0
	evalCount := 0
	evalYesCount := 0

	for _, r := range results {
		if r.Success {
			// Store in milliseconds to match histogram metrics
			latencies = append(latencies, float64(r.Latency.Milliseconds()))
			if r.TTFT > 0 {
				ttfts = append(ttfts, float64(r.TTFT.Milliseconds()))
			}
			if r.PromptEvalTime > 0 {
				promptEvalTimes = append(promptEvalTimes, float64(r.PromptEvalTime.Milliseconds()))
			}
			totalPromptTokens += r.PromptTokens
			totalCompletionTokens += r.CompletionTokens
			totalTurnaroundTimeMs += float64(r.Latency.Milliseconds())

			// Generation time = Total time - TTFT (more accurate than using PromptEvalTime)
			generationTime := r.Latency - r.TTFT
			if generationTime > 0 {
				totalGenerationTimeMs += float64(generationTime.Milliseconds())
			}

			successCount++

			// Track evaluation scores
			if r.EvalResponse != "" {
				totalEvalScore += r.EvalScore
				evalCount++
				if r.EvalResponse == "yes" {
					evalYesCount++
				}
			}
		}
	}

	if len(latencies) == 0 {
		// No successful results - still update with correct success rate (which will be 0)
		successRate := float64(successCount) / float64(len(results))
		metricsCollector.UpdateAggregates(model, testCase, temp, 0, 0, 0, 0, 0, 0, successRate, 0, 0, 0, 0, 0, 0)
		return
	}

	sort.Float64s(latencies)

	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)

	// Calculate TTFT percentiles
	ttftP50 := 0.0
	ttftP95 := 0.0
	if len(ttfts) > 0 {
		sort.Float64s(ttfts)
		ttftP50 = percentile(ttfts, 50)
		ttftP95 = percentile(ttfts, 95)
	}

	// Calculate prompt eval time percentiles
	promptEvalP50 := 0.0
	promptEvalP95 := 0.0
	if len(promptEvalTimes) > 0 {
		sort.Float64s(promptEvalTimes)
		promptEvalP50 = percentile(promptEvalTimes, 50)
		promptEvalP95 = percentile(promptEvalTimes, 95)
	}

	avgTotalTokens := float64(totalPromptTokens+totalCompletionTokens) / float64(successCount)
	successRate := float64(successCount) / float64(len(results))

	// Calculate evaluator metrics
	avgEvalScore := 0.0
	evalPassRate := 0.0
	if evalCount > 0 {
		avgEvalScore = totalEvalScore / float64(evalCount)
		evalPassRate = float64(evalYesCount) / float64(evalCount)
	}

	// Calculate TPS = (Input Tokens + Output Tokens) / Total Turnaround Time (TAT in seconds)
	avgTurnaroundTimeSec := (totalTurnaroundTimeMs / float64(successCount)) / 1000.0
	tokensPerSec := 0.0
	if avgTurnaroundTimeSec > 0 {
		tokensPerSec = avgTotalTokens / avgTurnaroundTimeSec
	}

	// Calculate Output TPS = Output Tokens / Time to Generate Output Tokens
	avgGenerationTimeSec := (totalGenerationTimeMs / float64(successCount)) / 1000.0
	avgOutputTokens := float64(totalCompletionTokens) / float64(successCount)
	outputTokensPerSec := 0.0
	if avgGenerationTimeSec > 0 {
		outputTokensPerSec = avgOutputTokens / avgGenerationTimeSec
	}

	// nsPerOp is passed in from the Go benchmark framework (b.Elapsed() / b.N)
	metricsCollector.UpdateAggregates(model, testCase, temp, p50, p95, ttftP50, ttftP95, promptEvalP50, promptEvalP95, successRate, avgTotalTokens, avgEvalScore, evalPassRate, tokensPerSec, outputTokensPerSec, nsPerOp)
}

// percentile calculates the nth percentile of a sorted slice
func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}

	index := (float64(p) / 100.0) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
