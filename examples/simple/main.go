package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/yourusername/ltt/pkg/exporter"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	// Create LTT exporter with default config (Windows-compatible)
	exp, err := exporter.New(exporter.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	defer tp.Shutdown(context.Background())

	// Set global provider
	otel.SetTracerProvider(tp)

	// Get tracer
	tracer := tp.Tracer("example-app")

	fmt.Println("Starting example application...")
	fmt.Println("Generating traces... (start TUI viewer with: ltt)")
	fmt.Println()

	// Simulate some traced operations
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
		fmt.Printf("\rSpans: %d | Dropped: %d | Buffer: %.1f%%  ",
			stats.ExportedSpans,
			stats.DroppedSpans,
			stats.BufferUsage,
		)

		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\nDone! Check the TUI viewer.")
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
