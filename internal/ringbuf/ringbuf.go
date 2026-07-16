package ringbuf

import (
	"sync/atomic"
)

// RingBuffer is a lock-free, bounded SPSC (Single Producer, Single Consumer) ring buffer
// This provides backpressure handling without locks or channels
type RingBuffer struct {
	buffer []interface{}
	mask   uint64
	// Using separate cache lines to avoid false sharing
	_       [7]uint64 // padding
	head    uint64    // write position (producer only)
	_       [7]uint64 // padding
	tail    uint64    // read position (consumer only)
	_       [7]uint64 // padding
	dropped uint64    // counter for dropped items
}

// NewRingBuffer creates a new ring buffer with size as a power of 2
func NewRingBuffer(size uint64) *RingBuffer {
	// Ensure size is power of 2 for efficient masking
	if size&(size-1) != 0 {
		panic("ring buffer size must be power of 2")
	}

	return &RingBuffer{
		buffer: make([]interface{}, size),
		mask:   size - 1,
	}
}

// TryPush attempts to push an item into the ring buffer
// Returns false if the buffer is full (non-blocking)
func (rb *RingBuffer) TryPush(item interface{}) bool {
	head := atomic.LoadUint64(&rb.head)
	tail := atomic.LoadUint64(&rb.tail)

	// Check if buffer is full
	if head-tail >= uint64(len(rb.buffer)) {
		atomic.AddUint64(&rb.dropped, 1)
		return false
	}

	// Write to buffer
	rb.buffer[head&rb.mask] = item

	// Publish write by advancing head
	atomic.StoreUint64(&rb.head, head+1)
	return true
}

// Pop removes and returns an item from the ring buffer
// Returns nil if buffer is empty
func (rb *RingBuffer) Pop() interface{} {
	tail := atomic.LoadUint64(&rb.tail)
	head := atomic.LoadUint64(&rb.head)

	// Check if buffer is empty
	if tail >= head {
		return nil
	}

	// Read from buffer
	item := rb.buffer[tail&rb.mask]

	// Advance tail
	atomic.StoreUint64(&rb.tail, tail+1)
	return item
}

// Len returns the current number of items in the buffer
func (rb *RingBuffer) Len() int {
	head := atomic.LoadUint64(&rb.head)
	tail := atomic.LoadUint64(&rb.tail)
	return int(head - tail)
}

// DroppedCount returns the number of items dropped due to buffer full
func (rb *RingBuffer) DroppedCount() uint64 {
	return atomic.LoadUint64(&rb.dropped)
}

// IsFull returns true if the buffer cannot accept more items
func (rb *RingBuffer) IsFull() bool {
	head := atomic.LoadUint64(&rb.head)
	tail := atomic.LoadUint64(&rb.tail)
	return head-tail >= uint64(len(rb.buffer))
}

// IsEmpty returns true if the buffer has no items
func (rb *RingBuffer) IsEmpty() bool {
	return rb.Len() == 0
}
