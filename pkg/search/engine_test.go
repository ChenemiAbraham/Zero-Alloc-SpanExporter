package search

import (
	"os"
	"testing"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
	"go.opentelemetry.io/otel/codes"
)

func TestSearchWithIndexes(t *testing.T) {
	// Create temporary storage
	dir, err := os.MkdirTemp("", "search-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	cfg := storage.DefaultConfig()
	cfg.Path = dir
	cfg.TTL = 1 * time.Hour

	store, err := storage.NewStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create search engine
	engine := NewEngine(store)

	// Store some test spans
	now := time.Now()

	spans := []*protocol.SpanMessage{
		{
			TraceID:    [16]byte{1},
			SpanID:     [8]byte{1},
			Name:       "Fast API Call",
			StartTime:  now,
			EndTime:    now.Add(10 * time.Millisecond),
			StatusCode: codes.Ok,
			Attributes: map[string]interface{}{"type": "api"},
		},
		{
			TraceID:    [16]byte{2},
			SpanID:     [8]byte{2},
			Name:       "Slow Database Query",
			StartTime:  now,
			EndTime:    now.Add(200 * time.Millisecond),
			StatusCode: codes.Ok,
			Attributes: map[string]interface{}{"type": "database"},
		},
		{
			TraceID:    [16]byte{3},
			SpanID:     [8]byte{3},
			Name:       "Failed Payment",
			StartTime:  now,
			EndTime:    now.Add(50 * time.Millisecond),
			StatusCode: codes.Error,
			Attributes: map[string]interface{}{"type": "payment"},
		},
	}

	for _, span := range spans {
		if err := store.Store(span); err != nil {
			t.Fatalf("Failed to store span: %v", err)
		}
	}

	// Wait for writes to settle
	time.Sleep(100 * time.Millisecond)

	// Test 1: Search by operation name
	t.Run("SearchByOperation", func(t *testing.T) {
		query, _ := NewQuery().
			Operation("Fast API Call").
			Limit(10).
			Build()

		result, err := engine.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(result.Spans) != 1 {
			t.Errorf("Expected 1 span, got %d", len(result.Spans))
		}

		if len(result.Spans) > 0 && result.Spans[0].Name != "Fast API Call" {
			t.Errorf("Expected 'Fast API Call', got '%s'", result.Spans[0].Name)
		}

		t.Logf("✅ Found span by operation in %v", result.QueryDuration)
	})

	// Test 2: Search by error status
	t.Run("SearchByError", func(t *testing.T) {
		query, _ := NewQuery().
			WithError().
			Limit(10).
			Build()

		result, err := engine.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(result.Spans) != 1 {
			t.Errorf("Expected 1 error span, got %d", len(result.Spans))
		}

		if len(result.Spans) > 0 && codes.Code(result.Spans[0].StatusCode) != codes.Error {
			t.Errorf("Expected error status, got %v", result.Spans[0].StatusCode)
		}

		t.Logf("✅ Found error spans in %v", result.QueryDuration)
	})

	// Test 3: Search by duration
	t.Run("SearchByDuration", func(t *testing.T) {
		query, _ := NewQuery().
			SlowerThan(100 * time.Millisecond).
			Limit(10).
			Build()

		result, err := engine.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(result.Spans) != 1 {
			t.Errorf("Expected 1 slow span, got %d", len(result.Spans))
		}

		if len(result.Spans) > 0 {
			duration := result.Spans[0].EndTime.Sub(result.Spans[0].StartTime)
			if duration < 100*time.Millisecond {
				t.Errorf("Expected duration > 100ms, got %v", duration)
			}
		}

		t.Logf("✅ Found slow spans in %v", result.QueryDuration)
	})

	// Test 4: Search by attribute
	t.Run("SearchByAttribute", func(t *testing.T) {
		query, _ := NewQuery().
			WithAttribute("type", "database").
			Limit(10).
			Build()

		result, err := engine.Search(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(result.Spans) != 1 {
			t.Errorf("Expected 1 database span, got %d", len(result.Spans))
		}

		t.Logf("✅ Found spans by attribute in %v", result.QueryDuration)
	})

	t.Log("✅ All search tests passed with index-based queries!")
}
