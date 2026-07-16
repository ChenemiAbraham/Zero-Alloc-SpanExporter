package storage

import (
	"os"
	"testing"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
)

func TestStorage(t *testing.T) {
	// Create temp directory
	tempDir := "./test-storage"
	defer os.RemoveAll(tempDir)

	// Create store
	config := Config{
		Path:            tempDir,
		TTL:             1 * time.Hour,
		SyncWrites:      true,
		CompactInterval: 10 * time.Minute,
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create test span
	span := &protocol.SpanMessage{
		TraceID:   [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:    [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		ParentID:  [8]byte{0, 0, 0, 0, 0, 0, 0, 0},
		Name:      "test-span",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(100 * time.Millisecond),
		Attributes: map[string]interface{}{
			"key1": "value1",
			"key2": int64(42),
		},
	}

	// Store span
	err = store.Store(span)
	if err != nil {
		t.Fatalf("Failed to store span: %v", err)
	}

	// Retrieve by trace ID
	spans, err := store.GetByTraceID(span.TraceID)
	if err != nil {
		t.Fatalf("Failed to get spans: %v", err)
	}

	if len(spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(spans))
	}

	if spans[0].Name != "test-span" {
		t.Errorf("Expected name 'test-span', got '%s'", spans[0].Name)
	}

	// Test count
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	// Test recent
	recent, err := store.GetRecent(10)
	if err != nil {
		t.Fatalf("Failed to get recent: %v", err)
	}

	if len(recent) != 1 {
		t.Errorf("Expected 1 recent span, got %d", len(recent))
	}

	t.Log("✅ All storage tests passed!")
}

func BenchmarkStore(b *testing.B) {
	// Create temp directory
	tempDir := "./bench-storage"
	defer os.RemoveAll(tempDir)

	// Create store
	config := Config{
		Path:            tempDir,
		TTL:             1 * time.Hour,
		SyncWrites:      false, // Async for benchmarking
		CompactInterval: 10 * time.Minute,
	}

	store, err := NewStore(config)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	span := &protocol.SpanMessage{
		TraceID:   [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:    [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		Name:      "benchmark-span",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(100 * time.Millisecond),
		Attributes: map[string]interface{}{
			"key": "value",
		},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = store.Store(span)
	}
}
