#!/usr/bin/env bash

#MISE description="Generate from SQLC files"
#MISE dir="{{ config_root }}"

set -e
exec sqlc generate -f ./sqlc.yaml
