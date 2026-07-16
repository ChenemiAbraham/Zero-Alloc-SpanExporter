package search

import (
	"fmt"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
	badger "github.com/dgraph-io/badger/v4"
	"go.opentelemetry.io/otel/codes"
)

// Engine provides search capabilities over stored spans
type Engine struct {
	store *storage.Store
}

// NewEngine creates a new search engine
func NewEngine(store *storage.Store) *Engine {
	return &Engine{store: store}
}

// Search executes a query and returns matching spans
func (e *Engine) Search(query Query) (*Result, error) {
	start := time.Now()

	// Validate query
	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	// For trace ID queries, use direct lookup
	if query.TraceID != nil {
		spans, err := e.store.GetByTraceID(*query.TraceID)
		if err != nil {
			return nil, fmt.Errorf("trace lookup failed: %w", err)
		}

		// Apply post-filters
		filtered := e.applyFilters(spans, query)

		return &Result{
			Spans:         filtered,
			Total:         len(filtered),
			QueryDuration: time.Since(start),
		}, nil
	}

	// For time-range queries without other filters, use storage's time range query
	if query.TimeRange != nil && e.isSimpleTimeQuery(query) {
		spans, err := e.store.GetByTimeRange(query.TimeRange.Start, query.TimeRange.End)
		if err != nil {
			return nil, fmt.Errorf("time range query failed: %w", err)
		}

		// Apply post-filters
		filtered := e.applyFilters(spans, query)

		// Apply limit/offset
		total := len(filtered)
		paginated := e.paginate(filtered, query.Limit, query.Offset)

		return &Result{
			Spans:         paginated,
			Total:         total,
			QueryDuration: time.Since(start),
		}, nil
	}

	// For filtered queries, use indexes
	spans, err := e.searchWithIndexes(query)
	if err != nil {
		return nil, fmt.Errorf("index search failed: %w", err)
	}

	return &Result{
		Spans:         spans,
		Total:         len(spans),
		QueryDuration: time.Since(start),
	}, nil
}

// isSimpleTimeQuery checks if query is just a time range with no other filters
func (e *Engine) isSimpleTimeQuery(query Query) bool {
	return query.Operation == "" &&
		query.Service == "" &&
		query.Status == nil &&
		query.HasError == nil &&
		query.DurationRange == nil &&
		len(query.Attributes) == 0
}

// searchWithIndexes uses secondary indexes for filtering
func (e *Engine) searchWithIndexes(query Query) ([]*protocol.SpanMessage, error) {
	// Choose which index to use based on query
	var indexPrefix string
	var seekKey []byte

	// Priority: Operation > Status > Duration > Attributes
	if query.Operation != "" {
		indexPrefix = "idx:op:" + query.Operation + ":"
		seekKey = []byte(indexPrefix)
	} else if query.Status != nil {
		indexPrefix = fmt.Sprintf("idx:status:%d:", *query.Status)
		seekKey = []byte(indexPrefix)
	} else if query.HasError != nil && *query.HasError {
		// Error status code
		indexPrefix = fmt.Sprintf("idx:status:%d:", codes.Error)
		seekKey = []byte(indexPrefix)
	} else if query.DurationRange != nil {
		// Find appropriate duration bucket(s)
		minBucket := getDurationBucket(query.DurationRange.Min)
		indexPrefix = fmt.Sprintf("idx:dur:%02d:", minBucket)
		seekKey = []byte(indexPrefix)
	} else if len(query.Attributes) > 0 {
		// Pick first attribute for index scan
		for key, value := range query.Attributes {
			indexPrefix = fmt.Sprintf("idx:attr:%s:%s:", key, value)
			seekKey = []byte(indexPrefix)
			break
		}
	} else {
		// No suitable index, fall back to recent scan
		recent, err := e.store.GetRecent(10000)
		if err != nil {
			return nil, err
		}
		filtered := e.applyFilters(recent, query)
		paginated := e.paginate(filtered, query.Limit, query.Offset)
		return paginated, nil
	}

	// Scan the selected index
	spans, err := e.scanIndex(indexPrefix, seekKey, query)
	if err != nil {
		return nil, err
	}

	return spans, nil
}

// scanIndex scans an index and retrieves matching spans
func (e *Engine) scanIndex(indexPrefix string, seekKey []byte, query Query) ([]*protocol.SpanMessage, error) {
	var spans []*protocol.SpanMessage
	seenKeys := make(map[string]bool)

	err := e.store.DB().View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // Index keys have empty values

		it := txn.NewIterator(opts)
		defer it.Close()

		// Scan index entries
		for it.Seek(seekKey); it.Valid(); it.Next() {
			key := it.Item().Key()

			// Stop if we've moved past this index prefix
			if len(key) < len(indexPrefix) || string(key[:len(indexPrefix)]) != indexPrefix {
				break
			}

			// Extract primary span key from index key
			spanKey := extractSpanKey(key)
			if spanKey == nil {
				continue
			}

			// Avoid duplicates (same span may match multiple attributes)
			keyStr := string(spanKey)
			if seenKeys[keyStr] {
				continue
			}
			seenKeys[keyStr] = true

			// Fetch the actual span
			item, err := txn.Get(spanKey)
			if err != nil {
				continue // Span might have expired
			}

			err = item.Value(func(val []byte) error {
				span, err := protocol.DecodePayload(val)
				if err != nil {
					return err
				}

				// Apply additional filters
				if e.matchesFilters(span, query) {
					spans = append(spans, span)
				}

				return nil
			})
			if err != nil {
				return err
			}

			// Stop if we've collected enough (with room for filtering)
			if len(spans) >= query.Limit*2 {
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Apply pagination
	paginated := e.paginate(spans, query.Limit, query.Offset)
	return paginated, nil
}

// getDurationBucket returns a bucket number for duration-based indexing
func getDurationBucket(duration time.Duration) int {
	ms := duration.Milliseconds()

	if ms < 10 {
		return 0
	} else if ms < 100 {
		return 1
	} else if ms < 1000 {
		return 2
	} else if ms < 10000 {
		return 3
	} else if ms < 100000 {
		return 4
	}
	return 5
}

// extractSpanKey extracts the primary span key from an index key
func extractSpanKey(indexKey []byte) []byte {
	// Index keys end with: <timestamp:8><trace_id:16><span_id:8>
	if len(indexKey) < 32 {
		return nil
	}
	return indexKey[len(indexKey)-32:]
}

// applyFilters applies all query filters to a list of spans
func (e *Engine) applyFilters(spans []*protocol.SpanMessage, query Query) []*protocol.SpanMessage {
	var result []*protocol.SpanMessage

	for _, span := range spans {
		if e.matchesFilters(span, query) {
			result = append(result, span)
		}
	}

	return result
}

// matchesFilters checks if a span matches all query filters
func (e *Engine) matchesFilters(span *protocol.SpanMessage, query Query) bool {
	// Operation filter
	if query.Operation != "" && span.Name != query.Operation {
		return false
	}

	// Status filter
	if query.Status != nil && codes.Code(span.StatusCode) != *query.Status {
		return false
	}

	// Error filter
	if query.HasError != nil {
		hasError := codes.Code(span.StatusCode) == codes.Error
		if *query.HasError != hasError {
			return false
		}
	}

	// Duration filter
	if query.DurationRange != nil {
		duration := span.EndTime.Sub(span.StartTime)
		if duration < query.DurationRange.Min || duration > query.DurationRange.Max {
			return false
		}
	}

	// Time range filter
	if query.TimeRange != nil {
		if span.StartTime.Before(query.TimeRange.Start) || span.StartTime.After(query.TimeRange.End) {
			return false
		}
	}

	// Attribute filters
	for key, value := range query.Attributes {
		spanValue, ok := span.Attributes[key]
		if !ok {
			return false
		}

		// Convert to string for comparison
		spanValueStr := fmt.Sprintf("%v", spanValue)
		if spanValueStr != value {
			return false
		}
	}

	return true
}

// paginate applies limit and offset to results
func (e *Engine) paginate(spans []*protocol.SpanMessage, limit, offset int) []*protocol.SpanMessage {
	// Apply offset
	if offset >= len(spans) {
		return []*protocol.SpanMessage{}
	}
	spans = spans[offset:]

	// Apply limit
	if limit > 0 && len(spans) > limit {
		spans = spans[:limit]
	}

	return spans
}
