# Implementation Plan: Tool Calling + OpenTelemetry Callback Spans

## Goal
1. **Add tool calling functionality** to benchmarks (inspired by `10-functions` sibling project)
2. **Implement OTel callback spans** to trace tool execution paths in Grafana/Tempo
3. **Create new tool-assisted benchmark scenarios** to test multi-step reasoning

## Current State
- **Single flat span**: `llm.generate` wraps entire LLM request/response
- **No tool calling**: Models only do direct text generation
- **No langchaingo callbacks**: Observability handled manually via OTel wrappers
- **Two integration points**: Benchmark client (llmclient) and evaluator agent
- **Exemplar correlation**: Working well (trace_id/span_id linking metrics to traces)

## Expected Outcome: Span Hierarchy in Grafana

### Before (current):
```
llm.generate (2500ms)
  [attributes: model, prompts, tokens, latency, ttft]
```

### After (with tool calling + callbacks):
```
llm.generate (5500ms)
  ├─ langchaingo.llm.generate.start (2ms)
  │  [llm.messages.count, llm.message.role, llm.message.content]
  ├─ langchaingo.tool.start (1ms)
  │  [tool.name=calculator, tool.input={"operation":"multiply","a":15,"b":23}]
  ├─ langchaingo.tool.end (250ms)
  │  [tool.name=calculator, tool.output="345"]
  ├─ langchaingo.llm.generate.start (2ms)
  │  [continuation with tool result]
  ├─ langchaingo.tool.start (1ms)
  │  [tool.name=calculator, tool.input={"operation":"divide","a":48,"b":4}]
  ├─ langchaingo.tool.end (200ms)
  │  [tool.name=calculator, tool.output="12"]
  ├─ langchaingo.llm.generate.end (3ms)
  │  [final synthesis with all tool results]
  └─ langchaingo.llm.generate.end (3ms)
     [llm.response.content, llm.usage.*_tokens]
```

## Architecture Decision
**Use parent-child spans from incoming context** (not stateful tracking)
- Simpler, goroutine-safe, no state management
- Each callback creates a child span from the incoming context's active span
- Child spans have same parent (not sequential chains)

## Implementation Steps

### Phase A: Tool Calling Infrastructure

#### Step A1: Create Tools Package
**New directory**: `tools/`
**New files**:
- `tools/calculator.go` - Mathematical operations tool
- `tools/code_executor.go` - Safe Python code execution tool
- `tools/http_client.go` - HTTP client for external APIs (reusable, like 10-functions)

**Calculator tool** (`tools/calculator.go`):
- Operations: add, subtract, multiply, divide, power, sqrt, factorial
- Returns results as JSON
- Tool definition with schema for langchaingo

**Code executor tool** (`tools/code_executor.go`):
- Executes Python code in isolated container (using testcontainers `GenericContainer`)
- Base image: `python:3.12-alpine` (lightweight)
- Returns stdout/stderr
- Safety limits: timeout, memory, no network access

**HTTP client** (`tools/http_client.go`):
- Generic HTTP GET/POST client
- Used for external API calls (optional, for future expansion)

#### Step A2: Create New Test Cases with Evaluation Criteria
**File**: `bench_llm_test.go` (add to existing test cases array)

Add 3 new tool-assisted test cases:

1. **calculator-reasoning** (lines ~74-79):
   - System: "You are a helpful assistant with access to a calculator tool."
   - User: "Calculate (125 * 47) + (980 / 20) - 156. Break down each step and use the calculator tool for each operation. Then explain the final result."
   - Expected: Model calls calculator tool 4 times, then synthesizes answer
   - **Parameter validation**: Check that operations are called in correct order with correct arguments
   - Expected tool calls:
     1. `multiply(125, 47)` → 5875
     2. `divide(980, 20)` → 49
     3. `add(5875, 49)` → 5924
     4. `subtract(5924, 156)` → 5768

2. **code-validation** (lines ~80-85):
   - System: "You are a helpful coding assistant with access to a Python code executor."
   - User: "Write Python code to generate the first 10 Fibonacci numbers, then execute it to verify correctness."
   - Expected: Model generates code, calls executor, validates output
   - **Parameter validation**: Check that generated code is valid Python and produces Fibonacci sequence
   - Expected tool call: `execute_python(code_containing_fibonacci_logic)`

3. **api-data-retrieval** (lines ~86-91):
   - System: "You are a helpful assistant with access to web APIs."
   - User: "Use the HTTP client to fetch information about repository 'testcontainers-go' from GitHub API and summarize the key details."
   - Expected: Model calls HTTP client, processes JSON, synthesizes summary
   - **Parameter validation**: Check that URL is correct GitHub API endpoint
   - Expected tool call: `http_get("https://api.github.com/repos/testcontainers/testcontainers-go")`

**New directory**: `evaluator/testdata/evaluation/tool-parameter-extraction/`
**New files**:
- `calculator-reasoning/system_prompt.txt` - Evaluator instructions for parameter correctness
- `calculator-reasoning/reference.txt` - Expected tool calls and parameters
- `code-validation/system_prompt.txt`
- `code-validation/reference.txt`
- `api-data-retrieval/system_prompt.txt`
- `api-data-retrieval/reference.txt`

#### Step A3: Extend LLM Client with Tool Support
**File**: `llmclient/llmclient.go`

Add new method: `GenerateWithTools(ctx, testCase, systemPrompt, userPrompt, temp, tools, maxIterations)`:
- Similar to `GenerateWithTemp` but accepts `[]llms.Tool`
- Implements tool execution loop (like 10-functions pattern)
- Iterates until model provides final answer or reaches maxIterations
- Executes tool calls and feeds results back to model
- Returns aggregate metrics: total latency, tool call count, iterations

Add tool execution helper:
- `executeToolCall(ctx, toolCall) (string, error)` - Routes to correct tool implementation
- Tracks tool execution time separately from LLM time

### Phase B: OpenTelemetry Callback Spans

#### Step B1: Create Callback Handler Package
**New file**: `callbacks/otel_handler.go`

Implement `callbacks.Handler` interface with:
- **Tracer**: `otel.Tracer("langchaingo-callbacks")`
- **7 methods with spans** (focus on tool calling):
  - `HandleLLMGenerateContentStart` → span: `langchaingo.llm.generate.start`
  - `HandleLLMGenerateContentEnd` → span: `langchaingo.llm.generate.end`
  - `HandleLLMError` → span: `langchaingo.llm.error`
  - `HandleToolStart` → span: `langchaingo.tool.start` **← KEY FOR OBSERVABILITY**
  - `HandleToolEnd` → span: `langchaingo.tool.end` **← KEY FOR OBSERVABILITY**
  - `HandleToolError` → span: `langchaingo.tool.error` **← KEY FOR OBSERVABILITY**
  - `HandleStreamingFunc` → no-op (skip streaming chunks per user preference)
- **9 stub methods**: No-op for chains/agents/text/retriever
- **String truncation**: Helper to limit content to 500 chars (prevent span explosion)

**Key attributes per span**:
- LLM Start: `llm.messages.count`, `llm.message.role`, `llm.message.content` (truncated)
- LLM End: `llm.response.content` (truncated), `llm.usage.*_tokens` (from GenerationInfo)
- LLM Error: `error.type`, `error.message`
- **Tool Start**: `tool.name`, `tool.input` (JSON string, truncated) **← MOST IMPORTANT**
- **Tool End**: `tool.name`, `tool.output` (truncated), `tool.duration_ms` **← MOST IMPORTANT**
- **Tool Error**: `tool.name`, `error.type`, `error.message`

#### Step B2: Extend Semantic Conventions
**File**: `semconv/semconv.go` (append after line 90)

Add new attribute key constants:
```go
// Langchaingo Callback Spans
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
    AttrToolName        = "tool.name"
    AttrToolInput       = "tool.input"
    AttrToolOutput      = "tool.output"
    AttrToolDurationMs  = "tool.duration_ms"
    AttrToolCallID      = "tool.call_id"

    // Error attributes
    AttrErrorType    = "error.type"
    AttrErrorMessage = "error.message"
)

// Additional metric names for tool calling
const (
    MetricLLMToolCallCount          = "llm.tool_call.count"
    MetricLLMToolCallLatency        = "llm.tool_call.latency"
    MetricLLMIterationCount         = "llm.iteration.count"
    MetricLLMToolSuccessRate        = "llm.tool.success_rate"
    MetricLLMToolParamAccuracy      = "llm.tool.param_accuracy"      // NEW: Parameter extraction accuracy
    MetricLLMToolSelectionAccuracy  = "llm.tool.selection_accuracy"  // NEW: Correct tool selection rate
)
```

#### Step B3: Wire Handler into LLM Client
**File**: `llmclient/llmclient.go`

**Change 1** (line 11): Add import
```go
import (
    ...
    "github.com/mdelapenya/genai-testcontainers-go/benchmarks/callbacks"
    ...
)
```

**Change 2** (lines 52-56): Add callback option
```go
opts := []openai.Option{
    openai.WithBaseURL(endpoint),
    openai.WithModel(model),
    openai.WithToken(apiKey),
    openai.WithCallback(callbacks.NewOTelCallbackHandler()), // NEW
}
```

#### Step B4: Wire Handler into Evaluator
**File**: `bench_main_test.go`

**Change 1** (line 11): Add import
```go
import (
    ...
    "github.com/mdelapenya/genai-testcontainers-go/benchmarks/callbacks"
    ...
)
```

**Change 2**: Add callback to OpenAI evaluator (in `initializeEvaluatorAgent()`)
```go
return openai.New(
    openai.WithModel("gpt-4o-mini"),
    openai.WithToken(apiKey),
    openai.WithCallback(callbacks.NewOTelCallbackHandler()), // NEW
)
```

**Change 3**: Add callback to DMR evaluator (in `initializeEvaluatorAgent()`)
```go
return openai.New(
    openai.WithModel(evaluatorModel),
    openai.WithBaseURL(dmrEndpoint),
    openai.WithToken("dummy"),
    openai.WithCallback(callbacks.NewOTelCallbackHandler()), // NEW
)
```

### Phase C: Metrics & Benchmarking

#### Step C1: Add Tool-Specific Metrics
**File**: `metrics.go`

Add new histogram for tool call latency:
```go
toolCallLatency, _ := meter.Float64Histogram(
    semconv.MetricLLMToolCallLatency,
    metric.WithDescription("Tool call execution latency"),
    metric.WithUnit("ms"),
)
```

Add new gauges for tool metrics:
```go
// Tool call count per operation
toolCallCount, _ := meter.Float64ObservableGauge(
    semconv.MetricLLMToolCallCount,
    metric.WithDescription("Average tool calls per operation"),
)

// LLM-tool iteration count
iterationCount, _ := meter.Float64ObservableGauge(
    semconv.MetricLLMIterationCount,
    metric.WithDescription("Average LLM-tool iterations per operation"),
)

// Tool success rate
toolSuccessRate, _ := meter.Float64ObservableGauge(
    semconv.MetricLLMToolSuccessRate,
    metric.WithDescription("Tool call success rate"),
)

// Tool parameter extraction accuracy (NEW)
toolParamAccuracy, _ := meter.Float64ObservableGauge(
    semconv.MetricLLMToolParamAccuracy,
    metric.WithDescription("Tool parameter extraction accuracy (0.0-1.0)"),
)

// Tool selection accuracy (NEW)
toolSelectionAccuracy, _ := meter.Float64ObservableGauge(
    semconv.MetricLLMToolSelectionAccuracy,
    metric.WithDescription("Correct tool selection rate (0.0-1.0)"),
)
```

Add recording methods:
- `RecordToolCallLatency(ctx, latency, toolName, model, testCase, temp)`
- Track tool call counts and iterations in benchmark results
- **Track parameter extraction accuracy per test case**
- **Track tool selection accuracy per test case**

#### Step C2: Extend Benchmark Logic
**File**: `bench_llm_test.go`

Detect tool-assisted test cases and use `GenerateWithTools()` instead of `GenerateWithTemp()`:
```go
if isToolAssistedCase(tc.Name) {
    tools := getToolsForCase(tc.Name)
    result := runSingleBenchmarkWithTools(ctx, client, modelName, tc, temp, tools)
} else {
    result := runSingleBenchmark(ctx, client, modelName, tc, temp)
}
```

Add helper `runSingleBenchmarkWithTools()` that:
- Calls `client.GenerateWithTools()`
- Records tool call metrics
- Tracks tool execution latency separately
- **Evaluates tool parameter extraction accuracy** using evaluator agent
- **Compares actual tool calls vs expected tool calls** from evaluation criteria
- Calculates parameter accuracy score (0.0 = wrong params, 0.5 = partial, 1.0 = correct)
- Updates gauges with tool-specific stats

#### Step C3: Extend Evaluator for Tool Parameter Validation
**File**: `evaluator/evaluator.go`

Add new method: `EvaluateToolCalls(ctx, testCase, actualToolCalls, expectedToolCalls) (*ToolEvaluationResult, error)`:
- Compares actual tool calls against expected tool calls from criteria
- Checks:
  1. **Tool selection**: Did model choose correct tool?
  2. **Parameter correctness**: Are parameters accurate?
  3. **Parameter completeness**: Are all required parameters present?
  4. **Call sequence**: Are tools called in logical order?
- Returns scores for each dimension

Add new struct:
```go
type ToolEvaluationResult struct {
    ToolSelectionScore  float64 // 0.0-1.0: correct tool chosen
    ParameterAccuracy   float64 // 0.0-1.0: parameters match expected
    SequenceScore       float64 // 0.0-1.0: logical call order
    OverallScore        float64 // Average of above
    Reason              string  // Explanation
}
```

**File**: `evaluator/criteria.go` (NEW)

Add helper to load tool evaluation criteria:
- `GetToolEvaluationCriteria(testCase string) (*ToolCriteria, error)`
- Loads expected tool calls from `testdata/evaluation/tool-parameter-extraction/{test-case}/reference.txt`

## Testing & Verification in Grafana

### 1. Run Benchmarks
```bash
cd /Users/mdelapenya/sourcecode/src/github.com/mdelapenya/generative-ai-with-testcontainers/11-benchmarks
go test -bench=. -benchtime=3x
```

This will now run:
- **5 original test cases** (no tools) across 4-5 models, 5 temperatures = ~125 scenarios
- **3 new tool-assisted test cases** across 4-5 models, 5 temperatures = ~75 scenarios
- **Total**: ~200 benchmark scenarios

### 2. Access Grafana
- Get URL from terminal output
- Login: admin / admin

### 3. Explore Tool Calling Traces in Tempo
1. Navigate to **Explore** → Select **Tempo** datasource
2. Search: `service.name="llm-benchmark"`
3. Filter by tool-assisted cases:
   - `name="llm.generate" && case="calculator-reasoning"`
   - `name="llm.generate" && case="code-validation"`
   - `name="llm.generate" && case="api-data-retrieval"`
4. Click on a trace to expand

### 4. Verify Span Hierarchy (Focus: Tool Calling)

**For tool-assisted test cases, check for**:
- ✓ Parent `llm.generate` span (total duration)
- ✓ `langchaingo.llm.generate.start` (initial request)
- ✓ **`langchaingo.tool.start`** spans (one per tool call) **← KEY VERIFICATION**
  - Check `tool.name` attribute (calculator, code_executor, http_client)
  - Check `tool.input` attribute (JSON arguments)
- ✓ **`langchaingo.tool.end`** spans (one per tool call) **← KEY VERIFICATION**
  - Check `tool.name` matches tool.start
  - Check `tool.output` attribute (truncated result)
  - Check `tool.duration_ms` attribute (execution time)
- ✓ Multiple LLM generate.start/end pairs (for iterative tool loops)
- ✓ Final `langchaingo.llm.generate.end` (synthesis)

**Span attribute verification**:
- Tool start: `tool.name="calculator"`, `tool.input='{"operation":"multiply","a":125,"b":47}'`
- Tool end: `tool.name="calculator"`, `tool.output="5875"`, `tool.duration_ms=150`
- LLM spans: Same as before (messages, tokens, etc.)

**Example trace structure for calculator-reasoning**:
```
llm.generate (5500ms) [case=calculator-reasoning, model=llama3.2:3B]
  ├─ langchaingo.llm.generate.start (2ms)
  ├─ langchaingo.tool.start (1ms) [tool.name=calculator, input={...multiply...}]
  ├─ langchaingo.tool.end (200ms) [tool.name=calculator, output="5875"]
  ├─ langchaingo.llm.generate.start (2ms)
  ├─ langchaingo.tool.start (1ms) [tool.name=calculator, input={...divide...}]
  ├─ langchaingo.tool.end (180ms) [tool.name=calculator, output="49"]
  ├─ langchaingo.llm.generate.start (2ms)
  ├─ langchaingo.tool.start (1ms) [tool.name=calculator, input={...add...}]
  ├─ langchaingo.tool.end (190ms) [tool.name=calculator, output="5924"]
  ├─ langchaingo.llm.generate.start (2ms)
  ├─ langchaingo.tool.start (1ms) [tool.name=calculator, input={...subtract...}]
  ├─ langchaingo.tool.end (195ms) [tool.name=calculator, output="5768"]
  └─ langchaingo.llm.generate.end (3ms) [final synthesis]
```

### 5. Verify Tool Metrics (Including Parameter Accuracy)
1. In Grafana dashboard, check for new panels:
   - **Tool Call Latency** histogram (by tool name)
   - **Tool Calls per Operation** gauge (by test case)
   - **LLM-Tool Iterations** gauge (by model/case)
   - **Tool Success Rate** gauge
   - **Tool Parameter Accuracy** gauge (by model/case) **← NEW**
   - **Tool Selection Accuracy** gauge (by model/case) **← NEW**
2. Filter by `case="calculator-reasoning"` to see tool-specific metrics
3. **Check parameter accuracy**:
   - Models with higher scores correctly extract parameters
   - Compare accuracy across different model sizes (1B vs 3B)
   - Verify that temperature affects parameter extraction quality

### 6. Verify Exemplar Correlation Still Works
1. Click on tool call latency histogram point
2. Click "View trace" from exemplar link
3. Verify trace shows **all spans** (parent + LLM + tool spans)
4. Confirm you can drill down from metrics → traces → tool execution details

## Critical Files

### New Files (Phase A: Tool Calling)
1. **tools/calculator.go** (NEW) - Calculator tool with math operations
2. **tools/code_executor.go** (NEW) - Python code execution in container
3. **tools/http_client.go** (NEW) - HTTP client for API calls (optional)
4. **evaluator/criteria.go** (NEW) - Tool evaluation criteria loader
5. **evaluator/testdata/evaluation/tool-parameter-extraction/** (NEW) - Evaluation criteria files for parameter validation

### New Files (Phase B: Observability)
6. **callbacks/otel_handler.go** (NEW) - Callback handler with tool span creation

### Modified Files
7. **semconv/semconv.go** - Add tool & LLM attribute constants + parameter accuracy metrics
8. **llmclient/llmclient.go** - Add `GenerateWithTools()` method + wire callback handler
9. **bench_llm_test.go** - Add 3 tool-assisted test cases + tool routing logic + parameter validation
10. **bench_main_test.go** - Wire callback handler into evaluator
11. **evaluator/evaluator.go** - Add `EvaluateToolCalls()` method for parameter validation
12. **metrics.go** - Add tool call metrics (histogram + gauges) + parameter accuracy metrics
13. **grafana_dash.go** - Add 6 tool metrics panels to dashboard

## Benefits

### Tool Calling Functionality
✓ **Multi-step reasoning benchmarks**: Test how models break down complex tasks
✓ **Tool selection accuracy**: Measure which models choose correct tools
✓ **Iterative execution**: See how models loop through tool calls to gather information
✓ **Real-world scenarios**: Calculator, code execution, API calls mirror production use cases

### Observability Enhancements
✓ **Tool execution visibility**: See exact tool calls, inputs, outputs in Grafana traces
✓ **Performance breakdown**: Separate LLM time vs tool execution time
✓ **Tool call routing**: Understand decision-making process (which tool, when, why)
✓ **Error diagnostics**: Pinpoint tool failures vs LLM failures
✓ **Iteration tracking**: Count how many LLM-tool roundtrips needed
✓ **Metric-trace correlation**: Exemplars link tool metrics to detailed traces
✓ **Parameter extraction accuracy**: Evaluate if LLM correctly extracts tool parameters
✓ **Tool selection accuracy**: Measure if LLM chooses the right tool for the task
✓ **Quality benchmarking**: Compare models on tool calling capabilities (not just speed)

### Backward Compatibility
✓ **Zero breaking changes**: Original 5 test cases still run without tools
✓ **Conditional tool usage**: Only tool-assisted cases use new functionality
✓ **Evaluator transparency**: Quality assessment calls now traceable
✓ **Low overhead**: < 0.2% impact on LLM latency

## Rollback Plan

### Quick rollback (disable callbacks only):
Comment out 3 callback options:
1. `llmclient/llmclient.go` line 56
2. `bench_main_test.go` evaluator init (2 locations)

System reverts to single-span tracing but keeps tool calling.

### Full rollback (disable tool calling):
Comment out tool-assisted test cases in `bench_llm_test.go`:
- Remove or comment out calculator-reasoning, code-validation, api-data-retrieval

Original 5 test cases continue working normally.

## Dependencies

### Existing Dependencies (already in go.mod)
- `github.com/tmc/langchaingo v0.1.14` - Tool calling support via `llms.WithTools()`
- `go.opentelemetry.io/otel v1.36.0` - Tracing and metrics
- `testcontainers-go v0.40.0` - Container orchestration (use `GenericContainer` for Python executor)

### New Dependencies Required
- **None** - All functionality uses existing dependencies
- Code executor uses `GenericContainer` with `python:3.12-alpine` base image
- HTTP client uses Go standard library (`net/http`)

## Commit Strategy (Atomic Commits)

### Phase A: Tool Calling Infrastructure

**Commit 1: Add semantic conventions for tool calling**
- File: `semconv/semconv.go`
- Add tool attribute constants and metric names
- Message: `feat(semconv): add semantic conventions for tool calling`

**Commit 2: Add calculator tool**
- File: `tools/calculator.go`
- Implement calculator with math operations
- Message: `feat(tools): add calculator tool with basic math operations`

**Commit 3: Add code executor tool**
- File: `tools/code_executor.go`
- Implement Python code execution in GenericContainer
- Message: `feat(tools): add Python code executor tool with container isolation`

**Commit 4: Add HTTP client tool**
- File: `tools/http_client.go`
- Implement generic HTTP client for API calls
- Message: `feat(tools): add HTTP client tool for external API calls`

**Commit 5: Extend LLM client with tool support**
- File: `llmclient/llmclient.go`
- Add `GenerateWithTools()` method and tool execution loop
- Message: `feat(llmclient): add tool calling support with iterative execution`

**Commit 6: Add tool-assisted test cases**
- File: `bench_llm_test.go`
- Add calculator-reasoning, code-validation, api-data-retrieval test cases
- Add tool routing logic and `runSingleBenchmarkWithTools()` helper
- Message: `feat(benchmarks): add 3 tool-assisted test cases`

**Commit 6a: Add tool parameter evaluation criteria**
- Directory: `evaluator/testdata/evaluation/tool-parameter-extraction/`
- Files: System prompts and reference files for each test case
- Message: `feat(evaluator): add tool parameter extraction evaluation criteria`

**Commit 6b: Extend evaluator for tool parameter validation**
- Files: `evaluator/evaluator.go`, `evaluator/criteria.go` (NEW)
- Add `EvaluateToolCalls()` method and `ToolEvaluationResult` struct
- Message: `feat(evaluator): add tool parameter extraction validation`

### Phase B: OpenTelemetry Callback Spans

**Commit 7: Add OpenTelemetry callback handler package**
- File: `callbacks/otel_handler.go`
- Implement callbacks.Handler with tool span creation
- Message: `feat(callbacks): add OpenTelemetry callback handler for langchaingo`

**Commit 8: Wire callback handler into LLM client**
- File: `llmclient/llmclient.go`
- Add callback option to OpenAI client initialization
- Message: `feat(llmclient): wire OpenTelemetry callback handler`

**Commit 9: Wire callback handler into evaluator**
- File: `bench_main_test.go`
- Add callback option to evaluator LLM initialization (both OpenAI and DMR)
- Message: `feat(evaluator): wire OpenTelemetry callback handler`

### Phase C: Metrics & Observability

**Commit 10: Add tool call metrics**
- File: `metrics.go`
- Add tool call latency histogram and tool-specific gauges
- Add `RecordToolCallLatency()` method
- Message: `feat(metrics): add tool call latency and iteration metrics`

**Commit 10a: Add tool parameter accuracy metrics**
- File: `metrics.go`
- Add parameter accuracy and tool selection accuracy gauges
- Message: `feat(metrics): add tool parameter extraction accuracy metrics`

**Commit 11: Record tool metrics in benchmarks**
- File: `bench_llm_test.go`
- Update benchmark logic to record tool metrics
- Update gauge calculations for tool stats
- **Call evaluator to assess parameter extraction**
- Message: `feat(benchmarks): record tool call metrics during execution`

**Commit 12: Add Grafana dashboard panels for tools**
- File: `grafana_dash.go`
- Add 6 new panels: tool call latency, tool calls/op, iterations, success rate, parameter accuracy, selection accuracy
- Message: `feat(grafana): add dashboard panels for tool calling metrics`

### Total: 15 atomic commits

Each commit is self-contained, testable, and represents a single meaningful feature or change.

## Inspiration from 10-functions Sibling Project

Patterns borrowed:
- ✓ Iterative tool execution loop (keep calling until model provides final answer)
- ✓ Tool result feedback mechanism (append tool responses to message history)
- ✓ Tool definition with JSON schemas
- ✓ Error handling for tool failures

Improvements made:
- ✓ Multiple tools instead of single Pokemon API tool
- ✓ OpenTelemetry spans for tool observability (not in 10-functions)
- ✓ Metrics for tool performance (not in 10-functions)
- ✓ Benchmarking-focused scenarios (not just demo)
- ✓ Evaluator for tool-assisted responses
