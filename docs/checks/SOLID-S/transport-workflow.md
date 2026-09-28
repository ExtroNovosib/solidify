# SOLID-S/transport-workflow

A typed HTTP entry point owns domain operations and multiple ordered persistence writes.

## Product contract

- Maturity: **experimental**. Select `profile: all` or explicitly enable this check; the stable profile remains seven checks.
- Analysis modes: types required; withheld in syntax and incomplete-auto packages.
- Surfaces: standalone CLI, module plugin, and matched-ABI Go plugin.
- Evidence records the owned contract or method sets and related positions. Existing JSON v3 and SARIF formats are preserved.

## Examples

Positive:

```go
wf := domain.NewWorkflow(...)
h.deps.Store.CreateWorkflow(ctx,wf)
h.deps.Store.InsertVersion(ctx,version)
```

Clean:

```go
result, err := h.deps.CreateWorkflow.Handle(ctx,input)
writeJSON(w,result)
```

## Limitations and remediation

Naming, length, an HTTP import, single reads or a constructor alone are insufficient. Nested dependency bags and statically known receiver helpers preserve evidence. Decode-command-encode adapters remain quiet. Review the reported evidence in context. Prefer a behavior-preserving correction or extraction. Intentional debt may use a reason-bearing suppression or a reviewed baseline v5 entry; this check does not supply automatic source fixes.
