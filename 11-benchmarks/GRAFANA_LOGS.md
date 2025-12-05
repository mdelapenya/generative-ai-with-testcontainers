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

### Option 2: Using Dashboards

Logs can also be viewed in pre-built dashboards:

1. Click **Dashboards** in the left sidebar
2. Select the **LLM Bench (DMR + Testcontainers)** dashboard
3. Scroll to log panels (if configured)

## LogQL Query Examples

### View All Evaluator Logs

Query evaluator agent responses:

```logql
{job="evaluator"}
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
{job="llmclient"}
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
{severity="info"}
```

### Filter by Specific Model

View logs for a specific model (e.g., llama3.2):

```logql
{job="llmclient"} |= "llama3.2"
```

### Filter by Score Range

View evaluator logs where score is 1.0 (perfect):

```logql
{job="evaluator"} | json | score="1.0"
```

View evaluator logs where score is less than 0.5:

```logql
{job="evaluator"} | json | score < 0.5
```

### Filter by Response Type

View only "yes" evaluations:

```logql
{job="evaluator"} | json | response="yes"
```

View "no" or "unsure" evaluations:

```logql
{job="evaluator"} | json | response=~"no|unsure"
```

### Time Range Queries

View logs from the last 15 minutes:

```logql
{job="evaluator"} [15m]
```

### Combined Filters

View llama3.2 model responses with high latency (>1000ms):

```logql
{job="llmclient"} | json | model="llama3.2" | latency_ms > 1000
```

View failed evaluations (score 0.0) with reasoning:

```logql
{job="evaluator"} | json | score="0.0" | line_format "{{.reason}}"
```

## Understanding Log Attributes

### Evaluator Log Attributes

Each evaluator log entry contains:

| Attribute | Type | Description |
|-----------|------|-------------|
| `question` | string | The question (truncated to 100 chars) |
| `answer` | string | The answer being evaluated (truncated to 200 chars) |
| `provided_answer` | string | Evaluator's summary of the answer |
| `response` | string | Evaluation result: "yes", "no", or "unsure" |
| `reason` | string | Explanation for the evaluation |
| `score` | float64 | Numeric score (1.0 = yes, 0.5 = unsure, 0.0 = no) |

Example log entry:
```json
{
  "question": "What is the capital of France?",
  "answer": "The capital of France is Paris, a city known for...",
  "provided_answer": "Paris",
  "response": "yes",
  "reason": "The answer correctly identifies Paris as the capital of France",
  "score": 1.0
}
```

### Model Response Log Attributes

Each model response log entry contains:

| Attribute | Type | Description |
|-----------|------|-------------|
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

Example log entry:
```json
{
  "model": "ai/llama3.2:3B-Q4_K_M",
  "system_prompt": "You are a helpful assistant.",
  "user_prompt": "What is the capital of France?",
  "temperature": 0.7,
  "response_content": "The capital of France is Paris.",
  "prompt_tokens": 24,
  "completion_tokens": 8,
  "total_tokens": 32,
  "latency_ms": 1250,
  "ttft_ms": 180
}
```

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
{job="llmclient"} | json | latency_ms > 5000
```

### Find Low-Scoring Evaluations

```logql
{job="evaluator"} | json | score < 0.5
```

### Compare Models by Token Usage

```logql
{job="llmclient"} | json | line_format "{{.model}}: {{.total_tokens}} tokens"
```

### Find High-Temperature Queries

```logql
{job="llmclient"} | json | temperature > 0.9
```

## Tips for Effective Log Exploration

1. **Use Time Range Filters**: Narrow down your search to specific time windows
2. **Combine Filters**: Use multiple `|` operators to refine results
3. **Use Pattern Matching**: Use `|~` for regex patterns and `|=` for exact matches
4. **JSON Extraction**: Use `| json` to parse structured log attributes
5. **Line Formatting**: Use `| line_format` to customize output display
6. **Rate Functions**: Use `rate()` to calculate log volumes over time

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
