package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Wire protocol for efficient span serialization
// Format: [4-byte length][payload]

// SpanMessage represents a serialized span over the wire
type SpanMessage struct {
	TraceID    [16]byte
	SpanID     [8]byte
	ParentID   [8]byte
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	StatusCode codes.Code
	StatusMsg  string
	Attributes map[string]interface{}
	Events     []SpanEvent
}

// SpanEvent represents a span event
type SpanEvent struct {
	Name       string
	Timestamp  time.Time
	Attributes map[string]interface{}
}

// EncodeTo writes the span message to the writer with length prefix
// Wire format: [4-byte length][payload]
// Payload: [trace_id:16][span_id:8][parent_id:8][name_len:2][name][start:8][end:8][status:1][status_msg_len:2][status_msg][attrs_count:2][attrs...][events_count:2][events...]
func (s *SpanMessage) EncodeTo(w io.Writer) error {
	// Build payload in a buffer
	buf := &bytes.Buffer{}

	// Write IDs (fixed size)
	buf.Write(s.TraceID[:])
	buf.Write(s.SpanID[:])
	buf.Write(s.ParentID[:])

	// Write name (length-prefixed string)
	if err := writeString(buf, s.Name); err != nil {
		return err
	}

	// Write timestamps (Unix nanos as int64)
	if err := binary.Write(buf, binary.LittleEndian, s.StartTime.UnixNano()); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, s.EndTime.UnixNano()); err != nil {
		return err
	}

	// Write status code (1 byte)
	if err := buf.WriteByte(byte(s.StatusCode)); err != nil {
		return err
	}

	// Write status message
	if err := writeString(buf, s.StatusMsg); err != nil {
		return err
	}

	// Write attributes count
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(s.Attributes))); err != nil {
		return err
	}

	// Write attributes
	for key, value := range s.Attributes {
		if err := writeString(buf, key); err != nil {
			return err
		}
		if err := writeValue(buf, value); err != nil {
			return err
		}
	}

	// Write events count
	if err := binary.Write(buf, binary.LittleEndian, uint16(len(s.Events))); err != nil {
		return err
	}

	// Write events
	for _, event := range s.Events {
		if err := writeString(buf, event.Name); err != nil {
			return err
		}
		if err := binary.Write(buf, binary.LittleEndian, event.Timestamp.UnixNano()); err != nil {
			return err
		}

		// Event attributes
		if err := binary.Write(buf, binary.LittleEndian, uint16(len(event.Attributes))); err != nil {
			return err
		}
		for key, value := range event.Attributes {
			if err := writeString(buf, key); err != nil {
				return err
			}
			if err := writeValue(buf, value); err != nil {
				return err
			}
		}
	}

	// Write length prefix + payload
	payload := buf.Bytes()
	if err := binary.Write(w, binary.LittleEndian, uint32(len(payload))); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// DecodeFrom reads a span message from the reader
// If the reader already contains just the payload (no length prefix), use DecodePayload instead
func DecodeFrom(r io.Reader) (*SpanMessage, error) {
	// Read 4-byte length prefix
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, err
	}

	// Read payload
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	return DecodePayload(payload)
}

// DecodePayload decodes a span message from raw payload bytes (without length prefix)
func DecodePayload(payload []byte) (*SpanMessage, error) {
	buf := bytes.NewReader(payload)
	msg := &SpanMessage{
		Attributes: make(map[string]interface{}),
		Events:     make([]SpanEvent, 0),
	}

	// Read IDs (fixed size)
	if _, err := io.ReadFull(buf, msg.TraceID[:]); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(buf, msg.SpanID[:]); err != nil {
		return nil, err
	}
	if _, err := io.ReadFull(buf, msg.ParentID[:]); err != nil {
		return nil, err
	}

	// Read name
	name, err := readString(buf)
	if err != nil {
		return nil, err
	}
	msg.Name = name

	// Read timestamps
	var startNanos, endNanos int64
	if err := binary.Read(buf, binary.LittleEndian, &startNanos); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &endNanos); err != nil {
		return nil, err
	}
	msg.StartTime = time.Unix(0, startNanos)
	msg.EndTime = time.Unix(0, endNanos)

	// Read status code
	statusByte, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	msg.StatusCode = codes.Code(statusByte)

	// Read status message
	msg.StatusMsg, err = readString(buf)
	if err != nil {
		return nil, err
	}

	// Read attributes
	var attrCount uint16
	if err := binary.Read(buf, binary.LittleEndian, &attrCount); err != nil {
		return nil, err
	}
	for i := 0; i < int(attrCount); i++ {
		key, err := readString(buf)
		if err != nil {
			return nil, err
		}
		value, err := readValue(buf)
		if err != nil {
			return nil, err
		}
		msg.Attributes[key] = value
	}

	// Read events
	var eventCount uint16
	if err := binary.Read(buf, binary.LittleEndian, &eventCount); err != nil {
		return nil, err
	}
	for i := 0; i < int(eventCount); i++ {
		event := SpanEvent{
			Attributes: make(map[string]interface{}),
		}

		event.Name, err = readString(buf)
		if err != nil {
			return nil, err
		}

		var timestampNanos int64
		if err := binary.Read(buf, binary.LittleEndian, &timestampNanos); err != nil {
			return nil, err
		}
		event.Timestamp = time.Unix(0, timestampNanos)

		var eventAttrCount uint16
		if err := binary.Read(buf, binary.LittleEndian, &eventAttrCount); err != nil {
			return nil, err
		}
		for j := 0; j < int(eventAttrCount); j++ {
			key, err := readString(buf)
			if err != nil {
				return nil, err
			}
			value, err := readValue(buf)
			if err != nil {
				return nil, err
			}
			event.Attributes[key] = value
		}

		msg.Events = append(msg.Events, event)
	}

	return msg, nil
}

// Helper functions for encoding/decoding

// writeString writes a length-prefixed string
func writeString(w io.Writer, s string) error {
	if len(s) > 65535 {
		return fmt.Errorf("string too long: %d", len(s))
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(len(s))); err != nil {
		return err
	}
	_, err := w.Write([]byte(s))
	return err
}

// readString reads a length-prefixed string
func readString(r io.Reader) (string, error) {
	var length uint16
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", err
	}
	return string(data), nil
}

// writeValue writes a typed value
// Format: [type:1][value]
func writeValue(w io.Writer, v interface{}) error {
	switch val := v.(type) {
	case string:
		w.Write([]byte{0}) // type: string
		return writeString(w, val)
	case int64:
		w.Write([]byte{1}) // type: int64
		return binary.Write(w, binary.LittleEndian, val)
	case float64:
		w.Write([]byte{2}) // type: float64
		return binary.Write(w, binary.LittleEndian, val)
	case bool:
		w.Write([]byte{3}) // type: bool
		if val {
			_, err := w.Write([]byte{1})
			return err
		}
		_, err := w.Write([]byte{0})
		return err
	default:
		// Fallback: convert to string
		w.Write([]byte{0})
		return writeString(w, fmt.Sprintf("%v", val))
	}
}

// readValue reads a typed value
func readValue(r io.Reader) (interface{}, error) {
	typeByte := make([]byte, 1)
	if _, err := r.Read(typeByte); err != nil {
		return nil, err
	}

	switch typeByte[0] {
	case 0: // string
		return readString(r)
	case 1: // int64
		var val int64
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return val, nil
	case 2: // float64
		var val float64
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		return val, nil
	case 3: // bool
		b := make([]byte, 1)
		if _, err := r.Read(b); err != nil {
			return nil, err
		}
		return b[0] == 1, nil
	default:
		return nil, fmt.Errorf("unknown value type: %d", typeByte[0])
	}
}

// FromReadOnlySpan converts an OTEL span to our wire format
func FromReadOnlySpan(span sdktrace.ReadOnlySpan) *SpanMessage {
	msg := &SpanMessage{
		Name:       span.Name(),
		StartTime:  span.StartTime(),
		EndTime:    span.EndTime(),
		StatusCode: span.Status().Code,
		StatusMsg:  span.Status().Description,
		Attributes: make(map[string]interface{}),
		Events:     make([]SpanEvent, 0),
	}

	// Copy IDs
	traceID := span.SpanContext().TraceID()
	spanID := span.SpanContext().SpanID()
	copy(msg.TraceID[:], traceID[:])
	copy(msg.SpanID[:], spanID[:])
	if span.Parent().IsValid() {
		parentSpanID := span.Parent().SpanID()
		copy(msg.ParentID[:], parentSpanID[:])
	}

	// Convert attributes
	for _, attr := range span.Attributes() {
		msg.Attributes[string(attr.Key)] = attributeValue(attr.Value)
	}

	// Convert events
	for _, event := range span.Events() {
		evt := SpanEvent{
			Name:       event.Name,
			Timestamp:  event.Time,
			Attributes: make(map[string]interface{}),
		}
		for _, attr := range event.Attributes {
			evt.Attributes[string(attr.Key)] = attributeValue(attr.Value)
		}
		msg.Events = append(msg.Events, evt)
	}

	return msg
}

// attributeValue converts OTEL attribute value to interface{}
func attributeValue(v attribute.Value) interface{} {
	switch v.Type() {
	case attribute.BOOL:
		return v.AsBool()
	case attribute.INT64:
		return v.AsInt64()
	case attribute.FLOAT64:
		return v.AsFloat64()
	case attribute.STRING:
		return v.AsString()
	default:
		return v.AsString()
	}
}

// Buffer pool for efficient serialization
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// NewBuffer gets a buffer from the pool
func NewBuffer() *bytes.Buffer {
	return bufferPool.Get().(*bytes.Buffer)
}

// PutBuffer returns a buffer to the pool
func PutBuffer(buf *bytes.Buffer) {
	buf.Reset()
	bufferPool.Put(buf)
}
