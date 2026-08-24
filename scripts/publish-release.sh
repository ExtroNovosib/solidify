#!/bin/sh

set -eu

usage() {
	cat <<'EOF'
Usage: scripts/publish-release.sh [--dry-run] [--yes] [--skip-checks] vMAJOR.MINOR.PATCH

Validate and publish an immutable solidlint release tag. Pushing the tag starts
the GitHub Release workflow, which qualifies the external consumer and uploads
the platform archives, checksums, and SBOMs.

Options:
  --dry-run      Validate and print the publishing commands without running
                 release checks, creating a tag, or pushing it.
  --yes          Do not ask for confirmation before creating and pushing a tag.
  --skip-checks  Skip make check-release. Intended only when the exact release
                 qualification has already passed for the current commit.
  -h, --help     Show this help.

Environment:
  RELEASE_REMOTE  Git remote to publish to (default: origin).
  RELEASE_BRANCH  Required release branch (default: main).
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
[ -z "$(git status --porcelain)" ] || die "working tree is not clean; commit or remove release changes first"

git remote get-url "$remote" >/dev/null 2>&1 || die "remote $remote does not exist"
git fetch --quiet "$remote" "$branch" --tags

head_commit=$(git rev-parse HEAD)
remote_commit=$(git rev-parse "refs/remotes/$remote/$branch")
[ "$head_commit" = "$remote_commit" ] || die "HEAD is not the published $remote/$branch commit; push or update the branch first"

if git rev-parse --verify --quiet "refs/tags/$version" >/dev/null; then
	die "local tag $version already exists; release tags are immutable"
fi
if git ls-remote --exit-code --tags "$remote" "refs/tags/$version" >/dev/null 2>&1; then
	die "remote tag $version already exists; choose a new version"
fi

grep -Fq "github.com/ExtroNovosib/solidify/cmd/solidlint@$version" README.md ||
	die "README installation examples do not reference $version"
grep -Fq "version: $version" README.md ||
	die "README plugin examples do not reference $version"

echo "Release candidate: $version"
echo "Commit:            $head_commit"
echo "Destination:       $remote/$branch"

if [ "$dry_run" = true ]; then
	if [ "$skip_checks" = false ]; then
		echo "Would run: make check-release SOLIDLINT_VERSION=$head_commit"
	fi
	echo "Would run: git tag -a $version -m 'solidlint $version'"
	echo "Would run: git push $remote refs/tags/$version"
	exit 0
fi

if [ "$skip_checks" = false ]; then
	make check-release SOLIDLINT_VERSION="$head_commit"
fi

if [ "$assume_yes" = false ]; then
	printf "Create and push immutable release tag %s? [y/N] " "$version"
	read -r answer
	case "$answer" in
		y | Y | yes | YES) ;;
		*) die "release cancelled" ;;
	esac
fi

git tag -a "$version" -m "solidlint $version"
if ! git push "$remote" "refs/tags/$version"; then
	echo "publish-release: tag push failed; local tag $version was kept for inspection" >&2
	echo "publish-release: after resolving the problem, push it with: git push $remote refs/tags/$version" >&2
	exit 1
fi

echo "Published tag $version. The GitHub Release workflow is now responsible for the binaries."
echo "Track it with: gh run list --workflow Release --limit 1"
