# solidlint v0.1 Full Hardening — Implementation Tracker

This file tracks execution of the user-approved inline plan dated 2026-08-19. The inline plan remains the authoritative specification; this tracker does not narrow or redesign it.

- [x] 1. Lock regressions and existing public-contract characterization before refactoring.
- [x] 2. Add complete rule maturity, scope, syntax capability, surface, and safe-fix metadata.
- [x] 3. Implement stable/all profiles, exact check selection, canonical YAML, and mode enforcement.
- [x] 4. Correct OCP syntax identity, TCC, multi-file LOC, DIP allowlists, interface matching, fixes, and filtering.
- [x] 5. Add registry-driven 27-rule corpus and identity/quality gates.
- [x] 6. Introduce subject/identity, fingerprint and baseline v4, JSON v3, and SARIF v4.
- [x] 7. Extract `internal/baseline`, `internal/report`, `internal/config`, and `internal/cli` with inward dependencies.
- [x] 8. Implement the shared analysis factory, module plugin, and matched-ABI Go plugin.
- [x] 9. Add integration/E2E suites and the canonical Makefile contract.
- [x] 10. Complete check documentation, README, changelog, contribution, security, CI, Dependabot, and releases.
- [x] 11. Pass the required unit, integration, E2E, schema, SARIF, precision, plugin, security, and snapshot gates.
- [x] 12. Perform final architecture, SOLID, correctness, security, and build-stability review and fix findings.

Verification note (2026-08-19): `make check` and the SBOM-enabled GoReleaser
snapshot pass on macOS. The module plugin is exercised in a real custom
GolangCI-Lint v2.12.2 binary locally. The shared-object plugin is necessarily
deferred to the Linux CI job because Go shared plugins cannot be loaded on
macOS; its matched-toolchain build-and-load E2E remains mandatory there.

Implementation constraints:

- Preserve all unrelated staged and uncommitted work; do not reset, revert, commit, or depend on a `HEAD` diff.
- Preserve all 27 concrete IDs and their default severities.
- Go 1.25 and `plugin-module-register` are explicitly approved production changes.
- Playwright remains N/A because this repository has no browser surface.
