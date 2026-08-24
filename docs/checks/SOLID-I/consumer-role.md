# SOLID-I/consumer-role

Reports an interface-typed consumer field whose actual role is materially
narrower than the injected interface. This experimental check is intended for
calibration and focused architectural review; the stable
`SOLID-I/usage-ratio` check remains the conservative default.

## Product contract

Maturity: **experimental**

Analysis modes: types required; unavailable in syntax-only mode and withheld
for incomplete packages in auto mode.

Surfaces: standalone CLI and both GolangCI plugin modes.

The check analyzes exported and unexported receiver types in configured logic
packages. It emits one finding per consumer field, preserving the receiver,
field, accepted interface identity, and sorted used method set in the stable
identity. Registry-style interfaces are excluded because a lookup registry is
normally a boundary object rather than a consumer-owned capability set.

The numeric fallback requires an interface with at least four methods and a
used-method percentage below `isp_usage_ratio_percent`. It also recognizes
high-confidence read-only/write-only capability leaks, such as a trace reader
receiving stage-mutation operations. Unknown interface escapes remain clean
rather than being guessed at.

## Examples

Positive: [consumer-role unit fixture](../../../internal/analyzer/isp_consumer_test.go)

Clean: [consumer-role unit fixture controls](../../../internal/analyzer/isp_consumer_test.go)

```go
type TraceStore interface {
	ListStages() error
	ListChunks() error
	GetChunks() error
	ListExtractions() error
	StartStage() error
	FinishStage() error
}

type TraceService struct{ trace TraceStore }

func (s *TraceService) Read() error {
	// A reader uses only the four query methods.
	return s.trace.ListStages()
}
```

## Evidence and configuration

Use `profile: calibration` to select this check together with
`SOLID-I/unused-dependency`. `isp_usage_ratio_percent` remains authoritative;
the bundled reference calibration configuration uses 61 percent to avoid the
known 66-percent small-interface false-positive boundary. Architecture
`logic_packages` and `composition_roots`, disabled checks, suppressions, and
baseline v5 filtering all remain authoritative.

The check assigns a warning to strong low-ratio, read/write, or queue-worker
role splits. It emits a note for lower-impact high-precision ratio findings.

## Limitations and remediation

This is a heuristic, not proof that every producer interface is wrong. Keep
aggregate interfaces at composition boundaries when they are genuinely wiring
objects. For a real finding, define the smallest consumer-owned role beside the
consumer and let the existing adapter satisfy it structurally. If the broader
contract is intentional, document a specific suppression or use baseline v5
with a review reason.
