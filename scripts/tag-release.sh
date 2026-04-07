#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./scripts/tag-release.sh <version> <target-sha>
EOF
}

if [ "$#" -ne 2 ]; then
  usage >&2
  exit 1
fi

VERSION="$1"
TARGET_SHA="$2"
TAG="v${VERSION}"

if ! git rev-parse --verify "${TARGET_SHA}^{commit}" >/dev/null 2>&1; then
  echo "Target commit '${TARGET_SHA}' does not exist." >&2
  exit 1
fi

if git rev-parse "refs/tags/${TAG}" >/dev/null 2>&1; then
  echo "Tag ${TAG} already exists; exiting."
  exit 0
fi

# Use environment variables instead of mutating git config in CI.
export GIT_AUTHOR_NAME="${GIT_AUTHOR_NAME:-github-actions[bot]}"
export GIT_AUTHOR_EMAIL="${GIT_AUTHOR_EMAIL:-41898282+github-actions[bot]@users.noreply.github.com}"
export GIT_COMMITTER_NAME="${GIT_COMMITTER_NAME:-${GIT_AUTHOR_NAME}}"
export GIT_COMMITTER_EMAIL="${GIT_COMMITTER_EMAIL:-${GIT_AUTHOR_EMAIL}}"

git tag -a "${TAG}" "${TARGET_SHA}" -m "Release ${TAG}"
git push origin "${TAG}"
