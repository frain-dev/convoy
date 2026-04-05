#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/release-workflow.sh should-run --event-name <event-name> [--today YYYY-MM-DD]
  ./scripts/release-workflow.sh monthly-plan [--today YYYY-MM-DD]
  ./scripts/release-workflow.sh ensure-cutoff-tag --tag <tag> [--sha <sha>]
  ./scripts/release-workflow.sh resolve-target --release-cutoff-tag <tag> --version <version>
  ./scripts/release-workflow.sh render-changelog --version <version> --target-sha <sha> [--previous-stable-tag <tag>] [--config <path>] [--path <path>]
  ./scripts/release-workflow.sh write-manifest --version <version> --release-line <line> --target-sha <sha> --cutoff-tag <tag> --next-cutoff-tag <tag> [--previous-stable-tag <tag>] [--path <path>]
  ./scripts/release-workflow.sh read-manifest [--path <path>]
  ./scripts/release-workflow.sh plan-patch --release-line <YY.M>
  ./scripts/release-workflow.sh validate-patch --commit-sha <sha> --base-tag <tag> --next-cutoff-tag <tag> [--main-ref <ref>]
EOF
}

require_value() {
  local flag="$1"
  local value="${2:-}"
  if [ -z "$value" ]; then
    echo "missing required value for ${flag}" >&2
    exit 1
  fi
}

utc_today() {
  python3 - <<'PY'
from datetime import datetime, timezone
print(datetime.now(timezone.utc).strftime("%Y-%m-%d"))
PY
}

date_field() {
  local iso_date="$1"
  local field="$2"
  python3 - "$iso_date" "$field" <<'PY'
from datetime import datetime
import sys

date_value = datetime.strptime(sys.argv[1], "%Y-%m-%d").date()
field = sys.argv[2]

if field == "is_last_day":
    from datetime import timedelta
    print("true" if (date_value + timedelta(days=1)).day == 1 else "false")
elif field == "current_year":
    print(date_value.strftime("%Y"))
elif field == "current_month_padded":
    print(date_value.strftime("%m"))
elif field == "previous_year_full":
    if date_value.month == 1:
        year = date_value.year - 1
        month = 12
    else:
        year = date_value.year
        month = date_value.month - 1
    print(f"{year:04d}")
elif field == "previous_year":
    if date_value.month == 1:
        year = date_value.year - 1
        month = 12
    else:
        year = date_value.year
        month = date_value.month - 1
    print(f"{year % 100:02d}")
elif field == "previous_month":
    month = 12 if date_value.month == 1 else date_value.month - 1
    print(month)
elif field == "previous_month_padded":
    month = 12 if date_value.month == 1 else date_value.month - 1
    print(f"{month:02d}")
else:
    raise SystemExit(f"unknown date field: {field}")
PY
}

iso_timestamp() {
  python3 - <<'PY'
from datetime import datetime, timezone
print(datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"))
PY
}

cmd_should_run() {
  local event_name=""
  local today=""

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --event-name)
        event_name="${2:-}"
        shift 2
        ;;
      --today)
        today="${2:-}"
        shift 2
        ;;
      *)
        echo "unknown argument for should-run: $1" >&2
        exit 1
        ;;
    esac
  done

  require_value "--event-name" "$event_name"
  if [ -z "$today" ]; then
    today="$(utc_today)"
  fi

  if [ "$event_name" != "schedule" ]; then
    echo "should_run=true"
    return 0
  fi

  if [ "$(date_field "$today" "is_last_day")" = "true" ]; then
    echo "should_run=true"
  else
    echo "should_run=false"
  fi
}

cmd_monthly_plan() {
  local today=""

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --today)
        today="${2:-}"
        shift 2
        ;;
      *)
        echo "unknown argument for monthly-plan: $1" >&2
        exit 1
        ;;
    esac
  done

  if [ -z "$today" ]; then
    today="$(utc_today)"
  fi

  local current_year current_month_padded release_year_full release_year release_month release_month_padded
  current_year="$(date_field "$today" "current_year")"
  current_month_padded="$(date_field "$today" "current_month_padded")"
  release_year_full="$(date_field "$today" "previous_year_full")"
  release_year="$(date_field "$today" "previous_year")"
  release_month="$(date_field "$today" "previous_month")"
  release_month_padded="$(date_field "$today" "previous_month_padded")"

  local version
  version="$("${SCRIPT_DIR}/calver.sh" "$release_year" "$release_month")"

  echo "version=${version}"
  echo "release_line=${release_year}.${release_month}"
  echo "current_cutoff_tag=release-cutoff-${current_year}-${current_month_padded}"
  echo "release_cutoff_tag=release-cutoff-${release_year_full}-${release_month_padded}"
}

cmd_ensure_cutoff_tag() {
  local tag=""
  local target_sha=""

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --tag)
        tag="${2:-}"
        shift 2
        ;;
      --sha)
        target_sha="${2:-}"
        shift 2
        ;;
      *)
        echo "unknown argument for ensure-cutoff-tag: $1" >&2
        exit 1
        ;;
    esac
  done

  require_value "--tag" "$tag"
  if [ -z "$target_sha" ]; then
    target_sha="$(git rev-parse HEAD)"
  fi

  if git rev-parse --verify "refs/tags/${tag}" >/dev/null 2>&1; then
    echo "created=false"
    echo "tag=${tag}"
    echo "target_sha=$(git rev-list -n 1 "${tag}")"
    return 0
  fi

  git tag "${tag}" "${target_sha}"
  git push origin "refs/tags/${tag}"

  echo "created=true"
  echo "tag=${tag}"
  echo "target_sha=${target_sha}"
}

cmd_resolve_target() {
  local release_cutoff_tag=""
  local version=""

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --release-cutoff-tag)
        release_cutoff_tag="${2:-}"
        shift 2
        ;;
      --version)
        version="${2:-}"
        shift 2
        ;;
      *)
        echo "unknown argument for resolve-target: $1" >&2
        exit 1
        ;;
    esac
  done

  require_value "--release-cutoff-tag" "$release_cutoff_tag"
  require_value "--version" "$version"

  if ! git rev-parse --verify "refs/tags/${release_cutoff_tag}" >/dev/null 2>&1; then
    echo "should_run=false"
    echo "reason=missing_cutoff_tag"
    return 0
  fi

  if git rev-parse --verify "refs/tags/v${version}" >/dev/null 2>&1; then
    echo "should_run=false"
    echo "reason=release_tag_exists"
    return 0
  fi

  local target_sha previous_stable_tag
  target_sha="$(git rev-list -n 1 "${release_cutoff_tag}")"
  previous_stable_tag="$(git tag --merged "${target_sha}" --list 'v*' --sort=-version:refname | sed -n '1p')"

  echo "should_run=true"
  echo "reason=release_ready"
  echo "target_sha=${target_sha}"
  echo "previous_stable_tag=${previous_stable_tag}"
}

cmd_render_changelog() {
  local version=""
  local target_sha=""
  local previous_stable_tag=""
  local config=".git-cliff.toml"
  local changelog_path="CHANGELOG.md"

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --version)
        version="${2:-}"
        shift 2
        ;;
      --target-sha)
        target_sha="${2:-}"
        shift 2
        ;;
      --previous-stable-tag)
        previous_stable_tag="${2:-}"
        shift 2
        ;;
      --config)
        config="${2:-}"
        shift 2
        ;;
      --path)
        changelog_path="${2:-}"
        shift 2
        ;;
      *)
        echo "unknown argument for render-changelog: $1" >&2
        exit 1
        ;;
    esac
  done

  require_value "--version" "$version"
  require_value "--target-sha" "$target_sha"

  if [ -n "$previous_stable_tag" ]; then
    git-cliff "${previous_stable_tag}..${target_sha}" --config "${config}" --tag "v${version}" --prepend "${changelog_path}"
  else
    git-cliff "${target_sha}" --config "${config}" --tag "v${version}" --prepend "${changelog_path}"
  fi

  echo "path=${changelog_path}"
}

cmd_write_manifest() {
  local version=""
  local release_line=""
  local target_sha=""
  local cutoff_tag=""
  local next_cutoff_tag=""
  local previous_stable_tag=""
  local path=".github/release-manifest.json"

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --version)
        version="${2:-}"
        shift 2
        ;;
      --release-line)
        release_line="${2:-}"
        shift 2
        ;;
      --target-sha)
        target_sha="${2:-}"
        shift 2
        ;;
      --cutoff-tag)
        cutoff_tag="${2:-}"
        shift 2
        ;;
      --next-cutoff-tag)
        next_cutoff_tag="${2:-}"
        shift 2
        ;;
      --previous-stable-tag)
        previous_stable_tag="${2:-}"
        shift 2
        ;;
      --path)
        path="${2:-}"
        shift 2
        ;;
      *)
        echo "unknown argument for write-manifest: $1" >&2
        exit 1
        ;;
    esac
  done

  require_value "--version" "$version"
  require_value "--release-line" "$release_line"
  require_value "--target-sha" "$target_sha"
  require_value "--cutoff-tag" "$cutoff_tag"
  require_value "--next-cutoff-tag" "$next_cutoff_tag"

  python3 - "$path" "$version" "$release_line" "$target_sha" "$cutoff_tag" "$next_cutoff_tag" "$previous_stable_tag" "$(iso_timestamp)" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
payload = {
    "version": sys.argv[2],
    "release_line": sys.argv[3],
    "target_sha": sys.argv[4],
    "cutoff_tag": sys.argv[5],
    "next_cutoff_tag": sys.argv[6],
    "previous_stable_tag": sys.argv[7],
    "generated_at": sys.argv[8],
}
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
PY

  echo "path=${path}"
}

cmd_read_manifest() {
  local path=".github/release-manifest.json"

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --path)
        path="${2:-}"
        shift 2
        ;;
      *)
        echo "unknown argument for read-manifest: $1" >&2
        exit 1
        ;;
    esac
  done

  python3 - "$path" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    payload = json.load(handle)

required = ["version", "target_sha"]
missing = [key for key in required if not payload.get(key)]
if missing:
    raise SystemExit(f"manifest is missing required keys: {', '.join(missing)}")

for key in [
    "version",
    "release_line",
    "target_sha",
    "cutoff_tag",
    "next_cutoff_tag",
    "previous_stable_tag",
    "generated_at",
]:
    print(f"{key}={payload.get(key, '')}")
PY
}

cmd_plan_patch() {
  local release_line=""

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --release-line)
        release_line="${2:-}"
        shift 2
        ;;
      *)
        echo "unknown argument for plan-patch: $1" >&2
        exit 1
        ;;
    esac
  done

  require_value "--release-line" "$release_line"

  if [[ ! "$release_line" =~ ^([0-9]{2})\.([0-9]{1,2})$ ]]; then
    echo "release_line must match YY.M, for example 26.3" >&2
    exit 1
  fi

  local release_year="${BASH_REMATCH[1]}"
  local release_month="${BASH_REMATCH[2]}"
  release_month=$((10#${release_month}))

  local next_year_full=$((2000 + 10#${release_year}))
  local next_month=$((release_month + 1))
  if [ "${next_month}" -gt 12 ]; then
    next_month=1
    next_year_full=$((next_year_full + 1))
  fi

  local version
  version="$("${SCRIPT_DIR}/calver.sh" "${release_year}" "${release_month}")"

  echo "release_year=${release_year}"
  echo "release_month=${release_month}"
  echo "next_cutoff_tag=$(printf 'release-cutoff-%04d-%02d' "${next_year_full}" "${next_month}")"
  echo "base_tag=v${release_year}.${release_month}.0"
  echo "version=${version}"
}

cmd_validate_patch() {
  local commit_sha=""
  local base_tag=""
  local next_cutoff_tag=""
  local main_ref="origin/main"

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --commit-sha)
        commit_sha="${2:-}"
        shift 2
        ;;
      --base-tag)
        base_tag="${2:-}"
        shift 2
        ;;
      --next-cutoff-tag)
        next_cutoff_tag="${2:-}"
        shift 2
        ;;
      --main-ref)
        main_ref="${2:-}"
        shift 2
        ;;
      *)
        echo "unknown argument for validate-patch: $1" >&2
        exit 1
        ;;
    esac
  done

  require_value "--commit-sha" "$commit_sha"
  require_value "--base-tag" "$base_tag"
  require_value "--next-cutoff-tag" "$next_cutoff_tag"

  if ! git rev-parse --verify "${commit_sha}^{commit}" >/dev/null 2>&1; then
    echo "Commit '${commit_sha}' does not exist." >&2
    exit 1
  fi

  if ! git rev-parse --verify "refs/tags/${base_tag}" >/dev/null 2>&1; then
    echo "Base release tag ${base_tag} does not exist yet." >&2
    exit 1
  fi

  if ! git rev-parse --verify "refs/tags/${next_cutoff_tag}" >/dev/null 2>&1; then
    echo "Next cutoff tag ${next_cutoff_tag} does not exist." >&2
    exit 1
  fi

  if ! git merge-base --is-ancestor "${commit_sha}" "${main_ref}"; then
    echo "Commit '${commit_sha}' is not reachable from ${main_ref}." >&2
    exit 1
  fi

  local next_cutoff_sha
  next_cutoff_sha="$(git rev-list -n 1 "${next_cutoff_tag}")"

  if ! git merge-base --is-ancestor "${commit_sha}" "${next_cutoff_sha}"; then
    echo "Commit '${commit_sha}' is newer than the next cutoff ${next_cutoff_tag}." >&2
    exit 1
  fi

  echo "next_cutoff_sha=${next_cutoff_sha}"
}

main() {
  local command="${1:-}"
  if [ -z "$command" ]; then
    usage >&2
    exit 1
  fi
  shift

  case "$command" in
    should-run)
      cmd_should_run "$@"
      ;;
    monthly-plan)
      cmd_monthly_plan "$@"
      ;;
    ensure-cutoff-tag)
      cmd_ensure_cutoff_tag "$@"
      ;;
    resolve-target)
      cmd_resolve_target "$@"
      ;;
    render-changelog)
      cmd_render_changelog "$@"
      ;;
    write-manifest)
      cmd_write_manifest "$@"
      ;;
    read-manifest)
      cmd_read_manifest "$@"
      ;;
    plan-patch)
      cmd_plan_patch "$@"
      ;;
    validate-patch)
      cmd_validate_patch "$@"
      ;;
    *)
      echo "unknown command: ${command}" >&2
      usage >&2
      exit 1
      ;;
  esac
}

main "$@"
