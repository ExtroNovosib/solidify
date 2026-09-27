# Changelog

## Unreleased

## v0.4.0

- Fix `fail_level` in the configuration file being ignored for the exit code; `-fail-level` still takes precedence.
- Fix rule-family suppressions such as `//solidify:ignore SOLID-I reason`, which passed validation but suppressed nothing; a family now matches every check of that rule. Accept the `//solidlint:ignore` spelling and add file-level `//solidlint:ignore-file <ID> <reason>` directives (`solidify:` spellings remain supported).
- Make syntax-only and ill-typed analysis agree with typed analysis on stable checks: `SOLID-O/type-dispatch` applies its `go/`, `encoding/`, `reflect.` and `ocp.allow_dispatch_types` allowlists to syntax sources, and `SOLID-S/large-type` uses the same four-signal rule, severity, and fingerprint in every mode instead of a bare method count.
- `SOLID-O/concrete-parameter` no longer reports parameters whose type is declared in the analyzed package or that are passed by value.
- Add naming-convention keys `srp.orchestrator_suffixes`, `isp.wiring_aggregate_suffixes`, `dip.domain_packages`, `dip.data_bag_suffixes`, and `dip.detail_imports`; defaults are unchanged. Misspelled nested YAML keys now report their line number.
- Key the cache on an explicit list of policy settings, key standard-library and `go.sum`-pinned imports by the Go release and module files instead of walking their exported APIs, run `go env` once per process, and remember the executable digest; `-cache-debug` reports `api_digests`. Entries written by earlier versions are not reused.
- Print text findings relative to the working directory, name the effective profile and check count in the discovered-config notice, and add `-quiet` and a repeatable `-threshold key=value` flag.
- Discover `.solidlint.yml` (now preferred) as well as `.solidify.yml`; a directory holding both is a configuration error.
- Show a check-specific before/after example and legitimate exception for every check in `checks explain`.
- Move `## Unreleased` entries under the new version when publishing (`scripts/rollover-changelog.sh`). CI now pins golangci-lint, govulncheck, and GoReleaser, cancels superseded runs, bounds every job's time, and uploads self-scan SARIF to code scanning.
- Remove unused internal entry points and `check-run.sh`; `go test -short ./...` skips the E2E tests that need network access.
- Fix a stack overflow on self-referential function types such as `type stateFn func(*lexer) stateFn`, and make SRP method order and data-clump reporting independent of package load order so repeated runs produce identical output.
- Qualify only colliding finding identities (`receiver=`, then `occurrence=`) instead of aborting the run; all other fingerprints are unchanged.
- Keep generated declarations in type information so packages using generated code, and their importers, retain typed checks; exclusions re-type-check only affected packages and fall back to loaded types with a warning.
- Detect standard-library packages with the active Go toolchain rather than the GOROOT recorded at build time, which misreported stdlib types in prebuilt binaries.
- Resolve relative patterns such as `./...` from the current directory, and count calls to later-declared methods when computing LCOM4.
- Speed up large packages and cold caches: near-linear data-clump pairing, no per-entry fsync, shared import digests, and constant-time finding ownership. Cache entries are now keyed by the exact executable, and `-cache-debug` no longer changes the cache key.

## v0.3.0

- Invalidate package cache entries when only a dependency's method set changes, and bump the cache format so older entries are not reused.
- Keep `syntax` and `auto` analysis running on ordinary type errors with the syntax-capable checks, report per-package type completeness in `stats`, and keep `-analysis=types` strict.
- Stop expired baseline entries from suppressing findings: an entry expires at the start of the UTC day after its `expires` date, and `-baseline-expired=warn|error` selects the policy.
- Add top-level `help`, before/after examples and legitimate-exception guidance in `checks explain`, and a reasoned coverage status for every check in `stats`.
- Load only the package information the selected checks need, without dependency syntax or type-info bodies.
- Add stable evaluation cases for embedding, generics, adapters, generated files, and platform-specific files, and give the race, E2E, and plugin gates exclusive ownership.
- Update `golang.org/x/tools` to v0.49.0.

## v0.2.0

- Add selection-before-execution plans and structural stats so disabled runner groups do not execute and cache behavior is inspectable.
- Add `check`, `checks list/explain`, `config init/validate/schema`, `stats`, and explicit baseline `init/diff/update/prune` commands while preserving legacy CLI invocation.
- Add annotated baseline v5 writes with v4 read compatibility; newly accepted findings require a review reason, and generic suppression edits are no longer advertised as safe fixes.
- Unify CLI/plugin snapshots, cache package and program groups, and move JSON/SARIF rendering into `internal/report` without changing result JSON v3, fingerprint v4, or SARIF 2.1.0 contracts.
- Add stable-rule evaluation evidence and fast/full/release quality tiers. No check IDs, default severities, or rule heuristics changed.
- Add the `calibration` profile with the high-precision `SOLID-I/consumer-role` and `SOLID-I/unused-dependency` checks, and guarded release publishing scripts.

## v0.1.0

- First automated release of the conservative seven-check stable profile.
- Twenty additional checks remain available through explicit experimental opt-in.
- JSON schema v3, baseline and fingerprint v4, and GolangCI-Lint v2.12.2 integrations.
