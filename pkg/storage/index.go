package storage

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	badger "github.com/dgraph-io/badger/v4"
)

// Index key formats:
// idx:op:<operation>:<timestamp>:<trace_id>:<span_id> → empty
// idx:status:<code>:<timestamp>:<trace_id>:<span_id> → empty
// idx:attr:<key>:<value>:<timestamp>:<trace_id>:<span_id> → empty
// idx:duration:<bucket>:<timestamp>:<trace_id>:<span_id> → empty

const (
	indexPrefixOperation = "idx:op:"
	indexPrefixStatus    = "idx:status:"
	indexPrefixAttr      = "idx:attr:"
	indexPrefixDuration  = "idx:dur:"
)

// indexKeys generates all index keys for a span
func indexKeys(span *protocol.SpanMessage) [][]byte {
	var keys [][]byte

	timestamp := uint64(span.StartTime.UnixNano())

	// Operation index
	if span.Name != "" {
		keys = append(keys, makeOperationKey(span.Name, timestamp, span.TraceID, span.SpanID))
	}

	// Status index
	keys = append(keys, makeStatusKey(int32(span.StatusCode), timestamp, span.TraceID, span.SpanID))

	// Duration index (bucket by magnitude for range queries)
	duration := span.EndTime.Sub(span.StartTime)
	bucket := getDurationBucket(duration)
	keys = append(keys, makeDurationKey(bucket, timestamp, span.TraceID, span.SpanID))

	// Attribute indexes (limit to avoid index explosion)
	count := 0
	for key, value := range span.Attributes {
		if count >= 10 { // Max 10 indexed attributes per span
			break
		}
		if valStr, ok := value.(string); ok {
			keys = append(keys, makeAttributeKey(key, valStr, timestamp, span.TraceID, span.SpanID))
			count++
		}
	}

	return keys
}

// makeOperationKey creates an operation index key
func makeOperationKey(operation string, timestamp uint64, traceID [16]byte, spanID [8]byte) []byte {
	// Format: idx:op:<operation>:<timestamp>:<trace_id>:<span_id>
	key := make([]byte, 0, len(indexPrefixOperation)+len(operation)+1+8+16+8)
	key = append(key, []byte(indexPrefixOperation)...)
	key = append(key, []byte(operation)...)
	key = append(key, ':')

	// Timestamp (8 bytes, big-endian for proper sorting)
	timeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBuf, timestamp)
	key = append(key, timeBuf...)

	// Trace ID (16 bytes)
	key = append(key, traceID[:]...)

	// Span ID (8 bytes)
	key = append(key, spanID[:]...)

	return key
}

// makeStatusKey creates a status index key
func makeStatusKey(status int32, timestamp uint64, traceID [16]byte, spanID [8]byte) []byte {
	statusStr := fmt.Sprintf("%d", status)
	key := make([]byte, 0, len(indexPrefixStatus)+len(statusStr)+1+8+16+8)
	key = append(key, []byte(indexPrefixStatus)...)
	key = append(key, []byte(statusStr)...)
	key = append(key, ':')

	timeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBuf, timestamp)
	key = append(key, timeBuf...)
	key = append(key, traceID[:]...)
	key = append(key, spanID[:]...)

	return key
}

// makeDurationKey creates a duration bucket index key
func makeDurationKey(bucket int, timestamp uint64, traceID [16]byte, spanID [8]byte) []byte {
	bucketStr := fmt.Sprintf("%02d", bucket)
	key := make([]byte, 0, len(indexPrefixDuration)+len(bucketStr)+1+8+16+8)
	key = append(key, []byte(indexPrefixDuration)...)
	key = append(key, []byte(bucketStr)...)
	key = append(key, ':')

	timeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBuf, timestamp)
	key = append(key, timeBuf...)
	key = append(key, traceID[:]...)
	key = append(key, spanID[:]...)

	return key
}

// makeAttributeKey creates an attribute index key
func makeAttributeKey(attrKey, attrValue string, timestamp uint64, traceID [16]byte, spanID [8]byte) []byte {
	key := make([]byte, 0, len(indexPrefixAttr)+len(attrKey)+1+len(attrValue)+1+8+16+8)
	key = append(key, []byte(indexPrefixAttr)...)
	key = append(key, []byte(attrKey)...)
	key = append(key, ':')
	key = append(key, []byte(attrValue)...)
	key = append(key, ':')

	timeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBuf, timestamp)
	key = append(key, timeBuf...)
	key = append(key, traceID[:]...)
	key = append(key, spanID[:]...)

	return key
}

// getDurationBucket returns a bucket number for duration-based indexing
// Buckets: 0-9ms, 10-99ms, 100-999ms, 1-9s, 10-99s, 100s+
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
	// We need to extract these to build the primary key
	if len(indexKey) < 32 {
		return nil
	}

	suffix := indexKey[len(indexKey)-32:]

	// Primary key format: <timestamp:8><trace_id:16><span_id:8>
	return suffix
}

// storeWithIndexes stores a span and all its indexes atomically
func storeWithIndexes(txn *badger.Txn, primaryKey []byte, spanData []byte, indexKeys [][]byte, ttl time.Duration) error {
	// Store primary data
	entry := badger.NewEntry(primaryKey, spanData).WithTTL(ttl)
	if err := txn.SetEntry(entry); err != nil {
		return fmt.Errorf("failed to store span: %w", err)
	}

	// Store index entries (empty values, key is the index)
	for _, indexKey := range indexKeys {
		entry := badger.NewEntry(indexKey, []byte{}).WithTTL(ttl)
		if err := txn.SetEntry(entry); err != nil {
			return fmt.Errorf("failed to store index: %w", err)
		}
	}

	return nil
}
