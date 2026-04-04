#!/usr/bin/env bash
#
# Stress Test: 1M Events + 5-Minute Backup to 3 Backends
#
# Usage:
#   1. Start infrastructure:  ./scripts/stress-test-backup.sh infra-up
#   2. Set backend env vars (see below), start Convoy, then:
#   3. Generate events:       ./scripts/stress-test-backup.sh generate
#   4. Wait 15-20 minutes for backup cycles
#   5. Verify results:        ./scripts/stress-test-backup.sh verify
#   6. Tear down:             ./scripts/stress-test-backup.sh infra-down
#
# Required env vars for event generation:
#   CONVOY_BASE_URL    - Convoy API URL (default: http://localhost:5005)
#   CONVOY_PROJECT_ID  - Target project ID
#   CONVOY_API_KEY     - API key with project admin access
#   CONVOY_ENDPOINT_ID - Target endpoint ID
#
# Backend env vars (set ONE group before starting Convoy):
#
#   --- MinIO (S3) ---
#   CONVOY_STORAGE_POLICY_TYPE=s3
#   CONVOY_STORAGE_AWS_BUCKET=convoy-stress-test
#   CONVOY_STORAGE_AWS_ACCESS_KEY=minioadmin
#   CONVOY_STORAGE_AWS_SECRET_KEY=minioadmin
#   CONVOY_STORAGE_AWS_REGION=us-east-1
#   CONVOY_STORAGE_AWS_ENDPOINT=http://localhost:9000
#
#   --- On-Prem ---
#   CONVOY_STORAGE_POLICY_TYPE=on_prem
#   CONVOY_STORAGE_PREM_PATH=/tmp/convoy-stress-test
#
#   --- Azure Blob ---
#   CONVOY_STORAGE_POLICY_TYPE=azure_blob
#   CONVOY_STORAGE_AZURE_ACCOUNT_NAME=<AZURE_ACCOUNT_NAME>
#   CONVOY_STORAGE_AZURE_ACCOUNT_KEY=<AZURE_ACCOUNT_KEY>
#   CONVOY_STORAGE_AZURE_CONTAINER_NAME=<AZURE_CONTAINER_NAME>
#   CONVOY_STORAGE_AZURE_ENDPOINT=<AZURE_ENDPOINT>
#
# Always set these for backup:
#   CONVOY_RETENTION_POLICY_ENABLED=true
#   CONVOY_RETENTION_POLICY=720h
#   CONVOY_BACKUP_INTERVAL=5m

set -euo pipefail

CONVOY_BASE_URL="${CONVOY_BASE_URL:-http://localhost:5005}"
BENCH_DIR="${BENCH_DIR:-/Users/rtukpe/Documents/dev/frain/convoy-bench}"
MINIO_CONTAINER="convoy-stress-minio"
AZURITE_CONTAINER="convoy-stress-azurite"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[stress-test]${NC} $*"; }
warn() { echo -e "${YELLOW}[stress-test]${NC} $*"; }
err()  { echo -e "${RED}[stress-test]${NC} $*" >&2; }

cmd_infra_up() {
    log "Starting MinIO..."
    docker run -d --name "$MINIO_CONTAINER" \
        -p 9000:9000 -p 9001:9001 \
        -e MINIO_ROOT_USER=minioadmin \
        -e MINIO_ROOT_PASSWORD=minioadmin \
        minio/minio:RELEASE.2024-01-16T16-07-38Z server /data --console-address ":9001" \
        2>/dev/null || warn "MinIO container already exists"

    log "Waiting for MinIO to be ready..."
    sleep 3

    log "Creating MinIO bucket..."
    docker run --rm --network host \
        --entrypoint /bin/sh minio/mc -c \
        "mc alias set local http://localhost:9000 minioadmin minioadmin && mc mb --ignore-existing local/convoy-stress-test" \
        2>/dev/null || true

    log "Starting Azurite..."
    docker run -d --name "$AZURITE_CONTAINER" \
        -p 10000:10000 \
        mcr.microsoft.com/azure-storage/azurite:3.31.0 \
        azurite-blob --blobHost 0.0.0.0 --blobPort 10000 --skipApiVersionCheck \
        2>/dev/null || warn "Azurite container already exists"

    log "Creating on-prem directory..."
    mkdir -p /tmp/convoy-stress-test

    log "Infrastructure ready."
    log "  MinIO:    http://localhost:9000 (console: http://localhost:9001)"
    log "  Azurite:  http://localhost:10000"
    log "  On-Prem:  /tmp/convoy-stress-test"
}

cmd_infra_down() {
    log "Stopping containers..."
    docker rm -f "$MINIO_CONTAINER" 2>/dev/null || true
    docker rm -f "$AZURITE_CONTAINER" 2>/dev/null || true
    rm -rf /tmp/convoy-stress-test
    log "Infrastructure torn down."
}

cmd_generate() {
    if [ -z "${CONVOY_PROJECT_ID:-}" ] || [ -z "${CONVOY_API_KEY:-}" ] || [ -z "${CONVOY_ENDPOINT_ID:-}" ]; then
        err "Missing required env vars: CONVOY_PROJECT_ID, CONVOY_API_KEY, CONVOY_ENDPOINT_ID"
        exit 1
    fi

    if [ ! -f "$BENCH_DIR/bench.sh" ]; then
        err "convoy-bench not found at $BENCH_DIR"
        exit 1
    fi

    log "Generating 1M events via convoy-bench..."
    log "  Target:   $CONVOY_BASE_URL"
    log "  Project:  $CONVOY_PROJECT_ID"
    log "  Endpoint: $CONVOY_ENDPOINT_ID"
    log "  Rate:     5000 req/s for 200s = ~1M events"

    cd "$BENCH_DIR"
    ./bench.sh -p http -t outgoing \
        -u "$CONVOY_BASE_URL" \
        -v 100 -r 5000 -d 200s \
        --endpoint-id "$CONVOY_ENDPOINT_ID" \
        --project-id "$CONVOY_PROJECT_ID" \
        --api-key "$CONVOY_API_KEY"

    log "Event generation complete."
}

cmd_verify() {
    local backend="${1:-all}"

    log "Verifying backup results..."

    if [ "$backend" = "all" ] || [ "$backend" = "s3" ] || [ "$backend" = "minio" ]; then
        log ""
        log "=== MinIO (S3) ==="
        if docker ps --format '{{.Names}}' | grep -q "$MINIO_CONTAINER"; then
            docker run --rm --network host \
                --entrypoint /bin/sh minio/mc -c \
                "mc alias set local http://localhost:9000 minioadmin minioadmin && mc ls --recursive local/convoy-stress-test/" \
                2>/dev/null || warn "Failed to list MinIO objects"
        else
            warn "MinIO container not running"
        fi
    fi

    if [ "$backend" = "all" ] || [ "$backend" = "onprem" ]; then
        log ""
        log "=== On-Prem ==="
        if [ -d /tmp/convoy-stress-test ]; then
            local count
            count=$(find /tmp/convoy-stress-test -name "*.jsonl.gz" 2>/dev/null | wc -l | tr -d ' ')
            log "Export files found: $count"
            find /tmp/convoy-stress-test -name "*.jsonl.gz" -exec ls -lh {} \; 2>/dev/null | head -10
            if [ "$count" -gt 0 ]; then
                local first_file
                first_file=$(find /tmp/convoy-stress-test -name "*.jsonl.gz" | head -1)
                local lines
                lines=$(gunzip -c "$first_file" | wc -l | tr -d ' ')
                log "Records in first file: $lines"
            fi
        else
            warn "On-prem directory not found"
        fi
    fi

    if [ "$backend" = "all" ] || [ "$backend" = "azure" ]; then
        log ""
        log "=== Azure Blob ==="
        if [ -n "${CONVOY_STORAGE_AZURE_ACCOUNT_NAME:-}" ]; then
            warn "Azure verification requires 'az' CLI. Run manually:"
            log "  az storage blob list --container-name <CONTAINER> --account-name <ACCOUNT> --output table"
        else
            warn "Azure env vars not set. Skipping."
        fi
    fi

    log ""
    log "=== Backup Jobs (DB) ==="
    warn "Check backup_jobs table manually:"
    log "  SELECT id, project_id, status, record_counts, created_at FROM convoy.backup_jobs ORDER BY created_at DESC LIMIT 20;"
}

cmd_help() {
    echo "Usage: $0 <command> [args]"
    echo ""
    echo "Commands:"
    echo "  infra-up      Start MinIO + Azurite + create on-prem dir"
    echo "  infra-down    Stop containers + remove on-prem dir"
    echo "  generate      Generate 1M events via convoy-bench"
    echo "  verify [backend]  Verify backup results (s3|onprem|azure|all)"
    echo "  help          Show this help"
    echo ""
    echo "Workflow:"
    echo "  1. $0 infra-up"
    echo "  2. Set backend env vars + CONVOY_BACKUP_INTERVAL=5m"
    echo "  3. Start Convoy (server + worker)"
    echo "  4. $0 generate"
    echo "  5. Wait 15-20 minutes"
    echo "  6. $0 verify"
    echo "  7. $0 infra-down"
}

case "${1:-help}" in
    infra-up)    cmd_infra_up ;;
    infra-down)  cmd_infra_down ;;
    generate)    cmd_generate ;;
    verify)      cmd_verify "${2:-all}" ;;
    help|*)      cmd_help ;;
esac
