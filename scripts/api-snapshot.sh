#!/usr/bin/env bash
# Regenerates api/core.txt, a snapshot of every exported signature.
#
# The snapshot is committed so that API changes show up in the pull request
# diff itself. A rename that keeps the name but changes the signature — the
# failure mode this repository has already shipped once, SetDb(key, db)
# becoming SetDb(db) — is then visible during review instead of at runtime.
#
# Doc comments are stripped: they are prose, they change independently of the
# API, and much of the existing prose is not yet in English.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="$ROOT/api/core.txt"

mkdir -p "$(dirname "$OUT")"

{
	echo "# Exported API of github.com/go-admin-team/go-admin-core"
	echo "# Regenerate with: make api-snapshot"
	echo

	cd "$ROOT"
	go list ./... | sort | while read -r pkg; do
		body=$(go doc -all "$pkg" 2>/dev/null \
			| grep -E '^(func |type |const |var |	)' \
			| grep -vE '^	*//' || true)
		[ -z "$body" ] && continue
		echo "## $pkg"
		echo "$body"
		echo
	done
} >"$OUT"

echo "wrote $OUT ($(wc -l <"$OUT" | tr -d ' ') lines)"
