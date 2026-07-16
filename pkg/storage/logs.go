package storage

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	badger "github.com/dgraph-io/badger/v4"
	"go.opentelemetry.io/otel/trace"
)

// StoreLog saves a log record to persistent storage with secondary indexes
func (s *Store) StoreLog(log *protocol.LogMessage) error {
	// Create primary key: log:<timestamp>:<trace_id>:<span_id>
	key := makeLogKey(log)

	// Serialize log to bytes
	buf := protocol.NewBuffer()
	defer protocol.PutBuffer(buf)

	if err := log.EncodeLogTo(buf); err != nil {
		return fmt.Errorf("failed to encode log: %w", err)
	}

	// Skip the 4-byte length prefix and store only the payload
	data := buf.Bytes()
	if len(data) < 4 {
		return fmt.Errorf("encoded log data too short: %d bytes", len(data))
	}
	payload := data[4:] // Skip length prefix

	// Generate secondary index keys for logs
	idxKeys := logIndexKeys(log)

	// Write log + indexes atomically with TTL
	err := s.db.Update(func(txn *badger.Txn) error {
		return storeWithIndexes(txn, key, payload, idxKeys, s.config.TTL)
	})

	if err != nil {
		return fmt.Errorf("failed to write log: %w", err)
	}

	return nil
}

// GetLogsByTraceID retrieves all log records for a given trace ID
func (s *Store) GetLogsByTraceID(traceID trace.TraceID) ([]*protocol.LogMessage, error) {
	var logs []*protocol.LogMessage

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true

		it := txn.NewIterator(opts)
		defer it.Close()

		// Scan all log keys and filter by trace ID
		prefix := []byte("log:")
		for it.Seek(prefix); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Skip if not a log key
			if len(key) < 4 || string(key[:4]) != "log:" {
				break
			}

			// Extract trace ID from key (skip "log:" + timestamp, get next 16 bytes)
			if len(key) < 28 { // log:(4) + timestamp(8) + trace_id(16)
				continue
			}

			keyTraceID := trace.TraceID{}
			copy(keyTraceID[:], key[12:28])

			if keyTraceID == traceID {
				// Found matching trace ID, decode log
				err := item.Value(func(val []byte) error {
					log, err := protocol.DecodeLog(val)
					if err != nil {
						return err
					}
					logs = append(logs, log)
					return nil
				})
				if err != nil {
					return err
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}

	return logs, nil
}

// GetRecentLogs retrieves the N most recent log records
func (s *Store) GetRecentLogs(limit int) ([]*protocol.LogMessage, error) {
	var logs []*protocol.LogMessage

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Reverse = true // Start from newest

		it := txn.NewIterator(opts)
		defer it.Close()

		count := 0

		for it.Seek([]byte("log:\xff\xff\xff\xff\xff\xff\xff\xff")); it.Valid() && count < limit; it.Next() {
			item := it.Item()
			key := item.Key()

			// Skip if not a log key
			if len(key) < 4 || string(key[:4]) != "log:" {
				continue
			}

			// Skip index entries
			if len(key) >= 8 && string(key[:8]) == "idx:log:" {
				continue
			}

			// Ensure it's a primary log key
			if !isLogPrimaryKey(key) {
				continue
			}

			err := item.Value(func(val []byte) error {
				log, err := protocol.DecodeLog(val)
				if err != nil {
					return err
				}
				logs = append(logs, log)
				return nil
			})
			if err != nil {
				return err
			}

			count++
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query recent logs: %w", err)
	}

	return logs, nil
}

// makeLogKey creates a key from log data
// Format: log:[8 byte timestamp][16 byte trace_id][8 byte span_id]
func makeLogKey(log *protocol.LogMessage) []byte {
	key := make([]byte, 4+8+16+8) // "log:" + timestamp + trace_id + span_id

	// Prefix
	copy(key[0:4], []byte("log:"))

	// Timestamp (nanoseconds)
	binary.BigEndian.PutUint64(key[4:12], uint64(log.Timestamp.UnixNano()))

	// Trace ID
	copy(key[12:28], log.TraceID[:])

	// Span ID
	copy(key[28:36], log.SpanID[:])

	return key
}

// logIndexKeys generates all index keys for a log record
func logIndexKeys(log *protocol.LogMessage) [][]byte {
	var keys [][]byte

	timestamp := uint64(log.Timestamp.UnixNano())

	// Severity index: idx:log:sev:<severity>:<timestamp>:<trace_id>:<span_id>
	keys = append(keys, makeLogSeverityKey(log.SeverityNumber, timestamp, log.TraceID, log.SpanID))

	// Trace ID index: idx:log:trace:<trace_id>:<timestamp>:<span_id>
	keys = append(keys, makeLogTraceKey(log.TraceID, timestamp, log.SpanID))

	// TODO: Text search index (for log body text search)
	// This would require tokenization and could be added later
	// For now, full-text search will use post-filtering

	return keys
}

// makeLogSeverityKey creates a severity index key
func makeLogSeverityKey(severity int32, timestamp uint64, traceID trace.TraceID, spanID trace.SpanID) []byte {
	// Format: idx:log:sev:<severity>:<timestamp>:<trace_id>:<span_id>
	prefix := "idx:log:sev:"
	sevStr := fmt.Sprintf("%02d", severity) // Zero-padded for sorting

	key := make([]byte, 0, len(prefix)+len(sevStr)+1+8+16+8)
	key = append(key, []byte(prefix)...)
	key = append(key, []byte(sevStr)...)
	key = append(key, ':')

	// Timestamp (8 bytes, big-endian)
	timeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBuf, timestamp)
	key = append(key, timeBuf...)

	// Trace ID (16 bytes)
	key = append(key, traceID[:]...)

	// Span ID (8 bytes)
	key = append(key, spanID[:]...)

	return key
}

// makeLogTraceKey creates a trace ID index key
func makeLogTraceKey(traceID trace.TraceID, timestamp uint64, spanID trace.SpanID) []byte {
	// Format: idx:log:trace:<trace_id>:<timestamp>:<span_id>
	prefix := "idx:log:trace:"

	key := make([]byte, 0, len(prefix)+16+8+8)
	key = append(key, []byte(prefix)...)

	// Trace ID (16 bytes)
	key = append(key, traceID[:]...)

	// Timestamp (8 bytes)
	timeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(timeBuf, timestamp)
	key = append(key, timeBuf...)

	// Span ID (8 bytes)
	key = append(key, spanID[:]...)

	return key
}

// isLogPrimaryKey checks if a key is a primary log key (not an index)
func isLogPrimaryKey(key []byte) bool {
	// Primary log keys: log:<timestamp>:<trace_id>:<span_id>
	// Index keys: idx:log:...
	if len(key) < 4 {
		return false
	}

	// Check if it starts with "log:" and NOT "idx:"
	return string(key[:4]) == "log:" && (len(key) < 4 || string(key[:4]) != "idx:")
}

// GetLogsByTimeRange retrieves logs within a time range
func (s *Store) GetLogsByTimeRange(start, end time.Time) ([]*protocol.LogMessage, error) {
	var logs []*protocol.LogMessage

	startKey := makeLogTimeKey(start)
	endKey := makeLogTimeKey(end)

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true

		it := txn.NewIterator(opts)
		defer it.Close()

		// Seek to start time
		for it.Seek(startKey); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Skip if not a log key
			if len(key) < 4 || string(key[:4]) != "log:" {
				break
			}

			// Skip index entries
			if len(key) >= 8 && string(key[:8]) == "idx:log:" {
				continue
			}

			// Stop if we've passed the end time
			if compareKeys(key, endKey) > 0 {
				break
			}

			err := item.Value(func(val []byte) error {
				log, err := protocol.DecodeLog(val)
				if err != nil {
					return err
				}
				logs = append(logs, log)
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query log time range: %w", err)
	}

	return logs, nil
}

// makeLogTimeKey creates a key prefix for time-based log queries
func makeLogTimeKey(t time.Time) []byte {
	key := make([]byte, 12) // "log:" + 8 bytes timestamp
	copy(key[0:4], []byte("log:"))
	binary.BigEndian.PutUint64(key[4:12], uint64(t.UnixNano()))
	return key
}
