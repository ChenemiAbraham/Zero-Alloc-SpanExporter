package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	// Port to listen on (default: 2112)
	Port int

	// ReadTimeout for HTTP requests
	ReadTimeout time.Duration

	// WriteTimeout for HTTP responses
	WriteTimeout time.Duration
}

// DefaultServerConfig returns sensible defaults
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:         2112,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}

// Server wraps an HTTP server for exposing metrics
type Server struct {
	config ServerConfig
	server *http.Server
	mux    *http.ServeMux
}

// NewServer creates a new metrics HTTP server
func NewServer(config ServerConfig) *Server {
	if config.Port == 0 {
		config = DefaultServerConfig()
	}

	mux := http.NewServeMux()

	s := &Server{
		config: config,
		mux:    mux,
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", config.Port),
			Handler:      mux,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
		},
	}

	// Register default endpoints
	s.registerEndpoints()

	return s
}

// registerEndpoints sets up the HTTP routes
func (s *Server) registerEndpoints() {
	// Prometheus metrics endpoint
	s.mux.Handle("/metrics", promhttp.Handler())

	// Basic info endpoint
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html>
<head><title>LTT Metrics</title></head>
<body>
<h1>Local Trace Tap Metrics Server</h1>
<p><a href="/metrics">Prometheus Metrics</a></p>
<p><a href="/health">Health Check</a></p>
<p><a href="/ready">Readiness Check</a></p>
</body>
</html>`)
	})
}

// RegisterHealthHandler adds a health check endpoint
func (s *Server) RegisterHealthHandler(path string, handler http.Handler) {
	s.mux.Handle(path, handler)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// StartAsync starts the HTTP server in a background goroutine
func (s *Server) StartAsync() {
	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			// Log error but don't crash
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Addr returns the server address
func (s *Server) Addr() string {
	return s.server.Addr
}
