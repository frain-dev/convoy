#!/usr/bin/env bash
#
# Calculate CalVer version in the format: YY.M.PATCH
# Usage: ./scripts/calver.sh
#
# Outputs the version to stdout (e.g., "26.2.0")

set -euo pipefail

# Get current year (2-digit) and month
YEAR=$(date -u +%y)
MONTH=$(date -u +%-m)

# Find the highest patch number for this year.month
PREFIX="v${YEAR}.${MONTH}."
# Escape dots for sed regex since they're special characters
PREFIX_ESCAPED="v${YEAR}\\.${MONTH}\\."
LATEST_PATCH=$(git tag --list "${PREFIX}*" | sed "s/^${PREFIX_ESCAPED}//" | sort -n | tail -1)

if [ -z "$LATEST_PATCH" ]; then
  PATCH=0
else
  PATCH=$((LATEST_PATCH + 1))
fi

echo "${YEAR}.${MONTH}.${PATCH}"
