#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENVIRONMENT="${1:-}"
EMAIL="${2:-${HOMESIGNAL_STAGING_PORTAL_USER_EMAIL:-}}"
DISPLAY_NAME="${3:-${HOMESIGNAL_STAGING_PORTAL_USER_DISPLAY_NAME:-}}"
REGION="${HOMESIGNAL_AWS_REGION:-${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}}"
SECRET_NAME="${HOMESIGNAL_DATABASE_URL_SECRET_NAME:-/homesignal/${ENVIRONMENT}/platform/database_url}"
COGNITO_USER_POOL_ID="${HOMESIGNAL_COGNITO_USER_POOL_ID:-}"
COGNITO_SUB="${HOMESIGNAL_COGNITO_SUB:-}"

fail() {
  echo "$1" >&2
  exit "${2:-1}"
}

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

usage() {
  cat >&2 <<USAGE
Usage: scripts/bootstrap-staging-portal-user.sh staging <email> [display-name]

Creates or reuses a Cognito staging portal user, resolves its Cognito sub, and
seeds matching Postgres users/account membership rows for staging smoke data.

Optional overrides:
  HOMESIGNAL_COGNITO_USER_POOL_ID
  HOMESIGNAL_COGNITO_SUB
  HOMESIGNAL_DATABASE_URL
USAGE
}

if [[ "$ENVIRONMENT" != "staging" || -z "$EMAIL" ]]; then
  usage
  exit 2
fi

if ! command -v go >/dev/null 2>&1; then
  fail "Missing required command: go" 127
fi

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

terraform_output() {
  local name="$1"
  local tf_bin
  tf_bin="$(terraform_command || true)"
  if [[ -z "$tf_bin" ]]; then
    return 1
  fi
  (
    cd "$ROOT/infra/envs/staging"
    "$tf_bin" output -raw "$name" 2>/dev/null || true
  )
}

if [[ -z "$COGNITO_USER_POOL_ID" ]]; then
  COGNITO_USER_POOL_ID="$(terraform_output cognito_user_pool_id)"
fi

if [[ -z "$COGNITO_SUB" ]]; then
  if [[ -z "$COGNITO_USER_POOL_ID" ]]; then
    fail "Missing Cognito user pool ID. Deploy staging Cognito first or set HOMESIGNAL_COGNITO_USER_POOL_ID." 2
  fi
  if ! command -v aws >/dev/null 2>&1; then
    fail "Missing required command: aws" 127
  fi

  echo "Ensuring Cognito portal user exists for $EMAIL"
  aws cognito-idp admin-create-user \
    --region "$REGION" \
    --user-pool-id "$COGNITO_USER_POOL_ID" \
    --username "$EMAIL" \
    --user-attributes "Name=email,Value=$EMAIL" "Name=email_verified,Value=true" \
    --desired-delivery-mediums EMAIL >/dev/null 2>&1 || true

  COGNITO_SUB="$(
    aws cognito-idp admin-get-user \
      --region "$REGION" \
      --user-pool-id "$COGNITO_USER_POOL_ID" \
      --username "$EMAIL" \
      --query 'UserAttributes[?Name==`sub`].Value | [0]' \
      --output text
  )"
fi

if [[ -z "$COGNITO_SUB" || "$COGNITO_SUB" == "None" ]]; then
  fail "Could not resolve Cognito sub for $EMAIL." 1
fi

DATABASE_URL_VALUE="$(resolve_database_url || true)"
if [[ -z "$DATABASE_URL_VALUE" ]]; then
  fail "Missing database URL. Populate $SECRET_NAME or set HOMESIGNAL_DATABASE_URL." 2
fi

(
  cd "$ROOT/backend"
  HOMESIGNAL_DATABASE_URL="$DATABASE_URL_VALUE" go run ./cmd/staging-portal-user \
    -email "$EMAIL" \
    -display-name "$DISPLAY_NAME" \
    -cognito-sub "$COGNITO_SUB"
)
