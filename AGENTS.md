# AGENTS.md

Instructions for coding agents. Keep execution rules here; `README.md` serves
human users and contributors. Do not duplicate agent instructions into it.

## Engineering priorities

- Prioritize correctness, readability, maintainability, then performance.
- Inspect the implementation, callers, and tests before changing behavior.
  Preserve unrelated work; keep changes focused, readable, and idiomatic Go.
- Prefer the smallest safe change; reuse or refactor local patterns and utilities
  before adding abstractions or dependencies.
- Validate review findings against requirements and reproducible behavior.
  Simplify unnecessary behavior instead of adding machinery to defend it.
- State the failure mode before architectural, security, persistence, or
  production-impacting changes.
- Report implementation, validation, and remaining risks before completion.
- Keep instructions, comments, and documentation concise. Explain and obtain
  explicit approval before removing, merging, or relocating guidance, including
  obsolete rules, completed-task narration, and duplicates.
- Keep public code, instructions, commits, and PR text self-contained; do not
  disclose private coordination repositories, paths, revisions, or discussions.

## Epistemic honesty and scope discipline

- Report defects, corrupted data, unmet requirements, and unfinished work plainly.
  Never conceal them in justifications or relabel them as simplifications,
  provisional scope, or expected limitations. If a problem is deeper than
  expected, stop at a safe boundary and report the exact failure, affected
  behavior or records, and remaining work.
- Counts, checksums, and passing tests show consistency, not semantic correctness.
  Before claiming Huma validation, JSON/CBOR decoding, or Firestore transformations
  work, inspect accepted inputs and resulting responses or profiles using
  synthetic fixtures or authorized data. Hunt for false positives; report the worst counterexamples,
  or describe the sample when none were found.
- Fix the bug class behind cited failures: correct the grammar, validation, query,
  or logic and audit all affected paths or records within the authorized scope.
  Do not hardcode cited values into exceptions or blocklists, or patch only named
  IDs.
- Do not weaken specifications, `AGENTS.md`, or active plans to excuse defects or
  shortcuts. Follow current task scope and stop gates. For review-only or
  report-only work, edit only an explicitly requested report.
- After the final code, schema, or generator change, inspect affected runtime
  OpenAPI, regenerate fixture artifacts, rebuild affected test data, and rerun
  relevant checks. Earlier passing results do not validate later edits.

## Commits and pull requests

- Use [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):
  `type: imperative description`. Add a scope only when
  it materially improves clarity. Types: `feat`, `fix`, `docs`, `test`, `refactor`,
  `perf`, `build`, `ci`, `chore`, and `revert`.
- Use the same format for PR titles, suitable for the final merge commit. Keep PRs
  focused; describe the problem, resulting behavior, validation, and remaining
  risks.
- Mark breaking changes with `!` and a `BREAKING CHANGE:` footer.
- Work on task branches, never directly on the default branch. Before committing,
  run `just qa` and `git diff --check`.
- This repository has no `CHANGELOG.md`; do not create one or require entries.

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
- Keep required `lint` and `ci` checks. Pure formatting/lint runs independently
  for early feedback. Keep module consistency, audits, tests, builds, and workflow scans in CI.
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

- Use `.agents/skills/edit-agent-instructions/SKILL.md` when reviewing or editing
  root or nested agent instructions.
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
