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

// createPercentilePanel creates a timeseries panel showing p50 and p95 percentiles
func createPercentilePanel(id int, title string, p50Metric, p95Metric string, x, y int, unit string) map[string]interface{} {
	return map[string]interface{}{
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
}

// createHistogramPanel creates a histogram panel with exemplar support
func createHistogramPanel(id int, title string, bucketMetric string, x, y int, unit string) map[string]interface{} {
	return map[string]interface{}{
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
}

// createSimpleTimeseriesPanel creates a basic timeseries panel with a single metric
func createSimpleTimeseriesPanel(id int, title string, metric string, x, y, w, h int, unit string, minMax map[string]interface{}) map[string]interface{} {
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

	return panel
}

// createNoLabelTimeseriesPanel creates a timeseries panel without label filters (for GPU metrics)
func createNoLabelTimeseriesPanel(id int, title, legendFormat string, metric string, x, y, w, h int, unit string, minMax map[string]interface{}) map[string]interface{} {
	panel := map[string]interface{}{
		"id":      id,
		"type":    "timeseries",
		"title":   title,
		"gridPos": map[string]int{"x": x, "y": y, "w": w, "h": h},
		"targets": []map[string]interface{}{
			{
				"expr":         metric,
				"legendFormat": legendFormat,
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

	return panel
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
	promScore := semconv.ToPrometheusMetricName(semconv.MetricLLMScore)

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
				createPercentilePanel(1, "Latency Percentiles (p50/p95)", promLatencyP50, promLatencyP95, 0, 0, "ms"),
				createHistogramPanel(2, "Latency Distribution (with Exemplars)", promLatency, 12, 0, "ms"),

				// TTFT metrics
				createPercentilePanel(3, "TTFT Percentiles (p50/p95)", promTTFTP50, promTTFTP95, 0, 8, "ms"),
				createHistogramPanel(4, "TTFT Distribution (with Exemplars)", promTTFT, 12, 8, "ms"),

				// Prompt Evaluation Time metrics
				createPercentilePanel(5, "Prompt Evaluation Time (p50/p95)", promPromptEvalTimeP50, promPromptEvalTimeP95, 0, 16, "ms"),
				createHistogramPanel(6, "Prompt Eval Time Distribution (with Exemplars)", promPromptEvalTime, 12, 16, "ms"),

				// Other metrics
				createSimpleTimeseriesPanel(7, "Tokens per Operation", promTokensPerOp, 0, 24, 8, 8, "short", nil),
				createSimpleTimeseriesPanel(8, "Success Rate", promSuccessRate, 8, 24, 8, 8, "percentunit", map[string]interface{}{"min": 0, "max": 1}),
				createSimpleTimeseriesPanel(9, "Tokens per Second", promTokensPerSecond, 16, 24, 8, 8, "short", nil),

				// GPU metrics
				createNoLabelTimeseriesPanel(10, "GPU Utilization", "GPU Utilization", promGPUUtilization, 0, 32, 12, 8, "percent", map[string]interface{}{"min": 0, "max": 100}),
				createNoLabelTimeseriesPanel(11, "GPU Memory Usage", "GPU Memory", promGPUMemory, 12, 32, 12, 8, "decmbytes", nil),

				// Score metric
				createSimpleTimeseriesPanel(12, "Score per Operation", promScore, 0, 40, 24, 8, "short", map[string]interface{}{"min": 0, "max": 1}),

				// ns/op metric (Go benchmark) - moved to bottom
				createSimpleTimeseriesPanel(13, "ns/op (Go Benchmark)", promNsPerOp, 0, 48, 24, 8, "ns", nil),
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
