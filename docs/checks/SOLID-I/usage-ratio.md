# SOLID-I/usage-ratio

Heuristic check for **usage ratio** smells in Go code.

Example: scan the package directory and review the reported evidence before suppressing with `//solidify:ignore SOLID-I/usage-ratio reason`.

## Scope

The check examines exported function and method parameters. It also examines
interface-typed fields on exported consumers and aggregates the methods used
across all receiver methods. Unexported helpers are implementation details, so
their parameter types are not treated as public dependency surfaces.

It analyzes named interfaces owned by the scanned module. Interfaces from
dependencies are excluded because the caller cannot change their method set;
define a local port when that dependency needs ISP review.

Packages matching the existing `architecture.composition_roots` patterns are
skipped for this check because composition roots are expected to wire broad
interfaces. This exemption applies to the package, not to an interface: an
exported consumer outside a configured root is still reported, including when
the consumed interface embeds other interfaces.

When `architecture.logic_packages` is configured, it is also used as an include
filter for `usage-ratio`. This allows application/use-case packages to stay
covered while adapter and persistence packages are excluded without disabling
the check globally.

## Product contract

- Maturity: **stable**. Experimental checks require `profile: all`, `-profile=all`, or explicit `enabled_checks`.
- Analysis modes: types required; withheld in syntax and incomplete-auto packages.
- Surfaces: standalone CLI, module plugin, and matched-ABI Go plugin.
- Evidence names the matched source construct; metrics record measured values, configured thresholds, and comparators. Fingerprints use the check ID, portable path, subject, and identity, never the message or measured counts.

## Examples

```go
// Positive: the focused positive fixture for SOLID-I/usage-ratio crosses the documented signal.
// Clean: its boundary and clean counterexamples remain at or below the signal.
```

The analyzer corpus contains the executable positive, boundary, and clean examples. Run `go test ./internal/analyzer -count=1` and `make precision` after changing detection behavior.

## Limitations and remediation

This is an explainable heuristic, not proof of a design defect. Review generated code, DTOs, composition roots, adapters, thin wrappers, and framework contracts before refactoring. Prefer a behavior-preserving extraction or narrower consumer-owned abstraction. The only automatic fix is a reason-bearing `//solidify:ignore SOLID-I/usage-ratio ...` triage comment; baselines v4 accept reviewed debt without changing source. Configure canonical snake_case thresholds where the check exposes them, and use exact IDs in `disabled_checks`, severity overrides, suppressions, and baselines.
