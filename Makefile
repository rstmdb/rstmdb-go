.PHONY: all build vet test test-race test-cover test-verbose clean fmt lint check examples

# Default target
all: check

# Build the library
build:
	go build ./...

# Run go vet
vet:
	go vet ./...

# Run tests
test:
	go test ./...

# Run tests with verbose output
test-verbose:
	go test -v ./...

# Run tests with race detector
test-race:
	go test -race ./...

# Run tests with coverage report
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo ""
	@echo "To view HTML report: go tool cover -html=coverage.out"

# Format all Go files
fmt:
	gofmt -s -w .

# Run staticcheck if installed, fall back to go vet
lint:
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not found, running go vet instead"; \
		go vet ./...; \
	fi

# Build all examples
examples:
	go build ./examples/...

# Full check: fmt, vet, build, test with race detector
check: fmt vet build test-race

# Clean build artifacts
clean:
	rm -f coverage.out
	go clean ./...
