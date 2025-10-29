#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BIN_PATH="$PROJECT_ROOT/bin/gitops-kustomzchk"
OUTPUT_DIR="$SCRIPT_DIR/output"
JURY_OUTPUT_DIR="$SCRIPT_DIR/jury_output"

echo "🧪 Running System Integration Test (Local Mode)"
echo "================================================"

# Clean output directory
echo "📁 Cleaning output directory..."
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Build the binary
echo "🔨 Building binary..."
cd "$PROJECT_ROOT"
make build > /dev/null 2>&1

# Run the binary
echo "🚀 Running gitops-kustomzchk in local mode..."
"$BIN_PATH" --run-mode local \
    --service my-app \
    --environments stg,prod \
    --lc-before-manifests-path "$SCRIPT_DIR/before/services" \
    --lc-after-manifests-path "$SCRIPT_DIR/after/services" \
    --policies-path "$SCRIPT_DIR/policies" \
    --templates-path "$SCRIPT_DIR/templates" \
    --output-dir "$OUTPUT_DIR" \
    --enable-export-report true \
    --enable-export-performance-report false \
    --debug false > /dev/null 2>&1

if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Binary execution failed${NC}"
    exit 1
fi

echo "✅ Binary execution completed"

# Compare outputs
echo ""
echo "📊 Comparing outputs..."
echo "================================================"

FAILED=0

# Compare report.json (normalizing timestamps)
if [ -f "$OUTPUT_DIR/report.json" ] && [ -f "$JURY_OUTPUT_DIR/report.json" ]; then
    # Use jq to remove timestamp fields and normalize diff timestamps in content
    if command -v jq > /dev/null 2>&1; then
        # Remove top-level timestamp and normalize diff timestamps in content strings
        jq '
            del(.timestamp) |
            .manifestChanges = (.manifestChanges | to_entries | map(
                .value.content |= (
                    gsub("--- before\\t[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+ [+-][0-9]{4})?"; "--- before\\tTIMESTAMP") |
                    gsub("\\+\\+\\+ after\\t[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+ [+-][0-9]{4})?"; "+++ after\\tTIMESTAMP")
                )
            ) | from_entries)
        ' "$OUTPUT_DIR/report.json" > "$OUTPUT_DIR/report.json.normalized"
        
        jq '
            del(.timestamp) |
            .manifestChanges = (.manifestChanges | to_entries | map(
                .value.content |= (
                    gsub("--- before\\t[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+ [+-][0-9]{4})?"; "--- before\\tTIMESTAMP") |
                    gsub("\\+\\+\\+ after\\t[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+ [+-][0-9]{4})?"; "+++ after\\tTIMESTAMP")
                )
            ) | from_entries)
        ' "$JURY_OUTPUT_DIR/report.json" > "$JURY_OUTPUT_DIR/report.json.normalized"
        
        if diff -q "$OUTPUT_DIR/report.json.normalized" "$JURY_OUTPUT_DIR/report.json.normalized" > /dev/null 2>&1; then
            echo -e "${GREEN}✅ report.json matches${NC}"
        else
            echo -e "${RED}❌ report.json differs${NC}"
            echo ""
            echo "Showing differences (normalized, timestamps removed):"
            echo "---------------------------------------------------"
            # Use diff with color if supported, otherwise fall back to regular diff
            if diff --color=always -u "$JURY_OUTPUT_DIR/report.json.normalized" "$OUTPUT_DIR/report.json.normalized" 2>/dev/null | head -100; then
                :
            else
                diff -u "$JURY_OUTPUT_DIR/report.json.normalized" "$OUTPUT_DIR/report.json.normalized" | head -100
            fi
            echo ""
            echo "Full diff saved to: $OUTPUT_DIR/report.json.diff"
            diff -u "$JURY_OUTPUT_DIR/report.json.normalized" "$OUTPUT_DIR/report.json.normalized" > "$OUTPUT_DIR/report.json.diff" || true
            FAILED=1
        fi
        
        # Clean up normalized files
        rm -f "$OUTPUT_DIR/report.json.normalized" "$JURY_OUTPUT_DIR/report.json.normalized"
    else
        echo -e "${YELLOW}⚠️  jq not found, skipping report.json comparison${NC}"
    fi
else
    echo -e "${RED}❌ report.json missing${NC}"
    FAILED=1
fi

# Compare report.md (ignoring timestamp lines and diff timestamps)
if [ -f "$OUTPUT_DIR/report.md" ] && [ -f "$JURY_OUTPUT_DIR/report.md" ]; then
    # Filter out timestamp lines and normalize diff timestamps (including nanoseconds and timezone)
    sed -E \
        -e '/^[0-9]{4}-[0-9]{2}-[0-9]{2}/d' \
        -e 's/(--- before\t)[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+ [+-][0-9]{4})?/\1TIMESTAMP/' \
        -e 's/(\+\+\+ after\t)[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+ [+-][0-9]{4})?/\1TIMESTAMP/' \
        "$OUTPUT_DIR/report.md" > "$OUTPUT_DIR/report.md.normalized"
    
    sed -E \
        -e '/^[0-9]{4}-[0-9]{2}-[0-9]{2}/d' \
        -e 's/(--- before\t)[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+ [+-][0-9]{4})?/\1TIMESTAMP/' \
        -e 's/(\+\+\+ after\t)[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+ [+-][0-9]{4})?/\1TIMESTAMP/' \
        "$JURY_OUTPUT_DIR/report.md" > "$JURY_OUTPUT_DIR/report.md.normalized"
    
    if diff -q "$OUTPUT_DIR/report.md.normalized" "$JURY_OUTPUT_DIR/report.md.normalized" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ report.md matches${NC}"
    else
        echo -e "${RED}❌ report.md differs${NC}"
        echo ""
        echo "Showing differences (normalized, timestamps removed):"
        echo "---------------------------------------------------"
        # Use diff with color if supported, otherwise fall back to regular diff
        if diff --color=always -u "$JURY_OUTPUT_DIR/report.md.normalized" "$OUTPUT_DIR/report.md.normalized" 2>/dev/null | head -100; then
            :
        else
            diff -u "$JURY_OUTPUT_DIR/report.md.normalized" "$OUTPUT_DIR/report.md.normalized" | head -100
        fi
        echo ""
        echo "Full diff saved to: $OUTPUT_DIR/report.md.diff"
        diff -u "$JURY_OUTPUT_DIR/report.md.normalized" "$OUTPUT_DIR/report.md.normalized" > "$OUTPUT_DIR/report.md.diff" || true
        FAILED=1
    fi
    
    # Clean up normalized files
    rm -f "$OUTPUT_DIR/report.md.normalized" "$JURY_OUTPUT_DIR/report.md.normalized"
else
    echo -e "${RED}❌ report.md missing${NC}"
    FAILED=1
fi

echo ""
echo "================================================"
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi

