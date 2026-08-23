# SOLID-I/fat-interface

Heuristic check for **fat interface** smells in Go code.

Interfaces marked deprecated in their doc comment (`Deprecated:`, `DEPRECATED:`, or a line starting with `deprecated `) are downgraded to `note` severity so migration-in-progress types stay visible without failing default enforcement (`-fail-level=warning`).

An interface that aggregates another interface already over the configured
method limit is also downgraded to `note`. This keeps broad wiring repositories
visible while preserving `warning` severity for the narrower business-facing
interface that should actually be split.

Example: scan the package directory and review the reported evidence before suppressing with `//solidify:ignore SOLID-I/fat-interface reason`.

## Product contract

- Maturity: **stable**. Experimental checks require `profile: all`, `-profile=all`, or explicit `enabled_checks`.
- Analysis modes: equivalent in syntax, auto, and types modes.
- Surfaces: standalone CLI, module plugin, and matched-ABI Go plugin.
- Evidence names the matched source construct; metrics record measured values, configured thresholds, and comparators. Fingerprints use the check ID, portable path, subject, and identity, never the message or measured counts.

## Examples

```go
// Positive: the focused positive fixture for SOLID-I/fat-interface crosses the documented signal.
// Clean: its boundary and clean counterexamples remain at or below the signal.
```

The analyzer corpus contains the executable positive, boundary, and clean examples. Run `go test ./internal/analyzer -count=1` and `make precision` after changing detection behavior.

## Limitations and remediation

This is an explainable heuristic, not proof of a design defect. Review generated code, DTOs, composition roots, adapters, thin wrappers, and framework contracts before refactoring. Prefer a behavior-preserving extraction or narrower consumer-owned abstraction. The only automatic fix is a reason-bearing `//solidify:ignore SOLID-I/fat-interface ...` triage comment; baselines v4 accept reviewed debt without changing source. Configure canonical snake_case thresholds where the check exposes them, and use exact IDs in `disabled_checks`, severity overrides, suppressions, and baselines.
