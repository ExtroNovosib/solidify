#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
publisher="$script_dir/publish-release.sh"

"$publisher" --help | grep -q 'vMAJOR.MINOR.PATCH'

for invalid in 0.2.0 v0.2 v0.02.0 v0.2.0.1 v0.2.x; do
	if "$publisher" --dry-run "$invalid" >/dev/null 2>&1; then
		echo "publish script accepted invalid version: $invalid" >&2
		exit 1
	fi
done

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/solidlint-publish-test.XXXXXX")
cleanup() {
	rm -rf "$work_dir"
}
trap cleanup EXIT HUP INT TERM

git init --bare --quiet "$work_dir/remote.git"
mkdir -p "$work_dir/repo/scripts"
cp "$publisher" "$work_dir/repo/scripts/publish-release.sh"
chmod +x "$work_dir/repo/scripts/publish-release.sh"
mkdir -p "$work_dir/bin"

cat >"$work_dir/bin/go" <<'EOF'
#!/bin/sh
set -eu

[ "$1" = install ]
[ "$2" = github.com/anchore/syft/cmd/syft@v1.51.0 ]
[ -n "${GOBIN:-}" ]
mkdir -p "$GOBIN"
printf '#!/bin/sh\nexit 0\n' >"$GOBIN/syft"
chmod +x "$GOBIN/syft"
EOF
chmod +x "$work_dir/bin/go"

cat >"$work_dir/bin/goreleaser" <<'EOF'
#!/bin/sh
exit 0
EOF
chmod +x "$work_dir/bin/goreleaser"

cat >"$work_dir/repo/README.md" <<'EOF'
go install github.com/ExtroNovosib/solidify/cmd/solidlint@v0.1.0
    version: v0.1.0
make publish VERSION=v0.1.0
EOF

cat >"$work_dir/repo/Makefile" <<'EOF'
.PHONY: check release-snapshot release-consumer-smoke

check:

release-snapshot:
	@test -n "$(GORELEASER)"
	@test "$(GORELEASER_CURRENT_TAG)" = v0.2.0
	@command -v syft >/dev/null

release-consumer-smoke:
	@test -n "$(SOLIDLINT_VERSION)"
EOF

(
	cd "$work_dir/repo"
	git init --quiet -b main
	git config user.name "solidlint release test"
	git config user.email "release-test@solidlint.invalid"
	git remote add origin "$work_dir/remote.git"
	git add README.md scripts/publish-release.sh
	git commit --quiet -m "initial"
	git push --quiet -u origin main
	echo "release content" >release.txt
	PATH="$work_dir/bin:$PATH" ./scripts/publish-release.sh --yes v0.2.0 >/dev/null
	[ -x .cache/release-tools/syft ]
	grep -Fq 'cmd/solidlint@v0.2.0' README.md
	grep -Fq 'version: v0.2.0' README.md
	grep -Fq 'VERSION=v0.2.0' README.md
	[ -z "$(git status --porcelain)" ]
	[ "$(git log -1 --format=%s)" = "Release solidlint v0.2.0" ]
)

git --git-dir="$work_dir/remote.git" rev-parse --verify refs/heads/main >/dev/null
git --git-dir="$work_dir/remote.git" rev-parse --verify refs/tags/v0.2.0 >/dev/null

echo "publish release tests passed"
