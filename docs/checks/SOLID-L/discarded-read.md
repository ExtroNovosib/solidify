# SOLID-L/discarded-read

A standard byte-stream Read consumes a record, copies a prefix and returns io.ErrShortBuffer without retaining its unread suffix.

## Product contract

- Maturity: **experimental**. Select `profile: all` or explicitly enable this check; the stable profile remains seven checks.
- Analysis modes: types required; withheld in syntax and incomplete-auto packages.
- Surfaces: standalone CLI, module plugin, and matched-ABI Go plugin.
- Evidence records the owned contract or method sets and related positions. Existing JSON v3 and SARIF formats are preserved.

## Examples

Positive:

```go
n := copy(p, plaintext)
if n < len(plaintext) { return n, io.ErrShortBuffer }
```

Clean:

```go
c.pending = plaintext[n:] // drain this tail before consuming another record
```

## Limitations and remediation

Requires typed standard stream exposure, a consumed decoded record, related copy count and short-buffer branch. A record API, retained tail, unrelated short-buffer error or unknown flow is not sufficient. Review the reported evidence in context. Prefer a behavior-preserving correction or extraction. Intentional debt may use a reason-bearing suppression or a reviewed baseline v5 entry; this check does not supply automatic source fixes.
