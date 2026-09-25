# Changelog

## Unreleased

- Fix a stack overflow on self-referential function types such as `type stateFn func(*lexer) stateFn`, and make SRP method order and data-clump reporting independent of package load order so repeated runs produce identical output.
- Qualify only colliding finding identities (`receiver=`, then `occurrence=`) instead of aborting the run; all other fingerprints are unchanged.
- Keep generated declarations in type information so packages using generated code, and their importers, retain typed checks; exclusions re-type-check only affected packages and fall back to loaded types with a warning.
- Detect standard-library packages with the active Go toolchain rather than the GOROOT recorded at build time, which misreported stdlib types in prebuilt binaries.
- Resolve relative patterns such as `./...` from the current directory, and count calls to later-declared methods when computing LCOM4.
- Speed up large packages and cold caches: near-linear data-clump pairing, no per-entry fsync, shared import digests, and constant-time finding ownership. Cache entries are now keyed by the exact executable, and `-cache-debug` no longer changes the cache key.
- Add selection-before-execution plans and structural stats so disabled runner groups do not execute and cache behavior is inspectable.
- Add `check`, `checks list/explain`, `config init/validate/schema`, `stats`, and explicit baseline `init/diff/update/prune` commands while preserving legacy CLI invocation.
- Add annotated baseline v5 writes with v4 read compatibility; newly accepted findings require a review reason, and generic suppression edits are no longer advertised as safe fixes.
- Unify CLI/plugin snapshots, cache package and program groups, and move JSON/SARIF rendering into `internal/report` without changing result JSON v3, fingerprint v4, or SARIF 2.1.0 contracts.
- Add stable-rule evaluation evidence and fast/full/release quality tiers. No check IDs, default severities, or rule heuristics changed.

## v0.1.0

- First automated release of the conservative seven-check stable profile.
- Twenty additional checks remain available through explicit experimental opt-in.
- JSON schema v3, baseline and fingerprint v4, and GolangCI-Lint v2.12.2 integrations.
