# 11-benchmarks

Contains an example of benchmarking multiple Small Language Models (SLMs) using Go's benchmarking framework, Docker Model Runner, OpenTelemetry, and the Grafana LGTM stack, using an Evaluator Agent pattern.

The goal is to find *the smallest model that won't completely fail you*.

## Libraries Involved

- `github.com/testcontainers/testcontainers-go`: [Testcontainers for Golang](https://github.com/testcontainers/testcontainers-go) is library for running Docker containers for integration tests.
- `github.com/testcontainers/testcontainers-go/modules/dockermodelrunner`: A module for running local language models using Testcontainers and the Docker Model Runner component of Docker Desktop.
- `github.com/testcontainers/testcontainers-go/modules/grafana-lgtm`: A module for running the complete Grafana observability stack (Loki, Grafana, Tempo, Mimir) using Testcontainers.
- `github.com/tmc/langchaingo`: A library for interacting with language models.
- `github.com/tmc/langchaingo/llms/openai`: A specific implementation of the language model interface for OpenAI.
- `go.opentelemetry.io/otel`: The OpenTelemetry SDK for Go, used for instrumentation.
- `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp`: OTLP exporter for metrics over HTTP.
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`: OTLP exporter for traces over HTTP.

## Benchmark Design: Full Factorial Experiments

This benchmark implements a **full factorial design** to systematically explore:
- **Models**: 4 local models + optional OpenAI GPT-5.1 (if `OPENAI_API_KEY` is set)
- **Test Cases**: 5 prompts (code-explanation, mathematical-operations, creative-writing, factual-question, code-generation)
- **Temperatures**: 5 values (0.1, 0.3, 0.5, 0.7, 0.9)

**Total**: 100 scenarios (125 with OpenAI) to answer questions like:
- Which model performs best at low/high temperatures?
- How does quality vary across temperature settings?
- What's the optimal temperature for each model-task combination?

## Code Explanation

The code demonstrates how to benchmark multiple Small Language Models using Go's benchmarking framework combined with observability tools. The benchmarks run models through Docker Model Runner and collect comprehensive metrics using OpenTelemetry, visualizing them in Grafana.

### Main Components

- `bench_main_test.go`: Contains the `TestMain` function that sets up the test environment. It performs the following steps:
  1. Starts the [Grafana LGTM container](https://golang.testcontainers.org/modules/grafana-lgtm/) using Testcontainers. The image used is `grafana/otel-lgtm:0.11.18`.
  2. Initializes OpenTelemetry with OTLP exporters (traces and metrics) pointing to the LGTM stack.
  3. Creates a metrics collector for tracking latency, tokens, success rates, and GPU metrics.
  4. Starts the [Docker Model Runner container](https://golang.testcontainers.org/modules/dockermodelrunner/) for running local language models.
  5. Creates a Grafana dashboard immediately via the Grafana API, allowing real-time monitoring as metrics are collected during benchmark execution.
  6. Runs all benchmarks defined in `bench_llm_test.go`.
  7. Cleans up all resources on exit.

- `bench_llm_test.go`: Contains the `BenchmarkLLMs` function that benchmarks multiple models:
  1. Checks for `OPENAI_API_KEY` to optionally include GPT-5.1 (runs first if present).
  2. Defines 4 local models from [Docker's GenAI catalog](https://hub.docker.com/catalogs/gen-ai): Llama 3.2 1B/3B, Qwen3 0.6B, Llama 3.2 1B Instruct.
  3. Runs all test cases × temperature combinations for each model.
  4. Records latency, prompt evaluation time, tokens, success rate, and optional GPU metrics.
  5. Creates OpenTelemetry traces with exemplars for each request.
  6. Reports aggregate metrics (p50, p95, tokens/sec, etc.).

- `llmclient/llmclient.go`: Wraps LLM client with OpenTelemetry tracing. Automatically detects OpenAI endpoints and handles authentication via `OPENAI_API_KEY`.

- `otel_setup.go`: Initializes OpenTelemetry with OTLP exporters for traces and metrics.

- `metrics.go`: Defines histograms (latency, prompt eval time with exemplars) and gauges (p50/p95, success rate, tokens/sec, GPU metrics).

- `gpu.go`: Samples GPU metrics with auto-detection for NVIDIA (`nvidia-smi`) and Apple Silicon (`ioreg`). See "GPU Metrics" section below for details.

- `grafana_dash.go`: Creates a Grafana dashboard titled "LLM Bench (DMR + Testcontainers)" with 10 panels:
  1. **Latency Percentiles (p50/p95)** - Overall response time metrics
  2. **Latency Histogram with Exemplars** - Response time distribution with drill-down to traces
  3. **Prompt Evaluation Time (p50/p95)** - NEW: Time to first token metrics
  4. **Prompt Eval Time Distribution with Exemplars** - NEW: Prompt processing patterns
  5. **Tokens per Operation** - Token usage and verbosity
  6. **Success Rate** - Model reliability metrics
  7. **Tokens per Second** - Generation throughput
  8. **GPU Utilization** (Optional) - Hardware efficiency
  9. **GPU Memory** (Optional) - Memory consumption
  10. **Score per Operation** - Quality metrics

  The dashboard includes template variables for filtering by model, test case, and temperature. The dashboard uses a fixed UID (`llm-bench-dmr-tc`) to ensure it is **automatically updated** on each benchmark run without creating duplicates.

## Running the Example

### Apple Silicon (M1/M2/M3/M4) - Recommended

**With GPU metrics** (requires sudo for utilization tracking):
```sh
sudo go test -bench=. -benchtime=5x -timeout=30m
```

**With OpenAI GPT-5.1** (optional):
```sh
export OPENAI_API_KEY="your-api-key-here"
sudo go test -bench=. -benchtime=5x -timeout=30m
```

**Why sudo?** Apple Silicon requires `powermetrics` for GPU utilization metrics. Without sudo, you'll only see GPU memory (not utilization). The benchmark works fine without sudo, but you'll get more complete GPU metrics with it.

### Other Platforms (Linux/Windows with NVIDIA)

**Basic usage**:
```sh
go test -bench=. -benchtime=5x -timeout=30m
```

**With OpenAI**:
```sh
export OPENAI_API_KEY="your-api-key-here"
go test -bench=. -benchtime=5x -timeout=30m
```

### What to Expect

- 5 iterations per benchmark, up to 30 min timeout (model downloads take time)
- **100 scenarios** (4 local models × 5 test cases × 5 temperatures)
- **125 scenarios** with OpenAI API key (adds GPT-5.1)
- Containers kept running after completion for dashboard exploration

The benchmark disables [Ryuk garbage collector](https://golang.testcontainers.org/features/garbage_collector/#ryuk) to keep containers running after completion for dashboard exploration.

**Console output** showing metrics:

```shell
BenchmarkLLMs/Pull/llama3.2-8                                  1         ...
BenchmarkLLMs/llama3.2/code-explanation/temp0.1-8              5         ... latency_p50_ms:250.00 latency_p95_ms:280.00 prompt_eval_p50_ms:45.00 prompt_eval_p95_ms:52.00 tokens_per_op:45.00 success_rate:1.00 score:0.85 tokens_per_sec:180.00
BenchmarkLLMs/llama3.2/code-explanation/temp0.3-8              5         ... latency_p50_ms:255.00 prompt_eval_p50_ms:46.00 ...
BenchmarkLLMs/llama3.2/code-explanation/temp0.5-8              5         ... latency_p50_ms:260.00 prompt_eval_p50_ms:47.00 ...
BenchmarkLLMs/llama3.2/code-explanation/temp0.7-8              5         ... latency_p50_ms:270.00 prompt_eval_p50_ms:48.00 ...
BenchmarkLLMs/llama3.2/code-explanation/temp0.9-8              5         ... latency_p50_ms:290.00 prompt_eval_p50_ms:50.00 ...
BenchmarkLLMs/llama3.2/mathematical-operations/temp0.1-8       5         ... latency_p50_ms:150.00 prompt_eval_p50_ms:30.00 ...
...
```

**Grafana URL** (printed at startup):
```shell
📊 Grafana Observability Stack Ready
=================================================
URL: http://localhost:xxxxx
Credentials: admin / admin
Dashboard will be created after benchmarks run
=================================================
```

Open in browser (`admin`/`admin`) to view real-time metrics, latency distributions, traces, and model comparisons.

### GPU Metrics

**Supported GPUs**:
- **NVIDIA**: Auto-detected via `nvidia-smi` (utilization % and memory MB)
- **Apple Silicon (M1/M2/M3/M4)**: Auto-detected via `powermetrics` and `ioreg`
  - **GPU Memory**: Tracks allocation via `ioreg` (works without sudo)
  - **GPU Utilization**: Tracks active residency via `powermetrics` (requires sudo)
  - **Recommended**: Run with `sudo go test ...` for complete metrics

**Container Limitation**: GPU detection fails in containerized environments (Docker, Claude Code CLI) because containers can't access host GPU tools. **Run directly on host** for GPU metrics.

**What you'll see on Apple Silicon**:
- **With sudo**: Both GPU memory spikes and utilization % during model inference
- **Without sudo**: Only GPU memory (utilization will show 0%)

## Understanding the Metrics

The benchmark collects and reports several key metrics to help you evaluate model performance:

### Console Metrics

- **latency_p50_ms / latency_p95_ms**: Median and 95th percentile response time (ms)
- **prompt_eval_p50_ms / prompt_eval_p95_ms**: Time to first token (TTFT) - prompt processing time (ms)
- **tokens_per_op**: Average tokens per request (prompt + completion)
- **success_rate**: Percentage of successful requests (0.0-1.0)
- **score**: Quality score (0.0-1.0) based on response characteristics
- **tokens_per_sec**: Generation throughput

### Grafana Dashboard Panels

The dashboard **"LLM Bench (DMR + Testcontainers)"** includes 10 panels with template variables (model, case, temp) for filtering:

#### 1-2. Latency (Percentiles & Histogram with Exemplars)
- **p50/p95**: Median and worst-case response times
- **Histogram**: Distribution patterns; click exemplars to drill into traces
- Lower = faster; compare consistency (p95 vs p50 gap)

#### 3-4. Prompt Eval Time (Percentiles & Distribution with Exemplars)
- Time to first token (TTFT) - prompt processing before generation
- Critical for perceived responsiveness
- Click exemplars to investigate slow evaluations

#### 5. Tokens per Operation
- Average tokens per request (verbosity indicator)
- Use for cost estimation: tokens/request × requests/day × cost/token

#### 6. Success Rate
- Reliability: % of requests completed without errors (0-100%)
- Target: 95%+ for production
- Note: Measures reliability, not quality

#### 7. Tokens per Second
- Generation throughput
- Higher = faster; critical for real-time apps

#### 8-9. GPU Utilization & Memory (Optional)
- **Utilization**: 0-100% usage (near 100% = may need more GPUs)
- **Memory**: MB consumed (check model fits your GPU)
- Requires host execution (not containers); see "GPU Metrics" section above

#### 10. Score per Operation
- Quality rating (0.0-1.0) based on response characteristics
- Customizable in `calculateScore` function
- Balance with speed and success rate

### Dashboard Template Variables

Filter results by:
- **model**: Specific models (e.g., `ai/llama3.2:1B-Q4_0`) - use "All" for overview or multi-select for comparison
- **case**: Test case type (e.g., `code-explanation`, `mathematical-operations`) - focus on your use cases
- **temp**: Temperature (0.1-0.9) - lower = deterministic, higher = creative

**Tips**:
- Combine dimensions (model + case + temp) to drill down
- Fix two dimensions, vary one to isolate effects
- Variables affect all panels simultaneously

## How to Read This Dashboard

### Example 1: Latency Histogram with Exemplars

![Latency Histogram with Exemplars](screenshots/latency-exemplars.png)

**What you're seeing**:
- **Histogram bars**: Distribution of response times across models/test cases
- **Colored dots (exemplars)**: Individual requests that fell into each bucket

**How to use it**:
- Identify slow buckets (right side of histogram)
- Look for patterns: Are certain models/temps consistently slower?
- Filter by a specific model and test case using template variables to understand variance within that configuration

### Example 2: Tokens/sec vs GPU Utilization

![Tokens per Second vs GPU Utilization](screenshots/tokens-gpu.png)

**What you're seeing**:
- **Top panel**: Token generation throughput (tokens/sec) per model
- **Bottom panel**: GPU utilization percentage during benchmark (requires sudo on Apple Silicon)

**How to interpret**:
- **High tokens/sec + High GPU util**: Model efficiently using GPU (ideal)
- **Low tokens/sec + High GPU util**: GPU-bound bottleneck (model is slow despite GPU usage)
- **High tokens/sec + Low GPU util**: Efficient model with GPU headroom (can run more models)
- **Low tokens/sec + Low GPU util**: Likely CPU-bound or other bottlenecks

**Use case**:
- Determine if you can run multiple models on the same GPU by checking utilization headroom
- Identify GPU vs CPU bottlenecks

**Note**: Run with `sudo` on Apple Silicon for utilization metrics (see "Running the Example" section).

## Model Selection Guide

**Goal**: Find the smallest model that meets your requirements.

### Selection Process

1. **Filter by use case** (e.g., `case="code-generation"`)
2. **Check Success Rate** → Eliminate models <95%
3. **Check Score** → Target >0.7 for quality
4. **Check Latency p95** → Must meet your SLA
5. **Check Tokens/sec** → Ensure adequate throughput
6. **Check GPU Memory** → Verify hardware fit
7. **Select smallest model** passing all criteria

### Example: Code Assistant

**Requirements**: p95<500ms, success>98%, score>0.75, GPU<4GB

**Results**:
- Qwen3 0.6B: Fast (p95=200ms) but low score (0.65) ❌
- Llama 3.2 1B: p95=350ms, score=0.80 ✅ **Winner**
- Llama 3.2 3B: p95=650ms, score=0.90 (too slow) ❌

### Cost Calculation

`Cost per request = (GPU cost/hour × utilization%) / requests per hour`

## Extending the Benchmark

Edit `bench_llm_test.go` to customize:

- **External API Models**: Edit `getModelsToTest()` - external models skip pull step and run first
- **Local Models**: Edit `localModels` slice (must be in [Docker's GenAI catalog](https://hub.docker.com/catalogs/gen-ai))
- **Temperatures**: Edit `temperatures` slice (line ~71)
- **Test Cases**: Edit `testCases` slice (line ~42)
- **Quality Scoring**: Improve `calculateScore()` (line ~243) with:
  - Semantic similarity vs expected outputs
  - LLM-as-evaluator (Evaluator Agent pattern)
  - BLEU, ROUGE, or task-specific validators
