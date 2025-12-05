# Accessing Logs in Grafana

This document explains how to access and query logs from the LLM benchmarking system in Grafana.

## Prerequisites

After running the benchmarks with `go test -v -run TestBenchmarkLLM`, the Grafana stack (LGTM) remains running so you can explore the logs and metrics.

## Accessing Grafana

1. **Find the Grafana URL**: When you run the benchmarks, the console output will display:
   ```
   📊 Grafana Observability Stack Ready
   =================================================
   URL: http://localhost:<port>
   Credentials: admin / admin
   =================================================
   ```

2. **Login**: Open the URL in your browser and log in with:
   - Username: `admin`
   - Password: `admin`

## Viewing Logs

### Option 1: Using Explore (Recommended)

The Explore view provides a powerful interface for querying logs in real-time.

1. Click **Explore** in the left sidebar (compass icon)
2. Select **Loki** as the data source from the dropdown at the top
3. Use LogQL queries to filter logs (see examples below)
4. Click on any log line to expand and view all OTLP attributes in detail

**Viewing Attributes in Grafana**:
- Expand a log line by clicking on it
- Attributes section shows all key-value pairs (model, temperature, tokens, etc.)
- Click on attribute values to add them as filters automatically

### Option 2: Using Dashboards

Logs can also be viewed in pre-built dashboards:

1. Click **Dashboards** in the left sidebar
2. Select the **LLM Bench (DMR + Testcontainers)** dashboard
3. Scroll to log panels (if configured)

## LogQL Query Examples

### Understanding OTLP Labels in Loki

All logs from this benchmark use OpenTelemetry Protocol (OTLP) and have these standard labels:

- **`service_name`**: Always `"llm-benchmark"` (set in `otel_setup.go`)
- **`instrumentation_scope_name`**: The logger name - either `"evaluator"` or `"llmclient"`
- **`level`**: Log severity - typically `"INFO"`

**Quick Query Pattern**:
```logql
{service_name="llm-benchmark", instrumentation_scope_name="<logger-name>"}
```

Or use text filters for simpler queries:
```logql
{service_name="llm-benchmark"} |= "<search-term>"
```

### View All Evaluator Logs

Query evaluator agent responses:

```logql
{service_name="llm-benchmark", instrumentation_scope_name="evaluator"}
```

Or simply filter by the logger name:

```logql
{service_name="llm-benchmark"} |= "evaluator"
```

This returns all evaluation logs including:
- Questions being evaluated
- Answers provided by models
- Evaluator's assessment (yes/no/unsure)
- Reasoning for the evaluation
- Score (0.0 to 1.0)

### View All Model Response Logs

Query model/LLM client responses:

```logql
{service_name="llm-benchmark", instrumentation_scope_name="llmclient"}
```

Or simply:

```logql
{service_name="llm-benchmark"} |= "llmclient"
```

This returns all model response logs including:
- Model name
- System and user prompts
- Temperature settings
- Response content
- Token usage (prompt, completion, total)
- Latency and TTFT metrics

### Filter by Severity

View only INFO level logs:

```logql
{service_name="llm-benchmark", level="INFO"}
```

### Filter by Specific Model

View logs for a specific model (e.g., llama3.2):

```logql
{service_name="llm-benchmark", instrumentation_scope_name="llmclient"} |= "llama3.2"
```

### Filter by Test Case

View logs for a specific test case (e.g., mathematical-operations):

```logql
{service_name="llm-benchmark"} |= "mathematical-operations"
```

Or filter evaluator logs for a specific test case:

```logql
{service_name="llm-benchmark", instrumentation_scope_name="evaluator"} |= `"test_case":"mathematical-operations"`
```

### Filter by Score Range

View evaluator logs where score is 1.0 (perfect):

```logql
{service_name="llm-benchmark", instrumentation_scope_name="evaluator"} |= "score" |= "1"
```

View evaluator logs where score is less than 0.5:

```logql
{service_name="llm-benchmark", instrumentation_scope_name="evaluator"} |= "score" |= "0."
```

**Note**: OTLP attributes in Loki are stored as structured data. Use `|=` for text matching or parse with `| logfmt` if needed.

### Filter by Response Type

View only "yes" evaluations:

```logql
{service_name="llm-benchmark", instrumentation_scope_name="evaluator"} |= `"response":"yes"`
```

View "no" or "unsure" evaluations:

```logql
{service_name="llm-benchmark", instrumentation_scope_name="evaluator"} |~ `"response":"(no|unsure)"`
```

### Time Range Queries

View logs from the last 15 minutes:

```logql
{service_name="llm-benchmark", instrumentation_scope_name="evaluator"} [15m]
```

### Combined Filters

View llama3.2 model responses with high latency (>1000ms):

```logql
{service_name="llm-benchmark", instrumentation_scope_name="llmclient"} |= "llama3.2" |= "latency_ms"
```

View failed evaluations (score 0.0) with reasoning:

```logql
{service_name="llm-benchmark", instrumentation_scope_name="evaluator"} |= `"score":0` |= "reason"
```

## Understanding Log Attributes

**Important**: Logs are sent via OpenTelemetry Protocol (OTLP) to Loki. Attributes are embedded in the log structure. When querying:
- Use `|=` for exact text matching within logs
- Use `|~` for regex pattern matching
- Attributes appear in the log body and can be viewed in Grafana's log details

### Evaluator Log Attributes

Each evaluator log entry contains these attributes (accessible in Grafana UI):

| Attribute | Type | Description |
|-----------|------|-------------|
| `test_case` | string | Test case name (e.g., "mathematical-operations", "code-explanation") |
| `question` | string | The question (truncated to 100 chars) |
| `answer` | string | The answer being evaluated (truncated to 200 chars) |
| `provided_answer` | string | Evaluator's summary of the answer |
| `response` | string | Evaluation result: "yes", "no", or "unsure" |
| `reason` | string | Explanation for the evaluation |
| `score` | float64 | Numeric score (1.0 = yes, 0.5 = unsure, 0.0 = no) |

Example OTLP log structure (as seen in Grafana):
```
Body: "Evaluator response"
Severity: INFO
Attributes:
  test_case: "factual-question"
  question: "What is the capital of France?"
  answer: "The capital of France is Paris, a city known for..."
  provided_answer: "Paris"
  response: "yes"
  reason: "The answer correctly identifies Paris as the capital of France"
  score: 1.0
```

These attribute names exactly match the field names in `evaluator/evaluator.go:104-110`.

### Model Response Log Attributes

Each model response log entry contains:

| Attribute | Type | Description |
|-----------|------|-------------|
| `test_case` | string | Test case name (e.g., "code-generation", "creative-writing") |
| `model` | string | Model name/identifier |
| `system_prompt` | string | System instruction (truncated to 100 chars) |
| `user_prompt` | string | User input (truncated to 200 chars) |
| `temperature` | float64 | Temperature parameter used |
| `response_content` | string | Model's output (truncated to 500 chars) |
| `prompt_tokens` | int | Input token count |
| `completion_tokens` | int | Output token count |
| `total_tokens` | int | Sum of input + output tokens |
| `latency_ms` | int64 | Total response time in milliseconds |
| `ttft_ms` | int64 | Time To First Token in milliseconds |

Example OTLP log structure (as seen in Grafana):
```
Body: "Model response"
Severity: INFO
Attributes:
  test_case: "code-explanation"
  model: "ai/llama3.2:3B-Q4_K_M"
  system_prompt: "You are a helpful assistant."
  user_prompt: "What is the capital of France?"
  temperature: 0.7
  response_content: "The capital of France is Paris."
  prompt_tokens: 24
  completion_tokens: 8
  total_tokens: 32
  latency_ms: 1250
  ttft_ms: 180
```

These attribute names exactly match the field names in `llmclient/llmclient.go:211-223`.

## Advanced Log Analysis

### Create Log Panels in Dashboards

You can add log panels to your custom dashboards:

1. Go to **Dashboards** → **New Dashboard**
2. Click **Add visualization**
3. Select **Loki** as the data source
4. Enter your LogQL query
5. Choose visualization type (Logs, Table, etc.)
6. Save the panel

### Export Logs

To export logs for analysis:

1. Run your query in Explore
2. Click **Inspector** at the top
3. Select **Data** tab
4. Click **Download as CSV** or **Download as JSON**

### Set Up Log Alerts

You can create alerts based on log patterns:

1. Go to **Alerting** → **Alert rules**
2. Click **New alert rule**
3. Select **Loki** as the data source
4. Define your LogQL query (e.g., high error rates)
5. Set thresholds and notification channels

## Correlating Logs with Traces and Metrics

The benchmark system uses OpenTelemetry to provide full observability:

### Trace Correlation

Each log entry contains a trace context. To view the full trace:

1. In Explore, view a log entry
2. Look for the `trace_id` field
3. Click **Tempo** in the data source selector
4. Enter the trace ID to view the full request trace

### Metric Correlation

Metrics are collected in parallel with logs:

1. In the **LLM Bench** dashboard, view metric panels
2. Use the time picker to align with your log queries
3. Compare latency metrics with log entries to identify patterns

## Common Log Queries for Troubleshooting

### Find Slow Model Responses

```logql
{service_name="llm-benchmark", instrumentation_scope_name="llmclient"} |= "latency_ms"
```

Then filter visually in Grafana, or use text search:

```logql
{service_name="llm-benchmark", instrumentation_scope_name="llmclient"} |~ `"latency_ms":\s*[5-9]\d{3}|[1-9]\d{4,}`
```

### Find Low-Scoring Evaluations

```logql
{service_name="llm-benchmark", instrumentation_scope_name="evaluator"} |= `"score":0`
```

### View All Models and Token Usage

```logql
{service_name="llm-benchmark", instrumentation_scope_name="llmclient"} |= "model" |= "total_tokens"
```

### Find High-Temperature Queries

```logql
{service_name="llm-benchmark", instrumentation_scope_name="llmclient"} |~ `"temperature":\s*0\.9`
```

## Tips for Effective Log Exploration

1. **Use Time Range Filters**: Narrow down your search to specific time windows
2. **Combine Filters**: Chain multiple `|=` or `|~` operators to refine results
3. **Text Matching**: Use `|=` for exact text and `|~` for regex patterns
4. **OTLP Attributes**: Attributes are in the log body - view them in Grafana's log details panel
5. **Visual Filtering**: Use Grafana UI to click on attribute values for quick filtering
6. **Rate Functions**: Use `rate()` to calculate log volumes over time

**OTLP-Specific Tips**:
- Logs include OpenTelemetry resource attributes (service name, version, etc.)
- Each log has a severity level (INFO, WARN, ERROR)
- Trace context is embedded for correlation with traces in Tempo

## Troubleshooting

### No Logs Appearing

1. Verify containers are running:
   ```bash
   docker ps --filter label=org.testcontainers.session-id
   ```

2. Check OTLP endpoint is accessible:
   ```bash
   docker logs <lgtm-container-id>
   ```

3. Ensure logs are being emitted (check console output during benchmark run)

### Logs Not Persisted

The LGTM container stores logs in memory by default. For persistent storage:

1. Configure Loki with persistent volume mounts
2. Modify the container startup in `bench_main_test.go`

### Query Performance Issues

If queries are slow:

1. Reduce time range
2. Add more specific filters
3. Use indexed fields (job, severity)
4. Consider aggregating logs with `sum by` or `count by`

## Additional Resources

- [Loki LogQL Documentation](https://grafana.com/docs/loki/latest/logql/)
- [Grafana Explore Documentation](https://grafana.com/docs/grafana/latest/explore/)
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/instrumentation/go/)

## Implementation Details

The logging implementation can be found in:

- **Evaluator Logging**: `evaluator/evaluator.go:98-111`
- **Model Response Logging**: `llmclient/llmclient.go:199-216`
- **OTLP Setup**: `otel_setup.go`
- **Container Setup**: `bench_main_test.go`
