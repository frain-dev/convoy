#!/usr/bin/env bash

#MISE description="Generate mocks for interfaces"
#MISE dir="{{ config_root }}"
#MISE cache=false
set -e

go generate ./...