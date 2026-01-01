# Justfile for Huma Playground
# https://github.com/casey/just

set dotenv-load := false

# Default recipe - show available commands
default:
    @just --list

# Build the application
build:
    go build -v ./...

# Run the server
run:
    go run ./cmd/server

# Run the server with custom port
run-port port="8080":
    PORT={{port}} go run ./cmd/server

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-verbose:
    go test -v ./...

# Run tests with coverage
test-coverage:
    go test -v -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...

# Generate coverage report
coverage: test-coverage
    go tool cover -func=coverage.out
    go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
    golangci-lint run ./...

# Apply formatters (gci, gofumpt, golines)
fmt:
    golangci-lint fmt ./...

# Run linter and apply formatters
fix:
    golangci-lint run --fix ./...

# Download dependencies
deps:
    go mod download

# Tidy dependencies
tidy:
    go mod tidy

# Update dependencies
update:
    go get -u -t ./...
    go mod tidy

# Check for vulnerabilities
vuln:
    govulncheck ./...

# Full check: build, test, lint
check: build test lint

# Clean build artifacts and coverage files
clean:
    rm -f coverage.out coverage.html
