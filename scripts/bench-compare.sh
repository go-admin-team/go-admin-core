#!/usr/bin/env bash
# Compares benchmark results between the working tree and a base revision.
#
# Allocation counts are gated by tests (see any alloc_budget_test.go); timings
# cannot be, because a threshold loose enough to survive a shared CI runner is
# too loose to catch anything. This script exists for the other half: it runs
# both revisions on one machine, back to back, and hands the numbers to
# benchstat so the noise is quantified rather than guessed at.
#
# Usage:
#   scripts/bench-compare.sh [base-ref] [bench-regexp] [count]
#
#   scripts/bench-compare.sh                 # against origin/main, all benchmarks
#   scripts/bench-compare.sh HEAD~1 Cache    # one revision back, cache only
#   scripts/bench-compare.sh main Enforce 10 # ten runs for a tighter interval
#
# The base revision is built in a throw-away git worktree, so the working tree
# is never touched and uncommitted changes are what gets measured.
#
# Read the output as benchstat intends: a delta with "~" means the difference
# did not clear the noise, however large the percentage looks. If everything
# reads "~", raise the count rather than the conclusion.
set -euo pipefail

BASE_REF="${1:-origin/main}"
PATTERN="${2:-.}"
COUNT="${3:-6}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

if ! git rev-parse --verify --quiet "$BASE_REF" >/dev/null; then
  echo "bench-compare: no such revision: $BASE_REF" >&2
  exit 1
fi

WORKDIR="$(mktemp -d)"
BASE_TREE="$WORKDIR/base"
cleanup() {
  git worktree remove --force "$BASE_TREE" >/dev/null 2>&1 || true
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

echo "==> base revision: $BASE_REF ($(git rev-parse --short "$BASE_REF"))"
git worktree add --detach "$BASE_TREE" "$BASE_REF" >/dev/null

run_bench() {
  local dir="$1" out="$2" label="$3"
  echo "==> benchmarking $label (count=$COUNT, pattern=$PATTERN)"
  # -run '^$' keeps ordinary tests out of the timings.
  ( cd "$dir" && go test -run '^$' -bench "$PATTERN" -benchmem -count "$COUNT" ./... ) \
    2>&1 | tee "$out" | grep -E '^(Benchmark|FAIL|ok)' || true
}

run_bench "$BASE_TREE" "$WORKDIR/base.txt" "base"
run_bench "$REPO_ROOT" "$WORKDIR/head.txt" "working tree"

echo
echo "==> benchstat base -> working tree"
go run golang.org/x/perf/cmd/benchstat@latest "$WORKDIR/base.txt" "$WORKDIR/head.txt"
