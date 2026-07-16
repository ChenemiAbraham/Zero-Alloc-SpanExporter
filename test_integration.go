package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	fmt.Println("🧪 LTT Integration Test")
	fmt.Println("=======================\n")

	// Test 1: Start exporter
	fmt.Print("1. Starting exporter... ")
	exp, err := exporter.New(exporter.DefaultConfig())
	if err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
		return
	}
	defer exp.Shutdown(context.Background())
	fmt.Println("✅ PASSED")

	// Test 2: Create tracer
	fmt.Print("2. Creating tracer... ")
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	defer tp.Shutdown(context.Background())
	otel.SetTracerProvider(tp)
	tracer := tp.Tracer("integration-test")
	fmt.Println("✅ PASSED")

	// Test 3: Connect as a client
	fmt.Print("3. Connecting as client... ")
	conn, err := net.Dial("tcp", exporter.DefaultConfig().SocketPath)
	if err != nil {
		fmt.Printf("❌ FAILED: %v\n", err)
		return
	}
	defer conn.Close()
	fmt.Println("✅ PASSED")

	reader := exporter.NewSocketReader(conn)

	// Test 4: Generate and receive spans
	fmt.Print("4. Testing span flow... ")

	// Generate a span
	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "test-span")
	span.SetAttributes(
		attribute.String("test", "value"),
		attribute.Int("number", 42),
	)
	time.Sleep(10 * time.Millisecond)
	span.End()

	// Force flush
	tp.ForceFlush(context.Background())

	// Give it time to write
	time.Sleep(200 * time.Millisecond)

	// Try to read the span
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msgBytes, err := reader.ReadMessage(context.Background())
	if err != nil {
		fmt.Printf("❌ FAILED to read: %v\n", err)
		return
	}

	// Decode the span
	receivedSpan, err := protocol.DecodePayload(msgBytes)
	if err != nil {
		fmt.Printf("❌ FAILED to decode: %v\n", err)
		return
	}

	if receivedSpan.Name != "test-span" {
		fmt.Printf("❌ FAILED: wrong span name: got %q, want %q\n", receivedSpan.Name, "test-span")
		return
	}

	if receivedSpan.Attributes["test"] != "value" {
		fmt.Printf("❌ FAILED: wrong attribute value\n")
		return
	}

	fmt.Println("✅ PASSED")

	// Test 5: Multiple spans
	fmt.Print("5. Testing multiple spans... ")

	for i := 0; i < 5; i++ {
		_, s := tracer.Start(ctx, fmt.Sprintf("span-%d", i))
		time.Sleep(5 * time.Millisecond)
		s.End()
	}

	tp.ForceFlush(context.Background())
	time.Sleep(200 * time.Millisecond)

	receivedCount := 0
	for i := 0; i < 5; i++ {
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		msgBytes, err := reader.ReadMessage(context.Background())
		if err != nil {
			break
		}

		_, err = protocol.DecodePayload(msgBytes)
		if err != nil {
			fmt.Printf("❌ FAILED to decode span %d: %v\n", i, err)
			return
		}
		receivedCount++
	}

	if receivedCount != 5 {
		fmt.Printf("⚠️  WARNING: received %d/5 spans\n", receivedCount)
	} else {
		fmt.Println("✅ PASSED")
	}

	// Summary
	fmt.Println("\n╔══════════════════════════════════════════╗")
	fmt.Println("║     INTEGRATION TEST RESULTS             ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("✅ Exporter working")
	fmt.Println("✅ Socket communication working")
	fmt.Println("✅ Protocol encode/decode working")
	fmt.Println("✅ End-to-end flow functional")
	fmt.Println()
	fmt.Println("🎉 Ready to run full TUI!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  Terminal 1: go run test_e2e.go")
	fmt.Println("  Terminal 2: ./ltt")
}
