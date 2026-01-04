# Huma Playground

A minimal REST API skeleton built with [Huma](https://github.com/danielgtaylor/huma) running on top of Chi via `humachi`. It demonstrates structured logging, RFC 9457 Problem Details for errors, and a modular route layout that is ready to grow into a larger service.

<img src="assets/gopher.svg" alt="Go Gopher mascot illustration" width="400">

<sub>Gopher illustration from [free-gophers-pack](https://github.com/MariaLetta/free-gophers-pack) by Maria Letta</sub>

## Features

- Layered middleware architecture with security headers, CORS, request IDs, real IP detection, and structured access logs
- Request-scoped Zap logger with Google Cloud Trace correlation via [W3C Trace Context](https://www.w3.org/TR/trace-context/) `traceparent` header, falling back to request ID when no trace exists
- [RFC 9457 Problem Details](https://datatracker.ietf.org/doc/html/rfc9457) for all error responses with optional field-level validation errors
- Content negotiation supporting [JSON (RFC 8259)](https://datatracker.ietf.org/doc/html/rfc8259) and [CBOR (RFC 8949)](https://datatracker.ietf.org/doc/html/rfc8949) formats via `Accept` header
- Cursor-based pagination with [RFC 8288 Link](https://datatracker.ietf.org/doc/html/rfc8288) headers
- [OpenAPI 3.1](https://spec.openapis.org/oas/v3.1.0) documentation with Swagger UI, auto-generated from Huma route schemas
- Health check endpoint (`/health`) for liveness probes

## API Design Principles

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

| Status | Use Case |
|--------|----------|
| 400 Bad Request | Malformed syntax, missing required fields |
| 422 Unprocessable Entity | Validation failures on specific fields |

### Content Negotiation

- Default: `application/json` ([RFC 8259](https://www.rfc-editor.org/rfc/rfc8259.html))
- Alternate: `application/cbor` ([RFC 8949](https://www.rfc-editor.org/rfc/rfc8949.html))
- Format selected via `Accept` header with q-value support

### Pagination

- Cursor-based tokens for stability
- Links provided via HTTP `Link` header per [RFC 8288](https://www.rfc-editor.org/rfc/rfc8288.html)

## Requirements

- Go 1.25+
- [Just](https://github.com/casey/just) command runner (optional)

## Go Workspace

This project uses a [Go workspace](https://go.dev/doc/tutorial/workspaces) (`go.work`) to manage multiple modules:

```go
go 1.25.5

use (
    .
    ./functions
)
```

The workspace allows simultaneous development across modules. Commands like `go build`, `go test`, and `go mod tidy` operate on all workspace modules when run from the root.

## Quick Start

```bash
go run ./cmd/server
```

Then visit:
- `http://localhost:8080/health` - service health probe
- `http://localhost:8080/v1/api-docs` - interactive API explorer
- `http://localhost:8080/v1/openapi` - generated OpenAPI schema

Sample request:
```bash
curl -s localhost:8080/health | jq
```

## Environment Variables

Copy `.env.example` to `.env` and customize as needed:

```bash
cp .env.example .env
```

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Server listen port | `8080` |
| `HOST` | Host address to bind to | `0.0.0.0` |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `FIREBASE_PROJECT_ID` | Firebase project ID for Cloud Trace correlation | - |
| `APP_ENVIRONMENT` | Environment label | `development` |
| `APP_URL` | Base URL for the application | `http://localhost:8080` |

## Project Layout

```
cmd/server/            # Application entrypoint and HTTP server bootstrap
internal/http/         # HTTP transport layer
  health/              # Health check handler (unversioned)
  v1/                  # Versioned API (v1)
    hello/             # Hello endpoint handlers
    items/             # Items endpoint handlers
    routes/            # Route registration
internal/platform/     # Cross-cutting infrastructure
  logging/             # Structured logging with Zap
  middleware/          # Security headers, CORS, request ID
  pagination/          # Cursor-based pagination
  respond/             # Panic recovery and Problem Details
  timeutil/            # Time formatting utilities
functions/             # Cloud Functions (separate Go module)
```

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check route |
| GET | `/v1/hello` | Default greeting |
| POST | `/v1/hello` | Create a personalized greeting |
| GET | `/v1/items` | List items with cursor-based pagination |

## Development

### Build and Test

```bash
go build -v ./...     # Build
go test ./...         # Run tests
go test -v ./...      # Verbose output
golangci-lint run ./...  # Lint
```

### Justfile Commands

| Command | Description |
|---------|-------------|
| `just build` | Build the application |
| `just run` | Run the server |
| `just test` | Run all tests |
| `just lint` | Run linter |
| `just check` | Full check (build + test + lint) |

Run `just` to see all available commands.

### Dependencies

```bash
go mod download        # Download dependencies
go get -u -t ./...     # Update dependencies
go mod tidy            # Clean up go.mod
```

## Adding Routes

1. Create a new package under `internal/http/v1/` (e.g., `users/handler.go`)
2. Define your handler using Huma with an output struct containing a `Body` field
3. Add a registration function and call it from `routes.Register`
4. Return errors using Huma's error helpers (`huma.Error400BadRequest()`, etc.)

## Docker

```bash
just docker-build      # Build image
just docker-up         # Run container detached
just docker-down       # Stop container
```

Or with Docker CLI:
```bash
docker build -t huma-playground:latest .
docker run --rm -p 8080:8080 huma-playground:latest
```

## Deployment

### Google Cloud Run

```bash
# Build and push
gcloud builds submit --tag REGION-docker.pkg.dev/PROJECT/REPO/huma-playground:latest

# Deploy with automatic base image updates
gcloud run deploy huma-playground \
  --image REGION-docker.pkg.dev/PROJECT/REPO/huma-playground:latest \
  --platform managed \
  --region REGION \
  --base-image go125 \
  --automatic-updates
```

The `--base-image` and `--automatic-updates` flags enable [automatic base image updates](https://cloud.google.com/run/docs/configuring/services/automatic-base-image-updates), allowing Google to apply security patches to the OS and runtime without rebuilding or redeploying.

Set a `FIREBASE_PROJECT_ID` environment variable to enable trace correlation in Cloud Logging.

## CI/CD

GitHub Actions workflows in `.github/workflows/`:

- **app-ci.yml**: Build check, tests, coverage report
- **app-lint.yml**: golangci-lint static analysis
- **labeler.yml**: Automatic PR labeling

## Contributing

See [AGENTS.md](AGENTS.md) for coding guidelines and conventions.

## License

MIT
