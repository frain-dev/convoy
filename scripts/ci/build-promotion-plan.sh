#!/usr/bin/env bash
#
# Parse a release line (YY.M) and compute the promotion plan outputs.
#
# Usage:
#   ./scripts/ci/build-promotion-plan.sh <release_line>
#
# Outputs (written to $GITHUB_OUTPUT):
#   release_year, release_month, next_cutoff_tag, base_tag, version

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <release_line>" >&2
  exit 1
fi

RELEASE_LINE="$1"

if ! [[ "${RELEASE_LINE}" =~ ^([0-9]{2})\.([0-9]{1,2})$ ]]; then
  echo "release_line must match YY.M, for example 26.3" >&2
  exit 1
fi

RELEASE_YEAR="${BASH_REMATCH[1]}"
RELEASE_MONTH=$((10#${BASH_REMATCH[2]}))
RELEASE_YEAR_FULL=$((2000 + 10#${RELEASE_YEAR}))

NEXT_MONTH=$((RELEASE_MONTH + 1))
NEXT_YEAR_FULL=${RELEASE_YEAR_FULL}
if [ "${NEXT_MONTH}" -gt 12 ]; then
  NEXT_MONTH=1
  NEXT_YEAR_FULL=$((RELEASE_YEAR_FULL + 1))
fi

NEXT_CUTOFF_TAG="$(printf 'release-cutoff-%04d-%02d' "${NEXT_YEAR_FULL}" "${NEXT_MONTH}")"
BASE_TAG="v${RELEASE_YEAR}.${RELEASE_MONTH}.0"
VERSION="$(./scripts/calver.sh "${RELEASE_YEAR}" "${RELEASE_MONTH}")"

echo "release_year=${RELEASE_YEAR}" >> "$GITHUB_OUTPUT"
echo "release_month=${RELEASE_MONTH}" >> "$GITHUB_OUTPUT"
echo "next_cutoff_tag=${NEXT_CUTOFF_TAG}" >> "$GITHUB_OUTPUT"
echo "base_tag=${BASE_TAG}" >> "$GITHUB_OUTPUT"
echo "version=${VERSION}" >> "$GITHUB_OUTPUT"
