#!/usr/bin/env bash
# scripts/check_prs.sh – Show raw PR data for the repo

set -euo pipefail

# Repository details
REPO="Das-rebel/finwipe"
API="https://api.github.com/repos/$REPO/pulls"

echo "🔎 Pull Request Status Overview for $REPO"
echo "--------------------------------------------------"
PR_DATA=$(curl -s "$API")
echo "$PR_DATA"