package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/mdelapenya/genai-testcontainers-go/benchmarks/llmclient"
	dmr "github.com/testcontainers/testcontainers-go/modules/dockermodelrunner"
)

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
			UserPrompt:   "Explain what this Go code does: func main() { fmt.Println(\"Hello\") }",
		},
		{
			Name:         "simple-math",
			SystemPrompt: "You are a mathematics tutor.",
			UserPrompt:   "What is 15 multiplied by 23?",
		},
		{
			Name:         "creative-writing",
			SystemPrompt: "You are a creative writer.",
			UserPrompt:   "Write a one-sentence story about a robot learning to paint.",
		},
		{
			Name:         "factual-question",
			SystemPrompt: "You are a knowledgeable assistant.",
			UserPrompt:   "What is the capital of France?",
		},
		{
			Name:         "code-generation",
			SystemPrompt: "You are a Go programming expert.",
			UserPrompt:   "Write a Go function that reverses a string.",
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
	PromptEvalTime   time.Duration // Time to evaluate prompt (time to first token)
	PromptTokens     int           // Input tokens
	CompletionTokens int           // Output tokens generated
	TotalTokens      int           // Total tokens (prompt + completion)
	Success          bool
	Score            float64
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
						result := runSingleBenchmark(ctx, client, modelName, tc, temp)
						results = append(results, result)

						// Record latency with OpenTelemetry
						metricsCollector.RecordLatency(ctx, result.Latency, modelName, tc.Name, temp)

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

					// Update OpenTelemetry gauges with model/case/temp labels
					updateGauges(modelName, tc.Name, temp, results)
				})
			}
		}
	}
}

// runSingleBenchmark executes a single benchmark iteration
func runSingleBenchmark(ctx context.Context, client *llmclient.Client, model string, tc TestCase, temp float64) BenchmarkResult {
	resp, err := client.GenerateWithTemp(ctx, tc.SystemPrompt, tc.UserPrompt, temp)

	result := BenchmarkResult{
		Model:    model,
		TestCase: tc.Name,
		Temp:     temp,
		Success:  err == nil,
	}

	if err == nil {
		result.Latency = resp.Latency
		result.PromptEvalTime = resp.PromptEvalTime
		result.PromptTokens = resp.PromptTokens
		result.CompletionTokens = resp.CompletionTokens
		result.TotalTokens = resp.TotalTokens
		result.Score = calculateScore(resp)
	} else {
		// Log error for debugging (will appear in benchmark output)
		fmt.Printf("❌ Error in %s/%s/temp%.1f: %v\n", model, tc.Name, temp, err)
	}

	return result
}

// calculateScore calculates a quality score for the response
func calculateScore(resp *llmclient.Response) float64 {
	// Simple scoring based on response characteristics
	// In a real evaluator, you would use more sophisticated metrics
	score := 0.0

	// Base score for getting a response
	if len(resp.Content) > 0 {
		score += 0.5
	}

	// Score based on response length (reasonable length is good)
	contentLen := len(resp.Content)
	if contentLen >= 20 && contentLen <= 500 {
		score += 0.3
	} else if contentLen > 500 {
		score += 0.2
	}

	// Score based on token efficiency
	if resp.CompletionTokens > 0 && resp.CompletionTokens < 200 {
		score += 0.2
	}

	return score
}

// reportAggregateMetrics calculates and reports aggregate metrics
func reportAggregateMetrics(b *testing.B, results []BenchmarkResult) {
	if len(results) == 0 {
		return
	}

	// Calculate latency percentiles
	latencies := make([]float64, 0, len(results))
	promptEvalTimes := make([]float64, 0, len(results))
	totalPromptTokens := 0
	totalCompletionTokens := 0
	totalTurnaroundTimeMs := 0.0
	totalGenerationTimeMs := 0.0
	successCount := 0
	totalScore := 0.0

	for _, r := range results {
		if r.Success {
			latencies = append(latencies, float64(r.Latency.Milliseconds()))
			if r.PromptEvalTime > 0 {
				promptEvalTimes = append(promptEvalTimes, float64(r.PromptEvalTime.Milliseconds()))
			}
			totalPromptTokens += r.PromptTokens
			totalCompletionTokens += r.CompletionTokens
			totalTurnaroundTimeMs += float64(r.Latency.Milliseconds())

			// Generation time = Total time - Prompt eval time
			generationTime := r.Latency - r.PromptEvalTime
			if generationTime > 0 {
				totalGenerationTimeMs += float64(generationTime.Milliseconds())
			}

			successCount++
			totalScore += r.Score
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
		b.ReportMetric(0, "score")
		b.ReportMetric(0, "tokens_per_sec")
		b.ReportMetric(0, "output_tokens_per_sec")
		return
	}

	sort.Float64s(latencies)

	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)

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
	avgScore := totalScore / float64(successCount)

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

	// Report custom metrics
	b.ReportMetric(p50, "latency_p50_ms")
	b.ReportMetric(p95, "latency_p95_ms")
	b.ReportMetric(promptEvalP50, "prompt_eval_p50_ms")
	b.ReportMetric(promptEvalP95, "prompt_eval_p95_ms")
	b.ReportMetric(avgTotalTokens, "tokens_per_op")
	b.ReportMetric(successRate, "success_rate")
	b.ReportMetric(avgScore, "score")
	b.ReportMetric(tokensPerSec, "tokens_per_sec")
	b.ReportMetric(outputTokensPerSec, "output_tokens_per_sec")
}

// updateGauges updates OpenTelemetry gauge metrics with model/case/temp labels
func updateGauges(model, testCase string, temp float64, results []BenchmarkResult) {
	if len(results) == 0 {
		return
	}

	latencies := make([]float64, 0, len(results))
	promptEvalTimes := make([]float64, 0, len(results))
	totalPromptTokens := 0
	totalCompletionTokens := 0
	totalTurnaroundTimeMs := 0.0
	totalGenerationTimeMs := 0.0
	successCount := 0
	totalScore := 0.0

	for _, r := range results {
		if r.Success {
			latencies = append(latencies, float64(r.Latency.Milliseconds()))
			if r.PromptEvalTime > 0 {
				promptEvalTimes = append(promptEvalTimes, float64(r.PromptEvalTime.Milliseconds()))
			}
			totalPromptTokens += r.PromptTokens
			totalCompletionTokens += r.CompletionTokens
			totalTurnaroundTimeMs += float64(r.Latency.Milliseconds())

			// Generation time = Total time - Prompt eval time
			generationTime := r.Latency - r.PromptEvalTime
			if generationTime > 0 {
				totalGenerationTimeMs += float64(generationTime.Milliseconds())
			}

			successCount++
			totalScore += r.Score
		}
	}

	if len(latencies) == 0 {
		// No successful results - still update with correct success rate (which will be 0)
		successRate := float64(successCount) / float64(len(results))
		metricsCollector.UpdateAggregates(model, testCase, temp, 0, 0, 0, 0, successRate, 0, 0, 0, 0)
		return
	}

	sort.Float64s(latencies)

	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)

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
	avgScore := totalScore / float64(successCount)

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

	metricsCollector.UpdateAggregates(model, testCase, temp, p50, p95, promptEvalP50, promptEvalP95, successRate, avgTotalTokens, avgScore, tokensPerSec, outputTokensPerSec)
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
