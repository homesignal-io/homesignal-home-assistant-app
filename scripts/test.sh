#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v go >/dev/null 2>&1; then
  echo "Missing required command: go" >&2
  echo "Install Go 1.25.x or place go on PATH, then rerun scripts/test.sh." >&2
  exit 127
fi

MODULES=(
  backend
  telemetry-ingest
  homesignal
)

for module in "${MODULES[@]}"; do
  if [[ -f "$ROOT/$module/go.mod" ]]; then
    echo "Testing $module"
    (
      cd "$ROOT/$module"
      go test ./...
    )
  fi
done

if [[ -f "$ROOT/portal/package.json" ]]; then
  if ! command -v npm >/dev/null 2>&1; then
    echo "Missing required command: npm" >&2
    echo "Install Node.js/npm, then rerun scripts/test.sh." >&2
    exit 127
  fi

  echo "Testing portal"
  (
    cd "$ROOT/portal"
    npm test
  )
fi

echo "Tests passed"
