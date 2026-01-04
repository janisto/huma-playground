# Justfile for Huma Playground
# https://github.com/casey/just

set dotenv-load := true

PORT := env("PORT", "8080")

# Default recipe - show available commands
default:
    @just --list

# Build the application
[group('build')]
build:
    go build -v ./...

# Clean build artifacts and coverage files
[group('build')]
clean:
    rm -f coverage.out coverage.html

# Run the server
[group('run')]
run:
    go run ./cmd/server

# Run the server with custom port
[group('run')]
run-port port=PORT:
    PORT={{port}} go run ./cmd/server

# Start Firebase emulators for E2E tests
[group('test')]
emulators:
    firebase emulators:start --only auth,firestore

# Run all tests
[group('test')]
test:
    go test ./...

# Run tests with verbose output
[group('test')]
test-verbose:
    go test -v ./...

# Run tests with coverage
[group('test')]
test-coverage:
    go test -v -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...

# Generate coverage report
[group('test')]
coverage: test-coverage
    go tool cover -func=coverage.out
    go tool cover -html=coverage.out -o coverage.html

# Run linter
[group('lint')]
lint:
    golangci-lint run ./...

# Apply formatters (gci, gofumpt, golines)
[group('lint')]
fmt:
    golangci-lint fmt ./...

# Run linter and apply formatters
[group('lint')]
fix:
    golangci-lint run --fix ./...

# Download module dependencies
[group('deps')]
download:
    go mod download

# Tidy go.mod and go.sum
[group('deps')]
tidy:
    go mod tidy

# Update all dependencies to latest versions
[group('deps')]
update: && tidy
    go get -u -t ./...

# Check for vulnerabilities
[group('check')]
vuln:
    govulncheck ./...

# Quality assurance: tidy, fix, build, and test
[group('check')]
qa: tidy fix build test

# Full check: lint, build, and test
[group('check')]
check: lint build test

# Build Docker image
[group('container')]
docker-build image="huma-playground:local" version="dev" runtime_img="":
    docker build \
        --build-arg VERSION={{version}} \
        {{ if runtime_img != "" { "--build-arg RUNTIME_IMAGE=" + runtime_img } else { "" } }} \
        -t {{image}} .

# Run Docker container
[group('container')]
docker-up image="huma-playground:local" name="huma-playground" port=PORT:
    docker run -d --rm --name {{name}} -p {{port}}:8080 {{image}}

# Show Docker container logs
[group('container')]
docker-logs name="huma-playground":
    docker logs -f {{name}}

# Stop Docker container
[group('container')]
docker-down name="huma-playground":
    -docker stop {{name}}
