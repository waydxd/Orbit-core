#!/bin/bash
# Verification script to check for mismatches between OpenAPI docs and actual code

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
echo "=== OpenAPI Specification Verification ==="
echo "Project root: $PROJECT_ROOT"
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

ERRORS=0
WARNINGS=0

check_endpoints() {
    echo "Checking endpoint coverage..."
    echo ""

    # Get all endpoints from code
    CODE_ENDPOINTS=$(grep -rh "HandleFunc" "$PROJECT_ROOT/internal"*/service.go 2>/dev/null | sed -E 's/.*HandleFunc\("([^"]+)".*/\1/' | sort -u || true)

    # Get all endpoints from OpenAPI
    OPENAPI_ENDPOINTS=$(grep -rhE "^  /api/v1|^  /health" "$PROJECT_ROOT/docs/openapi"/paths/*.yaml 2>/dev/null | sed 's/^  //; s/:$//' | sort -u || true)

    # Check each code endpoint is in OpenAPI
    for endpoint in $CODE_ENDPOINTS; do
        # Skip health check endpoint (prefix handling)
        if [[ "$endpoint" == "/health" ]] || [[ "$endpoint" == "/api/v1"* ]]; then
            if ! echo "$OPENAPI_ENDPOINTS" | grep -q "^$endpoint"; then
                echo -e "${RED}✗ Missing in OpenAPI: $endpoint${NC}"
                ((ERRORS++))
            fi
        fi
    done

    if [ $ERRORS -eq 0 ]; then
        echo -e "${GREEN}✓ All code endpoints are documented${NC}"
    fi
    echo ""
}

check_schemas() {
    echo "Checking schema coverage..."
    echo ""

    # Get all struct names from code (simplified)
    CODE_STRUCTS=$(grep -rh "^type.*struct" "$PROJECT_ROOT/internal"*/service.go 2>/dev/null | sed -E 's/^type ([A-Za-z_][A-Za-z0-9_]*) struct.*/\1/' | sort -u || true)

    # Get all schema names from OpenAPI
    OPENAPI_SCHEMAS=$(grep -rh "^    [A-Z][a-zA-Z]*:" "$PROJECT_ROOT/docs/openapi"/schemas/*.yaml 2>/dev/null | sed 's/^    //; s/:$//' | sort -u || true)

    MISSING=0
    for struct in $CODE_STRUCTS; do
        # Skip internal structs and services
        if [[ ! "$struct" =~ ^(Service|Repository|GRPCClient|Config|Logger)$ ]]; then
            if ! echo "$OPENAPI_SCHEMAS" | grep -q "^${struct}$"; then
                echo -e "${YELLOW}⚠ Potential missing schema: $struct${NC}"
                ((WARNINGS++))
            fi
        fi
    done

    if [ $WARNINGS -eq 0 ]; then
        echo -e "${GREEN}✓ Common schemas are documented${NC}"
    fi
    echo ""
}

check_openapi_structure() {
    echo "Checking merged OpenAPI structure..."
    echo ""

    GENERATED_FILE="$PROJECT_ROOT/docs/openapi.yaml"
    if [ ! -f "$GENERATED_FILE" ]; then
        echo -e "${YELLOW}⚠ Generated spec not found at docs/openapi.yaml; skipping structural checks${NC}"
        echo ""
        return
    fi

    STRUCT_OUTPUT=$(python3 - "$GENERATED_FILE" <<'PY'
import sys
from pathlib import Path

try:
    import yaml
except Exception as exc:
    print(f"ERR:pyyaml_missing:{exc}")
    sys.exit(2)

target = Path(sys.argv[1])
doc = yaml.safe_load(target.read_text(encoding="utf-8"))

errors = []

if not isinstance(doc, dict):
    errors.append("OpenAPI document root is not a mapping")
else:
    if not isinstance(doc.get("paths"), dict):
        errors.append("Top-level 'paths' is missing or not a mapping")
    if not isinstance(doc.get("components"), dict):
        errors.append("Top-level 'components' is missing or not a mapping")

def walk(node, path):
    if isinstance(node, dict):
        if "schema" in node and isinstance(node["schema"], dict) and "paths" in node["schema"]:
            errors.append(f"Invalid nested 'paths' inside schema at {'/'.join(path + ['schema'])}")
        if "properties" in node and isinstance(node["properties"], dict) and "components" in node["properties"]:
            errors.append(f"Invalid nested 'components' inside schema properties at {'/'.join(path + ['properties'])}")
        for key, value in node.items():
            walk(value, path + [str(key)])
    elif isinstance(node, list):
        for idx, item in enumerate(node):
            walk(item, path + [str(idx)])

walk(doc, [])

if errors:
    for err in errors:
        print(f"ERR:{err}")
    sys.exit(1)

print("OK")
PY
)
    STRUCT_EXIT=$?

    if [ $STRUCT_EXIT -eq 2 ]; then
        echo -e "${YELLOW}⚠ Could not run structural validation: $STRUCT_OUTPUT${NC}"
    elif [ $STRUCT_EXIT -ne 0 ]; then
        echo "$STRUCT_OUTPUT" | while IFS= read -r line; do
            if [[ "$line" == ERR:* ]]; then
                echo -e "${RED}✗ ${line#ERR:}${NC}"
            fi
        done
        ((ERRORS++))
    else
        echo -e "${GREEN}✓ Merged OpenAPI structure is valid${NC}"
    fi

    echo ""
}

check_security() {
    echo "Checking security configuration..."
    echo ""

    if grep -q "BearerAuth" "$PROJECT_ROOT/docs/openapi/components/security.yaml"; then
        echo -e "${GREEN}✓ BearerAuth security scheme defined${NC}"
    else
        echo -e "${RED}✗ BearerAuth security scheme not found${NC}"
        ((ERRORS++))
    fi

    # Check auth middleware in code
    if grep -q "AuthMiddleware" "$PROJECT_ROOT/internal/gateway/service.go"; then
        echo -e "${GREEN}✓ Auth middleware implemented in gateway${NC}"
    else
        echo -e "${RED}✗ Auth middleware not found in gateway${NC}"
        ((ERRORS++))
    fi
    echo ""
}

check_yaml_syntax() {
    echo "Checking YAML syntax..."
    echo ""

    # Count YAML files
    YAML_FILES=$(find "$PROJECT_ROOT/docs/openapi" -name "*.yaml" | wc -l)
    echo "Found $YAML_FILES YAML files"

    # Try basic YAML validation (very simple check)
    INVALID=0
    for file in $(find "$PROJECT_ROOT/docs/openapi" -name "*.yaml"); do
        if ! grep -q "^[a-z]" "$file"; then
            echo -e "${YELLOW}⚠ Potential issue in: $(basename $file)${NC}"
            ((INVALID++))
        fi
    done

    if [ $INVALID -eq 0 ]; then
        echo -e "${GREEN}✓ YAML files appear valid${NC}"
    else
        echo -e "${YELLOW}⚠ $INVALID YAML files may have issues (run with proper YAML validator)${NC}"
    fi
    echo ""
}

check_response_codes() {
    echo "Checking response codes..."
    echo ""

    # Get response codes from OpenAPI
    CODES=$(grep -rh "'[0-9][0-9][0-9]':" "$PROJECT_ROOT/docs/openapi"/paths/*.yaml | sed "s/.*'\([0-9]*\)'.*/\1/" | sort -u)

    echo "Response codes documented:"
    for code in $CODES; do
        case $code in
            200) echo -e "${GREEN}✓${NC} 200 (OK)" ;;
            201) echo -e "${GREEN}✓${NC} 201 (Created)" ;;
            204) echo -e "${GREEN}✓${NC} 204 (No Content)" ;;
            400) echo -e "${GREEN}✓${NC} 400 (Bad Request)" ;;
            401) echo -e "${GREEN}✓${NC} 401 (Unauthorized)" ;;
            404) echo -e "${GREEN}✓${NC} 404 (Not Found)" ;;
            409) echo -e "${GREEN}✓${NC} 409 (Conflict)" ;;
            410) echo -e "${GREEN}✓${NC} 410 (Gone)" ;;
            500) echo -e "${GREEN}✓${NC} 500 (Internal Error)" ;;
            *) echo -e "${YELLOW}?${NC} $code (Unknown)" ;;
        esac
    done
    echo ""
}

print_summary() {
    echo "=== Verification Summary ==="
    if [ $ERRORS -eq 0 ]; then
        echo -e "${GREEN}No errors found!${NC}"
    else
        echo -e "${RED}$ERRORS errors found${NC}"
    fi

    if [ $WARNINGS -gt 0 ]; then
        echo -e "${YELLOW}$WARNINGS warnings found${NC}"
    fi
    echo ""

    return $ERRORS
}

# Run all checks
check_endpoints
check_schemas
check_security
check_yaml_syntax
check_response_codes
check_openapi_structure
print_summary

