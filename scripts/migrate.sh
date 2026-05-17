#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENVIRONMENT="${1:-}"
ACTION="${2:-plan}"
REGION="${HOMESIGNAL_AWS_REGION:-${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}}"
SECRET_NAME="${HOMESIGNAL_DATABASE_URL_SECRET_NAME:-/homesignal/${ENVIRONMENT}/platform/database_url}"

fail() {
  echo "$1" >&2
  exit "${2:-1}"
}

require_command() {
  local command_name="$1"
  if ! command -v "$command_name" >/dev/null 2>&1; then
    fail "Missing required command: $command_name" 127
  fi
}

usage() {
  cat >&2 <<USAGE
Usage: scripts/migrate.sh staging [plan|status|status-if-configured|up]

Set HOMESIGNAL_DATABASE_URL or store the plain PostgreSQL URL in:
  $SECRET_NAME
USAGE
}

if [[ "$ENVIRONMENT" != "staging" ]]; then
  usage
  exit 2
fi

case "$ACTION" in
  plan|status|status-if-configured|up)
    ;;
  *)
    usage
    exit 2
    ;;
esac

require_command go

resolve_database_url() {
  if [[ -n "${HOMESIGNAL_DATABASE_URL:-}" ]]; then
    echo "$HOMESIGNAL_DATABASE_URL"
    return 0
  fi
  if [[ -n "${DATABASE_URL:-}" ]]; then
    echo "$DATABASE_URL"
    return 0
  fi
  if ! command -v aws >/dev/null 2>&1; then
    return 1
  fi

  local value
  value="$(
    aws secretsmanager get-secret-value \
      --region "$REGION" \
      --secret-id "$SECRET_NAME" \
      --query SecretString \
      --output text 2>/dev/null || true
  )"
  if [[ -z "$value" || "$value" == "None" ]]; then
    return 1
  fi
  echo "$value"
}

run_migrator() {
  local mode="$1"
  local database_url="${2:-}"
  (
    cd "$ROOT/backend"
    if [[ -n "$database_url" ]]; then
      HOMESIGNAL_DATABASE_URL="$database_url" go run ./cmd/migrate -mode "$mode" -dir migrations
    else
      go run ./cmd/migrate -mode "$mode" -dir migrations
    fi
  )
}

if [[ "$ACTION" == "plan" ]]; then
  run_migrator plan
  exit 0
fi

DATABASE_URL_VALUE="$(resolve_database_url || true)"
if [[ -z "$DATABASE_URL_VALUE" ]]; then
  if [[ "$ACTION" == "status-if-configured" ]]; then
    echo "Database URL is not configured yet; skipping migration status."
    echo "Create a Neon Postgres database in us-east-1 and store its URL in $SECRET_NAME."
    exit 0
  fi
  fail "Missing database URL. Set HOMESIGNAL_DATABASE_URL, DATABASE_URL, or populate $SECRET_NAME." 2
fi

if [[ "$ACTION" == "status-if-configured" ]]; then
  ACTION="status"
fi

run_migrator "$ACTION" "$DATABASE_URL_VALUE"
