# solidlint

`solidlint` finds explainable SOLID design smells in Go code. It complements
`golangci-lint`; it does not prove that a program is SOLID.

Version 0.1 requires Go 1.25. The default `stable` profile runs exactly seven
corpus-backed checks: `SOLID-S/large-type`, `SOLID-S/data-clump`,
`SOLID-O/type-dispatch`, all three `SOLID-I/*` checks, and
`SOLID-D/concrete-dependency`. Use `-profile=all` for all 27 checks or
`-enable-checks=SOLID-S/complex-function,...` for granular experimental opt-in.

The tool uses `golang.org/x/tools/go/packages` to load a consistent module
universe and `go/types` facts for cross-package checks. Its default `auto`
analysis mode collects type information when possible and falls back to
syntax-only heuristics when a package cannot be fully resolved.

`-analysis=syntax` never type-checks and runs only checks declared equivalent or
conservative in the registry. `-analysis=types` requires complete types for the
requested universe and exits 2 when resolution fails. `-analysis=auto` warns
once per incomplete package and runs only that package's syntax-capable checks.
A types-required check never emits from an incomplete package. Type information is used for SRP
metrics, interface contracts, embedded interfaces, concrete dependencies, and
module-wide OCP correlation; all checks remain explicitly heuristic.

## Installation

`solidlint` requires Go 1.25 when installed from source. Pin a release in local
and CI automation so upgrades remain deliberate:

```sh
go install github.com/ExtroNovosib/solidify@v0.1.0
solidlint -version
solidlint -fail=false ./...
```

Prebuilt Linux, macOS, and Windows archives for amd64 and arm64 are available
from [GitHub Releases](https://github.com/ExtroNovosib/solidify/releases). Each
release includes SHA-256 checksums and archive SBOMs. Prebuilt binaries do not
require a local Go toolchain.

For GitHub Actions, keep installation and enforcement explicit:

```yaml
- uses: actions/setup-go@v7
  with:
    go-version: "1.25.x"
- run: go install github.com/ExtroNovosib/solidify@v0.1.0
- run: solidlint ./...
```

Use the standalone CLI when you need all checks, module-wide correlations,
architecture policy, baselines, JSON, or SARIF. The GolangCI-Lint module plugin
described below intentionally exposes only package-scoped analyzers.

## Usage

```sh
go build -o solidlint .
./solidlint ./...
./solidlint -profile=all -fail=false ./...
./solidlint -enable-checks=SOLID-S/complex-function ./...
./solidlint ./internal/analyzer/
./solidlint -analysis=syntax -rules=S,I ./internal/...
./solidlint -tests -fail-level=error ./...
./solidlint -format=json ./... > findings.json
./solidlint -format=sarif ./... > findings.sarif
```

Supported rules are `S` (single responsibility), `O` (open/closed), `L`
(Liskov substitution), `I` (interface segregation), and `D` (dependency
inversion). Findings are heuristics: configure thresholds and review them in
the context of the codebase.

OCP checks include repeated/large type dispatch, repeated discriminator
branches, runtime-exhaustiveness defaults, concrete parameters used only
through methods, closed factories, configured implementation-package coupling,
and structurally parallel implementations. The existing
`SOLID-S/flag-argument` check covers flag arguments so they are not duplicated
under OCP.

DIP checks include zero-config concrete field/constructor dependencies plus
opt-in architecture-aware rules driven by the same `architecture:` lists as
OCP implementation coupling. `SOLID-D/layer-import` emits one finding per
forbidden import (threshold 1), while `SOLID-O/implementation-coupling`
aggregates implementation imports per package (default threshold 2). Additional
architecture-aware DIP checks flag hidden construction inside factories,
implementation wiring outside composition roots, infrastructure error leaks in
logic packages, and transport types in logic signatures.

| Check ID | What it flags |
|----------|---------------|
| `SOLID-D/concrete-dependency` | Struct field or `New*` constructor depends on a concrete `*Type` instead of an interface (works without architecture config) |
| `SOLID-D/layer-import` | Logic package imports an implementation package or built-in detail import (`database/sql`, `net/http`, …) |
| `SOLID-D/hidden-construction` | `New*` / `Provide*` factory constructs an implementation type into a returned struct field instead of receiving it via a parameter |
| `SOLID-D/wiring-outside-root` | Implementation constructor called outside a configured composition root (suppressed when already reported as hidden construction) |
| `SOLID-D/infra-error-leak` | Logic package compares against infrastructure sentinels such as `sql.ErrNoRows` |
| `SOLID-D/transport-leak` | Logic function exposes transport types such as `*http.Request` or `*sql.Tx` in parameters or results |

All `SOLID-D/*` checks are disable-able via `disabled_checks` and
`//solidify:ignore SOLID-D/<check-id> reason`.

LSP checks are deliberately narrow and require type information. They flag an
`io.Reader`-compatible `Read` method that reconstructs or wraps `io.EOF`
(callers may compare EOF with `==`) and an embedded interface that supplies
promoted methods but is never initialized anywhere in the analyzed workspace.
The latter is advisory because initialization outside the scanned workspace
cannot be proven absent. Unsupported/no-op interface methods remain ISP
findings, while type switches remain OCP findings.

Type-level SRP metrics aggregate receivers across every file in a package.
Pass package directories (for example `./internal/analyzer/`), not single
definition files such as `run.go`. When every CLI path is a `.go` file,
`solidlint` prints a stderr tip reminding you to scan the package directory.

## Output

`-format=text`, `-format=json`, and `-format=sarif` are supported. A finding
causes exit status 1 by default; pass `-fail=false` for report-only jobs.
`-fail-level=note|warning|error` selects the minimum severity that fails
(the default is `warning`). Pass `-tests` to include `_test.go` files, and use
`-version` to print the build version. The CLI also supports the thresholds
`-max-methods`, `-max-func-lines`, `-max-params`, `-max-switch-cases`, and
`-max-interface-methods`, `-isp-min-methods`, and `-isp-usage-ratio-percent`.
Defaults are `max-params=8` and `max-interface-methods=8` so cohesive Go
workflow helpers and moderately sized ports stay quiet. For discovery on large
packages, prefer `-rules S,I` and `-fail=false`; use baselines before turning
`-fail` on mega-packages.

### Canonical validation gate

Local development uses one reliability entry point:

```sh
make check
```

`make check` runs formatting, `go vet`, unit and race tests, analyzer coverage
floor, `golangci-lint`, self-enforcement, smoke and precision corpora, plugin
build, SARIF schema regression tests, JSON schema validation, version injection
checks, and `govulncheck`. Use it before pushing.

Keep report-only scans separate from enforcement. In this repository,
`make report` and `make enforce` scan `./internal/analyzer/...` so the tool
lints its own analyzer implementation. Fixture corpora under `testdata/` are
not part of that self-check; `make enforce` also reads `.solidlint-enforce.yml`
and fails on findings not listed in the checked-in `.solidlint-baseline.json`.
`make precision` verifies the fixture contract: the positive corpus produces
the expected findings and the clean corpus produces none.
`make lint` runs both `golangci-lint` and `make enforce`.

JSON is strict schema version 3. Each result has `fingerprintVersion: 4`, a
stable rule `id`, `maturity`, `subject`, `identity`, `severity`, `evidence`,
source location, and a portable fingerprint that survives
checkout-path and line-only changes. Repository paths in JSON and SARIF are
relative to the containing module root; external files are kept unambiguous.
Text output may show absolute filesystem paths for local readability; JSON and
SARIF default to portable relative paths. External findings preserve an
unambiguous URI or path and may not map to repository annotations in code
review UIs.
The output schema is checked in at `schemas/solidlint-result-v3.schema.json`.
SARIF output is SARIF 2.1.0 and includes rule metadata, locations, GitHub's
`primaryLocationLineHash`, and standards-compliant suggested fixes.

### Configuration, suppressions, and baselines

`solidlint` discovers `.solidify.yml` upwards from each scanned target, or uses
`-config path`. Targets that resolve to different configuration scopes must be
scanned separately or with an explicit `-config`; policy never silently depends
on the shell's current directory. CLI thresholds, rules, and `fail-level` take
precedence.
`allow_dependencies` applies to concrete dependencies reported by both DIP and
OCP checks.

```yaml
enabled_rules: [S, I, D]
exclude: [vendor/**, generated/**]
disabled_checks: [SOLID-S/data-clump]
allow_dependencies: [log.Logger]
fail_level: error
thresholds:
  max_methods: 8
  min_large_type_signals: 4
  min_import_cluster_methods: 3
  isp_min_methods: 3
  isp_usage_ratio_percent: 50
  ocp_min_dispatch_sites: 2
  ocp_min_shared_variants: 2
  ocp_dispatch_overlap_percent: 60
  ocp_min_concrete_parameter_methods: 2
  ocp_min_implementation_imports: 2
  ocp_min_parallel_functions: 3
  ocp_min_parallel_nodes: 20
  ocp_parallel_similarity_percent: 90
severity:
  SOLID-I/fat-interface: error
  SOLID-I/usage-ratio: warning
  SOLID-I/stub-implementation: warning
ocp:
  discriminator_fields: [Kind, Type, Status, Mode, Variant]
  allow_dispatch_types: []
  allow_packages: [github.com/ExtroNovosib/solidify/internal/parser/**]
architecture:
  logic_packages: [github.com/ExtroNovosib/solidify/internal/service/**, github.com/ExtroNovosib/solidify/internal/usecase/**]
  implementation_packages: [github.com/ExtroNovosib/solidify/internal/providers/**, github.com/ExtroNovosib/solidify/internal/adapters/**]
  composition_roots: [github.com/ExtroNovosib/solidify/cmd/**, github.com/ExtroNovosib/solidify/internal/app/**]
dip:
  infra_error_packages: [database/sql]
  transport_types: [net/http.Request, net/http.ResponseWriter, database/sql.Tx]
```

Suppress one finding with a specific rule and explanation, on the finding line,
the preceding line, or elsewhere in the owning declaration header:
`//solidify:ignore SOLID-I/fat-interface legacy RPC API`. A directive in a
multi-line function signature covers all matching parameter findings in that
signature, but not findings from the function body.

Exclude patterns use documented globs. `**` matches across path segments;
`generated/**` excludes any `generated` directory prefix. Malformed patterns
such as unclosed `[` are rejected at configuration load by
`ValidateExcludePatterns` instead of being treated as silent non-matches.
Excluded files are removed before metrics and related-location construction, so
they do not affect SRP aggregates or related locations on included findings.

Accepted DIP debt and other intentional findings can be baselined:

```sh
solidlint -write-baseline .solidlint-baseline.json ./...
git add .solidlint-baseline.json
solidlint -baseline .solidlint-baseline.json ./...
```

`-write-baseline` records v4 portable fingerprints for current findings (after
`exclude` patterns) using an atomic replacement. `-baseline` filters accepted fingerprints;
stale fingerprints that no longer match any finding are reported on stderr
without failing the run by default. Older baseline versions are rejected with
an explicit regeneration error. Version 4 identities remain portable across
checkout locations. Use `-baseline-stale=error` when stale entries should fail
the run, or `-baseline-stale=ignore` to suppress the notice.

The default package cache is stored in the platform user cache, namespaced by
the analysis root. Use `-cache-dir` to relocate it or `-cache=false` to disable
it. `-cache-debug` prints cache diagnostics to stderr. `-print-config` prints
the resolved, machine-readable configuration without loading or analyzing
packages.

Architecture package lists are intentionally opt-in. Composition roots are
excluded from implementation-coupling and DIP layer/wiring/leak findings
because wiring concrete implementations is their responsibility. When
`logic_packages` or `implementation_packages` are unset, architecture-aware DIP
checks no-op; `SOLID-D/concrete-dependency` continues to work without config.

Run `make precision` to execute the reproducible positive/negative corpus
gate before enabling a rule by default.

### Troubleshooting

- **Stale baseline notices:** regenerate with `-write-baseline` or use
  `-baseline-stale=ignore` / `-baseline-stale=error` as needed.
- **Configuration errors:** strict YAML rejects unknown fields, thresholds,
  disabled checks, and severity targets before analysis starts.
- **Cache surprises:** disable with `-cache=false` or relocate with
  `-cache-dir`; use `-cache-debug` for hit/miss diagnostics.
- **Plugin load failures:** build with `make plugin` and point golangci-lint at
  `./bin/solidlint.so`; plugins require a compatible Go toolchain and are
  primarily supported on Linux.
- **SARIF annotation gaps:** ensure repository artifacts use relative URIs with
  the bundled `ROOT` uriBase mapping; external files may not annotate in GitHub.

### Package directory scans

Type-level SRP metrics (`large-type`, `god-type`, cohesion,
`mixed-import-clusters`) aggregate every `.go` file in the same directory and
package name. Scanning a single definition file under-reports method counts
when receivers are spread across sibling files. Pass a package directory (for
example `./internal/analyzer/`) rather than one file such as `run.go`.

DIP field and constructor findings are suppressed in packages matching
`architecture.composition_roots`. Field findings are also suppressed on
zero-config composition roots that wire at least five concrete collaborators
(`dip_composition_root_fields`, default 5) and on thin bridge adapters with at
most two concrete struct dependencies and three relevant fields. Same-package
concrete types, `*Config` data bags, and behaviorless structs from domain
packages are ignored; cross-package concrete behavior dependencies still flag.
Vendor or intentional concrete types can be allowlisted with
`dip.allow_dependencies` or baselined. Standard-library concrete types (for example
`*net.UDPConn`) are ignored when type information is available. `large-type`
skips passive DTOs (no methods), DTOs with a few accessors, thin wrappers with
one or two fields, cohesive types whose TCC meets `min_tcc_percent`, and
homogeneous multi-bucket managers whose methods share the same structural
shape. By default it also requires four independent size signals
(`min_large_type_signals`), except for extreme surfaces with more than twice
the method limit and more than the field limit. Serialized data carriers
(mostly `json`, `yaml`, `toml`, `xml`, or `mapstructure` fields with no
behavior or mostly accessors) are excluded from low-cohesion and field-level
DIP findings. Types whose names end in `Handler` are also excluded from
`low-cohesion-type` because command and HTTP handlers are usually intentional
orchestrators. `http.ResponseWriter` and `context.Context` are excluded from
`usage-ratio` because callers typically touch only a small part of the stdlib
surface. `usage-ratio` examines exported function/method parameters and
interface-typed fields on exported consumers, aggregating field use across
receiver methods. Unexported helpers' parameter types are implementation
details. It analyzes module-owned interfaces, not interfaces from dependencies
such as database drivers; introduce a local port when that dependency needs ISP
review. Packages matching `architecture.composition_roots` are skipped. When
`architecture.logic_packages` is configured, it acts as an include filter so
business consumers remain covered without adapter noise. A concrete field
deliberately returned through its owning type's API is also treated as an
explicit concrete contract rather than a field-level DIP violation.

## GolangCI-Lint plugins

The recommended integration is the GolangCI-Lint v2.12.2 module plugin. It is
compiled into a custom GolangCI-Lint binary; installing `solidlint` does not add
it to the stock `golangci-lint` executable.

Add `.custom-gcl.yml` to the consuming repository:

```yaml
version: v2.12.2
name: golangci-lint-solidlint
destination: ./.bin
plugins:
  - module: github.com/ExtroNovosib/solidify
    import: github.com/ExtroNovosib/solidify/plugin/solidlint
    version: v0.1.0
```

Enable the module plugin in `.golangci.yml`:

```yaml
version: "2"
linters:
  default: none
  enable:
    - solidlint
  settings:
    custom:
      solidlint:
        type: module
        description: explainable package-scoped SOLID checks
        settings:
          profile: stable
```

Build and run the pinned custom executable:

```sh
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 custom -v
./.bin/golangci-lint-solidlint run ./...
```

The adapter is registered as `solidlint` from `plugin/solidlint`. Commit both
configuration files, but normally ignore the generated `.bin` directory.

The compatibility shared-object entry point is `cmd/solidlint-golangci`. Build
it with exactly the same Go 1.25.x toolchain, CGO setting, and overlapping
dependency graph as the host. Releases never distribute host-specific `.so`
files.

Build the compatibility plugin from a Solidify checkout:

```sh
make plugin
# equivalent to:
go build -tags plugin -buildmode=plugin -o bin/solidlint.so ./cmd/solidlint-golangci
```

Compatibility plugin settings use GolangCI-Lint v2 syntax:

```yaml
version: "2"
linters:
  default: none
  enable:
    - solidlint
  settings:
    custom:
      solidlint:
        type: goplugin
        path: ./bin/solidlint.so
        description: host-built solidlint compatibility plugin
        settings:
          profile: stable
```

The plugins expose exactly the nine SRP checks, `SOLID-L/non-exact-eof`, all
three ISP checks, and `SOLID-D/concrete-dependency`. Every OCP check,
`SOLID-L/nil-embedded-interface`, and the other five DIP checks remain CLI-only
program correlations. Plugins do not load baselines or CLI architecture
configuration. Use the CLI for those policies and for whole-program checks.

## Current limits

Type information improves interface-size analysis today. SRP type findings use
package-wide WMC, TCC, LCOM4, fan-out, and ATFD metrics; strict `god-type` and
`low-cohesion-type` findings are withheld when a package cannot be resolved.
`SOLID-S/mixed-import-clusters` (note severity) groups methods by shared
external import paths and complements LCOM4; it does not replace semantic
review of mixed concerns inside one package. Syntax-only runs retain advisory
checks such as `complex-function`, `mixed-input-surface`, `data-clump`, and
`flag-argument`.

OCP’s module-wide checks use canonical `go/types` identities and related
locations. In syntax mode, only local AST signals such as large type switches
are available; repeated interface, discriminator, factory, and concrete-
parameter checks require a complete package type graph.

Baseline v4 is the only accepted runtime format. Regenerate older baselines
after reviewing the new stable subjects and identities. Legacy YAML and rule-ID
aliases are intentionally rejected in this pre-release contract transition.
