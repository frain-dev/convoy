#!/usr/bin/env bash
#
# run-shard.sh GROUP SHARD TOTAL
#
# Splits the (non-e2e) test suite across parallel CI jobs so wall-clock time is
# bounded by the slowest shard instead of the serial total.
#
#   GROUP=api   -> shards the `./api` package by top-level test name. This package
#                  holds the heavy integration suites and is the dominant cost.
#   GROUP=unit  -> shards every other package (excluding e2e and the api root).
#
# SHARD is 1-based; TOTAL is the shard count for that group. Each shard runs
# against its own Postgres + Redis service (provided by the workflow matrix), so
# we keep `-race -p 1` for deterministic, collision-free runs within a shard.
set -euo pipefail

GROUP="${1:?usage: run-shard.sh GROUP SHARD TOTAL}"
SHARD="${2:?usage: run-shard.sh GROUP SHARD TOTAL}"
TOTAL="${3:?usage: run-shard.sh GROUP SHARD TOTAL}"

idx=$(( SHARD - 1 ))

case "$GROUP" in
    api)
        # Top-level Test* funcs in the api package, sorted for a stable partition.
        # TestMain is excluded: it is the entrypoint, not a -run target.
        # Populated with a read loop rather than mapfile for bash 3.2 portability.
        names=()
        while IFS= read -r n; do names+=("$n"); done < <(
            grep -rhoE '^func (Test[A-Za-z0-9_]+)' api/*_test.go \
                | awk '{print $2}' | grep -vx 'TestMain' | sort -u
        )
        sel=()
        for i in "${!names[@]}"; do
            if (( i % TOTAL == idx )); then sel+=("${names[$i]}"); fi
        done
        if (( ${#sel[@]} == 0 )); then
            echo "api shard ${SHARD}/${TOTAL}: no tests assigned"
            exit 0
        fi
        regex="^($(IFS='|'; echo "${sel[*]}"))$"
        echo "api shard ${SHARD}/${TOTAL}: ${#sel[@]} tests -> ${regex}"
        exec go test -race -p 1 ./api/ -run "$regex" -v -timeout 30m
        ;;
    unit)
        pkgs=()
        while IFS= read -r p; do pkgs+=("$p"); done < <(
            go list ./... | grep -v '/e2e' | grep -vE '/api$' | sort -u
        )
        sel=()
        for i in "${!pkgs[@]}"; do
            if (( i % TOTAL == idx )); then sel+=("${pkgs[$i]}"); fi
        done
        if (( ${#sel[@]} == 0 )); then
            echo "unit shard ${SHARD}/${TOTAL}: no packages assigned"
            exit 0
        fi
        echo "unit shard ${SHARD}/${TOTAL}: ${#sel[@]} packages"
        exec go test -race -p 1 "${sel[@]}" -v -timeout 30m
        ;;
    *)
        echo "unknown group: ${GROUP} (expected 'api' or 'unit')" >&2
        exit 2
        ;;
esac
