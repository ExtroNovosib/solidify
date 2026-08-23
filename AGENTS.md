# AGENTS.md

## Repository overview

- This repository contains `solidlint`, a Go 1.22 CLI and golangci-lint plugin for detecting explainable SOLID design smells.
- Treat findings as heuristics. Preserve stable rule IDs, severities, evidence, source locations, fingerprints, and existing text, JSON, and SARIF behavior unless the task explicitly changes them.
- Read `README.md` and the relevant implementation and tests before changing analyzer behavior.

## Working agreements

- Keep changes focused and consistent with the existing Go style; format modified Go files with `gofmt` or `go fmt`.
- Add or update focused tests alongside behavior changes. Prefer existing test helpers and table-driven patterns.
- Keep deliberate violations in `testdata/violations`, clean examples in `testdata/clean`, and analyzer fixtures under their existing testdata directories.
- When adding or changing a check, update its documentation under `docs/checks/` and preserve configuration, suppression, baseline, and syntax/types/auto analysis semantics.
- When changing JSON or SARIF output, update the corresponding schema and regression tests.
- Do not edit generated files or checked-in baselines unless the requested change requires regenerating them; mention regeneration in the handoff.
- Do not add production dependencies without confirmation.

## Validation

- During development, run the narrowest relevant `go test` command first.
- Run `go test ./...` for general Go changes.
- Run `make precision` when analyzer detection behavior changes.
- Run `make sarif-check schema-check` when JSON or SARIF output changes.
- Run `make check` before pushing or handing off substantial changes when the required local tools are available. Report any skipped or failing checks.

## Code review rules

- Flag changes that can make clean fixtures produce findings or expected violation fixtures lose findings.
- Flag diagnostics that lose portable repository-relative paths, stable fingerprints, or valid JSON/SARIF schema output.
- Flag analyzer changes that unintentionally diverge between syntax-only, types-required, and auto fallback modes.
- Flag suppressions, exclusions, baselines, or cache behavior that can silently hide new findings outside their documented scope.
