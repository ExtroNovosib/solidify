# SOLID-O/type-dispatch

Reports concrete type dispatch that must be edited whenever a new variant is added.

## Product contract

Maturity: **stable**

Analysis modes: conservative syntax fallback; full type information is used when available.

Surfaces: standalone CLI only; this is a program-scoped check.

## Examples

Positive: [stable positive evaluation fixture](../../../testdata/evaluation/positive/stable.go).

Clean: [stable negative evaluation fixture](../../../testdata/evaluation/negative/stable.go).

Evaluation case `stable-type-dispatch` uses five variants in the [positive fixture](../../../testdata/evaluation/positive/stable.go), while the [negative fixture](../../../testdata/evaluation/negative/stable.go) demonstrates the four-case boundary.

```go
package example

type A struct{}; type B struct{}; type C struct{}; type D struct{}; type E struct{}
func Dispatch(value any) {
	switch value.(type) {
	case A, B, C, D, E:
	}
}
```

## Evidence and configuration

Evidence records the dispatch source, correlated sites, shared variants, full variant set, and maximum cases. `max_switch_cases`, `ocp_min_dispatch_sites`, `ocp_min_shared_variants`, and `ocp_dispatch_overlap_percent` control the decision. This program-scoped check runs in the standalone CLI, not package plugins.

## Limitations and remediation

Closed protocol decoders, exhaustive boundary adapters, and compatibility shims may deliberately enumerate variants. Prefer behavior on an interface only when variants own the behavior and extension is expected. No generic suppression is advertised as a safe fix; use reviewed suppression or annotated baseline v5 debt when the closed set is intentional. See the [stable v0.2 evaluation](../../evaluations/stable-v0.2.md).
