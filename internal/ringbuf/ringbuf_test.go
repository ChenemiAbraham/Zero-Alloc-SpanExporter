package ringbuf

import (
	"sync"
	"testing"
)

func TestRingBufferBasicOps(t *testing.T) {
	rb := NewRingBuffer(8)

	// Test push and pop
	if !rb.TryPush("item1") {
		t.Error("Failed to push item1")
	}

	if rb.Len() != 1 {
		t.Errorf("Expected length 1, got %d", rb.Len())
	}

	item := rb.Pop()
	if item != "item1" {
		t.Errorf("Expected item1, got %v", item)
	}

	if rb.Len() != 0 {
		t.Errorf("Expected length 0, got %d", rb.Len())
	}

	// Test pop on empty buffer
	item = rb.Pop()
	if item != nil {
		t.Errorf("Expected nil from empty buffer, got %v", item)
	}
}

func TestRingBufferFull(t *testing.T) {
	rb := NewRingBuffer(4)

	// Fill buffer
	for i := 0; i < 4; i++ {
		if !rb.TryPush(i) {
			t.Errorf("Failed to push item %d", i)
		}
	}

	// Try to push when full
	if rb.TryPush("overflow") {
		t.Error("Should not be able to push to full buffer")
	}

	if rb.DroppedCount() != 1 {
		t.Errorf("Expected 1 dropped, got %d", rb.DroppedCount())
	}

	// Pop one and push again
	rb.Pop()
	if !rb.TryPush("new") {
		t.Error("Should be able to push after pop")
	}
}

func TestRingBufferConcurrent(t *testing.T) {
	rb := NewRingBuffer(1024)

	var wg sync.WaitGroup
	itemCount := 10000

	// Producer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < itemCount; i++ {
			for !rb.TryPush(i) {
				// Spin until push succeeds
			}
		}
	}()

	// Consumer
	consumed := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		for consumed < itemCount {
			if item := rb.Pop(); item != nil {
				consumed++
			}
		}
	}()

	wg.Wait()

	if consumed != itemCount {
		t.Errorf("Expected %d items consumed, got %d", itemCount, consumed)
	}
}

func BenchmarkRingBufferPush(b *testing.B) {
	rb := NewRingBuffer(8192)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rb.TryPush(i)
	}
}

func BenchmarkRingBufferPop(b *testing.B) {
	rb := NewRingBuffer(8192)

	// Pre-fill
	for i := 0; i < 8192; i++ {
		rb.TryPush(i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rb.Pop()
		rb.TryPush(i) // Keep buffer full
	}
}

func BenchmarkRingBufferConcurrent(b *testing.B) {
	rb := NewRingBuffer(8192)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				rb.TryPush(i)
			} else {
				rb.Pop()
			}
			i++
		}
	})
}
