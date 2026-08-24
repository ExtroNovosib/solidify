#!/bin/sh

set -eu

module="github.com/ExtroNovosib/solidify"
command="$module/cmd/solidlint"
version="${SOLIDLINT_VERSION:-}"

if [ -z "$version" ]; then
	echo "SOLIDLINT_VERSION is required (for example, v0.1.0 or a commit hash)" >&2
	exit 2
fi

work_dir="$(mktemp -d "${TMPDIR:-/tmp}/solidlint-consumer-smoke.XXXXXX")"
cleanup() {
	chmod -R u+w "$work_dir" 2>/dev/null || true
	rm -rf "$work_dir"
}
trap cleanup EXIT HUP INT TERM

export GOBIN="$work_dir/bin"
export GOCACHE="$work_dir/go-build"
export GOMODCACHE="$work_dir/go-mod"
export GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}"

mkdir -p "$GOBIN" "$work_dir/consumer"

go install "$command@$version"

installed_version="$($GOBIN/solidlint -version)"
if [ "$installed_version" = "dev" ] || [ -z "$installed_version" ]; then
	echo "go install produced a binary without module version metadata" >&2
	exit 1
fi
case "$version" in
	v[0-9]*)
		if [ "$installed_version" != "$version" ]; then
			echo "solidlint -version = $installed_version, want $version" >&2
			exit 1
		fi
		;;
esac

cat >"$work_dir/consumer/go.mod" <<'EOF'
module consumer.example/solidlint-smoke

go 1.25.0
EOF

cat >"$work_dir/consumer/fat.go" <<'EOF'
package consumer

type Wide interface {
	A()
	B()
	C()
	D()
	E()
	F()
	G()
	H()
	I()
}
EOF

(
	cd "$work_dir/consumer"
	"$GOBIN/solidlint" -format=json -fail=false ./... >"$work_dir/cli.json"
	"$GOBIN/solidlint" check -format=json -fail=false ./... >"$work_dir/cli-check.json"
	cmp "$work_dir/cli.json" "$work_dir/cli-check.json"
	"$GOBIN/solidlint" checks list -format=json >"$work_dir/checks.json"
	"$GOBIN/solidlint" checks explain SOLID-I/fat-interface -format=json >"$work_dir/explain.json"
	"$GOBIN/solidlint" config init >.solidify.yml
	"$GOBIN/solidlint" config validate .solidify.yml
	"$GOBIN/solidlint" config schema -format=json >"$work_dir/config-schema.json"
	"$GOBIN/solidlint" stats -format=json ./... >"$work_dir/stats.json"
	"$GOBIN/solidlint" baseline init -baseline .solidlint-baseline.json -baseline-reason "Reviewed external smoke debt" -baseline-owner release-smoke ./...
)
grep -q 'SOLID-I/fat-interface' "$work_dir/cli.json"
grep -q '"version": 5' "$work_dir/consumer/.solidlint-baseline.json"
grep -q 'Reviewed external smoke debt' "$work_dir/consumer/.solidlint-baseline.json"

cat >"$work_dir/.custom-gcl.yml" <<EOF
version: v2.12.2
name: golangci-lint-solidlint
destination: $work_dir/bin
plugins:
  - module: $module
    import: $module/plugin/solidlint
    version: $version
EOF

cat >"$work_dir/consumer/.golangci.yml" <<'EOF'
version: "2"
linters:
  default: none
  enable: [solidlint]
  settings:
    custom:
      solidlint:
        type: module
        description: explainable package-scoped SOLID checks
        settings:
          profile: stable
          enabled_rules: [I]
          enabled_checks: [SOLID-I/fat-interface]
EOF

(
	cd "$work_dir"
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 custom -v
)

if (
	cd "$work_dir/consumer"
	"$GOBIN/golangci-lint-solidlint" run ./... >"$work_dir/plugin.log" 2>&1
); then
	echo "custom golangci-lint unexpectedly accepted the violation fixture" >&2
	exit 1
fi
grep -q 'SOLID-I/fat-interface' "$work_dir/plugin.log"

echo "external consumer smoke passed for $module@$version"
