package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CreateGrafanaDashboard creates a Grafana dashboard for LLM benchmarks
// Uses a fixed UID to ensure the same dashboard is replaced on each run (no duplicates)
func CreateGrafanaDashboard(grafanaEndpoint, dashboardTitle string) error {
	// Ensure the endpoint has a scheme
	if !strings.HasPrefix(grafanaEndpoint, "http://") && !strings.HasPrefix(grafanaEndpoint, "https://") {
		grafanaEndpoint = "http://" + grafanaEndpoint
	}

	// Convert OTel metric names to Prometheus format
	promLatencyP50 := ToPrometheusMetricName(MetricLLMLatencyP50)
	promLatencyP95 := ToPrometheusMetricName(MetricLLMLatencyP95)
	promLatency := ToPrometheusMetricName(MetricLLMLatency)
	promPromptEvalTimeP50 := ToPrometheusMetricName(MetricLLMPromptEvalTimeP50)
	promPromptEvalTimeP95 := ToPrometheusMetricName(MetricLLMPromptEvalTimeP95)
	promPromptEvalTime := ToPrometheusMetricName(MetricLLMPromptEvalTime)
	promTokensPerOp := ToPrometheusMetricName(MetricLLMTokensPerOp)
	promSuccessRate := ToPrometheusMetricName(MetricLLMSuccessRate)
	promTokensPerSecond := ToPrometheusMetricName(MetricLLMTokensPerSecond)
	promGPUUtilization := ToPrometheusMetricName(MetricGPUUtilization)
	promGPUMemory := ToPrometheusMetricName(MetricGPUMemory)
	promScore := ToPrometheusMetricName(MetricLLMScore)

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
						"name":       AttrModel,
						"label":      "Model",
						"type":       "query",
						"query":      fmt.Sprintf("label_values(%s, %s)", promLatencyP50, AttrModel),
						"definition": fmt.Sprintf("label_values(%s, %s)", promLatencyP50, AttrModel),
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
						"name":       AttrCase,
						"label":      "Test Case",
						"type":       "query",
						"query":      fmt.Sprintf("label_values(%s, %s)", promLatencyP50, AttrCase),
						"definition": fmt.Sprintf("label_values(%s, %s)", promLatencyP50, AttrCase),
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
						"name":       AttrTemp,
						"label":      "Temperature",
						"type":       "query",
						"query":      fmt.Sprintf("label_values(%s, %s)", promLatencyP50, AttrTemp),
						"definition": fmt.Sprintf("label_values(%s, %s)", promLatencyP50, AttrTemp),
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
				{
					"id":      1,
					"type":    "timeseries",
					"title":   "Latency Percentiles (p50/p95)",
					"gridPos": map[string]int{"x": 0, "y": 0, "w": 12, "h": 8},
					"targets": []map[string]interface{}{
						{
							"expr": fmt.Sprintf("%s{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}",
								promLatencyP50, AttrModel, AttrModel, AttrCase, AttrCase, AttrTemp, AttrTemp),
							"legendFormat": fmt.Sprintf("p50 - {{%s}} - {{%s}} (T={{%s}})", AttrModel, AttrCase, AttrTemp),
							"datasource": map[string]interface{}{
								"type": "prometheus",
								"uid":  "prometheus",
							},
							"refId": "A",
						},
						{
							"expr": fmt.Sprintf("%s{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}",
								promLatencyP95, AttrModel, AttrModel, AttrCase, AttrCase, AttrTemp, AttrTemp),
							"legendFormat": fmt.Sprintf("p95 - {{%s}} - {{%s}} (T={{%s}})", AttrModel, AttrCase, AttrTemp),
							"datasource": map[string]interface{}{
								"type": "prometheus",
								"uid":  "prometheus",
							},
							"refId": "B",
						},
					},
					"fieldConfig": map[string]interface{}{
						"defaults": map[string]interface{}{
							"unit": "ms",
						},
					},
				},
				{
					"id":      2,
					"type":    "histogram",
					"title":   "Latency Distribution (with Exemplars)",
					"gridPos": map[string]int{"x": 12, "y": 0, "w": 12, "h": 8},
					"targets": []map[string]interface{}{
						{
							"expr": fmt.Sprintf("rate(%s_bucket{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}[5m])",
								promLatency, AttrModel, AttrModel, AttrCase, AttrCase, AttrTemp, AttrTemp),
							"legendFormat": fmt.Sprintf("{{%s}} - {{%s}} (T={{%s}}) - {{le}}", AttrModel, AttrCase, AttrTemp),
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
							"unit": "ms",
						},
					},
					"options": map[string]interface{}{
						"legend": map[string]interface{}{
							"displayMode": "list",
							"placement":   "bottom",
						},
					},
				},
				{
					"id":      3,
					"type":    "timeseries",
					"title":   "Prompt Evaluation Time (p50/p95)",
					"gridPos": map[string]int{"x": 0, "y": 8, "w": 12, "h": 8},
					"targets": []map[string]interface{}{
						{
							"expr": fmt.Sprintf("%s{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}",
								promPromptEvalTimeP50, AttrModel, AttrModel, AttrCase, AttrCase, AttrTemp, AttrTemp),
							"legendFormat": fmt.Sprintf("p50 - {{%s}} - {{%s}} (T={{%s}})", AttrModel, AttrCase, AttrTemp),
							"datasource": map[string]interface{}{
								"type": "prometheus",
								"uid":  "prometheus",
							},
							"refId": "A",
						},
						{
							"expr": fmt.Sprintf("%s{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}",
								promPromptEvalTimeP95, AttrModel, AttrModel, AttrCase, AttrCase, AttrTemp, AttrTemp),
							"legendFormat": fmt.Sprintf("p95 - {{%s}} - {{%s}} (T={{%s}})", AttrModel, AttrCase, AttrTemp),
							"datasource": map[string]interface{}{
								"type": "prometheus",
								"uid":  "prometheus",
							},
							"refId": "B",
						},
					},
					"fieldConfig": map[string]interface{}{
						"defaults": map[string]interface{}{
							"unit": "ms",
						},
					},
				},
				{
					"id":      4,
					"type":    "histogram",
					"title":   "Prompt Eval Time Distribution (with Exemplars)",
					"gridPos": map[string]int{"x": 12, "y": 8, "w": 12, "h": 8},
					"targets": []map[string]interface{}{
						{
							"expr": fmt.Sprintf("rate(%s_bucket{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}[5m])",
								promPromptEvalTime, AttrModel, AttrModel, AttrCase, AttrCase, AttrTemp, AttrTemp),
							"legendFormat": fmt.Sprintf("{{%s}} - {{%s}} (T={{%s}}) - {{le}}", AttrModel, AttrCase, AttrTemp),
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
							"unit": "ms",
						},
					},
					"options": map[string]interface{}{
						"legend": map[string]interface{}{
							"displayMode": "list",
							"placement":   "bottom",
						},
					},
				},
				{
					"id":      5,
					"type":    "timeseries",
					"title":   "Tokens per Operation",
					"gridPos": map[string]int{"x": 0, "y": 16, "w": 8, "h": 8},
					"targets": []map[string]interface{}{
						{
							"expr": fmt.Sprintf("%s{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}",
								promTokensPerOp, AttrModel, AttrModel, AttrCase, AttrCase, AttrTemp, AttrTemp),
							"legendFormat": fmt.Sprintf("{{%s}} - {{%s}} (T={{%s}})", AttrModel, AttrCase, AttrTemp),
							"datasource": map[string]interface{}{
								"type": "prometheus",
								"uid":  "prometheus",
							},
							"refId": "A",
						},
					},
					"fieldConfig": map[string]interface{}{
						"defaults": map[string]interface{}{
							"unit": "short",
						},
					},
				},
				{
					"id":      6,
					"type":    "timeseries",
					"title":   "Success Rate",
					"gridPos": map[string]int{"x": 8, "y": 16, "w": 8, "h": 8},
					"targets": []map[string]interface{}{
						{
							"expr": fmt.Sprintf("%s{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}",
								promSuccessRate, AttrModel, AttrModel, AttrCase, AttrCase, AttrTemp, AttrTemp),
							"legendFormat": fmt.Sprintf("{{%s}} - {{%s}} (T={{%s}})", AttrModel, AttrCase, AttrTemp),
							"datasource": map[string]interface{}{
								"type": "prometheus",
								"uid":  "prometheus",
							},
							"refId": "A",
						},
					},
					"fieldConfig": map[string]interface{}{
						"defaults": map[string]interface{}{
							"unit": "percentunit",
							"min":  0,
							"max":  1,
						},
					},
				},
				{
					"id":      7,
					"type":    "timeseries",
					"title":   "Tokens per Second",
					"gridPos": map[string]int{"x": 16, "y": 16, "w": 8, "h": 8},
					"targets": []map[string]interface{}{
						{
							"expr": fmt.Sprintf("%s{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}",
								promTokensPerSecond, AttrModel, AttrModel, AttrCase, AttrCase, AttrTemp, AttrTemp),
							"legendFormat": fmt.Sprintf("{{%s}} - {{%s}} (T={{%s}})", AttrModel, AttrCase, AttrTemp),
							"datasource": map[string]interface{}{
								"type": "prometheus",
								"uid":  "prometheus",
							},
							"refId": "A",
						},
					},
					"fieldConfig": map[string]interface{}{
						"defaults": map[string]interface{}{
							"unit": "short",
						},
					},
				},
				{
					"id":      8,
					"type":    "timeseries",
					"title":   "GPU Utilization",
					"gridPos": map[string]int{"x": 0, "y": 24, "w": 12, "h": 8},
					"targets": []map[string]interface{}{
						{
							"expr":         promGPUUtilization,
							"legendFormat": "GPU Utilization",
							"datasource": map[string]interface{}{
								"type": "prometheus",
								"uid":  "prometheus",
							},
							"refId": "A",
						},
					},
					"fieldConfig": map[string]interface{}{
						"defaults": map[string]interface{}{
							"unit": "percent",
							"min":  0,
							"max":  100,
						},
					},
				},
				{
					"id":      9,
					"type":    "timeseries",
					"title":   "GPU Memory Usage",
					"gridPos": map[string]int{"x": 12, "y": 24, "w": 12, "h": 8},
					"targets": []map[string]interface{}{
						{
							"expr":         promGPUMemory,
							"legendFormat": "GPU Memory",
							"datasource": map[string]interface{}{
								"type": "prometheus",
								"uid":  "prometheus",
							},
							"refId": "A",
						},
					},
					"fieldConfig": map[string]interface{}{
						"defaults": map[string]interface{}{
							"unit": "decmbytes",
						},
					},
				},
				{
					"id":      10,
					"type":    "timeseries",
					"title":   "Score per Operation",
					"gridPos": map[string]int{"x": 0, "y": 32, "w": 24, "h": 8},
					"targets": []map[string]interface{}{
						{
							"expr": fmt.Sprintf("%s{%s=~\"$%s\", %s=~\"$%s\", %s=~\"$%s\"}",
								promScore, AttrModel, AttrModel, AttrCase, AttrCase, AttrTemp, AttrTemp),
							"legendFormat": fmt.Sprintf("{{%s}} - {{%s}} (T={{%s}})", AttrModel, AttrCase, AttrTemp),
							"datasource": map[string]interface{}{
								"type": "prometheus",
								"uid":  "prometheus",
							},
							"refId": "A",
						},
					},
					"fieldConfig": map[string]interface{}{
						"defaults": map[string]interface{}{
							"unit": "short",
							"min":  0,
							"max":  1,
						},
					},
				},
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
