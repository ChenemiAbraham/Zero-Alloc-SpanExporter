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
	fmt.Println("🔍 Debugging LTT Connection...")
	fmt.Println()

	// Create exporter
	fmt.Println("1️⃣ Creating exporter...")
	config := exporter.DefaultConfig()
	fmt.Printf("   Socket path: %s\n", config.SocketPath)

	exp, err := exporter.New(config)
	if err != nil {
		log.Fatalf("❌ Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())
	fmt.Println("   ✅ Exporter created")
	fmt.Println()

	// Create tracer provider
	fmt.Println("2️⃣ Creating tracer provider...")
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exp),
	)
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)
	fmt.Println("   ✅ Tracer provider created")
	fmt.Println()

	// Get tracer
	tracer := otel.Tracer("debug-app")

	fmt.Println("3️⃣ Socket is now listening on 127.0.0.1:9090")
	fmt.Println("   📱 Now start the LTT viewer in another terminal:")
	fmt.Println("   PowerShell> .\\ltt.exe")
	fmt.Println()

	fmt.Println("⏳ Waiting 10 seconds for viewer to connect...")
	time.Sleep(10 * time.Second)

	fmt.Println()
	fmt.Println("4️⃣ Generating test spans...")

	// Generate spans
	for i := 0; i < 20; i++ {
		ctx := context.Background()
		_, span := tracer.Start(ctx, fmt.Sprintf("test-operation-%d", i))
		time.Sleep(100 * time.Millisecond)
		span.End()

		stats := exp.GetStats()
		fmt.Printf("   Span %d: Exported=%d, Dropped=%d, Failed=%d, IsConnected=%v\n",
			i+1,
			stats.ExportedSpans,
			stats.DroppedSpans,
			stats.FailedWrites,
			exp.IsConnected(),
		)
	}

	fmt.Println()
	fmt.Println("✅ Done!")
	fmt.Println()
	fmt.Println("📊 Final Stats:")
	stats := exp.GetStats()
	fmt.Printf("   Exported: %d\n", stats.ExportedSpans)
	fmt.Printf("   Dropped: %d\n", stats.DroppedSpans)
	fmt.Printf("   Failed: %d\n", stats.FailedWrites)
	fmt.Printf("   Buffer: %.1f%%\n", stats.BufferUsage)
	fmt.Printf("   Connected: %v\n", exp.IsConnected())
	fmt.Println()

	if exp.IsConnected() {
		fmt.Println("🎉 SUCCESS! Viewer is connected and receiving spans!")
	} else {
		fmt.Println("⚠️  WARNING: No viewer connected. Spans are buffered but not sent.")
		fmt.Println("   Make sure to start the viewer BEFORE running this program.")
	}

	fmt.Println()
	fmt.Println("Press Ctrl+C to exit...")
	select {}
}
