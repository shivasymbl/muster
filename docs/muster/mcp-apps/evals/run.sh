#!/usr/bin/env bash
set -euo pipefail
HERE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
if ! command -v node >/dev/null 2>&1; then
  printf '%s\n' '{"state":"not_ready","status":"blocked","reason":"Node.js is unavailable; runner cannot execute.","deployment_authorized":false}'
  mkdir -p "$HERE/results"
  printf '%s\n' '{"state":"not_ready","status":"blocked","reason":"Node.js is unavailable; runner cannot execute.","deployment_authorized":false}' > "$HERE/results/latest.json"
  exit 2
fi
exec node "$HERE/runner.mjs" "$@"
