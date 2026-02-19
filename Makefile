.PHONY: all build vet test test-race test-cover test-verbose clean fmt lint staticcheck check examples

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

# Run staticcheck (requires: go install honnef.co/go/tools/cmd/staticcheck@latest)
staticcheck:
	staticcheck ./...

# Run all linters: go vet + staticcheck
lint: vet staticcheck

# Build all examples
examples:
	go build ./examples/...

# Full check: fmt, lint, build, test with race detector
check: fmt lint build test-race

# Clean build artifacts
clean:
	rm -f coverage.out
	go clean ./...
