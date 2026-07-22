---
name: go-testing
description: Write or review tests for huma-playground using Go testing, Huma v2 humatest or httptest, Firebase emulators, race checks, coverage, and the independent function module.
---

# Go testing

Read `AGENTS.md`, the implementation under test, and nearby tests before choosing the test boundary in either Go module.
Apply `$adversarial-testing` first to rank failure modes and select mutation-resistant cases; this skill supplies Go,
Huma, fixture, emulator, and command conventions.

## Test at the narrowest useful boundary

- Pure helpers and service behavior: ordinary table-driven unit tests.
- One Huma operation: `humatest` or a small `humachi` API with only the required middleware.
- Routing or middleware composition: `httptest` against the composed router from `cmd/server` tests.
- Firestore and Firebase Auth behavior: the real local emulators, not production test switches.
- `functions/`: direct handler tests and the module's Functions Framework smoke path; do not import the root app.

Keep fakes and fixtures in the consuming `*_test.go` file. Never add `if testing` branches, environment backdoors,
production mocks, or mock-only hooks. Refactor toward a focused interface when a real dependency boundary needs
substitution.

## Request setup

Use `httptest.NewRequestWithContext(t.Context(), ...)`. Set `Content-Type` for JSON or CBOR bodies, `Authorization:
Bearer test-token` for protected routes, and `Accept` only when exercising negotiation. Set a fixed `X-Request-ID`
only when the assertion depends on correlation.

Use a small isolated Huma API for operation behavior. Keep composed Chi/Huma tests for mounts, middleware order,
request IDs, 404/405/recovery, docs, schemas, negotiation, security wiring, and access-log ownership.

## Assertions that matter

Verify observable contracts, not implementation trivia:

- exact HTTP status and relevant headers (`Content-Type`, `Location`, `Link`, `Allow`, `WWW-Authenticate`, and
  `X-Request-ID`);
- decoded response fields and RFC 9457 `huma.ErrorModel` responses;
- JSON and CBOR request/response negotiation at representative contract boundaries;
- malformed syntax, unknown fields or query parameters, validation limits, unsupported media types, and the 1 MiB
  application body limit where middleware is in scope;
- service error mapping, request deadlines, and absence of secrets or PII in observed logs;
- pagination boundaries, malformed/wrong-type/stale cursors, preserved filters and limits, and terminal Link behavior.

Use `t.Helper()` for assertion helpers, `t.Setenv()` for environment isolation, `errors.Is` for sentinel errors, and
fuzz seeds at parser boundaries. Avoid sleeps; use contexts, channels, or bounded polling.

## Firebase emulator tests

Ordinary local tests may skip when Auth and Firestore emulators are unavailable. Required integration validation uses:

```bash
just test-integration-ci
```

This sets `REQUIRE_FIREBASE_EMULATORS=1`, so missing emulators fail. Keep its coverage separate from fast unit
coverage. Emulator helpers must use `t.Context()`, bounded clients and response bodies, and reject non-2xx responses
before decoding. Exercise concurrent mutation semantics repeatedly when changing Firestore preconditions or
transactions; a single passing emulator run is insufficient evidence for contention-sensitive behavior. Unit-test
emulator availability, environment setup, status classification, bounded diagnostics, and read failures without
requiring a running emulator; reserve real SDK and cleanup behavior for the required integration lane.

## Commands

Use Just so `.env` and the pinned Go toolchain are applied:

```bash
just test
just test-race
just coverage
just test-functions
just test-race-functions
just functions-smoke
just check
```

After code changes, finish with `just build`, `just test`, and `just lint`. Run `just test-integration-ci` when Firebase
behavior changes.
