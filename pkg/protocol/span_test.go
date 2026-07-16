package protocol

import (
	"bytes"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
)

// TestEncodeDecodeRoundTrip tests that encoding and decoding are inverses
func TestEncodeDecodeRoundTrip(t *testing.T) {
	original := &SpanMessage{
		TraceID:    [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		ParentID:   [8]byte{0, 0, 0, 0, 0, 0, 0, 0},
		Name:       "test-span",
		StartTime:  time.Unix(1234567890, 123456789),
		EndTime:    time.Unix(1234567891, 987654321),
		StatusCode: codes.Ok,
		StatusMsg:  "success",
		Attributes: map[string]interface{}{
			"string_attr": "hello",
			"int_attr":    int64(42),
			"float_attr":  3.14,
			"bool_attr":   true,
		},
		Events: []SpanEvent{
			{
				Name:      "event1",
				Timestamp: time.Unix(1234567890, 500000000),
				Attributes: map[string]interface{}{
					"event_key": "event_value",
				},
			},
		},
	}

	// Encode
	buf := &bytes.Buffer{}
	if err := original.EncodeTo(buf); err != nil {
		t.Fatalf("EncodeTo failed: %v", err)
	}

	// Decode
	decoded, err := DecodeFrom(buf)
	if err != nil {
		t.Fatalf("DecodeFrom failed: %v", err)
	}

	// Verify
	if decoded.Name != original.Name {
		t.Errorf("Name mismatch: got %q, want %q", decoded.Name, original.Name)
	}

	if decoded.TraceID != original.TraceID {
		t.Errorf("TraceID mismatch")
	}

	if decoded.SpanID != original.SpanID {
		t.Errorf("SpanID mismatch")
	}

	if decoded.StatusCode != original.StatusCode {
		t.Errorf("StatusCode mismatch: got %v, want %v", decoded.StatusCode, original.StatusCode)
	}

	if decoded.StatusMsg != original.StatusMsg {
		t.Errorf("StatusMsg mismatch: got %q, want %q", decoded.StatusMsg, original.StatusMsg)
	}

	// Check timestamps (within millisecond due to precision)
	if decoded.StartTime.UnixNano() != original.StartTime.UnixNano() {
		t.Errorf("StartTime mismatch: got %v, want %v", decoded.StartTime, original.StartTime)
	}

	// Check attributes
	if len(decoded.Attributes) != len(original.Attributes) {
		t.Errorf("Attributes count mismatch: got %d, want %d", len(decoded.Attributes), len(original.Attributes))
	}

	if decoded.Attributes["string_attr"] != original.Attributes["string_attr"] {
		t.Errorf("string_attr mismatch")
	}

	if decoded.Attributes["int_attr"] != original.Attributes["int_attr"] {
		t.Errorf("int_attr mismatch: got %v, want %v", decoded.Attributes["int_attr"], original.Attributes["int_attr"])
	}

	// Check events
	if len(decoded.Events) != len(original.Events) {
		t.Errorf("Events count mismatch: got %d, want %d", len(decoded.Events), len(original.Events))
	}

	if len(decoded.Events) > 0 {
		if decoded.Events[0].Name != original.Events[0].Name {
			t.Errorf("Event name mismatch")
		}
	}
}

// TestEmptySpan tests encoding/decoding of an empty span
func TestEmptySpan(t *testing.T) {
	original := &SpanMessage{
		Name:       "empty-span",
		StartTime:  time.Now(),
		EndTime:    time.Now(),
		StatusCode: codes.Ok,
		Attributes: make(map[string]interface{}),
		Events:     make([]SpanEvent, 0),
	}

	buf := &bytes.Buffer{}
	if err := original.EncodeTo(buf); err != nil {
		t.Fatalf("EncodeTo failed: %v", err)
	}

	decoded, err := DecodeFrom(buf)
	if err != nil {
		t.Fatalf("DecodeFrom failed: %v", err)
	}

	if decoded.Name != original.Name {
		t.Errorf("Name mismatch")
	}
}

// TestLargeSpan tests encoding/decoding of a span with many attributes
func TestLargeSpan(t *testing.T) {
	original := &SpanMessage{
		Name:       "large-span",
		StartTime:  time.Now(),
		EndTime:    time.Now(),
		StatusCode: codes.Ok,
		Attributes: make(map[string]interface{}),
		Events:     make([]SpanEvent, 0),
	}

	// Add 100 attributes
	for i := 0; i < 100; i++ {
		key := string(rune('a' + (i % 26)))
		original.Attributes[key] = int64(i)
	}

	buf := &bytes.Buffer{}
	if err := original.EncodeTo(buf); err != nil {
		t.Fatalf("EncodeTo failed: %v", err)
	}

	decoded, err := DecodeFrom(buf)
	if err != nil {
		t.Fatalf("DecodeFrom failed: %v", err)
	}

	if len(decoded.Attributes) != len(original.Attributes) {
		t.Errorf("Attributes count mismatch: got %d, want %d", len(decoded.Attributes), len(original.Attributes))
	}
}

// BenchmarkEncode benchmarks the encoding performance
func BenchmarkEncode(b *testing.B) {
	msg := &SpanMessage{
		TraceID:    [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		Name:       "benchmark-span",
		StartTime:  time.Now(),
		EndTime:    time.Now(),
		StatusCode: codes.Ok,
		StatusMsg:  "ok",
		Attributes: map[string]interface{}{
			"key1": "value1",
			"key2": int64(42),
			"key3": 3.14,
		},
		Events: []SpanEvent{},
	}

	buf := &bytes.Buffer{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		if err := msg.EncodeTo(buf); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportMetric(float64(buf.Len()), "bytes/span")
}

// BenchmarkDecode benchmarks the decoding performance
func BenchmarkDecode(b *testing.B) {
	msg := &SpanMessage{
		TraceID:    [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		Name:       "benchmark-span",
		StartTime:  time.Now(),
		EndTime:    time.Now(),
		StatusCode: codes.Ok,
		StatusMsg:  "ok",
		Attributes: map[string]interface{}{
			"key1": "value1",
			"key2": int64(42),
		},
		Events: []SpanEvent{},
	}

	buf := &bytes.Buffer{}
	msg.EncodeTo(buf)
	encoded := buf.Bytes()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(encoded)
		if _, err := DecodeFrom(reader); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEncodeDecodeRoundTrip benchmarks the full round trip
func BenchmarkEncodeDecodeRoundTrip(b *testing.B) {
	msg := &SpanMessage{
		TraceID:    [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
		Name:       "benchmark-span",
		StartTime:  time.Now(),
		EndTime:    time.Now(),
		StatusCode: codes.Ok,
		Attributes: map[string]interface{}{
			"key1": "value1",
		},
		Events: []SpanEvent{},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf := &bytes.Buffer{}
		if err := msg.EncodeTo(buf); err != nil {
			b.Fatal(err)
		}
		if _, err := DecodeFrom(buf); err != nil {
			b.Fatal(err)
		}
	}
}
