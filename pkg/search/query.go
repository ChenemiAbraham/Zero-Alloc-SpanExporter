package search

import (
	"fmt"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	"go.opentelemetry.io/otel/codes"
)

// Query represents a search query for spans
type Query struct {
	// Service filters by service name (extracted from resource attributes)
	Service string

	// Operation filters by span name/operation
	Operation string

	// Status filters by status code
	Status *codes.Code

	// TimeRange filters by timestamp
	TimeRange *TimeRange

	// DurationRange filters by span duration
	DurationRange *DurationRange

	// Attributes filters by span attributes (key=value exact match)
	Attributes map[string]string

	// HasError filters spans with errors
	HasError *bool

	// TraceID filters by specific trace ID
	TraceID *[16]byte

	// Limit is the maximum number of results to return
	Limit int

	// Offset is the number of results to skip (for pagination)
	Offset int
}

// TimeRange represents a time range filter
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// DurationRange represents a duration range filter
type DurationRange struct {
	Min time.Duration
	Max time.Duration
}

// Validate checks if the query is valid
func (q *Query) Validate() error {
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
	if q.DurationRange != nil {
		if q.DurationRange.Min > q.DurationRange.Max {
			return fmt.Errorf("duration range min must be less than max")
		}
		if q.DurationRange.Min < 0 || q.DurationRange.Max < 0 {
			return fmt.Errorf("duration range must be non-negative")
		}
	}
	return nil
}

// Builder provides a fluent API for building queries
type Builder struct {
	query Query
}

// NewQuery creates a new query builder
func NewQuery() *Builder {
	return &Builder{
		query: Query{
			Limit: 100, // Default limit
		},
	}
}

func (b *Builder) Service(service string) *Builder {
	b.query.Service = service
	return b
}

func (b *Builder) Operation(operation string) *Builder {
	b.query.Operation = operation
	return b
}

func (b *Builder) Status(status codes.Code) *Builder {
	b.query.Status = &status
	return b
}

func (b *Builder) TimeRange(start, end time.Time) *Builder {
	b.query.TimeRange = &TimeRange{Start: start, End: end}
	return b
}

func (b *Builder) Last(duration time.Duration) *Builder {
	now := time.Now()
	b.query.TimeRange = &TimeRange{
		Start: now.Add(-duration),
		End:   now,
	}
	return b
}

func (b *Builder) DurationRange(min, max time.Duration) *Builder {
	b.query.DurationRange = &DurationRange{Min: min, Max: max}
	return b
}

func (b *Builder) SlowerThan(duration time.Duration) *Builder {
	b.query.DurationRange = &DurationRange{
		Min: duration,
		Max: time.Hour * 24, // Effectively unlimited
	}
	return b
}

func (b *Builder) FasterThan(duration time.Duration) *Builder {
	b.query.DurationRange = &DurationRange{
		Min: 0,
		Max: duration,
	}
	return b
}

func (b *Builder) WithAttribute(key, value string) *Builder {
	if b.query.Attributes == nil {
		b.query.Attributes = make(map[string]string)
	}
	b.query.Attributes[key] = value
	return b
}

func (b *Builder) WithError() *Builder {
	hasError := true
	b.query.HasError = &hasError
	return b
}

func (b *Builder) WithoutError() *Builder {
	hasError := false
	b.query.HasError = &hasError
	return b
}

func (b *Builder) ForTrace(traceID [16]byte) *Builder {
	b.query.TraceID = &traceID
	return b
}

func (b *Builder) Limit(limit int) *Builder {
	b.query.Limit = limit
	return b
}

func (b *Builder) Offset(offset int) *Builder {
	b.query.Offset = offset
	return b
}

func (b *Builder) Build() (Query, error) {
	if err := b.query.Validate(); err != nil {
		return Query{}, err
	}
	return b.query, nil
}

// Result represents a search result
type Result struct {
	// Spans matching the query
	Spans []*protocol.SpanMessage

	// Total is the total number of matches (before limit/offset)
	Total int

	// Duration is how long the search took
	QueryDuration time.Duration
}
