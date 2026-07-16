# Local Trace Tap (LTT) 🔍

> A blazingly fast, terminal-based trace visualizer for OpenTelemetry spans. No SaaS. No containers. Just pure Go performance.

## Why LTT?

**The Problem**: During local development, you instrument your code with OpenTelemetry spans, but then you have to:
- Wait for data to ship to external SaaS (Datadog, Honeycomb)
- Spin up heavy containers (Jaeger, Tempo)
- SSH into production just to see if your traces work

**The Solution**: LTT gives you instant, local trace visualization with:
- ⚡ **Zero-allocation exporter** - <50ns overhead per span
- 🖥️ **Beautiful TUI** - Interactive waterfall charts in your terminal
- 🚀 **100k+ spans/second** - Handle high-throughput workloads
- 🔌 **Drop-in replacement** - Standard OpenTelemetry exporter interface

## Quick Start

### Install

```bash
go install github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/cmd/ltt@latest
```

### Add to your Go app

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/sdk/trace"
    "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
)

func main() {
    // Create LTT exporter
    exp, err := exporter.New(exporter.Config{
        SocketPath: "/tmp/ltt.sock",
    })
    if err != nil {
        panic(err)
    }
    defer exp.Shutdown(context.Background())

    // Register with OTEL
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exp),
    )
    otel.SetTracerProvider(tp)

    // Your instrumented code here...
}
```

### Start the viewer

```bash
ltt
```

## Features

### Real-Time Visualization

```
15:02:44  [GET /users/:id]  (120ms)  ████████████░░░░
  └─── [DB SELECT Users]  (18ms)     ██░░
  └─── [Redis GET Cache]  (2ms)      ░
  └─── [Transform Response] (8ms)    █░
```

### Interactive Navigation

- **Arrow Keys**: Navigate spans
- **Enter**: Expand/collapse details
- **`/`**: Filter by service/operation
- **`t`**: Toggle timing view
- **`e`**: Export to JSON
- **`q`**: Quit

### Performance Dashboard

```
┌─ Statistics ────────────────────┐
│ Total Spans:    1,247           │
│ Avg Latency:    45ms            │
│ P95 Latency:    180ms           │
│ P99 Latency:    320ms           │
│ Error Rate:     0.2%            │
└─────────────────────────────────┘
```

## Architecture

### Zero-Allocation Design

LTT uses `sync.Pool` to eliminate heap allocations during span export:

```go
type Exporter struct {
    bufferPool sync.Pool
    // ...
}

func (e *Exporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
    buf := e.bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        e.bufferPool.Put(buf)
    }()
    
    // Serialize and write with zero allocations
    // ...
}
```

### Non-Blocking Architecture

The exporter never blocks your application:

```go
select {
case e.spanChan <- span:
    // Span queued
default:
    // Channel full, increment drop counter
    atomic.AddUint64(&e.droppedSpans, 1)
}
```

## Benchmarks

```
BenchmarkExportSpan-8           24,000,000    48.2 ns/op    0 B/op    0 allocs/op
BenchmarkSocketWrite-8          10,000,000    121 ns/op     0 B/op    0 allocs/op
BenchmarkTUIRender-8             1,000,000    1,234 ns/op   256 B/op  2 allocs/op
```

## Platform Support

- ✅ **Linux**: Unix domain sockets
- ✅ **macOS**: Unix domain sockets
- ✅ **Windows**: Named pipes (`\\.\pipe\ltt`)

## Development

### Build from source

```bash
git clone https://github.com/ChenemiAbraham/Zero-Alloc-SpanExporter.git
cd Zero-Alloc-SpanExporter
go build -o ltt ./cmd/ltt
```

### Run tests

```bash
go test ./...
```

### Run benchmarks

```bash
go test -bench=. -benchmem ./pkg/exporter
```

### Profile memory

```bash
go test -memprofile=mem.prof -bench=. ./pkg/exporter
go tool pprof mem.prof
```

## Examples

See [`examples/`](./examples/) for complete working examples:

- **Simple HTTP server** - Basic tracing with HTTP handlers
- **Microservices** - Multi-service trace propagation
- **Database operations** - Span hierarchy with DB queries
- **Error scenarios** - Failed spans and error attributes

## Contributing

Contributions welcome! Please see [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

MIT License - see [LICENSE](./LICENSE)

## Credits

Built with:
- [OpenTelemetry Go SDK](https://github.com/open-telemetry/opentelemetry-go)
- [Bubbletea](https://github.com/charmbracelet/bubbletea) by Charm
- [Lipgloss](https://github.com/charmbracelet/lipgloss) by Charm

---

**Show HN**: If you're tired of waiting for traces to show up in SaaS dashboards, give LTT a try! ⚡
