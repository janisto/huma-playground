# AGENTS.md

This file provides instructions for AI coding agents working on this repository.

## Project Overview

Huma Playground is a minimal REST API skeleton built with [Huma](https://github.com/danielgtaylor/huma) running on top of Chi via `humachi`. It demonstrates structured logging, consistent response envelopes, and a modular route layout.

### Key Features

- Chi middleware stack with CORS, request IDs, real IP detection, panic recovery, and structured access logs
- Request-scoped Zap logger with Google Cloud trace metadata enrichment
- Consistent `data/meta/error` envelopes for every response via generics
- Unified response helpers for success, redirects, and errors

## Setup

### Requirements

- Go 1.25+

### Install Dependencies

```bash
go mod download
```

### Build

```bash
go build -v ./...
```

### Run

```bash
go run ./cmd/server
```

The server starts on port 8080 with endpoints:
- `http://localhost:8080/health` – health probe
- `http://localhost:8080/docs` – interactive API explorer
- `http://localhost:8080/openapi.json` – OpenAPI schema

## Testing

Run all tests:

```bash
go test ./...
```

Run tests with verbose output:

```bash
go test -v ./...
```

Run tests with coverage:

```bash
go test -v -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
```

Generate coverage report:

```bash
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Project Structure

```
cmd/server/           # Application entrypoint and HTTP server bootstrap
internal/api/         # Shared envelope and response types (Envelope, Meta, ErrorBody)
internal/common/      # Shared logger construction
internal/middleware/  # Request logging middleware and context helpers
internal/respond/     # Centralized response helpers for consistent envelopes
internal/routes/      # Route registration grouped by domain
```

## Coding Conventions

### Response Envelopes

All API responses must use the shared envelope format defined in `internal/api/envelope.go`:

```go
type Envelope[T any] struct {
    Data  *T         `json:"data"`
    Meta  Meta       `json:"meta"`
    Error *ErrorBody `json:"error"`
}
```

- `data`: primary payload (pointer so `null` is explicit)
- `meta`: shared metadata including `traceId`
- `error`: populated only on failure

### Error Handling

- Use `respond.Error(...)` for error responses to maintain consistency
- Error messages default to human-friendly text
- The helper will log with the appropriate level and clone detail slices

### Logging

Use context-aware logging helpers from `internal/middleware`:

```go
appmiddleware.LogInfo(ctx, "message", zap.String("key", "value"))
appmiddleware.LogWarn(ctx, "message", zap.String("key", "value"))
appmiddleware.LogError(ctx, "message", err, zap.String("key", "value"))
appmiddleware.LogFatal(ctx, "message", err, zap.String("key", "value"))
```

These helpers preserve contextual fields such as trace IDs.

### Adding New Routes

1. Create a new file under `internal/routes` (e.g., `users.go`)
2. Define your handler using Huma and return the appropriate envelope type
3. Add a registration function and call it from `routes.Register`
4. Log within handlers using context-aware helpers
5. Return errors with `respond.Error(...)` for consistency
6. Use `respond.Success` / `respond.WriteRedirect` helpers for responses

### Handler Pattern

```go
func registerMyRoute(api huma.API) {
    huma.Get(api, "/my-route", func(ctx context.Context, _ *struct{}) (*respond.Body[myData], error) {
        appmiddleware.LogInfo(ctx, "my route", zap.String("path", "/my-route"))
        resp := respond.Success(ctx, myData{...})
        return &resp, nil
    })
}
```

### JSON Encoding

- JSON responses are UTF-8 with HTML escaping disabled
- Response bodies include a `$schema` pointer to the JSON Schema

## Testing Guidelines

### Test Structure

- Tests are colocated with source files using `_test.go` suffix
- Use Go's standard `testing` package
- Create test servers using Chi router and Huma

### Test Pattern

```go
func TestMyFeature(t *testing.T) {
    respond.Install()
    router := chi.NewRouter()
    router.Use(
        appmiddleware.RequestID(),
        chimiddleware.RealIP,
        appmiddleware.RequestLogger(),
        respond.Recoverer(),
    )
    api := humachi.New(router, huma.DefaultConfig("Test", "test"))
    routes.Register(api)

    req := httptest.NewRequest(http.MethodGet, "/endpoint", nil)
    req.Header.Set(chimiddleware.RequestIDHeader, "test-trace-id")
    resp := httptest.NewRecorder()
    router.ServeHTTP(resp, req)

    // Verify response
    if resp.Code != http.StatusOK {
        t.Fatalf("expected 200 OK, got %d", resp.Code)
    }

    var env apiinternal.Envelope[myData]
    if err := json.Unmarshal(resp.Body.Bytes(), &env); err != nil {
        t.Fatalf("failed to decode envelope: %v", err)
    }
    // Assert envelope fields...
}
```

### Coverage Requirements

- Tests should cover success paths, error paths, and edge cases
- Verify response envelopes contain expected data, meta, and error fields
- Test trace ID propagation through the request context

## Restrictions

- Never commit secrets or sensitive data
- Do not modify `go.mod` or `go.sum` without explicit request
- Keep response envelope structure consistent across all endpoints
- Always use the centralized `respond` package for API responses
- Do not add new dependencies without justification
