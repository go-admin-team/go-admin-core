#!/usr/bin/env bash
# Builds downstream consumers against the working tree of this repository.
#
# A written compatibility promise stops nobody; this is the check that fails
# when a change breaks a consumer. Each repository is built through a
# throw-away go.work file, so no consumer's go.mod is ever modified.
#
# Usage:
#   scripts/canary.sh /path/to/consumer [/path/to/another ...]
#   CANARY_REPOS="/path/a:/path/b" scripts/canary.sh
#
# Consumers still pinned to the pre-merge layout (separate go-admin-core/sdk
# module) cannot be built this way; see docs/known-issues.md.

set -uo pipefail

CORE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

repos=("$@")
if [ ${#repos[@]} -eq 0 ] && [ -n "${CANARY_REPOS:-}" ]; then
	IFS=':' read -r -a repos <<<"$CANARY_REPOS"
fi

if [ ${#repos[@]} -eq 0 ]; then
	echo "canary: no consumer repositories given" >&2
	echo "usage: scripts/canary.sh /path/to/consumer [...]" >&2
	exit 2
fi

cleanup_workfile() {
	rm -f "$1/go.work" "$1/go.work.sum"
}

failed=0

for repo in "${repos[@]}"; do
	name=$(basename "$repo")

	if [ ! -f "$repo/go.mod" ]; then
		echo "SKIP $name (no go.mod at $repo)"
		continue
	fi

	if [ -f "$repo/go.work" ]; then
		echo "SKIP $name (a go.work already exists; refusing to overwrite)"
		continue
	fi

	trap 'cleanup_workfile "$repo"' EXIT

	(cd "$repo" && go work init . "$CORE_DIR" >/dev/null 2>&1)

	# The exit status is the only verdict. cgo dependencies emit compiler
	# warnings on some platforms; those are not build failures and must not
	# be inferred from the output.
	output=$(cd "$repo" && go build ./... 2>&1)
	status=$?

	cleanup_workfile "$repo"
	trap - EXIT

	if [ "$status" -eq 0 ]; then
		echo "PASS $name"
	else
		echo "FAIL $name"
		echo "$output" | grep -vE 'warning:|^\s+\.\./' | head -15 | sed 's/^/    /'
		failed=1
	fi
done

exit $failed
