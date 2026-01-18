# Justfile for Huma Playground
# https://github.com/casey/just

set dotenv-load

PORT := env("PORT", "8080")

# Container runtime: prefer podman, fallback to docker
CONTAINER_RUNTIME := if `command -v podman 2>/dev/null || true` != "" { "podman" } else { "docker" }


@_:
    just --list

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
    PORT={{ port }} go run ./cmd/server

# Start Firebase emulators for E2E tests
[group('test')]
emulators:
    firebase emulators:start --only auth,firestore

# Run all tests
[group('test')]
test *args:
    go test ./... {{ args }}

# Run tests with verbose output
[group('test')]
test-verbose *args:
    go test -v ./... {{ args }}

# Run tests with coverage
[group('test')]
test-coverage *args:
    go test -v -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./... {{ args }}

# Generate coverage report
[group('test')]
coverage: test-coverage
    go tool cover -func=coverage.out
    go tool cover -html=coverage.out -o coverage.html

# Run linter
[group('qa')]
lint:
    golangci-lint run ./...

# Apply formatters (gci, gofumpt, golines)
[group('qa')]
fmt:
    golangci-lint fmt ./...

# Run linter and apply formatters
[group('qa')]
fix:
    golangci-lint run --fix ./...

# Check for vulnerabilities
[group('qa')]
vuln:
    govulncheck ./...

# Quality assurance: tidy, fix, build, and test
[group('qa')]
qa: tidy fix build test

# Full check: lint, build, and test
[group('qa')]
check: lint build test

# Download module dependencies
alias install := download
[group('lifecycle')]
download:
    go mod download

# Tidy go.mod and go.sum
[group('lifecycle')]
tidy:
    go mod tidy

# Update all dependencies to latest versions
[group('lifecycle')]
update: && tidy
    go get -u -t ./...

# Recreate project from clean state
[group('lifecycle')]
fresh: clean download build

# Container tasks
[group('container')]
container-build image="huma-playground:latest" version="dev" runtime_img="":
    {{ CONTAINER_RUNTIME }} build \
        --build-arg VERSION={{ version }} \
        {{ if runtime_img != "" { "--build-arg RUNTIME_IMAGE=" + runtime_img } else { "" } }} \
        -t {{ image }} .

[group('container')]
container-up image="huma-playground:latest" name="huma-playground" port=PORT:
    {{ CONTAINER_RUNTIME }} run -d --rm --name {{ name }} \
        {{ if path_exists(".env") == "true" { "--env-file .env" } else { "" } }} \
        -p {{ port }}:8080 {{ image }}

[group('container')]
container-logs name="huma-playground":
    {{ CONTAINER_RUNTIME }} logs -f {{ name }}

[group('container')]
container-down name="huma-playground":
    -{{ CONTAINER_RUNTIME }} stop {{ name }}
