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

This benchmark implements a **full factorial design** to systematically explore the parameter space:

**Dimensions**:
- **Models**: 4-5 models (Llama 3.2 1B, Llama 3.2 3B, Qwen3 0.6B, Llama 3.2 1B Instruct from HuggingFace, and optionally GPT-5.1 from OpenAI if `OPENAI_API_KEY` is set)
- **Test Cases**: 5 prompts (code-explanation, simple-math, creative-writing, factual-question, code-generation)
- **Temperatures**: 5 values (0.1, 0.3, 0.5, 0.7, 0.9)

**Total Combinations**:
- **Without OpenAI**: 4 models × 5 test cases × 5 temperatures = **100 unique benchmark scenarios**
- **With OpenAI**: 5 models × 5 test cases × 5 temperatures = **125 unique benchmark scenarios**

This design allows you to answer questions like:
- Which model performs best at low temperatures (deterministic)?
- How does creative writing quality vary across temperatures?
- Which model is most consistent across different temperature settings?
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

- `bench_llm_test.go`: Contains the `BenchmarkLLMs` function that benchmarks multiple models. It performs the following steps:
  1. Conditionally checks for `OPENAI_API_KEY` environment variable:
     - **If found**: Adds GPT-5.1 from OpenAI's API to the benchmark suite (runs first)
     - **If not found**: Skips OpenAI models and only benchmarks local models
  2. Defines multiple local models to benchmark: Llama 3.2 1B (`ai/llama3.2:1B-Q4_0`), Llama 3.2 3B (`ai/llama3.2:3B-Q4_K_M`), Qwen3 0.6B (`ai/qwen3:0.6B-Q4_0`), and Llama 3.2 1B Instruct from HuggingFace (`hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF`). All local models are available in [Docker's GenAI catalog](https://hub.docker.com/catalogs/gen-ai).
  3. Defines a prompt battery with test cases: code explanation, simple math, creative writing, factual questions, and code generation.
  4. For each model:
     - **Local models**: Pulls from Docker Model Runner (if not already cached)
     - **External models (OpenAI)**: Skips pull step, connects directly to API
  5. For each test case and temperature combination, runs the benchmark multiple times, sending prompts to the model and recording latency, prompt evaluation time, tokens, and success rate.
  6. Samples GPU metrics periodically using `nvidia-smi` or `ioreg` for Apple Silicon (optional, gracefully handles absence).
  7. Records OpenTelemetry traces with exemplars for each request, linking metrics to traces.
  8. Calculates aggregate metrics (p50, p95, prompt eval time, success rate, tokens/sec) and reports them.

- `llmclient/llmclient.go`: A wrapper around the language model client that adds OpenTelemetry tracing. Each request is traced with span attributes including model, prompts, token counts, and prompt evaluation time. The client automatically detects external OpenAI API endpoints and uses the `OPENAI_API_KEY` environment variable for authentication. Token counts and prompt evaluation time are extracted from the API response when available, with fallback estimation when not provided.

- `otel_setup.go`: Initializes OpenTelemetry with a `TracerProvider` (batch span processor) and `MeterProvider` (periodic reader with 5s interval), both configured with OTLP HTTP exporters.

- `metrics.go`: Defines the `MetricsCollector` that provides histograms for latency and prompt evaluation time (both with exemplar support) and observable gauges for aggregate metrics (latency p50/p95, prompt eval time p50/p95, success rate, tokens per operation, score, tokens per second, GPU utilization, GPU memory).

- `gpu.go`: Samples GPU metrics with vendor-specific implementations. Automatically detects GPU vendor (NVIDIA or Apple Silicon) and uses the appropriate sampling method:
  - **NVIDIA GPUs**: Uses `nvidia-smi` to query GPU utilization and memory usage
  - **Apple Silicon (M1/M2/M3/M4)**: Uses `ioreg` (no sudo required) to extract GPU metrics from IOAccelerator, including:
    - Memory usage from `IOAcceleratorAllocatedMemory`, `InUseSystemMemory`, or `vramUsedBytes`
    - Utilization attempts from `PerformanceStatistics`, active/idle ticks, or frequency ratios
    - Note: Utilization metrics may be limited or unavailable without sudo access to `powermetrics`
  - **No GPU**: Returns zero metrics gracefully without errors
  - **Note**: When running inside containers (like Docker), GPU detection may not work as expected since the container sees the container OS, not the host GPU. GPU metrics will show as unavailable in containerized environments.

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

### Basic Usage (Local Models Only)

To run the example with local models only, navigate to the `11-benchmarks` directory and run:

```sh
go test -bench=. -benchtime=5x -timeout=30m
```

### Including OpenAI GPT-5.1 (Optional)

To benchmark against OpenAI's GPT-5.1 model, set the `OPENAI_API_KEY` environment variable:

```sh
export OPENAI_API_KEY="your-api-key-here"
go test -bench=. -benchtime=5x -timeout=30m
```

**What happens**:
- ✅ GPT-5.1 is added to the benchmark suite and **runs first**
- ✅ All 5 test cases × 5 temperatures = **25 additional benchmark scenarios** for GPT-5.1
- ✅ Results appear in Grafana alongside local models for direct comparison
- ℹ️  Note: OpenAI API calls will incur costs based on your OpenAI pricing plan

**Without the API key**:
- ℹ️  The benchmark prints: `No OPENAI_API_KEY found - skipping OpenAI models`
- ✅ Only local models (4) are benchmarked = **100 scenarios**

The benchmark automatically disables the [Ryuk garbage collector](https://golang.testcontainers.org/features/garbage_collector/#ryuk) in `TestMain`, which keeps the Grafana LGTM and DMR containers running after the benchmark completes so you can explore the dashboard and metrics.

This will:
- Run 5 iterations of each benchmark (`-benchtime=5x`)
- Allow up to 30 minutes for completion (some models take time to download)
- Pull and test all configured models
- Send metrics and traces to the LGTM stack
- Create the Grafana dashboard automatically
- Keep containers running after completion for exploration

The benchmark will print results to the console:

```shell
BenchmarkLLMs/Pull/llama3.2-8                                  1         ...
BenchmarkLLMs/llama3.2/code-explanation/temp0.1-8              5         ... latency_p50_ms:250.00 latency_p95_ms:280.00 prompt_eval_p50_ms:45.00 prompt_eval_p95_ms:52.00 tokens_per_op:45.00 success_rate:1.00 score:0.85 tokens_per_sec:180.00
BenchmarkLLMs/llama3.2/code-explanation/temp0.3-8              5         ... latency_p50_ms:255.00 prompt_eval_p50_ms:46.00 ...
BenchmarkLLMs/llama3.2/code-explanation/temp0.5-8              5         ... latency_p50_ms:260.00 prompt_eval_p50_ms:47.00 ...
BenchmarkLLMs/llama3.2/code-explanation/temp0.7-8              5         ... latency_p50_ms:270.00 prompt_eval_p50_ms:48.00 ...
BenchmarkLLMs/llama3.2/code-explanation/temp0.9-8              5         ... latency_p50_ms:290.00 prompt_eval_p50_ms:50.00 ...
BenchmarkLLMs/llama3.2/simple-math/temp0.1-8                   5         ... latency_p50_ms:150.00 prompt_eval_p50_ms:30.00 ...
...
```

Note: Each test case runs with all temperature values (0.1, 0.3, 0.5, 0.7, 0.9) for comprehensive comparison.

The Grafana dashboard is created **before** benchmarks start, allowing you to monitor metrics in real-time. The console will print the Grafana URL at startup:

The Grafana URL is displayed at the start of the benchmark run:

```shell
📊 Grafana Observability Stack Ready
=================================================
URL: http://localhost:xxxxx
Credentials: admin / admin
Dashboard will be created after benchmarks run
=================================================
```

Open this URL in your browser (default credentials: `admin` / `admin`) to view:
- Real-time metrics and latency distributions
- Latency histogram with exemplars (click exemplars to jump to traces in Tempo)
- GPU utilization graphs (if GPU is available)
- Model comparison charts with template variables for filtering

### GPU Metrics (Optional)

GPU metrics are **optional** and support multiple GPU vendors:

#### Supported GPUs

- **NVIDIA GPUs**: Automatically detected using `nvidia-smi`
  - Records GPU utilization percentage and memory usage in MB
  - No additional configuration required if `nvidia-smi` is installed

- **Apple Silicon (M1/M2/M3/M4)**: Automatically detected on macOS ARM64
  - Uses `ioreg` to query GPU metrics from IOAccelerator (no sudo required)
  - **GPU Memory**: Extracts allocated memory from IOAccelerator properties
  - **GPU Utilization**: Attempts to extract from PerformanceStatistics or calculate from active/idle ticks
  - **Note**: Utilization metrics may be limited compared to `powermetrics` (which requires sudo), but memory metrics should work reliably

- **No GPU**: Gracefully continues without GPU metrics if no supported GPU is detected

#### Important Limitations

**Running in Containers**: When this benchmark runs inside Docker or other containerized environments (including Claude Code's execution environment), GPU detection may not work as expected because:
- The container sees `GOOS=linux` even when running on macOS
- Container isolation prevents access to host GPU tools like `nvidia-smi` or `powermetrics`
- GPU metrics will show as unavailable (`Available: false`)

To get GPU metrics:
- Run the benchmark **directly on the host** (not in a container)
- Ensure GPU vendor tools are installed:
  - **NVIDIA**: `nvidia-smi` must be available
  - **Apple Silicon**: `ioreg` (built into macOS, no installation needed)

**Note on Apple Silicon Utilization**: The `ioreg` approach provides memory metrics reliably but GPU utilization may be limited or zero. This is because:
- `ioreg` exposes some performance statistics, but not all systems/drivers populate these fields
- The most accurate utilization metrics require `powermetrics` which needs sudo
- If you need precise utilization metrics on Apple Silicon, you can run with sudo:
  ```sh
  sudo go test -bench=. -benchtime=5x -timeout=30m
  ```
  However, sudo is **not required** - the benchmark works without it, just with limited utilization data.

If GPU metrics are unavailable, the benchmark will continue normally and GPU panels in Grafana will show zero values.

## Understanding the Metrics

The benchmark collects and reports several key metrics to help you evaluate model performance:

### Console Metrics (Go Benchmark Output)

For each model and test case combination, you'll see:

- **latency_p50_ms**: The 50th percentile latency in milliseconds. This represents the median response time - half of requests are faster, half are slower.
- **latency_p95_ms**: The 95th percentile latency in milliseconds. 95% of requests complete faster than this time. Useful for understanding worst-case scenarios.
- **prompt_eval_p50_ms**: The 50th percentile prompt evaluation time in milliseconds. This is the time the model takes to process the input prompt before generating the response (also known as "time to first token" or TTFT).
- **prompt_eval_p95_ms**: The 95th percentile prompt evaluation time in milliseconds. Shows the worst-case prompt processing time.
- **tokens_per_op**: Average total tokens (prompt + completion) per operation. Higher numbers indicate more verbose responses or longer prompts.
- **success_rate**: Percentage of successful requests (0.0 to 1.0). A value of 1.0 means all requests succeeded.
- **score**: Quality score (0.0 to 1.0) based on response characteristics like length and token efficiency. Higher is better.
- **tokens_per_sec**: Throughput metric showing how many tokens the model generates per second. Higher values indicate faster generation.

### Grafana Dashboard Panels Explained

The automatically created dashboard titled **"LLM Bench (DMR + Testcontainers)"** provides comprehensive visualization of LLM benchmark results. Here's a detailed breakdown of each panel:

#### 1. Latency Percentiles Panel
- **What It Shows**: How long models take to respond
- **Metrics Displayed**:
  - **p50 (median)**: Typical response time - half of requests are faster, half are slower
  - **p95**: Worst-case response time - 95% of requests are faster than this
- **Purpose**: Compare model speed and understand response time variability
- **How to Use**:
  - Lower values = faster models
  - Compare p50 to see which model is typically fastest
  - Check p95 to understand worst-case scenarios for latency-sensitive apps
  - Look for consistency: if p95 is much higher than p50, responses are unpredictable

#### 2. Latency Histogram with Exemplars Panel
- **What It Shows**: Distribution of response times - how requests cluster around certain latencies
- **Purpose**: Understand latency patterns and identify anomalies
- **Key Feature**: Click on data points to see detailed traces of individual requests
- **How to Use**:
  - See if most responses cluster around one time (consistent) or spread out (variable)
  - Identify outliers: unusually slow or fast requests
  - Click exemplar points (dots) to drill into specific slow requests
  - Compare histogram shapes across models to see which is most predictable

#### 3. Prompt Evaluation Time (p50/p95) Panel
- **What It Shows**: Time for the model to process the input prompt before generating output (time to first token)
- **Metrics Displayed**:
  - **p50 (median)**: Typical prompt processing time
  - **p95**: Worst-case prompt processing time
- **Purpose**: Measure how quickly the model begins responding after receiving the prompt
- **Why It Matters**: Prompt evaluation time is critical for user experience - lower values mean faster perceived responsiveness
- **How to Use**:
  - Lower values = model starts responding faster (better perceived latency)
  - Compare across models: some models may be faster at prompt evaluation but slower at generation
  - Longer prompts typically take longer to evaluate
  - Useful for understanding if latency issues come from prompt processing or generation

#### 4. Prompt Eval Time Distribution (with Exemplars) Panel
- **What It Shows**: Distribution of prompt evaluation times across requests
- **Purpose**: Understand prompt processing patterns and identify prompt evaluation bottlenecks
- **Key Feature**: Click on data points to see detailed traces
- **How to Use**:
  - Identify if prompt evaluation time is consistent or varies widely
  - Find outliers: prompts that take unusually long to process
  - Compare across models to see which processes prompts most efficiently
  - Click exemplars to investigate slow prompt evaluations

#### 5. Tokens per Operation Panel
- **What It Shows**: Average number of tokens (words/pieces) used per request
- **Purpose**: Understand model verbosity and estimate costs
- **Why It Matters**: Most LLM APIs charge per token
- **How to Use**:
  - Lower values = more concise responses (cheaper, faster to read)
  - Higher values = more verbose responses (potentially more detailed but costlier)
  - Compare across models to find the right balance of detail vs. cost
  - Use to calculate operating costs: tokens/request × requests/day × cost/token

#### 6. Success Rate Panel
- **What It Shows**: Percentage of API calls that completed successfully without errors
- **Values**: 0-100% (where 100% = perfect reliability)
- **Purpose**: Measure model stability and reliability
- **What Success Means**: The model responded without crashing, timing out, or returning errors
- **What Failure Means**: Network issues, model crashes, timeouts, or API errors
- **How to Use**:
  - **High success rate (95-100%)**: Model is reliable and production-ready
  - **Low success rate (<95%)**: Model has stability issues - investigate or avoid
  - Compare success rates to identify which models are most stable
  - Filter by temperature to see if high temps cause instability
- **Important**: Success rate measures reliability, not quality. A model can have 100% success with poor output quality.

#### 7. Tokens per Second Panel
- **What It Shows**: How fast the model generates text
- **Purpose**: Measure throughput and generation speed
- **How to Use**:
  - Higher values = faster generation (better user experience)
  - Critical for real-time apps: chatbots, live coding assistants, etc.
  - Compare small vs. large models: sometimes smaller models are more efficient
  - Helps estimate capacity: tokens/sec × time = max output length

#### 8. GPU Utilization Panel (Optional)
- **What It Shows**: Percentage of GPU being used (0-100%)
- **Purpose**: Monitor hardware efficiency and plan capacity
- **Requires**:
  - NVIDIA GPU with `nvidia-smi` installed, OR
  - Apple Silicon Mac (M1/M2/M3/M4) running on host
- **Limitations**:
  - Shows zero values when running in containerized environments (e.g., Docker, Claude Code CLI)
  - On Apple Silicon, utilization may be zero or limited without sudo (memory metrics will still work)
  - For most accurate Apple Silicon metrics, run with sudo, but this is optional
- **How to Use**:
  - **Near 100%**: GPU fully utilized - may need more GPUs for scaling
  - **Below 50%**: GPU underutilized - can handle more load or use smaller GPU
  - Compare across model sizes: smaller models may use GPU more efficiently
  - Plan costs: can you run multiple models on one GPU?

#### 9. GPU Memory Panel (Optional)
- **What It Shows**: How much GPU memory each model consumes (in MB)
- **Purpose**: Understand memory requirements and plan hardware
- **Requires**:
  - NVIDIA GPU with `nvidia-smi` installed, OR
  - Apple Silicon Mac with `ioreg` (built into macOS, no installation needed)
  - Must run on host (not in container) for GPU access
- **Limitations**: Shows zero values when running in containerized environments
- **Apple Silicon Note**: Memory metrics work without sudo and should provide reliable data
- **How to Use**:
  - Compare memory footprints: larger models use more memory
  - Check if model fits your GPU (e.g., 16GB model needs 16GB+ GPU)
  - Plan for multiple models: total memory < GPU capacity
  - Identify if you can use cheaper GPUs with less memory for smaller models

#### 10. Score per Operation Panel
- **What It Shows**: Quality rating of model responses (0.0 = poor, 1.0 = excellent)
- **Purpose**: Measure output quality beyond just speed
- **What It Measures**: Response characteristics like length, completeness, and token efficiency
- **How to Use**:
  - Higher scores = better quality responses
  - Balance against speed: fastest model isn't always best if quality is low
  - Filter by test case to see which models excel at specific tasks
  - Combine with success rate: need both high quality AND reliability
- **Customizable**: You can modify the scoring logic to match your quality criteria

### Dashboard Template Variables

The dashboard includes powerful template variables for filtering and comparing results across all dimensions:

#### model
- **Purpose**: Filter results by specific model
- **Example Values**: `ai/llama3.2:1B-Q4_0`, `ai/llama3.2:3B-Q4_K_M`, `ai/qwen3:0.6B-Q4_0`
- **Usage**: Select one or multiple models to compare side-by-side
- **Tip**: Use "All" to see aggregate view, or select specific models for detailed comparison
- **Experiment**: Compare different model sizes or architectures on the same task

#### case
- **Purpose**: Filter by test case/prompt type
- **Example Values**: `code-explanation`, `simple-math`, `creative-writing`, `factual-question`, `code-generation`
- **Usage**: Focus on specific use cases that match your application needs
- **Tip**: Compare how different models perform on the same test case
- **Experiment**: Add your own test cases to evaluate models on your specific use cases

#### temp
- **Purpose**: Filter by temperature parameter
- **Example Values**: `0.1`, `0.3`, `0.5`, `0.7`, `0.9`
- **Usage**: Compare how temperature affects model behavior and quality
- **Tip**: Lower temperatures (0.1-0.3) produce more deterministic outputs; higher values (0.7-0.9) increase creativity and randomness
- **Experiment**: Run the same prompt with multiple temperatures to find the optimal setting for your use case

**Pro Tips for Using Template Variables**:
- **Combine all three dimensions**: Select specific model + case + temp to drill down into exact combinations
- **Use "All" strategically**: Start with all dimensions open to get an overview, then narrow down
- **Multi-select for comparisons**: Select 2-3 values in each dimension to compare directly
- **Experiment systematically**: Fix two dimensions and vary the third to isolate effects (e.g., same model and case, different temps)
- **Template variables affect all panels simultaneously** for consistent filtering across the dashboard

**Benchmarking Philosophy**: This is a flexible experimentation framework designed for systematic exploration. Every test case runs with every temperature value, creating a complete factorial design. You can easily add more models, test cases, or temperature values to expand the experiment.

## How to Use This for Model Selection

This benchmark helps you answer the question: *"Which is the smallest model that won't completely fail you?"*

### Step-by-Step Selection Process

#### 1. Performance vs. Size Trade-off
- **Action**: Compare latency and throughput across models of different sizes
- **Dashboard View**: Use the Latency Percentiles and Tokens per Second panels
- **Insight**: Qwen3 (0.6B) will be faster but may produce lower quality responses than Llama 3.2 1B or the larger Llama 3.2 3B model
- **Decision Criteria**: Find the smallest model that meets your latency requirements

#### 2. Use Case Evaluation
- **Action**: Evaluate performance on specific use cases
- **Dashboard View**: Use the `case` template variable to filter by test case
- **Insight**: A model might excel at simple math but struggle with creative writing
- **Decision Criteria**: Select models that perform well on your most critical use cases

#### 3. Quality Assessment
- **Action**: Monitor score metrics alongside performance
- **Dashboard View**: Score per Operation panel combined with Latency panels
- **Insight**: The score metric provides a quality indicator (customizable in `calculateScore` function)
- **Decision Criteria**: Don't sacrifice too much quality for speed; find the balance
- **Advanced**: Implement more sophisticated evaluation methods (see "Improve Quality Scoring" section)

#### 4. Resource Planning
- **Action**: Understand hardware requirements and costs
- **Dashboard View**: GPU Utilization and GPU Memory panels
- **Insight**: If a smaller model achieves acceptable quality with lower GPU utilization, it may be more cost-effective
- **Decision Criteria**: Calculate cost per request based on GPU usage and throughput
- **Formula**: `Cost per request = (GPU cost per hour × GPU utilization %) / (requests per hour)`

#### 5. Reliability Check
- **Action**: Verify model stability and success rates
- **Dashboard View**: Success Rate panel
- **Insight**: A fast model with low success rate is not production-ready
- **Decision Criteria**: Require 95%+ success rate for production use cases

#### 6. Iterative Refinement
- **Action**: Add your own test cases that match your actual use cases
- **Implementation**: Edit `testCases` in `bench_llm_test.go` to include representative prompts
- **Validation**: Re-run benchmarks and verify results in Grafana
- **Decision Criteria**: Model must perform well on your custom test cases, not just generic ones

### Example Decision Flow

```
1. Filter by your primary use case (e.g., case="code-generation")
2. Check Success Rate panel → Eliminate models with <95% success
3. Check Score panel → Identify models with score >0.7
4. Check Latency Percentiles → Find models with acceptable p95 latency
5. Check Tokens per Second → Ensure throughput meets requirements
6. Check GPU Memory → Verify model fits your hardware budget
7. Select the smallest model that passes all criteria
```

### Real-World Example

**Scenario**: Building a code assistant that needs to explain code snippets

**Requirements**:
- Latency p95 < 500ms
- Success rate > 98%
- Score > 0.75
- GPU memory < 4GB

**Process**:
1. Filter dashboard by `case="code-explanation"`
2. Qwen3 (0.6B): Fast (p95=200ms) but score=0.65 (too low)
3. Llama 3.2 1B: Moderate speed (p95=350ms), score=0.80 ✓
4. Llama 3.2 3B: Slow (p95=650ms) but high score=0.90 (too slow)
5. **Winner**: Llama 3.2 1B meets all requirements with smallest size

## Extending the Example

### Add External API Models (like OpenAI)

The benchmark supports external API models in addition to local models. To add more external models, edit the `getModelsToTest()` function in `bench_llm_test.go`.

External models:
- Skip the Docker Model Runner pull step
- Connect directly to the API endpoint
- Require appropriate API keys in environment variables
- Run first in the benchmark suite for quick comparison

### Add More Local Models

Edit the `localModels` slice in the `getModelsToTest()` function in `bench_llm_test.go`. Local models must be available in [Docker's GenAI catalog](https://hub.docker.com/catalogs/gen-ai).

### Add More Temperatures

Edit the `temperatures` slice in `bench_llm_test.go` (currently at line 71). Each test case will automatically run with all specified temperatures.

### Add More Test Cases

Edit the `testCases` slice in `bench_llm_test.go` (currently at line 42).

### Improve Quality Scoring

The `calculateScore` function in `bench_llm_test.go` (currently at line 243) uses a simple heuristic. You can improve it by:

- Implementing semantic similarity checks against expected outputs
- Using another LLM as an evaluator (Evaluator Agent pattern)
- Checking for specific keywords or patterns in responses
- Calculating BLEU, ROUGE, or other NLP metrics
- Adding task-specific validators for different test case types
