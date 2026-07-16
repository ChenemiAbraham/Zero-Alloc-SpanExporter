# LTT Observability Roadmap: Traces → Logs → Metrics

## 📊 Current State (What We Have)

✅ **Traces** - Fully implemented
- Custom `sdktrace.SpanExporter` implementation
- Zero-allocation hot path (<50ns overhead)
- Persistent storage with BadgerDB
- Secondary indexes for fast search
- Sampling strategies (probability, rate, tail, adaptive)
- Sub-millisecond query performance
- TUI viewer with waterfall visualization

---

## 🎯 Adding OpenTelemetry Logs & Metrics

### Overview: The Three Signals

```
┌─────────────────────────────────────────────────┐
│  OpenTelemetry Three Pillars                    │
├─────────────────────────────────────────────────┤
│                                                 │
│  ✅ TRACES    (Implemented)                     │
│     - Distributed request flow                  │
│     - Spans with parent-child relationships     │
│     - Duration, status, attributes              │
│                                                 │
│  🟡 LOGS      (Roadmap)                         │
│     - Structured log records                    │
│     - Correlated with traces via trace_id       │
│     - Severity, body, attributes                │
│                                                 │
│  🟡 METRICS   (Roadmap)                         │
│     - Time-series data                          │
│     - Counters, gauges, histograms              │
│     - Aggregation and bucketing                 │
│                                                 │
└─────────────────────────────────────────────────┘
```

---

## 🪵 Adding OpenTelemetry Logs

### What It Takes

#### 1. Implement `sdklog.Exporter` Interface

**Current (Traces):**
```go
type SpanExporter interface {
    ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error
    Shutdown(ctx context.Context) error
}
```

**Needed (Logs):**
```go
type Exporter interface {
    Export(ctx context.Context, records []sdklog.Record) error
    Shutdown(ctx context.Context) error
}
```

#### 2. Log Record Structure

```go
type LogRecord struct {
    Timestamp         time.Time
    ObservedTimestamp time.Time
    TraceID           [16]byte    // Correlation with traces!
    SpanID            [8]byte     // Correlation with traces!
    SeverityNumber    int32       // DEBUG=5, INFO=9, WARN=13, ERROR=17
    SeverityText      string      // "DEBUG", "INFO", "WARN", "ERROR"
    Body              string      // Log message
    Attributes        map[string]interface{}
    Resource          map[string]interface{}
}
```

#### 3. Implementation Complexity: **Medium** 🟡

**Why Medium:**
- ✅ Similar architecture to traces (serialize, store, index)
- ✅ Can reuse BadgerDB storage
- ✅ Can reuse transport layer (socket/TCP)
- ⚠️ Need separate indexes (severity, trace_id correlation)
- ⚠️ Log volume can be MUCH higher than traces
- ⚠️ Need text search for log bodies (full-text search)

#### 4. What Needs to Be Built

```
pkg/logexporter/
├── exporter.go        # Implements sdklog.Exporter
├── record.go          # Log record serialization
└── pool.go            # Buffer pooling for logs

pkg/storage/
├── logs.go            # Log storage (reuse BadgerDB)
└── log_index.go       # Indexes: severity, trace_id, timestamp, text

pkg/search/
└── log_query.go       # Query logs by severity, trace_id, text search

cmd/ltt/
└── logs_view.go       # TUI panel for logs
```

#### 5. Storage Strategy

**Option A: Same Database, Different Key Prefix**
```
<timestamp>:<trace_id>:<span_id> → span data          (existing)
log:<timestamp>:<trace_id>:<severity> → log data      (new)
metric:<timestamp>:<name>:<labels> → metric data      (new)
```

**Option B: Separate Databases** (Recommended)
```
./ltt-data/traces/   → Traces (existing)
./ltt-data/logs/     → Logs (new)
./ltt-data/metrics/  → Metrics (new)
```
Benefits: Independent TTL, compaction, easier to manage

#### 6. Index Strategy for Logs

```go
// Log indexes
idx:log:severity:<level>:<timestamp>:<log_id> → empty
idx:log:trace:<trace_id>:<timestamp>:<log_id> → empty
idx:log:text:<word>:<timestamp>:<log_id> → empty  // Full-text search
```

---

## 📈 Adding OpenTelemetry Metrics

### What It Takes

#### 1. Implement `sdkmetric.Exporter` Interface

```go
type Exporter interface {
    Export(ctx context.Context, metrics *metricdata.ResourceMetrics) error
    Shutdown(ctx context.Context) error
}
```

#### 2. Metric Data Structures

```go
// Three metric types
type Counter struct {
    Name       string
    Value      int64    // Monotonically increasing
    Attributes map[string]string
}

type Gauge struct {
    Name       string
    Value      float64  // Can go up or down
    Attributes map[string]string
}

type Histogram struct {
    Name         string
    Count        uint64
    Sum          float64
    Min          float64
    Max          float64
    Buckets      []Bucket  // Distribution of values
    Attributes   map[string]string
}
```

#### 3. Implementation Complexity: **High** 🔴

**Why High:**
- 🔴 Complex data model (counters, gauges, histograms)
- 🔴 Aggregation required (metrics are cumulative)
- 🔴 Need to handle temporality (delta vs. cumulative)
- 🔴 Time-series storage and querying
- 🔴 Cardinality explosion (many label combinations)
- 🔴 Different visualization needs (time-series graphs)

#### 4. What Needs to Be Built

```
pkg/metricexporter/
├── exporter.go        # Implements sdkmetric.Exporter
├── aggregation.go     # Aggregate metrics over time windows
├── temporality.go     # Handle delta vs. cumulative
└── cardinality.go     # Control label cardinality

pkg/storage/
├── metrics.go         # Time-series storage
└── metric_index.go    # Indexes: name, labels, time

pkg/search/
└── metric_query.go    # Query metrics by name, labels, time range

cmd/ltt/
└── metrics_view.go    # TUI with sparklines/graphs
```

#### 5. Storage Strategy for Metrics

**Challenge:** Metrics are time-series data, different from spans/logs

**Option A: BadgerDB (Keep it simple)**
```
metric:<name>:<labels_hash>:<timestamp> → metric_value
```
- Pros: Reuse existing storage
- Cons: Not optimized for time-series queries

**Option B: Dedicated Time-Series DB** (Recommended for production)
```
Use: VictoriaMetrics, Prometheus TSDB, InfluxDB
```
- Pros: Optimized for metrics
- Cons: Additional dependency

**Option C: In-Memory Aggregation + Periodic Flush**
```
Keep recent metrics in memory (last 5 minutes)
Flush to BadgerDB every minute
```
- Pros: Fast queries, manageable storage
- Cons: Data loss on crash (acceptable for dev tool)

#### 6. Metric Aggregation

**Metrics need aggregation:**
```go
// Raw metrics from app
request_count{path="/users"} = 1
request_count{path="/users"} = 1
request_count{path="/users"} = 1

// Aggregated for storage
request_count{path="/users"}[1m] = 3
request_count{path="/users"}[5m] = 15
```

This is complex and typically handled by Prometheus/VictoriaMetrics.

---

## 📋 Implementation Roadmap

### Phase 1: OpenTelemetry Logs (Estimated: 3-4 days)

**Priority: HIGH** - Logs are easier and very useful for debugging

#### Tasks:
1. ✅ Create `pkg/logexporter/` with `sdklog.Exporter` implementation
2. ✅ Add log record serialization (similar to spans)
3. ✅ Extend storage with log-specific indexes
4. ✅ Add log search API (by severity, trace_id, text)
5. ✅ Update TUI to show logs panel
6. ✅ Add trace-log correlation view

**Difficulty:** Medium 🟡  
**Value:** High 🚀 - Logs + traces together is very powerful

---

### Phase 2: OpenTelemetry Metrics (Estimated: 5-7 days)

**Priority: MEDIUM** - Metrics are complex, consider alternatives

#### Tasks:
1. ⚠️ Create `pkg/metricexporter/` with `sdkmetric.Exporter` implementation
2. ⚠️ Implement metric aggregation logic
3. ⚠️ Add time-series storage strategy
4. ⚠️ Create metric query API
5. ⚠️ Update TUI with metrics dashboard (sparklines, graphs)
6. ⚠️ Handle cardinality and memory limits

**Difficulty:** High 🔴  
**Value:** Medium ⚠️ - Prometheus already does this well

**Alternative:** Just expose Prometheus metrics endpoint, don't store metrics in LTT
```go
// Simpler approach: Use Prometheus exporter
import "github.com/prometheus/client_golang/prometheus"
import "github.com/prometheus/client_golang/prometheus/promhttp"

http.Handle("/metrics", promhttp.Handler())
```

---

## 🎯 Recommended Approach

### Start with Logs Only (Phase 1)

**Why:**
1. ✅ **High value** - Logs + traces correlation is killer feature
2. ✅ **Reasonable complexity** - Similar architecture to traces
3. ✅ **Fills a gap** - No good local log aggregation tool
4. ✅ **Natural fit** - Traces and logs are closely related

**Skip or Defer Metrics (Phase 2)**

**Why:**
1. ⚠️ **High complexity** - Time-series storage is hard
2. ⚠️ **Prometheus exists** - Already excellent local metrics solution
3. ⚠️ **Different use case** - Metrics are for monitoring, not debugging
4. ⚠️ **Cardinality issues** - Easy to explode storage with labels

**For Metrics, Recommend:**
- Use Prometheus for metrics collection
- Use LTT for traces + logs
- Link them via exemplars (Prometheus → LTT trace_id)

---

## 🏗️ Architecture: Three Signals Together

```
┌─────────────────────────────────────────────────────┐
│  Your Go Application                                │
│                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────┐ │
│  │   Tracer     │  │   Logger     │  │  Meter   │ │
│  └──────┬───────┘  └──────┬───────┘  └────┬─────┘ │
│         │                 │                │       │
└─────────┼─────────────────┼────────────────┼───────┘
          ↓                 ↓                ↓
    ┌──────────┐      ┌──────────┐    ┌──────────┐
    │   LTT    │      │   LTT    │    │Prometheus│
    │  Trace   │      │   Log    │    │ Metrics  │
    │ Exporter │      │ Exporter │    │ Exporter │
    └─────┬────┘      └─────┬────┘    └─────┬────┘
          │                 │                │
          ↓                 ↓                ↓
    ┌──────────────────────────────────────────────┐
    │           BadgerDB Storage                    │
    │                                               │
    │  traces/     logs/         (no metrics)      │
    └───────────────────┬──────────────────────────┘
                        ↓
              ┌──────────────────┐
              │   LTT TUI        │
              │                  │
              │  Tabs:           │
              │  • Traces 🔍     │
              │  • Logs 📝       │
              │  • Correlation 🔗│
              └──────────────────┘

              ┌──────────────────┐
              │  Prometheus UI   │ ← Separate tool
              │  (localhost:9090)│
              └──────────────────┘
```

---

## 💡 Key Design Decisions

### 1. Correlation is Key

**Link traces, logs, and metrics:**
```go
// In your app
ctx, span := tracer.Start(ctx, "operation")
defer span.End()

// Logger automatically inherits trace_id from context
logger.InfoContext(ctx, "Processing request", 
    "user_id", 123,
)

// Metric recorded with trace exemplar
requestCounter.Add(ctx, 1,
    attribute.String("endpoint", "/users"),
)
```

**In LTT:**
```go
// Search logs for a specific trace
logs := search.FindLogsByTraceID(traceID)

// Show trace with correlated logs
trace := search.GetTrace(traceID)
for _, span := range trace.Spans {
    logs := search.GetLogsForSpan(span.SpanID)
    // Display side-by-side
}
```

### 2. Unified Storage with Namespaces

```
ltt-data/
├── traces/
│   ├── MANIFEST
│   ├── 000001.vlog
│   └── 000002.sst
├── logs/
│   ├── MANIFEST
│   ├── 000001.vlog
│   └── 000002.sst
└── config.json
```

### 3. Intelligent Sampling

**Different sampling for each signal:**
```go
config := Config{
    Traces: &TraceConfig{
        Sampler: "tail",        // Keep errors + slow
        TTL:     24 * time.Hour,
    },
    Logs: &LogConfig{
        SeverityFilter: "INFO",  // Only INFO and above
        TTL:            6 * time.Hour,
    },
    // No metrics config - use Prometheus
}
```

### 4. TUI Redesign

**Three-panel view:**
```
┌───────────────────────────────────────────────┐
│ [Traces] [Logs] [Correlation]                 │
├───────────────────────────────────────────────┤
│                                               │
│  Traces Panel:                                │
│  • Waterfall chart                            │
│  • Click trace → show correlated logs         │
│                                               │
│  Logs Panel:                                  │
│  • Filterable log stream                      │
│  • Click log → jump to trace                  │
│                                               │
│  Correlation Panel:                           │
│  • Show trace with inline logs                │
│  • Timeline view                              │
│                                               │
└───────────────────────────────────────────────┘
```

---

## 📊 Effort vs. Value Matrix

```
High Value │                    
           │   LOGS ✅           
           │   (Do This)         
           │                    
           │                    TRACES ✅
           │                    (Already Done)
           │                    
Low Value  │   METRICS ⚠️        
           │   (Use Prometheus) 
           │                    
           └────────────────────
             Low Effort  →  High Effort
```

---

## 🎯 Recommendation

### ✅ DO: Add OpenTelemetry Logs

**Benefits:**
- Logs + Traces correlation is incredibly powerful for debugging
- Fills a gap (no good local log aggregation tool)
- Reasonable complexity (similar to traces)
- Natural extension of LTT

**Estimated Effort:** 3-4 days
**Value:** Very High 🚀

### ⚠️ CONSIDER: Defer OpenTelemetry Metrics

**Why:**
- Prometheus already does local metrics extremely well
- High implementation complexity (time-series, aggregation)
- Metrics are for monitoring, LTT is for debugging
- Can integrate later via exemplars

**Alternative:** Use Prometheus + link to LTT traces via trace_id

---

## 🚀 Next Steps (If Adding Logs)

1. Read OTEL Logs Spec: https://opentelemetry.io/docs/specs/otel/logs/
2. Implement `pkg/logexporter/exporter.go`
3. Extend storage with log indexes
4. Add log search API
5. Update TUI with logs panel
6. Create example showing trace-log correlation

---

## 📚 References

- [OpenTelemetry Logs Spec](https://opentelemetry.io/docs/specs/otel/logs/)
- [OpenTelemetry Metrics Spec](https://opentelemetry.io/docs/specs/otel/metrics/)
- [Go OTEL SDK - Logs](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/log)
- [Go OTEL SDK - Metrics](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/metric)

---

## ❓ Decision Time

**Question for you:**

Should we:
1. 🟢 **Add Logs support** (3-4 days, high value)
2. 🔴 **Add Metrics support** (5-7 days, lower value due to Prometheus)
3. 🟡 **Add both** (8-11 days total)
4. 🔵 **Skip both for now** (focus on TUI improvements)

**My recommendation:** Start with **Logs** (#1). It's the biggest bang for buck.
