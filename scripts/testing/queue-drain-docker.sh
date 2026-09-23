#!/usr/bin/env bash
# Durable operation storage tests. Does not certify application drain behavior.
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd)
task_dir=$(mktemp -d "${TMPDIR:-/tmp}/queue-drain-qa.XXXXXX")
task_log="$task_dir/qa.log"
project="queue-drain-qa-$$"
export QUEUE_DRAIN_TEST_BINARY="$task_dir/queue-drain.test"
compose=(docker compose --project-name "$project" --file "$repo/testdata/queue-drain/operations.compose.yaml")
cleanup() {
  task_status=$?
  trap - EXIT
  "${compose[@]}" down --volumes --remove-orphans >>"$task_log" 2>&1 || true
  printf '\nExit status: %s\nLog: %s\n' "$task_status" "$task_log"
  tail -45 "$task_log"
  exit "$task_status"
}
trap cleanup EXIT
printf 'Docker operation QA log: %s\n' "$task_log"
cd "$repo"
arch=$(docker info --format '{{.Architecture}}')
case "$arch" in aarch64|arm64) arch=arm64;; x86_64|amd64) arch=amd64;; *) echo "Unsupported Docker architecture"; exit 1;; esac
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go test -c -tags integration ./queue/drain -o "$QUEUE_DRAIN_TEST_BINARY" >>"$task_log" 2>&1
"${compose[@]}" up --detach --wait --wait-timeout 60 postgres redis >>"$task_log" 2>&1
"${compose[@]}" run --rm --no-deps qa -test.v -test.run '^TestDocker(OperationLifecycle|ConcurrentBeginAndAdmission|SourceFenceKeepsActiveAdmissionOpen|AdmissionGate|RuntimeCoordination|ExecutionExclusion|WorkerAuthorityLoss|CompletedOperationConfigurationChange)$' >>"$task_log" 2>&1
"${compose[@]}" run --rm --no-deps qa -test.v -test.run '^TestDockerConsumerSuspendResume$' >>"$task_log" 2>&1
"${compose[@]}" run --rm --no-deps qa -test.v -test.run '^TestDockerPostgresStoreFence$' >>"$task_log" 2>&1
"${compose[@]}" run --rm --no-deps qa -test.v -test.run '^TestDockerRedisStoreFence$' >>"$task_log" 2>&1
"${compose[@]}" restart redis >>"$task_log" 2>&1
"${compose[@]}" up --detach --wait --wait-timeout 60 redis >>"$task_log" 2>&1
"${compose[@]}" run --rm --no-deps qa -test.v -test.run '^TestDockerRedisFenceAfterRestart$' >>"$task_log" 2>&1
"${compose[@]}" stop postgres >>"$task_log" 2>&1
"${compose[@]}" run --rm --no-deps qa -test.v -test.run '^TestDockerOperationDatabaseUnavailable$' >>"$task_log" 2>&1
"${compose[@]}" up --detach --wait --wait-timeout 60 postgres >>"$task_log" 2>&1
"${compose[@]}" run --rm --no-deps qa -test.v -test.run '^TestDockerOperationDatabaseRecovered$' >>"$task_log" 2>&1
