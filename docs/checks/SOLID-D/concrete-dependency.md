# SOLID-D/concrete-dependency

Reports high-level constructors or fields coupled to behavioral concrete types from another package.

## Product contract

Maturity: **stable**

Analysis modes: conservative syntax fallback; full type information is used when available.

Surfaces: standalone CLI and both GolangCI plugin modes.

## Examples

Positive: [stable positive evaluation fixture](../../../testdata/evaluation/positive/stable.go).

Clean: [stable negative evaluation fixture](../../../testdata/evaluation/negative/stable.go).

Evaluation case `stable-concrete-dependency` uses the cross-package client in [the violations fixture](../../../testdata/violations/additional.go). The [negative fixture](../../../testdata/evaluation/negative/stable.go) shows a same-package concrete collaborator, which is intentionally ignored.

```go
package service

import "tempmod/database"
type ArchiveService struct{}
func NewArchiveService(client *database.PostgreSQLClient) *ArchiveService {
	_ = client
	return &ArchiveService{}
}
```

## Evidence and configuration

Evidence records the constructor or field and fully qualified concrete dependency. Same-package types, behaviorless domain data, `*Config` bags, composition roots, and entries in `dip.allow_dependencies` are excluded. Types improve classification; conservative syntax fallback remains available.

## Limitations and remediation

Concrete dependencies are appropriate in composition roots, adapters, and stable vendor contracts. Introduce a consumer-owned behavioral interface only when substitution or test seams are real requirements. There is no universal safe rewrite. Use a justified suppression or annotated baseline v5 entry for reviewed exceptions. See the [stable v0.2 evaluation](../../evaluations/stable-v0.2.md).
