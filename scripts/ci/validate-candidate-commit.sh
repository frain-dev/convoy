#!/usr/bin/env bash
#
# Validate that a candidate commit is eligible for a patch release:
#   - exists in the repository
#   - the base release tag and next cutoff tag both exist
#   - is reachable from origin/main
#   - is not newer than the next cutoff tag
#
# Usage:
#   ./scripts/ci/validate-candidate-commit.sh <candidate_sha> <next_cutoff_tag> <base_tag>
#
# Outputs (written to $GITHUB_OUTPUT):
#   next_cutoff_sha

set -euo pipefail

if [ "$#" -ne 3 ]; then
  echo "Usage: $0 <candidate_sha> <next_cutoff_tag> <base_tag>" >&2
  exit 1
fi

CANDIDATE_SHA="$1"
NEXT_CUTOFF_TAG="$2"
BASE_TAG="$3"

if ! git rev-parse --verify "${CANDIDATE_SHA}^{commit}" >/dev/null 2>&1; then
  echo "Commit '${CANDIDATE_SHA}' does not exist." >&2
  exit 1
fi

if ! git rev-parse --verify "refs/tags/${BASE_TAG}" >/dev/null 2>&1; then
  echo "Base release tag ${BASE_TAG} does not exist yet." >&2
  exit 1
fi

if ! git rev-parse --verify "refs/tags/${NEXT_CUTOFF_TAG}" >/dev/null 2>&1; then
  echo "Next cutoff tag ${NEXT_CUTOFF_TAG} does not exist." >&2
  exit 1
fi

if ! git merge-base --is-ancestor "${CANDIDATE_SHA}" "origin/main"; then
  echo "Commit '${CANDIDATE_SHA}' is not reachable from origin/main." >&2
  exit 1
fi

NEXT_CUTOFF_SHA="$(git rev-list -n 1 "${NEXT_CUTOFF_TAG}")"
if ! git merge-base --is-ancestor "${CANDIDATE_SHA}" "${NEXT_CUTOFF_SHA}"; then
  echo "Commit '${CANDIDATE_SHA}' is newer than the next cutoff ${NEXT_CUTOFF_TAG}." >&2
  exit 1
fi

echo "next_cutoff_sha=${NEXT_CUTOFF_SHA}" >> "$GITHUB_OUTPUT"
