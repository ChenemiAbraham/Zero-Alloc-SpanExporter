package search

import (
	"os"
	"testing"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
	"go.opentelemetry.io/otel/trace"
)

func TestLogSearch(t *testing.T) {
	// Create temporary storage
	dir, err := os.MkdirTemp("", "log-search-test-*")
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

	// Store test logs
	now := time.Now()
	traceID1 := trace.TraceID{1}
	traceID2 := trace.TraceID{2}

	logs := []*protocol.LogMessage{
		{
			Timestamp:      now,
			TraceID:        traceID1,
			SpanID:         trace.SpanID{1},
			SeverityNumber: SeverityInfo,
			SeverityText:   "INFO",
			Body:           "User logged in successfully",
			Attributes:     map[string]interface{}{"user_id": "123"},
		},
		{
			Timestamp:      now.Add(10 * time.Millisecond),
			TraceID:        traceID1,
			SpanID:         trace.SpanID{1},
			SeverityNumber: SeverityWarn,
			SeverityText:   "WARN",
			Body:           "Slow database query detected",
			Attributes:     map[string]interface{}{"query_time": "250ms"},
		},
		{
			Timestamp:      now.Add(20 * time.Millisecond),
			TraceID:        traceID1,
			SpanID:         trace.SpanID{2},
			SeverityNumber: SeverityError,
			SeverityText:   "ERROR",
			Body:           "Payment failed: insufficient funds",
			Attributes:     map[string]interface{}{"amount": "100.00"},
		},
		{
			Timestamp:      now.Add(30 * time.Millisecond),
			TraceID:        traceID2,
			SpanID:         trace.SpanID{3},
			SeverityNumber: SeverityInfo,
			SeverityText:   "INFO",
			Body:           "API request completed",
			Attributes:     map[string]interface{}{"status": "200"},
		},
	}

	for _, log := range logs {
		if err := store.StoreLog(log); err != nil {
			t.Fatalf("Failed to store log: %v", err)
		}
	}

	// Wait for writes to settle
	time.Sleep(100 * time.Millisecond)

	// Test 1: Search by trace ID
	t.Run("SearchByTraceID", func(t *testing.T) {
		query, _ := NewLogQuery().
			ForTrace(traceID1).
			Build()

		result, err := engine.SearchLogs(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if result.Total != 3 {
			t.Errorf("Expected 3 logs for trace1, got %d", result.Total)
		}

		t.Logf("✅ Found %d logs for trace in %v", result.Total, result.QueryDuration)
	})

	// Test 2: Search by severity
	t.Run("SearchBySeverity", func(t *testing.T) {
		query, _ := NewLogQuery().
			ErrorAndAbove().
			Last(1 * time.Hour).
			Build()

		result, err := engine.SearchLogs(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("Expected 1 error log, got %d", result.Total)
		}

		if result.Total > 0 && result.Logs[0].Body != "Payment failed: insufficient funds" {
			t.Errorf("Expected payment error, got: %s", result.Logs[0].Body)
		}

		t.Logf("✅ Found %d error logs in %v", result.Total, result.QueryDuration)
	})

	// Test 3: Search by body text
	t.Run("SearchByBodyText", func(t *testing.T) {
		query, _ := NewLogQuery().
			BodyContains("payment").
			Last(1 * time.Hour).
			Build()

		result, err := engine.SearchLogs(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("Expected 1 log with 'payment', got %d", result.Total)
		}

		t.Logf("✅ Found %d logs matching text in %v", result.Total, result.QueryDuration)
	})

	// Test 4: Search by attribute
	t.Run("SearchByAttribute", func(t *testing.T) {
		query, _ := NewLogQuery().
			WithAttribute("user_id", "123").
			Last(1 * time.Hour).
			Build()

		result, err := engine.SearchLogs(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if result.Total != 1 {
			t.Errorf("Expected 1 log with user_id=123, got %d", result.Total)
		}

		t.Logf("✅ Found %d logs by attribute in %v", result.Total, result.QueryDuration)
	})

	// Test 5: Combined filters
	t.Run("CombinedFilters", func(t *testing.T) {
		query, _ := NewLogQuery().
			ForTrace(traceID1).
			WarnAndAbove().
			Last(1 * time.Hour).
			Build()

		result, err := engine.SearchLogs(query)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Should find 2 logs: WARN and ERROR
		if result.Total != 2 {
			t.Errorf("Expected 2 logs (WARN+ERROR), got %d", result.Total)
		}

		t.Logf("✅ Found %d logs with combined filters in %v", result.Total, result.QueryDuration)
	})

	t.Log("✅ All log search tests passed!")
}

func TestTraceLogCorrelation(t *testing.T) {
	// Create temporary storage
	dir, err := os.MkdirTemp("", "correlation-test-*")
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

	engine := NewEngine(store)

	// Create a trace with spans and logs
	now := time.Now()
	traceID := trace.TraceID{1}
	spanID1 := trace.SpanID{1}
	spanID2 := trace.SpanID{2}

	// Store spans
	spans := []*protocol.SpanMessage{
		{
			TraceID:    traceID,
			SpanID:     spanID1,
			Name:       "HTTP Handler",
			StartTime:  now,
			EndTime:    now.Add(100 * time.Millisecond),
			StatusCode: 0, // OK
		},
		{
			TraceID:    traceID,
			SpanID:     spanID2,
			Name:       "Database Query",
			StartTime:  now.Add(10 * time.Millisecond),
			EndTime:    now.Add(90 * time.Millisecond),
			StatusCode: 2, // Error
		},
	}

	for _, span := range spans {
		if err := store.Store(span); err != nil {
			t.Fatalf("Failed to store span: %v", err)
		}
	}

	// Store logs
	logs := []*protocol.LogMessage{
		{
			Timestamp:      now.Add(5 * time.Millisecond),
			TraceID:        traceID,
			SpanID:         spanID1,
			SeverityNumber: SeverityInfo,
			Body:           "Request received",
		},
		{
			Timestamp:      now.Add(15 * time.Millisecond),
			TraceID:        traceID,
			SpanID:         spanID2,
			SeverityNumber: SeverityWarn,
			Body:           "Query is slow",
		},
		{
			Timestamp:      now.Add(85 * time.Millisecond),
			TraceID:        traceID,
			SpanID:         spanID2,
			SeverityNumber: SeverityError,
			Body:           "Query failed",
		},
	}

	for _, log := range logs {
		if err := store.StoreLog(log); err != nil {
			t.Fatalf("Failed to store log: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Test: GetTraceWithLogs
	t.Run("GetTraceWithLogs", func(t *testing.T) {
		result, err := engine.GetTraceWithLogs(traceID)
		if err != nil {
			t.Fatalf("Failed to get trace with logs: %v", err)
		}

		if len(result.Spans) != 2 {
			t.Errorf("Expected 2 spans, got %d", len(result.Spans))
		}

		if len(result.Logs) != 3 {
			t.Errorf("Expected 3 logs, got %d", len(result.Logs))
		}

		t.Logf("✅ Retrieved trace with %d spans and %d logs", len(result.Spans), len(result.Logs))
	})

	// Test: GetSpanWithLogs
	t.Run("GetSpanWithLogs", func(t *testing.T) {
		result, err := engine.GetSpanWithLogs(traceID, spanID2)
		if err != nil {
			t.Fatalf("Failed to get span with logs: %v", err)
		}

		if result.Span.Name != "Database Query" {
			t.Errorf("Expected Database Query span, got %s", result.Span.Name)
		}

		// Should have 2 logs for this span
		if len(result.Logs) != 2 {
			t.Errorf("Expected 2 logs for span, got %d", len(result.Logs))
		}

		t.Logf("✅ Retrieved span with %d logs", len(result.Logs))
	})

	// Test: Correlation Summary
	t.Run("CorrelationSummary", func(t *testing.T) {
		summary, err := engine.GetCorrelationSummary(traceID)
		if err != nil {
			t.Fatalf("Failed to get summary: %v", err)
		}

		if summary.TotalSpans != 2 {
			t.Errorf("Expected 2 spans, got %d", summary.TotalSpans)
		}

		if summary.TotalLogs != 3 {
			t.Errorf("Expected 3 logs, got %d", summary.TotalLogs)
		}

		if summary.SpansWithLogs != 2 {
			t.Errorf("Expected 2 spans with logs, got %d", summary.SpansWithLogs)
		}

		t.Logf("✅ Correlation summary: %d spans, %d logs, %.1f avg logs/span",
			summary.TotalSpans, summary.TotalLogs, summary.AverageLogsPerSpan)
	})

	// Test: Timeline
	t.Run("Timeline", func(t *testing.T) {
		timeline, err := engine.GetTimeline(traceID)
		if err != nil {
			t.Fatalf("Failed to get timeline: %v", err)
		}

		// Should have: 2 span_start + 2 span_end + 3 log = 7 events
		if len(timeline) != 7 {
			t.Errorf("Expected 7 timeline events, got %d", len(timeline))
		}

		// Verify chronological order
		for i := 1; i < len(timeline); i++ {
			if timeline[i].Timestamp.Before(timeline[i-1].Timestamp) {
				t.Errorf("Timeline not in chronological order at index %d", i)
			}
		}

		t.Logf("✅ Timeline has %d events in chronological order", len(timeline))
	})

	t.Log("✅ All trace-log correlation tests passed!")
}
