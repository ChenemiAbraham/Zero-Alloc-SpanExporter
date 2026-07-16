package sampler

import (
	"hash/fnv"
	"math"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
)

// ProbabilitySampler samples a fixed percentage of spans
// Uses deterministic hashing based on trace ID for consistency
type ProbabilitySampler struct {
	probability float64
	threshold   uint64
}

// NewProbabilitySampler creates a probability-based sampler
// probability should be between 0.0 and 1.0
// Example: 0.1 = 10% of spans are sampled
func NewProbabilitySampler(probability float64) *ProbabilitySampler {
	if probability < 0 {
		probability = 0
	}
	if probability > 1 {
		probability = 1
	}

	// Calculate threshold for deterministic sampling
	// Use full uint64 range for precision
	threshold := uint64(probability * float64(math.MaxUint64))

	return &ProbabilitySampler{
		probability: probability,
		threshold:   threshold,
	}
}

func (p *ProbabilitySampler) ShouldSample(span *protocol.SpanMessage) Decision {
	// Use trace ID for consistent sampling across all spans in a trace
	// This ensures we either sample ALL spans in a trace or NONE
	hash := hashTraceID(span.TraceID)

	if hash <= p.threshold {
		return Sample
	}
	return Drop
}

func (p *ProbabilitySampler) Name() string {
	return "probability"
}

// GetProbability returns the current sampling probability
func (p *ProbabilitySampler) GetProbability() float64 {
	return p.probability
}

// hashTraceID creates a deterministic hash from trace ID
func hashTraceID(traceID [16]byte) uint64 {
	h := fnv.New64a()
	h.Write(traceID[:])
	return h.Sum64()
}
