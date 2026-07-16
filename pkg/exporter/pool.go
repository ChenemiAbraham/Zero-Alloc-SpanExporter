package exporter

import (
	"bytes"
	"sync"
)

// BufferPool manages a pool of reusable byte buffers for zero-allocation serialization
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new buffer pool with the given initial capacity
func NewBufferPool(initialCapacity int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				// Pre-allocate buffer with reasonable capacity
				// to avoid resizing in hot path
				return bytes.NewBuffer(make([]byte, 0, initialCapacity))
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (p *BufferPool) Get() *bytes.Buffer {
	return p.pool.Get().(*bytes.Buffer)
}

// Put returns a buffer to the pool after resetting it
func (p *BufferPool) Put(buf *bytes.Buffer) {
	// Reset buffer to clear contents but retain capacity
	buf.Reset()
	p.pool.Put(buf)
}

// Stats provides pool statistics (useful for monitoring)
type PoolStats struct {
	BuffersCreated uint64
	BuffersReused  uint64
	CurrentSize    int
}

// GetStats returns current pool statistics
func (p *BufferPool) GetStats() PoolStats {
	// TODO: Implement metrics collection
	// This would require atomic counters in the pool
	return PoolStats{}
}
