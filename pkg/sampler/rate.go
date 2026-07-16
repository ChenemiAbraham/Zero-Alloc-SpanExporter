package sampler

import (
	"sync"
	"time"

	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/protocol"
)

// RateLimitingSampler limits spans to a maximum per second
// Uses token bucket algorithm for smooth rate limiting
type RateLimitingSampler struct {
	rate     int           // max spans per second
	tokens   float64       // current tokens available
	maxToken float64       // max tokens (burst capacity)
	lastTime time.Time     // last refill time
	mu       sync.Mutex    // protect token bucket
}

// NewRateLimitingSampler creates a rate-limiting sampler
// rate is the maximum number of spans per second
func NewRateLimitingSampler(rate int) *RateLimitingSampler {
	if rate <= 0 {
		rate = 1000 // default to 1000 spans/sec
	}

	return &RateLimitingSampler{
		rate:     rate,
		tokens:   float64(rate),
		maxToken: float64(rate),
		lastTime: time.Now(),
	}
}

func (r *RateLimitingSampler) ShouldSample(span *protocol.SpanMessage) Decision {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	r.lastTime = now

	// Add tokens for elapsed time
	r.tokens += elapsed * float64(r.rate)
	if r.tokens > r.maxToken {
		r.tokens = r.maxToken
	}

	// Try to consume a token
	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		return Sample
	}

	return Drop
}

func (r *RateLimitingSampler) Name() string {
	return "rate"
}

// GetRate returns the current rate limit
func (r *RateLimitingSampler) GetRate() int {
	return r.rate
}

// GetAvailableTokens returns current token count (for monitoring)
func (r *RateLimitingSampler) GetAvailableTokens() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tokens
}
