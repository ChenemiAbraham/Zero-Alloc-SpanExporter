package search

import (
	"fmt"
	"sort"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	"go.opentelemetry.io/otel/trace"
)

// TraceWithLogs represents a trace with its correlated logs
type TraceWithLogs struct {
	TraceID trace.TraceID
	Spans   []*protocol.SpanMessage
	Logs    []*protocol.LogMessage
	Start   time.Time
	End     time.Time
}

// SpanWithLogs represents a span with its correlated logs
type SpanWithLogs struct {
	Span *protocol.SpanMessage
	Logs []*protocol.LogMessage
}

// GetTraceWithLogs retrieves a complete trace with all correlated logs
func (e *Engine) GetTraceWithLogs(traceID trace.TraceID) (*TraceWithLogs, error) {
	// Get all spans for this trace
	spans, err := e.store.GetByTraceID(traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get spans: %w", err)
	}

	if len(spans) == 0 {
		return nil, fmt.Errorf("trace not found: %s", traceID.String())
	}

	// Get all logs for this trace
	logs, err := e.store.GetLogsByTraceID(traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	// Calculate trace time range
	start, end := calculateTimeRange(spans)

	result := &TraceWithLogs{
		TraceID: traceID,
		Spans:   spans,
		Logs:    logs,
		Start:   start,
		End:     end,
	}

	// Sort spans by start time
	sort.Slice(result.Spans, func(i, j int) bool {
		return result.Spans[i].StartTime.Before(result.Spans[j].StartTime)
	})

	// Sort logs by timestamp
	sort.Slice(result.Logs, func(i, j int) bool {
		return result.Logs[i].Timestamp.Before(result.Logs[j].Timestamp)
	})

	return result, nil
}

// GetSpanWithLogs retrieves a span with its correlated logs
func (e *Engine) GetSpanWithLogs(traceID trace.TraceID, spanID trace.SpanID) (*SpanWithLogs, error) {
	// Get the specific span
	spans, err := e.store.GetByTraceID(traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get spans: %w", err)
	}

	var targetSpan *protocol.SpanMessage
	for _, span := range spans {
		if span.SpanID == spanID {
			targetSpan = span
			break
		}
	}

	if targetSpan == nil {
		return nil, fmt.Errorf("span not found: %s", spanID.String())
	}

	// Get all logs for this trace
	allLogs, err := e.store.GetLogsByTraceID(traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	// Filter logs for this specific span
	var spanLogs []*protocol.LogMessage
	for _, log := range allLogs {
		if log.SpanID == spanID {
			spanLogs = append(spanLogs, log)
		}
	}

	// Sort logs by timestamp
	sort.Slice(spanLogs, func(i, j int) bool {
		return spanLogs[i].Timestamp.Before(spanLogs[j].Timestamp)
	})

	return &SpanWithLogs{
		Span: targetSpan,
		Logs: spanLogs,
	}, nil
}

// GetLogsForTimeRange retrieves logs within a trace's time range
func (e *Engine) GetLogsForTimeRange(traceID trace.TraceID, start, end time.Time) ([]*protocol.LogMessage, error) {
	// Get all logs for this trace
	logs, err := e.store.GetLogsByTraceID(traceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	// Filter by time range
	var filtered []*protocol.LogMessage
	for _, log := range logs {
		if log.Timestamp.After(start) && log.Timestamp.Before(end) {
			filtered = append(filtered, log)
		}
	}

	// Sort by timestamp
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})

	return filtered, nil
}

// FindTracesWithErrors finds all traces that have error logs
func (e *Engine) FindTracesWithErrors(timeRange TimeRange, limit int) ([]trace.TraceID, error) {
	// Search for error logs
	query, _ := NewLogQuery().
		ErrorAndAbove().
		TimeRange(timeRange.Start, timeRange.End).
		Limit(limit * 10). // Get more logs to find unique traces
		Build()

	result, err := e.SearchLogs(query)
	if err != nil {
		return nil, err
	}

	// Extract unique trace IDs
	traceSet := make(map[trace.TraceID]bool)
	var traces []trace.TraceID

	for _, log := range result.Logs {
		if !traceSet[log.TraceID] {
			traceSet[log.TraceID] = true
			traces = append(traces, log.TraceID)

			if len(traces) >= limit {
				break
			}
		}
	}

	return traces, nil
}

// CorrelationSummary provides statistics about trace-log correlation
type CorrelationSummary struct {
	TotalSpans       int
	TotalLogs        int
	SpansWithLogs    int
	SpansWithoutLogs int
	LogsWithSpan     int
	LogsWithoutSpan  int
	AverageLogsPerSpan float64
}

// GetCorrelationSummary analyzes trace-log correlation for a trace
func (e *Engine) GetCorrelationSummary(traceID trace.TraceID) (*CorrelationSummary, error) {
	trace, err := e.GetTraceWithLogs(traceID)
	if err != nil {
		return nil, err
	}

	summary := &CorrelationSummary{
		TotalSpans: len(trace.Spans),
		TotalLogs:  len(trace.Logs),
	}

	// Count spans with logs
	spanLogCount := make(map[[8]byte]int)
	for _, log := range trace.Logs {
		spanLogCount[log.SpanID]++
	}

	for _, span := range trace.Spans {
		if count, ok := spanLogCount[span.SpanID]; ok && count > 0 {
			summary.SpansWithLogs++
		} else {
			summary.SpansWithoutLogs++
		}
	}

	// Count logs with valid span IDs
	validSpans := make(map[[8]byte]bool)
	for _, span := range trace.Spans {
		validSpans[span.SpanID] = true
	}

	for _, log := range trace.Logs {
		if validSpans[log.SpanID] {
			summary.LogsWithSpan++
		} else {
			summary.LogsWithoutSpan++
		}
	}

	// Calculate average
	if summary.TotalSpans > 0 {
		summary.AverageLogsPerSpan = float64(summary.TotalLogs) / float64(summary.TotalSpans)
	}

	return summary, nil
}

// GetTimeline returns a merged timeline of spans and logs
type TimelineEvent struct {
	Timestamp time.Time
	Type      string // "span_start", "span_end", "log"
	Span      *protocol.SpanMessage
	Log       *protocol.LogMessage
}

func (e *Engine) GetTimeline(traceID trace.TraceID) ([]TimelineEvent, error) {
	trace, err := e.GetTraceWithLogs(traceID)
	if err != nil {
		return nil, err
	}

	var events []TimelineEvent

	// Add span start/end events
	for _, span := range trace.Spans {
		events = append(events, TimelineEvent{
			Timestamp: span.StartTime,
			Type:      "span_start",
			Span:      span,
		})
		events = append(events, TimelineEvent{
			Timestamp: span.EndTime,
			Type:      "span_end",
			Span:      span,
		})
	}

	// Add log events
	for _, log := range trace.Logs {
		events = append(events, TimelineEvent{
			Timestamp: log.Timestamp,
			Type:      "log",
			Log:       log,
		})
	}

	// Sort by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events, nil
}

// Helper function to calculate time range from spans
func calculateTimeRange(spans []*protocol.SpanMessage) (time.Time, time.Time) {
	if len(spans) == 0 {
		return time.Time{}, time.Time{}
	}

	start := spans[0].StartTime
	end := spans[0].EndTime

	for _, span := range spans[1:] {
		if span.StartTime.Before(start) {
			start = span.StartTime
		}
		if span.EndTime.After(end) {
			end = span.EndTime
		}
	}

	return start, end
}
