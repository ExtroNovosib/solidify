# Tunnel server calibration

This calibration follows the complete `server-internal-solid-review-vs-solidify-2026-09-28.md` report at Tunnel HEAD `9666c0af7293bbd22832f65908d981521e29addc`. The portable [manifest](../../testdata/tunnel_calibration/manifest.json) retains 254 source records across CP-01–CP-18 and CN-01–CN-10: 70 primary positive declarations, 11 typed enum cluster members, 52 adjudicated negative controls, and 121 source-evidence/context records. Source-evidence records explain a receiver's responsibilities; they do not each demand a separate diagnostic.

Every primary/cluster positive has a corresponding corrected fixture. Every primary negative has an explicit control fixture. The analyzer harness pins each family's cardinality independently, validates known check IDs, verifies method sets and related locations, and resolves enum members through both primary and related positions. The precision target runs this exhaustive corpus alongside the existing stable corpus. Runtime probes additionally exercise all three broken/corrected stream implementations and all three deadline setters.

Run the portable proofs from the Solidify root:

```sh
GOCACHE="$PWD/.cache/go-build" go test ./internal/analyzer ./tests/integration ./tests/e2e -run '^TestTunnel' -count=1
GOCACHE="$PWD/.cache/go-build" make precision
```

Run the live comparison against a read-only checkout:

```sh
GOCACHE="$PWD/.cache/go-build" python3 scripts/verify-tunnel-calibration.py \
  --source /path/to/tunnel/apps/server --output .cache/tunnel-calibration
```

The script builds current Solidify and the unchanged tracked HEAD in temporary storage, analyzes `apps/server/internal` with strict types, resolves current declarations by package/symbol/owner, and matches each reviewed check, field/parameter identity, contract evidence, and cluster membership. It writes complete reports and `comparison.json`, including normal stable coverage, raw all-profile coverage, policy-lane coverage, exact commands/configurations, binary metadata, actual per-package type completeness and check coverage from `stats`, cache parity, baseline filtering, source suppressions and before/after source/config/baseline hashes. An optional `--before-binary` can reuse a previously built unchanged binary. A different Tunnel HEAD fails rather than silently accepting a stale review.

Coverage requires separate lanes:

| Lane | Scope and purpose |
| --- | --- |
| Existing config + stable | Normal default behavior; seven stable checks. Experimental omissions remain explicit. |
| Existing config + all | Typed contract checks, ordinary consumer/dependency findings, receiver smells and enum clusters. |
| Execution policy | Adds `isp.execution_methods: [TryClaimRuleUse]` without restricting application consumer packages; proves CP-16 and its `Evaluate` control. |
| [Wiring policy](../../testdata/calibration/tunnel-server.yml) | Configures the mixed root Tunnel package as logic for CP-11 adapter import/wiring, with explicit real composition roots. The policy omits the builtin `net/http` import detail to respect runtime-owned transport signatures. It is solely a wiring lane. |
| Application HTTP policy | Configures the four reviewed application imports under real application logic; proves both layer-import and transport-leak absence for status/Header-only uses. |

The wiring policy alone does not produce complete coverage. Architecture `logic_packages` also restricts ISP consumers; combining a root-only wiring policy with execution capabilities would hide application checks. The verifier therefore generates distinct local policies, never installs them in Tunnel, and evaluates each record in its applicable lane.

Findings remain heuristics. The comparison accepts only the adjudicated check at its semantic site. Unrelated metrics, flags, candidate findings elsewhere, and total diagnostic counts do not establish coverage. CN-10 sealed StepConfig codecs are checked specifically for the StepType identity; unrelated FailurePolicyKind clusters are outside that reviewed control. CN-03's facade method context does not assert that every substantive storage method is clean.

The implementation preserves JSON schema v3, fingerprint v4, SARIF 2.1.0 and annotated baseline v5 behavior. The configuration schema is regenerated for the optional execution-method field. Cache format v12 prevents reuse of findings from earlier analyzer semantics. The verifier's temporary baseline demonstrates filtering separately from raw calibration reports; it never changes a target baseline or source file.
