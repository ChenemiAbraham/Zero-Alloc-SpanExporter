package exporter

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/internal/ringbuf"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/metrics"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
	"go.opentelemetry.io/otel/sdk/trace"
)

// Config holds exporter configuration
type Config struct {
	// SocketPath is the path to the Unix domain socket or named pipe
	SocketPath string

	// BufferSize is the size of the ring buffer (must be power of 2)
	// Default: 8192
	BufferSize uint64

	// InitialBufferCapacity is the initial capacity for pooled byte buffers
	// Default: 4096 bytes
	InitialBufferCapacity int

	// MaxBatchSize is the maximum number of spans to batch before writing
	// Default: 100
	MaxBatchSize int

	// FlushInterval is how often to flush batched spans
	// Default: 100ms
	FlushInterval time.Duration

	// Storage configuration (optional)
	// If nil, persistence is disabled
	Storage *storage.Config
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() Config {
	socketPath := "/tmp/ltt.sock"
	if runtime.GOOS == "windows" {
		socketPath = "127.0.0.1:9090"
	}

	return Config{
		SocketPath:            socketPath,
		BufferSize:            8192,
		InitialBufferCapacity: 4096,
		MaxBatchSize:          100,
		FlushInterval:         100 * time.Millisecond,
	}
}

// Exporter is a zero-allocation OpenTelemetry span exporter
type Exporter struct {
	config    Config
	transport *SocketTransport
	bufPool   *BufferPool
	ringBuf   *ringbuf.RingBuffer
	store     *storage.Store // Optional persistent storage

	// Worker goroutine coordination
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	exportedSpans uint64
	droppedSpans  uint64
	failedWrites  uint64
	metrics       *metrics.Metrics

	// Ensure single shutdown
	shutdownOnce sync.Once
}

// New creates a new LTT exporter
func New(config Config) (*Exporter, error) {
	// Apply defaults
	if config.SocketPath == "" {
		config = DefaultConfig()
	}
	if config.BufferSize == 0 {
		config.BufferSize = 8192
	}
	if config.InitialBufferCapacity == 0 {
		config.InitialBufferCapacity = 4096
	}
	if config.MaxBatchSize == 0 {
		config.MaxBatchSize = 100
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 100 * time.Millisecond
	}

	// Create socket transport
	transport, err := NewSocketTransport(config.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	exp := &Exporter{
		config:    config,
		transport: transport,
		bufPool:   NewBufferPool(config.InitialBufferCapacity),
		ringBuf:   ringbuf.NewRingBuffer(config.BufferSize),
		ctx:       ctx,
		cancel:    cancel,
		metrics:   metrics.GetGlobal(),
	}

	// Initialize persistent storage if configured
	if config.Storage != nil {
		store, err := storage.NewStore(*config.Storage)
		if err != nil {
			transport.Close()
			return nil, fmt.Errorf("failed to create storage: %w", err)
		}
		exp.store = store
	}

	// Start background worker
	exp.wg.Add(1)
	go exp.worker()

	// Start metrics collector goroutine
	exp.wg.Add(1)
	go exp.metricsCollector()

	return exp, nil
}

// ExportSpans implements the trace.SpanExporter interface
// This is the HOT PATH - must be zero-allocation and non-blocking
func (e *Exporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	count := len(spans)
	e.metrics.RecordSpanReceived(count)

	for _, span := range spans {
		// Convert to wire format (this allocates a SpanMessage)
		// TODO: Pool SpanMessage objects as well
		msg := protocol.FromReadOnlySpan(span)

		// Try to push to ring buffer (non-blocking)
		if !e.ringBuf.TryPush(msg) {
			// Buffer full, increment dropped counter
			atomic.AddUint64(&e.droppedSpans, 1)
			e.metrics.RecordSpanDropped(1)
			continue
		}

		atomic.AddUint64(&e.exportedSpans, 1)
	}

	return nil
}

// worker is the background goroutine that reads from ring buffer and writes to socket
func (e *Exporter) worker() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.FlushInterval)
	defer ticker.Stop()

	batch := make([]*protocol.SpanMessage, 0, e.config.MaxBatchSize)

	for {
		select {
		case <-e.ctx.Done():
			// Flush remaining spans before exit
			e.flushBatch(batch)
			return

		case <-ticker.C:
			// Periodic flush
			if len(batch) > 0 {
				e.flushBatch(batch)
				batch = batch[:0]
			}

		default:
			// Try to pop from ring buffer
			item := e.ringBuf.Pop()
			if item == nil {
				// Buffer empty, small sleep to avoid spinning
				time.Sleep(time.Millisecond)
				continue
			}

			msg := item.(*protocol.SpanMessage)
			batch = append(batch, msg)

			// Flush if batch is full
			if len(batch) >= e.config.MaxBatchSize {
				e.flushBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

// flushBatch writes a batch of spans to the socket and optionally to persistent storage
func (e *Exporter) flushBatch(batch []*protocol.SpanMessage) {
	if len(batch) == 0 {
		return
	}

	start := time.Now()

	// Write to persistent storage first (write-through cache)
	if e.store != nil {
		for _, msg := range batch {
			if err := e.store.Store(msg); err != nil {
				// Log error but continue - don't fail export if storage fails
				atomic.AddUint64(&e.failedWrites, 1)
			}
		}
	}

	// Get buffer from pool
	buf := e.bufPool.Get()
	defer e.bufPool.Put(buf)

	// Serialize batch
	encodeStart := time.Now()
	encodedCount := 0
	for _, msg := range batch {
		if err := msg.EncodeTo(buf); err != nil {
			// Log error but continue
			atomic.AddUint64(&e.failedWrites, 1)
			e.metrics.RecordSpanFailed(1)
			continue
		}
		encodedCount++
	}
	e.metrics.RecordEncodeDuration(time.Since(encodeStart))

	// Write to socket
	if err := e.transport.Write(buf.Bytes()); err != nil {
		atomic.AddUint64(&e.failedWrites, 1)
		e.metrics.RecordSpanFailed(encodedCount)
	} else {
		e.metrics.RecordSpanExported(encodedCount)
	}

	e.metrics.RecordExportDuration(time.Since(start))
}

// Shutdown gracefully shuts down the exporter
func (e *Exporter) Shutdown(ctx context.Context) error {
	var err error
	e.shutdownOnce.Do(func() {
		// Cancel worker
		e.cancel()

		// Wait for worker to finish with timeout
		done := make(chan struct{})
		go func() {
			e.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-ctx.Done():
			err = ctx.Err()
		}

		// Close transport
		if e.transport != nil {
			e.transport.Close()
		}

		// Close persistent storage
		if e.store != nil {
			if storeErr := e.store.Close(); storeErr != nil && err == nil {
				err = storeErr
			}
		}
	})

	return err
}

// ForceFlush is a no-op for this exporter since we flush periodically
func (e *Exporter) ForceFlush(ctx context.Context) error {
	return nil
}

// Stats returns current exporter statistics
type Stats struct {
	ExportedSpans uint64
	DroppedSpans  uint64
	FailedWrites  uint64
	BufferUsage   float64
}

// GetStats returns current statistics
func (e *Exporter) GetStats() Stats {
	bufSize := int(e.config.BufferSize)
	used := e.ringBuf.Len()

	return Stats{
		ExportedSpans: atomic.LoadUint64(&e.exportedSpans),
		DroppedSpans:  atomic.LoadUint64(&e.droppedSpans) + e.ringBuf.DroppedCount(),
		FailedWrites:  atomic.LoadUint64(&e.failedWrites),
		BufferUsage:   float64(used) / float64(bufSize) * 100,
	}
}

// metricsCollector periodically updates resource metrics
func (e *Exporter) metricsCollector() {
	defer e.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return

		case <-ticker.C:
			// Update buffer usage
			bufSize := int(e.config.BufferSize)
			used := e.ringBuf.Len()
			e.metrics.SetBufferUsage(used, bufSize)

			// Update goroutine count
			e.metrics.SetGoroutines(runtime.NumGoroutine())

			// Update memory usage
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			e.metrics.SetMemoryUsage(m.Alloc)
		}
	}
}

// IsConnected returns whether the socket is connected
func (e *Exporter) IsConnected() bool {
	return e.transport != nil && e.transport.IsConnected()
}

// GetBufferUsage returns current buffer usage as a percentage
func (e *Exporter) GetBufferUsage() float64 {
	bufSize := int(e.config.BufferSize)
	used := e.ringBuf.Len()
	return float64(used) / float64(bufSize)
}

// GetStore returns the persistent storage (may be nil)
func (e *Exporter) GetStore() *storage.Store {
	return e.store
}
