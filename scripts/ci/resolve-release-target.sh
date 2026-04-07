#!/usr/bin/env bash
#
# Resolve the target SHA for a release from its cutoff tag.
# Exits early (should_run=false) when the cutoff tag is missing
# or the release tag already exists.
#
# Usage:
#   ./scripts/ci/resolve-release-target.sh <release_cutoff_tag> <version>
#
# Outputs (written to $GITHUB_OUTPUT):
#   target_sha, previous_stable_tag, should_run

set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <release_cutoff_tag> <version>" >&2
  exit 1
fi

RELEASE_CUTOFF_TAG="$1"
VERSION="$2"

if ! git rev-parse --verify "refs/tags/${RELEASE_CUTOFF_TAG}" >/dev/null 2>&1; then
  echo "No prior cutoff tag ${RELEASE_CUTOFF_TAG}; bootstrap month only."
  echo "should_run=false" >> "$GITHUB_OUTPUT"
  exit 0
fi

if git rev-parse --verify "refs/tags/v${VERSION}" >/dev/null 2>&1; then
  echo "Release tag v${VERSION} already exists; skipping."
  echo "should_run=false" >> "$GITHUB_OUTPUT"
  exit 0
fi

TARGET_SHA="$(git rev-list -n 1 "${RELEASE_CUTOFF_TAG}")"
PREVIOUS_STABLE_TAG="$(git tag --merged "${TARGET_SHA}" --list 'v*' --sort=-version:refname | sed -n '1p')"

echo "target_sha=${TARGET_SHA}" >> "$GITHUB_OUTPUT"
echo "previous_stable_tag=${PREVIOUS_STABLE_TAG}" >> "$GITHUB_OUTPUT"
echo "should_run=true" >> "$GITHUB_OUTPUT"
