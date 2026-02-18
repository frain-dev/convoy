#!/usr/bin/env bash

#MISE description="Validate SQL migrations for mixed index/DDL operations"
#MISE dir="{{ config_root }}"
#MISE sources=["sql/*.sql", "cmd/validate-migrations/**/*.go"]

set -e

echo "Validating SQL migrations..."
exec go run cmd/validate-migrations/main.go
