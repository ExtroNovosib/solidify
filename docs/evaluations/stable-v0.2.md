# Stable profile evaluation — v0.2

Manifest revision: `stable-v0.2-r1` in `testdata/evaluation/stable-v0.2.json`.

Reproduce the adjudication gate with:

```sh
go test ./internal/analyzer -run '^(TestPrecisionCorpus|TestStableEvaluationManifestCoverageAndVerdicts)$' -count=1
```

## Results

| Check | Positive case | Negative case | Result |
|---|---|---|---|
| `SOLID-S/large-type` | `stable-large-type` | threshold-boundary service | pass |
| `SOLID-S/data-clump` | `stable-data-clump` | five-parameter pair | pass |
| `SOLID-O/type-dispatch` | `stable-type-dispatch` | four-variant boundary | pass |
| `SOLID-I/fat-interface` | `stable-fat-interface` | eight-method boundary | pass |
| `SOLID-I/usage-ratio` | `stable-usage-ratio` | full three-method consumer | pass |
| `SOLID-I/stub-implementation` | `stable-stub-implementation` | meaningful implementation | pass |
| `SOLID-D/concrete-dependency` | `stable-concrete-dependency` | same-package collaborator | pass |

## Review notes

- False-positive review: the negative corpus covers numerical boundaries, cohesive repeated inputs, full interface use, meaningful implementations, and same-package concrete construction. None emits its paired stable check.
- False-negative review: each positive case names the expected check ID, portable path, and subject. The gate fails if that exact finding disappears, moves unexpectedly, or changes identity ownership.
- Known limits remain documented per check: generated/framework surfaces, closed protocol dispatch, migration adapters, composition roots, and dependency-owned interfaces require human context.
- This report records heuristic evidence, not a claim of semantic proof. A maturity change still requires a separate manifest revision and reviewed corpus update.
