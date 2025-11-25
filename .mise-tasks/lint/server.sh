#!/usr/bin/env bash

#MISE description="Reformat Server Files"
#MISE dir="{{ config_root }}"

set -e

echo "Linting server"

exec gofmt -s -l .
