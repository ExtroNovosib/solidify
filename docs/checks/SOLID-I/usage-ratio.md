# SOLID-I/usage-ratio

Reports exported consumers that use only a small fraction of a module-owned interface.

## Product contract

Maturity: **stable**

Analysis modes: types required; unavailable in syntax-only mode.

Surfaces: standalone CLI and both GolangCI plugin modes.

## Examples

Positive: [stable positive evaluation fixture](../../../testdata/evaluation/positive/stable.go).

Clean: [stable negative evaluation fixture](../../../testdata/evaluation/negative/stable.go).

Evaluation case `stable-usage-ratio` uses one of three methods in the [positive fixture](../../../testdata/evaluation/positive/stable.go); the [negative fixture](../../../testdata/evaluation/negative/stable.go) calls all three.

```go
package example

type Repository interface { Get() error; Save() error; Delete() error }
func UseRepository(repository Repository) error { return repository.Get() }
```

## Evidence and configuration

Evidence records the function or field, consumer name, used count, total count, and method set. `isp_min_methods` and `isp_usage_ratio_percent` set the size and ratio boundaries. Complete types are required. Dependency-owned interfaces, composition roots, and packages outside configured `architecture.logic_packages` are excluded.

## Limitations and remediation

A consumer may forward an interface indirectly or intentionally expose a compatibility contract. Confirm call flow before introducing a consumer-owned port. No generic source edit is safe; document an exception or use annotated baseline v5 debt with a specific review reason. See the [stable v0.2 evaluation](../../evaluations/stable-v0.2.md).
