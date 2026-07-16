package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// LogMessage represents a log record in wire format
type LogMessage struct {
	Timestamp         time.Time
	ObservedTimestamp time.Time
	TraceID           trace.TraceID
	SpanID            trace.SpanID
	TraceFlags        trace.TraceFlags
	SeverityNumber    int32
	SeverityText      string
	Body              string
	Attributes        map[string]interface{}
	Resource          map[string]interface{}
	InstrumentationScope *InstrumentationScope
}

// InstrumentationScope represents the instrumentation scope
type InstrumentationScope struct {
	Name    string
	Version string
}

// EncodeLogTo encodes a log message to a buffer with length prefix
func (l *LogMessage) EncodeLogTo(buf *bytes.Buffer) error {
	// Reset buffer for reuse
	buf.Reset()

	// Build payload
	payload := &bytes.Buffer{}

	// Timestamp (8 bytes)
	binary.Write(payload, binary.BigEndian, l.Timestamp.UnixNano())

	// ObservedTimestamp (8 bytes)
	binary.Write(payload, binary.BigEndian, l.ObservedTimestamp.UnixNano())

	// TraceID (16 bytes)
	payload.Write(l.TraceID[:])

	// SpanID (8 bytes)
	payload.Write(l.SpanID[:])

	// TraceFlags (1 byte)
	payload.WriteByte(byte(l.TraceFlags))

	// SeverityNumber (4 bytes)
	binary.Write(payload, binary.BigEndian, l.SeverityNumber)

	// SeverityText
	writeLogString(payload, l.SeverityText)

	// Body
	writeLogString(payload, l.Body)

	// Attributes
	writeLogAttributes(payload, l.Attributes)

	// Resource
	writeLogAttributes(payload, l.Resource)

	// InstrumentationScope
	if l.InstrumentationScope != nil {
		payload.WriteByte(1)
		writeLogString(payload, l.InstrumentationScope.Name)
		writeLogString(payload, l.InstrumentationScope.Version)
	} else {
		payload.WriteByte(0)
	}

	// Write length prefix + payload to output buffer
	binary.Write(buf, binary.BigEndian, uint32(payload.Len()))
	buf.Write(payload.Bytes())

	return nil
}

// DecodeLog decodes a log message from payload (without length prefix)
func DecodeLog(data []byte) (*LogMessage, error) {
	if len(data) < 45 {
		return nil, fmt.Errorf("log data too short: %d bytes", len(data))
	}

	msg := &LogMessage{}
	offset := 0

	// Timestamp
	timestamp := int64(binary.BigEndian.Uint64(data[offset : offset+8]))
	msg.Timestamp = time.Unix(0, timestamp)
	offset += 8

	// ObservedTimestamp
	observed := int64(binary.BigEndian.Uint64(data[offset : offset+8]))
	msg.ObservedTimestamp = time.Unix(0, observed)
	offset += 8

	// TraceID
	copy(msg.TraceID[:], data[offset:offset+16])
	offset += 16

	// SpanID
	copy(msg.SpanID[:], data[offset:offset+8])
	offset += 8

	// TraceFlags
	msg.TraceFlags = trace.TraceFlags(data[offset])
	offset++

	// SeverityNumber
	msg.SeverityNumber = int32(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4

	// SeverityText
	severityText, n, err := readLogString(data[offset:])
	if err != nil {
		return nil, err
	}
	msg.SeverityText = severityText
	offset += n

	// Body
	body, n, err := readLogString(data[offset:])
	if err != nil {
		return nil, err
	}
	msg.Body = body
	offset += n

	// Attributes
	attrs, n, err := readLogAttributes(data[offset:])
	if err != nil {
		return nil, err
	}
	msg.Attributes = attrs
	offset += n

	// Resource
	resource, n, err := readLogAttributes(data[offset:])
	if err != nil {
		return nil, err
	}
	msg.Resource = resource
	offset += n

	// InstrumentationScope
	if offset < len(data) && data[offset] == 1 {
		offset++
		scope := &InstrumentationScope{}

		name, n, err := readLogString(data[offset:])
		if err != nil {
			return nil, err
		}
		scope.Name = name
		offset += n

		version, n, err := readLogString(data[offset:])
		if err != nil {
			return nil, err
		}
		scope.Version = version
		offset += n

		msg.InstrumentationScope = scope
	}

	return msg, nil
}

// Helper functions for log serialization
func writeLogString(buf *bytes.Buffer, s string) {
	length := uint32(len(s))
	binary.Write(buf, binary.BigEndian, length)
	if length > 0 {
		buf.Write([]byte(s))
	}
}

func readLogString(data []byte) (string, int, error) {
	if len(data) < 4 {
		return "", 0, fmt.Errorf("data too short for string length")
	}

	length := binary.BigEndian.Uint32(data[0:4])
	if uint32(len(data)) < 4+length {
		return "", 0, fmt.Errorf("data too short for string content")
	}

	s := string(data[4 : 4+length])
	return s, int(4 + length), nil
}

func writeLogAttributes(buf *bytes.Buffer, attrs map[string]interface{}) {
	count := uint32(len(attrs))
	binary.Write(buf, binary.BigEndian, count)

	for key, value := range attrs {
		writeLogString(buf, key)
		writeLogValue(buf, value)
	}
}

func readLogAttributes(data []byte) (map[string]interface{}, int, error) {
	if len(data) < 4 {
		return nil, 0, fmt.Errorf("data too short for attributes count")
	}

	count := binary.BigEndian.Uint32(data[0:4])
	offset := 4

	attrs := make(map[string]interface{})

	for i := uint32(0); i < count; i++ {
		key, n, err := readLogString(data[offset:])
		if err != nil {
			return nil, 0, err
		}
		offset += n

		value, n, err := readLogValue(data[offset:])
		if err != nil {
			return nil, 0, err
		}
		offset += n

		attrs[key] = value
	}

	return attrs, offset, nil
}

func writeLogValue(buf *bytes.Buffer, v interface{}) {
	switch val := v.(type) {
	case string:
		buf.WriteByte(0)
		writeLogString(buf, val)
	case int64:
		buf.WriteByte(1)
		binary.Write(buf, binary.BigEndian, val)
	case float64:
		buf.WriteByte(2)
		binary.Write(buf, binary.BigEndian, val)
	case bool:
		buf.WriteByte(3)
		if val {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
	case int:
		buf.WriteByte(1)
		binary.Write(buf, binary.BigEndian, int64(val))
	default:
		// Fallback to string representation
		buf.WriteByte(0)
		writeLogString(buf, fmt.Sprintf("%v", val))
	}
}

func readLogValue(data []byte) (interface{}, int, error) {
	if len(data) < 1 {
		return nil, 0, fmt.Errorf("data too short for value type")
	}

	valueType := data[0]
	offset := 1

	switch valueType {
	case 0: // string
		s, n, err := readLogString(data[offset:])
		return s, offset + n, err
	case 1: // int64
		if len(data) < offset+8 {
			return nil, 0, fmt.Errorf("data too short for int64")
		}
		val := int64(binary.BigEndian.Uint64(data[offset : offset+8]))
		return val, offset + 8, nil
	case 2: // float64
		if len(data) < offset+8 {
			return nil, 0, fmt.Errorf("data too short for float64")
		}
		bits := binary.BigEndian.Uint64(data[offset : offset+8])
		val := math.Float64frombits(bits)
		return val, offset + 8, nil
	case 3: // bool
		if len(data) < offset+1 {
			return nil, 0, fmt.Errorf("data too short for bool")
		}
		val := data[offset] == 1
		return val, offset + 1, nil
	default:
		return nil, 0, fmt.Errorf("unknown value type: %d", valueType)
	}
}
