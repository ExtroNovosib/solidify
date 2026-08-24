# SOLID-S/data-clump

Reports repeated parameter tuples that suggest a missing value object or request type.

## Product contract

Maturity: **stable**

Analysis modes: conservative syntax fallback; full type information is used when available.

Surfaces: standalone CLI and both GolangCI plugin modes.

## Examples

Positive: [stable positive evaluation fixture](../../../testdata/evaluation/positive/stable.go).

Clean: [stable negative evaluation fixture](../../../testdata/evaluation/negative/stable.go).

Evaluation case `stable-data-clump` comes from the [positive fixture](../../../testdata/evaluation/positive/stable.go); its [negative fixture](../../../testdata/evaluation/negative/stable.go) stays below the configured width.

```go
package example

func RegisterContact(name, email, phone, address, city, country, region, postal, company string) {}
func UpdateContact(name, email, phone, address, city, country, region, postal, company string) {}
```

## Evidence and configuration

Evidence identifies both functions and the stable tuple: `data-clump:function=UpdateContact;peer=RegisterContact;parameters=...`. The `max_params` threshold controls the minimum broad parameter surface considered by this family; exact check selection remains available through `enabled_checks` and `disabled_checks`.

## Limitations and remediation

A repeated tuple can be legitimate at a protocol boundary or in generated bindings. Check whether the values share validation, lifecycle, or meaning before introducing a parameter object. There is no automatic safe rewrite because a new type changes APIs. Record intentional debt with a reason-bearing suppression or annotated baseline v5 entry. See the [stable v0.2 evaluation](../../evaluations/stable-v0.2.md).
