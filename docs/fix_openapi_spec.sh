#!/bin/bash
# fix-swagger-all.sh

echo "🔥 Fixing Swagger 2.0 specs (all issues in one pass)..."

# Check OS for sed compatibility
if [[ "$(uname)" == "Darwin" ]]; then
    # macOS requires an empty string argument after -i
    sed_inplace() { sed -i '' "$@"; }
else
    # Linux
    sed_inplace() { sed -i "$@"; }
fi

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

    # Create backup
    cp "$file" "${file}.bak"

    # Step 1: First fix nullable with sed (works on both JSON and YAML)
    echo "  Step 1: Fixing nullable fields..."

    if [[ "$file" == *.json ]]; then
        # JSON fixes
        sed_inplace 's/"x-nullable": true/"nullable": true/g' "$file"
        sed_inplace 's/"x-nullable": false//g' "$file"
        # Clean up extra commas
        sed_inplace 's/,,/,/g' "$file"
        sed_inplace 's/{,/{/g' "$file"
        sed_inplace 's/,}/}/g' "$file"

    elif [[ "$file" == *.yaml ]] || [[ "$file" == *.yml ]]; then
        # YAML fixes
        sed_inplace 's/x-nullable: true/nullable: true/g' "$file"
        sed_inplace '/x-nullable: false/d' "$file"
        sed_inplace '/^[[:space:]]*x-nullable:/d' "$file"
    fi

    # Step 2: Use jq/yq for structural fixes
    echo "  Step 2: Fixing content-type, produces, and allOf structures..."

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
    rm "${file}.bak"

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
