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

echo "🧪 Running System Integration Test (Dynamic Path Mode)"
echo "========================================================"

# Clean output directory
echo "📁 Cleaning output directory..."
rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

# Build the binary
echo "🔨 Building binary..."
cd "$PROJECT_ROOT"
make build > /dev/null 2>&1

# Run the binary with new dynamic path flags
echo "🚀 Running gitops-kustomzchk with dynamic paths..."
echo "   Template: services/\$SERVICE/clusters/\$CLUSTER/\$ENV"
echo "   Values: SERVICE=my-app;CLUSTER=alpha,beta;ENV=stg,prod"

"$BIN_PATH" --run-mode local \
    --kustomize-build-path "services/my-app/clusters/\$CLUSTER/\$ENV" \
    --kustomize-build-values "CLUSTER=alpha,beta;ENV=stg,prod" \
    --lc-before-manifests-path "$SCRIPT_DIR/before" \
    --lc-after-manifests-path "$SCRIPT_DIR/after" \
    --policies-path "$SCRIPT_DIR/policies" \
    --templates-path "$SCRIPT_DIR/templates" \
    --output-dir "$OUTPUT_DIR" \
    --enable-export-report true \
    --debug false

if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Binary execution failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Binary execution completed${NC}"

# Validate output files exist
echo ""
echo "📊 Validating outputs..."
echo "========================================================"

FAILED=0

if [ -f "$OUTPUT_DIR/report.json" ]; then
    echo -e "${GREEN}✅ report.json exists${NC}"
    
    # Check that all expected overlay keys are present
    if command -v jq > /dev/null 2>&1; then
        OVERLAY_KEYS=$(jq -r '.overlayKeys | sort | join(",")' "$OUTPUT_DIR/report.json" 2>/dev/null || echo "")
        EXPECTED_KEYS="alpha/prod,alpha/stg,beta/prod,beta/stg"
        
        if [ "$OVERLAY_KEYS" == "$EXPECTED_KEYS" ]; then
            echo -e "${GREEN}✅ All expected overlay keys present: $OVERLAY_KEYS${NC}"
        else
            echo -e "${RED}❌ Overlay keys mismatch${NC}"
            echo "   Expected: $EXPECTED_KEYS"
            echo "   Got: $OVERLAY_KEYS"
            FAILED=1
        fi
        
        # Check that manifest changes exist for each key
        for key in "alpha/stg" "alpha/prod" "beta/stg" "beta/prod"; do
            ESCAPED_KEY=$(echo "$key" | sed 's/\//\\\//g')
            HAS_KEY=$(jq -r ".manifestChanges[\"$key\"] != null" "$OUTPUT_DIR/report.json" 2>/dev/null || echo "false")
            if [ "$HAS_KEY" == "true" ]; then
                echo -e "${GREEN}✅ Manifest changes for '$key' present${NC}"
            else
                echo -e "${RED}❌ Manifest changes for '$key' missing${NC}"
                FAILED=1
            fi
        done
        
        # Verify dynamic path metadata is present
        BUILD_PATH=$(jq -r '.kustomizeBuildPath' "$OUTPUT_DIR/report.json" 2>/dev/null || echo "")
        if [ -n "$BUILD_PATH" ] && [ "$BUILD_PATH" != "null" ]; then
            echo -e "${GREEN}✅ kustomizeBuildPath present: $BUILD_PATH${NC}"
        else
            echo -e "${RED}❌ kustomizeBuildPath missing${NC}"
            FAILED=1
        fi
    else
        echo -e "${YELLOW}⚠️  jq not found, skipping detailed validation${NC}"
    fi
else
    echo -e "${RED}❌ report.json missing${NC}"
    FAILED=1
fi

if [ -f "$OUTPUT_DIR/report.md" ]; then
    echo -e "${GREEN}✅ report.md exists${NC}"
    
    # Check for overlay keys in markdown
    for key in "alpha/stg" "alpha/prod" "beta/stg" "beta/prod"; do
        if grep -q "$key" "$OUTPUT_DIR/report.md"; then
            echo -e "${GREEN}✅ '$key' found in report.md${NC}"
        else
            echo -e "${YELLOW}⚠️  '$key' not found in report.md${NC}"
        fi
    done
else
    echo -e "${RED}❌ report.md missing${NC}"
    FAILED=1
fi

echo ""
echo "========================================================"
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi

