# SOLID-I/stub-implementation

Reports interface implementations forced to panic or return `errors.ErrUnsupported` for an operation they cannot support.

## Product contract

Maturity: **stable**

Analysis modes: types required; unavailable in syntax-only mode.

Surfaces: standalone CLI and both GolangCI plugin modes.

## Examples

Positive: [stable positive evaluation fixture](../../../testdata/evaluation/positive/stable.go).

Clean: [stable negative evaluation fixture](../../../testdata/evaluation/negative/stable.go).

Evaluation case `stable-stub-implementation` is in the [positive fixture](../../../testdata/evaluation/positive/stable.go); the [negative fixture](../../../testdata/evaluation/negative/stable.go) implements every method normally.

```go
package example

import "errors"
type Repository interface { Get() error; Save() error }
type ReadOnlyRepository struct{}
func (ReadOnlyRepository) Get() error { return nil }
func (ReadOnlyRepository) Save() error { return errors.ErrUnsupported }
```

## Evidence and configuration

Evidence identifies the method, implemented interface, and stub kind. `isp_min_methods` limits the interface surfaces considered. Complete type information is required to prove the method participates in an interface implementation.

## Limitations and remediation

Temporary migration adapters and explicitly optional protocols can legitimately reject operations. Prefer splitting capability interfaces when callers can depend on the smaller contract. The analyzer no longer offers suppression insertion as a safe fix; suppression and baseline v5 remain explicit review workflows. See the [stable v0.2 evaluation](../../evaluations/stable-v0.2.md).
