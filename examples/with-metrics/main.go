package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	fmt.Println("🚀 Starting LTT example with Prometheus metrics...")
	fmt.Println("📊 Metrics available at: http://localhost:2112/metrics")
	fmt.Println("🏥 Health check at: http://localhost:2112/health")
	fmt.Println("")

	// Create LTT exporter
	config := exporter.DefaultConfig()
	exp, err := exporter.New(config)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())

	// Create tracer provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exp),
	)
	defer tp.Shutdown(context.Background())

	// Register as global tracer provider
	otel.SetTracerProvider(tp)

	// Create tracer
	tracer := otel.Tracer("example-service")

	fmt.Println("💡 TIP: Run './ltt.exe' in another terminal to view traces")
	fmt.Println("💡 TIP: Run 'curl http://localhost:2112/metrics' to see Prometheus metrics")
	fmt.Println("")
	fmt.Println("Generating traces...")

	// Generate some traces
	for i := 0; i < 100; i++ {
		ctx := context.Background()
		_, span := tracer.Start(ctx, fmt.Sprintf("operation-%d", i))

		// Simulate work
		time.Sleep(10 * time.Millisecond)

		span.End()

		if (i+1)%10 == 0 {
			fmt.Printf("✓ Generated %d spans\n", i+1)

			// Print current stats
			stats := exp.GetStats()
			fmt.Printf("  - Exported: %d, Dropped: %d, Failed: %d, Buffer: %.1f%%\n",
				stats.ExportedSpans, stats.DroppedSpans, stats.FailedWrites, stats.BufferUsage)
		}
	}

	fmt.Println("")
	fmt.Println("✅ Done! Generated 100 spans")
	fmt.Println("📊 Check metrics: curl http://localhost:2112/metrics | grep ltt_")
	fmt.Println("🏥 Check health: curl http://localhost:2112/health")

	// Keep running so metrics can be scraped
	fmt.Println("")
	fmt.Println("Press Ctrl+C to exit...")
	select {}
}
