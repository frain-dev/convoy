#!/usr/bin/env bash

set -euo pipefail

CONVOY_IMAGE="${CONVOY_IMAGE:-getconvoy/convoy:latest}"
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
USED_HOST_PORTS=()

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
    if is_reserved_port "$candidate"; then
      candidate=$((candidate + 1))
      i=$((i + 1))
      continue
    fi

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

is_reserved_port() {
  local port="$1"
  local used
  for used in "${USED_HOST_PORTS[@]-}"; do
    if [ "$used" = "$port" ]; then
      return 0
    fi
  done
  return 1
}

reserve_port() {
  local port="$1"
  USED_HOST_PORTS+=("$port")
}

resolve_host_port() {
  local label="$1"
  local requested="$2"
  local fallback_start="$3"
  local rc

  if is_reserved_port "$requested"; then
    rc=0
  elif is_port_in_use "$requested"; then
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
      warn "Could not reliably test requested ${label} port ${requested}; using it as requested."
      printf "%s" "$requested"
      return 0
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
  USED_HOST_PORTS=()

  if ! SELECTED_HTTP_PORT="$(resolve_host_port "HTTP" "$REQUESTED_HTTP_PORT" 8080)"; then
    die "Unable to find a free HTTP port."
  fi
  reserve_port "$SELECTED_HTTP_PORT"

  if ! SELECTED_POSTGRES_PORT="$(resolve_host_port "Postgres" "$REQUESTED_POSTGRES_PORT" 5434)"; then
    die "Unable to find a free Postgres port."
  fi
  reserve_port "$SELECTED_POSTGRES_PORT"

  if ! SELECTED_PGBOUNCER_PORT="$(resolve_host_port "PgBouncer" "$REQUESTED_PGBOUNCER_PORT" 6433)"; then
    die "Unable to find a free PgBouncer port."
  fi
  reserve_port "$SELECTED_PGBOUNCER_PORT"

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

generate_config_files() {
  log "Generating Convoy config files"

  local dir="$INSTALL_DIR/configs/local"
  mkdir -p "$dir/conf"


  cat > "$dir/docker-compose.yml" <<'EOF'
services:
    migrate:
        image: ${CONVOY_IMAGE:-getconvoy/convoy:latest}
        command: ["migrate", "up", "--config", "convoy.json"]
        volumes:
            - ./convoy.json:/convoy.json
        depends_on:
            postgres:
                condition: service_healthy
            pgbouncer:
                condition: service_started
            redis_server:
                condition: service_started

    web:
        image: ${CONVOY_IMAGE:-getconvoy/convoy:latest}
        command: ["server", "--config", "convoy.json"]
        volumes:
            - ./convoy.json:/convoy.json
        restart: unless-stopped
        healthcheck:
            test: ["CMD-SHELL", "wget -q --spider http://localhost:5005/healthz"]
            interval: 5s
            timeout: 30s
            retries: 10
            start_period: 10s
        depends_on:
            migrate:
                condition: service_completed_successfully

    agent:
        image: ${CONVOY_IMAGE:-getconvoy/convoy:latest}
        command: ["agent", "--config", "convoy.json"]
        volumes:
            - ./convoy.json:/convoy.json
        restart: unless-stopped
        healthcheck:
            test: ["CMD-SHELL", "wget -q --spider http://localhost:5008/healthz"]
            interval: 5s
            timeout: 30s
            retries: 10
            start_period: 15s
        depends_on:
            migrate:
                condition: service_completed_successfully

    caddy:
        image: caddy:2-alpine
        restart: unless-stopped
        ports:
            - "80:80"
        volumes:
            - ./Caddyfile:/etc/caddy/Caddyfile
            - caddy_data:/data
        depends_on:
            web:
                condition: service_healthy
            agent:
                condition: service_healthy

    postgres:
        image: postgres:15.2-alpine
        restart: unless-stopped
        ports:
            - "5433:5432"
        environment:
            POSTGRES_DB: convoy
            POSTGRES_USER: convoy
            POSTGRES_PASSWORD: pg_password
            PGDATA: /data/postgres
        volumes:
            - postgres_data:/data/postgres
        healthcheck:
            test: ["CMD-SHELL", "pg_isready -U convoy"]
            interval: 5s
            timeout: 5s
            retries: 5
            start_period: 10s
        command:
            - "postgres"
            - "-c"
            - "wal_level=logical"

    pgbouncer:
        image: bitnamilegacy/pgbouncer
        hostname: pgbouncer
        restart: unless-stopped
        ports:
            - "6432:6432"
        depends_on:
            postgres:
                condition: service_healthy
        env_file:
            - ./conf/.env
        volumes:
            - ./conf/:/bitnami/pgbouncer/conf/
            - ./conf/userlists.txt:/bitnami/userlists.txt

    redis_server:
        image: redis:7-alpine
        restart: unless-stopped
        command: ["redis-server", "--maxmemory", "256mb", "--maxmemory-policy", "allkeys-lru"]
        volumes:
            - redis_data:/data


volumes:
    postgres_data:
    redis_data:
    caddy_data:
EOF

  cat > "$dir/Caddyfile" <<'EOF'
:80 {
    # Data Plane — webhook source ingestion
    handle /ingest/* {
        reverse_proxy agent:5008
    }

    # Data Plane — portal link event operations
    handle /portal-api/events/* {
        reverse_proxy agent:5008
    }
    handle /portal-api/events {
        reverse_proxy agent:5008
    }

    # Data Plane — portal link delivery operations
    handle /portal-api/eventdeliveries/* {
        reverse_proxy agent:5008
    }
    handle /portal-api/eventdeliveries {
        reverse_proxy agent:5008
    }

    # Control Plane — management API, UI, portal management, everything else
    handle {
        reverse_proxy web:5005
    }
}
EOF

  cat > "$dir/conf/.env" <<'EOF'
## ****** DEPLOYMENT VARIABLES ******
PGBOUNCER_VERSION=1.22.1

## ****** POSTGRES DB ******
POSTGRESQL_USER=convoy
POSTGRESQL_PASSWORD=pg_password
POSTGRESQL_DATABASE=convoy
POSTGRESQL_HOST=postgres #should be your db host address
POSTGRESQL_OPTIONS="sslmode=disable&connect_timeout=30"

PGBOUNCER_AUTH_TYPE=trust
PGBOUNCER_USERLIST_FILE=/bitnami/userlists.txt
PGBOUNCER_DATABASE=${POSTGRESQL_DATABASE}
PGBOUNCER_AUTH_USER=convoy
PGBOUNCER_POOL_MODE=transaction
PGBOUNCER_MAX_CLIENT_CONN=500
PGBOUNCER_DEFAULT_POOL_SIZE=80
PGBOUNCER_MAX_DB_CONNECTIONS=250
PGBOUNCER_MAX_PREPARED_STATEMENTS=100
PGBOUNCER_IGNORE_STARTUP_PARAMETERS=extra_float_digits

# host should be your db host address
PGBOUNCER_DSN_0=pg1=host=postgres port=5432 dbname=convoy
EOF

  cat > "$dir/conf/userlists.txt" <<'EOF'
"convoy" "pg_password"
EOF

  if [ ! -f "$dir/convoy.json" ]; then
    cat > "$dir/convoy.json" <<'EOF'
{
    "host": "http://localhost",
    "database": {
        "host": "pgbouncer",
        "username": "convoy",
        "password": "pg_password",
        "database": "convoy",
        "port": 6432
    },
    "redis": {
        "scheme": "redis",
        "port": 6379,
        "host": "redis_server"
    },
    "server": {
        "http": {
            "port": 5005,
            "agent_port": 5008
        }
    },
    "auth": {
        "is_signup_enabled": true,
        "native": {
            "enabled": true
        },
        "jwt": {
            "enabled": true,
            "secret": "local-access-secret",
            "refresh_secret": "local-refresh-secret"
        }
    }
}
EOF
  else
    log "Existing convoy.json found; keeping host/auth settings"
     normalize_redis_transport "$dir/convoy.json"
  fi
}

normalize_redis_transport() {
  local config_path="$1"
  local tmp_path="${config_path}.tmp"

  [ -f "$config_path" ] || return 0

  awk '
    function emit_redis(ind, trailing) {
      printf "%s\"redis\": {\n", ind
      printf "%s    \"scheme\": \"redis\",\n", ind
      printf "%s    \"port\": 6379,\n", ind
      printf "%s    \"host\": \"redis_server\"\n", ind
      printf "%s}%s\n", ind, trailing
    }
    BEGIN { state = 0 }
    state == 0 {
      if ($0 ~ /"redis"[[:space:]]*:[[:space:]]*\{/) {
        match($0, /^[[:space:]]*/)
        redis_ind = substr($0, 1, RLENGTH)
        line = $0; opens = gsub(/\{/, "", line)
        line = $0; closes = gsub(/\}/, "", line)
        depth = opens - closes
        if (depth <= 0) {
          emit_redis(redis_ind, ($0 ~ /},[[:space:]]*$/) ? "," : "")
        } else {
          state = 1
        }
        next
      }
      print
      next
    }
    state == 1 {
      line = $0; opens = gsub(/\{/, "", line)
      line = $0; closes = gsub(/\}/, "", line)
      depth += opens - closes
      if (depth <= 0) {
        emit_redis(redis_ind, ($0 ~ /,[[:space:]]*$/) ? "," : "")
        state = 0
      }
      next
    }
  ' "$config_path" > "$tmp_path" || {
    rm -f "$tmp_path"
    die "Failed to normalize Redis transport in $config_path."
  }

  mv "$tmp_path" "$config_path"
}

ensure_local_config() {
  local config_path="$INSTALL_DIR/configs/local/convoy.json"

  if [ ! -f "$config_path" ]; then
    die "Missing $config_path after generating config files."
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

Open the dashboard:
  ${SELECTED_HOST_URL}

Log in with the default credentials:
  Email:    superuser@default.com
  Password: default

Useful commands:
  docker compose -f "$COMPOSE_RENDERED_FILE" ps
  docker compose -f "$COMPOSE_RENDERED_FILE" logs -f web agent
  docker compose -f "$COMPOSE_RENDERED_FILE" down

EOF
}

main() {
  check_prereqs
  generate_config_files
  ensure_local_config
  resolve_ports
  update_local_host_config
  write_compose_file
  start_stack
  wait_for_health
  print_next_steps
}

main "$@"
