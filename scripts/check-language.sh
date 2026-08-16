#!/usr/bin/env bash
# Rejects CJK characters in newly added Go code and in commit messages.
#
# Only the diff against the base branch is inspected: existing Chinese
# comments are a separate, tracked cleanup and must not block unrelated work.
# Chinese remains allowed in issues, pull request discussion and README.zh-CN.md.
#
# Usage: scripts/check-language.sh [base-ref]   (default: origin/main)

set -euo pipefail

BASE="${1:-origin/main}"

if ! git rev-parse --verify --quiet "$BASE" >/dev/null; then
	echo "check-language: base ref '$BASE' not found" >&2
	exit 2
fi

MERGE_BASE=$(git merge-base "$BASE" HEAD)

python3 - "$MERGE_BASE" <<'PY'
import re
import subprocess
import sys

CJK = re.compile(r'[一-鿿　-〿＀-￯]')
base = sys.argv[1]
failed = False


def run(*args):
    return subprocess.run(args, capture_output=True, text=True, check=True).stdout


added = []
diff = run('git', 'diff', '--unified=0', f'{base}...HEAD', '--', '*.go')
path = None
for line in diff.splitlines():
    if line.startswith('+++ b/'):
        path = line[6:]
    elif line.startswith('+') and not line.startswith('+++'):
        if CJK.search(line):
            added.append((path, line[1:].strip()))

if added:
    failed = True
    print('CJK characters in newly added Go code:')
    for path, text in added[:20]:
        print(f'  {path}: {text[:100]}')
    if len(added) > 20:
        print(f'  ... and {len(added) - 20} more')
    print()

subjects = run('git', 'log', '--format=%H%x00%s%x00%b', f'{base}..HEAD')
bad = []
for entry in subjects.split('\n'):
    if not entry.strip():
        continue
    parts = entry.split('\0')
    if len(parts) < 2:
        continue
    sha, subject = parts[0], parts[1]
    body = parts[2] if len(parts) > 2 else ''
    if CJK.search(subject) or CJK.search(body):
        bad.append((sha[:8], subject))

if bad:
    failed = True
    print('CJK characters in commit messages:')
    for sha, subject in bad:
        print(f'  {sha} {subject[:80]}')
    print()

if failed:
    print('See CONTRIBUTING.md: English is the default for everything committed.')
    sys.exit(1)

print('check-language: ok')
PY
