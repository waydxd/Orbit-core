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

# ---------------------------------------------------------------------------
# Prefer Python3 (available on macOS by default) for proper YAML merging
# ---------------------------------------------------------------------------
if command -v python3 &>/dev/null; then
  echo "Using Python3 for YAML merging..."
  python3 - "$OPENAPI_DIR" "$OUTPUT_FILE" <<'PYEOF'
import sys, os, glob, re, subprocess

openapi_dir = sys.argv[1]
output_file = sys.argv[2]

# Try to load PyYAML; if missing, attempt to install it
try:
    import yaml
    HAS_YAML = True
except ImportError:
    HAS_YAML = False
    for pip_args in [
        [sys.executable, "-m", "pip", "install", "pyyaml", "--quiet"],
        [sys.executable, "-m", "pip", "install", "pyyaml", "--quiet", "--break-system-packages"],
    ]:
        try:
            subprocess.check_call(pip_args, stderr=subprocess.DEVNULL, stdout=subprocess.DEVNULL)
            import yaml
            HAS_YAML = True
            break
        except Exception:
            pass

# ---------------------------------------------------------------------------
# PyYAML path: parse + deep-merge + dump
# ---------------------------------------------------------------------------
if HAS_YAML:
    merged = {
        "openapi": "3.0.3",
        "info": {
            "title": "Orbit Core API",
            "description": "A comprehensive personal productivity API with calendar, tasks, location tracking, and integration capabilities",
            "version": "1.0.0",
            "contact": {"name": "Orbit Core Team"},
        },
        "servers": [{"url": "http://localhost:8080", "description": "Development server"}],
        "paths": {},
        "components": {"securitySchemes": {}, "schemas": {}},
    }

    def deep_merge(base, override):
        for k, v in override.items():
            if k in base and isinstance(base[k], dict) and isinstance(v, dict):
                deep_merge(base[k], v)
            else:
                base[k] = v

    for f in sorted(glob.glob(os.path.join(openapi_dir, "paths", "*.yaml"))):
        print(f"  Processing path: {os.path.basename(f)}")
        with open(f) as fh:
            data = yaml.safe_load(fh)
        if data and "paths" in data:
            deep_merge(merged["paths"], data["paths"])

    for f in sorted(glob.glob(os.path.join(openapi_dir, "schemas", "*.yaml"))):
        print(f"  Processing schema: {os.path.basename(f)}")
        with open(f) as fh:
            data = yaml.safe_load(fh)
        if data and "components" in data and "schemas" in data["components"]:
            deep_merge(merged["components"]["schemas"], data["components"]["schemas"])

    for f in sorted(glob.glob(os.path.join(openapi_dir, "components", "*.yaml"))):
        print(f"  Processing component: {os.path.basename(f)}")
        with open(f) as fh:
            data = yaml.safe_load(fh)
        if data and "components" in data:
            deep_merge(merged["components"], data["components"])

    with open(output_file, "w") as fh:
        yaml.dump(merged, fh, default_flow_style=False, allow_unicode=True, sort_keys=False)

# ---------------------------------------------------------------------------
# No-PyYAML path: manual string-based assembly (preserves original formatting)
# ---------------------------------------------------------------------------
else:
    print("PyYAML unavailable – using manual text assembly (formatting preserved)")

    def read_section(filepath, skip_prefixes):
        """Read a YAML file, skipping the specified leading key lines."""
        with open(filepath) as fh:
            lines = fh.readlines()
        result = []
        skip_set = set(skip_prefixes)
        for i, line in enumerate(lines):
            stripped = line.rstrip('\n')
            if stripped in skip_set:
                continue
            result.append(line)
        # Remove leading/trailing blank lines
        while result and not result[0].strip():
            result.pop(0)
        while result and not result[-1].strip():
            result.pop()
        return result

    out_lines = []
    out_lines.append("openapi: '3.0.3'\n")
    out_lines.append("info:\n")
    out_lines.append("  title: Orbit Core API\n")
    out_lines.append("  description: A comprehensive personal productivity API with calendar, tasks, location tracking, and integration capabilities\n")
    out_lines.append("  version: '1.0.0'\n")
    out_lines.append("  contact:\n")
    out_lines.append("    name: Orbit Core Team\n")
    out_lines.append("servers:\n")
    out_lines.append("  - url: http://localhost:8080\n")
    out_lines.append("    description: Development server\n")
    out_lines.append("paths:\n")

    for f in sorted(glob.glob(os.path.join(openapi_dir, "paths", "*.yaml"))):
        print(f"  Processing path: {os.path.basename(f)}")
        section = read_section(f, ["paths:"])
        out_lines.extend(section)
        out_lines.append("\n")

    out_lines.append("components:\n")
    out_lines.append("  securitySchemes:\n")

    for f in sorted(glob.glob(os.path.join(openapi_dir, "components", "*.yaml"))):
        print(f"  Processing component: {os.path.basename(f)}")
        section = read_section(f, ["components:", "  securitySchemes:"])
        out_lines.extend(section)
        out_lines.append("\n")

    out_lines.append("  schemas:\n")

    for f in sorted(glob.glob(os.path.join(openapi_dir, "schemas", "*.yaml"))):
        print(f"  Processing schema: {os.path.basename(f)}")
        section = read_section(f, ["components:", "  schemas:"])
        out_lines.extend(section)
        out_lines.append("\n")

    with open(output_file, "w") as fh:
        fh.writelines(out_lines)

print(f"\n✓ Merged OpenAPI spec written to: {output_file}")
PYEOF

# ---------------------------------------------------------------------------
# Fallback: pure-bash / awk approach (no external dependencies)
# ---------------------------------------------------------------------------
else
  echo "Python3 not found – using bash/awk fallback..."

  OUTPUT_FILE_TMP="${OUTPUT_FILE}.tmp"

  # Write the fixed header
  cat > "$OUTPUT_FILE_TMP" << 'HEADER'
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
paths:
HEADER

  # Helper: strip the top-level "paths:" line and emit remaining non-empty lines
  strip_paths_key() {
    local file="$1"
    awk 'NR==1 && /^paths:/ { next } NF { print }' "$file"
  }

  # Helper: strip "components:" and "  schemas:" wrapper lines
  strip_schemas_key() {
    local file="$1"
    awk 'NR==1 && /^components:/ { next }
         NR==2 && /^  schemas:/ { next }
         NF { print }' "$file"
  }

  # Helper: strip "components:" and "  securitySchemes:" wrapper lines
  strip_security_key() {
    local file="$1"
    awk 'NR==1 && /^components:/ { next }
         NR==2 && /^  securitySchemes:/ { next }
         NF { print }' "$file"
  }

  echo ""
  echo "Processing path files..."
  for file in $(find "$OPENAPI_DIR/paths" -name "*.yaml" -type f | sort); do
    echo "  Processing: $(basename "$file")"
    strip_paths_key "$file" >> "$OUTPUT_FILE_TMP"
    echo "" >> "$OUTPUT_FILE_TMP"
  done

  echo "components:" >> "$OUTPUT_FILE_TMP"
  echo "  securitySchemes:" >> "$OUTPUT_FILE_TMP"

  echo ""
  echo "Processing component files..."
  for file in $(find "$OPENAPI_DIR/components" -name "*.yaml" -type f | sort); do
    echo "  Processing: $(basename "$file")"
    strip_security_key "$file" >> "$OUTPUT_FILE_TMP"
    echo "" >> "$OUTPUT_FILE_TMP"
  done

  echo "  schemas:" >> "$OUTPUT_FILE_TMP"

  echo ""
  echo "Processing schema files..."
  for file in $(find "$OPENAPI_DIR/schemas" -name "*.yaml" -type f | sort); do
    echo "  Processing: $(basename "$file")"
    strip_schemas_key "$file" >> "$OUTPUT_FILE_TMP"
    echo "" >> "$OUTPUT_FILE_TMP"
  done

  mv "$OUTPUT_FILE_TMP" "$OUTPUT_FILE"
  echo ""
  echo "✓ Merged OpenAPI spec written to: $OUTPUT_FILE"
fi
