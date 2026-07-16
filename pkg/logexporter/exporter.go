package logexporter

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/storage"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/log"
)

// Exporter exports log records to local storage
type Exporter struct {
	store *storage.Store

	// Stats
	exportedLogs uint64
	droppedLogs  uint64

	// State
	mu     sync.Mutex
	closed bool
}

// Config holds log exporter configuration
type Config struct {
	// Storage backend for logs
	Storage *storage.Store
}

// New creates a new log exporter
func New(config Config) (*Exporter, error) {
	if config.Storage == nil {
		return nil, fmt.Errorf("storage is required for log exporter")
	}

	return &Exporter{
		store: config.Storage,
	}, nil
}

// Export exports log records
func (e *Exporter) Export(ctx context.Context, records []log.Record) error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	for _, record := range records {
		// Convert OTEL log record to our internal format
		logMsg := convertRecord(record)

		// Store the log
		if err := e.store.StoreLog(logMsg); err != nil {
			atomic.AddUint64(&e.droppedLogs, 1)
			continue
		}

		atomic.AddUint64(&e.exportedLogs, 1)
	}

	return nil
}

// ForceFlush forces pending logs to be exported
func (e *Exporter) ForceFlush(ctx context.Context) error {
	// No buffering in this implementation, so nothing to flush
	return nil
}

// Shutdown shuts down the exporter
func (e *Exporter) Shutdown(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}

	e.closed = true
	return nil
}

// GetStats returns exporter statistics
func (e *Exporter) GetStats() Stats {
	return Stats{
		ExportedLogs: atomic.LoadUint64(&e.exportedLogs),
		DroppedLogs:  atomic.LoadUint64(&e.droppedLogs),
	}
}

// Stats holds log exporter statistics
type Stats struct {
	ExportedLogs uint64
	DroppedLogs  uint64
}

// convertRecord converts an OTEL log record to our internal format
func convertRecord(record log.Record) *protocol.LogMessage {
	msg := &protocol.LogMessage{
		Timestamp:         record.Timestamp(),
		ObservedTimestamp: record.ObservedTimestamp(),
		TraceID:           record.TraceID(),
		SpanID:            record.SpanID(),
		TraceFlags:        record.TraceFlags(),
		SeverityNumber:    int32(record.Severity()),
		SeverityText:      record.SeverityText(),
		Body:              record.Body().AsString(),
		Attributes:        make(map[string]interface{}),
	}

	// Extract attributes
	record.WalkAttributes(func(kv otellog.KeyValue) bool {
		msg.Attributes[kv.Key] = convertLogValue(kv.Value)
		return true
	})

	// Extract resource attributes
	if resource := record.Resource(); resource != nil {
		msg.Resource = make(map[string]interface{})
		for _, attr := range resource.Attributes() {
			msg.Resource[string(attr.Key)] = attr.Value.AsInterface()
		}
	}

	// Extract instrumentation scope
	scope := record.InstrumentationScope()
	msg.InstrumentationScope = &protocol.InstrumentationScope{
		Name:    scope.Name,
		Version: scope.Version,
	}

	return msg
}

// convertLogValue converts an OTEL log Value to interface{}
func convertLogValue(v otellog.Value) interface{} {
	switch v.Kind() {
	case otellog.KindString:
		return v.AsString()
	case otellog.KindInt64:
		return v.AsInt64()
	case otellog.KindFloat64:
		return v.AsFloat64()
	case otellog.KindBool:
		return v.AsBool()
	case otellog.KindBytes:
		return string(v.AsBytes())
	case otellog.KindSlice:
		slice := v.AsSlice()
		result := make([]interface{}, len(slice))
		for i, item := range slice {
			result[i] = convertLogValue(item)
		}
		return result
	case otellog.KindMap:
		kvs := v.AsMap()
		result := make(map[string]interface{})
		for _, kv := range kvs {
			result[kv.Key] = convertLogValue(kv.Value)
		}
		return result
	default:
		return v.AsString()
	}
}
