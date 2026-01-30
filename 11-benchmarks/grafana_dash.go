package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mdelapenya/genai-testcontainers-go/benchmarks/semconv"
)

// createPercentilePanelWithLinks creates a timeseries panel showing p50 and p95 percentiles with optional data links
func createPercentilePanelWithLinks(id int, title string, p50Metric, p95Metric string, x, y int, unit string, dataLinks []map[string]interface{}) map[string]interface{} {
	panel := map[string]interface{}{
		"id":      id,
		"type":    "timeseries",
		"title":   title,
		"gridPos": map[string]int{"x": x, "y": y, "w": 12, "h": 8},
		"targets": []map[string]interface{}{
			{
				"expr": fmt.Sprintf("%s{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}",
					p50Metric, semconv.AttrModel, semconv.AttrModel, semconv.AttrCase, semconv.AttrCase, semconv.AttrTemp, semconv.AttrTemp),
				"legendFormat": fmt.Sprintf("p50 - {{%s}} - {{%s}} (T={{%s}})", semconv.AttrModel, semconv.AttrCase, semconv.AttrTemp),
				"datasource": map[string]interface{}{
					"type": "prometheus",
					"uid":  "prometheus",
				},
				"refId": "A",
			},
			{
				"expr": fmt.Sprintf("%s{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}",
					p95Metric, semconv.AttrModel, semconv.AttrModel, semconv.AttrCase, semconv.AttrCase, semconv.AttrTemp, semconv.AttrTemp),
				"legendFormat": fmt.Sprintf("p95 - {{%s}} - {{%s}} (T={{%s}})", semconv.AttrModel, semconv.AttrCase, semconv.AttrTemp),
				"datasource": map[string]interface{}{
					"type": "prometheus",
					"uid":  "prometheus",
				},
				"refId": "B",
			},
		},
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"unit": unit,
			},
		},
	}

	// Add data links if provided
	if len(dataLinks) > 0 {
		defaults := panel["fieldConfig"].(map[string]interface{})["defaults"].(map[string]interface{})
		defaults["links"] = dataLinks
	}

	return panel
}

// createHistogramPanelWithLinks creates a histogram panel with exemplar support and optional data links
func createHistogramPanelWithLinks(id int, title string, bucketMetric string, x, y int, unit string, dataLinks []map[string]interface{}) map[string]interface{} {
	panel := map[string]interface{}{
		"id":      id,
		"type":    "histogram",
		"title":   title,
		"gridPos": map[string]int{"x": x, "y": y, "w": 12, "h": 8},
		"targets": []map[string]interface{}{
			{
				"expr": fmt.Sprintf("rate(%s_bucket{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}[5m])",
					bucketMetric, semconv.AttrModel, semconv.AttrModel, semconv.AttrCase, semconv.AttrCase, semconv.AttrTemp, semconv.AttrTemp),
				"legendFormat": fmt.Sprintf("{{%s}} - {{%s}} (T={{%s}}) - {{le}}", semconv.AttrModel, semconv.AttrCase, semconv.AttrTemp),
				"datasource": map[string]interface{}{
					"type": "prometheus",
					"uid":  "prometheus",
				},
				"refId":    "A",
				"exemplar": true,
			},
		},
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"unit": unit,
			},
		},
		"options": map[string]interface{}{
			"legend": map[string]interface{}{
				"displayMode": "list",
				"placement":   "bottom",
			},
		},
	}

	// Add data links if provided
	if len(dataLinks) > 0 {
		defaults := panel["fieldConfig"].(map[string]interface{})["defaults"].(map[string]interface{})
		defaults["links"] = dataLinks
	}

	return panel
}

// createSimpleTimeseriesPanelWithLinks creates a basic timeseries panel with a single metric and optional data links
func createSimpleTimeseriesPanelWithLinks(id int, title string, metric string, x, y, w, h int, unit string, minMax map[string]interface{}, dataLinks []map[string]interface{}) map[string]interface{} {
	panel := map[string]interface{}{
		"id":      id,
		"type":    "timeseries",
		"title":   title,
		"gridPos": map[string]int{"x": x, "y": y, "w": w, "h": h},
		"targets": []map[string]interface{}{
			{
				"expr": fmt.Sprintf("%s{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}",
					metric, semconv.AttrModel, semconv.AttrModel, semconv.AttrCase, semconv.AttrCase, semconv.AttrTemp, semconv.AttrTemp),
				"legendFormat": fmt.Sprintf("{{%s}} - {{%s}} (T={{%s}})", semconv.AttrModel, semconv.AttrCase, semconv.AttrTemp),
				"datasource": map[string]interface{}{
					"type": "prometheus",
					"uid":  "prometheus",
				},
				"refId": "A",
			},
		},
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"unit": unit,
			},
		},
	}

	// Add min/max if provided
	if minMax != nil {
		defaults := panel["fieldConfig"].(map[string]interface{})["defaults"].(map[string]interface{})
		for k, v := range minMax {
			defaults[k] = v
		}
	}

	// Add data links if provided
	if len(dataLinks) > 0 {
		defaults := panel["fieldConfig"].(map[string]interface{})["defaults"].(map[string]interface{})
		defaults["links"] = dataLinks
	}

	return panel
}

// Data link helper functions

// LinkFunc is a function that returns data links for a panel
type LinkFunc func() []map[string]interface{}

// combineLinks combines multiple link functions into a single slice of data links
// This allows panels to have links to logs, metrics, and (future) traces
func combineLinks(linkFuncs ...LinkFunc) []map[string]interface{} {
	var result []map[string]interface{}
	for _, fn := range linkFuncs {
		result = append(result, fn()...)
	}
	return result
}

// buildMetricsDrilldownLink creates a Grafana Metrics Drilldown data link for Prometheus metrics
func buildMetricsDrilldownLink(title string) []map[string]interface{} {
	// Build URL with filters for model, case, and temp
	// Note: ${__field.labels.*} variables are interpolated by Grafana at click time
	url := `/a/grafana-metricsdrilldown-app/drilldown?from=$__from&to=$__to&timezone=browser&var-ds=prometheus&var-filters=model|=|${__field.labels.model}&var-filters=case|=|${__field.labels.case}&var-filters=temp|=|${__field.labels.temp}`

	return []map[string]interface{}{
		{
			"title": title,
			"url":   url,
		},
	}
}

// metricsLink creates a data link to Prometheus metrics drilldown filtered by model, case, and temp
func metricsLink() []map[string]interface{} {
	return buildMetricsDrilldownLink("View Metrics (${__field.labels.model} - ${__field.labels.case} - T=${__field.labels.temp})")
}

// buildTracesExploreLink creates a Grafana Traces Explore data link for Tempo traces
func buildTracesExploreLink(title string) []map[string]interface{} {
	// Build URL with time range only - filters via var-filters cause TraceQL syntax errors
	// Note: ${__field.labels.*} variables are interpolated by Grafana at click time
	url := `/a/grafana-exploretraces-app/explore?from=$__from&to=$__to&timezone=browser&var-ds=tempo&var-metric=rate&var-groupBy=resource.service.name&actionView=breakdown`

	return []map[string]interface{}{
		{
			"title": title,
			"url":   url,
		},
	}
}

// tracesLink creates a data link to Tempo traces explore filtered by model, case, and temp
func tracesLink() []map[string]interface{} {
	return buildTracesExploreLink("View Traces (${__field.labels.model} - ${__field.labels.case} - T=${__field.labels.temp})")
}

// buildLokiExploreLink creates a Grafana Explore data link for Loki logs
// - title: Link title shown in the panel context menu
// - logBodyFilter: The string to filter log bodies (e.g., "Model response", "Evaluator response")
// - testCaseField: The field name for test_case in the log (usually "test_case", but "case" for benchmark errors)
// - tempField: The field name for temperature in the log (usually "temperature", but "temp" for benchmark errors)
func buildLokiExploreLink(title, logBodyFilter, testCaseField, tempField string) []map[string]interface{} {
	// Build LogQL query with filters for model, test_case, and temperature
	// Note: ${__field.labels.*} variables are interpolated by Grafana at click time
	query := fmt.Sprintf(`{service_name=\"llm-benchmark\"} |= \"%s\" | json | model=\"${__field.labels.model}\" | %s=\"${__field.labels.case}\" | %s=\"${__field.labels.temp}\"`,
		logBodyFilter, testCaseField, tempField)

	url := fmt.Sprintf(`/explore?orgId=1&schemaVersion=1&panes={"lnk":{"datasource":"loki","queries":[{"refId":"A","expr":"%s","queryType":"range"}],"range":{"from":"$__from","to":"$__to"}}}`,
		query)

	return []map[string]interface{}{
		{
			"title": title,
			"url":   url,
		},
	}
}

// llmClientLogLink creates a data link to llmclient logs filtered by model, test_case, and temperature
func llmClientLogLink() []map[string]interface{} {
	return buildLokiExploreLink(
		"View Model Responses (${__field.labels.model} - ${__field.labels.case} - T=${__field.labels.temp})",
		"Model response",
		"test_case",
		"temperature",
	)
}

// evaluatorLogLink creates a data link to evaluator logs filtered by model, test_case, and temperature
func evaluatorLogLink() []map[string]interface{} {
	return buildLokiExploreLink(
		"View Individual Evaluations (${__field.labels.model} - ${__field.labels.case} - T=${__field.labels.temp})",
		"Evaluator response",
		"test_case",
		"temperature",
	)
}

// toolEvaluatorLogLink creates a data link to tool evaluation logs filtered by model, test_case, and temperature
func toolEvaluatorLogLink() []map[string]interface{} {
	return buildLokiExploreLink(
		"View Tool Evaluations (${__field.labels.model} - ${__field.labels.case} - T=${__field.labels.temp})",
		"Tool evaluation response",
		"test_case",
		"temperature",
	)
}

// benchmarkErrorLogLink creates a data link to benchmark error logs filtered by model, test_case, and temperature
func benchmarkErrorLogLink() []map[string]interface{} {
	return buildLokiExploreLink(
		"View Errors (${__field.labels.model} - ${__field.labels.case} - T=${__field.labels.temp})",
		"error",
		"case", // benchmark errors use "case" instead of "test_case"
		"temp", // benchmark errors use "temp" instead of "temperature"
	)
}

// CreateGrafanaDashboard creates a Grafana dashboard for LLM benchmarks
// Uses a fixed UID to ensure the same dashboard is replaced on each run (no duplicates)
func CreateGrafanaDashboard(grafanaEndpoint, dashboardTitle string) error {
	// Ensure the endpoint has a scheme
	if !strings.HasPrefix(grafanaEndpoint, "http://") && !strings.HasPrefix(grafanaEndpoint, "https://") {
		grafanaEndpoint = "http://" + grafanaEndpoint
	}

	// Convert OTel metric names to Prometheus format
	promLatencyP50 := semconv.ToPrometheusMetricName(semconv.MetricLLMLatencyP50)
	promLatencyP95 := semconv.ToPrometheusMetricName(semconv.MetricLLMLatencyP95)
	promLatency := semconv.ToPrometheusMetricName(semconv.MetricLLMLatency)
	promTTFTP50 := semconv.ToPrometheusMetricName(semconv.MetricLLMTTFTP50)
	promTTFTP95 := semconv.ToPrometheusMetricName(semconv.MetricLLMTTFTP95)
	promTTFT := semconv.ToPrometheusMetricName(semconv.MetricLLMTTFT)
	promPromptEvalTimeP50 := semconv.ToPrometheusMetricName(semconv.MetricLLMPromptEvalTimeP50)
	promPromptEvalTimeP95 := semconv.ToPrometheusMetricName(semconv.MetricLLMPromptEvalTimeP95)
	promPromptEvalTime := semconv.ToPrometheusMetricName(semconv.MetricLLMPromptEvalTime)
	promTokensPerOp := semconv.ToPrometheusMetricName(semconv.MetricLLMTokensPerOp)
	promSuccessRate := semconv.ToPrometheusMetricName(semconv.MetricLLMSuccessRate)
	promTokensPerSecond := semconv.ToPrometheusMetricName(semconv.MetricLLMTokensPerSecond)
	promNsPerOp := semconv.ToPrometheusMetricName(semconv.MetricLLMNsPerOp)
	promGPUUtilization := semconv.ToPrometheusMetricName(semconv.MetricGPUUtilization)
	promGPUMemory := semconv.ToPrometheusMetricName(semconv.MetricGPUMemory)
	promEvalScore := semconv.ToPrometheusMetricName(semconv.MetricLLMEvalScore)
	promEvalPassRate := semconv.ToPrometheusMetricName(semconv.MetricLLMEvalPassRate)
	// Tool calling metrics
	promToolCallLatency := semconv.ToPrometheusMetricName(semconv.MetricLLMToolCallLatency)
	promToolCallCount := semconv.ToPrometheusMetricName(semconv.MetricLLMToolCallCount)
	promToolIterationCount := semconv.ToPrometheusMetricName(semconv.MetricLLMIterationCount)
	promToolSuccessRate := semconv.ToPrometheusMetricName(semconv.MetricLLMToolSuccessRate)
	promToolParamAccuracy := semconv.ToPrometheusMetricName(semconv.MetricLLMToolParamAccuracy)
	promToolSelectionAccuracy := semconv.ToPrometheusMetricName(semconv.MetricLLMToolSelectionAccuracy)
	promToolConvergence := semconv.ToPrometheusMetricName(semconv.MetricLLMToolConvergence)

	dashboard := map[string]interface{}{
		"dashboard": map[string]interface{}{
			"uid":           "llm-bench-dmr-tc", // Fixed UID ensures we replace the same dashboard
			"title":         dashboardTitle,
			"tags":          []string{"llm", "benchmark", "testcontainers"},
			"timezone":      "browser",
			"schemaVersion": 16,
			"version":       0,
			"refresh":       "5s",
			"templating": map[string]interface{}{
				"list": []map[string]interface{}{
					{
						"name":       semconv.AttrModel,
						"label":      "Model",
						"type":       "query",
						"query":      fmt.Sprintf("label_values(%s, %s)", promLatencyP50, semconv.AttrModel),
						"definition": fmt.Sprintf("label_values(%s, %s)", promLatencyP50, semconv.AttrModel),
						"datasource": map[string]interface{}{
							"type": "prometheus",
							"uid":  "prometheus",
						},
						"refresh": 1,
						"current": map[string]interface{}{
							"selected": false,
							"text":     "All",
							"value":    "$__all",
						},
						"multi":      true,
						"includeAll": true,
						"allValue":   ".*",
					},
					{
						"name":       semconv.AttrCase,
						"label":      "Test Case",
						"type":       "query",
						"query":      fmt.Sprintf("label_values(%s, %s)", promLatencyP50, semconv.AttrCase),
						"definition": fmt.Sprintf("label_values(%s, %s)", promLatencyP50, semconv.AttrCase),
						"datasource": map[string]interface{}{
							"type": "prometheus",
							"uid":  "prometheus",
						},
						"refresh": 1,
						"current": map[string]interface{}{
							"selected": false,
							"text":     "All",
							"value":    "$__all",
						},
						"multi":      true,
						"includeAll": true,
						"allValue":   ".*",
					},
					{
						"name":       semconv.AttrTemp,
						"label":      "Temperature",
						"type":       "query",
						"query":      fmt.Sprintf("label_values(%s, %s)", promLatencyP50, semconv.AttrTemp),
						"definition": fmt.Sprintf("label_values(%s, %s)", promLatencyP50, semconv.AttrTemp),
						"datasource": map[string]interface{}{
							"type": "prometheus",
							"uid":  "prometheus",
						},
						"refresh": 1,
						"current": map[string]interface{}{
							"selected": false,
							"text":     "All",
							"value":    "$__all",
						},
						"multi":      true,
						"includeAll": true,
						"allValue":   ".*",
					},
				},
			},
			"panels": []map[string]interface{}{
				// Latency metrics
				createPercentilePanelWithLinks(1, "Latency Percentiles (p50/p95)", promLatencyP50, promLatencyP95, 0, 0, "ms", combineLinks(llmClientLogLink, metricsLink, tracesLink)),
				createHistogramPanelWithLinks(2, "Latency Distribution (with Exemplars)", promLatency, 12, 0, "ms", combineLinks(llmClientLogLink, metricsLink, tracesLink)),

				// TTFT metrics
				createPercentilePanelWithLinks(3, "TTFT Percentiles (p50/p95)", promTTFTP50, promTTFTP95, 0, 8, "ms", combineLinks(llmClientLogLink, metricsLink, tracesLink)),
				createHistogramPanelWithLinks(4, "TTFT Distribution (with Exemplars)", promTTFT, 12, 8, "ms", combineLinks(llmClientLogLink, metricsLink, tracesLink)),

				// Prompt Evaluation Time metrics
				createPercentilePanelWithLinks(5, "Prompt Evaluation Time (p50/p95)", promPromptEvalTimeP50, promPromptEvalTimeP95, 0, 16, "ms", combineLinks(llmClientLogLink, metricsLink, tracesLink)),
				createHistogramPanelWithLinks(6, "Prompt Eval Time Distribution (with Exemplars)", promPromptEvalTime, 12, 16, "ms", combineLinks(llmClientLogLink, metricsLink, tracesLink)),

				// Other metrics
				createSimpleTimeseriesPanelWithLinks(7, "Tokens per Operation", promTokensPerOp, 0, 24, 8, 8, "short", nil, combineLinks(llmClientLogLink, metricsLink, tracesLink)),
				createSimpleTimeseriesPanelWithLinks(8, "Success Rate", promSuccessRate, 8, 24, 8, 8, "percentunit", map[string]interface{}{"min": 0, "max": 1}, combineLinks(benchmarkErrorLogLink, metricsLink, tracesLink)),
				createSimpleTimeseriesPanelWithLinks(9, "Tokens per Second", promTokensPerSecond, 16, 24, 8, 8, "short", nil, combineLinks(llmClientLogLink, metricsLink, tracesLink)),

				// GPU metrics
				createSimpleTimeseriesPanelWithLinks(10, "GPU Utilization", promGPUUtilization, 0, 32, 12, 8, "percent", map[string]interface{}{"min": 0, "max": 100}, combineLinks(llmClientLogLink, metricsLink, tracesLink)),
				createSimpleTimeseriesPanelWithLinks(11, "GPU Memory Usage", promGPUMemory, 12, 32, 12, 8, "decmbytes", nil, combineLinks(llmClientLogLink, metricsLink, tracesLink)),

				// Evaluator metrics with data links to Loki logs
				// IMPORTANT: These metrics show aggregated average scores calculated from multiple benchmark iterations.
				// Each data point represents the mean evaluator score across all iterations for that model/test_case combination,
				// collected over the metric export interval (5s). Clicking a point shows individual evaluation logs within the
				// dashboard time window. You'll see multiple log entries (one per benchmark iteration)
				// with individual scores (0.0, 0.5, or 1.0) and detailed reasoning from the evaluator LLM.
				createSimpleTimeseriesPanelWithLinks(12, "Evaluator Score", promEvalScore, 0, 40, 12, 8, "short",
					map[string]interface{}{"min": 0, "max": 1}, combineLinks(evaluatorLogLink, metricsLink, tracesLink)),
				createSimpleTimeseriesPanelWithLinks(13, "Evaluator Pass Rate", promEvalPassRate, 12, 40, 12, 8, "percentunit",
					map[string]interface{}{"min": 0, "max": 1}, combineLinks(evaluatorLogLink, metricsLink, tracesLink)),

				// Tool calling metrics (only populated for tool-assisted test cases)
				createHistogramPanelWithLinks(15, "Tool Call Latency", promToolCallLatency, 0, 48, "ms", combineLinks(llmClientLogLink, metricsLink, tracesLink)),
				createSimpleTimeseriesPanelWithLinks(16, "Tool Calls per Operation", promToolCallCount, 0, 56, 8, 8, "short", nil, combineLinks(llmClientLogLink, metricsLink, tracesLink)),
				createSimpleTimeseriesPanelWithLinks(17, "LLM-Tool Iterations", promToolIterationCount, 8, 56, 8, 8, "short", nil, combineLinks(llmClientLogLink, metricsLink, tracesLink)),
				createSimpleTimeseriesPanelWithLinks(18, "Tool Success Rate", promToolSuccessRate, 16, 56, 8, 8, "percentunit",
					map[string]interface{}{"min": 0, "max": 1}, combineLinks(llmClientLogLink, metricsLink, tracesLink)),
				createSimpleTimeseriesPanelWithLinks(19, "Tool Parameter Accuracy", promToolParamAccuracy, 0, 64, 12, 8, "percentunit", map[string]interface{}{"min": 0, "max": 1}, combineLinks(toolEvaluatorLogLink, metricsLink, tracesLink)),
				createSimpleTimeseriesPanelWithLinks(20, "Tool Selection Accuracy", promToolSelectionAccuracy, 12, 64, 12, 8, "percentunit", map[string]interface{}{"min": 0, "max": 1}, combineLinks(toolEvaluatorLogLink, metricsLink, tracesLink)),
				createSimpleTimeseriesPanelWithLinks(21, "Tool Convergence (Path Efficiency)", promToolConvergence, 0, 72, 24, 8, "percentunit",
					map[string]interface{}{"min": 0, "max": 1}, combineLinks(llmClientLogLink, metricsLink, tracesLink)),

				// ns/op metric (Go benchmark) - moved to bottom
				createSimpleTimeseriesPanelWithLinks(22, "ns/op (Go Benchmark)", promNsPerOp, 0, 80, 24, 8, "ns", nil, combineLinks(llmClientLogLink, metricsLink, tracesLink)),
			},
		},
		"overwrite": true,
	}

	dashboardJSON, err := json.Marshal(dashboard)
	if err != nil {
		return fmt.Errorf("failed to marshal dashboard: %w", err)
	}

	url := fmt.Sprintf("%s/api/dashboards/db", grafanaEndpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(dashboardJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "admin")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to create dashboard: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}
