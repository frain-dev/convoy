#!/usr/bin/env bash
#
# Create and push a release cutoff tag at HEAD if it does not already exist.
#
# Usage:
#   ./scripts/ci/create-cutoff-tag.sh <cutoff_tag>

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <cutoff_tag>" >&2
  exit 1
fi

CUTOFF_TAG="$1"
CURRENT_SHA="$(git rev-parse HEAD)"

if git rev-parse --verify "refs/tags/${CUTOFF_TAG}" >/dev/null 2>&1; then
  echo "Cutoff tag ${CUTOFF_TAG} already exists."
  exit 0
fi

git tag "${CUTOFF_TAG}" "${CURRENT_SHA}"
git push origin "refs/tags/${CUTOFF_TAG}"
