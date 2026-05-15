#!/usr/bin/env bash
set -euo pipefail

ENVIRONMENT="${1:-}"
REGION="${HOMESIGNAL_AWS_REGION:-${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}}"

if [[ "$ENVIRONMENT" != "staging" ]]; then
  echo "Usage: scripts/logs.sh staging" >&2
  exit 2
fi

if ! command -v aws >/dev/null 2>&1; then
  echo "Missing required command: aws" >&2
  exit 127
fi

aws logs tail /homesignal/staging/control-plane --follow --region "$REGION"
