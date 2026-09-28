# SOLID-I/constructor-role

A constructor requires a broader interface than the narrower consumer field it stores.

## Product contract

- Maturity: **experimental**. Select `profile: all` or explicitly enable this check; the stable profile remains seven checks.
- Analysis modes: types required; withheld in syntax and incomplete-auto packages.
- Surfaces: standalone CLI, module plugin, and matched-ABI Go plugin.
- Evidence records the owned contract or method sets and related positions. Existing JSON v3 and SARIF formats are preserved.

## Examples

Positive:

```go
func NewRunner(s BroadStore) *Runner { return &Runner{store:s} }
```

Clean:

```go
func NewRunner(s executionStore) *Runner { return &Runner{store:s} }
```

## Limitations and remediation

Compares complete Go method sets and follows aliases and variadic indexing. Extra constructor-time methods, ambiguous aliases and unknown escapes prevent a recommendation. Review the reported evidence in context. Prefer a behavior-preserving correction or extraction. Intentional debt may use a reason-bearing suppression or a reviewed baseline v5 entry; this check does not supply automatic source fixes.
