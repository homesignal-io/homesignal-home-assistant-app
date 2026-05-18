#!/usr/bin/env bash
set -euo pipefail

ENVIRONMENT="${1:-staging}"
REGION="${HOMESIGNAL_AWS_REGION:-${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}}"
SECRET_NAME="${HOMESIGNAL_DATABASE_URL_SECRET_NAME:-/homesignal/${ENVIRONMENT}/platform/database_url}"

fail() {
  echo "$1" >&2
  exit "${2:-1}"
}

if [[ "$ENVIRONMENT" != "staging" ]]; then
  fail "Usage: scripts/set-staging-database-url.sh [staging]"
fi

if ! command -v aws >/dev/null 2>&1; then
  fail "Missing required command: aws" 127
fi

database_url="${HOMESIGNAL_DATABASE_URL:-${DATABASE_URL:-}}"
if [[ -z "$database_url" ]]; then
  if [[ ! -t 0 ]]; then
    fail "Missing database URL. Run from an interactive terminal or set HOMESIGNAL_DATABASE_URL." 2
  fi
  printf "Paste the current Neon staging PostgreSQL URL: " >&2
  restore_echo() {
    stty echo 2>/dev/null || true
  }
  trap restore_echo EXIT
  stty -echo
  read -r database_url
  restore_echo
  trap - EXIT
  printf "\n" >&2
fi

if [[ "$database_url" != postgresql://* && "$database_url" != postgres://* ]]; then
  fail "Refusing to store a value that does not look like a PostgreSQL URL." 2
fi

aws secretsmanager put-secret-value \
  --region "$REGION" \
  --secret-id "$SECRET_NAME" \
  --secret-string "$database_url" >/dev/null

unset database_url

echo "Updated $SECRET_NAME in $REGION."
echo "Run: scripts/migrate.sh staging status"
