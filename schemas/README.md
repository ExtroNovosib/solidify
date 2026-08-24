# Bundled schemas

`sarif-schema-2.1.0.json` is the official OASIS SARIF 2.1.0 Errata 1
schema, pinned from:

https://github.com/oasis-tcs/sarif-spec/blob/main/sarif-2.1/schema/sarif-schema-2.1.0.json

It is bundled so `make sarif-check` validates reports without depending on
network availability.

`solidlint-result-v3.schema.json` strictly describes solidlint's JSON result
format, including typed nested objects and fingerprint version 4 identity.

`solidlint-config-v1.schema.json` is generated deterministically from the same
threshold and check registries used by runtime validation. `solidlint config
schema -format=json` emits the same document, and `make schema-check` guards
against drift.

`solidlint-baseline-v5.schema.json` describes annotated debt entries with
portable fingerprints, check IDs, paths, subjects, required review reasons,
and optional owners and expiry dates. Runtime readers retain baseline v4
compatibility, but every new write is canonical v5.

Schema ownership is split by artifact: `internal/report` owns result JSON and
SARIF encoding, `internal/config` owns config introspection, and
`internal/baseline` owns baseline validation and canonical writes.
