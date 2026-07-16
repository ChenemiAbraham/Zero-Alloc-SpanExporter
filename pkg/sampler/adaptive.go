package sampler

import (
	"sync"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
)

// AdaptiveSampler automatically adjusts sampling rate to hit target throughput
// Uses feedback control to maintain desired spans per second
type AdaptiveSampler struct {
	targetRate     int
	adjustInterval time.Duration
	minProb        float64
	maxProb        float64

	mu             sync.RWMutex
	currentProb    float64
	probSampler    *ProbabilitySampler
	spansReceived  int
	lastAdjustTime time.Time
}

// NewAdaptiveSampler creates an adaptive sampler
func NewAdaptiveSampler(config AdaptiveConfig) *AdaptiveSampler {
	// Default configuration
	if config.TargetSpansPerSecond == 0 {
		config.TargetSpansPerSecond = 1000
	}
	if config.AdjustInterval == 0 {
		config.AdjustInterval = 10 * time.Second
	}
	if config.MinProbability == 0 {
		config.MinProbability = 0.001 // 0.1%
	}
	if config.MaxProbability == 0 {
		config.MaxProbability = 1.0 // 100%
	}

	initialProb := 0.1 // Start at 10%

	return &AdaptiveSampler{
		targetRate:     config.TargetSpansPerSecond,
		adjustInterval: config.AdjustInterval,
		minProb:        config.MinProbability,
		maxProb:        config.MaxProbability,
		currentProb:    initialProb,
		probSampler:    NewProbabilitySampler(initialProb),
		lastAdjustTime: time.Now(),
	}
}

func (a *AdaptiveSampler) ShouldSample(span *protocol.SpanMessage) Decision {
	a.mu.Lock()
	a.spansReceived++

	// Check if it's time to adjust
	now := time.Now()
	if now.Sub(a.lastAdjustTime) >= a.adjustInterval {
		a.adjust(now)
	}
	a.mu.Unlock()

	// Delegate to probability sampler
	a.mu.RLock()
	sampler := a.probSampler
	a.mu.RUnlock()

	return sampler.ShouldSample(span)
}

func (a *AdaptiveSampler) adjust(now time.Time) {
	elapsed := now.Sub(a.lastAdjustTime).Seconds()
	a.lastAdjustTime = now

	// Calculate actual rate
	actualRate := float64(a.spansReceived) / elapsed
	a.spansReceived = 0

	// Calculate desired sampling rate adjustment
	// If actual > target, decrease probability
	// If actual < target, increase probability
	ratio := float64(a.targetRate) / actualRate

	// Adjust with dampening to avoid oscillation
	newProb := a.currentProb * ratio * 0.5 // 0.5 = dampening factor

	// Clamp to min/max
	if newProb < a.minProb {
		newProb = a.minProb
	}
	if newProb > a.maxProb {
		newProb = a.maxProb
	}

	a.currentProb = newProb
	a.probSampler = NewProbabilitySampler(newProb)
}

func (a *AdaptiveSampler) Name() string {
	return "adaptive"
}

// GetCurrentProbability returns the current sampling probability
func (a *AdaptiveSampler) GetCurrentProbability() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.currentProb
}

// GetTargetRate returns the target spans per second
func (a *AdaptiveSampler) GetTargetRate() int {
	return a.targetRate
}
