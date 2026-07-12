---
name: readme-maintenance
description: Audit or update huma-playground README.md when routes, configuration, development commands, CI, containers, Firebase behavior, OpenAPI, or function deployment guidance changes.
---

# README maintenance

Read `AGENTS.md` first, then verify every affected `README.md` claim against the current repository. Keep the README
focused on software engineers and new contributors; keep agent execution rules and detailed coding patterns in
`AGENTS.md` or task-specific skills.

## Source of truth

Read only areas relevant to the documentation change, using these files as the primary map:

- application and configuration: `cmd/server/main.go`, `cmd/server/application.go`, and `cmd/server/config.go`;
- routes and contracts: `internal/http/health/`, `internal/http/v1/routes/`, and registered handler packages;
- Huma documentation and schemas: API construction in `cmd/server/application.go` and contract tests in
  `cmd/server/main_test.go`;
- separate function: `functions/function.go`, `functions/cmd/server/main.go`, and `functions/go.mod`;
- tooling and deployment: `Justfile`, `Dockerfile`, `firebase.json`, `.env.example`, and `.github/workflows/`;
- dependencies and versions: `go.mod`, `functions/go.mod`, and pinned workflow or container references.

Do not copy historical claims from `plans/` into README without re-verifying them against current code and primary
documentation.

## Required README contract

Keep the document concise and onboarding-oriented. Preserve or update these subjects when implemented:

- project purpose and significant capabilities;
- routes, status semantics, JSON and CBOR requests/responses, and RFC 9457 errors;
- requirements, safe local defaults, and a working quick start;
- environment variables, direct-peer IP trust, CORS premise, and offline/emulator/live Firebase modes;
- independent root and function module commands;
- the distinction that Firebase CLI runs Auth and Firestore emulators but does not deploy the Go function;
- the Cloud Run source-deployment function path and container-image service path;
- runtime-generated OpenAPI, Stoplight Elements, schemas, container behavior, and required CI;
- concise project layout, contribution pointer, and license.

Organize material for readers rather than preserving a rigid heading order. Remove stale sections instead of
maintaining compatibility with an old README structure.

## Accuracy rules

- Require every command and path to exist with its current argument order.
- Match defaults and failure behavior to `loadConfig` and application composition.
- Describe the Go modules as independent; do not imply root `./...` crosses into `functions/`.
- Describe `/health` as dependency-free liveness, not Firebase readiness.
- State that Huma accepts and returns JSON and CBOR; error format follows `Accept`, not request `Content-Type`.
- Keep emulator variables development-only and explain why production rejects them.
- Keep the separate function deliberately small; do not describe it as Huma or Firebase Admin code.
- Do not claim this repository is deployed to production.
- Prefer primary sources for runtime, framework, deployment, and security claims that may change.
- Do not add agent instructions, source tutorials, speculative features, or duplicated `AGENTS.md` content.

## Verification

Verify named recipes without mutating dependencies:

```bash
just --dry-run build
just --dry-run test
just --dry-run lint
just --dry-run check
just --dry-run test-functions
just --dry-run vuln-functions
just --dry-run functions-run 8081
just --dry-run test-integration-ci
just --dry-run update
```

Search route registration and configuration directly rather than trusting prose. If documentation changes alongside
application behavior, run `just build`, `just test`, and `just lint`; use `just check` for cross-module changes. For a
documentation-only correction, validate links, paths, commands, YAML where touched, and `git diff --check`.

Before finishing, reread the complete README for contradictions, duplicated explanations, unexpanded acronyms, and
claims more confident than the evidence.
