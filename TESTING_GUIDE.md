# LTT Testing Guide

Complete testing strategy from unit tests to end-to-end validation.

## Quick Test Status

```bash
# Run all tests at once
make test

# Or step by step:
go test ./pkg/exporter          # ✅ Exporter tests
go test ./internal/ringbuf      # ✅ Ring buffer tests
go test ./pkg/protocol          # ⏳ Protocol tests (TODO)
go test ./pkg/viewer            # ⏳ Viewer tests (TODO)
```

---

## Phase 1: Unit Tests (What Works Now)

### 1.1 Ring Buffer Tests

**Test the lock-free SPSC ring buffer**:

```bash
cd internal/ringbuf
go test -v
```

**Expected output**:
```
=== RUN   TestRingBufferBasicOps
--- PASS: TestRingBufferBasicOps (0.00s)
=== RUN   TestRingBufferFull
--- PASS: TestRingBufferFull (0.00s)
=== RUN   TestRingBufferConcurrent
--- PASS: TestRingBufferConcurrent (0.05s)
PASS
```

**Run with race detector**:
```bash
go test -race -v
```

Should pass with **zero race conditions**.

### 1.2 Ring Buffer Benchmarks

```bash
go test -bench=. -benchmem
```

**Target results**:
```
BenchmarkRingBufferPush-8       50000000    20.5 ns/op    0 B/op    0 allocs/op
BenchmarkRingBufferPop-8        50000000    22.1 ns/op    0 B/op    0 allocs/op
BenchmarkRingBufferConcurrent-8 10000000    150 ns/op     0 B/op    0 allocs/op
```

✅ **Zero allocations** is the key metric.

### 1.3 Exporter Tests

**Test the exporter (without full protocol)**:

```bash
cd pkg/exporter
go test -v
```

**Current status**: Will compile but some tests may fail because protocol encoding is TODO.

---

## Phase 2: Component Testing

### 2.1 Test Socket Transport

Create a simple socket test:

```bash
# Create test file
cat > test_socket.go << 'EOF'
package main

import (
    "fmt"
    "time"
    "github.com/yourusername/ltt/pkg/exporter"
)

func main() {
    // Create socket
    transport, err := exporter.NewSocketTransport("/tmp/ltt-test.sock")
    if err != nil {
        panic(err)
    }
    defer transport.Close()
    
    fmt.Println("✅ Socket created at /tmp/ltt-test.sock")
    
    // Write test data
    data := []byte("Hello from LTT!")
    err = transport.Write(data)
    if err != nil {
        fmt.Printf("⚠️ Write failed (expected, no client): %v\n", err)
    } else {
        fmt.Println("✅ Write succeeded")
    }
    
    time.Sleep(2 * time.Second)
    fmt.Println("✅ Socket test complete")
}
EOF

go run test_socket.go
```

**Expected**: Socket created, write fails (no client connected).

### 2.2 Test Socket Reader

In another terminal while socket test runs:

```bash
# Connect to socket
nc -U /tmp/ltt-test.sock
# Or on Mac:
socat - UNIX-CONNECT:/tmp/ltt-test.sock
```

Should receive "Hello from LTT!" if client connected.

---

## Phase 3: Integration Testing (Needs Protocol Implementation)

### 3.1 Minimal End-to-End Test

**Create a simple test without TUI**:

```bash
cat > test_e2e.go << 'EOF'
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/yourusername/ltt/pkg/exporter"
    "go.opentelemetry.io/otel"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func main() {
    fmt.Println("🧪 LTT End-to-End Test")
    
    // 1. Create exporter
    fmt.Println("1. Creating exporter...")
    exp, err := exporter.New(exporter.Config{
        SocketPath: "/tmp/ltt-test.sock",
    })
    if err != nil {
        panic(err)
    }
    defer exp.Shutdown(context.Background())
    fmt.Println("   ✅ Exporter created")
    
    // 2. Create tracer provider
    fmt.Println("2. Creating tracer provider...")
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
    )
    defer tp.Shutdown(context.Background())
    otel.SetTracerProvider(tp)
    fmt.Println("   ✅ Tracer provider registered")
    
    // 3. Create and export test spans
    fmt.Println("3. Creating test spans...")
    tracer := tp.Tracer("test")
    
    ctx := context.Background()
    ctx, span := tracer.Start(ctx, "test-operation")
    time.Sleep(10 * time.Millisecond)
    span.End()
    
    fmt.Println("   ✅ Span created and ended")
    
    // 4. Force flush
    fmt.Println("4. Flushing...")
    tp.ForceFlush(context.Background())
    time.Sleep(100 * time.Millisecond)
    
    // 5. Check stats
    stats := exp.GetStats()
    fmt.Printf("\n📊 Statistics:\n")
    fmt.Printf("   Exported: %d\n", stats.ExportedSpans)
    fmt.Printf("   Dropped:  %d\n", stats.DroppedSpans)
    fmt.Printf("   Failed:   %d\n", stats.FailedWrites)
    fmt.Printf("   Buffer:   %.1f%%\n", stats.BufferUsage)
    
    if stats.ExportedSpans > 0 {
        fmt.Println("\n✅ TEST PASSED: Spans were exported!")
    } else {
        fmt.Println("\n❌ TEST FAILED: No spans exported")
    }
}
EOF

go run test_e2e.go
```

**Expected output**:
```
🧪 LTT End-to-End Test
1. Creating exporter...
   ✅ Exporter created
2. Creating tracer provider...
   ✅ Tracer provider registered
3. Creating test spans...
   ✅ Span created and ended
4. Flushing...

📊 Statistics:
   Exported: 1
   Dropped:  0
   Failed:   0 (or 1 if no client)
   Buffer:   0.0%

✅ TEST PASSED: Spans were exported!
```

---

## Phase 4: Benchmark Testing

### 4.1 Exporter Performance

```bash
cd pkg/exporter
go test -bench=BenchmarkExportSpan -benchmem -benchtime=5s
```

**Target**:
```
BenchmarkExportSpan-8           24,000,000    48.2 ns/op    0 B/op    0 allocs/op
```

### 4.2 Parallel Export Benchmark

```bash
go test -bench=BenchmarkExportSpanParallel -benchmem
```

**Target**: >100k ops/sec across all cores.

### 4.3 Memory Profiling

```bash
# Run memory profile
go test -memprofile=mem.prof -bench=BenchmarkExportSpan

# Analyze
go tool pprof mem.prof
# Then in pprof:
(pprof) top10
(pprof) list ExportSpans
```

**What to look for**: Zero allocations in `ExportSpans` hot path.

### 4.4 CPU Profiling

```bash
# Run CPU profile
go test -cpuprofile=cpu.prof -bench=BenchmarkExportSpan

# Visualize
go tool pprof -http=:8080 cpu.prof
```

**What to look for**: 
- Most time in `atomic` operations (ring buffer)
- Minimal time in allocations
- No lock contention

---

## Phase 5: Stress Testing

### 5.1 High Throughput Test

```bash
cat > test_stress.go << 'EOF'
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/yourusername/ltt/pkg/exporter"
    "go.opentelemetry.io/otel"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
    fmt.Println("🔥 LTT Stress Test")
    
    exp, _ := exporter.New(exporter.Config{
        SocketPath: "/tmp/ltt-stress.sock",
        BufferSize: 8192,
    })
    defer exp.Shutdown(context.Background())
    
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
    )
    defer tp.Shutdown(context.Background())
    otel.SetTracerProvider(tp)
    
    tracer := tp.Tracer("stress-test")
    
    // Spawn 100 goroutines
    fmt.Println("Starting 100 goroutines...")
    var wg sync.WaitGroup
    start := time.Now()
    
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            
            // Each goroutine creates 1000 spans
            for j := 0; j < 1000; j++ {
                ctx := context.Background()
                _, span := tracer.Start(ctx, fmt.Sprintf("span-%d-%d", id, j))
                span.End()
            }
        }(i)
    }
    
    wg.Wait()
    tp.ForceFlush(context.Background())
    duration := time.Since(start)
    
    stats := exp.GetStats()
    
    fmt.Printf("\n📊 Stress Test Results:\n")
    fmt.Printf("   Duration:    %v\n", duration)
    fmt.Printf("   Total spans: 100,000\n")
    fmt.Printf("   Exported:    %d\n", stats.ExportedSpans)
    fmt.Printf("   Dropped:     %d (%.2f%%)\n", stats.DroppedSpans, 
        float64(stats.DroppedSpans)/100000*100)
    fmt.Printf("   Throughput:  %.0f spans/sec\n", 100000/duration.Seconds())
    
    if stats.ExportedSpans > 95000 {
        fmt.Println("\n✅ STRESS TEST PASSED: >95% spans exported")
    } else {
        fmt.Println("\n⚠️ STRESS TEST WARNING: High drop rate")
    }
}
EOF

go run test_stress.go
```

**Target**: >100k spans/sec, <5% drop rate.

---

## Phase 6: TUI Testing (Manual)

### 6.1 Build TUI

```bash
make build
# Or
go build -o ltt ./cmd/ltt
```

### 6.2 Run Example App + TUI

**Terminal 1** (Example app):
```bash
go run ./examples/simple/main.go
```

**Terminal 2** (TUI viewer):
```bash
./ltt
```

**What to verify**:
- [ ] TUI starts without errors
- [ ] Connects to socket
- [ ] UI renders properly
- [ ] Can quit with 'q'

**Current limitation**: Protocol encoding not implemented, so spans won't display yet.

---

## Phase 7: Race Detection

**Critical for concurrency correctness**:

```bash
# Test all packages with race detector
go test -race ./...

# Run example with race detector
go run -race ./examples/simple/main.go
```

**Must pass with zero races**.

---

## Phase 8: CI/CD Testing (Future)

### 8.1 GitHub Actions Workflow

```yaml
name: Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      
      - name: Test
        run: make test
      
      - name: Benchmark
        run: make bench
      
      - name: Race Detection
        run: go test -race ./...
```

---

## Current Testing Status

### ✅ Working Now
- [x] Compilation (all packages build)
- [x] Ring buffer unit tests
- [x] Ring buffer benchmarks
- [x] Socket creation
- [x] Exporter initialization
- [x] Basic span export flow

### ⏳ Needs Protocol Implementation
- [ ] Protocol encode/decode tests
- [ ] End-to-end span flow
- [ ] TUI span rendering
- [ ] Socket message reading

### 🎯 Next Steps for Full Testing

1. **Implement Protocol Codec** ([pkg/protocol/span.go:47-60](pkg/protocol/span.go#L47-L60))
   ```go
   func (s *SpanMessage) EncodeTo(w io.Writer) error {
       // TODO: Implement binary encoding
   }
   
   func DecodeFrom(r io.Reader) (*SpanMessage, error) {
       // TODO: Implement binary decoding
   }
   ```

2. **Test Protocol**
   ```bash
   # Add protocol tests
   cd pkg/protocol
   go test -v
   ```

3. **Test End-to-End**
   ```bash
   # Run example + TUI
   go run ./examples/simple/main.go
   ./ltt
   ```

---

## Performance Validation Checklist

When testing performance:

- [ ] `BenchmarkExportSpan` shows **0 allocs/op**
- [ ] `BenchmarkExportSpan` shows **<50ns/op**
- [ ] Stress test achieves **>100k spans/sec**
- [ ] Race detector passes **all tests**
- [ ] Memory profile shows **no leaks**
- [ ] CPU profile shows **minimal lock contention**
- [ ] Drop rate under load **<5%**

---

## Debugging Tips

### Socket Issues
```bash
# Check if socket exists
ls -la /tmp/ltt.sock

# Check socket permissions
stat /tmp/ltt.sock

# Kill stuck processes
lsof /tmp/ltt.sock
kill <PID>

# Clean up
rm /tmp/ltt*.sock
```

### Memory Issues
```bash
# Check for leaks
go test -memprofile=mem.prof ./pkg/exporter
go tool pprof -http=:8080 mem.prof
# Look at "inuse_space" view
```

### Performance Issues
```bash
# Profile CPU
go test -cpuprofile=cpu.prof -bench=. ./pkg/exporter
go tool pprof cpu.prof
(pprof) top10
(pprof) web  # Opens browser visualization
```

---

## Quick Test Commands Summary

```bash
# Basic compilation check
go build ./...

# Unit tests
go test ./...

# Unit tests with race detection
go test -race ./...

# Benchmarks
make bench

# Stress test (after creating test_stress.go)
go run test_stress.go

# Full CI-style test
make test && make bench && go test -race ./...

# Build and run
make build
go run ./examples/simple/main.go &
./ltt
```

---

## What Works Right Now

Run this to see what's functional:

```bash
#!/bin/bash
echo "🧪 LTT Test Suite"
echo ""

echo "1. Compiling..."
if go build ./...; then
    echo "   ✅ All packages compile"
else
    echo "   ❌ Compilation failed"
    exit 1
fi

echo ""
echo "2. Ring Buffer Tests..."
go test -v ./internal/ringbuf | grep -E "PASS|FAIL"

echo ""
echo "3. Ring Buffer Benchmarks..."
go test -bench=. -benchmem ./internal/ringbuf | grep "Benchmark"

echo ""
echo "4. Exporter Tests..."
go test ./pkg/exporter -v 2>&1 | grep -E "PASS|FAIL"

echo ""
echo "✅ Test suite complete!"
echo ""
echo "🎯 Next: Implement protocol codec to enable E2E testing"
```

Save as `run_tests.sh` and execute with `bash run_tests.sh`.
