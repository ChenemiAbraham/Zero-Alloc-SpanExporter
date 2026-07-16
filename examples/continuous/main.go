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
	fmt.Println("🔄 LTT Continuous Span Generator")
	fmt.Println("📡 Generates spans FOREVER - connect anytime!")
	fmt.Println()

	// Create LTT exporter
	exp, err := exporter.New(exporter.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())

	// Create trace provider with syncer
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	defer tp.Shutdown(context.Background())

	otel.SetTracerProvider(tp)

	tracer := otel.Tracer("continuous-app")

	fmt.Println("✅ Exporter ready on 127.0.0.1:9090")
	fmt.Println("💡 Start viewer ANYTIME: ./ltt.exe")
	fmt.Println("💡 Press Ctrl+C to stop")
	fmt.Println()

	count := 0
	for {
		ctx := context.Background()

		// Root span
		ctx, span := tracer.Start(ctx, "API Request")
		span.SetAttributes(
			attribute.String("http.method", []string{"GET", "POST", "PUT", "DELETE"}[rand.Intn(4)]),
			attribute.String("http.route", fmt.Sprintf("/api/v1/resource-%d", rand.Intn(10))),
			attribute.Int("user.id", rand.Intn(1000)),
		)

		// Simulate work
		simulateWork(ctx, tracer)

		// Randomly fail some requests
		if rand.Float64() < 0.05 {
			span.SetStatus(codes.Error, "Server error")
		} else {
			span.SetStatus(codes.Ok, "")
		}

		span.End()
		count++

		// Print status every 10 spans
		if count%10 == 0 {
			stats := exp.GetStats()
			fmt.Printf("\r[%s] Spans: %d | Connected: %v | Dropped: %d  ",
				time.Now().Format("15:04:05"),
				count,
				exp.IsConnected(),
				stats.DroppedSpans,
			)
		}

		time.Sleep(500 * time.Millisecond) // 2 spans per second
	}
}

func simulateWork(ctx context.Context, tracer trace.Tracer) {
	// Database query
	_, dbSpan := tracer.Start(ctx, "Database Query")
	dbSpan.SetAttributes(
		attribute.String("db.system", "postgresql"),
	)
	time.Sleep(time.Duration(10+rand.Intn(30)) * time.Millisecond)
	dbSpan.End()

	// Cache lookup
	_, cacheSpan := tracer.Start(ctx, "Cache Lookup")
	cacheSpan.SetAttributes(
		attribute.String("cache.system", "redis"),
	)
	time.Sleep(time.Duration(1+rand.Intn(5)) * time.Millisecond)
	cacheSpan.End()

	// Business logic
	_, logicSpan := tracer.Start(ctx, "Business Logic")
	time.Sleep(time.Duration(5+rand.Intn(15)) * time.Millisecond)
	logicSpan.End()
}
