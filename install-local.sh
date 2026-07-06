#!/usr/bin/env bash

set -euo pipefail

REPO_URL="${CONVOY_REPO_URL:-https://github.com/frain-dev/convoy.git}"
INSTALL_DIR="${CONVOY_INSTALL_DIR:-$HOME/convoy}"
MAX_WAIT_SECONDS="${CONVOY_MAX_WAIT_SECONDS:-180}"

log() {
  printf "\n==> %s\n" "$1"
}

warn() {
  printf "\n[WARN] %s\n" "$1"
}

die() {
  printf "\n[ERROR] %s\n" "$1"
  exit 1
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

is_port_in_use() {
  local port="$1"
  if command_exists lsof; then
    lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
    return $?
  fi

  # Fallback when lsof is unavailable: check host binding via Python.
  if command_exists python3; then
    python3 - "$port" <<'PY'
import socket
import sys

port = int(sys.argv[1])
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
try:
    s.bind(("0.0.0.0", port))
except OSError as e:
    # Permission denied on privileged ports (e.g. 80) is not proof that
    # another process is listening; treat as unknown.
    if getattr(e, "errno", None) == 13:
        sys.exit(2)  # unknown
    sys.exit(0)  # in use
else:
    sys.exit(1)  # free
finally:
    s.close()
PY
    return $?
  fi

  # Last resort when lsof/python3 are unavailable.
  warn "Could not reliably check port $port (missing lsof/python3); continuing."
  return 1
}

check_prereqs() {
  log "Checking prerequisites"

  if ! command_exists git; then
    die "Git is not installed. Install Git first and run this script again."
  fi

  if ! command_exists curl; then
    die "curl is not installed. Install curl first and run this script again."
  fi

  if ! command_exists docker; then
    cat <<'EOF'
[ERROR] Docker is not installed.

Install Docker Desktop (macOS/Windows):
  https://docs.docker.com/desktop/

Install Docker Engine (Linux):
  https://docs.docker.com/engine/install/
EOF
    exit 1
  fi

  if ! docker compose version >/dev/null 2>&1; then
    die "Docker Compose v2 is not available. Install/enable docker compose and try again."
  fi

  if ! docker info >/dev/null 2>&1; then
    cat <<'EOF'
[ERROR] Docker daemon is not running.

Start Docker Desktop, wait until it is fully running, then run this script again.
EOF
    exit 1
  fi
}

check_ports() {
  # These are the host ports published by configs/local/docker-compose.yml.
  local required_ports=(80 5433 6432)
  local conflicts=()
  local unknown=()
  local p
  local rc

  log "Checking required ports"

  for p in "${required_ports[@]}"; do
    is_port_in_use "$p"
    rc=$?
    if [ "$rc" -eq 0 ]; then
      conflicts+=("$p")
    elif [ "$rc" -eq 2 ]; then
      unknown+=("$p")
    fi
  done

  if [ "${#conflicts[@]}" -gt 0 ]; then
    cat <<EOF
[ERROR] Required ports are already in use: ${conflicts[*]}

Stop conflicting services/containers, then retry.
Helpful checks:
  docker ps --format 'table {{.Names}}\t{{.Ports}}'
  lsof -nP -iTCP -sTCP:LISTEN

If you previously started Convoy local stack:
  docker compose -f "$INSTALL_DIR/configs/local/docker-compose.yml" down
EOF
    exit 1
  fi

  if [ "${#unknown[@]}" -gt 0 ]; then
    warn "Could not reliably check privileged ports: ${unknown[*]} (permission denied without lsof)."
  fi
}

prepare_repo() {
  log "Preparing Convoy repository"

  if [ -d "$INSTALL_DIR/.git" ]; then
    printf "Found existing repo at %s. Pull latest changes? [Y/n]: " "$INSTALL_DIR"
    if ! read -r pull_choice; then
      pull_choice=""
    fi
    pull_choice="${pull_choice:-Y}"
    if [[ "$pull_choice" =~ ^[Yy]$ ]]; then
      git -C "$INSTALL_DIR" pull --ff-only
    fi
  else
    if [ -d "$INSTALL_DIR" ] && [ "$(ls -A "$INSTALL_DIR" 2>/dev/null)" ]; then
      die "Install directory '$INSTALL_DIR' exists and is not a git repo. Use an empty path or set CONVOY_INSTALL_DIR to another directory."
    fi
    git clone "$REPO_URL" "$INSTALL_DIR"
  fi
}

ensure_local_config() {
  local config_path="$INSTALL_DIR/configs/local/convoy.json"

  if [ -f "$config_path" ]; then
    return
  fi

  log "Ensuring local Convoy config exists"

  # Recover deleted tracked file from git if available.
  if [ -d "$INSTALL_DIR/.git" ] && git -C "$INSTALL_DIR" ls-files --error-unmatch "configs/local/convoy.json" >/dev/null 2>&1; then
    git -C "$INSTALL_DIR" checkout -- "configs/local/convoy.json"
  fi

  if [ ! -f "$config_path" ]; then
    die "Missing $config_path. Restore it from the repository or create it before running installer."
  fi
}

start_stack() {
  local compose_dir="$INSTALL_DIR/configs/local"

  [ -d "$compose_dir" ] || die "Missing compose directory: $compose_dir"

  if [ "${CONVOY_SKIP_PULL:-0}" != "1" ]; then
    log "Pulling latest images"
    docker compose -f "$compose_dir/docker-compose.yml" pull
  fi

  log "Starting Convoy stack"
  docker compose -f "$compose_dir/docker-compose.yml" up -d
}

wait_for_health() {
  local elapsed=0
  local health_url="http://localhost/healthz"

  log "Waiting for Convoy health endpoint ($health_url)"

  until curl -fsS "$health_url" >/dev/null 2>&1; do
    if [ "$elapsed" -ge "$MAX_WAIT_SECONDS" ]; then
      die "Timed out waiting for health after ${MAX_WAIT_SECONDS}s. Check logs with: docker compose -f \"$INSTALL_DIR/configs/local/docker-compose.yml\" logs"
    fi

    sleep 3
    elapsed=$((elapsed + 3))
  done

  log "Convoy is healthy"
}

print_next_steps() {
  cat <<EOF

🎉 Convoy is set up.

Useful commands:
  docker compose -f "$INSTALL_DIR/configs/local/docker-compose.yml" ps
  docker compose -f "$INSTALL_DIR/configs/local/docker-compose.yml" logs -f web agent
  docker compose -f "$INSTALL_DIR/configs/local/docker-compose.yml" down

Open:
  http://localhost

EOF
}

main() {
  check_prereqs
  check_ports
  prepare_repo
  ensure_local_config
  start_stack
  wait_for_health
  print_next_steps
}

main "$@"
