#!/bin/sh

# Move the bullets under "## Unreleased" into a new "## VERSION" section
# directly below an emptied Unreleased heading. Fails when Unreleased holds no
# bullets or VERSION already has a section, so a release cannot ship without
# release notes or rewrite an existing section.

set -eu

die() {
	echo "rollover-changelog: $*" >&2
	exit 1
}

[ "$#" -ge 1 ] && [ "$#" -le 2 ] || {
	echo "usage: scripts/rollover-changelog.sh VERSION [CHANGELOG_PATH]" >&2
	exit 2
}
version=$1
changelog=${2:-CHANGELOG.md}

[ -n "$version" ] || die "VERSION must not be empty"
[ -f "$changelog" ] || die "$changelog does not exist"
grep -q '^## Unreleased[[:space:]]*$' "$changelog" || die "$changelog has no '## Unreleased' section"
if grep -Fxq "## $version" "$changelog"; then
	die "$changelog already has a '## $version' section"
fi

tmp=$(mktemp "${TMPDIR:-/tmp}/solidlint-changelog.XXXXXX")
trap 'rm -f "$tmp"' EXIT HUP INT TERM

# awk exits 3 when the Unreleased section contains no bullet.
status=0
awk -v version="$version" '
function flush(    first, last, i) {
	first = 1
	while (first <= count && body[first] ~ /^[[:space:]]*$/) first++
	last = count
	while (last >= first && body[last] ~ /^[[:space:]]*$/) last--
	print ""
	print "## " version
	print ""
	for (i = first; i <= last; i++) print body[i]
	if (!at_end) print ""
	flushed = 1
}
state == 0 && /^## Unreleased[[:space:]]*$/ { print; state = 1; next }
state == 1 && /^## / { flush(); state = 2; print; next }
state == 1 { body[++count] = $0; if ($0 ~ /^[-*] /) bullets++; next }
{ print }
END {
	if (state == 1) { at_end = 1; flush() }
	if (bullets == 0) exit 3
}
' "$changelog" >"$tmp" || status=$?
case "$status" in
	0) ;;
	3) die "$changelog has no Unreleased entries to release as $version" ;;
	*) die "could not rewrite $changelog" ;;
esac

cat "$tmp" >"$changelog"
