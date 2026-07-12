---
name: openapi-contract
description: Maintain and verify huma-playground runtime-generated OpenAPI 3.1, Huma operation metadata and schemas, Problem Details media types, bearer security, schema links, and Stoplight Elements when routes, models, errors, or API documentation change.
---

# OpenAPI contract maintenance

Read `AGENTS.md`, `cmd/server/application.go`, and affected handlers and models before changing the public contract or
documentation integration.

## Architecture

The public contract has four connected parts:

1. Huma operation metadata and model tags define paths, operations, validation, schemas, responses, and errors.
2. `cmd/server/application.go` configures the `/v1` server prefix, runtime OpenAPI routes, Stoplight Elements, bearer
   security, and JSON/CBOR media types.
3. Huma serves OpenAPI JSON/YAML and component schemas at runtime; no generated specification is committed.
4. `cmd/server/main_test.go` verifies routes, media types, security, prefix behavior, and every advertised component
   schema link.

Do not add a parallel generator, hand-maintained specification, or runtime filesystem dependency for the contract.

## Contract rules

- Keep every operation ID unique and stable.
- Keep paths relative to the Huma API mounted at the configured prefix.
- Add clear summaries, useful descriptions, and existing top-level tags.
- Include every status reachable from validation, body limits, authentication, service mapping, and request deadlines.
- Add `Security: auth.RequireAuth()` to every protected operation and nowhere else.
- Document `Location` and `Link` through typed output header fields.
- Use `doc` tags on public fields and representative `example` tags where useful.
- Keep examples consistent with camelCase JSON names and UTC millisecond timestamps.
- Keep JSON and CBOR request/success content aligned with actual Huma formats.
- Ensure error responses expose `application/problem+json` and `application/problem+cbor`.
- Preserve Huma `$schema` fields and `describedBy` links that resolve beneath the configured API prefix.

If a systematic contract correction is required, update the Huma configuration hook in `cmd/server/application.go`
and its tests rather than patching a serialized document.

## Workflow

1. Inspect route registration, handler behavior, input/output models, and relevant error mapping.
2. Update operation metadata or model tags.
3. Add focused handler tests for changed status, media type, validation, security, or headers.
4. Add cross-cutting assertions in `cmd/server/main_test.go` when an invariant spans operations or mounted routes.
5. Start the composed application or use its test router and inspect `/v1/openapi.json` plus affected schemas.
6. Reject unrelated schema churn, duplicate IDs, missing statuses, incorrect security, broken links, or media types that
   disagree with runtime negotiation.
7. Run `just build`, `just test`, and `just lint`; use `just check` when shared tooling or both modules are in scope.

## Review checklist

- OpenAPI remains 3.1 and all registered routes are represented.
- Semantic tests assert the exact path, method, status, security, and media-type contract for every operation.
- Every operation ID is unique and protected operations use HTTP bearer authentication.
- Request bodies and successful responses advertise JSON and CBOR where implemented.
- Documented errors use both Problem Details media types.
- Every documented error is reachable; bodyless GET operations do not advertise request-body-only 413 or 415 errors.
- 201 responses document `Location`; paginated 200 responses document `Link`.
- All component schema URLs and response `describedBy` links resolve.
- Stoplight Elements remains at `/v1/api-docs` and OpenAPI JSON/YAML routes remain available.
- Untrusted forwarding headers cannot change schema origins or links.
