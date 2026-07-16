package health

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// Status represents the health status of the system
type Status struct {
	Healthy   bool              `json:"healthy"`
	Version   string            `json:"version"`
	Uptime    string            `json:"uptime"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]Check  `json:"checks"`
	System    SystemInfo        `json:"system"`
}

// Check represents a single health check
type Check struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// SystemInfo contains system-level information
type SystemInfo struct {
	Goroutines   int    `json:"goroutines"`
	MemoryMB     uint64 `json:"memory_mb"`
	GOOS         string `json:"goos"`
	GOARCH       string `json:"goarch"`
	GoVersion    string `json:"go_version"`
}

// Checker is a function that performs a health check
type Checker func() Check

// Handler manages health checks
type Handler struct {
	version   string
	startTime time.Time
	checkers  map[string]Checker
	mu        sync.RWMutex
}

// New creates a new health check handler
func New(version string) *Handler {
	return &Handler{
		version:   version,
		startTime: time.Now(),
		checkers:  make(map[string]Checker),
	}
}

// Register adds a new health checker
func (h *Handler) Register(name string, checker Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[name] = checker
}

// Check runs all health checks and returns the status
func (h *Handler) Check() Status {
	h.mu.RLock()
	checkers := make(map[string]Checker, len(h.checkers))
	for k, v := range h.checkers {
		checkers[k] = v
	}
	h.mu.RUnlock()

	// Run all checks
	checks := make(map[string]Check)
	allHealthy := true

	for name, checker := range checkers {
		check := checker()
		checks[name] = check
		if !check.Healthy {
			allHealthy = false
		}
	}

	// Gather system info
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return Status{
		Healthy:   allHealthy,
		Version:   h.version,
		Uptime:    time.Since(h.startTime).String(),
		Timestamp: time.Now(),
		Checks:    checks,
		System: SystemInfo{
			Goroutines: runtime.NumGoroutine(),
			MemoryMB:   m.Alloc / 1024 / 1024,
			GOOS:       runtime.GOOS,
			GOARCH:     runtime.GOARCH,
			GoVersion:  runtime.Version(),
		},
	}
}

// ServeHTTP implements http.Handler for health endpoint
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status := h.Check()

	w.Header().Set("Content-Type", "application/json")
	if !status.Healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}

// LivenessHandler returns a simple liveness check (always healthy if running)
func LivenessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"alive": true})
}

// ReadinessHandler returns readiness based on health checks
func (h *Handler) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	status := h.Check()

	w.Header().Set("Content-Type", "application/json")
	if !status.Healthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]bool{"ready": false})
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"ready": true})
}

// Common health checkers

// SocketHealthChecker checks if socket is operational
func SocketHealthChecker(isConnected func() bool) Checker {
	return func() Check {
		if isConnected() {
			return Check{
				Healthy: true,
				Message: "Socket operational",
			}
		}
		return Check{
			Healthy: false,
			Message: "Socket not connected",
		}
	}
}

// BufferHealthChecker checks if buffer usage is healthy
func BufferHealthChecker(getUsage func() float64, threshold float64) Checker {
	return func() Check {
		usage := getUsage()
		if usage < threshold {
			return Check{
				Healthy: true,
				Message: "Buffer usage normal",
			}
		}
		return Check{
			Healthy: false,
			Message: "Buffer usage high",
		}
	}
}

// MemoryHealthChecker checks if memory usage is healthy
func MemoryHealthChecker(maxMB uint64) Checker {
	return func() Check {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		currentMB := m.Alloc / 1024 / 1024

		if currentMB < maxMB {
			return Check{
				Healthy: true,
				Message: "Memory usage normal",
			}
		}
		return Check{
			Healthy: false,
			Message: "Memory usage high",
		}
	}
}
