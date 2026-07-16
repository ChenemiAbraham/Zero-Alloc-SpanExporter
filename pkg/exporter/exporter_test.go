package exporter

import (
	"context"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// BenchmarkExportSpan benchmarks the hot path - must be zero-allocation
func BenchmarkExportSpan(b *testing.B) {
	exp, err := New(Config{
		SocketPath: "/tmp/ltt-bench.sock",
		BufferSize: 8192,
	})
	if err != nil {
		b.Fatalf("Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())

	// Create a test span
	span := createTestSpan()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		exp.ExportSpans(context.Background(), []sdktrace.ReadOnlySpan{span})
	}

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "spans/sec")
}

// BenchmarkExportSpanParallel benchmarks concurrent exports
func BenchmarkExportSpanParallel(b *testing.B) {
	exp, err := New(Config{
		SocketPath: "/tmp/ltt-bench.sock",
		BufferSize: 8192,
	})
	if err != nil {
		b.Fatalf("Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())

	span := createTestSpan()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			exp.ExportSpans(context.Background(), []sdktrace.ReadOnlySpan{span})
		}
	})

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "spans/sec")
}

// TestExporterShutdown tests graceful shutdown
func TestExporterShutdown(t *testing.T) {
	exp, err := New(Config{
		SocketPath: "/tmp/ltt-test.sock",
	})
	if err != nil {
		t.Fatalf("Failed to create exporter: %v", err)
	}

	// Export some spans
	span := createTestSpan()
	err = exp.ExportSpans(context.Background(), []sdktrace.ReadOnlySpan{span})
	if err != nil {
		t.Errorf("ExportSpans failed: %v", err)
	}

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = exp.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

// TestExporterStats tests statistics collection
func TestExporterStats(t *testing.T) {
	exp, err := New(Config{
		SocketPath: "/tmp/ltt-test.sock",
		BufferSize: 16,
	})
	if err != nil {
		t.Fatalf("Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())

	// Export spans
	span := createTestSpan()
	for i := 0; i < 10; i++ {
		exp.ExportSpans(context.Background(), []sdktrace.ReadOnlySpan{span})
	}

	time.Sleep(100 * time.Millisecond)

	stats := exp.GetStats()
	if stats.ExportedSpans != 10 {
		t.Errorf("Expected 10 exported spans, got %d", stats.ExportedSpans)
	}
}

// TestExporterBackpressure tests backpressure handling when buffer is full
func TestExporterBackpressure(t *testing.T) {
	exp, err := New(Config{
		SocketPath: "/tmp/ltt-test.sock",
		BufferSize: 16, // Small buffer to trigger backpressure
	})
	if err != nil {
		t.Fatalf("Failed to create exporter: %v", err)
	}
	defer exp.Shutdown(context.Background())

	// Flood with spans
	span := createTestSpan()
	for i := 0; i < 1000; i++ {
		exp.ExportSpans(context.Background(), []sdktrace.ReadOnlySpan{span})
	}

	stats := exp.GetStats()

	// Some spans should be dropped due to buffer overflow
	total := stats.ExportedSpans + stats.DroppedSpans
	if total != 1000 {
		t.Errorf("Expected 1000 total spans, got %d", total)
	}

	if stats.DroppedSpans == 0 {
		t.Logf("Warning: No spans dropped, buffer might be too large for test")
	}

	t.Logf("Stats: Exported=%d, Dropped=%d, BufferUsage=%.1f%%",
		stats.ExportedSpans, stats.DroppedSpans, stats.BufferUsage)
}

// createTestSpan creates a test span for benchmarking
func createTestSpan() sdktrace.ReadOnlySpan {
	return tracetest.SpanStub{
		Name:      "test-span",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(10 * time.Millisecond),
	}.Snapshot()
}
