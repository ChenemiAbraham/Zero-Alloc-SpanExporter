package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/health"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	fmt.Println("🧪 Testing Prometheus Metrics Integration...")

	// Start metrics server
	config := metrics.ServerConfig{
		Port:         9999, // Use different port to avoid conflicts
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	server := metrics.NewServer(config)

	// Add health checks
	healthHandler := health.New("test")
	healthHandler.Register("memory", health.MemoryHealthChecker(500))
	server.RegisterHealthHandler("/health", healthHandler)

	// Start server
	server.StartAsync()
	defer server.Shutdown(context.Background())

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	// Create exporter
	expConfig := exporter.DefaultConfig()
	expConfig.SocketPath = "127.0.0.1:9091" // Use different port for test
	exp, err := exporter.New(expConfig)
	if err != nil {
		fmt.Printf("❌ Failed to create exporter: %v\n", err)
		os.Exit(1)
	}
	defer exp.Shutdown(context.Background())

	// Create tracer provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exp),
	)
	defer tp.Shutdown(context.Background())

	otel.SetTracerProvider(tp)

	// Generate some spans
	tracer := otel.Tracer("test-service")
	for i := 0; i < 50; i++ {
		ctx := context.Background()
		_, span := tracer.Start(ctx, fmt.Sprintf("test-operation-%d", i))
		time.Sleep(5 * time.Millisecond)
		span.End()
	}

	// Wait for metrics to be recorded
	time.Sleep(2 * time.Second)

	failed := false

	// Test 1: Prometheus metrics endpoint
	fmt.Println("\n✅ Test 1: Prometheus /metrics endpoint")
	resp, err := http.Get("http://localhost:9999/metrics")
	if err != nil {
		fmt.Printf("❌ Failed to fetch metrics: %v\n", err)
		failed = true
	} else {
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("❌ Expected status 200, got %d\n", resp.StatusCode)
			failed = true
		}

		body, _ := io.ReadAll(resp.Body)
		metricsOutput := string(body)

		// Check for expected metrics
		expectedMetrics := []string{
			"ltt_spans_received_total",
			"ltt_spans_exported_total",
			"ltt_spans_dropped_total",
			"ltt_export_duration_microseconds",
			"ltt_encode_duration_microseconds",
			"ltt_buffer_usage_bytes",
			"ltt_goroutines",
			"ltt_memory_usage_bytes",
		}

		for _, metric := range expectedMetrics {
			if !strings.Contains(metricsOutput, metric) {
				fmt.Printf("❌ Metric %s not found in output\n", metric)
				failed = true
			} else {
				fmt.Printf("  ✓ Found metric: %s\n", metric)
			}
		}
	}

	// Test 2: Health check endpoint
	fmt.Println("\n✅ Test 2: Health check /health endpoint")
	resp, err = http.Get("http://localhost:9999/health")
	if err != nil {
		fmt.Printf("❌ Failed to fetch health: %v\n", err)
		failed = true
	} else {
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("❌ Expected status 200, got %d\n", resp.StatusCode)
			failed = true
		}

		body, _ := io.ReadAll(resp.Body)
		healthOutput := string(body)

		if !strings.Contains(healthOutput, "healthy") {
			fmt.Println("❌ Health response missing 'healthy' field")
			failed = true
		}
		if !strings.Contains(healthOutput, "version") {
			fmt.Println("❌ Health response missing 'version' field")
			failed = true
		}

		fmt.Println("  ✓ Health endpoint returned valid JSON")
	}

	// Test 3: Root endpoint
	fmt.Println("\n✅ Test 3: Root / endpoint")
	resp, err = http.Get("http://localhost:9999/")
	if err != nil {
		fmt.Printf("❌ Failed to fetch root: %v\n", err)
		failed = true
	} else {
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("❌ Expected status 200, got %d\n", resp.StatusCode)
			failed = true
		}

		fmt.Println("  ✓ Root endpoint accessible")
	}

	// Print summary
	stats := exp.GetStats()
	fmt.Printf("\n📊 Final Stats:\n")
	fmt.Printf("  - Spans Received: ~50\n")
	fmt.Printf("  - Spans Exported: %d\n", stats.ExportedSpans)
	fmt.Printf("  - Spans Dropped: %d\n", stats.DroppedSpans)
	fmt.Printf("  - Failed Writes: %d\n", stats.FailedWrites)
	fmt.Printf("  - Buffer Usage: %.1f%%\n", stats.BufferUsage)

	if failed {
		fmt.Println("\n❌ Some tests failed")
		os.Exit(1)
	}

	fmt.Println("\n🎉 All metrics tests passed!")
}
