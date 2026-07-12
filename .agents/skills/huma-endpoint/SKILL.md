---
name: huma-endpoint
description: Create or change Huma v2 endpoints in huma-playground, including route registration, typed inputs and outputs, validation, authentication, Problem Details, JSON or CBOR, OpenAPI, and tests.
---

# Huma v2 endpoints

Read `AGENTS.md`, the neighboring handler package, `internal/http/v1/routes/routes.go`, and the relevant platform or
service contracts before editing the root Huma application.

Do not apply this skill to `functions/`. That module is intentionally a separate, minimal Functions Framework example.

## Design boundary

- Put transport models and handlers under `internal/http/v1/<resource>/`.
- Keep business and persistence behavior behind focused interfaces under `internal/service/` when needed.
- Reuse `internal/platform/`; do not introduce a generic controller, response envelope, or service container.
- Register operations on the versioned Huma API in `internal/http/v1/routes/routes.go`.
- Keep `/health` as a dependency-free Chi liveness handler outside Huma.

The router installs `auth.NewAuthMiddleware` once. Protected operations declare `Security: auth.RequireAuth()`; the
middleware rejects missing, invalid, unavailable, or identity-less authentication before the handler runs. Read the
verified identity with `auth.UserFromContext(ctx)` and enforce ownership at the service boundary.

## Operation pattern

Use resource-prefixed input and output names because Huma has a global schema registry:

```go
type ResourceGetInput struct {
	ID string `path:"id" doc:"Resource identifier" example:"res-001"`
}

type ResourceGetOutput struct {
	Body Resource
}

func Register(api huma.API, service Service) {
	huma.Register(api, huma.Operation{
		OperationID: "get-resource",
		Method:      http.MethodGet,
		Path:        "/resources/{id}",
		Summary:     "Get a resource",
		Tags:        []string{"Resources"},
		Errors:      []int{http.StatusNotFound, http.StatusServiceUnavailable},
	}, func(ctx context.Context, input *ResourceGetInput) (*ResourceGetOutput, error) {
		resource, err := service.Get(ctx, input.ID)
		if err != nil {
			return nil, mapServiceError(err)
		}
		return &ResourceGetOutput{Body: *resource}, nil
	})
}
```

Use plain `Body` outputs, a typed header field for `Location` or `Link`, and `DefaultStatus` for 201 or 204. Construct
resource URLs from the explicit API prefix passed to registration; do not derive them from OpenAPI server metadata.

Huma validates typed path, query, header, and body fields. Use `doc`, representative `example`, and appropriate
validation tags. Keep JSON names camelCase, map storage names in the service layer, and use `timeutil.Time` for the
repository's UTC millisecond response contract. The application accepts JSON and CBOR bodies and applies a global 1
MiB body limit.

Map expected service errors to Huma's RFC 9457 helpers, including 400, 401, 403, 404, 409, 422, and 503. Return a
generic 500 for unexpected failures; log the underlying error once without secrets or PII.

Use `obs.Logger(ctx)` only for useful request-scoped events. The access logger already records every request. Startup,
background work, scripts, and direct service tests must use an explicit process logger.

## OpenAPI contract

Keep `OperationID`, summary, description, tags, security, reachable errors, headers, DTO tags, and JSON/CBOR media
types consistent with runtime behavior. Huma generates OpenAPI and component schemas at runtime; do not commit a
generated specification. Bodyless GET operations must not inherit request-body-only 413 or 415 responses. Use the
`openapi-contract` skill when the public contract changes.

## Verification

Add colocated tests for success, validation, authentication or authorization, service failures, negotiated JSON and
CBOR, and relevant headers. Use the `go-testing` skill for repository test conventions.

Run the focused package tests, then `just build`, `just test`, and `just lint`. Run `just check` when shared routing or
both modules are affected.
