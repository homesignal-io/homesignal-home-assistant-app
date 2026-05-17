#!/usr/bin/env bash
set -euo pipefail

ENVIRONMENT="${1:-}"
REGION="${HOMESIGNAL_AWS_REGION:-${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}}"
SECRET_NAME="${HOMESIGNAL_DATABASE_URL_SECRET_NAME:-/homesignal/${ENVIRONMENT}/platform/database_url}"

if [[ "$ENVIRONMENT" != "staging" ]]; then
  echo "Usage: scripts/rotate-db-credentials.sh staging" >&2
  exit 2
fi

if ! command -v aws >/dev/null 2>&1; then
  echo "Missing required command: aws" >&2
  exit 127
fi

echo "Database credential rotation is ready for AWS secret validation, but still needs the Neon account/API boundary."
echo "Validated AWS secret target:"
echo "  region: $REGION"
echo "  secret: $SECRET_NAME"

if aws secretsmanager describe-secret --region "$REGION" --secret-id "$SECRET_NAME" >/dev/null 2>&1; then
  echo "AWS secret metadata exists."
else
  echo "AWS secret metadata does not exist yet. Run scripts/deploy.sh staging before rotation work." >&2
fi

echo "Next rotation implementation input: Neon project/database credential policy and a Neon API token or manual rotation runbook."
exit 2
