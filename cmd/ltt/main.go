package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/health"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/metrics"
	"github.com/ChenemiAbraham/Zero-Alloc-SpanExporter/pkg/viewer"
)

const version = "0.0.1"

func main() {
	// Parse command line flags
	var (
		socketPath  = flag.String("socket", "127.0.0.1:9090", "Socket path or address to connect to")
		metricsPort = flag.Int("metrics-port", 2112, "Port for Prometheus metrics HTTP server")
		noMetrics   = flag.Bool("no-metrics", false, "Disable metrics server")
		showVersion = flag.Bool("version", false, "Show version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Local Trace Tap (LTT) v%s\n", version)
		return
	}

	// Override from environment variable if set
	if path := os.Getenv("LTT_SOCKET"); path != "" {
		*socketPath = path
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start metrics server if enabled
	var metricsServer *metrics.Server
	if !*noMetrics {
		metricsServer = startMetricsServer(*metricsPort)
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			metricsServer.Shutdown(shutdownCtx)
		}()
	}

	// Create model
	model := viewer.NewModel(*socketPath)

	// Create program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Set program reference in model for span notifications
	model.SetProgram(p)

	// Run TUI in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if _, err := p.Run(); err != nil {
			errChan <- err
		}
		close(errChan)
	}()

	// Wait for either TUI exit or interrupt signal
	select {
	case err := <-errChan:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
			os.Exit(1)
		}
	case <-sigChan:
		p.Quit()
	}
}

func startMetricsServer(port int) *metrics.Server {
	config := metrics.ServerConfig{
		Port:         port,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	server := metrics.NewServer(config)

	// Create health check handler
	healthHandler := health.New(version)

	// Register health checks
	healthHandler.Register("memory", health.MemoryHealthChecker(500)) // 500MB max

	// Register health endpoints
	server.RegisterHealthHandler("/health", healthHandler)
	server.RegisterHealthHandler("/ready", http.HandlerFunc(healthHandler.ReadinessHandler))
	server.RegisterHealthHandler("/live", http.HandlerFunc(health.LivenessHandler))

	// Start server in background
	server.StartAsync()

	fmt.Printf("✓ Metrics server started on http://localhost:%d/metrics\n", port)
	fmt.Printf("✓ Health check available at http://localhost:%d/health\n", port)

	return server
}
