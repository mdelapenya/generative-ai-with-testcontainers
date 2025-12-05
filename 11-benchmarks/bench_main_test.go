package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/testcontainers/testcontainers-go"
	dmr "github.com/testcontainers/testcontainers-go/modules/dockermodelrunner"
	lgtm "github.com/testcontainers/testcontainers-go/modules/grafana-lgtm"
)

var (
	dmrContainer     testcontainers.Container
	lgtmContainer    testcontainers.Container
	otelSetup        *OtelSetup
	metricsCollector *MetricsCollector
)

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	loadErr := godotenv.Load()
	if loadErr != nil {
		log.Printf("No .env file found, continuing without it: %v", loadErr)
	}

	// Load the models to benchmark
	models = getModelsToTest()

	ctx := context.Background()

	// Disable Ryuk to keep containers running after tests complete
	// This allows you to explore the Grafana dashboard with all collected metrics
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	fmt.Printf("\n=================================================\n")
	fmt.Printf("🚀 Starting LLM Benchmark Environment\n")
	fmt.Printf("=================================================\n")
	fmt.Printf("Note: Containers will remain running after the\n")
	fmt.Printf("benchmark completes so you can explore Grafana.\n")
	fmt.Printf("=================================================\n\n")

	// Start LGTM stack
	var err error
	lgtmCtr, err := lgtm.Run(
		ctx, "grafana/otel-lgtm:0.11.18",
		testcontainers.WithReuseByName("lgtm-llm-benchmarks"),
	)
	if err != nil {
		log.Fatalf("Failed to start LGTM container: %s", err)
	}
	lgtmContainer = lgtmCtr

	// Get OTLP endpoint
	otlpEndpoint, err := lgtmCtr.OtlpHttpEndpoint(ctx)
	if err != nil {
		log.Fatalf("Failed to get OTLP endpoint: %s", err)
	}

	// Initialize OpenTelemetry
	otelSetup, err = InitOTel(ctx, otlpEndpoint)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %s", err)
	}

	// Initialize metrics collector
	metricsCollector, err = NewMetricsCollector()
	if err != nil {
		log.Fatalf("Failed to create metrics collector: %s", err)
	}

	// Start DMR container
	dmrCtr, err := dmr.Run(ctx, testcontainers.WithReuseByName("dmr-llm-benchmarks"))
	if err != nil {
		log.Fatalf("Failed to start DMR container: %s", err)
	}
	dmrContainer = dmrCtr

	// Get Grafana endpoint and create dashboard
	grafanaEndpoint, err := lgtmCtr.HttpEndpoint(ctx)
	if err != nil {
		log.Printf("Warning: Failed to get Grafana endpoint: %s", err)
		grafanaEndpoint = ""
	} else {
		fmt.Printf("\n=================================================\n")
		fmt.Printf("📊 Grafana Observability Stack Ready\n")
		fmt.Printf("=================================================\n")
		fmt.Printf("URL: %s\n", grafanaEndpoint)
		fmt.Printf("Credentials: admin / admin\n")
		fmt.Printf("=================================================\n\n")

		// Create Grafana dashboard immediately so users can watch metrics populate in real-time
		fmt.Printf("📊 Creating Grafana dashboard...\n")
		dashboardTitle := "LLM Bench (DMR + Testcontainers)"
		if err := CreateGrafanaDashboard(grafanaEndpoint, dashboardTitle); err != nil {
			log.Printf("Warning: Failed to create Grafana dashboard: %s", err)
		} else {
			fmt.Printf("✅ Dashboard created! Visit %s/dashboards to watch metrics populate\n\n", grafanaEndpoint)
		}
	}

	// Run tests
	exitCode := m.Run()

	// Shutdown OpenTelemetry to flush remaining data
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := otelSetup.Shutdown(shutdownCtx); err != nil {
		log.Printf("Warning: Failed to shutdown OpenTelemetry: %s", err)
	}

	// Print completion banner with instructions
	fmt.Printf("\n=================================================\n")
	fmt.Printf("✅ Benchmark Complete!\n")
	fmt.Printf("=================================================\n")
	if grafanaEndpoint != "" {
		fmt.Printf("Grafana is still running at:\n")
		fmt.Printf("  %s/dashboards\n\n", grafanaEndpoint)
		fmt.Printf("Explore your metrics and traces, then stop\n")
		fmt.Printf("containers when done:\n")
		fmt.Printf("  docker ps --filter label=org.testcontainers.session-id\n")
		fmt.Printf("  docker stop <container-ids>\n")
	}
	fmt.Printf("=================================================\n\n")

	os.Exit(exitCode)
}
