# SOLID-S/large-type

Reports a type only when several independent size and complexity signals agree. The default stable rule avoids treating one large-looking metric as proof of mixed responsibility.

## Product contract

Maturity: **stable**

Analysis modes: conservative syntax fallback; full type information is used when available.

Surfaces: standalone CLI and both GolangCI plugin modes.

## Examples

Positive: [stable positive evaluation fixture](../../../testdata/evaluation/positive/stable.go).

Clean: [stable negative evaluation fixture](../../../testdata/evaluation/negative/stable.go).

Evaluation case `stable-large-type` is implemented in [the positive fixture](../../../testdata/evaluation/positive/stable.go) and checked against [the boundary fixture](../../../testdata/evaluation/negative/stable.go).

```go
package example

type LargeService struct {
	ID int
	Name string
	Enabled bool
	Payload []byte
	Metadata map[string]string
}
func (*LargeService) Create() {}
func (*LargeService) Delete() {}
```

The checked-in positive expands this compilable shape to eleven heterogeneous fields, eleven exported methods, and aggregate complexity above the default WMC boundary. The negative case stays at ten fields and ten methods.

## Evidence and configuration

Evidence uses `large-type:type=<name>;methods=<n>;exported_methods=<n>;fields=<n>;loc=<n>;wmc=<n>;signals=<n>`. Metrics identify each configured comparator. Relevant keys are `max_methods`, `max_fields`, `max_exported_methods`, `max_type_lines`, `max_type_complexity`, and `min_large_type_signals`.

## Limitations and remediation

Generated models, DTOs, serialization records, and framework-owned structures may be intentionally broad. Confirm that the reported methods and fields change for different reasons before extracting cohesive services or value types. Solidlint does not advertise a generic suppression as a safe repair. Use a justified source suppression for a local exception, or baseline v5 with a review reason, owner, and optional expiry for tracked debt. See the [stable v0.2 evaluation](../../evaluations/stable-v0.2.md).
