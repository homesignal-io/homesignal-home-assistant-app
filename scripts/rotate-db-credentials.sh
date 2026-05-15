#!/usr/bin/env bash
set -euo pipefail

echo "Database credential rotation is not part of the first deployment slice." >&2
echo "Keep this command failing until the database boundary is added intentionally." >&2
exit 2
