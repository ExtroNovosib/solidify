# Rule maturity and graduation

The default `stable` profile is deliberately conservative. A stable rule must have zero false positives and zero false negatives in the curated verdict corpus, deterministic identities, documented mode behavior, and complete positive, boundary, and clean examples.

An experimental rule does not graduate merely because synthetic fixtures pass. Graduation requires a separately recorded manual evaluation on representative real Go projects, review of false-positive and false-negative examples, and an explicit registry maturity change in a later release.

The standalone CLI is the complete program analyzer. GolangCI plugins are package-scoped and expose only the nine SRP checks, `SOLID-L/non-exact-eof`, the three ISP checks, and `SOLID-D/concrete-dependency`.
