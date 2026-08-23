# Contributing

solidlint targets Go 1.25. Before submitting a rule change, run `make check` and confirm the rule has all of the following:

- a stable concrete ID, default severity, maturity, scope, syntax capability, surfaces, and safe-fix declaration in the registry;
- a check page describing intent, signals, thresholds, modes, surfaces, evidence, metrics, examples, limitations, configuration, suppression, baselines, and safe remediation;
- positive, boundary or near-miss, and clean fixtures with exact IDs, subjects, identities, and relevant metrics;
- expectations for every supported analysis mode and plugin surface;
- no regression in portable paths, fingerprint uniqueness, cache cold/warm parity, JSON/SARIF schemas, or compile-safe suggested fixes.

Experimental graduation additionally requires stable corpus results and a recorded manual evaluation on representative real Go projects. Synthetic fixtures alone are not sufficient.

Do not commit generated release artifacts or host-specific Go shared-object plugins.
