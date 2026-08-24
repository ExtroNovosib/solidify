# Changelog

## Unreleased

- Add selection-before-execution plans and structural stats so disabled runner groups do not execute and cache behavior is inspectable.
- Add `check`, `checks list/explain`, `config init/validate/schema`, `stats`, and explicit baseline `init/diff/update/prune` commands while preserving legacy CLI invocation.
- Add annotated baseline v5 writes with v4 read compatibility; newly accepted findings require a review reason, and generic suppression edits are no longer advertised as safe fixes.
- Unify CLI/plugin snapshots, cache package and program groups, and move JSON/SARIF rendering into `internal/report` without changing result JSON v3, fingerprint v4, or SARIF 2.1.0 contracts.
- Add stable-rule evaluation evidence and fast/full/release quality tiers. No check IDs, default severities, or rule heuristics changed.

## v0.1.0

- First automated release of the conservative seven-check stable profile.
- Twenty additional checks remain available through explicit experimental opt-in.
- JSON schema v3, baseline and fingerprint v4, and GolangCI-Lint v2.12.2 integrations.
