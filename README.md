# Huma Playground

A minimal REST API skeleton built with [Huma](https://github.com/danielgtaylor/huma) running on top of Chi via `humachi`. It demonstrates structured logging, consistent response envelopes, and a modular route layout that is ready to grow into a larger service.

## Features
- Chi middleware stack with CORS, request IDs, real IP detection, panic recovery, and structured access logs
- Request-scoped Zap logger that automatically enriches entries with Google Cloud trace metadata when available (accepting Cloud Trace headers with or without the sampling directive) and falls back to the request ID when no trace header exists
- Consistent `data/meta/error` envelopes for every response via generics in `internal/api`
- Unified response helpers in `internal/respond` that log and emit the shared envelope for success, redirects, and errors
- Route registration collected under `internal/routes` with context-aware logging helpers (`LogInfo`, `LogWarn`, `LogError`, `LogFatal`)

## Requirements
- Go 1.25+

## Quick Start
```bash
go run ./cmd/server
```

Then visit:
- `http://localhost:8080/health` – service health probe
- `http://localhost:8080/docs` – interactive API explorer provided by Huma
- `http://localhost:8080/openapi.json` – generated OpenAPI schema

Sample request:
```bash
curl -s localhost:8080/health | jq
```

Sample response:

```json
{
  "$schema": "http://localhost:8080/schemas/EnvelopeHealthData.json",
  "data": {
    "message": "healthy"
  },
  "meta": {
    "traceId": "..."
  },
  "error": null
}
```

## Routes
| Method | Path      | Description        |
|--------|-----------|--------------------|
| GET    | `/health` | Health check route |

## Observability & Logging
- `internal/middleware.RequestLogger` injects a request-scoped `zap.Logger` enriched with trace IDs derived from the `X-Cloud-Trace-Context` header when a Google Cloud project ID (`GOOGLE_CLOUD_PROJECT`, `GCP_PROJECT`, `GCLOUD_PROJECT`, or `PROJECT_ID`) is present.
- `internal/middleware.AccessLogger` emits a single structured log per request including duration, status, and byte count.
- `internal/middleware.CORS` wraps go-chi/cors with permissive API defaults and is installed at the top of the stack. Adjust the options there if the service needs to restrict origins or headers.
- Use the helper functions `LogInfo`, `LogWarn`, `LogError`, and `LogFatal` to write logs inside handlers; they preserve contextual fields such as trace IDs and automatically attach the `error` field when provided.
- `internal/respond` centralizes success, redirect, and error responses so everything uses the shared envelope and logging helpers.
- JSON responses are emitted as UTF-8 with HTML escaping disabled so payloads (such as URLs) arrive unmodified.
- Response bodies include a `$schema` pointer to the JSON Schema describing the payload; this comes from Huma's schema link transformer (enabled via `huma.DefaultConfig`) so tooling can discover contracts automatically. You can remove it by customizing the Huma config if you don't need schema discovery.

## Response Envelopes
All responses follow a predictable envelope defined in `internal/api/envelope.go`:
- `data`: primary payload (pointer so `null` is explicit)
- `meta`: shared metadata including `traceId`
- `error`: populated only on failure using `NewErrorEnvelope`, which clones provided field issues to keep responses immutable regardless of caller mutations
- Error messages default to human-friendly text even for non-standard HTTP status codes.

## Project Layout
```
cmd/server          # Application entrypoint and HTTP server bootstrap
 internal/api        # Shared envelope and response types
 internal/common     # Shared logger construction
 internal/middleware # Request logging middleware and helper functions
 internal/respond    # Centralized response helpers backed by NewErrorEnvelope
 internal/routes     # Route registration grouped by domain
```

## Adding Routes
1. Create a new file under `internal/routes` (e.g. `users.go`).
2. Define your handler using Huma and return the appropriate envelope type.
3. Add a registration function and call it from `routes.Register`.
4. Log within handlers using the context-aware helpers to include trace metadata automatically.
5. Return errors with `respond.Error(...)` (or helper constructors you add) so responses stay consistent. The helper will log with the appropriate level, clone detail slices, and ensure the error code/message always have sensible defaults.
6. Use `respond.Success` / `respond.WriteRedirect` helpers whenever writing responses directly; they apply the shared envelope, disable HTML escaping, and set `Content-Type: application/json; charset=utf-8`.

## Testing
```bash
go test ./...
```

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
curl -H 'X-Cloud-Trace-Context: 3d23d071b5bfd6579171efce907685cb/643745351650131537;o=1' http://localhost:8080/health
```
Watch the server stdout; the access log for this request should now include `logging.googleapis.com/trace`, `logging.googleapis.com/spanId`, `logging.googleapis.com/trace_sampled`, and still carry the `requestId` / `traceId`.

`X-Cloud-Trace-Context` format: `traceId/spanId;o=sampled`

## Deployment Notes
- Production deployments should set the Google Cloud project ID env var so trace links point to the correct project.
- Logs are JSON on stdout, ready for ingestion by Cloud Run or any log aggregator.

## License
MIT
