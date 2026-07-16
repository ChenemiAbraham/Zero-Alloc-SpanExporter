package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/yourusername/ltt/pkg/exporter"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	fmt.Println("🧪 LTT End-to-End Test")
	fmt.Println("=====================\n")

	// Create exporter
	fmt.Println("1. Creating exporter...")
	exp, err := exporter.New(exporter.DefaultConfig())
	if err != nil {
		panic(err)
	}
	defer exp.Shutdown(context.Background())
	fmt.Println("   ✅ Exporter created on", exporter.DefaultConfig().SocketPath)

	// Create tracer provider
	fmt.Println("2. Setting up OpenTelemetry...")
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)
	tracer := tp.Tracer("e2e-test")
	fmt.Println("   ✅ Tracer ready")

	fmt.Println("\n3. Generating test traces...")
	fmt.Println("   💡 Start the TUI viewer in another terminal: ./ltt")
	fmt.Println("   💡 Press Ctrl+C to stop\n")

	count := 0
	for {
		// Create root span
		ctx := context.Background()
		ctx, rootSpan := tracer.Start(ctx, "GET /api/users/:id")
		rootSpan.SetAttributes(
			attribute.String("http.method", "GET"),
			attribute.String("http.route", "/api/users/:id"),
			attribute.Int("user.id", rand.Intn(1000)),
		)

		// Simulate database query
		_, dbSpan := tracer.Start(ctx, "Database Query")
		dbSpan.SetAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
		)
		time.Sleep(time.Duration(10+rand.Intn(20)) * time.Millisecond)
		dbSpan.End()

		// Simulate cache lookup
		_, cacheSpan := tracer.Start(ctx, "Cache Lookup")
		cacheSpan.SetAttributes(
			attribute.String("cache.system", "redis"),
			attribute.String("cache.key", fmt.Sprintf("user:%d", rand.Intn(1000))),
		)
		time.Sleep(time.Duration(1+rand.Intn(5)) * time.Millisecond)
		cacheSpan.End()

		// Simulate business logic
		_, logicSpan := tracer.Start(ctx, "Process User Data")
		time.Sleep(time.Duration(5+rand.Intn(10)) * time.Millisecond)
		logicSpan.End()

		// Randomly fail some requests
		if rand.Float64() < 0.1 {
			rootSpan.SetStatus(codes.Error, "Internal server error")
			rootSpan.SetAttributes(attribute.String("error", "Database timeout"))
		} else {
			rootSpan.SetStatus(codes.Ok, "")
		}

		rootSpan.End()

		count++
		stats := exp.GetStats()

		fmt.Printf("\r   Traces: %d | Exported: %d | Dropped: %d | Buffer: %.1f%%  ",
			count,
			stats.ExportedSpans,
			stats.DroppedSpans,
			stats.BufferUsage,
		)

		time.Sleep(500 * time.Millisecond)
	}
}
