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

rollover="$script_dir/rollover-changelog.sh"
empty_changelog="$work_dir/empty-CHANGELOG.md"
printf '# Changelog\n\n## Unreleased\n\n## v0.1.0\n\n- First release.\n' >"$empty_changelog"
if "$rollover" v0.2.0 "$empty_changelog" >/dev/null 2>&1; then
	echo "rollover accepted an empty Unreleased section" >&2
	exit 1
fi
grep -Fxq '## Unreleased' "$empty_changelog"
if grep -Fxq '## v0.2.0' "$empty_changelog"; then
	echo "failed rollover modified the changelog" >&2
	exit 1
fi

git init --bare --quiet "$work_dir/remote.git"
mkdir -p "$work_dir/repo/scripts"
cp "$publisher" "$work_dir/repo/scripts/publish-release.sh"
cp "$rollover" "$work_dir/repo/scripts/rollover-changelog.sh"
chmod +x "$work_dir/repo/scripts/publish-release.sh" "$work_dir/repo/scripts/rollover-changelog.sh"
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

cat >"$work_dir/repo/CHANGELOG.md" <<'EOF'
# Changelog

## Unreleased

- Add the release rollover.

## v0.1.0

- First release.
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
	git add README.md CHANGELOG.md scripts/publish-release.sh scripts/rollover-changelog.sh
	git commit --quiet -m "initial"
	git push --quiet -u origin main
	echo "release content" >release.txt
	dry_run=$(PATH="$work_dir/bin:$PATH" ./scripts/publish-release.sh --dry-run v0.2.0)
	printf '%s\n' "$dry_run" | grep -Fq 'Would move CHANGELOG.md Unreleased entries under v0.2.0'
	grep -Fq -- '- Add the release rollover.' CHANGELOG.md
	if grep -Fxq '## v0.2.0' CHANGELOG.md; then
		echo "dry run modified CHANGELOG.md" >&2
		exit 1
	fi
	PATH="$work_dir/bin:$PATH" ./scripts/publish-release.sh --yes v0.2.0 >/dev/null
	[ -x .cache/release-tools/syft ]
	grep -Fq 'cmd/solidlint@v0.2.0' README.md
	grep -Fq 'version: v0.2.0' README.md
	grep -Fq 'VERSION=v0.2.0' README.md
	expected_changelog=$(printf '# Changelog\n\n## Unreleased\n\n## v0.2.0\n\n- Add the release rollover.\n\n## v0.1.0\n\n- First release.')
	[ "$(cat CHANGELOG.md)" = "$expected_changelog" ] || {
		echo "unexpected CHANGELOG.md after release:" >&2
		cat CHANGELOG.md >&2
		exit 1
	}
	git show --name-only --format= HEAD | grep -Fxq CHANGELOG.md
	[ -z "$(git status --porcelain)" ]
	[ "$(git log -1 --format=%s)" = "Release solidlint v0.2.0" ]
)

git --git-dir="$work_dir/remote.git" rev-parse --verify refs/heads/main >/dev/null
git --git-dir="$work_dir/remote.git" rev-parse --verify refs/tags/v0.2.0 >/dev/null

echo "publish release tests passed"
