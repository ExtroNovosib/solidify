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

echo "publish release argument tests passed"
