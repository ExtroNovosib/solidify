# Rule maturity and graduation

The default `stable` profile is deliberately conservative. A stable rule must have zero false positives and zero false negatives in the curated verdict corpus, deterministic identities, documented mode behavior, and complete positive, boundary, and clean examples. Every stable rule must have a unique case in `testdata/evaluation/stable-v0.2.json`; the named positive and negative fixture roots must execute to the declared ID, path, subject, and verdict. `make precision` rejects missing, duplicate, and orphan cases.

An experimental rule does not graduate merely because synthetic fixtures pass. Graduation requires a separately recorded manual evaluation on representative real Go projects, review of false-positive and false-negative examples, an updated report under `docs/evaluations/`, and an explicit registry maturity change in a later release. The manifest and report are review evidence, not generated claims of correctness.

The standalone CLI is the complete program analyzer. GolangCI plugins are package-scoped and expose only the nine SRP checks, `SOLID-L/non-exact-eof`, the three ISP checks, and `SOLID-D/concrete-dependency`.
