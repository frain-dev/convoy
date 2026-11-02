#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
compose_file=${1:-$root/configs/local/docker-compose.yml}
task_dir=$(mktemp -d)
project="convoy-readiness-$$"
compose=(docker compose -p "$project" -f "$task_dir/compose.json")
up_pid=""
cleanup() {
    status=$?
    trap - EXIT
    if [ "$status" -ne 0 ]; then
        cat "$task_dir/up.log" 2>/dev/null || true
        "${compose[@]}" logs --tail 60 migrate pgbouncer 2>/dev/null || true
    fi
    if [ -n "$up_pid" ]; then kill "$up_pid" 2>/dev/null || true; fi
    "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
    rm -rf "$task_dir"
    exit "$status"
}
trap cleanup EXIT

cp -R "$root/configs/local" "$task_dir/fixtures"
docker compose --project-directory "$task_dir/fixtures" -f "$compose_file" config --format json > "$task_dir/base.json"
python3 - "$task_dir/base.json" "$task_dir/compose.json" <<'PY'
import json
import sys

with open(sys.argv[1]) as source:
    config = json.load(source)
for service in config['services'].values():
    service.pop('ports', None)
# Compose's normalized output contains explicit project-scoped names. Let the
# unique test project generate its own resources instead of reusing those names.
for resource_type in ('volumes', 'networks'):
    for resource in config.get(resource_type, {}).values():
        resource.pop('name', None)
# Make the startup race deterministic, without changing the actual readiness probe.
config['services']['pgbouncer']['entrypoint'] = [
    '/bin/bash', '-ec',
    'sleep 15; exec /opt/bitnami/scripts/pgbouncer/entrypoint.sh /opt/bitnami/scripts/pgbouncer/run.sh',
]
with open(sys.argv[2], 'w') as target:
    json.dump(config, target)
PY

"${compose[@]}" up -d migrate > "$task_dir/up.log" 2>&1 &
up_pid=$!
deadline=$((SECONDS + 90))
pool=""
until [ -n "$pool" ] && [ "$(docker inspect -f '{{.State.Running}}' "$pool")" = true ]; do
    if [ "$SECONDS" -ge "$deadline" ]; then echo 'FAIL: PgBouncer did not start'; exit 1; fi
    pool=$("${compose[@]}" ps -aq pgbouncer)
    sleep 1
done

migrate=$("${compose[@]}" ps -aq migrate)
if [ -n "$migrate" ] && [ "$(docker inspect -f '{{.State.Running}}' "$migrate")" = true ]; then
    echo 'FAIL: migrations started while PgBouncer was still starting'
    exit 1
fi

wait "$up_pid"
up_pid=""
migrate=$("${compose[@]}" ps -aq migrate)
until [ "$(docker inspect -f '{{.State.Status}}' "$migrate")" = exited ]; do
    if [ "$SECONDS" -ge "$deadline" ]; then echo 'FAIL: migrations did not finish'; exit 1; fi
    sleep 1
done
test "$(docker inspect -f '{{.State.ExitCode}}' "$migrate")" = 0
test "$(docker inspect -f '{{.State.Health.Status}}' "$pool")" = healthy
echo 'PASS: delayed PgBouncer becomes query-ready before migrations complete successfully'
