# Plan: Solidlint quality, convenience, efficiency, and reliability improvements
Implementation profile: `gpt-5.6-terra` / `xhigh`
Profile rationale: The work is bounded to established analyzer, CLI, baseline, plugin, and validation owners, but cache identity, partial type analysis, public CLI behavior, and cross-platform gates interact enough that Terra/high is insufficient.
Plan contract: `create-plan/v3`
Execution unit: `single`
Execution unit ID: `solidlint-quality`
Plan revision: `REV-2`
Scope mode: `bounded-open`

## Context & Goal
The current checkout began with four reproduced reliability defects: dependency-only method-set changes could reuse stale package-cache results; `syntax` and `auto` analysis aborted on ordinary type errors despite their documented fallback contracts; the Linux shared-plugin gate invoked an executable it did not build; and expired baseline entries continued to suppress findings. REV-0 repaired those defects, added initial CLI/coverage and loader improvements, and broadened first-party quality gates. Independent source verification then found five remaining obligations from the user's “do all” instruction: loader depth still does not depend on selected checks and still requests full dependency load fields; help and explanations omit command discovery, concrete examples, and legitimate exceptions; analysis coverage lacks per-check unavailability reasons; the stable evaluation corpus does not cover the named realistic edge styles; and race/E2E ownership still rebuilds expensive plugin tooling. REV-1 closes those obligations while preserving stable check IDs, severities, evidence, source locations, fingerprints, JSON v3, SARIF 2.1.0, configuration, suppression, baseline-read compatibility, and plugin selection behavior.

## Architecture Analysis
`internal/analyzer/workspace.go` owns package loading and analysis completeness, `internal/analyzer/cache.go` owns cache identity, `internal/baseline` owns accepted-debt policy, `internal/cli` owns command behavior and metadata rendering, and the Makefile/CI workflow own executable validation. REV-0 preserved those owners while repairing the four defects. REV-1 will pass execution-plan requirements into workspace loading, use initial-package syntax/types information while deriving imported type packages from `go/types` so dependency bodies are not requested, extend execution stats additively with deterministic per-check coverage reasons, enrich command metadata at the CLI boundary, add versioned evaluation cases and adjudication counters under the existing precision owner, and make expensive E2E/race ownership mutually exclusive. No dependency framework, finding-report schema revision, or analyzer-registry redesign is warranted.

## Security Closure Verdict
| Security boundary / risk ID | Abuse or collision sequence | Existing owners and trust trace | Change effect and severity | In-scope mitigation or safety evidence | Named negative proof | Verdict |
| --- | --- | --- | --- | --- | --- | --- |
| R7 — cache/report identities cannot cross repository or policy scopes | An untrusted checkout creates an entry with a colliding package/check key and a later scan consumes it under different source or policy state | `internal/analyzer/cache.go:newPackageCache` -> `packageHash` -> `entryPath` -> `load`; cache configuration, source, package, dependency API, tool version, and execution-plan keys are checked before use | unchanged — Low | S1 preserves and strengthens existing scoped hash validation; cache files never grant process authority or expose secrets | `internal/analyzer/cache_test.go` — corrupt, stale, root, and configuration isolation cases | PASS |

## Out of Scope
New SOLID checks, maturity changes, altered stable-profile membership, release publication, Go module dependency changes, and broad redesigns of individual smell heuristics are excluded.

## Assumptions & Open Questions
None

## Scope Contract
- **Primary paths:** `internal/analyzer/cache.go`, `internal/analyzer/workspace.go`, `internal/analyzer/stats.go`, `internal/baseline/**`, `internal/cli/**`, `Makefile`, `.github/workflows/ci.yml` — production and validation owners for the accepted work
- **Bounded support areas:** `internal/analyzer/*_test.go`, `internal/baseline/*_test.go`, `internal/cli/*_test.go`, `tests/integration/**`, `tests/e2e/**`, `testdata/**`, `docs/**`, `README.md`, `schemas/**`, `scripts/**` — focused fixtures, process proofs, documentation, and schema-compatible support
- **Explicit exclusions:** `go.mod`, `go.sum`, generated release artifacts, checked-in baselines unless an explicit format migration becomes necessary, and unrelated analyzer heuristic code
- **Generated-output families:** None expected

## Files
- **New:** focused fixture files only if existing temporary-module helpers cannot express a regression clearly
- **Modified:** analyzer load/cache/stats owners, baseline policy, CLI metadata/help/stats rendering, Make/CI validation, and directly corresponding documentation
- **Deleted:** None
- **Tests:** focused unit tests under `internal/analyzer`, `internal/baseline`, and `internal/cli`; process/integration coverage under `tests/e2e` and `tests/integration`
- **Documentation and supporting artifacts:** `README.md`, relevant `docs/checks/**` only if observable check guidance changes, and evaluation/gate docs if required by machine checks
- **Affected but not modified:** result JSON v3 and SARIF schemas, plugin registration API, published release workflow
- **Dependencies:** None

## Plan Applicability Verdict
| Promised behavior / failure branch | Trigger and production coordinator | Async/state/render path | Known owners / implementation area | Test file and exact case | Verdict |
| --- | --- | --- | --- | --- | --- |
| Dependency-only method-set changes invalidate cached findings | `internal/analyzer/cache.go:dependencyAPIDigest` | Contributors: `packageHash`, imported API digest, local dependency facts, cache version; cached and uncached runner output must converge | `internal/analyzer/cache.go`, `internal/analyzer/run_test.go`, `internal/analyzer/cache_test.go` | `internal/analyzer/run_test.go` — `TestRun_CacheInvalidatesWhenImportedMethodSetChanges` | PASS |
| Syntax mode analyzes parseable ill-typed source without type checking | `internal/analyzer/workspace.go:LoadWorkspace` | Contributors: `workspacePackagesLoadMode`, `collectWorkspaceLoadErrors`, `packageFilesFromLoaded`; syntax-capable runner output reaches CLI | `internal/analyzer/workspace.go`, analyzer/CLI process tests | `tests/e2e/cli_test.go` — `TestAnalysisModesHandleIllTypedSource` | PASS |
| Auto mode warns and runs syntax-capable checks on ill-typed packages | `internal/analyzer/workspace.go:LoadWorkspace` | Contributors: load-error classification, per-package completeness, execution coverage, CLI warning output | `internal/analyzer/workspace.go`, `internal/analyzer/stats.go`, `internal/cli/**` | `tests/e2e/cli_test.go` — `TestAnalysisModesHandleIllTypedSource` | PASS |
| Expired accepted debt no longer silently suppresses a live finding | `internal/baseline:Read` and CLI baseline application | Contributors: baseline document entries, current date policy, filter set, warning/error policy | `internal/baseline/**`, `internal/cli/**` | `internal/baseline/baseline_test.go` — `TestExpiredEntryPolicy`; `tests/integration/cli_workflow_test.go` — `TestExpiredBaselineWorkflow` | PASS |
| Linux shared-plugin proof cannot pass by negating a missing executable | `Makefile:plugin-go-e2e` | Contributors: explicit host build, plugin build, violation invocation, output assertion | `Makefile`, `scripts/**` or `tests/e2e/**` if a portable harness is needed | Make recipe and CI syntax/source assertion plus Linux CI execution | PASS |
| Selected syntax-capable checks avoid typed dependency loading | `internal/cli/policy.go:executeAnalysis` -> `internal/analyzer/workspace.go:LoadWorkspace` | Contributors: execution plan, requested analysis mode, initial-package load fields, imported `go/types` package map | `internal/analyzer/workspace.go`, `internal/cli/policy.go`, workspace tests | `internal/analyzer/workspace_test.go` — `TestWorkspaceLoadRequirementsFollowSelectedChecks` and `TestTypedWorkspaceDoesNotLoadDependencySyntax` | PASS |
| Every selected or skipped check has an observable coverage reason | `internal/analyzer/stats.go:snapshot` | Contributors: profile/config selection, integration surface, type completeness, architecture configuration; exactly one status per registered check | `internal/analyzer/plan.go`, `internal/analyzer/stats.go`, CLI render/tests | `internal/cli/commands_test.go` — `TestAnalysisCoverageReasons` | PASS |
| Expensive plugin E2E construction runs once in the canonical aggregate | `Makefile:check` | Contributors: `test-race`, `test-e2e`, `plugin-module-e2e`, `plugin-go-e2e-contract`, `plugin-go-e2e`; no overlapping `./tests/e2e` package execution | `Makefile`, source-contract E2E test | `tests/e2e/plugin_test.go` — `TestCanonicalGateOwnsExpensivePluginBuildOnce` | PASS |

## Prerequisites & Authority
- **Services:** None
- **Credential names:** None
- **Writable caches:** repository `.cache/` and `/private/tmp` for Go build/module/test artifacts
- **External authority required:** no

## Execution Capability Audit
| Capability | Required for | Availability evidence | Plan treatment |
| --- | --- | --- | --- |
| Go toolchain | P1-P8 | AVAILABLE: `go version` and focused package/E2E tests ran in the planning checkout | REQUIRED: compile, unit, integration, race, benchmark, and process tests |
| Make | P7-P8 | AVAILABLE: checked-in Makefile and shell environment | REQUIRED: focused gate inspection and final `make check` |
| Python 3 plan validator/protocol | plan validation and execution sidecar | AVAILABLE: callable local scripts under selected skills | REQUIRED: validate, begin, record, manifest, finish |
| Git | DOD-3 | AVAILABLE: clean `git status --short` planning baseline | REQUIRED: ownership and final diff/status audit |
| Linux CI | shared-object runtime execution | AVAILABLE: repository GitHub Actions workflow is the existing platform owner; local macOS proof is limited to source/recipe tests | OPTIONAL: post-push confirmation; local completion requires a deterministic Make/source test and cannot claim fresh Linux execution |

## Proof Readiness & Sequencing
Execution readiness: READY — every local proof is callable; Linux runtime execution remains an existing CI responsibility and is not claimed as fresh local evidence.

| Proof ID | Proof command | Kind | Baseline evidence | Introduced by | Expected-red owner(s) | First required green |
| --- | --- | --- | --- | --- | --- | --- |
| P1 | `GOCACHE=$PWD/.cache/go-build go test ./internal/analyzer -run 'TestRun_CacheInvalidatesWhenImported(MethodSet|Type)Changes|Test.*Cache' -count=1` | planned | N/A: planned proof; manual reproduction is expected red because a method-set change currently returns a stale hit | S1 | S1 | S1 |
| P2 | `GOCACHE=$PWD/.cache/go-build go test ./tests/e2e -run '^TestAnalysisModesHandleIllTypedSource$' -count=1` | planned | N/A: planned proof; manual reproduction is expected red because syntax and auto currently exit 2 | S2 | S2 | S2 |
| P3 | `GOCACHE=$PWD/.cache/go-build go test ./internal/baseline ./internal/cli ./tests/integration -run 'Test.*ExpiredBaseline|TestExpiredEntryPolicy' -count=1` | planned | N/A: planned proof; manual reproduction is expected red because year-2000 expiry still suppresses | S3 | S3 | S3 |
| P4 | `GOCACHE=$PWD/.cache/go-build go test ./internal/cli ./tests/e2e -run 'Test.*(Help|ChecksExplain|AnalysisCoverage)' -count=1` | planned | N/A: planned proof for CLI metadata and coverage cases | S4 | S4 | S4 |
| P5 | `make plugin-go-e2e-contract` | planned | N/A: planned proof; no deterministic prerequisite/negative-command contract test exists | S5 | S5 | S5 |
| P6 | `GOCACHE=$PWD/.cache/go-build go test ./internal/analyzer -run '^$' -bench '^BenchmarkRunSelected(Warm|Disabled)$' -benchtime=5x -count=1 -benchmem` | existing plan-owned | PASS: command runs; planning sample shows approximately 220 MB total allocation per scan and package loading dominates CPU | existing | None | before implementation |
| P7 | `GOCACHE=$PWD/.cache/go-build go test ./internal/... ./plugin/... ./cmd/... ./tests/integration ./tests/e2e -count=1` | existing plan-owned | PASS: package/integration tests and the focused E2E subset passed in planning | existing | S1, S2, S3, S4, S5, S6 | S6 |
| P8 | `GOCACHE=$PWD/.cache/go-build make check` | existing plan-owned | N/A: full unchanged aggregate was not run during planning | existing | S1, S2, S3, S4, S5, S6 | S6 |
| P9 | `GOCACHE=$PWD/.cache/go-build go test ./internal/analyzer ./internal/baseline ./internal/cli -count=1` | existing plan-owned | PASS: equivalent focused package set passed during planning | existing | S1, S2, S3, S4, S6 | S6 |
| P10 | `GOCACHE=$PWD/.cache/go-build go test ./tests/integration -count=1` | existing unchanged | PASS: integration package passed during planning | existing | None | before implementation |
| P11 | `GOCACHE=$PWD/.cache/go-build go test ./tests/e2e -count=1` | existing plan-owned | PASS: focused E2E subset passed; complete package is expected to remain green after added cases | existing | S2, S4, S5 | S5 |
| P12 | `git status --short && git diff --check && git diff -- go.mod go.sum` | existing unchanged | PASS: planning checkout was clean and dependency files unchanged | existing | None | before implementation |
| P13 | `GOCACHE=$PWD/.cache/go-build go test ./internal/analyzer ./internal/cli -run 'Test.*(WorkspaceLoadRequirements|DependencySyntax|AnalysisCoverageReasons)' -count=1` | planned | N/A: planned proof; source verification shows workspace loading currently ignores the execution plan and coverage has only a boolean | S7 | S7 | S7 |
| P14 | `GOCACHE=$PWD/.cache/go-build go test ./internal/cli ./tests/e2e -run 'Test.*(TopLevelHelp|ChecksExplainExamples)' -count=1` | planned | N/A: planned proof; current help omits commands and explanation metadata omits examples/exceptions | S8 | S8 | S8 |
| P15 | `GOCACHE=$PWD/.cache/go-build go test ./internal/analyzer -run '^(TestPrecisionCorpus|TestStableEvaluationManifestCoverageAndVerdicts|TestStableEvaluationEdgeStyles)$' -count=1` | planned | N/A: planned proof for embedding, generics, adapters, generated, and platform evaluation cases | S9 | S9 | S9 |
| P16 | `GOCACHE=$PWD/.cache/go-build make test-ownership-contract` | planned | N/A: planned proof; current race and E2E ownership overlaps | S10 | S10 | S10 |

## Subtasks
- [x] **S1 — Make dependency-sensitive package caching sound and retain cold/warm parity with a cache-format bump and focused invalidation tests** — depends on: none
- [x] **S2 — Restore truthful syntax/auto fallback and expose per-package analysis completeness without weakening strict types mode** — depends on: S1
- [x] **S3 — Enforce an explicit expired-baseline policy with deterministic dates, warnings/errors, compatibility, and workflow tests** — depends on: S2
- [x] **S4 — Improve CLI help, check explanations, thresholds/remediation metadata, and coverage visibility while preserving text/JSON/SARIF finding contracts** — depends on: S3
- [x] **S5 — Repair Linux shared-plugin gate ownership and broaden first-party lint/vet coverage without hiding fixture violations or duplicating expensive plugin builds** — depends on: S4
- [x] **S6 — Remove avoidable loader/hash work, retain selected-check semantics, update documentation, and pass package plus canonical aggregate gates** — depends on: S5
- [x] **S7 — Make auto-mode load depth follow selected checks, avoid dependency syntax/type-info bodies where exported type APIs suffice, and report deterministic per-check coverage reasons** — depends on: S6
- [x] **S8 — Complete CLI discoverability with top-level command help plus concise before/after examples and legitimate-exception guidance for every principle** — depends on: S7
- [x] **S9 — Add versioned stable evaluation cases and FP/FN accounting for embedding, generics, adapters, generated files, and platform-specific files** — depends on: S8
- [x] **S10 — Give race, ordinary E2E, module-plugin E2E, and Go-plugin contract/runtime checks exclusive ownership, then rerun the canonical aggregate** — depends on: S9

## Risks & Edge Cases
- **R1 — Cache false negatives:** any imported API fact used by a check must participate in cache identity, including pointer/value method sets, embedding, aliases, and generics.
- **R2 — False fallback:** syntax/auto must recover only from type-resolution failures; package discovery and parse errors remain fatal, and types mode remains fail-closed.
- **R3 — Baseline time drift:** expiry comparisons must use an injected date in unit tests and a documented UTC/local-date rule in production.
- **R4 — Public output compatibility:** existing finding text, JSON v3, SARIF, fingerprints, severities, and exit statuses remain stable except documented help/stats/expired-baseline behavior.
- **R5 — Platform gate false green:** non-Linux hosts may skip runtime shared-object execution, but Linux recipes must build every invoked artifact and distinguish expected findings from launch failures.
- **R6 — Performance regression:** loader-mode reductions must preserve complete type facts for selected checks and be evaluated with parity tests before accepting allocation/latency changes.
- **R7 — Cache scope collision:** strengthened dependency hashing must retain repository, configuration, plan, tool-version, package, and check separation so an unrelated cache entry cannot be accepted.
- **R8 — Under-loading selected checks:** load optimization must preserve imported type identity and full initial-package type information for every selected typed check.
- **R9 — Misleading coverage status:** every registered check must have exactly one deterministic selected, unavailable, or skipped reason without implying that no finding proves architectural quality.
- **R10 — Evaluation overclaim:** added corpora must record machine-observed FP/FN counts and project-style provenance without claiming external human adjudication that did not occur.
- **R11 — Gate coverage gap:** removing E2E packages from race must retain race coverage for all first-party production/integration packages and normal coverage for every process workflow.

## Test Strategy
Change classification: non-user-facing

| Layer | Required or N/A | Behaviors and risks covered | Test files | Verification command |
| --- | --- | --- | --- | --- |
| Unit | Required | cache invalidation, mode classification, expiry, metadata, stats | `internal/analyzer/*_test.go`, `internal/baseline/*_test.go`, `internal/cli/*_test.go` | `GOCACHE=$PWD/.cache/go-build go test ./internal/analyzer ./internal/baseline ./internal/cli -count=1` |
| Integration | Required | baseline workflow, plugin settings, report/CLI boundary | `tests/integration/**` | `GOCACHE=$PWD/.cache/go-build go test ./tests/integration -count=1` |
| E2E | Required | subprocess mode/help behavior and plugin gate contract | `tests/e2e/**`, `Makefile` | `GOCACHE=$PWD/.cache/go-build go test ./tests/e2e -count=1` |
| Playwright UI | N/A: command-line linter has no rendered browser surface | No browser behavior exists | N/A | `N/A: no browser surface` |

### Coverage Traceability
| Risk or acceptance criterion | Proving layer | Test file | Named test case | Verification command |
| --- | --- | --- | --- | --- |
| R1, AC-S1-1 | Unit | `internal/analyzer/run_test.go`, `internal/analyzer/cache_test.go` | `TestRun_CacheInvalidatesWhenImportedMethodSetChanges` and cache parity cases | `GOCACHE=$PWD/.cache/go-build go test ./internal/analyzer -run 'TestRun_CacheInvalidatesWhenImported(MethodSet|Type)Changes|Test.*Cache' -count=1` |
| R2, AC-S2-1 | E2E | `tests/e2e/cli_test.go` | `TestAnalysisModesHandleIllTypedSource` | `GOCACHE=$PWD/.cache/go-build go test ./tests/e2e -run '^TestAnalysisModesHandleIllTypedSource$' -count=1` |
| R3, AC-S3-1 | Unit and Integration | `internal/baseline/baseline_test.go`, `tests/integration/cli_workflow_test.go` | `TestExpiredEntryPolicy`, `TestExpiredBaselineWorkflow` | `GOCACHE=$PWD/.cache/go-build go test ./internal/baseline ./internal/cli ./tests/integration -run 'Test.*ExpiredBaseline|TestExpiredEntryPolicy' -count=1` |
| R4, AC-S4-1 | Unit and E2E | `internal/cli/*_test.go`, `tests/e2e/cli_test.go` | help, explain, and analysis-coverage cases | `GOCACHE=$PWD/.cache/go-build go test ./internal/cli ./tests/e2e -run 'Test.*(Help|ChecksExplain|AnalysisCoverage)' -count=1` |
| R5, AC-S5-1 | E2E contract | `Makefile`, focused script/test support | plugin host prerequisite and expected-diagnostic assertion | `make plugin-go-e2e-contract` |
| R6, AC-S6-1 | Unit, Integration, E2E | full first-party test scope | all package and process cases | `GOCACHE=$PWD/.cache/go-build go test ./internal/... ./plugin/... ./cmd/... ./tests/integration ./tests/e2e -count=1` |
| R7 | Unit | `internal/analyzer/cache_test.go` | corrupt, stale, root, and configuration isolation cases | `GOCACHE=$PWD/.cache/go-build go test ./internal/analyzer -run 'TestRun_CacheInvalidatesWhenImported(MethodSet|Type)Changes|Test.*Cache' -count=1` |
| R8, R9, AC-S7-1 | Unit | `internal/analyzer/workspace_test.go`, `internal/cli/commands_test.go` | selected-load, dependency-body, and coverage-reason cases | `GOCACHE=$PWD/.cache/go-build go test ./internal/analyzer ./internal/cli -run 'Test.*(WorkspaceLoadRequirements|DependencySyntax|AnalysisCoverageReasons)' -count=1` |
| R4, AC-S8-1 | Unit and E2E | `internal/cli/*_test.go`, `tests/e2e/cli_test.go` | top-level help and examples/exceptions in text/JSON | `GOCACHE=$PWD/.cache/go-build go test ./internal/cli ./tests/e2e -run 'Test.*(TopLevelHelp|ChecksExplainExamples)' -count=1` |
| R10, AC-S9-1 | Unit evaluation | `internal/analyzer/precision_test.go`, `testdata/evaluation/**` | stable edge-style matrix and exact FP/FN accounting | `GOCACHE=$PWD/.cache/go-build go test ./internal/analyzer -run '^(TestPrecisionCorpus|TestStableEvaluationManifestCoverageAndVerdicts|TestStableEvaluationEdgeStyles)$' -count=1` |
| R11, AC-S10-1 | Gate contract | `Makefile`, `tests/e2e/plugin_test.go` | exclusive expensive-test ownership | `GOCACHE=$PWD/.cache/go-build make test-ownership-contract` |
| DOD-1, DOD-2 | Aggregate | repository validation owners | canonical local gate | `GOCACHE=$PWD/.cache/go-build make check` |
| DOD-3 | Source audit | repository root | no unrelated or dependency changes | `git status --short && git diff --check && git diff -- go.mod go.sum` |

## Acceptance Criteria
### S1
- [x] **AC-S1-1:** A dependency-only change that adds or removes relevant methods cannot reuse stale cached findings, and focused cache parity proof P1 passes.

### S2
- [x] **AC-S2-1:** Parseable ill-typed packages still produce syntax-capable findings in syntax and auto modes, auto reports incomplete coverage, and types mode exits 2; proof P2 passes.

### S3
- [x] **AC-S3-1:** Expired baseline entries follow the documented policy and cannot silently suppress live findings; legacy/current documents and deterministic tests pass via P3.

### S4
- [x] **AC-S4-1:** Help exits successfully, check explanation output includes actionable configuration/remediation context, and stats reveal analysis completeness by package without changing finding schemas; proof P4 passes.

### S5
- [x] **AC-S5-1:** The Linux shared-plugin recipe builds each invoked artifact, rejects launch failures, and quality targets cover all first-party production packages with expensive plugin construction owned once; proof P5 passes.

### S6
- [x] **AC-S6-1:** Avoidable package-loading or hashing work is removed without changing findings, selection, or analysis-mode behavior, and P6/P7 show current performance plus complete functional parity.

### S7
- [x] **AC-S7-1:** Auto-mode loader requirements are derived from selected checks, typed initial packages retain required imported API facts without loading dependency syntax/type-info bodies, and stats give every check an exact coverage status/reason; P13 passes.

### S8
- [x] **AC-S8-1:** Top-level help lists commands and successful usage, while text and JSON explanations include relevant configuration, remediation, a compact before/after example, and legitimate-exception guidance; P14 passes.

### S9
- [x] **AC-S9-1:** A versioned evaluation matrix covers embedding, generics, adapters, generated files, and platform-specific files, records exact observed false-positive/false-negative counts without external-review claims, and P15 passes.

### S10
- [x] **AC-S10-1:** Race and each expensive E2E/plugin workflow have one canonical owner, all changed process cases remain in the aggregate, and P16 plus the exact aggregate P8 pass.

### Overall (Definition of Done)
- [x] **DOD-1:** Cached/uncached analysis, selected-load depth, partial-type fallback, baseline expiry, complete coverage explanations, CLI guidance, evaluation accounting, and plugin validation behave according to the repaired documented contracts.
- [x] **DOD-2:** Stable IDs, severities, evidence, paths, fingerprints, JSON v3, SARIF 2.1.0, configuration, suppression, baseline compatibility, and supported plugin selection remain compatible, with the canonical local gate green after REV-1.
- [x] **DOD-3:** REV-0 and REV-1 changes stay within the declared bounded support areas, introduce no production dependency, and leave unrelated work untouched.

## Plan Revision Log
| Revision | Trigger | Agent profile | Decisions changed | Evidence invalidated | Validation |
| --- | --- | --- | --- | --- | --- |
| REV-0 | Initial plan | `gpt-5` / `high` | Initial architecture and scope | None | v3 validator passed |
| REV-1 | Independent result verification found unimplemented parts of the user's “do all” request | `gpt-5` / `high` | Add selected-check loader depth, dependency-body avoidance, full CLI guidance, exact coverage reasons, edge-style evaluation accounting, and exclusive expensive-test ownership | DOD-1, DOD-2, and DOD-3 require fresh completion evidence; prior implementation criteria remain valid | v3 validator passed |
| REV-2 | Correct stale REV-1 validation-state bookkeeping after the completed plan was freshly validated | `gpt-5.6-terra` / `xhigh` | Record the passed REV-1 validator result and revise only plan metadata | None; production/test source and proof commands are unchanged | v3 validator passed |

## Evidence Impact
| Revision | Item ID | Impact | Replacement IDs | Reason |
| --- | --- | --- | --- | --- |
| REV-0 | all | INITIAL | None | Initial plan has no prior evidence |
| REV-1 | S1 | PRESERVE | None | Cache behavior and P1 are unchanged by the added obligations |
| REV-1 | AC-S1-1 | PRESERVE | None | Cache criterion and proof are unchanged |
| REV-1 | S2 | PRESERVE | None | Fallback behavior and P2 are unchanged |
| REV-1 | AC-S2-1 | PRESERVE | None | Fallback criterion and proof are unchanged |
| REV-1 | S3 | PRESERVE | None | Expiry behavior and P3 are unchanged |
| REV-1 | AC-S3-1 | PRESERVE | None | Expiry criterion and proof are unchanged |
| REV-1 | S4 | PRESERVE | None | REV-0 CLI metadata criterion remains satisfied; S8 adds distinct guidance obligations |
| REV-1 | AC-S4-1 | PRESERVE | None | Existing help-success/config/remediation criterion remains unchanged |
| REV-1 | S5 | PRESERVE | None | Plugin-host correctness and broadened quality scope remain unchanged |
| REV-1 | AC-S5-1 | PRESERVE | None | Existing plugin contract proof remains unchanged |
| REV-1 | S6 | PRESERVE | None | Existing no-op file-policy and export-metadata optimization remains valid; S7 adds distinct loader-depth obligations |
| REV-1 | AC-S6-1 | PRESERVE | None | Existing optimization/parity criterion remains unchanged |
| REV-1 | DOD-1 | REVALIDATE | None | Completion scope now includes selected loading, explanations, and evaluation accounting |
| REV-1 | DOD-2 | REVALIDATE | None | Additional code changes require a fresh compatibility and aggregate proof |
| REV-1 | DOD-3 | REVALIDATE | None | Additional paths require a fresh scope/dependency audit |
| REV-1 | S7 | REVALIDATE | None | New selected-loader and coverage-reason obligation starts without prior evidence |
| REV-1 | AC-S7-1 | REVALIDATE | None | New selected-loader and coverage-reason criterion starts without prior evidence |
| REV-1 | S8 | REVALIDATE | None | New complete CLI-discoverability obligation starts without prior evidence |
| REV-1 | AC-S8-1 | REVALIDATE | None | New complete CLI-guidance criterion starts without prior evidence |
| REV-1 | S9 | REVALIDATE | None | New edge-style evaluation obligation starts without prior evidence |
| REV-1 | AC-S9-1 | REVALIDATE | None | New evaluation-accounting criterion starts without prior evidence |
| REV-1 | S10 | REVALIDATE | None | New exclusive test-ownership obligation starts without prior evidence |
| REV-1 | AC-S10-1 | REVALIDATE | None | New test-ownership criterion starts without prior evidence |
| REV-2 | S1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | AC-S1-1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | S2 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | AC-S2-1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | S3 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | AC-S3-1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | S4 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | AC-S4-1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | S5 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | AC-S5-1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | S6 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | AC-S6-1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | S7 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | AC-S7-1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | S8 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | AC-S8-1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | S9 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | AC-S9-1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | S10 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | AC-S10-1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | DOD-1 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | DOD-2 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
| REV-2 | DOD-3 | PRESERVE | None | Metadata-only revision; production source and proof evidence are unchanged |
