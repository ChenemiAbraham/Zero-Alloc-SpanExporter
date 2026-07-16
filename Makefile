.PHONY: all build test bench clean fmt lint install example

all: fmt lint test build

# Build TUI viewer
build:
	@echo "Building ltt..."
	@go build -o ltt ./cmd/ltt

# Build with optimizations
build-release:
	@echo "Building optimized release..."
	@go build -ldflags="-s -w" -o ltt ./cmd/ltt

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@go test -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -cover ./...
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./pkg/exporter
	@go test -bench=. -benchmem ./internal/ringbuf

# Run CPU profiling
profile-cpu:
	@echo "Running CPU profile..."
	@go test -cpuprofile=cpu.prof -bench=. ./pkg/exporter
	@go tool pprof -http=:8080 cpu.prof

# Run memory profiling
profile-mem:
	@echo "Running memory profile..."
	@go test -memprofile=mem.prof -bench=. ./pkg/exporter
	@go tool pprof -http=:8080 mem.prof

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint
lint:
	@echo "Running linters..."
	@go vet ./...
	@test -z "$$(gofmt -s -l . | tee /dev/stderr)"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Run example application
example:
	@echo "Running example app..."
	@go run ./examples/simple/main.go

# Run TUI viewer
run:
	@echo "Starting TUI viewer..."
	@./ltt

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f ltt ltt.exe
	@rm -f *.prof *.out *.html
	@rm -f /tmp/ltt*.sock
	@go clean

# Install ltt to GOPATH/bin
install:
	@echo "Installing ltt..."
	@go install ./cmd/ltt

# Help
help:
	@echo "Available targets:"
	@echo "  make build         - Build the TUI viewer"
	@echo "  make test          - Run tests"
	@echo "  make test-race     - Run tests with race detector"
	@echo "  make test-coverage - Run tests with coverage report"
	@echo "  make bench         - Run benchmarks"
	@echo "  make profile-cpu   - Profile CPU usage"
	@echo "  make profile-mem   - Profile memory usage"
	@echo "  make fmt           - Format code"
	@echo "  make lint          - Run linters"
	@echo "  make deps          - Install dependencies"
	@echo "  make example       - Run example application"
	@echo "  make run           - Run TUI viewer"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make install       - Install to GOPATH/bin"
