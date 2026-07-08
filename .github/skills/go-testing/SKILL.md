---
name: go-testing
description: Guide for writing Go tests following this project's patterns including httptest, test organization, and coverage requirements.
---

# Go Testing

Use this skill when writing tests for this Huma REST API application.

For comprehensive testing guidelines, see `AGENTS.md` in the repository root.

## Test Organization

Tests are colocated with source files using `_test.go` suffix:

```
internal/
    http/
        v1/
            routes/
                routes.go
                routes_test.go
            items/
                handler.go
                handler_test.go
    platform/
        middleware/
            accesslog.go
            accesslog_test.go
```

## Test Server Setup

Create test servers using Chi router and Huma:

```go
package routes_test

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/danielgtaylor/huma/v2"
    humachi "github.com/danielgtaylor/huma/v2/adapters/humachi"
    "github.com/go-chi/chi/v5"
    chimiddleware "github.com/go-chi/chi/v5/middleware"
    "github.com/janisto/huma-observability"
    "go.uber.org/zap"

    "github.com/janisto/huma-playground/internal/http/health"
    "github.com/janisto/huma-playground/internal/http/v1/hello"
    appmiddleware "github.com/janisto/huma-playground/internal/platform/middleware"
    "github.com/janisto/huma-playground/internal/platform/respond"
)

func setupTestRouter() *chi.Mux {
    logger := zap.NewNop()
    router := chi.NewRouter()
    httpAccessLogger := appmiddleware.AccessLogger()
    router.NotFound(httpAccessLogger(respond.NotFoundHandler()).ServeHTTP)
    router.MethodNotAllowed(httpAccessLogger(respond.MethodNotAllowedHandler()).ServeHTTP)
    router.Use(
        obs.HTTPRequestContext(obs.HTTPRequestContextConfig{Logger: logger}),
        respond.Recoverer(logger),
        chimiddleware.ClientIPFromRemoteAddr,
    )

    // Root-level endpoints (unversioned)
    router.Group(func(r chi.Router) {
        r.Use(httpAccessLogger)
        r.Get("/health", health.Handler)
    })

    // Versioned API
    router.Route("/v1", func(r chi.Router) {
        api := humachi.New(r, huma.DefaultConfig("Test", "test"))
        api.UseMiddleware(obs.RequestContext(obs.RequestContextConfig{Logger: logger}))
        api.UseMiddleware(obs.AccessLogger(obs.AccessLoggerConfig{Logger: logger}))
        hello.Register(api)
    })
    return router
}
```

## Basic Test Pattern

```go
func TestHealthEndpoint(t *testing.T) {
    router := setupTestRouter()

    req := httptest.NewRequest(http.MethodGet, "/health", nil)
    req.Header.Set(chimiddleware.RequestIDHeader, "test-trace-id")
    resp := httptest.NewRecorder()

    router.ServeHTTP(resp, req)

    if resp.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.Code)
    }

    var body health.Response
    if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
        t.Fatalf("failed to decode response: %v", err)
    }
    if body.Status != "healthy" {
        t.Fatalf("unexpected status: %s", body.Status)
    }
}
```

## Testing Error Responses

Verify RFC 9457 Problem Details format:

```go
func TestNotFoundReturns404(t *testing.T) {
    router := setupTestRouter()

    req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
    resp := httptest.NewRecorder()

    router.ServeHTTP(resp, req)

    if resp.Code != http.StatusNotFound {
        t.Fatalf("expected 404, got %d", resp.Code)
    }

    var problem huma.ErrorModel
    if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
        t.Fatalf("failed to unmarshal problem: %v", err)
    }
    if problem.Status != http.StatusNotFound {
        t.Fatalf("expected status 404, got %d", problem.Status)
    }
    if problem.Title != "Not Found" {
        t.Fatalf("unexpected title: %s", problem.Title)
    }
}
```

## Testing POST Requests

```go
func TestCreateResource(t *testing.T) {
    router := setupTestRouter()

    body := `{"name": "Test Resource"}`
    req := httptest.NewRequest(http.MethodPost, "/v1/resources", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp := httptest.NewRecorder()

    router.ServeHTTP(resp, req)

    if resp.Code != http.StatusCreated {
        t.Fatalf("expected 201, got %d", resp.Code)
    }

    location := resp.Header().Get("Location")
    if location == "" {
        t.Fatal("expected Location header")
    }
}
```

## Testing Validation Errors

```go
func TestValidationReturns422(t *testing.T) {
    router := setupTestRouter()

    body := `{"name": ""}` // Empty name should fail validation
    req := httptest.NewRequest(http.MethodPost, "/v1/resources", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    resp := httptest.NewRecorder()

    router.ServeHTTP(resp, req)

    if resp.Code != http.StatusUnprocessableEntity {
        t.Fatalf("expected 422, got %d", resp.Code)
    }

    var problem huma.ErrorModel
    if err := json.Unmarshal(resp.Body.Bytes(), &problem); err != nil {
        t.Fatalf("failed to unmarshal: %v", err)
    }
    if len(problem.Errors) == 0 {
        t.Fatal("expected validation errors")
    }
}
```

## Table-Driven Tests

Use subtests for comprehensive coverage:

```go
func TestListItems(t *testing.T) {
    router := setupTestRouter()

    tests := []struct {
        name       string
        query      string
        wantStatus int
        wantItems  int
    }{
        {"default limit", "", http.StatusOK, 20},
        {"custom limit", "?limit=5", http.StatusOK, 5},
        {"filter category", "?category=electronics", http.StatusOK, 10},
        {"invalid cursor", "?cursor=invalid", http.StatusBadRequest, 0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            req := httptest.NewRequest(http.MethodGet, "/v1/items"+tt.query, nil)
            resp := httptest.NewRecorder()

            router.ServeHTTP(resp, req)

            if resp.Code != tt.wantStatus {
                t.Errorf("got status %d, want %d", resp.Code, tt.wantStatus)
            }
        })
    }
}
```

## Testing Link Headers

For paginated endpoints:

```go
func TestPaginationLinkHeader(t *testing.T) {
    router := setupTestRouter()

    req := httptest.NewRequest(http.MethodGet, "/v1/items?limit=5", nil)
    resp := httptest.NewRecorder()

    router.ServeHTTP(resp, req)

    if resp.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.Code)
    }

    link := resp.Header().Get("Link")
    if link == "" {
        t.Fatal("expected Link header for pagination")
    }

    if !strings.Contains(link, `rel="next"`) {
        t.Error("expected next link in Link header")
    }
}
```

## Testing Content Negotiation

```go
func TestCBORResponse(t *testing.T) {
    router := setupTestRouter()

    req := httptest.NewRequest(http.MethodGet, "/health", nil)
    req.Header.Set("Accept", "application/cbor")
    resp := httptest.NewRecorder()

    router.ServeHTTP(resp, req)

    if resp.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.Code)
    }

    contentType := resp.Header().Get("Content-Type")
    if !strings.Contains(contentType, "application/cbor") {
        t.Errorf("expected CBOR content type, got %s", contentType)
    }
}
```

## Test Naming Convention

Pattern: `Test<Function>_<Scenario>` or `Test<Endpoint>Returns<Status><Condition>`

```go
func TestHealthEndpoint(t *testing.T) { ... }
func TestCreateResource_Returns201OnSuccess(t *testing.T) { ... }
func TestGetResource_Returns404WhenNotFound(t *testing.T) { ... }
func TestListItems_WithInvalidCursor_Returns400(t *testing.T) { ... }
```

## Running Tests

```bash
# Run all tests
just test

# Verbose output
just test-verbose

# With coverage
just coverage

# Coverage report
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Coverage Requirements

Tests should cover:
- Success paths (200, 201, 204)
- Error paths (400, 404, 422, 500)
- Edge cases (empty input, boundary values)
- Problem Details format verification
- Trace ID propagation
- Content negotiation (JSON/CBOR)

## Important Notes

- Always set `X-Request-ID` header for trace testing
- Verify response Content-Type matches Accept header
- Check Problem Details structure for all error responses
- Test both valid and invalid enum values
- Verify Location header for 201 Created responses
