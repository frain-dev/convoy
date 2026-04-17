#!/usr/bin/env bash

#MISE description="Generate Swagger docs"
#MISE dir="{{ config_root }}"
#MISE sources=["api/**/*.go"]

set -e

# preflight: check required tools
for tool in swag jq yq api-spec-converter openapi; do
  command -v "$tool" >/dev/null 2>&1 || { echo "❌ $tool not found. Run 'mise install' to install required tools."; exit 1; }
done

echo "Generating docs"

#generate custom swag tags
go run docs/annotate_dtos/main.go

#generate v2 openapi specs
swag init --generatedTime --parseDependency --parseDependencyLevel 3 --parseInternal -g handlers/main.go -d api/ api/*
swag fmt -d ./api

# fix openapi2 specs (structural fixes, add proper produce/consume tags, replace x-nullable..)
bash docs/fix_openapi_spec.sh

#generate v3 specs
api-spec-converter --from=swagger_2 --to=openapi_3 -s yaml ./docs/swagger.yaml > ./docs/v3/openapi3.yaml
api-spec-converter --from=swagger_2 --to=openapi_3 ./docs/swagger.json > ./docs/v3/openapi3.json

# add region descriptions and EU server (swag only supports a single host)
yq -i '.servers[0].description = "US Region" | .servers += [{"url": "https://eu.getconvoy.cloud/api", "description": "EU Region"}]' ./docs/v3/openapi3.yaml
jq '.servers[0].description = "US Region" | .servers += [{"url": "https://eu.getconvoy.cloud/api", "description": "EU Region"}]' ./docs/v3/openapi3.json > ./docs/v3/openapi3.json.tmp && mv ./docs/v3/openapi3.json.tmp ./docs/v3/openapi3.json

# validate specs
echo "Validating specs..."
openapi swagger validate ./docs/swagger.json
openapi swagger validate ./docs/swagger.yaml
openapi spec validate ./docs/v3/openapi3.yaml
