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
#
# The copy is migrated with coreupgrade before it is built, so that the check
# survives this module changing its own import paths. Without that step a
# consumer on the old paths quietly resolves them from the proxy and the build
# stops saying anything about this tree at all — which is what happened the
# moment the module moved to /v2.

set -uo pipefail

CORE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CORE_MOD="$(awk '/^module /{print $2; exit}' "$CORE_DIR/go.mod")"

# -v2 is only meaningful once this module carries a major version in its path.
major_flag=()
case "$CORE_MOD" in
*/v[0-9]*) major_flag=(-v2) ;;
esac

tooldir=$(mktemp -d)
cleanup_workfile() {
	rm -f "$1/go.work" "$1/go.work.sum"
}

current_repo=""
cleanup() {
	[ -n "$current_repo" ] && cleanup_workfile "$current_repo"
	rm -rf "$tooldir"
}
trap cleanup EXIT
if ! (cd "$CORE_DIR" && go build -o "$tooldir/coreupgrade" ./tools/coreupgrade); then
	echo "canary: could not build coreupgrade" >&2
	exit 1
fi

repos=("$@")
if [ ${#repos[@]} -eq 0 ] && [ -n "${CANARY_REPOS:-}" ]; then
	IFS=':' read -r -a repos <<<"$CANARY_REPOS"
fi

if [ ${#repos[@]} -eq 0 ]; then
	echo "canary: no consumer repositories given" >&2
	echo "usage: scripts/canary.sh /path/to/consumer [...]" >&2
	exit 2
fi

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

	current_repo="$repo"

	(cd "$repo" && go work init . "$CORE_DIR" >/dev/null 2>&1)

	# Bring the copy onto this tree's import paths. The repository on disk is
	# never touched: callers pass a checkout they are willing to lose, which
	# in CI is a throw-away one.
	"$tooldir/coreupgrade" -w "${major_flag[@]}" "$repo" >/dev/null 2>&1

	# A consumer that imports some other version of this module resolves it
	# from the proxy, builds happily, and says nothing whatsoever about this
	# tree. Asking whether the module is in the workspace does not catch that
	# — it always is. Ask whether the build actually depends on it.
	# Captured rather than piped into grep -q: this script runs with pipefail,
	# and grep leaving early sends go list a SIGPIPE that reads as failure.
	deps=$(cd "$repo" && go list -deps ./... 2>/dev/null)
	if ! printf '%s\n' "$deps" | grep -q "^$CORE_MOD/"; then
		echo "FAIL $name"
		echo "    nothing in $name depends on $CORE_MOD; the build did not use this tree"
		cleanup_workfile "$repo"
		current_repo=""
		failed=1
		continue
	fi

	# The exit status is the only verdict. cgo dependencies emit compiler
	# warnings on some platforms; those are not build failures and must not
	# be inferred from the output.
	output=$(cd "$repo" && go build ./... 2>&1)
	status=$?

	cleanup_workfile "$repo"
	current_repo=""

	if [ "$status" -eq 0 ]; then
		echo "PASS $name"
	else
		echo "FAIL $name"
		echo "$output" | grep -vE 'warning:|^\s+\.\./' | head -15 | sed 's/^/    /'
		failed=1
	fi
done

exit $failed
