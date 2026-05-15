#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENVIRONMENT="${1:-}"
REGION="${HOMESIGNAL_AWS_REGION:-${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}}"
VERSION="${HOMESIGNAL_VERSION:-}"
BUDGET_ALERT_EMAIL="${HOMESIGNAL_BUDGET_ALERT_EMAIL:-}"
BUDGET_GUARDRAIL_CONFIRMED="${HOMESIGNAL_BUDGET_GUARDRAIL_CONFIRMED:-}"
MONTHLY_BUDGET_AMOUNT="${HOMESIGNAL_STAGING_BUDGET_AMOUNT:-25}"
OWNER_TAG="${HOMESIGNAL_OWNER_TAG:-platform}"

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
  fail "Usage: scripts/deploy.sh staging"
fi

if [[ "$REGION" != "us-east-1" ]]; then
  fail "First deploy is pinned to us-east-1 to stay colocated with staging Neon."
fi

require_command aws

TF_BIN="$(terraform_command || true)"
if [[ -z "$TF_BIN" ]]; then
  fail "Missing required command: tofu or terraform" 127
fi

export AWS_REGION="$REGION"
export AWS_DEFAULT_REGION="$REGION"

ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
CALLER_ARN="$(aws sts get-caller-identity --query Arn --output text)"
if [[ "$CALLER_ARN" == arn:aws:iam::*:root ]]; then
  fail "Refusing to deploy with root AWS credentials. Use a named deploy principal."
fi

if [[ -z "$VERSION" ]] && command -v git >/dev/null 2>&1; then
  VERSION="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || true)"
fi
VERSION="${VERSION:-dev}"

echo "Deploying HomeSignal staging"
echo "  account: $ACCOUNT_ID"
echo "  caller:  $CALLER_ARN"
echo "  region:  $REGION"
echo "  version: $VERSION"

if [[ -z "$BUDGET_ALERT_EMAIL" && "$BUDGET_GUARDRAIL_CONFIRMED" != "1" ]]; then
  fail "Missing staging budget guardrail. Set HOMESIGNAL_BUDGET_ALERT_EMAIL so Terraform can create it, or set HOMESIGNAL_BUDGET_GUARDRAIL_CONFIRMED=1 if it already exists." 2
fi

if [[ -z "$BUDGET_ALERT_EMAIL" && "$BUDGET_GUARDRAIL_CONFIRMED" == "1" ]]; then
  echo "Budget guardrail marked as already configured; Terraform will skip the AWS Budget resource."
fi

HOMESIGNAL_VERSION="$VERSION" "$ROOT/scripts/build.sh"

(
  cd "$ROOT/infra/envs/staging"
  "$TF_BIN" init
  "$TF_BIN" apply \
    -auto-approve \
    -var="aws_region=$REGION" \
    -var="lambda_package_path=$ROOT/backend/dist/control-plane/bootstrap.zip" \
    -var="version=$VERSION" \
    -var="budget_alert_email=$BUDGET_ALERT_EMAIL" \
    -var="monthly_budget_amount=$MONTHLY_BUDGET_AMOUNT" \
    -var="owner_tag=$OWNER_TAG"

  echo "Deploy complete"
  echo "  staging_base_url: $("$TF_BIN" output -raw staging_base_url)"
)
