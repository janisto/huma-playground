# Huma Playground

[![Build and test](https://img.shields.io/github/actions/workflow/status/janisto/huma-playground/app-ci.yml?branch=main&label=build%20%26%20test)](https://github.com/janisto/huma-playground/actions/workflows/app-ci.yml)
[![Code quality](https://img.shields.io/github/actions/workflow/status/janisto/huma-playground/app-lint.yml?branch=main&label=code%20quality)](https://github.com/janisto/huma-playground/actions/workflows/app-lint.yml)
[![Go version](https://img.shields.io/github/go-mod/go-version/janisto/huma-playground?filename=go.mod)](https://github.com/janisto/huma-playground/blob/main/go.mod)
[![License](https://img.shields.io/github/license/janisto/huma-playground)](LICENSE)

A compact, production-conscious REST API example using [Huma v2](https://huma.rocks/) on [Chi](https://github.com/go-chi/chi). It demonstrates API contracts, JSON and CBOR negotiation, RFC 9457 errors, structured observability, Firebase Authentication, Firestore persistence, and a separate Go Cloud Run function without turning a playground into a framework.

<img src="assets/gopher.svg" alt="Go Gopher mascot illustration" width="400">

<sub>Gopher illustration from [free-gophers-pack](https://github.com/MariaLetta/free-gophers-pack) by Maria Letta.</sub>

## What this example demonstrates

- Huma v2 typed operations and runtime-generated OpenAPI 3.1
- Exact fourteen-operation portable contract plus `GET /openapi.json` discovery
- Strict JSON and CBOR requests and responses; JSON Problem Details with the same model in ordinary CBOR
- One Huma error pipeline for operation and Chi-level 404, 405, and recovery responses
- Request IDs, trace metadata, operation-aware access logs, and request-scoped Zap loggers
- Cursor pagination with RFC 8288 `Link` headers
- Firebase ID-token verification with revocation checks
- Firestore atomic create, transaction-safe partial update, existence-checked delete, and audit events
- Explicit development-offline, emulator, and live Firebase modes
- Bounded request, upstream response, server, and shutdown work
- Anonymous, public-only GitHub proxy requests with fixed-origin transport, redirect, timeout, and response-size bounds
- Non-root distroless container execution
- A deliberately small, separately deployable Go Functions Framework example
- Required CI execution for both Go modules plus separate emulator-backed coverage

## Requirements

- Go 1.26.5+ (the repository currently pins 1.26.5)
- [Just](https://github.com/casey/just)
- [golangci-lint v2](https://golangci-lint.run/)
- [actionlint](https://github.com/rhysd/actionlint) (`brew install actionlint`)
- Firebase CLI and Java 21 when running emulator integration tests
- Docker or Podman for container checks

Copy the example environment:

```bash
cp .env.example .env
```

The default configuration is safe for local exploration: the public API starts without cloud credentials and protected Firebase routes return `503 Service Unavailable`.

## Quick start

```bash
just install
just run
```

Open:

- `http://localhost:8080/health`
- `http://localhost:8080/openapi.json`

Example:

```bash
curl --fail --silent http://localhost:8080/v1/hello
curl --fail --silent \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada"}' \
  http://localhost:8080/v1/hello
```

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `HOST` | `0.0.0.0` | Listen host |
| `PORT` | `8080` | Listen port |
| `APP_ENVIRONMENT` | `development` | `development`, `staging`, or `production` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `FIREBASE_MODE` | `offline` | `offline`, `emulator`, or `live` |
| `FIREBASE_PROJECT_ID` | `demo-test-project` outside live mode | Firebase project |
| `FIREBASE_AUTH_EMULATOR_HOST` | unset | Auth emulator address |
| `FIRESTORE_EMULATOR_HOST` | unset | Firestore emulator address |
| `CORS_ALLOWED_ORIGINS` | `*` in development | Comma-separated browser origins; required outside development |
| `GOTOOLCHAIN` | set by `.env` | Repository Go toolchain pin |

### Firebase modes

`offline` is development-only. Public routes work; protected routes fail closed with 503. No Firebase SDK client is created.

`emulator` is development-only and requires both emulator host variables as valid `host:port` authorities without a URL scheme or whitespace. Use a `demo-*` project ID. Partial or malformed emulator configuration is rejected at startup.

`live` requires a non-demo project and Application Default Credentials. Emulator variables and `demo-*` projects are rejected. Production and staging also require explicit non-wildcard CORS origins.

These checks prevent accidental use of the Auth emulator outside development, where unsigned test tokens would be unsafe.

`firestore.rules` denies direct client reads and writes. The Admin SDK bypasses those rules, so the API enforces ownership by deriving a collision-safe profile document path from the verified Firebase UID rather than accepting a user ID from the request.

## API

| Method | Path | Description |
|---|---|---|
| GET | `/health` | Liveness probe |
| GET | `/v1/hello` | Default greeting |
| POST | `/v1/hello` | Generate a personalized greeting |
| GET | `/v1/items` | Cursor-paginated static items |
| POST | `/v1/profile` | Create the authenticated user's profile |
| GET | `/v1/profile` | Read the authenticated user's profile |
| PATCH | `/v1/profile` | Partially update the authenticated user's profile |
| DELETE | `/v1/profile` | Delete the authenticated user's profile |
| GET | `/v1/github/owners/{owner}` | GitHub owner information |
| GET | `/v1/github/owners/{owner}/repos` | Cursor-paginated public repositories |
| GET | `/v1/github/repos/{owner}/{repo}` | Repository details |
| GET | `/v1/github/repos/{owner}/{repo}/activity` | Cursor-paginated activity |
| GET | `/v1/github/repos/{owner}/{repo}/languages` | Repository language bytes |
| GET | `/v1/github/repos/{owner}/{repo}/tags` | Cursor-paginated tags |
| GET | `/openapi.json` | Generated OpenAPI 3.1 document |

Profile JSON uses camelCase (`firstName`, `lastName`, `contactEmail`, `phoneNumber`, `marketingOptIn`, `termsAccepted`). Firestore uses independent snake_case storage names. `contactEmail` is normalized user-supplied contact data and is not the verified Firebase identity email. `termsAccepted` must be `true`; updates cannot revoke it.

Profile creation uses Firestore create-if-absent semantics, partial updates preserve unrelated stored fields, no-op updates perform no write, and deletion uses an existence precondition rather than a read-before-delete transaction. Stored records are validated before exposure so legacy or corrupt documents fail closed.

Each sample item's `price` is a closed money object with non-negative `amountMinor` and fixed `currency: "USD"`. The static catalog contains 30 items and list pages default to 20 with a maximum of 100.

## Content negotiation and errors

Use `Accept: application/json` or `Accept: application/cbor`. JSON is the default.

Errors are RFC 9457 Problem Details:

- `application/problem+json`
- `application/cbor` when CBOR is negotiated

Success and error objects are closed and contain no framework-added envelope or schema-link fields. Malformed syntax returns `400`; semantic validation returns `422`; unsupported request media returns `415`; and an unacceptable response representation returns `406`.

Operation metadata lists only errors reachable for that operation. Unexpected Firebase and GitHub dependency failures are logged once with request correlation and a safe operation name; clients receive generic Problem Details without upstream internals.

Request bodies are limited to exactly 1,000,000 bytes. Unknown or repeated scalar query parameters, unknown body properties, duplicate JSON object members, non-finite CBOR floats, and trailing documents are rejected. JSON and CBOR container nesting is capped at 32 levels. Missing, malformed, repeated, or comma-combined `X-Request-ID` values are replaced with a generated identifier. Application request contexts expire before the server write timeout so Firebase and GitHub work is canceled within the response budget.

## Development commands

All repository workflows go through Just so `.env` and `GOTOOLCHAIN` are applied consistently.

| Command | Purpose |
|---|---|
| `just build` | Build both Go modules |
| `just test` | Test both Go modules |
| `just test-race` | Run both modules with the race detector |
| `just fuzz` | Fuzz the cursor and GitHub Link-header parsers |
| `just lint` | Lint both modules |
| `just fmt` | Format both modules |
| `just fmt-check` | Reject formatting drift |
| `just tidy-check` | Reject module-file drift |
| `just vuln` | Run `govulncheck` against both modules |
| `just workflow-check` | Validate GitHub Actions locally with the installed `actionlint` |
| `just coverage` | Generate root application coverage reports |
| `just functions-run` | Run the local Functions Framework target |
| `just functions-smoke` | Build and probe the registered function target |
| `just emulators` | Start Auth and Firestore emulators |
| `just test-integration-ci` | Require emulator-backed tests and generate their separate coverage report |
| `just container-smoke` | Build and probe the final non-root image |
| `just profile-migration-audit PROJECT [MANIFEST]` | Read and classify profile records without writing |
| `just profile-migration-apply PROJECT MANIFEST CONFIRM` | Apply an authorized profile cutover after exact project confirmation |

`go.work` is optional, local-only convenience. It is ignored intentionally. Every root recipe sets `GOWORK=off` for the nested function module, so clean clones and CI do not depend on a workspace file.

## Firebase emulator tests

Start emulators:

```bash
just emulators
```

Ports:

- Auth: `127.0.0.1:7110`
- Firestore: `127.0.0.1:7130`
- Emulator UI: `127.0.0.1:4000`

Ordinary local tests skip emulator cases when emulators are absent. The required CI recipe sets `REQUIRE_FIREBASE_EMULATORS=1`, so unavailable or broken emulators fail rather than silently reducing coverage. It writes a separate `integration-coverage.*` report; CI does not merge that profile with the fast unit report.

## One-time profile data migration

Deployments with pre-contract profile records must audit them before serving the adopted profile lifecycle. The audit recognizes only the exact known legacy shape and the accepted canonical shape. It blocks ambiguous ownership, duplicate logical principals, invalid data, and legacy records without explicit terms-acceptance evidence; output uses one-way principal fingerprints rather than identifiers or profile data.

Start read-only with the intended Firestore project and, when legacy records exist, a reviewed manifest based on [`docs/profile-migration-manifest.example.json`](docs/profile-migration-manifest.example.json):

```bash
just profile-migration-audit PROJECT_ID path/to/reviewed-manifest.json
```

The apply command is intentionally separate and requires the project ID twice. It preflights the full dataset before writing and replaces each authorized record transactionally, so reruns are safe after interruption:

```bash
just profile-migration-apply PROJECT_ID path/to/reviewed-manifest.json PROJECT_ID
```

Quiesce profile writes for the complete audit-and-apply window; otherwise a record created or changed after the dataset preflight can make the reviewed migration set stale. Both commands use Application Default Credentials. Run neither against a project that has not been explicitly selected and reviewed.

## Separate Go function

`functions/` is an independent Go module. The HTTP registration is at the module root beside `functions/go.mod`, as required by Cloud Run source functions. `functions/cmd/server` is a local Functions Framework runner.

The example intentionally contains one small handler, one test file, and one runner. It does not import Huma, Firebase Admin, or the application observability stack.
Its timestamp layout is deliberately duplicated: sharing one constant would couple two independently built and deployed modules to remove a trivial line.

Run it:

```bash
just functions-run
curl --fail --silent 'http://localhost:8080/?name=Ada'
```

The local recipe sets `LOCAL_ONLY=true`, so the runner binds to `127.0.0.1`. Direct runner invocations can omit that variable to use the Functions Framework's all-interface default.

Deploy it as a Cloud Run function, not through Firebase CLI:

```bash
gcloud run deploy huma-playground-hello \
  --source functions \
  --function Hello \
  --base-image go126 \
  --region REGION \
  --allow-unauthenticated
```

Choose authenticated invocation instead when the function should not be public.

## Container and Cloud Run service

```bash
just container-build
just container-smoke
```

The final image is distroless, statically linked, and runs as UID/GID `65532:65532`. The build injects the supplied version into startup logs and OCI labels.

Deploy the already-built image:

```bash
gcloud run deploy huma-playground \
  --image REGION-docker.pkg.dev/PROJECT_ID/REPOSITORY/huma-playground:TAG \
  --region REGION
```

Do not combine this image deployment with `--base-image` or `--automatic-updates`. Go standard-library, dependency, and base-image fixes require rebuilding and redeploying the compiled artifact.

## Architecture

```text
.agents/skills/                 six portable project workflows with Codex UI metadata
.github/agents/                 evidence-based security review profile for GitHub Copilot
cmd/server/                     typed config, composition, lifecycle
cmd/profile-migrate/            guarded one-time profile storage cutover
internal/http/health/           unversioned liveness transport
internal/http/v1/               Huma operations grouped by resource
internal/http/v1/routes/        route composition
internal/platform/auth/         Firebase verification and Huma auth middleware
internal/platform/firebase/     Firebase Admin client initialization
internal/platform/middleware/   HTTP security, CORS, Vary, Chi access logs
internal/platform/pagination/   transport-independent cursor mechanics
internal/platform/portable/     accepted representations, errors, validation, and OpenAPI projection
internal/platform/respond/      Chi recovery/errors delegated to Huma
internal/platform/timeutil/     fixed-precision JSON/CBOR timestamps
internal/service/github/        bounded GitHub API adapter
internal/service/profile/       Firestore profile store
internal/testutil/              emulator-only test helpers
functions/                      independent Functions Framework module
```

The application uses constructor-style composition and narrow interfaces. It deliberately does not add a DI container, repository framework, generic service layer, in-process distributed rate limiter, cache, or OpenTelemetry dependency.

Repository guidance follows the canonical [AGENTS.md format](https://github.com/agentsmd/agents.md). Portable skills use
the canonical [Agent Skills specification and documentation](https://github.com/agentskills/agentskills), with the
detailed [format specification](https://agentskills.io/specification), under `.agents/skills/`. See
[AGENTS.md](AGENTS.md) for the working rules and current skill catalog.

## Observability

The app uses `github.com/janisto/huma-observability/v2`. `obs.HTTPRequestContext` is installed at the Chi boundary so liveness, recovery, 404, 405, and Huma routes share request IDs, request-scoped loggers, and W3C trace metadata. The HTTP and Huma middleware explicitly use Trace Context Level 1. Huma routes additionally use `obs.RequestContext` and `obs.AccessLogger` for operation-aware logs.

Access logs are privacy-minimized: Huma records route templates and operation IDs without raw paths, peer IPs, or user agents; the local Chi logger follows the same boundary and wraps only Chi-only routes and error handlers. Escaping non-abort panics before a response write are recorded as status 500 with `terminal_reason: "panic"` and error severity. This split also prevents duplicate `/v1` access logs. `obs.Logger(ctx)` is intentionally request-bound; process and background work must receive an explicit logger.

Forwarding headers are removed at the outer HTTP boundary because this example does not define a trusted-proxy boundary; this also prevents forwarded-host values from influencing Huma schema links.

## Security notes

- Bearer tokens and user-supplied profile/greeting values are not logged.
- Prefer local ADC (`gcloud auth application-default login`) for live-mode experiments; do not place service-account keys in the repository.
- CORS credentials are disabled.
- HSTS belongs at the trusted TLS edge, not this HTTP application.
- Public rate limiting belongs at Cloud Run, API Gateway, or Cloud Armor unless the application gets an identity-aware quota requirement.
- `/health` is liveness only. Add dependency readiness only for a deployment with a concrete readiness contract.

## License

[MIT](LICENSE)
