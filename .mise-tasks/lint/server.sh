#!/usr/bin/env bash

#MISE description="Reformat Server Files"
#MISE dir="{{ config_root }}"

set -e

echo "Linting server"

go vet ./...
exec gofmt -s -l .
