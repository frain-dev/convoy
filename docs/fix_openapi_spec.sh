#!/bin/bash
# fix-swagger-all.sh

echo "🔥 Fixing Swagger 2.0 specs (all issues in one pass)..."

# Check for required tools
if ! command -v jq &> /dev/null; then
    echo "❌ jq is required. Install with: brew install jq (macOS) or apt-get install jq (Linux)"
    exit 1
fi

if ! command -v yq &> /dev/null; then
    echo "❌ yq is required. Install with: brew install yq (macOS) or snap install yq (Linux)"
    exit 1
fi

# Function to fix a file (handles both JSON and YAML)
fix_file_inplace() {
    local file="$1"

    if [ ! -f "$file" ]; then
        echo "⚠️  $file not found, skipping..."
        return
    fi

    echo "Processing: $file"

    echo "  Fixing content-type, produces, and allOf structures..."

    # Create temp file
    tmpfile=$(mktemp)

    if [[ "$file" == *.json ]]; then
        # Process JSON with jq
        jq '
        # CRITICAL FIX: Ensure produces field exists for clean OpenAPI 3.0 conversion
        # If no produces field exists, add it
        if .produces | not then
            .produces = ["application/json"]
        # If produces is ["*/*"], fix it
        elif .produces == ["*/*"] then
            .produces = ["application/json"]
        else . end |

        # Also ensure consumes exists for POST/PUT/PATCH
        if .consumes | not then
            .consumes = ["application/json"]
        else . end |

        # Fix 2: Simplify allOf in definitions
        .definitions |= with_entries(
            .value |= (
                if type == "object" and has("allOf") then
                    .allOf |
                    reduce .[] as $item ({};
                        if $item | type == "object" then
                            .properties = (.properties // {}) + ($item.properties // {}) |
                            .required = (.required // []) + ($item.required // [])
                        else . end
                    )
                else . end
            )
        ) |

        # Fix 3: Open up arbitrary-JSON object properties. A bare
        # {type: object} with no properties/additionalProperties is a CLOSED
        # empty object; strict SDK generators (e.g. zod) strip every key of
        # such payloads. json.RawMessage-backed fields must be open maps, and
        # they serialize as JSON null when unset (Go nil map/RawMessage), so
        # they must also be nullable or strict parsers crash on reads.
        .definitions |= with_entries(
            if .value.properties then
                .value.properties |= map_values(
                    if type == "object" and .type == "object"
                       and (has("properties") | not) then
                        (if has("additionalProperties") | not then
                            .additionalProperties = true
                         else . end) |
                        .["x-nullable"] = true
                    else . end
                )
            else . end
        ) |

        # Fix 3b: Named map definitions (datastore.M, datastore.HttpHeader,
        # httpheader.HTTPHeader, handlers.Stub) are bare objects referenced
        # via $ref. Go serializes nil maps as JSON null, so the definitions
        # themselves must be open and nullable or strict parsers crash on
        # reads (e.g. filter raw_headers: null). Typed maps (for example
        # httpheader.HTTPHeader: map of string arrays) keep their typed
        # additionalProperties and only gain the nullable marker; overwriting
        # them with true widens generated client types to map[string]any.
        .definitions |= with_entries(
            if .value.type == "object" and (.value | has("properties") | not) then
                (if (.value.additionalProperties? | type) != "object" then
                    .value.additionalProperties = true
                 else . end) |
                .value["x-nullable"] = true
            else . end
        ) |

        # Fix 4: Accept Go zero values in string enums. The server serializes
        # unset enum fields as "", but a closed enum makes strict generated
        # parsers (Jackson, PHP/Ruby OpenAPI Generator) crash on reads.
        # Server-side request validation still rejects "" where required.
        .definitions |= with_entries(
            .value |= (
                if .type == "string" and has("enum") then
                    .enum |= (if index("") then . else . + [""] end)
                else . end
            ) |
            if .value.properties then
                .value.properties |= map_values(
                    if type == "object" and .type == "string" and has("enum") then
                        .enum |= (if index("") then . else . + [""] end)
                    else . end
                )
            else . end
        ) |

        # Fix 4b: portal link endpoints_metadata is aggregated in SQL with
        # ARRAY_AGG(DISTINCT CASE ...), which emits [null] when the link has
        # no endpoints. Deployed servers ship this, so the items must be
        # nullable for strict parsers even after the SQL is fixed.
        .definitions |= with_entries(
            if .value.properties.endpoints_metadata?.items["$ref"]? then
                .value.properties.endpoints_metadata.items |=
                    {"allOf": [{"$ref": .["$ref"]}], "x-nullable": true}
            else . end
        ) |

        # Fix 7: the onboard op declares a JSON body plus a multipart file
        # param; swagger->openapi3 conversion collapses that dual shape to
        # application/octet-stream, which the server rejects (415). Keep the
        # generated-SDK contract JSON-only; CSV upload remains a server
        # feature outside the generated clients.
        .paths |= with_entries(
            if (.key | endswith("/onboard")) and .value.post then
                .value.post.consumes = ["application/json"] |
                .value.post.parameters |= map(select(.in != "formData"))
            else . end
        ) |

        # Fix 5: handlers.Stub envelope data is always null on the wire
        # (ServerResponse{data=Stub} handlers render data: null). Mark those
        # inline response data properties nullable so strict parsers accept it.
        .paths |= walk(
            if type == "object" and (.data? | type) == "object"
               and .data["$ref"]? == "#/definitions/handlers.Stub" then
                .data = {"allOf": [{"$ref": "#/definitions/handlers.Stub"}], "x-nullable": true}
            else . end
        )
        ' "$file" > "$tmpfile"

    elif [[ "$file" == *.yaml ]] || [[ "$file" == *.yml ]]; then
        # Convert YAML to JSON, fix it, convert back
        yq eval -o=json "$file" | jq '
        # CRITICAL FIX: Ensure produces field exists
        if .produces | not then
            .produces = ["application/json"]
        elif .produces == ["*/*"] then
            .produces = ["application/json"]
        else . end |

        if .consumes | not then
            .consumes = ["application/json"]
        else . end |

        .definitions |= with_entries(
            .value |= (
                if type == "object" and has("allOf") then
                    .allOf |
                    reduce .[] as $item ({};
                        if $item | type == "object" then
                            .properties = (.properties // {}) + ($item.properties // {}) |
                            .required = (.required // []) + ($item.required // [])
                        else . end
                    )
                else . end
            )
        ) |

        # Fix 3: Open up arbitrary-JSON object properties. A bare
        # {type: object} with no properties/additionalProperties is a CLOSED
        # empty object; strict SDK generators (e.g. zod) strip every key of
        # such payloads. json.RawMessage-backed fields must be open maps, and
        # they serialize as JSON null when unset (Go nil map/RawMessage), so
        # they must also be nullable or strict parsers crash on reads.
        .definitions |= with_entries(
            if .value.properties then
                .value.properties |= map_values(
                    if type == "object" and .type == "object"
                       and (has("properties") | not) then
                        (if has("additionalProperties") | not then
                            .additionalProperties = true
                         else . end) |
                        .["x-nullable"] = true
                    else . end
                )
            else . end
        ) |

        # Fix 3b: Named map definitions (datastore.M, datastore.HttpHeader,
        # httpheader.HTTPHeader, handlers.Stub) are bare objects referenced
        # via $ref. Go serializes nil maps as JSON null, so the definitions
        # themselves must be open and nullable or strict parsers crash on
        # reads (e.g. filter raw_headers: null). Typed maps (for example
        # httpheader.HTTPHeader: map of string arrays) keep their typed
        # additionalProperties and only gain the nullable marker; overwriting
        # them with true widens generated client types to map[string]any.
        .definitions |= with_entries(
            if .value.type == "object" and (.value | has("properties") | not) then
                (if (.value.additionalProperties? | type) != "object" then
                    .value.additionalProperties = true
                 else . end) |
                .value["x-nullable"] = true
            else . end
        ) |

        # Fix 4: Accept Go zero values in string enums. The server serializes
        # unset enum fields as "", but a closed enum makes strict generated
        # parsers (Jackson, PHP/Ruby OpenAPI Generator) crash on reads.
        # Server-side request validation still rejects "" where required.
        .definitions |= with_entries(
            .value |= (
                if .type == "string" and has("enum") then
                    .enum |= (if index("") then . else . + [""] end)
                else . end
            ) |
            if .value.properties then
                .value.properties |= map_values(
                    if type == "object" and .type == "string" and has("enum") then
                        .enum |= (if index("") then . else . + [""] end)
                    else . end
                )
            else . end
        ) |

        # Fix 4b: portal link endpoints_metadata is aggregated in SQL with
        # ARRAY_AGG(DISTINCT CASE ...), which emits [null] when the link has
        # no endpoints. Deployed servers ship this, so the items must be
        # nullable for strict parsers even after the SQL is fixed.
        .definitions |= with_entries(
            if .value.properties.endpoints_metadata?.items["$ref"]? then
                .value.properties.endpoints_metadata.items |=
                    {"allOf": [{"$ref": .["$ref"]}], "x-nullable": true}
            else . end
        ) |

        # Fix 7: the onboard op declares a JSON body plus a multipart file
        # param; swagger->openapi3 conversion collapses that dual shape to
        # application/octet-stream, which the server rejects (415). Keep the
        # generated-SDK contract JSON-only; CSV upload remains a server
        # feature outside the generated clients.
        .paths |= with_entries(
            if (.key | endswith("/onboard")) and .value.post then
                .value.post.consumes = ["application/json"] |
                .value.post.parameters |= map(select(.in != "formData"))
            else . end
        ) |

        # Fix 5: handlers.Stub envelope data is always null on the wire
        # (ServerResponse{data=Stub} handlers render data: null). Mark those
        # inline response data properties nullable so strict parsers accept it.
        .paths |= walk(
            if type == "object" and (.data? | type) == "object"
               and .data["$ref"]? == "#/definitions/handlers.Stub" then
                .data = {"allOf": [{"$ref": "#/definitions/handlers.Stub"}], "x-nullable": true}
            else . end
        )
        ' | yq eval -P - > "$tmpfile"
    fi

    # Replace original if different
    if ! diff -q "$file" "$tmpfile" > /dev/null; then
        cp "$tmpfile" "$file"
        echo "  ✅ File updated"
    else
        echo "  ✓ No structural changes needed"
    fi

    # Cleanup
    rm "$tmpfile"

    echo ""
}

# Main execution
echo ""

# Process JSON
if [ -f "./docs/swagger.json" ]; then
    fix_file_inplace "./docs/swagger.json"
else
    echo "⚠️  ./docs/swagger.json not found"
fi

# Process YAML
if [ -f "./docs/swagger.yaml" ]; then
    fix_file_inplace "./docs/swagger.yaml"
elif [ -f "./docs/swagger.yml" ]; then
    fix_file_inplace "./docs/swagger.yml"
else
    echo "⚠️  ./docs/swagger.yaml not found"
fi

echo ""
echo "✨ Swagger 2.0 fixes complete!"
echo ""
