# SOLID-D/wiring-outside-root

Heuristic check for **wiring outside root** smells in Go code.

Example: scan the package directory and review the reported evidence before suppressing with `//solidify:ignore SOLID-D/wiring-outside-root reason`.

## Product contract

- Maturity: **experimental**. Experimental checks require `profile: all`, `-profile=all`, or explicit `enabled_checks`.
- Analysis modes: types required; withheld in syntax and incomplete-auto packages.
- Surfaces: standalone CLI only because the check performs program correlation.
- Evidence names the matched source construct; metrics record measured values, configured thresholds, and comparators. Fingerprints use the check ID, portable path, subject, and identity, never the message or measured counts.

## Examples

```go
// Positive: the focused positive fixture for SOLID-D/wiring-outside-root crosses the documented signal.
// Clean: its boundary and clean counterexamples remain at or below the signal.
```

The analyzer corpus contains the executable positive, boundary, and clean examples. Run `go test ./internal/analyzer -count=1` and `make precision` after changing detection behavior.

## Limitations and remediation

This is an explainable heuristic, not proof of a design defect. Review generated code, DTOs, composition roots, adapters, thin wrappers, and framework contracts before refactoring. Prefer a behavior-preserving extraction or narrower consumer-owned abstraction. Solidlint does not advertise generic suppression insertion as an automatic or safe source fix. For intentional debt, add a reason-bearing `//solidify:ignore SOLID-D/wiring-outside-root ...` manually or use an annotated baseline v5 entry with review context. Configure canonical snake_case thresholds where the check exposes them, and use exact IDs in `disabled_checks`, severity overrides, suppressions, and baselines.
