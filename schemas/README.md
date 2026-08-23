# Bundled schemas

`sarif-schema-2.1.0.json` is the official OASIS SARIF 2.1.0 Errata 1
schema, pinned from:

https://github.com/oasis-tcs/sarif-spec/blob/main/sarif-2.1/schema/sarif-schema-2.1.0.json

It is bundled so `make sarif-check` validates reports without depending on
network availability.

`solidlint-result-v3.schema.json` strictly describes solidlint's JSON result
format, including typed nested objects and fingerprint version 4 identity.
