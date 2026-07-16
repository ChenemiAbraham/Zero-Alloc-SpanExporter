package search

import (
	"fmt"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	"go.opentelemetry.io/otel/trace"
)

// LogQuery represents a search query for log records
type LogQuery struct {
	// TraceID filters by specific trace ID
	TraceID *trace.TraceID

	// SpanID filters by specific span ID
	SpanID *trace.SpanID

	// SeverityMin filters logs with severity >= this level
	SeverityMin *int32

	// SeverityMax filters logs with severity <= this level
	SeverityMax *int32

	// TimeRange filters by timestamp
	TimeRange *TimeRange

	// BodyContains filters logs where body contains this text (case-insensitive)
	BodyContains string

	// Attributes filters by log attributes (key=value exact match)
	Attributes map[string]string

	// Limit is the maximum number of results to return
	Limit int

	// Offset is the number of results to skip (for pagination)
	Offset int
}

// LogResult represents log search results
type LogResult struct {
	// Logs matching the query
	Logs []*protocol.LogMessage

	// Total is the total number of matches (before limit/offset)
	Total int

	// Duration is how long the search took
	QueryDuration time.Duration
}

// LogQueryBuilder provides a fluent API for building log queries
type LogQueryBuilder struct {
	query LogQuery
}

// NewLogQuery creates a new log query builder
func NewLogQuery() *LogQueryBuilder {
	return &LogQueryBuilder{
		query: LogQuery{
			Limit: 100, // Default limit
		},
	}
}

func (b *LogQueryBuilder) ForTrace(traceID trace.TraceID) *LogQueryBuilder {
	b.query.TraceID = &traceID
	return b
}

func (b *LogQueryBuilder) ForSpan(spanID trace.SpanID) *LogQueryBuilder {
	b.query.SpanID = &spanID
	return b
}

func (b *LogQueryBuilder) SeverityMin(severity int32) *LogQueryBuilder {
	b.query.SeverityMin = &severity
	return b
}

func (b *LogQueryBuilder) SeverityMax(severity int32) *LogQueryBuilder {
	b.query.SeverityMax = &severity
	return b
}

func (b *LogQueryBuilder) SeverityRange(min, max int32) *LogQueryBuilder {
	b.query.SeverityMin = &min
	b.query.SeverityMax = &max
	return b
}

func (b *LogQueryBuilder) TimeRange(start, end time.Time) *LogQueryBuilder {
	b.query.TimeRange = &TimeRange{Start: start, End: end}
	return b
}

func (b *LogQueryBuilder) Last(duration time.Duration) *LogQueryBuilder {
	now := time.Now()
	b.query.TimeRange = &TimeRange{
		Start: now.Add(-duration),
		End:   now,
	}
	return b
}

func (b *LogQueryBuilder) BodyContains(text string) *LogQueryBuilder {
	b.query.BodyContains = text
	return b
}

func (b *LogQueryBuilder) WithAttribute(key, value string) *LogQueryBuilder {
	if b.query.Attributes == nil {
		b.query.Attributes = make(map[string]string)
	}
	b.query.Attributes[key] = value
	return b
}

func (b *LogQueryBuilder) Limit(limit int) *LogQueryBuilder {
	b.query.Limit = limit
	return b
}

func (b *LogQueryBuilder) Offset(offset int) *LogQueryBuilder {
	b.query.Offset = offset
	return b
}

func (b *LogQueryBuilder) Build() (LogQuery, error) {
	if err := b.query.Validate(); err != nil {
		return LogQuery{}, err
	}
	return b.query, nil
}

// Validate checks if the log query is valid
func (q *LogQuery) Validate() error {
	if q.Limit < 0 {
		return fmt.Errorf("limit must be non-negative")
	}
	if q.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}
	if q.TimeRange != nil {
		if q.TimeRange.Start.After(q.TimeRange.End) {
			return fmt.Errorf("time range start must be before end")
		}
	}
	if q.SeverityMin != nil && q.SeverityMax != nil {
		if *q.SeverityMin > *q.SeverityMax {
			return fmt.Errorf("severity min must be less than max")
		}
	}
	return nil
}

// SearchLogs executes a log search query
func (e *Engine) SearchLogs(query LogQuery) (*LogResult, error) {
	start := time.Now()

	// Validate query
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	// For trace ID queries, use direct lookup (optimized path)
	if query.TraceID != nil {
		logs, err := e.store.GetLogsByTraceID(*query.TraceID)
		if err != nil {
			return nil, fmt.Errorf("trace lookup failed: %w", err)
		}

		// Apply post-filters
		filtered := e.applyLogFilters(logs, query)

		return &LogResult{
			Logs:          filtered,
			Total:         len(filtered),
			QueryDuration: time.Since(start),
		}, nil
	}

	// For time-range queries, use time range lookup
	if query.TimeRange != nil {
		logs, err := e.store.GetLogsByTimeRange(query.TimeRange.Start, query.TimeRange.End)
		if err != nil {
			return nil, fmt.Errorf("time range query failed: %w", err)
		}

		// Apply post-filters
		filtered := e.applyLogFilters(logs, query)

		// Apply pagination
		total := len(filtered)
		paginated := e.paginateLogs(filtered, query.Limit, query.Offset)

		return &LogResult{
			Logs:          paginated,
			Total:         total,
			QueryDuration: time.Since(start),
		}, nil
	}

	// Otherwise, get recent logs and filter
	logs, err := e.store.GetRecentLogs(10000) // Scan last 10k logs
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	// Apply all filters
	filtered := e.applyLogFilters(logs, query)

	// Apply pagination
	total := len(filtered)
	paginated := e.paginateLogs(filtered, query.Limit, query.Offset)

	return &LogResult{
		Logs:          paginated,
		Total:         total,
		QueryDuration: time.Since(start),
	}, nil
}

// applyLogFilters applies all query filters to a list of logs
func (e *Engine) applyLogFilters(logs []*protocol.LogMessage, query LogQuery) []*protocol.LogMessage {
	var result []*protocol.LogMessage

	for _, log := range logs {
		if e.matchesLogFilters(log, query) {
			result = append(result, log)
		}
	}

	return result
}

// matchesLogFilters checks if a log matches all query filters
func (e *Engine) matchesLogFilters(log *protocol.LogMessage, query LogQuery) bool {
	// SpanID filter
	if query.SpanID != nil && log.SpanID != *query.SpanID {
		return false
	}

	// Severity min filter
	if query.SeverityMin != nil && log.SeverityNumber < *query.SeverityMin {
		return false
	}

	// Severity max filter
	if query.SeverityMax != nil && log.SeverityNumber > *query.SeverityMax {
		return false
	}

	// Time range filter
	if query.TimeRange != nil {
		if log.Timestamp.Before(query.TimeRange.Start) || log.Timestamp.After(query.TimeRange.End) {
			return false
		}
	}

	// Body contains filter (case-insensitive)
	if query.BodyContains != "" {
		// Simple case-insensitive substring search
		// For production, consider using a proper text search library
		bodyLower := toLower(log.Body)
		searchLower := toLower(query.BodyContains)
		if !contains(bodyLower, searchLower) {
			return false
		}
	}

	// Attribute filters
	for key, value := range query.Attributes {
		logValue, ok := log.Attributes[key]
		if !ok {
			return false
		}

		// Convert to string for comparison
		logValueStr := fmt.Sprintf("%v", logValue)
		if logValueStr != value {
			return false
		}
	}

	return true
}

// paginateLogs applies limit and offset to log results
func (e *Engine) paginateLogs(logs []*protocol.LogMessage, limit, offset int) []*protocol.LogMessage {
	// Apply offset
	if offset >= len(logs) {
		return []*protocol.LogMessage{}
	}
	logs = logs[offset:]

	// Apply limit
	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}

	return logs
}

// Helper functions for string operations
func toLower(s string) string {
	// Simple ASCII lowercase (for more robust solution, use strings.ToLower)
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(needle) > len(haystack) {
		return false
	}

	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// Common severity levels (OpenTelemetry standard)
const (
	SeverityTrace int32 = 1
	SeverityDebug int32 = 5
	SeverityInfo  int32 = 9
	SeverityWarn  int32 = 13
	SeverityError int32 = 17
	SeverityFatal int32 = 21
)

// Helper methods for common severity filters
func (b *LogQueryBuilder) InfoAndAbove() *LogQueryBuilder {
	return b.SeverityMin(SeverityInfo)
}

func (b *LogQueryBuilder) WarnAndAbove() *LogQueryBuilder {
	return b.SeverityMin(SeverityWarn)
}

func (b *LogQueryBuilder) ErrorAndAbove() *LogQueryBuilder {
	return b.SeverityMin(SeverityError)
}

func (b *LogQueryBuilder) OnlyErrors() *LogQueryBuilder {
	return b.SeverityRange(SeverityError, SeverityFatal)
}
