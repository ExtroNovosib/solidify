# Plan: solidlint v0.2 Quality, Efficiency, and Usability Hardening

Plan contract: `create-plan/v1`

## Context & Goal

The released v0.1.0 codebase has strong output contracts and a broad local gate, but the current implementation still pays for checks that are later filtered out, constructs incomplete package identity in the GolangCI bridge, performs cache hashing work even when caching is disabled, and concentrates command orchestration and SARIF rendering in `internal/cli/run.go`. The public baseline and suppression workflows also encourage opaque debt acceptance, while the stable-rule corpus is too small to support confident promotion decisions.

This plan delivers a coherent v0.2 developer-tooling milestone. Done means that CLI and plugin execution share one selection and package-context model; disabled checks do not execute; plugin findings match CLI findings for package-scoped rules; cache work is lazy and measurable; command discovery, configuration validation, statistics, and baseline maintenance are explicit workflows; report/config/baseline responsibilities sit outside the CLI coordinator; stable rules have recorded positive and negative evidence; and the repository has fast, complete, and release gate tiers without weakening the existing authoritative checks.

The milestone preserves the 27 check IDs, severities, fingerprint v4, result JSON v3, SARIF 2.1.0 shape, existing legacy CLI invocation, configuration precedence, syntax/types/auto behavior, repository-relative paths, and the v0.1.0 plugin names. It does not add analyzer heuristics.

## Architecture Analysis

### Current architecture

- `cmd/solidlint/main.go` is the composition root and delegates to `internal/cli.Run`.
- `internal/cli/run.go` currently parses flags, discovers and applies configuration, loads packages, runs analysis, applies baselines, renders text/JSON/SARIF, and decides exit status. Its `Run` and `writeSARIF` functions combine unrelated responsibilities and are themselves reported by the all-profile complex-function check.
- `internal/analyzer/registry.go` owns check metadata and selection, but `internal/analyzer/run.go` schedules all family runners before applying the resolved selection. Runner groups such as `srp-package` can therefore compute experimental checks that the stable profile later discards.
- `internal/analysisapi/factory.go` creates four package analyzers for every configuration, builds snapshots from a short package name, treats a non-nil `TypesInfo` as complete types, and filters findings after each group runner. `internal/analysisapi/isp.go` exposes a second legacy analyzer path, so factory behavior is not the behavior tested by most current bridge tests.
- `internal/analyzer/workspace.go` builds dependency fact strings during every typed load. `internal/analyzer/cache.go` then rereads source to hash it, and program-scoped groups are not cached.
- `internal/analyzer/control.go` and `internal/analyzer/control_yaml.go` repeat threshold keys and validation rules. `internal/config` is currently a forwarding wrapper rather than the owner of configuration introspection.
- `internal/report` forwards JSON encoding while SARIF remains in the CLI package. `internal/baseline` owns a deterministic but opaque v4 fingerprint list.
- `Makefile` has a strong `check` target, but that target repeats overlapping suites and the existing coverage package set does not exercise the real factory/CLI paths proportionally.

### Chosen approach

1. Add an immutable `analyzer.ExecutionPlan`, built from registry metadata, profile, enabled rules/checks, analysis mode, and target surface. It is the authoritative selection used by the CLI runner, package/program schedulers, cache identity, statistics, and plugin factory. Runner groups receive their selected member IDs before execution. The compatibility `Run` entry point remains and delegates to the plan-aware path.
2. Replace positional snapshot construction with a `SnapshotInput` value that carries package path/name, module path, `*types.Package`, type information, type errors, imports, and generated-file state. The plugin bridge populates it from `analysis.Pass.Pkg`, `Pass.Module`, `Pass.TypesInfo`, and `Pass.TypeErrors`. A selected group is represented by an analyzer only when the group has a supported selected member. Group execution still tolerates parse/type errors where metadata permits syntax fallback, but checks marked type-required receive an incomplete snapshot and do not emit.
3. Store compact precomputed source/dependency digests on loaded packages, compute dependency digests only when the execution plan enables cacheable groups that depend on them, and add workspace-level cache entries for program runner groups. Return structured `ExecutionStats` from the plan-aware runner so tests and the `stats` command can prove which groups ran, skipped, hit, or missed without parsing timing logs.
4. Keep the standard-library `flag` package and existing dependencies. Introduce a small command dispatcher in `internal/cli`: a missing command or leading flag continues to mean `check`; explicit `check`, `checks list`, `checks explain`, `config init`, `config validate`, `config schema`, `baseline init`, `baseline diff`, `baseline update`, `baseline prune`, and `stats` route to focused coordinators. Parsing, policy resolution, execution, baseline triage, and rendering become separate files and functions. This improves SRP without adding a CLI framework.
5. Make `internal/report` the machine-rendering owner for JSON and SARIF. Make `internal/config` the introspection/schema owner while keeping analyzer runtime policy types in `internal/analyzer` to avoid an import cycle. Replace repeated threshold switches/lists with a registry of threshold specifications used by apply, validate, suggestion, effective-config, and schema generation.
6. Introduce baseline document v5 with annotated entries (`fingerprint`, check ID, portable path, subject, required reason, optional owner and expiry). Reading v4 remains supported; new writes use v5. Update preserves existing annotations and requires an explicit reason for newly accepted findings; prune removes stale entries only after an explicit command. Remove the generated “accepted for now” suggested fix and mark checks as having a safe fix only when they own a real behavior-preserving source edit.
7. Expand evidence for the seven stable checks through a machine-readable adjudication manifest, focused corpus fixtures, and a checked-in evaluation report. Stable check pages receive real positive/negative Go examples and limitations tied to those cases. Every check page receives the cross-cutting baseline v5 and truthful-fix wording; experimental pages otherwise retain their existing rule guidance.
8. Split gates by intent: `check-fast` for formatting/vet/unit/integration and self-lint, `check` as the full local reliability gate with each suite executed once, and `check-release` for snapshot/external-consumer qualification. CI uses the matching tier. The existing individual targets remain callable.

This follows Go-shaped SOLID boundaries: interfaces are introduced only at the filesystem/clock seams needed for deterministic baseline and cache tests; registry values and concrete structs are returned elsewhere; the CLI composition root wires concrete coordinators; report/config packages depend inward on analyzer contracts; and no DI framework is introduced.

### Alternatives considered

- **Filter findings after execution:** rejected because it preserves the correctness surface but not the efficiency or plugin analyzer-count contract; it also cannot prove that a disabled experimental runner did not run.
- **Patch `SnapshotFromSyntax` with two more positional parameters:** rejected because package/module/type completeness is already a multi-field contract used by two surfaces; a named input struct is safer and easier to extend without argument-order errors.
- **Add Cobra or another CLI framework:** rejected because the command tree is small, compatibility with leading flags is important, and AGENTS.md requires confirmation for production dependencies.
- **Delete baseline v4 support:** rejected because published v0.1.0 users need a compatibility path. The reader supports v4 and v5; explicit update performs the migration.
- **Keep suppression insertion as a universal safe fix but improve its wording:** rejected because accepting debt is a review decision, not a behavior-preserving repair. Baseline and documented manual suppression workflows own that decision.
- **Rewrite all 27 check implementations or documentation pages:** rejected as disproportionate. This milestone strengthens the execution platform and the seven stable rules; experimental rule tuning remains evidence-driven follow-up work.

### Architectural debt touched but not fully removed

The analyzer package still contains the concrete rule implementations and runtime configuration type. Moving each SOLID family into a separate package would create a large internal API cutover without improving the v0.2 outcomes. The plan instead creates stable planning, reporting, and configuration seams that make a later family split possible.

## Out of Scope

- New check IDs, rule heuristics, default severities, maturity promotions, or threshold retuning.
- Fingerprint v5, result JSON v4, SARIF version changes, or changes to repository-relative path semantics.
- A browser UI, language-server protocol, editor extension, hosted service, daemon, or network telemetry.
- JSONL output, automatic source rewrites, and automatic insertion of suppression comments.
- Replacement of the standard-library flag parser or addition of production dependencies.
- Immutable SHA pinning for GitHub Actions, changing Go/GolangCI/GoReleaser versions, publishing v0.2.0, or modifying `.github/workflows/release.yml`; those require a separate release-policy decision and live version verification.
- Adding examples, changing rule-specific guidance, or expanding precision corpora for the 20 experimental checks; their shared baseline/fix wording is updated because those public contracts change globally.
- Editing checked-in generated artifacts outside the two new schemas and the explicit stable evaluation report described below.

## Assumptions & Open Questions

- The legacy form `solidlint [flags] <targets>` remains equivalent to `solidlint check [flags] <targets>`. Existing exit statuses remain: `0` success/no policy failure, `1` findings or baseline diff under the selected policy, and `2` usage/configuration/operational error.
- The published `-write-baseline <path>` flag remains as a deprecated compatibility alias for initializing/updating the target with an explicitly supplied `-baseline-reason`; new documentation uses the baseline subcommands.
- `checks list`, `checks explain`, `config schema`, and `stats` support deterministic JSON for automation; human text remains the default. Existing `-format=json|sarif|text` behavior for check results is unchanged.
- Directory arguments keep the documented recursive behavior for backward compatibility. A `.go` argument is explicitly documented and tested as a package-loading selector, because `go/packages` analyzes its containing package; the misleading undercount tip is removed.
- Baseline v5 reasons are required, trimmed, at least 12 characters, and rejected when equal (case-insensitively) to `accepted for now`, `todo`, `ignore`, or `temporary`. Existing v4 files remain readable without invented annotations and are migrated only by an explicit baseline update/init command with a supplied reason.
- `baseline update` adds current unaccepted findings and preserves existing live annotations. It does not prune stale entries unless `--prune` is supplied. `baseline prune` removes stale entries and leaves live annotations byte-for-byte equivalent after canonical reordering.
- Cache performance acceptance is structural and benchmark-backed, not tied to a wall-clock percentage that would be flaky across machines.
- There are no unresolved product or architecture questions. Implementation freedom remains for private type names and helper decomposition inside the declared files, provided the public contracts and test cases below are preserved.

## Files

- **New:**
  - `internal/analyzer/plan.go` — immutable execution-plan and runner-group selection model.
  - `internal/analyzer/stats.go` — structured execution/cache counters and deterministic stats DTO.
  - `internal/analysisapi/diagnostic.go` — diagnostic conversion moved out of the legacy analyzer declarations.
  - `internal/config/schema.go` — configuration schema generation from analyzer registries.
  - `internal/report/sarif.go` — SARIF renderer moved from the CLI coordinator.
  - `internal/cli/options.go` — compatibility-aware command and flag parsing.
  - `internal/cli/policy.go` — configuration discovery, precedence, selection, and execution-plan resolution.
  - `internal/cli/commands.go` — checks/config/baseline/stats command coordinators.
  - `internal/cli/render.go` — text/JSON/SARIF output and exit-policy handling.
  - `schemas/solidlint-config-v1.schema.json` — checked-in generated configuration schema validated against `config schema` output.
  - `schemas/solidlint-baseline-v5.schema.json` — baseline v5 document schema.
  - `docs/evaluations/stable-v0.2.md` — reproducible stable-profile evaluation summary and adjudication notes.
  - `testdata/evaluation/stable-v0.2.json` — machine-readable case manifest for all stable checks.
  - `testdata/evaluation/positive/stable.go` — additional named positive stable-check cases.
  - `testdata/evaluation/negative/stable.go` — additional named clean lookalikes and boundary cases.
  - `internal/analysisapi/testdata/src/externaliface/go.mod` — plugin parity fixture module.
  - `internal/analysisapi/testdata/src/externaliface/api/api.go` — external interface declaration fixture.
  - `internal/analysisapi/testdata/src/externaliface/consumer/consumer.go` — consumer that must not be reported as a local interface smell.
- **Modified:**
  - `internal/analyzer/types.go` — snapshot/digest fields needed by plan-aware execution without changing Issue/fingerprint contracts.
  - `internal/analyzer/registry.go` — expose runner-group metadata, selected group members, threshold-independent check descriptors, and truthful safe-fix metadata.
  - `internal/analyzer/run.go` — build/delegate to an execution plan, schedule selected groups before execution, and return stats from the plan-aware entry point.
  - `internal/analyzer/export.go` — replace positional snapshot creation with `SnapshotInput` and selected-group execution.
  - `internal/analyzer/workspace.go` — plan-aware load options, correct package/module/type completeness, package semantics for file targets, and lazy compact digests.
  - `internal/analyzer/cache.go` — consume precomputed digests, add program-group entries, and expose counters through `ExecutionStats`.
  - `internal/analyzer/control.go` — authoritative threshold specification registry and baseline-quality-neutral runtime validation.
  - `internal/analyzer/control_yaml.go` — derive known threshold names/suggestions from the registry.
  - `internal/analyzer/fixes.go` — remove universal suppression attachment; retain explicit suppression edit construction for callers that deliberately request it.
  - `internal/analyzer/isp_usage.go` — use full package/module identity from plugin snapshots when deciding whether an interface is external.
  - `internal/analysisapi/factory.go` — create analyzers from selected supported runner groups and populate complete snapshots from `analysis.Pass`.
  - `internal/cli/run.go` — reduce to command dispatch/composition while preserving `Run(args, BuildInfo) int`.
  - `internal/baseline/baseline.go` — v4-compatible reader, v5 annotated model, diff/update/prune operations, deterministic atomic writes, and reason validation.
  - `internal/config/config.go` — expose validation, initialization, threshold metadata, and schema services rather than forwarding YAML calls only.
  - `internal/report/json.go` — own result JSON encoding previously implemented in analyzer while preserving schema v3 bytes.
  - `plugin/solidlint/solidlint.go` — use the factory’s selected analyzer set without double-building during construction.
  - `cmd/solidlint/main.go` — wire the refactored CLI application while preserving build metadata.
  - `Makefile` — add non-overlapping `check-fast`, `check`, and `check-release` tiers; make E2E and schema targets execute the new harnesses.
  - `.github/workflows/ci.yml` — map matrix and hardening jobs to the new gate tiers without changing tool versions.
  - `.solidlint-baseline.json` — migrate the repository’s empty baseline to canonical v5.
  - `schemas/README.md` — document result, SARIF, config, and baseline schema ownership/versioning.
  - `docs/rule-graduation.md` — require the checked-in manifest/report evidence used by stable maturity decisions.
  - `docs/checks/SOLID-S/large-type.md`
  - `docs/checks/SOLID-S/data-clump.md`
  - `docs/checks/SOLID-S/god-type.md`
  - `docs/checks/SOLID-S/low-cohesion-type.md`
  - `docs/checks/SOLID-S/high-fan-out-type.md`
  - `docs/checks/SOLID-S/complex-function.md`
  - `docs/checks/SOLID-S/mixed-input-surface.md`
  - `docs/checks/SOLID-S/flag-argument.md`
  - `docs/checks/SOLID-S/mixed-import-clusters.md`
  - `docs/checks/SOLID-O/type-dispatch.md`
  - `docs/checks/SOLID-O/discriminator-dispatch.md`
  - `docs/checks/SOLID-O/runtime-exhaustiveness.md`
  - `docs/checks/SOLID-O/concrete-parameter.md`
  - `docs/checks/SOLID-O/closed-factory.md`
  - `docs/checks/SOLID-O/implementation-coupling.md`
  - `docs/checks/SOLID-O/parallel-implementations.md`
  - `docs/checks/SOLID-L/non-exact-eof.md`
  - `docs/checks/SOLID-L/nil-embedded-interface.md`
  - `docs/checks/SOLID-I/fat-interface.md`
  - `docs/checks/SOLID-I/usage-ratio.md`
  - `docs/checks/SOLID-I/stub-implementation.md`
  - `docs/checks/SOLID-D/concrete-dependency.md` — replace placeholder prose with real positive/negative Go examples, evidence interpretation, limitations, and links to the v0.2 evaluation cases.
  - `docs/checks/SOLID-D/layer-import.md`
  - `docs/checks/SOLID-D/wiring-outside-root.md`
  - `docs/checks/SOLID-D/hidden-construction.md`
  - `docs/checks/SOLID-D/infra-error-leak.md`
  - `docs/checks/SOLID-D/transport-leak.md` — update the shared fix/baseline wording; experimental pages otherwise retain rule-specific content.
  - `README.md` — command tree, compatibility syntax, file-target semantics, cache/stats interpretation, baseline v5 migration, and gate tiers.
  - `CONTRIBUTING.md` — evidence-manifest and documentation requirements for stable-rule changes.
  - `CHANGELOG.md` — v0.2 unreleased compatibility and migration notes.
  - `SECURITY.md` — remove the stale pre-v0.1.0 statement while retaining the existing disclosure process.
- **Deleted:**
  - `internal/analysisapi/isp.go` — remove the second analyzer construction path after diagnostic helpers move to `internal/analysisapi/diagnostic.go`.
  - `internal/analyzer/json_output.go` — JSON rendering moves to `internal/report/json.go`; analyzer retains the Issue contract only.
- **Tests:** The following exact test artifacts form the owned verification surface.
  - **Unit:** modify `internal/analyzer/registry_test.go`, `internal/analyzer/run_test.go`, `internal/analyzer/cache_test.go`, `internal/analyzer/run_bench_test.go`, `internal/analyzer/control_test.go`, `internal/analyzer/fixes_test.go`, `internal/analyzer/suppression_test.go`, `internal/analysisapi/isp_test.go`, `internal/analysisapi/diagnostic_test.go`, `internal/baseline/baseline_test.go`, `internal/config/config_test.go`, `internal/report/json_test.go`, `internal/cli/run_test.go`, `internal/cli/sarif_test.go`, `plugin/solidlint/solidlint_test.go`, and `cmd/solidlint/main_test.go`; add `internal/analyzer/plan_test.go`, `internal/config/schema_test.go`, `internal/report/sarif_test.go`, and `internal/cli/commands_test.go`; delete `internal/analyzer/json_output_test.go` after moving its schema assertions to report tests.
  - **Integration:** modify `tests/integration/flow_test.go`; add `tests/integration/plugin_parity_test.go` and `tests/integration/cli_workflow_test.go`.
  - **E2E:** add `tests/e2e/cli_test.go` and `tests/e2e/plugin_test.go`; modify `scripts/release-consumer-smoke.sh` to cover the v0.2 compatibility syntax, config validation, annotated baseline, and selected-plugin behavior.
  - **Playwright UI:** None; the repository has no browser runtime, DOM, HTTP application, or Playwright harness.
- **Affected but not modified:**
  - `cmd/solidlint-golangci/main.go`, `.custom-gcl.yml`, `.golangci-plugin.yml`, and `.golangci-go-plugin.yml` continue to consume `analysisapi.NewAnalyzers` without a configuration-shape change.
  - `.solidify.yml` and `.solidlint-enforce.yml` remain valid configuration consumers and are exercised by schema/config validation.
  - `schemas/solidlint-result-v3.schema.json` and `schemas/sarif-schema-2.1.0.json` remain the compatibility authorities for unchanged machine output.
  - `.goreleaser.yml` and `.github/workflows/release.yml` continue to use existing release behavior; release-policy pinning is out of scope.
  - Existing analyzer family implementations (`internal/analyzer/srp*.go`, `internal/analyzer/ocp*.go`, `internal/analyzer/lsp.go`, `internal/analyzer/isp*.go` except `isp_usage.go`, and `internal/analyzer/dip*.go`) remain behavior owners but receive selected group members through the registry adapter rather than heuristic changes.
- **Dependencies:** No production dependency additions, removals, or version updates. The standard library, existing `golang.org/x/tools/go/analysis`, `go/packages`, YAML library, and GolangCI plugin registration package already provide the required boundaries. `go.mod` and `go.sum` must remain unchanged.

## Plan Applicability Verdict

| Promised behavior / failure branch | Trigger and production coordinator | Async/state/render path | Files that must change | Test file and exact case | Verdict |
| --- | --- | --- | --- | --- | --- |
| Only selected runner groups execute, and a plugin exposes only groups with supported selected checks | `internal/analyzer/plan.go:NewExecutionPlan` and `internal/analysisapi/factory.go:NewAnalyzers` | Contributors: `internal/analyzer/registry.go:checkRegistry`, `internal/analyzer/run.go:runPackageScopedChecks`, `internal/analyzer/run.go:runProgramScopedChecks`, `internal/analysisapi/factory.go:NewAnalyzers`; resolved selection -> group membership -> scheduler/factory -> stats/diagnostics | `internal/analyzer/plan.go`, `internal/analyzer/registry.go`, `internal/analyzer/run.go`, `internal/analysisapi/factory.go`, `internal/analyzer/stats.go` | `internal/analyzer/plan_test.go` — `TestExecutionPlanSchedulesSelectedRunnerGroups`; `internal/analysisapi/isp_test.go` — `TestFactoryBuildsSelectedAnalyzerGroups` | PASS |
| No plugin finding is emitted from incomplete type facts for a type-required check, and external interfaces remain external | `internal/analysisapi/factory.go:newPackageAnalyzer` | Contributors: `internal/analysisapi/factory.go:newPackageAnalyzer`, `internal/analyzer/export.go:SnapshotFromSyntax`, `internal/analyzer/isp_usage.go:isExternalInterface`; analysis pass context -> snapshot completeness/module identity -> metadata mode gate -> diagnostic | `internal/analysisapi/factory.go`, `internal/analyzer/export.go`, `internal/analyzer/isp_usage.go`, `internal/analysisapi/testdata/src/externaliface/go.mod`, `internal/analysisapi/testdata/src/externaliface/api/api.go`, `internal/analysisapi/testdata/src/externaliface/consumer/consumer.go` | `tests/integration/plugin_parity_test.go` — `TestPluginAndCLIParityForExternalAndIncompleteTypes` | PASS |
| No generic suppression edit is advertised as a safe fix | `internal/analyzer/fixes.go:AttachDefaultSuppressions` removal | Contributors: `internal/analyzer/registry.go:completeCheckMetadata`, `internal/analyzer/run.go:Run`, `internal/analyzer/export.go:PackageSnapshot`, `internal/analysisapi/diagnostic.go:suggestedFixes`, `internal/report/sarif.go:EncodeSARIF`; finding creation -> truthful fix metadata -> JSON/SARIF/plugin renderer | `internal/analyzer/registry.go`, `internal/analyzer/run.go`, `internal/analyzer/export.go`, `internal/analyzer/fixes.go`, `internal/analysisapi/diagnostic.go`, `internal/report/sarif.go` | `internal/analyzer/fixes_test.go` — `TestFindingsDoNotReceiveGenericSuppressionFix`; `internal/report/sarif_test.go` — `TestSARIFOmitUnownedSuppressionFixes` | PASS |
| Baseline update never discards live annotations; prune only removes stale entries | `internal/cli/commands.go:runBaselineCommand` | Contributors: `internal/baseline/baseline.go:Update`, `internal/baseline/baseline.go:Prune`, `internal/cli/commands.go:runBaselineCommand`; current findings + v4/v5 document -> diff classification -> explicit mutation -> atomic canonical write | `internal/baseline/baseline.go`, `internal/cli/commands.go`, `internal/cli/options.go` | `tests/integration/cli_workflow_test.go` — `TestBaselineUpdatePreservesAnnotationsAndPruneIsExplicit` | PASS |
| No `SOLID-S/complex-function` finding remains in the v0.2 CLI/config/report coordinators under the repository all-profile self-scan | `Makefile:lint` and `internal/cli.Run` | Contributors: `internal/cli/run.go:Run`, `internal/cli/options.go`, `internal/cli/policy.go`, `internal/cli/commands.go`, `internal/cli/render.go`, `internal/report/sarif.go`, `internal/config/schema.go`, `.solidlint-enforce.yml`; command dispatch/render/config ownership -> all-profile scan -> finding set | `internal/cli/run.go`, `internal/cli/options.go`, `internal/cli/policy.go`, `internal/cli/commands.go`, `internal/cli/render.go`, `internal/report/sarif.go`, `internal/config/schema.go`, `Makefile` | `tests/e2e/cli_test.go` — `TestAllProfileSelfScanHasNoCoordinatorComplexFunctionFindings` | PASS |
| Every stable check has recorded positive and negative evidence in the v0.2 manifest and documentation | `internal/analyzer/registry.go:completeCheckMetadata` | Contributors: `internal/analyzer/registry.go:checkRegistry`, `testdata/evaluation/stable-v0.2.json`, `internal/analyzer/precision_test.go`, the seven modified `docs/checks/` pages, `docs/evaluations/stable-v0.2.md`; stable metadata -> manifest membership -> executable fixture -> generated summary/doc link | `internal/analyzer/registry.go`, `testdata/evaluation/stable-v0.2.json`, `testdata/evaluation/positive/stable.go`, `testdata/evaluation/negative/stable.go`, `docs/evaluations/stable-v0.2.md`, `docs/rule-graduation.md`, the seven modified `docs/checks/` pages | `internal/analyzer/precision_test.go` — `TestStableEvaluationManifestCoverageAndVerdicts` | PASS |

## Subtasks

- [x] 1. Introduce the execution-plan and stats contracts in the analyzer registry, preserving the compatibility `Run` entry point and all public finding identities — depends on: none
- [x] 2. Make package and program scheduling consume selected runner groups before execution, and make cache identity use the same plan — depends on: 1
- [x] 3. Replace positional package snapshots with complete named context and make the factory-created plugin analyzers the canonical bridge — depends on: 1, 2
- [x] 4. Make workspace/cache hashing lazy and compact, add program-group cache entries, and expose structural efficiency counters/benchmarks — depends on: 1, 2
- [x] 5. Centralize threshold metadata and add config init/validate/schema services with a checked-in schema — depends on: 1
- [x] 6. Move JSON/SARIF rendering into `internal/report` without changing result JSON v3, SARIF 2.1.0, fingerprints, or portable paths — depends on: 1
- [x] 7. Refactor the CLI into dispatcher, options, policy, commands, and rendering coordinators; add checks discovery and stats while retaining legacy check syntax — depends on: 2, 4, 5, 6
- [x] 8. Implement annotated baseline v5, v4 read compatibility, explicit diff/update/prune workflows, and remove generic suppression fixes — depends on: 6, 7
- [x] 9. Add the stable evaluation manifest/corpus/report and replace placeholder examples for the seven stable check pages — depends on: 2, 8
- [x] 10. Add real integration/E2E parity and workflow harnesses, then de-duplicate Makefile gates and map CI to fast/full tiers — depends on: 3, 4, 5, 6, 7, 8, 9
- [x] 11. Update user/contributor/security/changelog/schema documentation and migrate the repository baseline to v5 — depends on: 8, 9, 10

## Risks & Edge Cases

- **R1 — Runner-group over-selection:** a selected member can share a runner with disabled members; the runner must avoid emitting or doing expensive member-specific work for disabled checks while still executing shared parsing once.
- **R2 — Plugin/CLI context drift:** `analysis.Pass` and `go/packages.Package` expose similar data through different fields. Package path, module path, imports, generated status, and type completeness must normalize to the same snapshot semantics.
- **R3 — Ill-typed package fallback:** `RunDespiteErrors` must not turn partial `TypesInfo` into apparent completeness; syntax-equivalent/conservative checks can run, while type-required checks remain silent and diagnostics preserve valid locations.
- **R4 — Cache invalidation:** compact digests and program caching can return stale results if local source, imported API facts, analysis mode, selection, configuration, exclusions, tests, or tool version are omitted from the key.
- **R5 — Cache-disabled overhead:** loader code can accidentally continue reading dependency export/source files even after execution caching is disabled.
- **R6 — CLI compatibility:** command dispatch can reinterpret a target directory or legacy leading flag as a subcommand, change help/version output, or alter exit codes and broken-pipe handling.
- **R7 — Machine-output drift:** moving JSON/SARIF can change ordering, schema-owned fields, URI bases, fingerprints, related locations, fixes, or version metadata.
- **R8 — Baseline migration/data loss:** v4 compatibility, annotated v5 updates, stale classification, atomic writes, and prune behavior must preserve live debt records and reject malformed or placeholder reasons.
- **R9 — Evidence inflation:** a manifest can claim coverage without reaching the intended check or can place a known violation in the negative set; execution must verify expected check IDs and reject orphan/duplicate cases.
- **R10 — Gate duplication or weakening:** tiering can accidentally omit race, precision, cache, schema, plugin, security, snapshot, or external-consumer evidence, or run the same expensive suite twice in the full tier.
- **R11 — Platform variance:** Go shared plugins remain Linux-specific, path normalization must pass on Windows/macOS/Linux, and E2E tests must use temporary directories without writing into scanned trees.

## Test Strategy

Change classification: non-user-facing

This is a developer-tooling CLI/library/plugin change with no browser, DOM, HTTP application, or end-user web workflow. Unit, integration, and process-level E2E coverage are required because the change crosses internal packages and executable/plugin boundaries. Playwright is technically inapplicable.

| Layer | Required or N/A | Behaviors and risks covered | Test files | Verification command |
| --- | --- | --- | --- | --- |
| Unit | Required | Selection-before-execution, snapshot completeness, cache keys/counters, threshold schema, render compatibility, baseline v5 operations, suppression metadata, command parsing; R1, R3-R9 | `internal/analyzer/plan_test.go`, `internal/analyzer/registry_test.go`, `internal/analyzer/run_test.go`, `internal/analyzer/cache_test.go`, `internal/analyzer/run_bench_test.go`, `internal/analyzer/control_test.go`, `internal/analyzer/fixes_test.go`, `internal/analyzer/suppression_test.go`, `internal/analysisapi/isp_test.go`, `internal/analysisapi/diagnostic_test.go`, `internal/baseline/baseline_test.go`, `internal/config/config_test.go`, `internal/config/schema_test.go`, `internal/report/json_test.go`, `internal/report/sarif_test.go`, `internal/cli/run_test.go`, `internal/cli/sarif_test.go`, `internal/cli/commands_test.go`, `plugin/solidlint/solidlint_test.go`, `cmd/solidlint/main_test.go`, `internal/analyzer/precision_test.go` | `go test ./internal/analyzer ./internal/analysisapi ./internal/baseline ./internal/config ./internal/report ./internal/cli ./plugin/solidlint ./cmd/solidlint -count=1` |
| Integration | Required | CLI/analyzer/report/baseline flow, factory plugin parity, external/incomplete types, config/baseline workflow; R2, R3, R7, R8 | `tests/integration/flow_test.go`, `tests/integration/plugin_parity_test.go`, `tests/integration/cli_workflow_test.go` | `go test ./tests/integration -count=1` |
| E2E | Required | Built CLI legacy/new syntax, self-scan architecture, custom GolangCI module plugin selection, release-consumer compatibility, cross-platform temp isolation; R6, R10, R11 | `tests/e2e/cli_test.go`, `tests/e2e/plugin_test.go`, `scripts/release-consumer-smoke.sh` | `go test ./tests/e2e -count=1` and `make plugin-module-e2e` |
| Playwright UI | N/A: repository has no browser runtime, DOM, HTTP application, or Playwright harness | Browser interaction cannot exercise this Go CLI and GolangCI plugin | None | N/A |

### Coverage Traceability

| Risk or acceptance criterion | Proving layer | Test file | Named test case | Verification command |
| --- | --- | --- | --- | --- |
| R1 — disabled members and groups are not invoked | Unit | `internal/analyzer/plan_test.go` | `TestExecutionPlanSchedulesSelectedRunnerGroups` | `go test ./internal/analyzer -run '^TestExecutionPlanSchedulesSelectedRunnerGroups$' -count=1` |
| R1 — shared runner emits selected members while parsing the group once | Unit | `internal/analyzer/run_test.go` | `TestRunPlanFiltersInsideRunnerGroup` | `go test ./internal/analyzer -run '^TestRunPlanFiltersInsideRunnerGroup$' -count=1` |
| R2 — factory plugin and CLI produce matching package-scoped findings | Integration | `tests/integration/plugin_parity_test.go` | `TestPluginAndCLIParityForExternalAndIncompleteTypes` | `go test ./tests/integration -run '^TestPluginAndCLIParityForExternalAndIncompleteTypes$' -count=1` |
| R3 — partial types do not activate type-required checks | Unit | `internal/analysisapi/isp_test.go` | `TestFactoryRespectsSyntaxSupportOnTypeErrors` | `go test ./internal/analysisapi -run '^TestFactoryRespectsSyntaxSupportOnTypeErrors$' -count=1` |
| R4 — local/import/config/selection changes invalidate package and program entries | Unit | `internal/analyzer/cache_test.go` | `TestCacheIdentityCoversExecutionPlanAndProgramInputs` | `go test ./internal/analyzer -run '^TestCacheIdentityCoversExecutionPlanAndProgramInputs$' -count=1` |
| R5 — cache-disabled loads perform zero dependency-manifest reads | Unit | `internal/analyzer/cache_test.go` | `TestCacheDisabledSkipsDependencyDigestIO` | `go test ./internal/analyzer -run '^TestCacheDisabledSkipsDependencyDigestIO$' -count=1` |
| Structural cache efficiency remains benchmark-visible | Unit | `internal/analyzer/run_bench_test.go` | `BenchmarkRunSelectedCold` | `go test ./internal/analyzer -run '^$' -bench '^BenchmarkRunSelected(Cold|Warm|Disabled)$' -benchmem -count=3` |
| R6 — legacy leading flags and explicit check command are equivalent | E2E | `tests/e2e/cli_test.go` | `TestLegacyAndExplicitCheckSyntaxAreEquivalent` | `go test ./tests/e2e -run '^TestLegacyAndExplicitCheckSyntaxAreEquivalent$' -count=1` |
| File targets use containing-package semantics and do not print the obsolete undercount tip | Unit | `internal/cli/run_test.go` | `TestRunSingleGoFileUsesPackageSemanticsWithoutUndercountTip` | `go test ./internal/cli -run '^TestRunSingleGoFileUsesPackageSemanticsWithoutUndercountTip$' -count=1` |
| R7 — result JSON bytes remain schema v3 compatible after renderer move | Unit | `internal/report/json_test.go` | `TestEncodeIssuesJSONV3Compatibility` | `go test ./internal/report -run '^TestEncodeIssuesJSONV3Compatibility$' -count=1` |
| R7 — SARIF preserves rules, locations, URI base, fingerprints, related locations, fixes, and version | Unit | `internal/report/sarif_test.go` | `TestEncodeSARIFCompatibilityContract` | `go test ./internal/report -run '^TestEncodeSARIFCompatibilityContract$' -count=1` |
| R8 — v4 reads and explicit v5 migration preserve live records | Unit | `internal/baseline/baseline_test.go` | `TestVersionFiveMigrationPreservesLiveEntries` | `go test ./internal/baseline -run '^TestVersionFiveMigrationPreservesLiveEntries$' -count=1` |
| R8 — update preserves annotations and prune is explicit | Integration | `tests/integration/cli_workflow_test.go` | `TestBaselineUpdatePreservesAnnotationsAndPruneIsExplicit` | `go test ./tests/integration -run '^TestBaselineUpdatePreservesAnnotationsAndPruneIsExplicit$' -count=1` |
| Generic suppression edits are absent from findings and SARIF | Unit | `internal/analyzer/fixes_test.go` | `TestFindingsDoNotReceiveGenericSuppressionFix` | `go test ./internal/analyzer -run '^TestFindingsDoNotReceiveGenericSuppressionFix$' -count=1` |
| Every check page reflects baseline v5 and truthful safe-fix contracts | Unit | `internal/analyzer/registry_test.go` | `TestCheckDocsReflectBaselineAndFixContracts` | `go test ./internal/analyzer -run '^TestCheckDocsReflectBaselineAndFixContracts$' -count=1` |
| Config schema and threshold registry remain synchronized | Unit | `internal/config/schema_test.go` | `TestCheckedInConfigSchemaMatchesThresholdRegistry` | `go test ./internal/config -run '^TestCheckedInConfigSchemaMatchesThresholdRegistry$' -count=1` |
| Checks discovery reports registry metadata in deterministic order | Unit | `internal/cli/commands_test.go` | `TestChecksListAndExplainUseRegistryMetadata` | `go test ./internal/cli -run '^TestChecksListAndExplainUseRegistryMetadata$' -count=1` |
| Stats reports selected/skipped/executed/cache counters without timing-log parsing | Unit | `internal/cli/commands_test.go` | `TestStatsCommandReportsExecutionPlanCounters` | `go test ./internal/cli -run '^TestStatsCommandReportsExecutionPlanCounters$' -count=1` |
| R9 — stable manifest maps every stable check to executable positive and negative cases | Unit | `internal/analyzer/precision_test.go` | `TestStableEvaluationManifestCoverageAndVerdicts` | `go test ./internal/analyzer -run '^TestStableEvaluationManifestCoverageAndVerdicts$' -count=1` |
| R10 — gate tiers preserve unique ownership of required checks | Unit | `internal/cli/commands_test.go` | `TestMakeGateTierContract` | `go test ./internal/cli -run '^TestMakeGateTierContract$' -count=1` |
| R10 — selected plugin checks run in a real custom GolangCI binary | E2E | `tests/e2e/plugin_test.go` | `TestCustomGolangCIModulePluginHonorsSelectedChecks` | `go test ./tests/e2e -run '^TestCustomGolangCIModulePluginHonorsSelectedChecks$' -count=1` |
| Coordinator self-scan has no complex-function findings | E2E | `tests/e2e/cli_test.go` | `TestAllProfileSelfScanHasNoCoordinatorComplexFunctionFindings` | `go test ./tests/e2e -run '^TestAllProfileSelfScanHasNoCoordinatorComplexFunctionFindings$' -count=1` |
| R11 — CLI/plugin E2E uses isolated temporary workspaces and portable paths | E2E | `tests/e2e/cli_test.go` | `TestE2EArtifactsStayOutsideScannedWorkspace` | `go test ./tests/e2e -run '^TestE2EArtifactsStayOutsideScannedWorkspace$' -count=1` |
| End-to-end v0.2 analyzer/report/baseline flow remains integrated | Integration | `tests/integration/flow_test.go` | `TestAnalyzerReportBaselineFlow` | `go test ./tests/integration -run '^TestAnalyzerReportBaselineFlow$' -count=1` |

## Acceptance Criteria

### Subtask 1

- [x] `ExecutionPlan` resolves profile/rules/checks/mode/surface once, exposes deterministic selected IDs and runner groups, and contributes the same selection to cache identity and stats.
- [x] The existing `analyzer.Run(pkgs, cfg, enabled)` signature remains source-compatible and delegates to the plan-aware runner.
- [x] Add Unit tests in `internal/analyzer/plan_test.go` and `internal/analyzer/registry_test.go`; `go test ./internal/analyzer -run '^(TestExecutionPlan|TestCheckRegistry|TestResolveCheckSelection)' -count=1` passes.

### Subtask 2

- [x] Package/program schedulers receive the plan and do not invoke unselected groups or emit disabled group members.
- [x] Shared runner parsing is performed once per selected package/group and program/group, with execution counts observable in `ExecutionStats`.
- [x] Add Unit tests in `internal/analyzer/run_test.go`; `go test ./internal/analyzer -run '^TestRunPlan' -count=1` passes.

### Subtask 3

- [x] `SnapshotInput` carries package path/name, module path, types package/info/errors, imports, and generated state; type completeness requires all required fields and an empty type-error set.
- [x] `NewAnalyzers` returns deterministic selected supported groups, and both module and Go plugin adapters use that factory path.
- [x] CLI and factory-plugin findings match for the external-interface, incomplete-types, clean, fat-interface, and concrete-dependency fixtures.
- [x] Add Unit and Integration tests in `internal/analysisapi/isp_test.go`, `plugin/solidlint/solidlint_test.go`, and `tests/integration/plugin_parity_test.go`; `go test ./internal/analysisapi ./plugin/solidlint ./tests/integration -run '(Factory|PluginAndCLIParity)' -count=1` passes.

### Subtask 4

- [x] Cache-disabled loading skips dependency digest I/O; cache-enabled loading hashes each local source through one owned digest path and stores compact digests rather than source/export blobs.
- [x] Package and program group keys include tool version, mode, tests, selection, configuration, exclusions, local source, and imported API facts.
- [x] Warm results are finding/fingerprint/SARIF-equivalent to cold and disabled-cache results.
- [x] Add Unit tests and benchmarks in `internal/analyzer/cache_test.go`, `internal/analyzer/run_test.go`, and `internal/analyzer/run_bench_test.go`; `go test ./internal/analyzer -run 'Test.*Cache' -count=1` and `go test ./internal/analyzer -run '^$' -bench '^BenchmarkRunSelected(Cold|Warm|Disabled)$' -benchmem -count=3` pass.

### Subtask 5

- [x] One threshold specification registry supplies names, bounds, getters/setters, descriptions, suggestions, effective config, and schema properties.
- [x] `config init` emits canonical YAML that validates; `config validate` reports file/line and canonical suggestions; `config schema -format=json` is byte-equivalent to `schemas/solidlint-config-v1.schema.json` after canonical formatting.
- [x] Add Unit tests in `internal/analyzer/control_test.go`, `internal/config/config_test.go`, and `internal/config/schema_test.go`; `go test ./internal/analyzer ./internal/config -run '(Threshold|Config|Schema)' -count=1` passes.

### Subtask 6

- [x] `internal/report` owns JSON v3 and SARIF encoding; `internal/cli` supplies build metadata and chooses a renderer but does not define machine-output DTOs.
- [x] Representative JSON and SARIF output remains compatible with checked-in schemas and preserves ordering, locations, fingerprints, URI bases, related information, safe fixes, and version fields.
- [x] Move/update Unit tests in `internal/report/json_test.go` and `internal/report/sarif_test.go`; `make sarif-check schema-check` passes.

### Subtask 7

- [x] Legacy check syntax and explicit `check` syntax have equivalent stdout, stderr, and exit status for clean, violation, invalid-config, broken-pipe, version, and print-config paths.
- [x] `checks list`, `checks explain`, and `stats` use registry/plan data and deterministic ordering; file arguments are documented and rendered as containing-package semantics without the obsolete tip.
- [x] `internal/cli/run.go` is a small composition/dispatch owner, with parsing, policy, commands, and rendering assigned to the new focused files.
- [x] Add Unit and E2E tests in `internal/cli/run_test.go`, `internal/cli/commands_test.go`, and `tests/e2e/cli_test.go`; `go test ./internal/cli ./tests/e2e -run '(Legacy|Checks|Stats|SingleGoFile)' -count=1` passes.

### Subtask 8

- [x] v5 writes annotated canonical entries, v4 reads remain supported, malformed entries and placeholder reasons fail with exit status 2, and mutation uses same-directory atomic replacement.
- [x] `baseline diff` is read-only; `baseline update` preserves live metadata and requires a reason for additions; `baseline prune` removes stale entries; explicit `--prune` combines update and pruning.
- [x] Legacy `-write-baseline` remains available as a deprecated alias and requires `-baseline-reason` when it would add v5 entries.
- [x] Findings do not receive universal suppression fixes, and registry/SARIF/plugin metadata advertises a fix only when a check supplies a real safe edit.
- [x] Add Unit and Integration tests in `internal/baseline/baseline_test.go`, `internal/analyzer/fixes_test.go`, `internal/report/sarif_test.go`, and `tests/integration/cli_workflow_test.go`; `go test ./internal/baseline ./internal/analyzer ./internal/report ./tests/integration -run '(Baseline|Suppression|SARIF)' -count=1` passes.

### Subtask 9

- [x] The manifest identifies each stable check’s positive and negative cases, expected finding ID/path/subject, rationale, and linked documentation.
- [x] The precision test executes every manifest case, rejects missing/orphan/duplicate IDs, and verifies negative cases do not emit the target stable check.
- [x] The seven stable pages contain compilable Go examples drawn from the manifest plus check-specific evidence, limitations, configuration, triage, and remediation guidance.
- [x] `docs/evaluations/stable-v0.2.md` records the exact manifest revision, command, per-check results, and reviewed false-positive/false-negative notes.
- [x] Add/update Unit evidence in `internal/analyzer/precision_test.go`; `make precision` passes.

### Subtask 10

- [x] Integration tests execute the real analyzer/report/baseline and factory-plugin boundaries; E2E tests build subprocesses and custom GolangCI binaries in isolated temporary directories.
- [x] `check-fast` owns the short local loop; `check` executes formatting, vet, unit, integration, E2E, race, coverage, self-lint, smoke, precision, cache parity, SARIF, schemas, plugin, version, and vulnerability checks once; `check-release` adds snapshot and external-consumer qualification.
- [x] CI matrix jobs run the fast cross-platform contract, the Linux hardening job runs `make check`, and release snapshot remains independently visible without changing release versions.
- [x] Add E2E tests in `tests/e2e/cli_test.go` and `tests/e2e/plugin_test.go`; `go test ./tests/e2e -count=1`, `make check-fast`, and `make check` pass.

### Subtask 11

- [x] README, contributor, changelog, security, schema, graduation, and stable-check documents match the implemented commands/contracts and contain no stale pre-v0.1.0 publication text.
- [x] All 27 check pages describe baseline v5 and do not claim that a generic suppression insertion is an automatic safe fix; the seven stable pages additionally carry the manifest-backed examples and evaluation links.
- [x] `.solidlint-baseline.json` is a canonical empty v5 document accepted by `baseline diff` and the self-enforcement target.
- [x] `scripts/release-consumer-smoke.sh` proves installation, legacy/new check syntax, config validation, annotated baseline behavior, and selected module-plugin checks for an explicitly supplied published version or commit.
- [x] Documentation command examples are exercised by `tests/e2e/cli_test.go`; `go test ./tests/e2e -run '^TestDocumentedCLIExamples$' -count=1` passes.

### Overall (Definition of Done)

- [x] All subtask acceptance criteria are met and the v0.2 milestone preserves the declared v0.1 public compatibility contracts.
- [x] Changes stay within the exact files declared in this plan, with no production dependency change and no analyzer-heuristic expansion.
- [x] The repository is independently usable after every ordered subtask; no later subtask is needed to restore compilation or a contract changed by an earlier subtask.
- [x] Implementation handoff records benchmark data, gate results, platform-specific plugin coverage, schema/baseline migrations, and any skipped environment-dependent release evidence separately.
