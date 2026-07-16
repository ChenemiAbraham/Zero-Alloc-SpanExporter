# LTT Integration Guide

How to point your applications to Local Trace Tap for trace visualization.

---

## 🎯 Overview

**Local Trace Tap (LTT)** is an **in-process exporter** - you import it as a Go library into your application. It's **NOT** an external collector like Jaeger/Zipkin.

```
┌─────────────────────┐
│   Your Go App       │
│                     │
│  ┌──────────────┐   │
│  │ LTT Exporter │   │ ← Import as library
│  └──────┬───────┘   │
└─────────┼───────────┘
          │
          ↓ (writes spans)
    ┌─────────────┐
    │  BadgerDB   │ ← Persistent storage
    │   Storage   │
    └─────────────┘
          ↑
          │ (reads spans)
    ┌─────────────┐
    │  LTT TUI    │ ← Separate viewer
    │   Viewer    │
    └─────────────┘
```

---

## 🚀 Integration Methods

### Method 1: OpenTelemetry SDK (Recommended)

Use LTT as a standard OpenTelemetry exporter in your Go application.

#### Step 1: Install Dependencies

```bash
go get github.com/ChenemiAbraham/Zero-Alloc-SpanExporter
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/sdk
```

#### Step 2: Initialize LTT Exporter

```go
package main

import (
    "context"
    "log"

    "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
    "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
    "go.opentelemetry.io/otel"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
    // Configure LTT exporter
    config := exporter.DefaultConfig()
    
    // Optional: Enable persistent storage
    storageConfig := storage.DefaultConfig()
    storageConfig.Path = "./traces"
    storageConfig.TTL = 24 * time.Hour
    config.Storage = &storageConfig
    
    // Create exporter
    exp, err := exporter.New(config)
    if err != nil {
        log.Fatalf("Failed to create LTT exporter: %v", err)
    }
    defer exp.Shutdown(context.Background())

    // Create trace provider
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),           // Use batcher for async export
        sdktrace.WithSampler(sdktrace.AlwaysSample()),
    )
    defer tp.Shutdown(context.Background())

    // Set as global provider
    otel.SetTracerProvider(tp)

    // Now use OpenTelemetry normally!
    tracer := otel.Tracer("my-service")
    
    ctx, span := tracer.Start(context.Background(), "my-operation")
    defer span.End()
    
    // ... your code ...
}
```

#### Step 3: Run Your App + TUI Viewer

**Terminal 1 - Run your app:**
```bash
go run main.go
```

**Terminal 2 - Start TUI viewer:**
```bash
cd path/to/ZasExporter-Go
go run ./cmd/ltt
```

---

### Method 2: Multiple Exporters (LTT + Prometheus/Jaeger)

Send traces to **both** LTT (for local dev) and external systems (for production).

```go
import (
    lttexporter "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initTracing() (*sdktrace.TracerProvider, error) {
    // 1. Create LTT exporter (local dev)
    lttExp, err := lttexporter.New(lttexporter.DefaultConfig())
    if err != nil {
        return nil, err
    }

    // 2. Create OTLP exporter (production - optional)
    otlpExp, err := otlptracehttp.New(context.Background(),
        otlptracehttp.WithEndpoint("otel-collector:4318"),
        otlptracehttp.WithInsecure(),
    )
    if err != nil {
        return nil, err
    }

    // 3. Use both exporters
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(lttExp),    // Local visualization
        sdktrace.WithBatcher(otlpExp),   // Production backend
        sdktrace.WithSampler(sdktrace.AlwaysSample()),
    )

    otel.SetTracerProvider(tp)
    return tp, nil
}
```

---

### Method 3: Replace Jaeger Exporter with LTT

If you're currently using Jaeger locally, just swap the exporter:

**Before (Jaeger):**
```go
import "go.opentelemetry.io/otel/exporters/jaeger"

exp, _ := jaeger.New(jaeger.WithCollectorEndpoint(
    jaeger.WithEndpoint("http://localhost:14268/api/traces"),
))
```

**After (LTT):**
```go
import "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"

exp, _ := exporter.New(exporter.DefaultConfig())
```

Everything else stays the same! LTT implements the standard `sdktrace.SpanExporter` interface.

---

## ⚙️ Configuration Options

### Basic Configuration

```go
config := exporter.Config{
    // Network transport
    SocketPath: "127.0.0.1:9090",  // TCP socket (Windows/Linux)
    // SocketPath: "/tmp/ltt.sock", // Unix socket (Linux/Mac)
    
    // Buffer settings
    BufferSize: 10000,  // Max spans to buffer
    
    // Export behavior
    BatchSize:    100,             // Spans per batch
    ExportTimeout: 5 * time.Second, // Timeout per export
}
```

### With Persistent Storage

```go
import (
    "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
    "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
)

config := exporter.DefaultConfig()

// Enable storage
storageConfig := storage.Config{
    Path:            "./ltt-data",
    TTL:             24 * time.Hour,  // Keep traces for 24h
    SyncWrites:      false,           // Async writes (faster)
    CompactInterval: 1 * time.Hour,   // GC interval
}
config.Storage = &storageConfig
```

### With Sampling

```go
import (
    "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
    "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/sampler"
)

config := exporter.DefaultConfig()

// Sample 10% of traces
config.Sampler = &sampler.Config{
    Type:        "probability",
    Probability: 0.1,  // 10%
}

// OR: Keep errors + slow spans, sample others at 1%
config.Sampler = &sampler.Config{
    Type: "tail",
    Tail: sampler.TailConfig{
        SampleErrors:    true,
        SlowThreshold:   500 * time.Millisecond,
        BaseProbability: 0.01,
    },
}
```

---

## 🔌 Prometheus Integration

**LTT does NOT export Prometheus metrics directly**, but you can:

### Option 1: Use OTEL Metrics + LTT Traces

```go
import (
    "go.opentelemetry.io/otel/metric"
    "go.opentelemetry.io/otel/exporters/prometheus"
)

// Traces → LTT
traceProvider := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(lttExporter),
)

// Metrics → Prometheus
promExporter, _ := prometheus.New()
meterProvider := metric.NewMeterProvider(
    metric.WithReader(promExporter),
)

// Use both
otel.SetTracerProvider(traceProvider)
otel.SetMeterProvider(meterProvider)
```

### Option 2: Generate RED Metrics from Spans (Future Feature)

LTT could generate RED metrics (Rate, Errors, Duration) from stored spans.
This is on the roadmap but not yet implemented.

---

## 🎨 Using LTT with Existing Apps

### For Go Apps Already Using OpenTelemetry

**Just swap the exporter** - that's it! 

```diff
- import "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
+ import "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"

- exp, _ := otlptrace.New(...)
+ exp, _ := exporter.New(exporter.DefaultConfig())
```

### For Apps NOT Using OpenTelemetry

Add OpenTelemetry SDK + LTT:

```bash
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/sdk
go get github.com/ChenemiAbraham/Zero-Alloc-SpanExporter
```

Then instrument your code:
```go
tracer := otel.Tracer("my-service")

ctx, span := tracer.Start(context.Background(), "operation-name")
defer span.End()

// Add attributes
span.SetAttributes(
    attribute.String("http.method", "GET"),
    attribute.Int("http.status_code", 200),
)
```

---

## 📊 Architecture Comparison

### LTT vs. Jaeger/Zipkin

| Feature | LTT | Jaeger/Zipkin |
|---------|-----|---------------|
| **Deployment** | In-process library | External collector |
| **Network** | Unix socket (local) | HTTP/gRPC (network) |
| **Latency** | <50ns per span | ~1-10ms per span |
| **Setup** | `go get` + 3 lines code | Docker, config, ports |
| **Storage** | BadgerDB (embedded) | Cassandra/Elasticsearch |
| **UI** | TUI (terminal) | Web UI |
| **Use Case** | Local development | Production monitoring |

### Why Use LTT?

✅ **Zero network latency** - in-process, no HTTP calls  
✅ **Zero infrastructure** - no Docker, no Kafka, no databases  
✅ **Zero configuration** - works out of the box  
✅ **Fast iteration** - see traces instantly during dev  
✅ **Works offline** - no internet/VPN needed  

---

## 🔍 Verifying Integration

### Check if LTT is receiving spans:

```go
stats := exp.GetStats()
fmt.Printf("Exported: %d | Dropped: %d | Buffer: %.1f%%\n",
    stats.ExportedSpans,
    stats.DroppedSpans,
    stats.BufferUsage,
)
```

### Query stored spans:

```go
import "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/search"

engine := search.NewEngine(store)

query, _ := search.NewQuery().
    WithError().
    Last(1 * time.Hour).
    Build()

results, _ := engine.Search(query)
fmt.Printf("Found %d error spans\n", results.Total)
```

---

## 🛠️ Troubleshooting

### "Connection refused" error

**Problem:** TUI viewer can't connect to socket.

**Solutions:**
1. Make sure your app is running first (creates the socket)
2. Check `SocketPath` matches in both app and viewer
3. On Windows, use TCP: `127.0.0.1:9090` instead of Unix socket

### "No spans showing up"

**Problem:** TUI is empty even though app is running.

**Solutions:**
1. Check if spans are actually being created (`tracer.Start(...)`)
2. Verify exporter is initialized before creating spans
3. Check sampling config - you might be dropping spans
4. Look at `exp.GetStats()` to see if spans are exported

### "Storage is full"

**Problem:** BadgerDB using too much disk space.

**Solutions:**
1. Lower TTL: `storageConfig.TTL = 1 * time.Hour`
2. Enable sampling: `config.Sampler = &sampler.Config{...}`
3. Clear old data: `rm -rf ./ltt-data`

---

## 📚 Examples

See working examples in the repo:

- **[examples/simple/](../examples/simple/)** - Basic integration
- **[examples/with-storage/](../examples/with-storage/)** - Persistent traces
- **[examples/with-sampling/](../examples/with-sampling/)** - Sampling strategies
- **[examples/search-demo/](../examples/search-demo/)** - Search API usage

---

## 🚦 Production Considerations

**LTT is designed for LOCAL DEVELOPMENT, not production.**

For production:
- Use **OTLP exporter** → Jaeger/Tempo/SignalFx
- Use **multi-exporter setup** (LTT for dev, OTLP for prod)
- Enable sampling in production to control costs

```go
func getExporter() sdktrace.SpanExporter {
    if os.Getenv("ENV") == "production" {
        return otlpExporter() // Cloud-based
    }
    return lttExporter()      // Local dev
}
```

---

## ❓ FAQ

**Q: Can I use LTT with non-Go languages?**  
A: Not directly. LTT is a Go library. For other languages, use OTLP exporter.

**Q: Can LTT receive OTLP spans over HTTP?**  
A: No, LTT is in-process only. For remote spans, use a collector.

**Q: Does LTT work with Prometheus?**  
A: LTT handles traces, not metrics. Use OTEL metrics SDK for Prometheus.

**Q: Can multiple apps share one LTT instance?**  
A: No, each app embeds its own LTT exporter. But you can use shared storage.

**Q: Does LTT replace DataDog/NewRelic?**  
A: No, LTT is for local dev. Use APM tools for production monitoring.

---

## 🎯 Next Steps

1. **Add LTT to your app** - See Method 1 above
2. **Start the TUI viewer** - `go run ./cmd/ltt`
3. **Generate traces** - Run your app normally
4. **Search spans** - Use the search API or TUI filters
5. **Configure sampling** - Reduce data volume if needed

Happy tracing! 🚀
