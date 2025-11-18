#!/usr/bin/env bash

#MISE description="Lint SQL files"
#MISE dir="{{ config_root }}"
#MISE sources=["sql/*.sql", ".squawk.toml"]

set -e

echo "Linting SQL files"
exec squawk 'sql/*.sql'
