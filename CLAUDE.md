# Local Trace Tap (LTT) - Real-Time CLI Trace Visualizer

## Project Overview

A high-performance, terminal-based trace visualization tool for OpenTelemetry spans that eliminates the need for external SaaS platforms or heavy containers during local development.

## Architecture

### Core Components

1. **Zero-Allocation Memory Exporter** (`pkg/exporter/`)
   - Custom `sdktrace.SpanExporter` implementation
   - Unix domain socket transport (Windows: named pipes)
   - Memory pooling with `sync.Pool` for zero-heap allocation
   - Non-blocking channel architecture with ring buffers
   - Graceful degradation on backpressure

2. **TUI Viewer** (`cmd/ltt/`)
   - Built with Bubbletea framework
   - Interactive hierarchical waterfall visualization
   - Real-time span ingestion
   - Keyboard navigation and filtering
   - Export capabilities

### Data Flow

```
Go App (with LTT Exporter)
    ↓ (serialize span)
sync.Pool (reuse []byte buffers)
    ↓ (write to socket)
Unix Domain Socket / Named Pipe
    ↓ (read from socket)
TUI Listener (Bubbletea)
    ↓ (parse & render)
Terminal Display (Waterfall Chart)
```

## Technical Requirements

### Dependencies

- Go 1.23+
- OpenTelemetry SDK (`go.opentelemetry.io/otel`)
- Bubbletea (`github.com/charmbracelet/bubbletea`)
- Bubbles (`github.com/charmbracelet/bubbles`)
- Lipgloss (`github.com/charmbracelet/lipgloss`)

### Performance Targets

- **Exporter overhead**: <50ns per span export
- **Memory allocation**: Zero heap allocations in hot path
- **Throughput**: 100k+ spans/second
- **Latency**: <1ms socket write time
- **TUI refresh**: 60fps rendering

## Project Structure

```
ZasExporter-Go/
├── cmd/
│   └── ltt/              # TUI viewer CLI
│       └── main.go
├── pkg/
│   ├── exporter/         # Core exporter logic
│   │   ├── exporter.go   # SpanExporter implementation
│   │   ├── socket.go     # Socket transport
│   │   ├── pool.go       # Memory pool management
│   │   └── codec.go      # Serialization logic
│   ├── viewer/           # TUI components
│   │   ├── model.go      # Bubbletea model
│   │   ├── view.go       # Rendering logic
│   │   ├── waterfall.go  # Waterfall chart
│   │   └── trace.go      # Trace data structures
│   └── protocol/         # Wire protocol
│       └── span.go       # Span message format
├── examples/
│   └── simple/           # Example instrumented app
│       └── main.go
├── internal/
│   └── ringbuf/          # Lock-free ring buffer
│       └── ringbuf.go
├── go.mod
├── go.sum
├── README.md
└── CLAUDE.md
```

## Development Guidelines

### Memory Safety

- All span serialization must use `sync.Pool` for buffer reuse
- Never allocate in the hot path (exporter.ExportSpans)
- Use ring buffers for channel-free span queueing
- Implement backpressure handling to prevent memory leaks

### Concurrency Patterns

- Exporter runs in separate goroutine per socket write
- Non-blocking channel sends with select + default
- Graceful shutdown with context cancellation
- TUI updates via message passing (Bubbletea Msg pattern)

### Error Handling

- Exporter must never panic or block the host app
- Socket failures should log but continue
- TUI crashes should not affect the exporter
- Implement dead-letter queue for dropped spans (optional)

## Building & Running

### Build the TUI viewer
```bash
go build -o ltt ./cmd/ltt
```

### Run the example app
```bash
go run ./examples/simple
```

### In another terminal, start the viewer
```bash
./ltt
```

## Key Behaviors

### Exporter Initialization

The exporter creates a Unix domain socket (or named pipe on Windows) at a well-known path (e.g., `/tmp/ltt.sock` or `\\.\pipe\ltt`).

### Span Export Flow

1. App emits span via OTEL SDK
2. Exporter's `ExportSpans()` is called
3. Span is serialized using pooled buffer
4. Buffer is written to socket (non-blocking)
5. Buffer is returned to pool
6. TUI reads from socket and updates display

### TUI Interaction

- Arrow keys: Navigate spans
- Enter: Expand/collapse span details
- `/`: Filter by service/operation
- `q`: Quit
- `e`: Export current view to file

## Testing Strategy

- Unit tests for serialization/deserialization
- Benchmark tests for allocation profiling
- Integration tests with example app
- Stress tests for high-throughput scenarios
- TUI snapshot tests (golden files)

## Performance Monitoring

Use these commands during development:

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.

# Memory profiling
go test -memprofile=mem.prof -bench=.

# Allocation tracking
go test -benchmem -bench=.

# Race detection
go test -race ./...

# View profiles
go tool pprof cpu.prof
```

## Future Enhancements

- [ ] Distributed trace correlation
- [ ] Metric integration (RED metrics per span)
- [ ] Log correlation
- [ ] Span sampling strategies
- [ ] Multi-service trace aggregation
- [ ] Web-based viewer alternative
- [ ] Span analytics (P95, P99 latency)
- [ ] Export to Jaeger/Zipkin format
