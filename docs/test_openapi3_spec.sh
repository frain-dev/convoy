#!/usr/bin/env bash
set -euo pipefail

# Validate the generated contract and regression-test the post-conversion fix.
spec=docs/v3/openapi3.json
path='/v1/projects/{projectID}/eventdeliveries/{eventDeliveryID}/deliveryattempts'
jq -e --arg path "$path" '
  .paths[$path].get as $op |
  $op.responses["200"].content["application/json"].schema.allOf[1].properties.data.oneOf as $union |
  $op["x-convoy-paginated-response"] == true and
  ($union | length) == 2 and
  $union[0].type == "array" and
  $union[0].items["$ref"] == "#/components/schemas/datastore.DeliveryAttempt" and
  $union[1].type == "object" and
  $union[1].required == ["content", "next_cursor"] and
  $union[1].properties.content == $union[0] and
  $union[1].properties.next_cursor.type == "string"
' "$spec" > /dev/null

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT
jq -S . "$spec" > "$tmpdir/expected.json"
jq -S -f docs/fix_openapi3_spec.jq "$spec" > "$tmpdir/actual.json"
cmp "$tmpdir/expected.json" "$tmpdir/actual.json"

# Starting from the Swagger-converted array must recreate the committed union,
# without changing other operations, response codes, envelope fields or schemas.
jq --arg path "$path" '
  .paths[$path].get.responses["200"].content["application/json"].schema.allOf[1].properties.data |= .oneOf[0]
' "$spec" | jq -S -f docs/fix_openapi3_spec.jq > "$tmpdir/actual.json"
cmp "$tmpdir/expected.json" "$tmpdir/actual.json"

# An unexpected source shape must fail rather than silently ship a broken SDK.
if jq --arg path "$path" '
  .paths[$path].get.responses["200"].content["application/json"].schema.allOf[1].properties.data = {type: "string"}
' "$spec" | jq -f docs/fix_openapi3_spec.jq > /dev/null 2>&1; then
    echo 'Expected invalid source response to fail' >&2
    exit 1
fi

# YAML conversion normalizes date examples differently elsewhere in the spec.
# Compare the entire affected operation, not unrelated pre-existing differences.
yq -o=json docs/v3/openapi3.yaml | jq -S --arg path "$path" '.paths[$path].get' > "$tmpdir/yaml.json"
jq -S --arg path "$path" '.paths[$path].get' "$spec" > "$tmpdir/operation.json"
cmp "$tmpdir/operation.json" "$tmpdir/yaml.json"
echo 'OpenAPI pagination contract checks passed.'
