# SOLID-O/concrete-parameter

Heuristic check for **concrete parameter** smells in Go code.

Only a pointer parameter to a named type declared in another package, used
solely through at least `ocp_min_concrete_parameter_methods` methods, is
reported. Like `SOLID-D/concrete-dependency` constructor findings, parameters
whose type is declared in the analyzed package and parameters passed by value
are skipped: the package's own types are its vocabulary, and a by-value copy is
not a collaborator a caller could substitute.

Types listed in `allow_dependencies` are excluded so an intentional concrete
dependency is treated consistently by both DIP and OCP checks.

Example: scan the package directory and review the reported evidence before suppressing with `//solidify:ignore SOLID-O/concrete-parameter reason`.

## Product contract

- Maturity: **experimental**. Experimental checks require `profile: all`, `-profile=all`, or explicit `enabled_checks`.
- Analysis modes: types required; withheld in syntax and incomplete-auto packages.
- Surfaces: standalone CLI only because the check performs program correlation.
- Evidence names the matched source construct; metrics record measured values, configured thresholds, and comparators. Fingerprints use the check ID, portable path, subject, and identity, never the message or measured counts.

## Examples

```go
// Positive: the focused positive fixture for SOLID-O/concrete-parameter crosses the documented signal.
// Clean: its boundary and clean counterexamples remain at or below the signal.
```

The analyzer corpus contains the executable positive, boundary, and clean examples. Run `go test ./internal/analyzer -count=1` and `make precision` after changing detection behavior.

## Limitations and remediation

This is an explainable heuristic, not proof of a design defect. Review generated code, DTOs, composition roots, adapters, thin wrappers, and framework contracts before refactoring. Prefer a behavior-preserving extraction or narrower consumer-owned abstraction. Solidlint does not advertise generic suppression insertion as an automatic or safe source fix. For intentional debt, add a reason-bearing `//solidify:ignore SOLID-O/concrete-parameter ...` manually or use an annotated baseline v5 entry with review context. Configure canonical snake_case thresholds where the check exposes them, and use exact IDs in `disabled_checks`, severity overrides, suppressions, and baselines.

Rich domain aggregates used only through zero-argument data accessors are mapper inputs. Domain package placement alone does not exempt an injected behavioral collaborator: mutations, commands, parameters, error results and named state transitions remain eligible. A domain collaborator with `Execute() bool` and `Advance() int` is covered by a positive control alongside clean ID/name mapping.
