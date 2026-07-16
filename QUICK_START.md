# LTT Quick Start - 3 Minutes to First Trace

## 🚀 Fastest Way to Get Started

### Step 1: Install (30 seconds)

```bash
# In your Go project
go get github.com/ChenemiAbraham/Zero-Alloc-SpanExporter
go get go.opentelemetry.io/otel/sdk
```

### Step 2: Add to Your Code (1 minute)

```go
package main

import (
    "context"
    "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/exporter"
    "go.opentelemetry.io/otel"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
    // Initialize LTT
    exp, _ := exporter.New(exporter.DefaultConfig())
    defer exp.Shutdown(context.Background())

    tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
    defer tp.Shutdown(context.Background())
    
    otel.SetTracerProvider(tp)

    // ✅ That's it! Now create spans normally:
    tracer := otel.Tracer("my-app")
    
    ctx, span := tracer.Start(context.Background(), "my-operation")
    defer span.End()
    
    // ... your code ...
}
```

### Step 3: View Traces (1 minute)

**Terminal 1 - Run your app:**
```bash
go run main.go
```

**Terminal 2 - Start viewer:**
```bash
cd path/to/ZasExporter-Go
go run ./cmd/ltt
```

**Done!** You'll see traces appear in the TUI. 🎉

---

## 📖 Key Concepts

### 1. LTT is In-Process
- **NOT** an external service like Jaeger
- Import as a library in your Go app
- Zero network latency, zero infrastructure

### 2. Standard OpenTelemetry
- Uses official OTEL SDK
- Just swap the exporter
- Works with existing OTEL code

### 3. Two Components
- **Exporter** (in your app) - Captures spans
- **Viewer** (separate) - Displays spans

---

## 🎯 Common Use Cases

### "I want persistent storage"

```go
import "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"

config := exporter.DefaultConfig()
storageConfig := storage.DefaultConfig()
storageConfig.Path = "./traces"
config.Storage = &storageConfig

exp, _ := exporter.New(config)
```

### "I want to sample traces"

```go
import "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/sampler"

config := exporter.DefaultConfig()
config.Sampler = &sampler.Config{
    Type:        "probability",
    Probability: 0.1,  // Keep 10%
}

exp, _ := exporter.New(config)
```

### "I want to search traces"

```go
import "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/search"

engine := search.NewEngine(store)

// Find all errors in last hour
query, _ := search.NewQuery().
    WithError().
    Last(1 * time.Hour).
    Build()

results, _ := engine.Search(query)
```

### "I want LTT + Jaeger (both)"

```go
lttExp, _ := lttexporter.New(lttexporter.DefaultConfig())
jaegerExp, _ := jaeger.New(...)

tp := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(lttExp),     // Local dev
    sdktrace.WithBatcher(jaegerExp),  // Production
)
```

---

## 🛠️ Configuration Cheatsheet

```go
config := exporter.Config{
    SocketPath:    "127.0.0.1:9090",      // Socket address
    BufferSize:    10000,                 // Max buffered spans
    BatchSize:     100,                   // Spans per export
    ExportTimeout: 5 * time.Second,       // Export timeout
    Storage:       &storageConfig,        // Optional: persistent storage
    Sampler:       &samplerConfig,        // Optional: sampling
}
```

---

## 📊 Verify It's Working

```go
// Print stats
stats := exp.GetStats()
fmt.Printf("Exported: %d, Dropped: %d\n", 
    stats.ExportedSpans, 
    stats.DroppedSpans,
)
```

If `ExportedSpans` is increasing, LTT is capturing traces! ✅

---

## 🔧 Troubleshooting

| Problem | Solution |
|---------|----------|
| No spans in TUI | Make sure app runs BEFORE viewer |
| "Connection refused" | Use TCP socket on Windows: `127.0.0.1:9090` |
| TUI shows old spans | Storage enabled - traces persist |
| Too many spans | Enable sampling |

---

## 📚 Learn More

- **[INTEGRATION_GUIDE.md](./INTEGRATION_GUIDE.md)** - Detailed integration guide
- **[examples/](./examples/)** - Working code examples
- **[CLAUDE.md](./CLAUDE.md)** - Full project documentation

---

## 🎯 Next Steps

1. ✅ Add LTT to your app (see Step 2 above)
2. ✅ Start TUI viewer (see Step 3 above)
3. ✅ Generate some traces
4. ✅ Try searching: `/code-review` → see search examples
5. ✅ Configure sampling/storage as needed

Happy tracing! 🚀

---

## 💡 Pro Tips

- Use **LTT for local dev**, **OTLP/Jaeger for production**
- Enable **storage** to view traces after app stops
- Use **tail sampling** to keep errors + slow spans
- The **search API** is faster than filtering in TUI
- Check `exp.GetStats()` if spans aren't showing up
