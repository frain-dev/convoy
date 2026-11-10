#!/usr/bin/env bash
set -euo pipefail

# Run after Swagger conversion in both make generate_docs and mise gen/docs.
# The annotation and item schema remain owned by the Go handler.
tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
jq -f docs/fix_openapi3_spec.jq docs/v3/openapi3.json > "$tmpdir/spec.json"
mv "$tmpdir/spec.json" docs/v3/openapi3.json

# Edit only the annotated YAML response, preserving unrelated YAML formatting.
# Build each union from the YAML source too, so either source drifting fails.
yq -o=json docs/v3/openapi3.yaml | jq -f docs/fix_openapi3_spec.jq > "$tmpdir/spec.json"
while IFS=$'\t' read -r OP_PATH OP_METHOD; do
    export OP_PATH OP_METHOD
    jq --arg path "$OP_PATH" --arg method "$OP_METHOD" \
        '.paths[$path][$method].responses["200"].content["application/json"].schema.allOf[1].properties.data' \
        "$tmpdir/spec.json" | yq -P -o=yaml > "$tmpdir/data.yaml"
    export DATA_FILE="$tmpdir/data.yaml"
    yq -i '.paths[strenv(OP_PATH)][strenv(OP_METHOD)].responses["200"].content["application/json"].schema.allOf[1].properties.data = load(strenv(DATA_FILE))' docs/v3/openapi3.yaml
done < <(jq -r '.paths | to_entries[] | .key as $path | .value | to_entries[] | select(.value["x-convoy-paginated-response"]? == true) | [$path, .key] | @tsv' "$tmpdir/spec.json")
