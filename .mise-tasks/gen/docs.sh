#!/usr/bin/env bash

#MISE description="Generate Swagger docs"
#MISE dir="{{ config_root }}"
#MISE sources=["api/**/*.go"]

set -e

echo "Generating docs"
swag init --generatedTime --parseDependency --parseDependencyLevel 3 --parseInternal -g handlers/main.go -d api/ api/*
swag fmt -d ./api
go run v3gen/main.go
