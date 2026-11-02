#!/usr/bin/env bash
# Disposable inventory QA only. Never touches the normal QA project or uses
# host Convoy credentials. Source-only drain execution has a separate matrix.
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd)
task_dir=$(mktemp -d "${TMPDIR:-/tmp}/queue-inventory-qa.XXXXXX")
task_log="$task_dir/qa.log"
project="queue-inventory-qa-$$"
export QUEUE_INVENTORY_TEST_BINARY="$task_dir/queue-inventory.test"
compose=(docker compose --project-name "$project" --file "$repo/testdata/queue-inventory/inventory.compose.yaml")
cleanup() {
  task_status=$?
  trap - EXIT
  "${compose[@]}" down --volumes --remove-orphans >>"$task_log" 2>&1 || true
  printf '\nExit status: %s\nLog: %s\n' "$task_status" "$task_log"
  tail -45 "$task_log"
  exit "$task_status"
}
trap cleanup EXIT
printf 'Docker QA log: %s\n' "$task_log"
cd "$repo"
arch=$(docker info --format '{{.Architecture}}')
case "$arch" in aarch64|arm64) arch=arm64;; x86_64|amd64) arch=amd64;; *) echo "Unsupported Docker architecture"; exit 1;; esac
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go test -c -tags integration ./queue/inventory -o "$QUEUE_INVENTORY_TEST_BINARY" >>"$task_log" 2>&1
"${compose[@]}" up --detach --wait --wait-timeout 60 postgres redis >>"$task_log" 2>&1
"${compose[@]}" run --rm --no-deps qa -test.v -test.run '^TestDockerInventory(BothProviderDirections|ReadDeadline)$' >>"$task_log" 2>&1
for provider in redis postgres; do
  "${compose[@]}" stop "$provider" >>"$task_log" 2>&1
  "${compose[@]}" run --rm --no-deps -e "QUEUE_INVENTORY_STOPPED_PROVIDER=$provider" qa -test.v -test.run '^TestDockerInventoryDisconnectedPreviousStore$' >>"$task_log" 2>&1
  "${compose[@]}" up --detach --wait --wait-timeout 60 "$provider" >>"$task_log" 2>&1
  "${compose[@]}" run --rm --no-deps qa -test.v -test.run '^TestDockerInventory(BothProviderDirections|ReadDeadline)$' >>"$task_log" 2>&1
done
