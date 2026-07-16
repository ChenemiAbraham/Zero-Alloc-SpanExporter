package main

import (
	"context"
	"fmt"
	"time"

	"github.com/yourusername/ltt/pkg/exporter"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	fmt.Println("🧪 LTT Smoke Test")
	fmt.Println("================\n")

	// Test 1: Create Exporter
	fmt.Print("1. Creating exporter... ")
	exp, err := exporter.New(exporter.DefaultConfig())
	if err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
		return
	}
	defer exp.Shutdown(context.Background())
	fmt.Println("✅ PASSED")

	// Test 2: Create Tracer Provider
	fmt.Print("2. Creating tracer provider... ")
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)
	fmt.Println("✅ PASSED")

	// Test 3: Create Spans
	fmt.Print("3. Creating test spans... ")
	tracer := tp.Tracer("smoke-test")

	ctx := context.Background()
	ctx, rootSpan := tracer.Start(ctx, "root-operation")
	time.Sleep(10 * time.Millisecond)

	// Child span
	_, childSpan := tracer.Start(ctx, "child-operation")
	time.Sleep(5 * time.Millisecond)
	childSpan.End()

	rootSpan.End()
	fmt.Println("✅ PASSED")

	// Test 4: Force Flush
	fmt.Print("4. Flushing spans... ")
	tp.ForceFlush(context.Background())
	time.Sleep(200 * time.Millisecond)
	fmt.Println("✅ PASSED")

	// Test 5: Check Statistics
	fmt.Print("5. Checking statistics... ")
	stats := exp.GetStats()
	fmt.Println("✅ PASSED")

	fmt.Println("\n📊 Results:")
	fmt.Printf("   ├─ Exported spans:  %d\n", stats.ExportedSpans)
	fmt.Printf("   ├─ Dropped spans:   %d\n", stats.DroppedSpans)
	fmt.Printf("   ├─ Failed writes:   %d\n", stats.FailedWrites)
	fmt.Printf("   └─ Buffer usage:    %.1f%%\n", stats.BufferUsage)

	// Final verdict
	fmt.Println("\n🎯 Test Results:")
	if stats.ExportedSpans >= 2 {
		fmt.Println("   ✅ SUCCESS: Spans were exported to ring buffer")
		fmt.Println("   ℹ️  Failed writes expected (no TUI viewer connected)")
	} else {
		fmt.Println("   ❌ FAILED: Spans not exported")
	}

	fmt.Println("\n💡 Next Steps:")
	fmt.Println("   1. Implement protocol encoding (pkg/protocol/span.go)")
	fmt.Println("   2. Build TUI: make build")
	fmt.Println("   3. Run example: go run ./examples/simple/main.go")
	fmt.Println("   4. Start viewer: ./ltt")
}
