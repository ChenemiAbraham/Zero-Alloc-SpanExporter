package storage

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	badger "github.com/dgraph-io/badger/v4"
)

// Config holds storage configuration
type Config struct {
	// Path to the database directory
	Path string

	// TTL for spans (default: 24 hours)
	// Spans older than this will be garbage collected
	TTL time.Duration

	// SyncWrites enables synchronous writes (slower but safer)
	// Default: false (async writes)
	SyncWrites bool

	// CompactInterval is how often to run compaction
	// Default: 1 hour
	CompactInterval time.Duration

	// MaxTableSize is the maximum size of each table file
	// Default: 64MB
	MaxTableSize int64
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Path:            "./ltt-data",
		TTL:             24 * time.Hour,
		SyncWrites:      false,
		CompactInterval: 1 * time.Hour,
		MaxTableSize:    64 << 20, // 64MB
	}
}

// Store provides persistent storage for spans
type Store struct {
	db     *badger.DB
	config Config
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewStore creates a new persistent store
func NewStore(config Config) (*Store, error) {
	// Apply defaults
	if config.Path == "" {
		config = DefaultConfig()
	}
	if config.TTL == 0 {
		config.TTL = 24 * time.Hour
	}
	if config.CompactInterval == 0 {
		config.CompactInterval = 1 * time.Hour
	}
	if config.MaxTableSize == 0 {
		config.MaxTableSize = 64 << 20
	}

	// Configure BadgerDB
	opts := badger.DefaultOptions(config.Path)
	opts.SyncWrites = config.SyncWrites
	opts.NumVersionsToKeep = 1 // Keep only latest version
	// Note: MaxTableSize is managed internally in BadgerDB v4

	// Open database
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger: %w", err)
	}

	s := &Store{
		db:     db,
		config: config,
		stopCh: make(chan struct{}),
	}

	// Start background compaction
	s.wg.Add(1)
	go s.compactionWorker()

	return s, nil
}

// Store saves a span to persistent storage
func (s *Store) Store(span *protocol.SpanMessage) error {
	// Create key: timestamp + trace_id + span_id for time-based ordering
	key := makeKey(span)

	// Serialize span to bytes (EncodeTo includes 4-byte length prefix)
	buf := protocol.NewBuffer()
	defer protocol.PutBuffer(buf)

	if err := span.EncodeTo(buf); err != nil {
		return fmt.Errorf("failed to encode span: %w", err)
	}

	// Skip the 4-byte length prefix and store only the payload
	data := buf.Bytes()
	if len(data) < 4 {
		return fmt.Errorf("encoded data too short: %d bytes", len(data))
	}
	payload := data[4:] // Skip length prefix

	// Write to BadgerDB with TTL
	err := s.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry(key, payload).WithTTL(s.config.TTL)
		return txn.SetEntry(entry)
	})

	if err != nil {
		return fmt.Errorf("failed to write span: %w", err)
	}

	return nil
}

// GetByTraceID retrieves all spans for a given trace ID
func (s *Store) GetByTraceID(traceID [16]byte) ([]*protocol.SpanMessage, error) {
	var spans []*protocol.SpanMessage

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true

		it := txn.NewIterator(opts)
		defer it.Close()

		// Scan all keys and filter by trace ID
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Extract trace ID from key (skip timestamp, get next 16 bytes)
			if len(key) < 24 {
				continue
			}

			keyTraceID := [16]byte{}
			copy(keyTraceID[:], key[8:24])

			if keyTraceID == traceID {
				// Found matching trace ID, decode span
				err := item.Value(func(val []byte) error {
					span, err := protocol.DecodePayload(val)
					if err != nil {
						return err
					}
					spans = append(spans, span)
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
		return nil, fmt.Errorf("failed to query spans: %w", err)
	}

	return spans, nil
}

// GetRecent retrieves the N most recent spans
func (s *Store) GetRecent(limit int) ([]*protocol.SpanMessage, error) {
	var spans []*protocol.SpanMessage

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Reverse = true // Start from newest

		it := txn.NewIterator(opts)
		defer it.Close()

		count := 0
		for it.Rewind(); it.Valid() && count < limit; it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				span, err := protocol.DecodePayload(val)
				if err != nil {
					return err
				}
				spans = append(spans, span)
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
		return nil, fmt.Errorf("failed to query recent spans: %w", err)
	}

	return spans, nil
}

// GetByTimeRange retrieves spans within a time range
func (s *Store) GetByTimeRange(start, end time.Time) ([]*protocol.SpanMessage, error) {
	var spans []*protocol.SpanMessage

	startKey := makeTimeKey(start)
	endKey := makeTimeKey(end)

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true

		it := txn.NewIterator(opts)
		defer it.Close()

		// Seek to start time
		for it.Seek(startKey); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Stop if we've passed the end time
			if compareKeys(key, endKey) > 0 {
				break
			}

			err := item.Value(func(val []byte) error {
				span, err := protocol.DecodePayload(val)
				if err != nil {
					return err
				}
				spans = append(spans, span)
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query time range: %w", err)
	}

	return spans, nil
}

// Count returns the approximate number of spans stored
func (s *Store) Count() (int, error) {
	count := 0

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // Don't load values, just count keys

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to count spans: %w", err)
	}

	return count, nil
}

// Close closes the store and releases resources
func (s *Store) Close() error {
	close(s.stopCh)
	s.wg.Wait()

	return s.db.Close()
}

// compactionWorker runs periodic compaction in the background
func (s *Store) compactionWorker() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CompactInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return

		case <-ticker.C:
			// Run value log garbage collection
			// This reclaims space from deleted/expired entries
			err := s.db.RunValueLogGC(0.5) // Reclaim if 50% or more is garbage
			if err != nil && err != badger.ErrNoRewrite {
				// ErrNoRewrite means no GC was needed, which is fine
				fmt.Printf("GC error: %v\n", err)
			}
		}
	}
}

// makeKey creates a key from span data
// Format: [8 byte timestamp][16 byte trace_id][8 byte span_id]
// This gives us time-ordered keys with trace locality
func makeKey(span *protocol.SpanMessage) []byte {
	key := make([]byte, 32)

	// Timestamp (nanoseconds)
	binary.BigEndian.PutUint64(key[0:8], uint64(span.StartTime.UnixNano()))

	// Trace ID
	copy(key[8:24], span.TraceID[:])

	// Span ID
	copy(key[24:32], span.SpanID[:])

	return key
}

// makeTimeKey creates a key prefix for time-based queries
func makeTimeKey(t time.Time) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, uint64(t.UnixNano()))
	return key
}

// compareKeys compares two keys lexicographically
func compareKeys(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}

	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}

	return 0
}
