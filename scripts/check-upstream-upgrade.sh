#!/bin/sh
# Fail when an upstream fork merge would conflict on Muster-owned paths.
# Usage: scripts/check-upstream-upgrade.sh [upstream-ref]
# With no ref, print the protected paths and exit 0.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$ROOT"

PROTECTED="
server/routes/mcp.go
server/routes/mcp_skills.go
server/routes/oauth.go
server/services/mcp
server/db/mcp_events.go
server/db/mcp_writes.go
docs/specs
docs/muster
docs/designs
deploy
Dockerfile
"

if [ $# -lt 1 ]; then
  echo "No upstream ref given. Protected paths:"
  printf '%s\n' "$PROTECTED" | sed '/^$/d'
  echo "Pass a fetched ref, for example: scripts/check-upstream-upgrade.sh upstream/main"
  exit 0
fi

REF=$1
git rev-parse --verify "$REF" >/dev/null
BASE=$(git merge-base HEAD "$REF")
TREE=$(git merge-tree "$BASE" HEAD "$REF")

fail=0
printf '%s\n' "$PROTECTED" | sed '/^$/d' | while read -r path; do
  if printf '%s\n' "$TREE" | grep -F "changed in both" | grep -F "$path" >/dev/null; then
    echo "conflict on protected path: $path"
    echo 1 >"$ROOT/.upgrade-conflict"
  fi
done

if [ -f "$ROOT/.upgrade-conflict" ]; then
  fail=1
  rm -f "$ROOT/.upgrade-conflict"
fi

if [ "$fail" -ne 0 ]; then
  echo "Upstream merge conflicts with Muster-owned files. Resolve those paths by hand. Do not take the upstream copy over MCP, auth, deploy, or spec files."
  exit 1
fi

echo "No protected-path conflict against $REF (base $BASE)."
