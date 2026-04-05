#!/usr/bin/env bash
#
# Compute the monthly release plan from the current UTC date.
# Derives the release version for the previous month and the
# corresponding cutoff tags.
#
# Usage:
#   ./scripts/ci/build-release-plan.sh
#
# Outputs (written to $GITHUB_OUTPUT):
#   version, release_line, current_cutoff_tag, release_cutoff_tag

set -euo pipefail

CURRENT_YEAR="$(date -u +%Y)"
CURRENT_MONTH_PADDED="$(date -u +%m)"

PREVIOUS_MONTH_DATE="$(date -u -d "$(date -u +%Y-%m-01) -1 day" +%Y-%m-%d)"
RELEASE_YEAR_FULL="$(date -u -d "${PREVIOUS_MONTH_DATE}" +%Y)"
RELEASE_YEAR="$(date -u -d "${PREVIOUS_MONTH_DATE}" +%y)"
RELEASE_MONTH="$(date -u -d "${PREVIOUS_MONTH_DATE}" +%-m)"
RELEASE_MONTH_PADDED="$(date -u -d "${PREVIOUS_MONTH_DATE}" +%m)"

RELEASE_VERSION="$(./scripts/calver.sh "${RELEASE_YEAR}" "${RELEASE_MONTH}")"
CURRENT_CUTOFF_TAG="release-cutoff-${CURRENT_YEAR}-${CURRENT_MONTH_PADDED}"
RELEASE_CUTOFF_TAG="release-cutoff-${RELEASE_YEAR_FULL}-${RELEASE_MONTH_PADDED}"
RELEASE_LINE="${RELEASE_YEAR}.${RELEASE_MONTH}"

echo "version=${RELEASE_VERSION}" >> "$GITHUB_OUTPUT"
echo "release_line=${RELEASE_LINE}" >> "$GITHUB_OUTPUT"
echo "current_cutoff_tag=${CURRENT_CUTOFF_TAG}" >> "$GITHUB_OUTPUT"
echo "release_cutoff_tag=${RELEASE_CUTOFF_TAG}" >> "$GITHUB_OUTPUT"
