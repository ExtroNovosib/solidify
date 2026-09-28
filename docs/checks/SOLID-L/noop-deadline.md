# SOLID-L/noop-deadline

A net.Conn deadline setter ignores its timestamp and unconditionally reports successful nil.

## Product contract

- Maturity: **experimental**. Select `profile: all` or explicitly enable this check; the stable profile remains seven checks.
- Analysis modes: types required; withheld in syntax and incomplete-auto packages.
- Surfaces: standalone CLI, module plugin, and matched-ABI Go plugin.
- Evidence records the owned contract or method sets and related positions. Existing JSON v3 and SARIF formats are preserved.

## Examples

Positive:

```go
func (c *Conn) SetDeadline(_ time.Time) error { return nil }
```

Clean:

```go
func (c *Conn) SetDeadline(t time.Time) error { return c.base.SetDeadline(t) }
```

## Limitations and remediation

Requires typed net.Conn satisfaction, including promoted methods, the standard setter signature and ignored input. Forwarding, applied state and unrelated methods remain quiet. Review the reported evidence in context. Prefer a behavior-preserving correction or extraction. Intentional debt may use a reason-bearing suppression or a reviewed baseline v5 entry; this check does not supply automatic source fixes.
