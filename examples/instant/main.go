package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	fmt.Println("🚀 LTT Instant Export Example")
	fmt.Println("📡 Spans are exported IMMEDIATELY (no batching)")
	fmt.Println()

	// Create LTT exporter
	exp, err := exporter.New(exporter.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())

	// Create trace provider with SYNCER (not batcher!)
	// This exports spans immediately
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp), // <-- Key change: syncer instead of batcher
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	defer tp.Shutdown(context.Background())

	// Set global provider
	otel.SetTracerProvider(tp)

	// Get tracer
	tracer := tp.Tracer("example-app")

	fmt.Println("✅ Exporter ready - spans will appear in TUI instantly!")
	fmt.Println("💡 Start the viewer: ./ltt.exe")
	fmt.Println()
	fmt.Println("Generating traces...")

	// Generate some traces
	for i := 0; i < 100; i++ {
		ctx := context.Background()

		// Root span
		ctx, span := tracer.Start(ctx, "GET /users/:id")
		span.SetAttributes(
			attribute.String("http.method", "GET"),
			attribute.String("http.route", "/users/:id"),
			attribute.Int("user.id", i),
		)

		// Simulate work
		simulateRequest(ctx, tracer)

		// Randomly fail some requests
		if rand.Float64() < 0.05 {
			span.SetStatus(codes.Error, "Internal server error")
			span.SetAttributes(attribute.String("error", "Database connection failed"))
		} else {
			span.SetStatus(codes.Ok, "")
		}

		span.End()

		// Print stats
		stats := exp.GetStats()
		fmt.Printf("\rSpans: %d | Dropped: %d | Buffer: %.1f%% | Connected: %v  ",
			stats.ExportedSpans,
			stats.DroppedSpans,
			stats.BufferUsage,
			exp.IsConnected(),
		)

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\n\n✅ Done! Generated 100 spans")
	fmt.Println("Check the TUI viewer to see the trace waterfall.")

	// Force flush any remaining spans
	tp.ForceFlush(context.Background())

	// Keep running so you can view metrics
	fmt.Println()
	fmt.Println("Press Ctrl+C to exit...")
	select {}
}

// simulateRequest simulates a traced HTTP request with database and cache calls
func simulateRequest(ctx context.Context, tracer trace.Tracer) {
	// Database query
	_, dbSpan := tracer.Start(ctx, "DB SELECT Users")
	dbSpan.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.table", "users"),
	)
	time.Sleep(time.Duration(10+rand.Intn(20)) * time.Millisecond)
	dbSpan.End()

	// Cache check
	_, cacheSpan := tracer.Start(ctx, "Redis GET Cache")
	cacheSpan.SetAttributes(
		attribute.String("cache.system", "redis"),
		attribute.String("cache.key", "user:123"),
	)
	time.Sleep(time.Duration(1+rand.Intn(5)) * time.Millisecond)
	cacheSpan.End()

	// Transform response
	_, transformSpan := tracer.Start(ctx, "Transform Response")
	transformSpan.SetAttributes(
		attribute.String("transform.type", "json"),
	)
	time.Sleep(time.Duration(5+rand.Intn(10)) * time.Millisecond)
	transformSpan.End()
}
