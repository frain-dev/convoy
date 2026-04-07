#!/usr/bin/env bash
#
# Write the release manifest JSON used by the tag-stable-release trigger.
#
# Usage:
#   ./scripts/ci/write-release-manifest.sh <version> <release_line> \
#       <target_sha> <cutoff_tag> <next_cutoff_tag> <previous_stable_tag>
#
# Output:
#   Writes .github/release-manifest.json

set -euo pipefail

if [ "$#" -ne 6 ]; then
  echo "Usage: $0 <version> <release_line> <target_sha> <cutoff_tag> <next_cutoff_tag> <previous_stable_tag>" >&2
  exit 1
fi

VERSION="$1"
RELEASE_LINE="$2"
TARGET_SHA="$3"
CUTOFF_TAG="$4"
NEXT_CUTOFF_TAG="$5"
PREVIOUS_STABLE_TAG="$6"

cat <<EOF > .github/release-manifest.json
{
  "version": "${VERSION}",
  "release_line": "${RELEASE_LINE}",
  "target_sha": "${TARGET_SHA}",
  "cutoff_tag": "${CUTOFF_TAG}",
  "next_cutoff_tag": "${NEXT_CUTOFF_TAG}",
  "previous_stable_tag": "${PREVIOUS_STABLE_TAG}",
  "generated_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF
