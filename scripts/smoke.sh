#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENVIRONMENT="${1:-}"
BASE_URL="${HOMESIGNAL_STAGING_BASE_URL:-}"

terraform_command() {
  if command -v tofu >/dev/null 2>&1; then
    echo "tofu"
    return 0
  fi
  if command -v terraform >/dev/null 2>&1; then
    echo "terraform"
    return 0
  fi
  return 1
}

if [[ "$ENVIRONMENT" != "staging" ]]; then
  echo "Usage: scripts/smoke.sh staging" >&2
  exit 2
fi

if [[ -z "$BASE_URL" ]]; then
  TF_BIN="$(terraform_command || true)"
  if [[ -n "$TF_BIN" ]]; then
    BASE_URL="$(
      cd "$ROOT/infra/envs/staging"
      "$TF_BIN" output -raw staging_base_url 2>/dev/null || true
    )"
  fi
fi

if [[ -z "$BASE_URL" ]]; then
  echo "Missing staging base URL." >&2
  echo "Set HOMESIGNAL_STAGING_BASE_URL or run from an applied staging Terraform workspace." >&2
  exit 2
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "Missing required command: curl" >&2
  exit 127
fi

BASE_URL="${BASE_URL%/}"
for path in /healthz /readyz /version; do
  echo "Checking $BASE_URL$path"
  curl -fsS "$BASE_URL$path" >/dev/null
done

echo "Smoke checks passed"
