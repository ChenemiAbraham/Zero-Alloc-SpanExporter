package sampler

import (
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
	"go.opentelemetry.io/otel/codes"
)

// TailSampler implements tail-based sampling
// Keeps important traces (errors, slow) and samples others at base rate
type TailSampler struct {
	sampleErrors    bool
	slowThreshold   time.Duration
	baseSampler     *ProbabilitySampler
}

// NewTailSampler creates a tail sampler
func NewTailSampler(config TailConfig) *TailSampler {
	// Default configuration
	if config.SlowThreshold == 0 {
		config.SlowThreshold = 1 * time.Second
	}
	if config.BaseProbability == 0 {
		config.BaseProbability = 0.01 // 1% base sampling
	}

	return &TailSampler{
		sampleErrors:  config.SampleErrors,
		slowThreshold: config.SlowThreshold,
		baseSampler:   NewProbabilitySampler(config.BaseProbability),
	}
}

func (t *TailSampler) ShouldSample(span *protocol.SpanMessage) Decision {
	// Always sample errors
	if t.sampleErrors && span.StatusCode == codes.Error {
		return Sample
	}

	// Always sample slow spans
	duration := span.EndTime.Sub(span.StartTime)
	if duration >= t.slowThreshold {
		return Sample
	}

	// For normal spans, use base probability
	return t.baseSampler.ShouldSample(span)
}

func (t *TailSampler) Name() string {
	return "tail"
}

// GetConfig returns current configuration
func (t *TailSampler) GetConfig() TailConfig {
	return TailConfig{
		SampleErrors:    t.sampleErrors,
		SlowThreshold:   t.slowThreshold,
		BaseProbability: t.baseSampler.GetProbability(),
	}
}
