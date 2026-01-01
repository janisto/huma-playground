# Huma Playground

A minimal REST API skeleton built with [Huma](https://github.com/danielgtaylor/huma) running on top of Chi via `humachi`. It demonstrates structured logging, RFC 9457 Problem Details for errors, and a modular route layout that is ready to grow into a larger service.

<img src="assets/gopher.svg" alt="Go Gopher mascot illustration" width="400">

<sub>Gopher illustration from [free-gophers-pack](https://github.com/MariaLetta/free-gophers-pack) by Maria Letta</sub>

## Features
- Chi middleware stack with security headers, CORS, request IDs, real IP detection, panic recovery, and structured access logs
- Request-scoped Zap logger that automatically enriches entries with Google Cloud trace metadata when available (accepting Cloud Trace headers with or without the sampling directive) and falls back to the request ID when no trace header exists
- Plain response bodies with RFC 9457 Problem Details for errors
- Content negotiation supporting JSON and CBOR formats
- Cursor-based pagination with RFC 8288 Link headers
- Route registration collected under `internal/routes` with context-aware logging helpers (`LogInfo`, `LogWarn`, `LogError`, `LogFatal`)

## API Design Principles

This API follows Resource-Oriented Architecture (ROA) and RESTful conventions:

### URI Design

- Use plural nouns for collections (`/users`, not `/user`)
- Avoid verbs in URIs; let HTTP methods convey the action
- Nest resources to express relationships (`/posts/{postId}/comments`); limit nesting to one level

### HTTP Methods & Status Codes

| Method | Purpose | Success Status |
|--------|---------|----------------|
| GET | Retrieve resource(s) | 200 OK |
| POST | Create a resource | 201 Created |
| PUT | Replace a resource entirely | 200 OK or 204 No Content |
| PATCH | Partial update | 200 OK or 204 No Content |
| DELETE | Remove a resource | 204 No Content |

### Error Responses

Errors follow [RFC 9457 Problem Details](https://www.rfc-editor.org/rfc/rfc9457.html) and honor content negotiation:
- `application/problem+json` when JSON is requested (default)
- `application/problem+cbor` when CBOR is requested

The `Accept` header controls the error format, independent of the request `Content-Type`. Even CBOR requests receive JSON errors unless `Accept: application/cbor` is set.

| Status | Use Case |
|--------|----------|
| 400 Bad Request | Malformed syntax, missing required fields |
| 422 Unprocessable Entity | Validation failures on specific fields |

### Content Negotiation

Content negotiation follows [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110.html#section-12.5.1):
- Default: `application/json` ([RFC 8259](https://www.rfc-editor.org/rfc/rfc8259.html))
- Alternate: `application/cbor` ([RFC 8949](https://www.rfc-editor.org/rfc/rfc8949.html))
- Format selected via `Accept` header with q-value support (e.g., `Accept: application/cbor;q=1.0, application/json;q=0.9`)
- Q-value is the primary ranking factor; specificity determines which q-value applies and acts as a tie-breaker
- `q=0` excludes a format entirely
- Responses include `Vary: Accept` header for proper cache behavior

> **Note:** The API docs (`/api-docs`) may default to CBOR in the content type dropdown (alphabetical ordering). Select `application/json` when testing via the browser, as the docs UI cannot encode CBOR binary format.

### Request ID

The `X-Request-ID` header tracks requests end-to-end.

### Pagination

- Cursor-based tokens for stability
- Links provided via HTTP `Link` header per [RFC 8288](https://www.rfc-editor.org/rfc/rfc8288.html)

## Requirements
- Go 1.25+
- [Just](https://github.com/casey/just) command runner (optional)

## Justfile

The project includes a [Justfile](Justfile) for common development tasks. Run `just` to see available commands:

| Command | Description |
|---------|-------------|
| `just` | List available commands |
| `just build` | Build the application |
| `just run` | Run the server |
| `just test` | Run all tests |
| `just test-coverage` | Run tests with coverage |
| `just coverage` | Generate HTML coverage report |
| `just lint` | Run linter |
| `just fmt` | Apply formatters |
| `just fix` | Run linter and apply formatters |
| `just check` | Full check (build + test + lint) |
| `just vuln` | Check for vulnerabilities |
| `just clean` | Remove coverage artifacts |

## Dependency Management

Download all dependencies in the current directory and its subdirectories, without modifying `go.mod` or `go.sum`:

```bash
go mod download
```

Update all dependencies in the current directory and its subdirectories, including test dependencies:

```bash
go get -u -t ./...
go mod tidy
```

## Quick Start
```bash
go run ./cmd/server
```

Then visit:
- `http://localhost:8080/health` - service health probe
- `http://localhost:8080/api-docs` - interactive API explorer provided by Huma
- `http://localhost:8080/openapi.json` - generated OpenAPI schema

Sample request:
```bash
curl -s localhost:8080/health | jq
```

Sample response:

```json
{
  "$schema": "http://localhost:8080/schemas/HealthData.json",
  "message": "healthy"
}
```

## Routes
| Method | Path      | Description                            |
|--------|-----------|----------------------------------------|
| GET    | `/health` | Health check route                     |
| GET    | `/hello`  | Default greeting                       |
| POST   | `/hello`  | Create a personalized greeting         |
| GET    | `/items`  | List items with cursor-based pagination|

## Observability & Logging
- `internal/middleware.Security` sets security headers on all responses following OWASP REST Security Cheat Sheet recommendations.
- `internal/middleware.Vary` adds the `Accept` header to `Vary` for proper cache behavior with content negotiation.
- `internal/middleware.RequestLogger` injects a request-scoped `zap.Logger` enriched with trace IDs derived from the W3C `traceparent` header when a Google Cloud project ID (`GOOGLE_CLOUD_PROJECT`, `GCP_PROJECT`, `GCLOUD_PROJECT`, or `PROJECT_ID`) is present.
- `internal/middleware.AccessLogger` emits a single structured log per request including duration, status, and byte count.
- `internal/middleware.CORS` wraps go-chi/cors with permissive API defaults and is installed at the top of the stack. Adjust the options there if the service needs to restrict origins or headers.
- Use the helper functions `LogInfo`, `LogWarn`, `LogError`, and `LogFatal` to write logs inside handlers; they preserve contextual fields such as trace IDs and automatically attach the `error` field when provided.
- `internal/respond` provides panic recovery and error handlers that emit RFC 9457 Problem Details.
- JSON responses are emitted as UTF-8 with HTML escaping disabled so payloads (such as URLs) arrive unmodified.
- Response bodies include a `$schema` pointer to the JSON Schema describing the payload; this comes from Huma's schema link transformer (enabled via `huma.DefaultConfig`) so tooling can discover contracts automatically. You can remove it by customizing the Huma config if you don't need schema discovery.

## Error Handling
Errors follow RFC 9457 Problem Details format:
- `title`: human-readable summary (e.g., "Not Found")
- `status`: HTTP status code
- `detail`: specific explanation of the error

Content-Type follows the `Accept` header:
- `application/problem+json` (default) - RFC 9457 registered media type
- `application/problem+cbor` (extension) - project-specific extension for CBOR clients

> **Note:** RFC 9457 only registers `application/problem+json` and `application/problem+xml`. 
> The `application/problem+cbor` media type used here follows the same structured suffix 
> convention per RFC 6839 for clients that prefer binary encoding. Clients expecting strictly 
> RFC 9457-compliant media types should use the JSON format.

Use Huma's built-in error helpers: `huma.Error400BadRequest()`, `huma.Error404NotFound()`, `huma.Error422UnprocessableEntity()`, etc.

## Project Layout
```
cmd/server/          # Application entrypoint and HTTP server bootstrap
internal/common/     # Shared logger construction
internal/middleware/ # Security headers, CORS, request ID, logging middleware and context helpers
internal/pagination/ # Cursor-based pagination (Cursor, Params, Link headers)
internal/respond/    # Panic recovery and Problem Details error handlers
internal/routes/     # Route registration grouped by domain
```

## Adding Routes
1. Create a new file under `internal/routes` (e.g. `users.go`).
2. Define your handler using Huma with an output struct containing a `Body` field.
3. Add a registration function and call it from `routes.Register`.
4. Log within handlers using the context-aware helpers to include trace metadata automatically.
5. Return errors using Huma's built-in error helpers (e.g., `huma.Error400BadRequest()`, `huma.Error404NotFound()`).
6. For redirects, use `respond.WriteRedirect()` helper.

## Testing
```bash
go test ./...
```

Run with verbose output:
```bash
go test -v ./...
```

Run with coverage:
```bash
go test -v -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

The project has 151 tests across 13 test files.

## Linting
The project uses [golangci-lint](https://golangci-lint.run/) v2 for static analysis and code formatting. Configuration is defined in `.golangci.yml`.

Run linter:
```bash
golangci-lint run ./...
```

Apply formatters (gci, gofumpt, golines) automatically:
```bash
golangci-lint fmt ./...
```

Run linter and apply formatters in one step:
```bash
golangci-lint run --fix ./...
```

## Trace Logging Check

Start the server with a project ID so Cloud Trace fields get populated:
```bash
export PROJECT_ID=test-project-id && go run ./cmd/server
```
Hit an endpoint while supplying the trace header:
```bash
curl -H 'traceparent: 00-3d23d071b5bfd6579171efce907685cb-08f067aa0ba902b7-01' http://localhost:8080/health
```
Watch the server stdout; the access log for this request should now include `logging.googleapis.com/trace`, `logging.googleapis.com/spanId`, `logging.googleapis.com/trace_sampled`, and still carry the `requestId` / `traceId`.

`traceparent` format: `{version}-{trace-id}-{parent-id}-{trace-flags}` (W3C Trace Context)

## Deployment Notes

### Google Cloud Run

This application is designed for deployment on [Cloud Run](https://cloud.google.com/run). Key considerations:

**Container Contract**
- Cloud Run terminates TLS and forwards requests as HTTP/1 to your container
- Listen on the port specified by the `PORT` environment variable (default: 8080)
- Cloud Run sets `K_SERVICE`, `K_REVISION`, and `K_CONFIGURATION` environment variables

**Request Headers**
- Cloud Run acts as a trusted reverse proxy and sets `X-Forwarded-For` with the client IP
- The `RealIP` middleware safely extracts client IPs from proxy headers in this environment
- `traceparent` header is automatically populated for distributed tracing

> **Security Warning:** The `RealIP` middleware trusts `X-Forwarded-For` and `X-Real-IP` headers.
> Only use this middleware behind a trusted reverse proxy (Cloud Run, nginx, HAProxy, etc.).
> Without a trusted proxy, malicious clients can spoof their IP address, bypassing
> IP-based access controls, rate limiting, and audit logging.

**Tracing**
- Cloud Run uses the W3C `traceparent` header for automatic trace propagation
- Set a project ID environment variable (`GOOGLE_CLOUD_PROJECT`, `GCP_PROJECT`, `GCLOUD_PROJECT`, or `PROJECT_ID`) to enable trace correlation in Cloud Logging

**Logging**
- JSON logs on stdout are automatically ingested by Cloud Logging
- Structured logs with trace fields are correlated with Cloud Trace spans

**Graceful Shutdown**
- Cloud Run sends `SIGTERM` 10 seconds before `SIGKILL`; in-flight requests are given time to complete

### General Production Notes
- Production deployments should set the Google Cloud project ID env var so trace links point to the correct project
- Logs are JSON on stdout, ready for ingestion by Cloud Run or any log aggregator

### Server Timeouts
The HTTP server is configured with the following timeouts for security and resource management:

| Setting | Value | Purpose |
|---------|-------|---------|
| `ReadTimeout` | 5s | Maximum time to read entire request including body |
| `ReadHeaderTimeout` | 2s | Maximum time to read request headers (slowloris protection) |
| `WriteTimeout` | 10s | Maximum time to write response |
| `IdleTimeout` | 60s | Maximum time to wait for next request on keep-alive |
| `MaxHeaderBytes` | 64 KB | Maximum size of request headers |

## License
MIT
