# Huma Playground Modernization Plan

Status: implemented and locally verified
Reviewed: 2026-07-12
Baseline: `c5991fe` (`main`, synchronized with `origin/main`)
Scope: application code, Huma v2 usage, tests, the independent Cloud Function module, CI, tooling, container/deployment guidance, and repository documentation

This file contains the original review baseline followed by the implementation result. All other files in `plans/` are historical records and remain unchanged; apparent conflicts are evidence of the repository's evolution, not cleanup targets.

## Implementation progress

- [x] Reconciled implementation baseline with current `main` at `c5991fe`.
- [x] Moved the Go function registration to the function-module root, added strict bounded request handling and tests, and added a runnable Functions Framework target.
- [x] Removed unsupported Firebase CLI Go-function configuration and unused Function/Storage emulators.
- [x] Made aggregate Just recipes cover both Go modules explicitly, using `GOWORK=off` for the nested function module; added function smoke, race, format, tidy, vulnerability, workflow, and emulator-CI recipes.
- [x] Added pinned Go tool dependencies for `govulncheck` and `actionlint`.
- [x] Refactored CI workflows around stable aggregate `ci` and `lint` gates, required emulator tests, both-module vulnerability checks, and final-image verification.
- [x] Replaced monolithic startup with typed configuration, explicit application composition, local offline Firebase mode, bounded request contexts, and testable server lifecycle.
- [x] Unified Chi errors with Huma negotiation/schema transforms; corrected JSON/CBOR error metadata and removed the parallel custom Accept implementation.
- [x] Corrected profile JSON/Firestore naming, removed the misleading consent boolean and hidden normalization, narrowed Firestore writes, and removed production test doubles.
- [x] Simplified timestamp CBOR handling with the existing library, corrected the money model, and bounded/validated GitHub client construction and responses.
- [x] Made security/cache/CORS policies route- and environment-aware and corrected the portable container build/version path.
- [x] Synchronized README, AGENTS, `.github/agents`, and repository-local `.agents/skills` documentation with the implemented contracts and commands.
- [x] Completed the local validation matrix and recorded the remaining hosted-only limitations.

## Second-pass completion audit

The first green matrix was treated as a checkpoint rather than proof that the review was exhausted. Runtime probes, source review, current dependency checks, and comparison against the documented acceptance criteria found the following additional gaps:

- [x] Publish emulator-backed coverage as the separate CI artifact promised by this plan; the prior emulator job ran tests but uploaded no coverage.
- [x] Make `tidy-check` non-mutating with Go's native diff mode, verify `go fix -diff` already fails on patches, and add the modernization check to required lint CI.
- [x] Install mounted Huma subrouter 404/405 handlers so 405 responses include the RFC-required `Allow` header.
- [x] Reject non-empty cursors without the endpoint's exact resource type and bound cursor query length.
- [x] Reject GitHub base URLs without an origin or with credentials, path, query, or fragment components that request resolution would ignore.
- [x] Classify transient Firestore failures as service unavailable rather than generic internal errors.
- [x] Reject nil or identity-less successful authentication results instead of allowing protected handlers to panic.
- [x] Abort a partially written response after recovery; a Problem Details body cannot safely replace bytes already sent.
- [x] Bind the server socket before emitting the listening log so failed startup cannot produce a false success event.
- [x] Remove untrusted forwarding headers before Huma builds schema URLs; Huma v2 otherwise accepts client-provided `Forwarded` and `X-Forwarded-Host` values.
- [x] Validate emulator endpoints as Firebase-compatible `host:port` values so malformed configuration fails at startup.
- [x] Allow both W3C `traceparent` and `tracestate` through browser preflight rather than advertising partial trace-context support.
- [x] Align GitHub owner validation with current account-login rules instead of accepting dots, consecutive hyphens, or trailing hyphens.
- [x] Remove unused response/timestamp helpers and avoid exporting implementation-only function payload types.
- [x] Prevent unrelated `pull_request_target` labeler runs from canceling each other and keep OCI version/revision metadata semantically correct.
- [x] Expand the security-review guide to the real trust boundaries and exclude common Firebase Admin key filenames from Git and container contexts.
- [x] Restrict `LOG_LEVEL` to the documented non-terminating levels instead of accepting Zap's `dpanic`, `panic`, and `fatal` values.
- [x] Remove the obsolete `*_mock.go` formatter exemption after deleting production mock implementations.
- [x] Synchronize affected README, AGENTS, `.github/agents`, and `.agents/skills` guidance and rerun the full matrix.
- [x] Migrate repository skills, including `readme-maintenance`, to `.agents/skills/<skill-name>/SKILL.md`, correct stale examples and review instructions, and include the new path in documentation labeling.
- [x] Align the complete five-skill set with established repository conventions, including a Huma-specific `openapi-contract` skill, while preserving runtime-generated OpenAPI, JSON/CBOR request support, and Huma-owned validation/error behavior.

## Post-change review remediation

The 2026-07-12 post-change review reran the complete local matrix instead of relying on the earlier green checkpoint. It
found one required-CI blocker and four correctness/operability gaps. This section is the active remediation record;
earlier completion statements remain evidence of the checkpoint that the review invalidated.

- [x] Replace the lock-heavy read-before-delete Firestore transaction with an atomic existence-preconditioned delete,
  preserve 404 semantics, and prove concurrent deletion repeatedly against the emulator.
- [x] Remove unreachable 408/413/415 responses from bodyless GET OpenAPI operations and add an exact contract assertion
  so generic error lists cannot reintroduce them.
- [x] Reject malformed Firebase emulator hosts, including whitespace/control-character hostnames, before SDK startup.
- [x] Reject GitHub repository path values that consist only of dot segments before `url.ResolveReference` can normalize
  them into another upstream endpoint.
- [x] Log unavailable and unexpected profile/GitHub dependency failures exactly once with request metadata while keeping
  client Problem Details generic and free of dependency internals.
- [x] Reconcile README, AGENTS, `.agents`, and `.github` guidance with the remediated behavior.
- [x] Rerun the full local matrix, including repeated required emulator tests and final-container inspection, then replace
  the superseded evidence table below with honest final results.

Repository guidance/tooling work completed before this remediation:

- Migrated `go-testing`, `huma-endpoint`, and `pagination-endpoint` from `.github/skills/` to the portable
  `.agents/skills/<skill-name>/SKILL.md` layout and removed the old tracked copies.
- Migrated the former `.github/agents/readme-review.agent.md` prompt into the portable `readme-maintenance` skill.
- Added the Huma-specific `openapi-contract` skill so runtime-generated schemas, media types, security, and Stoplight
  behavior have a focused maintenance workflow.
- Added Codex UI metadata at `agents/openai.yaml` for all five skills and documented the Agent Skills discovery and
  custom-agent separation rules in README and AGENTS.
- Kept `security-review.agent.md` in `.github/agents/` as the single GitHub Copilot custom-agent profile and aligned it
  with the application's real Firebase, GitHub, forwarding-header, logging, container, and CI trust boundaries.
- Updated `.github/labeler.yml` so portable Markdown skills are documentation and tooling files receive the tooling
  label; updated labeler workflows to use PR-scoped concurrency and current exact action tags.
- Updated app CI/lint workflows to preserve stable required `ci`/`lint` jobs while checking both modules, required
  emulators, vulnerabilities, dependency changes, final images, formatting, module drift, workflows, and Go fixes.
- Updated Dependabot for root Go, function Go, Actions, and Docker with quarterly grouped updates and a one-day routine
  cooldown. Security updates remain immediate.

Post-remediation evidence:

- `just test-integration-ci -- -count=3` passed three full emulator-backed executions, and a separate focused
  `TestFirestoreConcurrentDelete` run passed five consecutive executions. A final integration coverage run passed at
  86.6% statement coverage.
- `TestBodylessGetResponseStatusesMatchRuntime` asserts the exact documented status set for every bodyless example GET;
  dot-only repository validation and warning/error logging paths have focused handler tests.
- All five skill frontmatter documents and their `agents/openai.yaml` metadata parse successfully, names match their
  directories, and no stale `.github/skills`, `readme-review.agent.md`, or Swagger references remain outside historical
  plan records.
- The complete matrix below was rerun after remediation. Generated coverage and emulator logs were removed afterward.

## Cross-repository alignment follow-up

The final implementation was compared with a mature Go API example after the remediation pass. Transferable
improvements were adapted; generated Swagger, custom binding/negotiation, and framework-specific middleware were
intentionally not copied into Huma's runtime-generated contract model.

- [x] Made Echo comparison a durable AGENTS working agreement so future shared tooling, CI, guide, and test improvements
  are evaluated for Huma instead of drifting independently.
- [x] Added ordinary unit coverage for emulator reachability, environment setup, bounded non-success diagnostics, and
  response-read failures while retaining real Firebase behavior in the required emulator lane.
- [x] Expanded the GitHub security-review agent with explicit independent-function, final-container, workflow-permission,
  checkout-credential, action-version, aggregate-gate, and deployment-tool checks.
- [x] Expanded golangci-lint with high-signal correctness analyzers for canonical HTTP headers, JSON encoding errors,
  logger calls, nil/error behavior, duration and slice mistakes, receiver consistency, test APIs, compiler directives,
  security-sensitive Unicode, tags, and wasted work.
- [x] Rejected noisy candidate linters that conflict with valid project behavior: exhaustive gRPC enum handling,
  Unicode-script rejection in an intentional cursor round trip, generic `nil,nil` rejection for Huma no-content
  handlers, and context inference that misclassified the recovery closure.
- [x] Fixed issues exposed by the stronger lint policy: checked JSON encoder results in the health handler and standalone
  function, used canonical `net/http` header keys, made recovery context capture explicit, and removed a predeclared-name
  collision in a test store.
- [x] Corrected the stale AGENTS Firebase example to use the current `SkipIfEmulatorUnavailable`, `SetupEmulator`, cleanup,
  and Firestore client lifecycle.

Follow-up validation passed: `just check`, `just test -- -shuffle=on -count=3`, `just test-race`,
`just test-integration-ci`, `just functions-smoke`, `just tidy-check`, `just modernize-check`, `just workflow-check`,
`just vuln`, `just coverage`, `just container-smoke`, skill metadata parsing, and `git diff --check`. The clean fast-suite
coverage is 74.7%; the required emulator-backed profile is 86.8%. The final image remains non-root at `65532:65532`,
reports version `ci-smoke`, and has no false OCI revision label. The updated `go-testing` package also passes the
canonical `skill-creator` `quick_validate.py` validator with the user-installed PyYAML runtime.

## Final review closure

The final full review on 2026-07-12 covered production composition, handlers, service boundaries, Firebase modes,
the independent function, tests, OpenAPI, Just recipes, lint policy, workflows, container behavior, repository-local
skills, the GitHub security-review profile, and every current Markdown guide. Historical plan files remain unchanged.

Issues found and fixed in this pass:

- Firestore integration tests no longer discard fixture-creation failures or use a timing sleep to infer timestamp
  ordering. A shared test helper fails at the setup boundary, and cleanup reports Firestore client close failures.
- Firebase emulator helpers now reuse the bounded non-success diagnostic path when creating users, so response-read
  failures are no longer discarded. Remaining Firebase client test cleanup also reports close failures.
- OpenAPI semantic tests now cover every registered operation's exact response statuses and bearer-security contract,
  require JSON/CBOR and Problem Details media-type pairs, reject missing or extra paths and operations, and retain the
  existing unique-operation-ID and schema-link checks. Live `/v1/openapi.json` inspection confirmed the expected
  contract, including reachable 408 responses for request-body read timeouts.
- Automatic and manual labeler workflows no longer request `issues: write`. All referenced labels were verified to
  exist, so `contents: read` and `pull-requests: write` are sufficient for the configured behavior.
- The `openapi-contract` skill now requires exact whole-contract semantic tests. All five skill frontmatter files and
  `agents/openai.yaml` documents pass the canonical skill validator and metadata checks.
- AGENTS now states accurately that emulator tests configure fixed endpoints themselves rather than depending on
  `.env`, and its Firestore example reports client-close failures. README was checked in full against current routes,
  configuration, commands, deployment guidance, and architecture; no README correction was needed.
- The clean fast-suite coverage record was corrected from the environment-dependent 79.0% checkpoint to the
  reproducible emulator-free 74.7%. Required emulator-backed coverage remains 86.8%; no arbitrary percentage gate was
  added because the explicit integration, race, contract, vulnerability, and final-image gates are stronger evidence.

Final-pass validation: `just check`, `just test -- -shuffle=on -count=3`, `just test-race`, `just test-integration-ci`,
`just functions-smoke`, `just tidy-check`, `just modernize-check`, `just workflow-check`, `just vuln`, `just coverage`,
`just container-smoke`, README recipe dry-runs, YAML parsing, all five skill validations, skill metadata checks, and
`git diff --check` passed. No production deployment or live Firebase project was changed. Hosted GitHub workflow
execution remains the only validation that requires a pushed branch or pull request.

## Original review verdict

The following verdict records the pre-implementation state at `c5991fe`. It is retained as the rationale for the changes and is superseded by the implementation result at the end of this file.

The repository has a good base: typed Huma operations, strict request-body schemas, RFC 9457 errors, JSON and CBOR responses, request-scoped structured logging, Firebase token revocation checks, transaction-safe Firestore mutations, server timeouts, a non-root container, and broad unit coverage. Root `just build`, `just test`, `just lint`, and `just vuln` pass. The root coverage run reports 76.5%, and live GitHub checks for build, lint, CodeQL, and dependency graph are green at the reviewed commit.

It is not yet a solid reference implementation. The green CI result excludes the independent `functions/` module and silently skips every Firebase emulator test. The Firebase configuration advertises a Go deployment path the Firebase CLI does not support, and the function source is not at the module root required by Cloud Run source deployment. The generated and runtime API contracts have verified inconsistencies, production test doubles are committed as ordinary source, startup lifecycle code bypasses cleanup on fatal paths, and the container currently loses its supplied build version because `ARG VERSION` is out of scope in the builder stage.

There are no known reachable vulnerabilities in either module under Go 1.26.5. Root `govulncheck` reports one advisory in a required module, but no imported reachable symbol. The function scan reports no vulnerabilities. The function result was obtained manually through the root Justfile because current repository automation does not scan that module.

## Constraints and decisions

- Keep `functions/` as a separate Go module and keep its `Hello` implementation intentionally small. Do not add Huma, Firebase Admin, the application observability stack, or a shared cross-module package to this example.
- Duplication of the timestamp layout in the independent function is acceptable. Coupling the modules to remove one constant would make the example worse.
- Keep Chi plus `humachi`. Replacing Chi, adopting `humacli`, or moving all routes into Huma does not solve a verified problem.
- Keep `/health` as a cheap liveness endpoint. Do not add dependency probes until a deployment actually needs application-controlled readiness semantics.
- Keep GitHub repository and tag endpoints intentionally capped at 30 results. Activity already demonstrates upstream cursor pagination; implementing a second pagination strategy adds little teaching value.
- Do not add OpenTelemetry in this pass. `huma-observability` already establishes the intended logging/request-context boundary; real tracing should be a separate product decision with an explicit dependency review.
- Do not add an in-memory rate limiter. It would be per-instance and misleading on Cloud Run. Public deployment controls belong in Cloud Run/IAM/API Gateway/Cloud Armor unless the application gets a concrete identity-aware quota requirement.
- Do not adopt `huma.NewGroup` in this pass. A group would relocate or complicate the existing versioned docs/schema layout. An explicit API prefix is smaller and removes the current metadata-derived prefix without changing routes.
- Do not add new runtime dependencies. Proposed simplifications use the standard library, Huma, and `fxamacker/cbor`, which are already direct dependencies.
- Go `tool` directives for development tooling change `go.mod`/`go.sum`; this implementation was explicitly authorized and adds the entries required by repository automation.
- Treat this as a pre-stable playground API. Correct `firstname`/`lastname` now rather than preserve a known inconsistent contract. No production deployment or migration workflow is present in the repository, and `.firebaserc` points to `demo-test-project`.
- Align tooling and policy with established Go example-project practices where the repository shapes match: explicit two-module recipes, `GOWORK=off` for the nested module, a real Functions Framework target smoke, required emulators in CI, stable aggregate `ci`/`lint` jobs, exact action release tags, quarterly grouped Dependabot updates with a one-day version-update cooldown, typed startup configuration, and final-image probes. Do not copy framework-specific Swagger generation, response-only CBOR, middleware, or generated-spec embedding; Huma owns those concerns differently.

## Original review verification

This section is the pre-implementation evidence gathered from `c5991fe`, not the final result.

- Compared a completed modernization process and inspected its function layout, Functions Framework runner, Justfile, workflows, Dependabot policy, Firebase configuration, auth classification, startup lifecycle, and Dockerfile where transferability was not obvious.
- `just build`: pass.
- `just test`: pass for all root-module packages.
- `just lint`: pass, zero issues.
- `just coverage`: pass, 76.5% total root-module statement coverage.
- `just vuln`: pass, zero reachable vulnerabilities; one required-module advisory is unreachable.
- Function module, invoked through the root Justfile with `functions/` as the working directory:
  - `build`: pass.
  - `test`: pass, but `functions/hello` reports `[no test files]`.
  - `lint`: pass, zero issues.
  - `vuln`: pass, no vulnerabilities found.
- Firebase Auth and Firestore emulators were unavailable. All real verifier and Firestore CRUD tests skipped; coverage for `FirebaseVerifier.Verify`, `InitializeClients`, and `FirestoreStore` CRUD was 0% in this run.
- Live runtime checks:
  - `/v1/openapi.json`: 200.
  - `/v1/openapi`: 404.
  - `/v1/schemas/ErrorModel.json`: 200.
  - `/schemas/ErrorModel.json`: 404.
  - A root Chi 404 advertises `</schemas/ErrorModel.json>` and emits `$schema: http://HOST/schemas/ErrorModel.json`; both point to the verified 404 path.
  - Generated operation responses document `application/problem+json` but not the runtime-supported `application/problem+cbor`.
- Live GitHub state:
  - Build, lint, CodeQL, and dependency-graph runs are green for `c5991fe`.
  - CodeQL default setup already scans Go and Actions weekly; do not add a duplicate CodeQL workflow.
  - The active `CI/CD` ruleset requires job contexts `ci` and `lint`. Preserve these job IDs during workflow refactoring.

## Priority findings

### P0. The documented Go function deployment path is invalid

Evidence:

- `firebase.json` declares `functions.runtime: go126` and a Functions emulator, but Firebase CLI's supported function authoring languages are JavaScript, TypeScript, and Python. Go functions use Cloud Run functions/source deployment instead.
- Cloud Run's Go source contract requires the package containing `init` registration at the module root beside `go.mod`. The current registration is in `functions/hello/function.go`.
- There is no runnable local Functions Framework target, so CI cannot prove that `FUNCTION_TARGET=Hello` is registered in the built module.
- The Storage emulator and `storage.rules` are configured even though the application has no Storage integration.

Failure mode: users follow repository instructions into a deployment that cannot work; a function can compile while its runtime target is unregistered; unused emulator configuration implies features the example does not contain.

Decision:

1. Keep the function separate and simple, but move its one production file to `functions/function.go` at the module root.
2. Add `functions/cmd/server/main.go` which blank-imports the root function package and starts the Functions Framework. This is a production-valid local entry point, not test scaffolding.
3. Add `functions-run` and a bounded `functions-smoke` recipe using `FUNCTION_TARGET=Hello`; CI must start the target, send one request, validate JSON and timestamp, and stop it deterministically.
4. Remove the unsupported `functions` block and Functions emulator from `firebase.json`. Remove the unused Storage emulator and `storage.rules`; keep only Auth, Firestore, and UI emulator configuration.
5. Document the function deployment separately from the service, using Cloud Run source deployment with `--source functions --function Hello --base-image go126`. Keep service deployment on the container-image path.

Acceptance:

- A clean checkout can run the function through the Functions Framework without `go.work`.
- Cloud Run source layout has `functions/go.mod` and `functions/function.go` in the same directory.
- No Firebase CLI command claims to deploy or emulate the Go function.
- Firebase configuration contains only services the repository actually uses.

### P1. CI does not validate the repository as checked out

Evidence:

- Root `go build ./...`, `go test ./...`, `golangci-lint run ./...`, and `govulncheck ./...` stop at the root module boundary.
- `functions/go.mod` defines a second module, but current app workflows never change into `functions/`.
- The local `go.work` includes both modules but is ignored and untracked. A clean checkout does not have it.
- README claims the project uses `go.work`, even though that file is local-only.
- The lint workflow runs `golangci-lint`, but never runs `golangci-lint fmt --diff`; golangci-lint v2 does not enforce configured formatters through `run` alone.

Failure mode: function regressions merge with green required checks; formatting drift also merges despite configured gci/gofumpt/golines rules; local workspace behavior differs from CI.

Decision:

1. Keep `go.work` ignored and explicitly describe it as optional local tooling.
2. Make Just recipes module-aware with explicit app/function recipes and aggregate public recipes.
3. Install a pinned Just release in CI using the installation method documented by the Just project, and invoke only Just recipes after tool setup.
4. Use exact action release tags consistently. This deliberately accepts mutable release tags in exchange for readable, Dependabot-managed workflow updates; retain least-privilege permissions and `persist-credentials: false` as the compensating controls.
5. Preserve `ci` and `lint` as stable aggregate jobs even if implementation work is split into internal app, function, emulator, vulnerability, and container jobs.

Required Justfile shape:

- `build-app`, `build-functions`, and aggregate `build`.
- `test-app`, `test-functions`, and aggregate `test`.
- `test-race-app`, `test-race-functions`, and aggregate `test-race`.
- `lint-app`, `lint-functions`, and aggregate `lint`.
- `fmt-check-app`, `fmt-check-functions`, and aggregate `fmt-check`, using `golangci-lint fmt --diff` and failing when a diff is produced.
- `tidy-app`, `tidy-functions`, and aggregate `tidy`; add `tidy-check` that runs tidy and fails on a Git diff.
- `vuln-app`, `vuln-functions`, and aggregate `vuln`.
- Keep mutation recipes (`fmt`, `fix`, `qa`, `update`) explicit about affecting both modules.
- Avoid hidden reliance on `go.work`; every nested-module command must use `cd functions && GOWORK=off ...`.
- Pin repository tools through Go's `tool` directive where supported, including `govulncheck` and `actionlint`, instead of relying on ambient binaries. The function vulnerability scan may use the root tool with `go tool -modfile=../go.mod govulncheck ./...` so the example does not duplicate tool dependencies.

Acceptance:

- A deliberately broken root function package compile fails required `ci`.
- A deliberately unformatted file in either module fails required `lint`.
- A clean clone without `go.work` passes all aggregate recipes.
- README and AGENTS no longer imply that a tracked workspace exists.

### P1. Firebase integration tests are optional in practice

Evidence:

- Emulator tests call `t.Skip` when ports 7110/7130 are unavailable.
- Current CI does not start Firebase emulators.
- The reviewed coverage run skipped all real Firebase Auth and Firestore operations while still reporting a green suite and 76.5% aggregate coverage.
- Firebase documents `firebase emulators:exec` as the CI-oriented lifecycle command.

Failure mode: changes to Firebase SDK initialization, token verification, transaction semantics, normalization, or error mapping can merge without executing the real integration path.

Decision:

1. Keep developer-friendly local skips, but make the skip decision explicit and observable.
2. Add `REQUIRE_FIREBASE_EMULATORS=1`; when set, `internal/testutil` must fail rather than skip if Auth or Firestore is unavailable.
3. Add `just test-integration-ci` that runs uncached tests through:
   `firebase emulators:exec --only auth,firestore --project demo-test-project 'REQUIRE_FIREBASE_EMULATORS=1 just test-app -- -count=1'`.
4. Keep the emulator lane separate from fast unit jobs, but do not introduce build tags merely to reorganize an already working test suite. The required aggregate `ci` job must depend on the emulator lane.
5. Install a pinned Firebase CLI in required `ci` and use the runner's supported Java version. Do not add emulator caching initially; measure download/runtime cost before adding a cache trust surface.
6. Publish unit and emulator-integration coverage as separate named artifacts and summaries. Do not merge them or quote one aggregate threshold until both reports are stable and representative.
7. Harden `internal/testutil/emulator.go`:
   - use `t.Context()` and a bounded HTTP client;
   - check every emulator REST response status;
   - include a bounded response body in failures;
   - check `json.Marshal` errors instead of discarding them;
   - verify sign-up responses contain a non-empty token and local ID.

Acceptance:

- Required `ci` output shows non-skipped Firebase Auth and Firestore integration tests.
- Killing either emulator makes the integration recipe fail, not skip.
- Ordinary local tests remain runnable without Java, Node, or Firebase CLI and report their skips clearly.

### P1. Chi Problem Details publish broken schema links and diverge from Huma

Evidence:

- Huma is mounted under `/v1`, and its schema registry is reachable at `/v1/schemas/ErrorModel.json`.
- `internal/platform/respond.writeProblem` hardcodes `/schemas/ErrorModel.json` in both `Link` and `$schema`.
- Existing tests assert only that strings contain `/schemas/ErrorModel.json`; they never request the advertised target.
- `respond` implements roughly 170 lines of custom Accept parsing/selection for three Chi-level error paths, creating behavior distinct from Huma's own configured negotiation.
- The OpenAPI CBOR hook copies `application/json` to `application/cbor`, but does not copy `application/problem+json` to `application/problem+cbor`.

Failure mode: clients and editors following schema discovery receive 404; the OpenAPI contract omits a runtime error format; Huma and Chi error negotiation can change independently.

Decision:

1. Make the API prefix explicit in API construction so Huma's schema transformer precomputes `/v1/schemas/ErrorModel.json`.
2. Create a `humachi` context for Chi-level errors and pass `huma.ErrorModel` through the configured API's `Negotiate`, `Transform`, and `Marshal` methods. This delegates `$schema`, `Link`, format selection, and Problem Details content-type filtering to the same Huma v2 pipeline used by operations.
3. Delete the bespoke request-host/proxy-scheme and schema-wrapper logic along with the custom Accept parser. The supported request `Accept` values are `application/json` and `application/cbor`; error responses use Huma's `ContentTypeFilter` result (`application/problem+json` or `application/problem+cbor`).
4. Build the Huma API on a dedicated Chi subrouter before mounting it at `/v1`, allowing the Chi responder to hold the configured `huma.API` without global state.
5. Keep the panic-aware response-writer wrapper and `http.ErrAbortHandler` semantics; those are valid net/http behavior, not duplication.
6. Extend the OpenAPI content hook to copy both success media types and Problem Details media types.
7. Add end-to-end tests that follow every advertised schema link and assert 200 plus the expected schema content type.

Acceptance:

- Root 404, 405, and recovered panic responses advertise a schema URL that returns 200.
- Huma validation errors and Chi errors choose the same format for the same `Accept` header.
- OpenAPI lists both `application/problem+json` and `application/problem+cbor` where the operation can return errors.
- Unsupported `Accept` behavior is deliberate and tested once at the integration boundary.

### P1. Test doubles are production source

Evidence:

- `internal/platform/auth/mock.go`, `internal/service/profile/mock.go`, and `internal/service/github/mock.go` are ordinary non-test files.
- AGENTS explicitly says tests belong in `*_test.go` and production code must remain test-agnostic.
- The mocks add more than 300 lines of mutable in-memory behavior plus tests for the mocks themselves.

Failure mode: the example teaches the opposite of its own testability rule, expands the production package API, and spends maintenance effort proving test scaffolding instead of production behavior.

Decision:

1. Delete all three production mock files and their mock-only test files.
2. Define narrow local fakes in the consuming `*_test.go` packages. Prefer function fields for error/result injection over miniature in-memory repositories.
3. For shared full-router tests, create test-local implementations in `cmd/server/main_test.go` and `internal/http/v1/routes/routes_test.go`; do not create an importable production testutil fake package.
4. Keep compile-time interface assertions on production implementations only. A fake proves interface satisfaction by compiling where it is passed.
5. Remove AGENTS examples that direct agents to production `NewMock*` constructors.

Acceptance:

- `rg --glob '!**/*_test.go' 'Mock|TestUser' internal` returns no test scaffolding.
- Handler and wiring tests still cover injected failures and success paths.
- Total test code decreases without reducing behavioral assertions.

### P1. The independent function has no tested input contract

Evidence:

- `functions/hello/function.go` ignores JSON decoder errors.
- Malformed input therefore produces 200 and falls back to query/default data.
- The handler does not define which methods or content types it accepts.
- The module has no tests, and current required CI does not compile it.

Failure mode: a simple example demonstrates silent error acceptance and can regress without any required check.

Decision: keep the example simple and make only bounded corrections.

1. Support `GET` with optional `name` query and `POST` with exactly one JSON object; return 405 with `Allow: GET, POST` for other methods.
2. For POST, require `application/json` while accepting valid parameters such as `charset=utf-8`, cap the body at 1 MiB, disallow unknown fields, and reject malformed JSON, `null`, empty bodies, multiple top-level values, and oversized bodies with explicit 4xx responses.
3. Keep body `name` precedence over query `name`, then default to `World`.
4. Use a package-level handler function registered from `init`; do not add dependency injection, Huma, shared application packages, or observability middleware.
5. Bound `name` to 100 Unicode code points. Do not add normalization or business validation to a greeting example.
6. Add table-driven `httptest` coverage for GET default/query, POST body and precedence, JSON with charset, empty/null/unknown/multiple/malformed/oversized input, overlong Unicode names, unsupported content type, method not allowed, response content type, and timestamp format.
7. Keep the duplicated `RFC3339Millis` constant and document why module independence is more important than eliminating one constant.

Acceptance:

- The root function package has meaningful handler coverage and required CI execution.
- Malformed or oversized JSON never returns 200.
- The registered target is exercised through the actual Functions Framework, not only by calling the handler directly.
- The production function remains one small file unless splitting the test file.

### P1. Startup failure paths bypass cleanup and are not testing real wiring

Evidence:

- `main` calls `logger.Fatal` for configuration/Firebase failures and `os.Exit(1)` for listen failures; these skip deferred cleanup.
- Root coverage reports `main` at 0%.
- `cmd/server/main_test.go` is large, but much of it reconstructs routers and server behavior instead of calling production builders.
- `listenErr` only sends non-`ErrServerClosed` errors, so an unexpected clean `Server.Close` can leave the select waiting for a signal.

Failure mode: logs and clients are not reliably flushed/closed on startup/listen errors, and tests can pass while production wiring changes incorrectly.

Decision:

1. Keep `main` minimal: create logger, call `run`, log the returned error, sync, and exit once.
2. Extract small production functions, not a framework:
   - `loadConfig(getenv func(string) string) (Config, error)`;
   - `newRouter(dependencies, config, logger) http.Handler`;
   - `newServer(handler, config) *http.Server`;
   - `serve(ctx, server, logger) error`.
3. Return wrapped errors instead of using `Fatal` below `main`.
4. Make server goroutine always report its terminal result; normalize `http.ErrServerClosed` in the coordinator.
5. On shutdown timeout, call `Server.Close` and return a joined/wrapped error if forced close also fails.
6. Use Go 1.26 `signal.NotifyContext` cancellation cause in the process entry point, but inject a context into `serve` so tests need no real OS signals.
7. Test the production router for docs, schemas, middleware order, auth wiring, access-log split, and CBOR OpenAPI content. Remove duplicate synthetic setup where a narrower package test already covers behavior.
8. Add an application request deadline shorter than `http.Server.WriteTimeout`; a write timeout alone does not cancel dependency work. Preserve `context.Canceled`, map dependency deadline exhaustion deliberately, and test a handler that blocks until its request context is canceled.

Acceptance:

- No `Fatal`, panic, or `os.Exit` below `main`.
- Production router and server constructors have direct tests.
- Startup, listen error, graceful shutdown, and forced shutdown tests do not use real process signals.
- Logger sync and Firebase close run on all non-abrupt exits.
- Requests cannot continue Firebase or GitHub work indefinitely after the response budget is exhausted.

## P2 implementation improvements

### Make the Huma/OpenAPI contract explicit

Current operations mostly rely on a generic `default` error response. That hides actual statuses produced by Huma validation, authentication middleware, services, and upstream mapping.

Implement:

- Set `cfg.RejectUnknownQueryParameters = true`; add 422 tests for misspelled query keys.
- Add exact `Operation.Errors` lists per explicit operation. Include automatic request errors where relevant (400 parse, 408 body timeout, 413 body limit, 415 media type, 422 validation) plus handler/middleware statuses (401, 403, 404, 409, 429, 500, 502, 503).
- Add boundary tests proving Huma's actual request contract for JSON and CBOR: parameterized content types, unknown fields, `null`, empty bodies, multiple top-level values, malformed payloads, and the configured body limit. Use Huma's decoder/validation pipeline; do not copy Echo's manual JSON decoder into Huma handlers.
- Keep the hello GET shorthand and document it as the intentional Huma convenience-registration example; POST and resource operations remain explicit metadata examples.
- Add top-level API description, license, and stable tag descriptions. Do not add fake contact details.
- Add a focused generated-spec test for operation IDs, security requirements, status codes, response headers, JSON/CBOR media types, schema names, and property names. Prefer targeted assertions over a brittle full-file snapshot.
- Correct docs to say Huma's default renderer is Stoplight Elements, not Swagger UI.
- Correct all OpenAPI URLs to `/v1/openapi.json` and `/v1/openapi.yaml`. Do not add a redirect unless a real consumer needs the bare path.

### Use one explicit API prefix

`routes.apiPrefix` parses the first OpenAPI server URL to build runtime links, while profile creation hardcodes `/v1/profile`.

Implement:

- Define `/v1` once in application configuration.
- Use it to mount the API subrouter, populate `cfg.Servers`, build schema and pagination links, and create the profile `Location` header.
- Pass the prefix explicitly to registration/helpers that create links.
- Delete `routes.apiPrefix` and its metadata parsing.
- Test the router under a non-default prefix in one focused constructor test so links cannot silently hardcode `/v1` again.

### Align endpoint and middleware semantics

- Change `POST /hello` from 201 to 200 and rename its summary from “Create” to “Generate.” It does not persist a resource and has no `Location`; profile creation already demonstrates a correct 201 response.
- Remove redundant `required:"true"` tags from request-body object fields. Huma body fields are required by default unless `omitempty`/`omitzero` makes them optional; reserve `required` for header/query/cookie parameters as documented by Huma.
- Remove `default` tags from required GitHub path parameters. Keep `example` values for the docs UI; a path segment cannot be omitted to receive a runtime default.
- Make Firebase middleware enforce the named `bearerAuth` scheme rather than treating every non-empty OpenAPI security requirement as Firebase auth. Test optional-security and unrelated-scheme operations so future schemes cannot be intercepted accidentally.
- Check and log `huma.WriteErr` failures in auth middleware. A response-writer failure cannot be repaired, but silently discarding it hides the transport failure.
- Use `obs.DefaultStatusLevel` with `zap.Logger.Log` for Chi-only access logs so 4xx/5xx severity matches Huma access logs.
- Correct README's “real IP detection” claim. `ClientIPFromRemoteAddr` safely records the direct peer; it does not trust or resolve proxy forwarding headers. Do not add `RealIP` until the deployment defines trusted proxies.
- Remove `X-CSRF-Token` from CORS allowed headers while authentication is bearer-only and credentials are disabled; keeping unused CSRF vocabulary implies a cookie flow that does not exist.

Acceptance:

- POST hello returns 200 in JSON and CBOR and OpenAPI documents 200.
- Generated schemas keep required body properties without redundant tags.
- Chi and Huma access logs use the same status-to-level mapping.
- An operation secured by a non-Firebase scheme is not silently processed as Firebase bearer auth.

### Correct profile API and storage naming now

The repository requires camelCase JSON and snake_case Firestore, but profile uses `firstname`/`lastname` in both layers.

Implement in one deliberate breaking change:

- JSON: `firstName`, `lastName`, `phoneNumber`, `createdAt`, `updatedAt`.
- Go: `FirstName`, `LastName` across HTTP and service models.
- Firestore: `first_name`, `last_name`, `phone_number`, `created_at`, `updated_at`.
- Remove `terms` instead of renaming it. A context-free boolean teaches a misleading consent model; a real consent record needs at least policy/version identity and an acceptance timestamp. Add that model only when the example has an actual policy workflow.
- Reject whitespace-only names. Keep built-in Huma validation visible in OpenAPI; do not silently trim names.
- Rename the user-supplied field to `contactEmail` in JSON/Go and `contact_email` in Firestore. Document that it is unverified contact data and distinct from the verified Firebase identity claim.
- Remove hidden lowercasing/trimming from the Store contract. Reject surrounding whitespace through Huma validation and preserve accepted values exactly; canonical identity comparison belongs to a separately named field or policy, not an implicit persistence side effect.
- Because repository evidence points to demo-only state, clear/reseed emulator data rather than add a dual-read migration layer. If a real Firestore project is discovered before implementation, stop this item and write a separate migration plan.

Acceptance:

- Generated schema contains only the corrected camelCase properties.
- Firestore integration tests inspect raw stored keys and assert snake_case.
- A whitespace-only name returns 422 with a field-level location.
- Accepted profile values round-trip without undocumented normalization.
- No compatibility aliases or duplicate legacy fields remain in the example.

### Simplify timestamp encoding

`internal/platform/timeutil/time.go` manually implements CBOR text-string framing, including length parsing that the existing CBOR dependency already implements. The 152-line implementation has an 873-line test file.

Implement:

- Preserve the public JSON/CBOR contract: UTC RFC 3339 with fixed millisecond precision.
- Replace manual CBOR major-type and length encoding/decoding with `fxamacker/cbor` marshaling of the formatted string and unmarshaling into a string before `time.Parse`.
- Reject trailing/malformed CBOR through the library rather than maintaining a partial decoder.
- Keep focused JSON, CBOR, round-trip, UTC conversion, zero-value, and invalid-input tests; delete tests that only prove the removed homemade parser.
- Make `Now()` return UTC or remove it if unused after implementation.

Acceptance:

- Wire representation remains a CBOR text string with the documented timestamp.
- Production code and tests shrink materially.
- JSON and CBOR generated schemas still report string/date-time behavior correctly.

### Use the narrowest Firestore write primitives

Current create performs a read transaction before writing, and update rewrites the complete decoded document with `Set`.

Implement:

- Use `DocumentRef.Create` for atomic create-if-absent and map Firestore `AlreadyExists` to `ErrAlreadyExists`. This removes an unnecessary read transaction.
- Keep a transaction for partial update, but write only changed fields plus `updated_at` with `Transaction.Update`. Do not overwrite unknown fields that may have been written by a newer deployment or migration.
- Keep transactional read-before-delete because the API deliberately distinguishes a missing profile with 404.
- Centralize Firestore status-code mapping and wrap unexpected errors with operation context while preserving `errors.Is` behavior for domain sentinels.
- Rename `Service` to `Store` if the package remains only persistence CRUD. Do not imply a business-service layer that does not exist.
- Preserve audit success/failure categories and the intentional request-scoped logger boundary.

Acceptance:

- Concurrent create still yields exactly one success and remaining conflicts.
- Partial update preserves an injected unknown Firestore field.
- Missing update/delete return the same public 404 contract.
- Emulator integration tests cover every write primitive.

### Harden the GitHub upstream client without adding a framework

The client has a total timeout and good status mapping, but it trusts unbounded upstream bodies and closes error responses without bounded draining. Constructor inputs are also not validated.

Implement:

- Make `NewClient` return `(*Client, error)` and validate a non-nil HTTP client plus an absolute HTTP(S) base URL.
- Store a parsed `*url.URL` and resolve escaped endpoint paths against it instead of concatenating strings.
- Limit decoded success/error bodies to a documented upper bound appropriate for 30-item GitHub responses.
- Drain a small bounded amount before closing responses to preserve connection reuse without accepting unbounded work.
- Reject multiple top-level JSON values after the first decoded value.
- Preserve context cancellation and the existing 10-second total client timeout.
- Classify Firebase Admin errors with the same discipline: known credential/token failures remain 401, while certificate fetch, unavailable/internal/unknown service failures, and deadline exhaustion are not mislabeled as bad credentials. Wrap causes so `errors.Is` and `context.Cause` remain useful.
- Keep GitHub response details out of downstream 5xx bodies and logs; retain only safe status/rate-limit metadata.
- Add `Errors` metadata and tests for 429 retry headers, 403, 404, malformed JSON, oversized responses, invalid base URL, nil client, and cancellation.

Do not add retries automatically. Retrying rate limits or non-idempotent future operations without a budget/backoff contract would be unsafe. Do not add application caching until cache freshness and deployment topology are defined.

### Stop teaching floating-point money

`items.Item.Price float64` is harmless mock data but is a poor reference model for currency.

Implement:

- Replace `price` with `priceMinor int64` and add `currency string` with an ISO 4217 example such as `USD`. A `priceCents` field silently assumes every currency has two decimal places.
- Update mock data and tests mechanically.
- Document that the integer is in the currency's minor unit; keep the static example constrained to a small known set rather than introducing a currency library.
- Do not add a decimal dependency for static demonstration data.

### Make HTTP cache and browser policies route-aware

The security middleware sets `Cache-Control: no-store` globally and completely skips all headers for any path with the `/v1/api-docs` prefix. The prefix test also skips lookalike paths such as `/v1/api-docs-anything`.

Implement:

- Replace prefix skipping with exact docs-path matching plus intentional asset/spec subpaths.
- Always set `X-Content-Type-Options`, `Referrer-Policy`, `X-Frame-Options`/`frame-ancestors`, and permissions policies where applicable.
- Use a docs-specific CSP that permits only the renderer's required sources. Do not bypass all headers.
- Use `Referrer-Policy: no-referrer` for API responses.
- Do not set HSTS in the application by default. Cloud Run terminates TLS and local development uses HTTP; document HSTS at the trusted edge/custom domain.
- Move cache policy out of generic security middleware:
  - profile and Problem Details: `no-store`;
  - health: `no-store`;
  - docs HTML, schema, and OpenAPI documents: `no-cache` so clients may store but must revalidate;
  - public demo GETs: `no-cache` until a freshness contract and shared cache exist.
- Make CORS origins configuration-driven. Wildcard is allowed only in explicit development/demo mode; production startup must reject an empty or wildcard allowlist if bearer-authenticated profile routes are enabled.
- Keep `AllowCredentials` disabled.

Acceptance:

- Docs load successfully under their CSP.
- A lookalike docs path receives normal API security headers.
- Profile and error responses remain non-cacheable.
- CORS tests cover development wildcard, explicit production origins, disallowed origins, preflight, exposed headers, and invalid configuration.

### Make environment configuration truthful

README and `.env.example` list variables not consumed by the server, including `HOST`, `LOG_LEVEL`, and `APP_URL`.

Implement:

- Add a small typed config loader using only the standard library.
- Support `PORT`, `HOST`, `APP_ENVIRONMENT`, `FIREBASE_PROJECT_ID`, `GITHUB_TOKEN`, and `LOG_LEVEL` only if each is actually used.
- Parse `LOG_LEVEL` into Zap's level and pass it to `obs.NewLogger`.
- Remove `APP_URL` unless canonical external URL generation is implemented. Relative schema links remove the current need.
- Remove any other unused variables from `.env.example`, README, and agent guidance.
- Validate port/host, environment enum, required production Firebase project ID, and production CORS origins before creating external clients.
- Treat Firebase mode as one validated unit:
  - production rejects `demo-*` project IDs and any Auth/Firestore emulator host;
  - development accepts both emulator hosts together, never a partial pair;
  - development may start public routes without Firebase only through an explicit offline mode, with protected/profile operations returning 503 rather than accepting unsigned tokens or panicking;
  - never infer production-safe behavior merely from a project-ID prefix.
- Never include the GitHub token value in formatted config or logs.

### Correct Cloud Run/container documentation

The current Dockerfile builds a complete distroless image. README then deploys that image with `--base-image go126 --automatic-updates`, but Cloud Run automatic base-image updates require an application image built on `scratch` or a compatible source/buildpacks flow. Compiled Go applications still require rebuilding for Go standard-library security fixes.

The build also declares `ARG VERSION=dev` before the first `FROM` but does not redeclare it in the builder stage. Docker therefore expands `${VERSION}` to an empty string in the `go build -ldflags` command even when the caller supplies a version.

Implement:

- Keep the existing distroless Dockerfile as the portable container path.
- Deploy it with `gcloud run deploy --image ...` without `--base-image` or `--automatic-updates`.
- Remove the incompatible automatic-update commands and comments from README/Dockerfile.
- State that Go/security dependency updates require rebuilding and redeploying the binary.
- Redeclare `ARG VERSION=dev` immediately after the builder `FROM`; remove verbose `go mod download -x`; add OCI source, revision, and version labels.
- Replace BuildKit-only bind/cache mounts with the conventional `COPY go.mod go.sum` / `go mod download` / `COPY . .` sequence. The image remains multi-stage and cacheable while also building on ordinary Docker-compatible builders; do not claim portability while requiring a BuildKit frontend.
- Pin builder/runtime images to immutable digests while retaining readable tags in comments or Dependabot metadata.
- Add a Docker ecosystem entry to Dependabot so digest updates are automated.
- Add a CI final-image smoke job. Build with a sentinel version, run as the configured non-root UID, start in explicit development/offline mode or against required emulators, probe `/health`, Huma docs, OpenAPI, bearer security metadata, and the exposed/logged version, then stop deterministically. Assert matching OCI labels.
- Keep `.dockerignore`; it already excludes `.env`, `go.work`, plans, function sources, and build artifacts. Add a regression test or documented checklist if its scope changes.

Do not document source deployment with automatic updates unless the repository adds and verifies a distinct buildpacks/scratch deployment artifact.

## P3 cleanup and documentation

### Reduce test duplication while preserving behavior

- Use Huma's `humatest` package for operation-level handler tests.
- Keep a smaller number of real Chi/humachi tests for middleware ordering, mounts, 404/405/recovery, request IDs, access-log ownership, docs, schemas, and content negotiation.
- Convert repeated status-mapping and validation cases to table-driven tests.
- Add fuzz seed tests for cursor decoding, GitHub Link parsing, and any retained custom header parsing. Standard `just test` executes the seeds; sustained fuzzing can remain manual.
- Use `t.Context()` for requests and external test helpers under Go 1.26.
- Avoid chasing a higher global percentage by testing declarations, mocks, or trivial getters. Coverage should expose missing production paths, not become the design target.

### Align repository guidance

- Update README, AGENTS, `.github/agents`, and `.agents/skills` after implementation.
- Fix stale package references: `internal/pagination` -> `internal/platform/pagination`, `internal/respond` -> `internal/platform/respond`.
- Remove guidance that recommends production `Mock*` implementations.
- Document `go.work` as optional and local-only.
- Document exact JSON/CBOR Accept behavior and Problem Details response content types.
- Correct the API renderer name and OpenAPI URLs.
- Add the separate function's build/test/lint commands.
- Update `.github/labeler.yml` so documentation includes all tracked Markdown/guidance files and tooling changes receive an appropriate label.
- Remove `.vscode/settings.json`'s `unusedwrite: false` override unless a current reproducible gopls false positive is documented.
- Keep every older file in `plans/` unchanged as historical evidence. Refer to this file as the current implementation baseline without editing, annotating, archiving, or deleting the historical plans.

### Tighten GitHub Actions safely

- Reduce app build/lint workflows to `contents: read`; add only `actions: write` if artifact upload demonstrably requires it under current GitHub permissions.
- Remove unused `issues: write`/`pull-requests: write` from non-mutating workflows.
- Use exact release tags for actions consistently and let Dependabot update them. Do not mix floating majors, exact tags, and SHA pins without an explicit repository-wide policy change.
- Set `persist-credentials: false` on checkout in non-publishing jobs.
- Validate workflows with pinned `actionlint`, parse YAML, dry-run/list Just recipes, run `git diff --check`, and search for stale paths as part of the lint contract.
- Keep the existing repository-managed CodeQL and dependency graph; do not duplicate them.
- Add official Go `govulncheck` coverage for both modules on dependency changes and a weekly schedule. This complements CodeQL because it performs reachable-symbol vulnerability analysis.
- Add GitHub dependency review for pull requests; this public repository already has the dependency graph enabled.
- Keep the existing quarterly grouped cadence for root Go, function Go, Actions, and Docker. Add a one-day cooldown for routine version updates while leaving security updates immediate.
- Keep Dependabot auto-merge limited to patch/minor updates and dependent on the existing required `ci`/`lint` checks. Do not auto-merge majors.

## Implementation sequence

Each phase is independently reviewable. Do not combine all changes into one pull request.

### Phase 1: Establish truthful required checks

1. Correct the function source/deployment layout and remove unsupported Firebase function/storage configuration.
2. Make Just recipes explicitly cover both modules with `GOWORK=off`, including a real function target smoke.
3. Add formatting, tidy, workflow, and tool checks.
4. Add the strict but intentionally small function contract and tests.
5. Update required `ci`/`lint` aggregate workflows without renaming their job IDs.
6. Add mandatory Firebase emulator integration and final-container lanes.
7. Reduce workflow permissions and standardize exact action release tags.

Exit gate:

- Clean clone, no `go.work`: aggregate build/test/lint/fmt-check pass.
- Required CI runs both modules and non-skipped emulator tests.
- The Functions Framework target and final image are exercised, including sentinel version injection.
- `just test-race` passes.

### Phase 2: Fix runtime/API correctness

1. Extract production config/router/server/serve functions.
2. Add validated Firebase modes and a bounded application request deadline.
3. Introduce one explicit API prefix.
4. Fix Chi schema links and reuse Huma negotiation/marshaling.
5. Complete OpenAPI JSON/CBOR and explicit error metadata.
6. Correct profile API/Firestore names, remove fake consent and hidden normalization, and tighten validation.
7. Add generated-contract and follow-the-link tests.

Exit gate:

- All advertised docs/spec/schema URLs return 200.
- OpenAPI matches observed JSON/CBOR and status behavior.
- No fatal/exit calls below `main`.

### Phase 3: Remove accidental complexity

1. Remove production mocks and replace them with test-local fakes.
2. Simplify `timeutil` CBOR using the existing library.
3. Consolidate handler tests with `humatest` while keeping boundary integration tests.
4. Apply Go 1.26 `go fix` modernizers to both modules, review every semantic diff, then run format/build/test/race/lint/vulnerability gates.
5. Correct `price` to an integer minor-unit amount plus currency.

Exit gate:

- Production LOC and test duplication decrease.
- Behavior and generated schemas remain intentional.
- No new dependency is introduced.

### Phase 4: Production-facing policy and docs

1. Make CORS/security/cache policies explicit by environment and route class.
2. Harden the GitHub client boundary.
3. Correct Cloud Run/container guidance and add container verification.
4. Align README, AGENTS, agent prompts, and skills.
5. Confirm current README/AGENTS guidance follows this baseline while historical plan files remain untouched.

Exit gate:

- Documentation commands work from a clean clone.
- Portable container and deployment instructions match the actual artifact.
- No documented environment variable is inert.

## Final verification matrix

Run through Just recipes only after implementation:

- `just build`
- `just test`
- `just test-integration-ci`
- `just functions-smoke`
- `just test-race`
- `just fmt-check`
- `just lint`
- `just tidy-check`
- `just vuln`
- `just coverage`
- `just container-build`
- `just container-smoke`

Also verify:

- `git status --short` contains only intended changes and no coverage/emulator artifacts.
- `curl` checks for `/health`, `/v1/api-docs`, `/v1/openapi.json`, `/v1/openapi.yaml`, and every emitted schema link.
- JSON and CBOR success plus validation, auth, 404, 405, panic, rate-limit, and upstream-error responses.
- OpenAPI 3.1 generated document parses and contains expected operations/status/media types/security.
- Required GitHub job contexts remain `ci` and `lint` and both pass on a pull request.
- CodeQL/default dependency graph remain enabled and non-duplicated.

## Proposals deliberately not included

- No framework/router replacement.
- No shared module between the app and Cloud Function.
- No Huma or observability middleware in the simple function.
- No repository-wide service container or dependency-injection framework.
- No OpenTelemetry dependency in this pass.
- No in-process distributed rate limiting or cache.
- No readiness endpoint without a deployment requirement.
- No Huma auto-patch or auto-registration conversion.
- No decimal package for static item examples.
- No embedded/generated OpenAPI artifact. Huma's runtime-generated OpenAPI remains the source of truth and receives semantic contract tests instead.
- No copying Echo's response-only CBOR behavior; this Huma example intentionally keeps negotiated JSON and CBOR request/response support.
- No arbitrary coverage threshold until integration coverage is mandatory and representative.
- No duplicate CodeQL workflow.

## Primary references

- [Huma features and defaults](https://huma.rocks/features/)
- [Huma operation model](https://huma.rocks/features/operations/)
- [Huma request validation and unknown query parameters](https://huma.rocks/features/request-validation/)
- [Huma request limits](https://huma.rocks/features/request-limits/)
- [Huma response errors](https://huma.rocks/features/response-errors/)
- [Huma generated OpenAPI and configured paths](https://huma.rocks/features/openapi-generation/)
- [Huma test utilities](https://huma.rocks/features/test-utilities/)
- [Go 1.26 release notes and modernized `go fix`](https://go.dev/doc/go1.26)
- [Go multi-module workspace behavior](https://go.dev/doc/tutorial/workspaces)
- [Go vulnerability management](https://go.dev/doc/security/vuln/)
- [Go security best practices](https://go.dev/doc/security/best-practices)
- [Firebase Emulator Suite installation and CI guidance](https://firebase.google.com/docs/emulator-suite/install_and_configure)
- [Firebase Functions supported language setup](https://firebase.google.com/docs/functions/get-started)
- [GitHub Actions dependency caching and cache trust](https://docs.github.com/en/actions/concepts/workflows-and-actions/dependency-caching)
- [GitHub Actions secure-use guidance](https://docs.github.com/en/actions/reference/security/secure-use)
- [GitHub account username character and normalization rules](https://docs.github.com/en/enterprise-cloud@latest/admin/managing-iam/iam-configuration-reference/username-considerations-for-external-authentication)
- [Dependabot version-update cooldown](https://docs.github.com/en/code-security/dependabot/dependabot-version-updates/configuration-options-for-the-dependabot.yml-file#cooldown)
- [Golangci-lint v2 CLI behavior for `run` and `fmt`](https://golangci-lint.run/docs/configuration/cli/)
- [Cloud Run automatic base image update requirements](https://docs.cloud.google.com/run/docs/configuring/services/automatic-base-image-updates)
- [Cloud Run Go 1.26 runtime](https://docs.cloud.google.com/run/docs/runtimes/go)
- [Cloud Run function source layout for Go](https://docs.cloud.google.com/run/docs/write-functions)
- [Deploy a Cloud Run function from source](https://docs.cloud.google.com/run/docs/deploy-functions)

## Implementation result

Completed on 2026-07-12 against the reviewed `c5991fe` baseline. The repository now provides one explicit application composition path, one independent and runnable function example, Huma-owned error negotiation, required emulator coverage in CI, and module-complete Just automation. The resulting implementation and test diff remains a substantial net deletion despite adding configuration, composition, integration, and contract coverage.

Key outcomes:

- Root and function modules build, test, lint, format, tidy, modernize, and scan independently without relying on a local `go.work`.
- Startup configuration rejects unsafe production/offline/emulator/CORS combinations and supports deterministic offline local use.
- Chi and Huma errors share Huma's Problem Details transformation and JSON/CBOR negotiation, including valid schema links and OpenAPI media types.
- Mounted Huma 405 responses derive complete `Allow` headers from the registered Chi route tree, and every OpenAPI component schema route is exercised.
- Firebase authentication distinguishes invalid credentials from dependency failure; protected routes fail closed when authentication is unavailable.
- Firestore CRUD uses create semantics, narrow transactional field updates, and an existence-preconditioned delete; it preserves unrelated document fields, classifies transient dependency failures, and uses a clear camelCase API to snake_case storage boundary.
- Untrusted forwarding headers are removed before Huma or observability runs, preventing forwarded-host schema-link poisoning without inventing a trusted-proxy policy.
- Production mock implementations and their self-tests were replaced by narrow test-local fakes.
- The GitHub client validates its base URL and bounds response reads; the example money and timestamp models now have unambiguous JSON/CBOR contracts.
- CI exposes stable aggregate `ci` and `lint` jobs while separately proving unit, emulator, vulnerability, dependency-review, and final-container behavior.
- The Go function is intentionally small but has a strict bounded HTTP contract, direct handler tests, and an actual Functions Framework smoke path.
- README, AGENTS, `.github/agents/*.md`, and `.agents/skills/*/SKILL.md` guidance describe the implemented repository rather than the historical shape.

Final local evidence:

| Check | Result |
|---|---|
| `just build` | Pass, root and function modules |
| `just test` | Pass, root and function modules |
| `just test -shuffle=on -count=3` | Pass, repeated shuffled root and function tests |
| `just test-integration-ci -- -count=3` | Pass, three complete non-skipped Auth and Firestore emulator executions; final follow-up coverage 86.8% |
| `just functions-smoke` | Pass, registered `Hello` target exercised through Functions Framework |
| `just test-race` | Pass, root and function modules |
| `just fmt-check` | Pass |
| `just lint` | Pass, zero issues in both modules |
| `just tidy-check` | Pass |
| `just workflow-check` | Pass with pinned `actionlint` |
| `just modernize-check` | Pass for both modules |
| `just vuln` | Pass; no reachable vulnerabilities in either module, with one unreachable advisory in a required root module |
| `just coverage` | Pass, 74.7% total root-module statement coverage with emulators stopped |
| `just container-smoke` | Pass with health, API docs, OpenAPI, schema, and build metadata probes |
| Final image inspection | Pass, runtime user `65532:65532`, version `ci-smoke`, and no false OCI revision label |
| `git diff --check` | Pass |

Current-state research also confirmed Go 1.26.5 is the latest stable toolchain, all direct Go dependencies report no available update, and every exact GitHub Action release tag referenced by the workflows exists.

Remaining limitations and deliberate boundaries:

- The updated GitHub workflows have not run on GitHub because this work is local and uncommitted. Required `ci` and `lint` context wiring is statically validated but needs a pushed pull request for hosted proof.
- No live Firebase project or Cloud Run deployment was changed or tested. Local Firebase emulators and the local Functions Framework prove application behavior without expanding this playground task into infrastructure work.
- Root unit coverage is 74.7% with emulators stopped, while the required emulator-backed profile is 86.8%. No arbitrary threshold was added; behavioral boundary, emulator, race, and container checks are the stronger gates for this example.
- Exact action release tags are intentionally readable and Dependabot-managed rather than commit-SHA pinned. Least-privilege permissions and disabled persisted checkout credentials remain the compensating controls.
- The example does not add tracing, distributed rate limiting, readiness dependency probes, a DI framework, or shared application/function packages. Those would add machinery without a demonstrated requirement.
