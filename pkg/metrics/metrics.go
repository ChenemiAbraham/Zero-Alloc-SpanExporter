package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for LTT
type Metrics struct {
	// Span metrics
	spansReceived   prometheus.Counter
	spansDropped    prometheus.Counter
	spansExported   prometheus.Counter
	spansFailed     prometheus.Counter

	// Latency metrics
	exportDuration  prometheus.Histogram
	encodeDuration  prometheus.Histogram

	// Resource metrics
	bufferUsage     prometheus.Gauge
	bufferCapacity  prometheus.Gauge
	goroutines      prometheus.Gauge
	memoryUsage     prometheus.Gauge

	// Connection metrics
	activeConnections prometheus.Gauge
	totalConnections  prometheus.Counter

	// Internal state
	startTime time.Time
	mu        sync.RWMutex
}

var (
	// Global metrics instance
	globalMetrics *Metrics
	once          sync.Once
)

// New creates a new Metrics instance with Prometheus collectors
func New(namespace string) *Metrics {
	if namespace == "" {
		namespace = "ltt"
	}

	m := &Metrics{
		startTime: time.Now(),

		// Span counters
		spansReceived: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "spans_received_total",
			Help:      "Total number of spans received",
		}),

		spansDropped: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "spans_dropped_total",
			Help:      "Total number of spans dropped due to buffer full",
		}),

		spansExported: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "spans_exported_total",
			Help:      "Total number of spans successfully exported",
		}),

		spansFailed: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "spans_failed_total",
			Help:      "Total number of spans that failed to export",
		}),

		// Latency histograms
		exportDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "export_duration_microseconds",
			Help:      "Time spent exporting spans in microseconds",
			Buckets:   prometheus.ExponentialBuckets(10, 2, 10), // 10µs to 5ms
		}),

		encodeDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "encode_duration_microseconds",
			Help:      "Time spent encoding spans in microseconds",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 12), // 1µs to 2ms
		}),

		// Resource gauges
		bufferUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "buffer_usage_bytes",
			Help:      "Current buffer usage in bytes",
		}),

		bufferCapacity: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "buffer_capacity_bytes",
			Help:      "Total buffer capacity in bytes",
		}),

		goroutines: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "goroutines",
			Help:      "Number of goroutines currently running",
		}),

		memoryUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "memory_usage_bytes",
			Help:      "Current memory usage in bytes",
		}),

		// Connection metrics
		activeConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_connections",
			Help:      "Number of currently active TUI connections",
		}),

		totalConnections: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "connections_total",
			Help:      "Total number of TUI connections since start",
		}),
	}

	return m
}

// GetGlobal returns the global metrics instance (creates if needed)
func GetGlobal() *Metrics {
	once.Do(func() {
		globalMetrics = New("ltt")
	})
	return globalMetrics
}

// RecordSpanReceived increments the received counter
func (m *Metrics) RecordSpanReceived(count int) {
	m.spansReceived.Add(float64(count))
}

// RecordSpanDropped increments the dropped counter
func (m *Metrics) RecordSpanDropped(count int) {
	m.spansDropped.Add(float64(count))
}

// RecordSpanExported increments the exported counter
func (m *Metrics) RecordSpanExported(count int) {
	m.spansExported.Add(float64(count))
}

// RecordSpanFailed increments the failed counter
func (m *Metrics) RecordSpanFailed(count int) {
	m.spansFailed.Add(float64(count))
}

// RecordExportDuration records how long an export took
func (m *Metrics) RecordExportDuration(duration time.Duration) {
	m.exportDuration.Observe(float64(duration.Microseconds()))
}

// RecordEncodeDuration records how long encoding took
func (m *Metrics) RecordEncodeDuration(duration time.Duration) {
	m.encodeDuration.Observe(float64(duration.Microseconds()))
}

// SetBufferUsage sets the current buffer usage
func (m *Metrics) SetBufferUsage(used, capacity int) {
	m.bufferUsage.Set(float64(used))
	m.bufferCapacity.Set(float64(capacity))
}

// SetGoroutines sets the current goroutine count
func (m *Metrics) SetGoroutines(count int) {
	m.goroutines.Set(float64(count))
}

// SetMemoryUsage sets the current memory usage
func (m *Metrics) SetMemoryUsage(bytes uint64) {
	m.memoryUsage.Set(float64(bytes))
}

// RecordConnection increments connection counters
func (m *Metrics) RecordConnection() {
	m.activeConnections.Inc()
	m.totalConnections.Inc()
}

// RecordDisconnection decrements active connections
func (m *Metrics) RecordDisconnection() {
	m.activeConnections.Dec()
}

// GetUptime returns how long the exporter has been running
func (m *Metrics) GetUptime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Since(m.startTime)
}
