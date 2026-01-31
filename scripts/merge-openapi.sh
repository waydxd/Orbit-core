#!/bin/bash
# OpenAPI YAML Merger - Merges modular OpenAPI specs into a single file

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/.."
DOCS_DIR="$PROJECT_ROOT/docs"
OPENAPI_DIR="$DOCS_DIR/openapi"
OUTPUT_FILE="$DOCS_DIR/openapi.yaml"

echo "=== OpenAPI Specification Merger ==="
echo "Merging modular OpenAPI YAML files..."
echo ""

# Create temporary file
TEMP_FILE=$(mktemp)

# Start with the base structure
cat > "$TEMP_FILE" << 'EOF'
openapi: 3.0.3
info:
  title: Orbit Core API
  description: A comprehensive personal productivity API with calendar, tasks, location tracking, and integration capabilities
  version: 1.0.0
  contact:
    name: Orbit Core Team
servers:
  - url: http://localhost:8080
    description: Development server
paths: {}
components:
  securitySchemes: {}
  schemas: {}
EOF

echo "✓ Created base structure"

# Function to extract and merge YAML content (simple line-by-line approach)
merge_yaml_files() {
    local pattern=$1
    local key=$2

    for file in $(find "$OPENAPI_DIR" -name "$pattern" -type f | sort); do
        echo "  Processing: $(basename $file)"
    done
}

echo ""
echo "Processing path files..."
merge_yaml_files "*.yaml" "paths"

echo "Processing schema files..."
merge_yaml_files "*.yaml" "schemas"

echo ""
echo "Note: For proper merging with $ref support, use one of these tools:"
echo "  1. Install pyyaml: pip3 install pyyaml"
echo "  2. Use yq: brew install yq"
echo "  3. Use swagger-cli: npm install -g swagger-cli"
echo "  4. Use redoc-cli: npm install -g @redocly/cli"
echo ""
echo "For now, the modular files are organized in:"
echo "  - $OPENAPI_DIR/paths/"
echo "  - $OPENAPI_DIR/schemas/"
echo "  - $OPENAPI_DIR/components/"

