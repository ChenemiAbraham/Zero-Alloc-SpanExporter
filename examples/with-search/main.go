package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/search"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	fmt.Println("🔍 LTT with Advanced Search Example")
	fmt.Println("📊 Demonstrates search and filtering capabilities")
	fmt.Println()

	// Create config with storage enabled (required for search)
	config := exporter.DefaultConfig()
	config.SocketPath = "127.0.0.1:9091" // Use different port to avoid conflict
	storageConfig := storage.DefaultConfig()
	storageConfig.Path = "./search-demo-data"
	storageConfig.TTL = 1 * time.Hour
	config.Storage = &storageConfig

	// Create LTT exporter
	exp, err := exporter.New(config)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())

	// Create search engine
	searchEngine := search.NewEngine(exp.GetStore())

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp),
	)
	defer tp.Shutdown(context.Background())

	otel.SetTracerProvider(tp)

	tracer := otel.Tracer("search-demo")

	fmt.Println("✅ Storage and search engine initialized")
	fmt.Println("📝 Generating sample traces...")
	fmt.Println()

	// Generate some diverse traces for searching
	generateSampleTraces(tracer)

	time.Sleep(2 * time.Second) // Wait for spans to be stored

	fmt.Println("🔍 Running search queries...")
	fmt.Println()

	// Example 1: Find all errors
	fmt.Println("1️⃣ Search: Find all errors in last hour")
	errorQuery, _ := search.NewQuery().
		WithError().
		Last(1 * time.Hour).
		Limit(10).
		Build()

	results, err := searchEngine.Search(errorQuery)
	if err != nil {
		log.Printf("Search failed: %v", err)
	} else {
		fmt.Printf("   Found %d error spans in %v\n", results.Total, results.QueryDuration)
		for _, span := range results.Spans {
			fmt.Printf("   - %s (%.1fms)\n", span.Name, float64(span.EndTime.Sub(span.StartTime).Microseconds())/1000)
		}
	}
	fmt.Println()

	// Example 2: Find slow spans
	fmt.Println("2️⃣ Search: Find spans slower than 100ms")
	slowQuery, _ := search.NewQuery().
		SlowerThan(100 * time.Millisecond).
		Last(1 * time.Hour).
		Limit(5).
		Build()

	results, err = searchEngine.Search(slowQuery)
	if err != nil {
		log.Printf("Search failed: %v", err)
	} else {
		fmt.Printf("   Found %d slow spans in %v\n", results.Total, results.QueryDuration)
		for _, span := range results.Spans {
			fmt.Printf("   - %s (%.1fms)\n", span.Name, float64(span.EndTime.Sub(span.StartTime).Microseconds())/1000)
		}
	}
	fmt.Println()

	// Example 3: Find specific operation
	fmt.Println("3️⃣ Search: Find 'Payment Processing' operations")
	opQuery, _ := search.NewQuery().
		Operation("Payment Processing").
		Last(1 * time.Hour).
		Limit(10).
		Build()

	results, err = searchEngine.Search(opQuery)
	if err != nil {
		log.Printf("Search failed: %v", err)
	} else {
		fmt.Printf("   Found %d matching spans in %v\n", results.Total, results.QueryDuration)
	}
	fmt.Println()

	// Example 4: Fast successful spans
	fmt.Println("4️⃣ Search: Fast successful spans (<50ms, no errors)")
	fastQuery, _ := search.NewQuery().
		WithoutError().
		FasterThan(50 * time.Millisecond).
		Last(1 * time.Hour).
		Limit(10).
		Build()

	results, err = searchEngine.Search(fastQuery)
	if err != nil {
		log.Printf("Search failed: %v", err)
	} else {
		fmt.Printf("   Found %d fast successful spans in %v\n", results.Total, results.QueryDuration)
	}
	fmt.Println()

	fmt.Println("✅ Search demonstration complete!")
	fmt.Println()
	fmt.Println("💡 Search capabilities:")
	fmt.Println("   - Filter by operation/service")
	fmt.Println("   - Find errors or successes")
	fmt.Println("   - Duration range queries (fast/slow)")
	fmt.Println("   - Time range filtering")
	fmt.Println("   - Attribute-based search")
}

func generateSampleTraces(tracer trace.Tracer) {
	ctx := context.Background()

	// Generate some fast successful spans
	for i := 0; i < 5; i++ {
		_, span := tracer.Start(ctx, "Quick API Call")
		span.SetAttributes(attribute.String("type", "api"))
		time.Sleep(20 * time.Millisecond)
		span.SetStatus(codes.Ok, "")
		span.End()
	}

	// Generate some slow spans
	for i := 0; i < 3; i++ {
		_, span := tracer.Start(ctx, "Slow Database Query")
		span.SetAttributes(attribute.String("type", "database"))
		time.Sleep(150 * time.Millisecond)
		span.SetStatus(codes.Ok, "")
		span.End()
	}

	// Generate some errors
	for i := 0; i < 4; i++ {
		_, span := tracer.Start(ctx, "Payment Processing")
		span.SetAttributes(attribute.String("type", "payment"))
		time.Sleep(30 * time.Millisecond)
		span.SetStatus(codes.Error, "Payment failed")
		span.End()
	}

	// Generate mixed spans
	_, span := tracer.Start(ctx, "User Registration")
	span.SetAttributes(attribute.String("type", "auth"))
	time.Sleep(75 * time.Millisecond)
	span.SetStatus(codes.Ok, "")
	span.End()

	fmt.Println("   Generated: 5 fast + 3 slow + 4 errors + 1 mixed = 13 spans")
}
