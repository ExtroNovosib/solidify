# SOLID-I/unused-dependency

Reports an exported injected interface field that is never consumed in its
package, including a short field-to-field assignment chain that ends at an
unread collaborator field.

## Product contract

Maturity: **experimental**

Analysis modes: types required; unavailable in syntax-only mode and withheld
for incomplete packages in auto mode.

Surfaces: standalone CLI and both GolangCI plugin modes.

The check is limited to configured logic packages and skips composition roots,
named service bundles, dependency bags, and `*Stores` wiring aggregates. It
follows only direct struct-field transfers. Any call, return, local variable,
or unresolved flow is treated conservatively as consumed instead of becoming a
finding.

## Examples

Positive: [unused-flow unit fixture](../../../internal/analyzer/isp_consumer_test.go)

Clean: [consumed-flow unit fixture](../../../internal/analyzer/isp_consumer_test.go)

```go
type Ports struct {
	Unused Store
}

// No consumer ever selects or forwards Ports.Unused.
```

## Evidence and configuration

Use `profile: calibration` to select this check together with
`SOLID-I/consumer-role`. Findings identify the owning type, field, and named
interface, and carry `flow=unread` in their evidence. Disabled checks,
selected profile, architecture filters, suppressions, and baseline v5 filtering
remain authoritative.

## Limitations and remediation

The analyzer intentionally does not attempt whole-program alias analysis. A
dependency passed through an unknown helper is considered live. Remove a truly
unused constructor/dependency-bag field and its composition assignment; do not
delete an interface operation that has other consumers. For intentional future
wiring, document the reason with a narrow suppression or baseline v5 entry.
