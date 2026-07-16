# Logs Integration Example

## Status: Work in Progress

This example demonstrates trace-log correlation in LTT. However, it requires the latest OpenTelemetry Logs SDK which has breaking API changes.

**Current Status:**
- ✅ Log storage implemented
- ✅ Log search API implemented
- ✅ Trace-log correlation working
- ✅ All unit tests passing
- ⚠️ OTEL Logs SDK integration needs API update

## What Works Now

You can test the logs functionality using the unit tests:

```bash
# Test log search
go test ./pkg/search -v -run TestLogSearch

# Test trace-log correlation  
go test ./pkg/search -v -run TestTraceLogCorrelation
```

## Architecture

The logs implementation includes:

1. **pkg/logexporter/** - OTEL Logs SDK Exporter
2. **pkg/protocol/log.go** - Log message serialization
3. **pkg/storage/logs.go** - Log storage with indexes
4. **pkg/search/log_query.go** - Log search API
5. **pkg/search/correlation.go** - Trace-log correlation

## API Usage (When OTEL SDK is ready)

```go
// Setup log exporter
logExp, _ := logexporter.New(logexporter.Config{
    Storage: store,
})

// Setup OTEL provider
lp := sdklog.NewLoggerProvider(
    sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
)
global.SetLoggerProvider(lp)

// Logs are automatically linked to traces via context
ctx, span := tracer.Start(ctx, "operation")
logger.InfoContext(ctx, "message") // Includes trace_id!

// Search logs
engine := search.NewEngine(store)
query, _ := search.NewLogQuery().
    ForTrace(traceID).
    ErrorAndAbove().
    Build()

result, _ := engine.SearchLogs(query)

// Get trace with all logs
traceWithLogs, _ := engine.GetTraceWithLogs(traceID)
fmt.Printf("Trace has %d spans and %d logs\n", 
    len(traceWithLogs.Spans), 
    len(traceWithLogs.Logs))
```

## Alternative: Direct Storage Testing

You can also test directly with the storage layer:

```go
package main

import (
    "time"
    "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
    "github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
    "go.opentelemetry.io/otel/trace"
)

func main() {
    // Create storage
    cfg := storage.DefaultConfig()
    store, _ := storage.NewStore(cfg)
    defer store.Close()

    // Store a log
    log := &protocol.LogMessage{
        Timestamp:      time.Now(),
        TraceID:        trace.TraceID{1, 2, 3},
        SpanID:         trace.SpanID{4, 5, 6},
        SeverityNumber: 17, // ERROR
        SeverityText:   "ERROR",
        Body:           "Payment failed",
        Attributes:     map[string]interface{}{"amount": 100.0},
    }
    
    store.StoreLog(log)

    // Retrieve logs
    logs, _ := store.GetLogsByTraceID(log.TraceID)
    fmt.Printf("Found %d logs\n", len(logs))
}
```

## Next Steps

To complete this example, we need to:
1. Update to the latest OTEL Logs SDK stable API
2. Use proper Logger.Emit() methods instead of Record struct literals
3. Add structured logging library integration (slog, zap, logrus)

For now, the core functionality is proven via unit tests!
