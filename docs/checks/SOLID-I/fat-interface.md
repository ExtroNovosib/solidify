# SOLID-I/fat-interface

Reports interfaces whose method count exceeds the configured maximum. Deprecated interfaces and broad aggregating interfaces may be downgraded to note severity so migrations remain visible without becoming default failures.

## Product contract

Maturity: **stable**

Analysis modes: equivalent in syntax, auto, and types modes.

Surfaces: standalone CLI and both GolangCI plugin modes.

## Examples

Positive: [stable positive evaluation fixture](../../../testdata/evaluation/positive/stable.go).

Clean: [stable negative evaluation fixture](../../../testdata/evaluation/negative/stable.go).

Evaluation case `stable-fat-interface` declares nine methods in the [positive fixture](../../../testdata/evaluation/positive/stable.go). The [negative fixture](../../../testdata/evaluation/negative/stable.go) declares exactly eight.

```go
package example

type WidePort interface {
	A(); B(); C(); D(); E(); F(); G(); H(); I()
}
```

## Evidence and configuration

Evidence is `fat-interface:interface=WidePort;methods=9;max=8`. Configure `max_interface_methods`; syntax, auto, and types modes produce equivalent method-count evidence.

## Limitations and remediation

Framework façades, generated APIs, and explicit aggregation ports may be intentionally broad. Review consumers and implementers before splitting the interface into role-specific contracts. Solidlint does not treat accepting debt as an automatic source fix. Use a reason-bearing suppression or baseline v5 annotation after review. See the [stable v0.2 evaluation](../../evaluations/stable-v0.2.md).
