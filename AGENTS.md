# AGENTS.md

Instructions for coding agents working in this repository.

## Engineering

- Inspect the affected implementation, callers, and tests first. Reuse local
  patterns and keep changes focused, readable, and idiomatic Go.
- State the failure mode before architectural, security, persistence, or
  production-impacting changes. Preserve unrelated work.
- Validate review findings against actual behavior and requirements. Simplify
  unnecessary behavior instead of adding machinery to defend it.
- Keep instructions actionable. Remove obsolete rules, completed-task narration,
  and duplicated onboarding or tutorials. README.md owns contributor setup.
- Report implementation, validation, and remaining risks before completion.

## Commits and pull requests

- Work on task branches, never directly on the default branch.
- Use Conventional Commits and PR titles: `type: imperative description`, with a
  scope only when useful. Mark breaking changes with `!` and a
  `BREAKING CHANGE:` footer explaining the impact.
- Keep each PR focused; explain the problem, resulting behavior, validation,
  and remaining risk. Run `just qa` and `git diff --check` before committing.
- This repository has no CHANGELOG.md; do not introduce one.

## Architecture and contracts

- The root application and `functions/` are independent Go modules. Root
  `./...` does not cover Functions; validate both through the Just recipes.
- Platform packages must not import `internal/http`. Keep the separate
  Functions Framework example independent of Huma and application internals.
- Preserve current public behavior and stable, unique operation IDs. Use
  resource-prefixed input/output types and closed JSON/CBOR bodies with
  camelCase properties and UTC millisecond timestamps. Keep Firestore field
  tags snake_case independently of JSON names.
- Use `internal/platform/portable` for operation errors. Chi-level 404, 405,
  and recovery use `internal/platform/respond` and the configured Huma API.
  Keep generated OpenAPI aligned with runtime behavior.
- Protected operations declare `auth.RequireAuth()`; registration installs
  authentication middleware once. Derive profile ownership from verified UID.
- Firestore create is atomic, updates use transactions, and deletes use an
  existence precondition. Do not replace these with read-then-write sequences.
- Use `obs.Logger(ctx)` for request work and an explicit logger for process or
  background work. Preserve Trace Context Level 1 and one terminal access log.
  Do not log raw paths, tokens, profile data, peer IPs, or user agents. Sanitize
  public dependency errors; log their cause once at the owning boundary.
- Keep offline mode credential-free. Emulators require both local hosts and a
  synthetic `demo-*` project. Live mode requires explicit real credentials.
  Never run cloud or live-data mutations as part of local validation.
- This repository is not deployed; GCP is a possible future target. Keep
  generic deployment examples accurate without claiming deployment readiness.

## Tooling and GitHub automation

- Support only the current stable Go line. Update both modules, the example
  toolchain pin, CI, containers, implementation, and tests together. Review
  official dependency/tool migrations and explain concrete upstream constraints.
- Prefer Just recipes, which load `.env` and its Go toolchain selection.
- Keep an independent pure formatting/lint workflow for early feedback. Keep
  module consistency, audits, tests, builds, and workflow scans in CI.
- Critical: before changing Actions, inspect GitHub Settings > Rules > Rulesets
  in Chrome. Preserve required check names and GitHub App sources or update
  them in Chrome with the change. Keep existing required checks reporting
  until replacement enforcement reaches trunk. Verify actual PR checks and
  final settings in Chrome; API/CLI inspection supplements this verification.
  Never weaken protection to make a PR pass.
- Pin Actions to explicit full release tags and downloaded tools to reviewed
  versions. Keep local and CI actionlint/zizmor selections aligned. Enforce one
  read-only offline pedantic zizmor scan in plain mode, without SARIF uploads.
- Keep auto-merge limited to verified Dependabot minor/patch updates behind
  required checks. Do not add duplicate package filters, enablement triggers,
  state queries, or revocation machinery. Handle old PR cleanup once.
- Keep Dependabot security alerts and automated security updates disabled.
  Preserve scheduled version updates, native audits, and secret protection.
  Do not add replacement alert bots or repeatedly triage unchanged blockers.

## Skills and validation

- Use `.agents/skills/adversarial-testing/SKILL.md` for test work and apply
  `.agents/skills/go-testing/SKILL.md` for Go/Huma conventions.
- Use `.agents/skills/readme-maintenance/SKILL.md` to assess README impact for
  behavior, configuration, command, tooling, CI, or deployment-guidance changes.
- Use the existing endpoint, pagination, and OpenAPI skills for those tasks.
- Exercise production boundaries and credible failure paths. Keep narrow fakes
  in tests; do not add production test flags, backdoors, or mock-only hooks.
- Run focused checks first, then `just qa`. Run race, fuzz, required emulator,
  Functions smoke, and real container checks when their paths are affected.
  Required emulator validation must fail when emulators are unavailable.
- Validate modified skills with the skill-creator validator and check local
  Markdown links. Do not claim checks that were not executed.
