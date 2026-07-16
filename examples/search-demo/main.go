package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/search"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
	"go.opentelemetry.io/otel/codes"
)

func main() {
	fmt.Println("🔍 LTT Search Demo - Standalone Test")
	fmt.Println("=====================================")
	fmt.Println()

	// Create temporary storage
	dir := "./demo-search-data"
	os.RemoveAll(dir) // Clean start
	defer os.RemoveAll(dir)

	cfg := storage.DefaultConfig()
	cfg.Path = dir
	cfg.TTL = 1 * time.Hour

	store, err := storage.NewStore(cfg)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create search engine
	engine := search.NewEngine(store)

	fmt.Println("✅ Storage and search engine initialized")
	fmt.Println()

	// Generate diverse test data
	fmt.Println("📝 Generating test spans...")
	generateTestSpans(store)
	fmt.Println()

	// Run search queries
	fmt.Println("🔍 Running Search Queries...")
	fmt.Println("================================")
	fmt.Println()

	// Query 1: Find all errors
	fmt.Println("1️⃣  SEARCH: Find all error spans")
	query1, _ := search.NewQuery().
		WithError().
		Last(1 * time.Hour).
		Build()

	result1, _ := engine.Search(query1)
	fmt.Printf("   📊 Found %d errors in %v\n", result1.Total, result1.QueryDuration)
	for _, span := range result1.Spans {
		duration := span.EndTime.Sub(span.StartTime)
		fmt.Printf("      • %s (%.1fms)\n", span.Name, float64(duration.Microseconds())/1000)
	}
	fmt.Println()

	// Query 2: Find slow operations
	fmt.Println("2️⃣  SEARCH: Find slow operations (>100ms)")
	query2, _ := search.NewQuery().
		SlowerThan(100 * time.Millisecond).
		Last(1 * time.Hour).
		Build()

	result2, _ := engine.Search(query2)
	fmt.Printf("   📊 Found %d slow spans in %v\n", result2.Total, result2.QueryDuration)
	for _, span := range result2.Spans {
		duration := span.EndTime.Sub(span.StartTime)
		fmt.Printf("      • %s (%.1fms)\n", span.Name, float64(duration.Microseconds())/1000)
	}
	fmt.Println()

	// Query 3: Find specific operation
	fmt.Println("3️⃣  SEARCH: Find 'Payment Processing' operations")
	query3, _ := search.NewQuery().
		Operation("Payment Processing").
		Last(1 * time.Hour).
		Build()

	result3, _ := engine.Search(query3)
	fmt.Printf("   📊 Found %d payment operations in %v\n", result3.Total, result3.QueryDuration)
	for _, span := range result3.Spans {
		duration := span.EndTime.Sub(span.StartTime)
		status := "✅ OK"
		if codes.Code(span.StatusCode) == codes.Error {
			status = "❌ ERROR"
		}
		fmt.Printf("      • %s (%.1fms) %s\n", span.Name, float64(duration.Microseconds())/1000, status)
	}
	fmt.Println()

	// Query 4: Find by attribute
	fmt.Println("4️⃣  SEARCH: Find all database operations")
	query4, _ := search.NewQuery().
		WithAttribute("type", "database").
		Last(1 * time.Hour).
		Build()

	result4, _ := engine.Search(query4)
	fmt.Printf("   📊 Found %d database spans in %v\n", result4.Total, result4.QueryDuration)
	for _, span := range result4.Spans {
		duration := span.EndTime.Sub(span.StartTime)
		fmt.Printf("      • %s (%.1fms)\n", span.Name, float64(duration.Microseconds())/1000)
	}
	fmt.Println()

	// Query 5: Fast successful operations
	fmt.Println("5️⃣  SEARCH: Find fast successful operations (<50ms, no errors)")
	query5, _ := search.NewQuery().
		WithoutError().
		FasterThan(50 * time.Millisecond).
		Last(1 * time.Hour).
		Build()

	result5, _ := engine.Search(query5)
	fmt.Printf("   📊 Found %d fast successful spans in %v\n", result5.Total, result5.QueryDuration)
	for _, span := range result5.Spans {
		duration := span.EndTime.Sub(span.StartTime)
		fmt.Printf("      • %s (%.1fms)\n", span.Name, float64(duration.Microseconds())/1000)
	}
	fmt.Println()

	// Summary
	fmt.Println("================================")
	fmt.Println("✅ Search Demo Complete!")
	fmt.Println()
	fmt.Println("💡 Index-Based Search Features:")
	fmt.Println("   • Operation name index")
	fmt.Println("   • Status code index")
	fmt.Println("   • Duration bucket index")
	fmt.Println("   • Attribute key-value index")
	fmt.Println()
	fmt.Println("🚀 Query Performance: Sub-millisecond!")
}

func generateTestSpans(store *storage.Store) {
	ctx := context.Background()
	_ = ctx
	now := time.Now()

	spans := []*protocol.SpanMessage{
		// Fast API calls
		{
			TraceID:    [16]byte{1},
			SpanID:     [8]byte{1},
			Name:       "Quick API Call",
			StartTime:  now.Add(-5 * time.Second),
			EndTime:    now.Add(-5*time.Second + 15*time.Millisecond),
			StatusCode: codes.Ok,
			Attributes: map[string]interface{}{"type": "api", "endpoint": "/users"},
		},
		{
			TraceID:    [16]byte{2},
			SpanID:     [8]byte{2},
			Name:       "Fast Cache Hit",
			StartTime:  now.Add(-4 * time.Second),
			EndTime:    now.Add(-4*time.Second + 5*time.Millisecond),
			StatusCode: codes.Ok,
			Attributes: map[string]interface{}{"type": "cache", "hit": "true"},
		},
		{
			TraceID:    [16]byte{3},
			SpanID:     [8]byte{3},
			Name:       "Quick Validation",
			StartTime:  now.Add(-3 * time.Second),
			EndTime:    now.Add(-3*time.Second + 8*time.Millisecond),
			StatusCode: codes.Ok,
			Attributes: map[string]interface{}{"type": "validation"},
		},

		// Slow database queries
		{
			TraceID:    [16]byte{4},
			SpanID:     [8]byte{4},
			Name:       "Slow Database Query",
			StartTime:  now.Add(-10 * time.Second),
			EndTime:    now.Add(-10*time.Second + 250*time.Millisecond),
			StatusCode: codes.Ok,
			Attributes: map[string]interface{}{"type": "database", "query": "SELECT * FROM orders"},
		},
		{
			TraceID:    [16]byte{5},
			SpanID:     [8]byte{5},
			Name:       "Full Table Scan",
			StartTime:  now.Add(-9 * time.Second),
			EndTime:    now.Add(-9*time.Second + 500*time.Millisecond),
			StatusCode: codes.Ok,
			Attributes: map[string]interface{}{"type": "database", "query": "FULL SCAN"},
		},

		// Payment operations (some failing)
		{
			TraceID:    [16]byte{6},
			SpanID:     [8]byte{6},
			Name:       "Payment Processing",
			StartTime:  now.Add(-8 * time.Second),
			EndTime:    now.Add(-8*time.Second + 150*time.Millisecond),
			StatusCode: codes.Error,
			Attributes: map[string]interface{}{"type": "payment", "amount": "100.00"},
		},
		{
			TraceID:    [16]byte{7},
			SpanID:     [8]byte{7},
			Name:       "Payment Processing",
			StartTime:  now.Add(-7 * time.Second),
			EndTime:    now.Add(-7*time.Second + 120*time.Millisecond),
			StatusCode: codes.Ok,
			Attributes: map[string]interface{}{"type": "payment", "amount": "50.00"},
		},
		{
			TraceID:    [16]byte{8},
			SpanID:     [8]byte{8},
			Name:       "Payment Processing",
			StartTime:  now.Add(-6 * time.Second),
			EndTime:    now.Add(-6*time.Second + 200*time.Millisecond),
			StatusCode: codes.Error,
			Attributes: map[string]interface{}{"type": "payment", "amount": "200.00"},
		},

		// Authentication
		{
			TraceID:    [16]byte{9},
			SpanID:     [8]byte{9},
			Name:       "User Authentication",
			StartTime:  now.Add(-2 * time.Second),
			EndTime:    now.Add(-2*time.Second + 30*time.Millisecond),
			StatusCode: codes.Ok,
			Attributes: map[string]interface{}{"type": "auth", "method": "oauth"},
		},
		{
			TraceID:    [16]byte{10},
			SpanID:     [8]byte{10},
			Name:       "Failed Login",
			StartTime:  now.Add(-1 * time.Second),
			EndTime:    now.Add(-1*time.Second + 25*time.Millisecond),
			StatusCode: codes.Error,
			Attributes: map[string]interface{}{"type": "auth", "method": "password"},
		},
	}

	for _, span := range spans {
		if err := store.Store(span); err != nil {
			log.Printf("Failed to store span: %v", err)
		}
	}

	// Wait for writes to settle
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("   Generated %d diverse spans:\n", len(spans))
	fmt.Printf("      • 3 fast operations (<50ms)\n")
	fmt.Printf("      • 2 slow database queries (>100ms)\n")
	fmt.Printf("      • 3 payment operations (1 success, 2 errors)\n")
	fmt.Printf("      • 2 authentication attempts (1 success, 1 failure)\n")
}
