# Calibration profile

`profile: calibration` is an opt-in, high-precision ISP review lane. It runs
only these experimental package checks:

- `SOLID-I/consumer-role`
- `SOLID-I/unused-dependency`

It does not turn off user policy: `disabled_checks`, architecture filters,
thresholds, suppressions, baselines, and an explicit CLI profile/check override
remain authoritative.

For the reference backend oracle, run the dedicated configuration against the
reviewed production scope:

```sh
solidlint check \
  -config testdata/calibration/reference-backend.yml \
  -fail=false \
  /path/to/backend/internal
```

The configuration uses a 61-percent numeric fallback. It is deliberately below
the 70–80 percent range that admits cohesive 2-of-3 and high-usage-store noise;
the capability-aware part of the profile covers the remaining read/write
splits.
