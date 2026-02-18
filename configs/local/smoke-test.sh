#!/bin/sh
set -e

COMPOSE_FILE="configs/local/docker-compose.yml"
MAX_WAIT=120

cleanup() {
    echo "Tearing down..."
    docker compose -f "$COMPOSE_FILE" down -v --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

echo "Starting services..."
docker compose -f "$COMPOSE_FILE" up -d

echo "Waiting for Caddy to become reachable (up to ${MAX_WAIT}s)..."
elapsed=0
until curl -sf http://localhost/healthz > /dev/null 2>&1; do
    if [ "$elapsed" -ge "$MAX_WAIT" ]; then
        echo "FAIL: Caddy not reachable after ${MAX_WAIT}s"
        docker compose -f "$COMPOSE_FILE" ps
        docker compose -f "$COMPOSE_FILE" logs
        exit 1
    fi
    sleep 2
    elapsed=$((elapsed + 2))
done
echo "Caddy reachable after ${elapsed}s"

fail=0

assert_status() {
    label="$1"
    url="$2"
    expected="$3"
    actual=$(curl -s -o /dev/null -w "%{http_code}" "$url")
    if [ "$actual" = "$expected" ]; then
        echo "PASS: $label -> $actual"
    else
        echo "FAIL: $label -> expected $expected, got $actual"
        fail=1
    fi
}

echo ""
echo "=== Route assertions ==="
assert_status "GET  /healthz (control plane)"          "http://localhost/healthz"                200
assert_status "GET  / (dashboard UI)"                   "http://localhost/"                       200
assert_status "GET  /api/v1/projects (control plane)"   "http://localhost/api/v1/projects"        401
assert_status "GET  /ingest/test (data plane)"          "http://localhost/ingest/test"            404
assert_status "GET  /portal-api/events (data plane)"    "http://localhost/portal-api/events"      401
assert_status "GET  /portal-api/eventdeliveries (dp)"   "http://localhost/portal-api/eventdeliveries" 401
assert_status "GET  /portal-api/endpoints (ctrl plane)" "http://localhost/portal-api/endpoints"   401

echo ""
if [ "$fail" -ne 0 ]; then
    echo "SMOKE TEST FAILED"
    exit 1
fi
echo "ALL SMOKE TESTS PASSED"
