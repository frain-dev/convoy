#!/usr/bin/env bash

set -euo pipefail

REPO_URL="${CONVOY_REPO_URL:-https://github.com/frain-dev/convoy.git}"
INSTALL_DIR="${CONVOY_INSTALL_DIR:-$HOME/convoy}"
MAX_WAIT_SECONDS="${CONVOY_MAX_WAIT_SECONDS:-180}"

# Default requested host ports (override via env when needed).
REQUESTED_HTTP_PORT="${CONVOY_HTTP_PORT:-80}"
REQUESTED_POSTGRES_PORT="${CONVOY_POSTGRES_PORT:-5433}"
REQUESTED_PGBOUNCER_PORT="${CONVOY_PGBOUNCER_PORT:-6432}"

SELECTED_HTTP_PORT=""
SELECTED_POSTGRES_PORT=""
SELECTED_PGBOUNCER_PORT=""
SELECTED_HOST_URL=""
COMPOSE_BASE_FILE=""
COMPOSE_RENDERED_FILE=""

log() {
  printf "\n==> %s\n" "$1"
}

warn() {
  printf "\n[WARN] %s\n" "$1" >&2
}

die() {
  printf "\n[ERROR] %s\n" "$1" >&2
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

find_available_port() {
  local start_port="$1"
  local max_tries="${2:-200}"
  local candidate="$start_port"
  local i=0

  while [ "$i" -lt "$max_tries" ]; do
    if is_port_in_use "$candidate"; then
      : # occupied
    else
      case "$?" in
        1)
          printf "%s" "$candidate"
          return 0
          ;;
        2)
          : # unknown; skip to next candidate
          ;;
      esac
    fi
    candidate=$((candidate + 1))
    i=$((i + 1))
  done

  return 1
}

resolve_host_port() {
  local label="$1"
  local requested="$2"
  local fallback_start="$3"
  local rc

  if is_port_in_use "$requested"; then
    rc=0
  else
    rc=$?
  fi

  case "$rc" in
    1)
      printf "%s" "$requested"
      return 0
      ;;
    0)
      ;;
    2)
      warn "Could not reliably test requested ${label} port ${requested}; searching fallback range."
      ;;
    *)
      ;;
  esac

  local selected
  selected="$(find_available_port "$fallback_start")" || return 1
  warn "${label} port ${requested} is unavailable. Using ${selected} instead."
  printf "%s" "$selected"
}

resolve_ports() {
  log "Resolving host ports"

  if ! SELECTED_HTTP_PORT="$(resolve_host_port "HTTP" "$REQUESTED_HTTP_PORT" 8080)"; then
    die "Unable to find a free HTTP port."
  fi
  if ! SELECTED_POSTGRES_PORT="$(resolve_host_port "Postgres" "$REQUESTED_POSTGRES_PORT" 5434)"; then
    die "Unable to find a free Postgres port."
  fi
  if ! SELECTED_PGBOUNCER_PORT="$(resolve_host_port "PgBouncer" "$REQUESTED_PGBOUNCER_PORT" 6433)"; then
    die "Unable to find a free PgBouncer port."
  fi

  if [ "$SELECTED_HTTP_PORT" = "80" ]; then
    SELECTED_HOST_URL="http://localhost"
  else
    SELECTED_HOST_URL="http://localhost:${SELECTED_HTTP_PORT}"
  fi

  log "Selected ports: HTTP=${SELECTED_HTTP_PORT}, Postgres=${SELECTED_POSTGRES_PORT}, PgBouncer=${SELECTED_PGBOUNCER_PORT}"
}

run_compose() {
  docker compose -f "$COMPOSE_RENDERED_FILE" "$@"
}

write_compose_file() {
  COMPOSE_BASE_FILE="$INSTALL_DIR/configs/local/docker-compose.yml"
  COMPOSE_RENDERED_FILE="$INSTALL_DIR/configs/local/docker-compose.install.generated.yml"
  local stale_override_file="$INSTALL_DIR/configs/local/docker-compose.install.override.yml"

  [ -f "$COMPOSE_BASE_FILE" ] || die "Missing compose file: $COMPOSE_BASE_FILE"

  # Cleanup stale file from older installer versions.
  rm -f "$stale_override_file"

  awk \
    -v http_port="$SELECTED_HTTP_PORT" \
    -v postgres_port="$SELECTED_POSTGRES_PORT" \
    -v pgbouncer_port="$SELECTED_PGBOUNCER_PORT" \
    '{
      gsub(/"80:80"/, "\"" http_port ":80\"");
      gsub(/"5433:5432"/, "\"" postgres_port ":5432\"");
      gsub(/"6432:6432"/, "\"" pgbouncer_port ":6432\"");
      print;
    }' \
    "$COMPOSE_BASE_FILE" > "$COMPOSE_RENDERED_FILE"
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

update_local_host_config() {
  local config_path="$INSTALL_DIR/configs/local/convoy.json"
  local tmp_path="${config_path}.tmp"

  [ -f "$config_path" ] || die "Missing $config_path."

  awk -v host_url="$SELECTED_HOST_URL" '
    BEGIN { updated = 0 }
    {
      if (updated == 0 && $0 ~ /^[[:space:]]*"host"[[:space:]]*:[[:space:]]*"/) {
        sub(/"host"[[:space:]]*:[[:space:]]*"[^"]*"/, "\"host\": \"" host_url "\"")
        updated = 1
      }
      print
    }
    END {
      if (updated == 0) {
        exit 2
      }
    }
  ' "$config_path" > "$tmp_path" || {
    rm -f "$tmp_path"
    die "Failed to set host in $config_path."
  }

  mv "$tmp_path" "$config_path"
}

start_stack() {
  local compose_dir="$INSTALL_DIR/configs/local"

  [ -d "$compose_dir" ] || die "Missing compose directory: $compose_dir"

  if [ "${CONVOY_SKIP_PULL:-0}" != "1" ]; then
    log "Pulling latest images"
    run_compose pull
  fi

  log "Starting Convoy stack"
  run_compose up -d
}

wait_for_health() {
  local elapsed=0
  local health_url="${SELECTED_HOST_URL}/healthz"

  log "Waiting for Convoy health endpoint ($health_url)"

  until curl -fsS "$health_url" >/dev/null 2>&1; do
    if [ "$elapsed" -ge "$MAX_WAIT_SECONDS" ]; then
      die "Timed out waiting for health after ${MAX_WAIT_SECONDS}s. Check logs with: docker compose -f \"$COMPOSE_RENDERED_FILE\" logs"
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
  docker compose -f "$COMPOSE_RENDERED_FILE" ps
  docker compose -f "$COMPOSE_RENDERED_FILE" logs -f web agent
  docker compose -f "$COMPOSE_RENDERED_FILE" down

Open:
  ${SELECTED_HOST_URL}

EOF
}

main() {
  check_prereqs
  prepare_repo
  ensure_local_config
  resolve_ports
  update_local_host_config
  write_compose_file
  start_stack
  wait_for_health
  print_next_steps
}

main "$@"
