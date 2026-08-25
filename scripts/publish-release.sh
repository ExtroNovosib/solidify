#!/bin/sh

set -eu

usage() {
	cat <<'EOF'
Usage: scripts/publish-release.sh [--dry-run] [--yes] [--skip-checks] vMAJOR.MINOR.PATCH

Prepare, qualify, commit, push, and tag a solidlint release. Pushing the tag
starts the GitHub Release workflow, which uploads the platform archives,
checksums, and SBOMs.

Options:
  --dry-run      Validate and print the publishing operations without changing
                 files, running checks, committing, pushing, or tagging.
  --yes          Do not ask for confirmation before committing and publishing.
  --skip-checks  Skip local qualification and the public-consumer smoke test.
                 Intended only when both passed for the exact release content.
  -h, --help     Show this help.

Environment:
  RELEASE_REMOTE  Git remote to publish to (default: origin).
  RELEASE_BRANCH  Required release branch (default: main).
  RELEASE_COMMIT_MESSAGE  Release commit message (default: Release solidlint VERSION).
  GORELEASER      Existing GoReleaser executable to use instead of the pinned local tool.
  SYFT            Existing Syft executable to use instead of the pinned local tool.
EOF
}

die() {
	echo "publish-release: $*" >&2
	exit 1
}

dry_run=false
assume_yes=false
skip_checks=false
version=""

while [ "$#" -gt 0 ]; do
	case "$1" in
		--dry-run) dry_run=true ;;
		--yes) assume_yes=true ;;
		--skip-checks) skip_checks=true ;;
		-h | --help)
			usage
			exit 0
			;;
		-*) die "unknown option: $1" ;;
		*)
			[ -z "$version" ] || die "provide exactly one release version"
			version="$1"
			;;
	esac
	shift
done

[ -n "$version" ] || {
	usage >&2
	exit 2
}

case "$version" in
	v0 | v0.*[!0-9.]* | v[!0-9]* | *.*.*.* | v*.*.*)
		# The stricter checks below reject empty and leading-zero components.
		;;
	*) die "version must use vMAJOR.MINOR.PATCH format (for example, v0.2.0)" ;;
esac

plain_version=${version#v}
major=${plain_version%%.*}
remainder=${plain_version#*.}
minor=${remainder%%.*}
patch=${remainder#*.}
for component in "$major" "$minor" "$patch"; do
	case "$component" in
		"" | *[!0-9]*) die "version must use vMAJOR.MINOR.PATCH format" ;;
		0) ;;
		0*) die "version components must not contain leading zeroes" ;;
	esac
done

remote=${RELEASE_REMOTE:-origin}
branch=${RELEASE_BRANCH:-main}
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH= cd -- "$script_dir/.." && pwd)
cd "$repo_root"

command -v git >/dev/null 2>&1 || die "git is required"
command -v make >/dev/null 2>&1 || die "make is required"

[ "$(git branch --show-current)" = "$branch" ] || die "releases must be published from branch $branch"
git remote get-url "$remote" >/dev/null 2>&1 || die "remote $remote does not exist"
if [ "$dry_run" = true ]; then
	remote_commit=$(git ls-remote --heads "$remote" "refs/heads/$branch" | awk 'NR == 1 { print $1 }')
	[ -n "$remote_commit" ] || die "remote branch $remote/$branch does not exist"
	git cat-file -e "$remote_commit^{commit}" 2>/dev/null ||
		die "remote $branch contains commits not available locally; fetch it before the dry run"
else
	git fetch --quiet "$remote" "$branch" --tags
	remote_commit=$(git rev-parse "refs/remotes/$remote/$branch")
fi
git merge-base --is-ancestor "$remote_commit" HEAD ||
	die "local $branch has diverged from or is behind $remote/$branch; synchronize it first"

if git rev-parse --verify --quiet "refs/tags/$version" >/dev/null; then
	die "local tag $version already exists; release tags are immutable"
fi
if git ls-remote --exit-code --tags "$remote" "refs/tags/$version" >/dev/null 2>&1; then
	die "remote tag $version already exists; choose a new version"
fi

echo "Release candidate: $version"
echo "Destination:       $remote/$branch"
echo "Current changes:"
git status --short

if [ "$dry_run" = true ]; then
	echo "Would update release-version examples in README.md to $version"
	if [ "$skip_checks" = false ]; then
		echo "Would run: make check"
		echo "Would bootstrap pinned GoReleaser v2.17.1 when no executable is configured"
		echo "Would run: make release-snapshot with that GoReleaser executable"
	fi
	echo "Would stage all repository changes and create a release commit when needed"
	echo "Would run: git push $remote $branch"
	if [ "$skip_checks" = false ]; then
		echo "Would run the external-consumer smoke test against the pushed commit"
	fi
	echo "Would run: git tag -a $version -m 'solidlint $version'"
	echo "Would run: git push $remote refs/tags/$version"
	exit 0
fi

readme_tmp=$(mktemp "${TMPDIR:-/tmp}/solidlint-readme.XXXXXX")
cleanup() {
	rm -f "$readme_tmp"
}
trap cleanup EXIT HUP INT TERM
sed -E \
	-e "s|(github.com/ExtroNovosib/solidify/cmd/solidlint@)v[0-9]+\.[0-9]+\.[0-9]+|\\1$version|g" \
	-e "s|^([[:space:]]+version: )v[0-9]+\.[0-9]+\.[0-9]+$|\\1$version|" \
	-e "s|(SOLIDLINT_VERSION=)v[0-9]+\.[0-9]+\.[0-9]+|\\1$version|g" \
	-e "s|(VERSION=)v[0-9]+\.[0-9]+\.[0-9]+|\\1$version|g" \
	README.md >"$readme_tmp"
if ! cmp -s README.md "$readme_tmp"; then
	cp "$readme_tmp" README.md
fi

grep -Fq "github.com/ExtroNovosib/solidify/cmd/solidlint@$version" README.md ||
	die "could not update README installation examples to $version"
grep -Fq "version: $version" README.md ||
	die "could not update README plugin examples to $version"

echo "Release change set:"
git status --short
git diff --stat

if [ "$assume_yes" = false ]; then
	printf "Qualify, commit all changes, push %s, and publish %s? [y/N] " "$branch" "$version"
	read -r answer
	case "$answer" in
		y | Y | yes | YES) ;;
		*) die "release cancelled" ;;
	 esac
fi

if [ "$skip_checks" = false ]; then
	make check
	if [ -n "${GORELEASER:-}" ]; then
		goreleaser_bin=$GORELEASER
	elif command -v goreleaser >/dev/null 2>&1; then
		goreleaser_bin=$(command -v goreleaser)
	else
		goreleaser_dir="$repo_root/.cache/release-tools"
		goreleaser_bin="$goreleaser_dir/goreleaser"
		if [ ! -x "$goreleaser_bin" ]; then
			echo "Installing pinned GoReleaser v2.17.1 into $goreleaser_dir"
			mkdir -p "$goreleaser_dir"
			GOBIN="$goreleaser_dir" go install github.com/goreleaser/goreleaser/v2@v2.17.1
		fi
	fi
	[ -x "$goreleaser_bin" ] || die "GoReleaser executable is not available: $goreleaser_bin"
	if [ -n "${SYFT:-}" ]; then
		syft_bin=$SYFT
	elif command -v syft >/dev/null 2>&1; then
		syft_bin=$(command -v syft)
	else
		syft_dir="$repo_root/.cache/release-tools"
		syft_bin="$syft_dir/syft"
		if [ ! -x "$syft_bin" ]; then
			echo "Installing pinned Syft v1.51.0 into $syft_dir"
			mkdir -p "$syft_dir"
			GOBIN="$syft_dir" go install github.com/anchore/syft/cmd/syft@v1.51.0
		fi
	fi
	[ -x "$syft_bin" ] || die "Syft executable is not available: $syft_bin"
	syft_dir=$(CDPATH= cd -- "$(dirname -- "$syft_bin")" && pwd)
	PATH="$syft_dir:$PATH" GORELEASER_CURRENT_TAG="$version" make release-snapshot GORELEASER="$goreleaser_bin"
fi

git add -A
if ! git diff --cached --quiet; then
	commit_message=${RELEASE_COMMIT_MESSAGE:-Release solidlint $version}
	git commit -m "$commit_message"
fi

git push "$remote" "HEAD:refs/heads/$branch"
release_commit=$(git rev-parse HEAD)

if [ "$skip_checks" = false ]; then
	make release-consumer-smoke SOLIDLINT_VERSION="$release_commit"
fi

git tag -a "$version" -m "solidlint $version"
if ! git push "$remote" "refs/tags/$version"; then
	echo "publish-release: tag push failed; local tag $version was kept for inspection" >&2
	echo "publish-release: after resolving the problem, push it with: git push $remote refs/tags/$version" >&2
	exit 1
fi

echo "Published $version from $release_commit. The GitHub Release workflow is now building the binaries."
echo "Track it with: gh run list --workflow Release --limit 1"
