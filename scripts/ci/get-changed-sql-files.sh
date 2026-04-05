#!/usr/bin/env bash
#
# Detect SQL migration files changed in a pull request.
#
# Usage:
#   ./scripts/ci/get-changed-sql-files.sh <base_ref>
#
# Outputs (written to $GITHUB_OUTPUT):
#   has_changes  ("true" or "false")
#   files        (space-separated list of changed SQL files)

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <base_ref>" >&2
  exit 1
fi

BASE_REF="$1"
CHANGED_FILES=$(git diff --name-only --diff-filter=ACMRT "origin/${BASE_REF}...HEAD" -- 'sql/*.sql' || echo "")

if [ -z "$CHANGED_FILES" ]; then
  echo "No SQL files changed"
  echo "has_changes=false" >> "$GITHUB_OUTPUT"
else
  echo "Changed SQL files:"
  echo "$CHANGED_FILES"
  echo "has_changes=true" >> "$GITHUB_OUTPUT"
  FILES_LIST=$(echo "$CHANGED_FILES" | tr '\n' ' ')
  echo "files=$FILES_LIST" >> "$GITHUB_OUTPUT"
fi
