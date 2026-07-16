package sampler

import (
	"sync"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
)

// Decision represents whether a span should be sampled
type Decision int

const (
	// Drop means the span should not be exported
	Drop Decision = iota
	// Sample means the span should be exported
	Sample
)

// Sampler makes sampling decisions for spans
type Sampler interface {
	// ShouldSample decides whether a span should be sampled
	ShouldSample(span *protocol.SpanMessage) Decision

	// Name returns the sampler name for metrics/logging
	Name() string
}

// Config holds sampler configuration
type Config struct {
	// Type specifies which sampler to use
	// Options: "probability", "rate", "tail", "adaptive", "always", "never"
	Type string

	// Probability is the sampling rate (0.0 to 1.0) for probability sampler
	// Example: 0.1 = 10% of spans
	Probability float64

	// Rate is the max spans per second for rate limiting sampler
	Rate int

	// TailConfig holds tail sampling configuration
	Tail TailConfig

	// AdaptiveConfig holds adaptive sampling configuration
	Adaptive AdaptiveConfig
}

// TailConfig configures tail sampling
type TailConfig struct {
	// SampleErrors keeps all spans with errors
	SampleErrors bool

	// SlowThreshold keeps spans slower than this duration
	SlowThreshold time.Duration

	// BaseProbability is the sampling rate for non-tail spans
	BaseProbability float64
}

// AdaptiveConfig configures adaptive sampling
type AdaptiveConfig struct {
	// TargetSpansPerSecond is the desired throughput
	TargetSpansPerSecond int

	// AdjustInterval is how often to recalculate sampling rate
	AdjustInterval time.Duration

	// MinProbability is the minimum sampling rate (never go below)
	MinProbability float64

	// MaxProbability is the maximum sampling rate (never go above)
	MaxProbability float64
}

// NewSampler creates a sampler from config
func NewSampler(config Config) Sampler {
	switch config.Type {
	case "probability":
		return NewProbabilitySampler(config.Probability)
	case "rate":
		return NewRateLimitingSampler(config.Rate)
	case "tail":
		return NewTailSampler(config.Tail)
	case "adaptive":
		return NewAdaptiveSampler(config.Adaptive)
	case "always":
		return AlwaysSampler{}
	case "never":
		return NeverSampler{}
	default:
		// Default to always sampling
		return AlwaysSampler{}
	}
}

// ChainSampler combines multiple samplers with AND logic
// All samplers must agree to sample for the span to be kept
type ChainSampler struct {
	samplers []Sampler
}

// NewChainSampler creates a chained sampler
func NewChainSampler(samplers ...Sampler) *ChainSampler {
	return &ChainSampler{samplers: samplers}
}

func (c *ChainSampler) ShouldSample(span *protocol.SpanMessage) Decision {
	for _, sampler := range c.samplers {
		if sampler.ShouldSample(span) == Drop {
			return Drop
		}
	}
	return Sample
}

func (c *ChainSampler) Name() string {
	return "chain"
}

// AlwaysSampler always samples (for testing)
type AlwaysSampler struct{}

func (AlwaysSampler) ShouldSample(*protocol.SpanMessage) Decision {
	return Sample
}

func (AlwaysSampler) Name() string {
	return "always"
}

// NeverSampler never samples (for testing)
type NeverSampler struct{}

func (NeverSampler) ShouldSample(*protocol.SpanMessage) Decision {
	return Drop
}

func (NeverSampler) Name() string {
	return "never"
}

// Stats tracks sampling statistics
type Stats struct {
	Sampled  uint64
	Rejected uint64
	mu       sync.RWMutex
}

func (s *Stats) RecordSampled() {
	s.mu.Lock()
	s.Sampled++
	s.mu.Unlock()
}

func (s *Stats) RecordRejected() {
	s.mu.Lock()
	s.Rejected++
	s.mu.Unlock()
}

func (s *Stats) Get() (sampled, rejected uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Sampled, s.Rejected
}

func (s *Stats) SamplingRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := s.Sampled + s.Rejected
	if total == 0 {
		return 0
	}
	return float64(s.Sampled) / float64(total)
}
