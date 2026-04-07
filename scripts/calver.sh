#!/usr/bin/env bash
#
# Calculate CalVer version in the format: YY.M.PATCH
# Usage:
#   ./scripts/calver.sh
#   ./scripts/calver.sh <yy> <month>
#
# Outputs the version to stdout (e.g., "26.2.0")

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./scripts/calver.sh
  ./scripts/calver.sh <yy> <month>
EOF
}

if [ "$#" -eq 0 ]; then
  YEAR=$(date -u +%y)
  MONTH=$(date -u +%-m)
elif [ "$#" -eq 2 ]; then
  YEAR="$1"
  MONTH="$2"
else
  usage >&2
  exit 1
fi

if ! [[ "$YEAR" =~ ^[0-9]{2}$ ]]; then
  echo "year must be a two-digit value: got '$YEAR'" >&2
  exit 1
fi

if ! [[ "$MONTH" =~ ^[0-9]{1,2}$ ]] || [ "$MONTH" -lt 1 ] || [ "$MONTH" -gt 12 ]; then
  echo "month must be between 1 and 12: got '$MONTH'" >&2
  exit 1
fi

MONTH=$((10#$MONTH))

LATEST_PATCH="$(
  git tag --list "v${YEAR}.${MONTH}.*" |
    sed -nE "s/^v${YEAR}\.${MONTH}\.([0-9]+)$/\1/p" |
    sort -n |
    tail -1
)"

if [ -z "$LATEST_PATCH" ]; then
  PATCH=0
else
  PATCH=$((LATEST_PATCH + 1))
fi

echo "${YEAR}.${MONTH}.${PATCH}"
