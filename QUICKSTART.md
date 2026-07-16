# Local Trace Tap - Quick Start Guide

## 🚀 Get Started in 3 Minutes

### Prerequisites
- Go 1.23+ installed
- Terminal with 256-color support

### Step 1: Install Dependencies
```bash
cd ZasExporter-Go
go mod tidy
```

### Step 2: Build the TUI Viewer
```bash
make build
# OR
go build -o ltt ./cmd/ltt
```

### Step 3: Run the Example App
In one terminal:
```bash
go run ./examples/simple/main.go
```

### Step 4: Start the TUI Viewer
In another terminal:
```bash
./ltt
```

You should see traces flowing in real-time! 🎉

---

## 📁 Project Structure

```
ZasExporter-Go/
├── cmd/
│   └── ltt/                    # TUI viewer executable
│       └── main.go
├── pkg/
│   ├── exporter/               # Core exporter (zero-allocation)
│   │   ├── exporter.go         # OTEL SpanExporter implementation
│   │   ├── socket.go           # Unix socket / named pipe transport
│   │   └── pool.go             # sync.Pool for buffer reuse
│   ├── protocol/               # Wire protocol
│   │   └── span.go             # Binary serialization format
│   └── viewer/                 # TUI components
│       ├── model.go            # Bubbletea model
│       ├── trace.go            # Trace tree data structure
│       └── waterfall.go        # Waterfall chart renderer
├── internal/
│   └── ringbuf/                # Lock-free ring buffer
│       └── ringbuf.go          # SPSC ring buffer implementation
├── examples/
│   └── simple/                 # Example instrumented app
│       └── main.go
└── tests/                      # Test files
```

---

## 🔧 Configuration

### Environment Variables

```bash
# Change socket path
export LTT_SOCKET=/tmp/my-traces.sock

# Set log level
export LTT_LOG_LEVEL=debug
```

### Programmatic Configuration

```go
exp, err := exporter.New(exporter.Config{
    SocketPath:            "/tmp/ltt.sock",
    BufferSize:            8192,              // Must be power of 2
    InitialBufferCapacity: 4096,              // Bytes
    MaxBatchSize:          100,               // Spans per batch
    FlushInterval:         100 * time.Millisecond,
})
```

---

## 🧪 Testing

### Run all tests
```bash
make test
```

### Run benchmarks
```bash
make bench
```

### Check for race conditions
```bash
make test-race
```

### Generate coverage report
```bash
make test-coverage
open coverage.html
```

---

## 🎯 Key Features

### Zero-Allocation Exporter
Uses `sync.Pool` and ring buffers to eliminate heap allocations in the hot path.

```go
// Benchmark results (target)
BenchmarkExportSpan-8    24,000,000    48.2 ns/op    0 B/op    0 allocs/op
```

### Lock-Free Ring Buffer
SPSC (Single Producer, Single Consumer) ring buffer with atomic operations.

```go
// Usage
rb := ringbuf.NewRingBuffer(8192)
rb.TryPush(item)  // Non-blocking
item := rb.Pop()  // Non-blocking
```

### Real-Time Visualization
Interactive TUI with:
- Hierarchical span tree
- Duration-scaled waterfall bars
- Keyboard navigation
- Search and filtering

---

## 📊 Performance Targets

| Metric | Target | Current Status |
|--------|--------|----------------|
| Export overhead | <50ns/span | ⏳ In progress |
| Throughput | 100k+ spans/sec | ⏳ In progress |
| Allocations | 0 in hot path | ⏳ In progress |
| Socket latency | <1ms | ⏳ In progress |
| TUI frame rate | 60fps | ⏳ In progress |

---

## 🐛 Troubleshooting

### Socket permission denied
```bash
# Linux/Mac: Check socket permissions
ls -l /tmp/ltt.sock

# Windows: Run as administrator
```

### Connection refused
```bash
# Make sure example app is running FIRST
# The exporter creates the socket, viewer connects to it
```

### Spans not showing up
```bash
# Check if spans are being exported
# Look for stats in example app output:
# Spans: 42 | Dropped: 0 | Buffer: 0.5%
```

### TUI rendering issues
```bash
# Ensure terminal supports 256 colors
echo $TERM  # Should be xterm-256color or similar

# Try resizing terminal window
# Minimum recommended: 80x24
```

---

## 📚 Next Steps

1. **Read the Architecture**: [CLAUDE.md](CLAUDE.md)
2. **Check the Project Plan**: [PROJECT_PLAN.md](PROJECT_PLAN.md)
3. **Explore Examples**: [examples/](examples/)
4. **Run Benchmarks**: `make bench`
5. **Profile Performance**: `make profile-cpu`

---

## 🤝 Contributing

### Development Workflow

```bash
# 1. Make changes
vim pkg/exporter/exporter.go

# 2. Format code
make fmt

# 3. Run tests
make test

# 4. Run benchmarks (if touching hot path)
make bench

# 5. Check allocations
go test -benchmem -bench=BenchmarkExportSpan ./pkg/exporter
```

### Commit Messages
Follow conventional commits:
```
feat: add span filtering to TUI
fix: resolve race condition in ring buffer
perf: optimize buffer pool allocation
docs: update quickstart guide
test: add benchmarks for codec
```

---

## 🎓 Learning Resources

### Understanding the Stack

1. **OpenTelemetry SDK**
   - [OTEL Go Docs](https://opentelemetry.io/docs/languages/go/)
   - [Span Exporter Interface](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace#SpanExporter)

2. **Bubbletea TUI Framework**
   - [Bubbletea Tutorial](https://github.com/charmbracelet/bubbletea/tree/master/tutorials)
   - [Bubbles Components](https://github.com/charmbracelet/bubbles)

3. **Performance Optimization**
   - [sync.Pool](https://pkg.go.dev/sync#Pool)
   - [pprof Profiling](https://go.dev/blog/pprof)
   - [Lock-Free Programming](https://preshing.com/20120612/an-introduction-to-lock-free-programming/)

---

## ⚡ Pro Tips

### Maximize Performance

1. **Buffer sizing**: Set `BufferSize` to power of 2 for efficient masking
2. **Batch size**: Larger batches = better throughput, higher latency
3. **Flush interval**: Lower = lower latency, higher syscall overhead

### Debugging

```bash
# Enable OTEL SDK logging
export OTEL_LOG_LEVEL=debug

# Profile the exporter
go test -cpuprofile=cpu.prof -bench=BenchmarkExportSpan ./pkg/exporter
go tool pprof -http=:8080 cpu.prof

# Check for memory leaks
go test -memprofile=mem.prof -bench=. ./pkg/exporter
go tool pprof -http=:8080 mem.prof
```

### TUI Shortcuts

- `↑/↓` or `j/k`: Navigate spans
- `Enter` or `Space`: Expand/collapse
- `/`: Open filter
- `r`: Refresh (clear traces)
- `e`: Export to file
- `q` or `Ctrl+C`: Quit

---

## 🏆 Showcase Your Work

Built something cool with LTT? Share it!

1. **Twitter/X**: Tag `@anthropic` and use `#LocalTraceTap`
2. **Reddit**: Post to `/r/golang`
3. **Show HN**: [Hacker News](https://news.ycombinator.com/submit)
4. **GitHub**: Star the repo and share your use case

---

## 📞 Get Help

- **Issues**: [GitHub Issues](https://github.com/yourusername/ltt/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourusername/ltt/discussions)
- **Email**: your.email@example.com

---

**Happy Tracing!** 🔍✨
