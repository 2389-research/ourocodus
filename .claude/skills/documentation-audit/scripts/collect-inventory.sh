#!/bin/bash
# Documentation Audit - Inventory Collection
# Discovers all documentation and builds code index

set -eo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(dirname "$SCRIPT_DIR")"
REPO_ROOT="$(cd "$SKILL_DIR/../../.." && pwd)"
REPORTS_DIR="$SKILL_DIR/reports"
CONFIG_FILE="$SKILL_DIR/config.yaml"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Create reports directory
mkdir -p "$REPORTS_DIR"

log_info "Starting documentation inventory collection..."
log_info "Repository root: $REPO_ROOT"

# Initialize JSON output
OUTPUT_FILE="$REPORTS_DIR/inventory.json"
cat > "$OUTPUT_FILE" <<'EOF'
{
  "timestamp": "",
  "repo_root": "",
  "documentation": {
    "files": [],
    "total_count": 0,
    "by_type": {}
  },
  "code": {
    "packages": [],
    "total_packages": 0,
    "exported_symbols": 0,
    "has_doc": 0
  },
  "metrics": {
    "doc_coverage_pct": 0,
    "total_docs_files": 0,
    "total_go_files": 0,
    "total_lines_of_docs": 0
  }
}
EOF

# Temporary files
TEMP_DOCS_LIST="$REPORTS_DIR/.temp_docs_list.txt"
TEMP_CODE_INDEX="$REPORTS_DIR/.temp_code_index.json"

log_info "Discovering documentation files..."

# Find all markdown files (excluding excluded paths)
# Note: Working from current directory, using absolute paths
find "$REPO_ROOT" -type f \( -name "*.md" -o -name "doc.go" \) \
  ! -path "*/vendor/*" \
  ! -path "*/node_modules/*" \
  ! -path "*/.git/*" \
  ! -path "*/build/*" \
  ! -path "*/dist/*" \
  ! -path "*/.worktrees/*" \
  ! -path "*/.claude/skills/documentation-audit/*" \
  > "$TEMP_DOCS_LIST" || true

DOCS_COUNT=$(wc -l < "$TEMP_DOCS_LIST" | tr -d ' ')
log_success "Found $DOCS_COUNT documentation files"

# Build documentation index
log_info "Categorizing documentation files..."

# Use awk instead of grep to avoid bash/grep interaction issues
README_FILES=$(awk '/[Rr][Ee][Aa][Dd][Mm][Ee]\.md/ {count++} END {print count+0}' "$TEMP_DOCS_LIST")
CONTRIBUTING_FILES=$(awk '/[Cc][Oo][Nn][Tt][Rr][Ii][Bb][Uu][Tt][Ii][Nn][Gg]\.md/ {count++} END {print count+0}' "$TEMP_DOCS_LIST")
SECURITY_FILES=$(awk '/[Ss][Ee][Cc][Uu][Rr][Ii][Tt][Yy]\.md/ {count++} END {print count+0}' "$TEMP_DOCS_LIST")
DOC_GO_FILES=$(awk '/doc\.go/ {count++} END {print count+0}' "$TEMP_DOCS_LIST")
OTHER_DOCS=$(( DOCS_COUNT - README_FILES - CONTRIBUTING_FILES - SECURITY_FILES - DOC_GO_FILES ))

log_info "  README files: $README_FILES"
log_info "  CONTRIBUTING files: $CONTRIBUTING_FILES"
log_info "  SECURITY files: $SECURITY_FILES"
log_info "  doc.go files: $DOC_GO_FILES"
log_info "  Other docs: $OTHER_DOCS"

# Build code index using go list
log_info "Building code index..."

if command -v go &> /dev/null; then
    (cd "$REPO_ROOT" && go list -json ./...) > "$TEMP_CODE_INDEX" 2>/dev/null || {
        log_warning "Failed to run 'go list', code index will be incomplete"
        echo "[]" > "$TEMP_CODE_INDEX"
    }

    # Count packages
    PACKAGE_COUNT=$( (cd "$REPO_ROOT" && go list ./...) 2>/dev/null | wc -l | tr -d ' ')
    log_success "Found $PACKAGE_COUNT Go packages"

    # Count total Go files
    GO_FILES=$(find "$REPO_ROOT" -type f -name "*.go" \
        ! -path "*/vendor/*" \
        ! -path "*_test.go" \
        ! -name "*_gen.go" \
        ! -name "*.pb.go" | wc -l | tr -d ' ')
    log_info "  Go source files: $GO_FILES"

    # Count exported symbols (approximate using grep)
    # This is a rough heuristic - Phase 2 will do proper AST analysis
    EXPORTED_SYMBOLS=$(grep -r "^func [A-Z]" "$REPO_ROOT" --include="*.go" \
        --exclude-dir=vendor \
        --exclude-dir=.git 2>/dev/null | wc -l 2>/dev/null | tr -d ' ')
    EXPORTED_SYMBOLS=${EXPORTED_SYMBOLS:-0}
    log_info "  Exported functions (approx): $EXPORTED_SYMBOLS"

else
    log_warning "Go not found in PATH, skipping code index"
    PACKAGE_COUNT=0
    GO_FILES=0
    EXPORTED_SYMBOLS=0
fi

# Count total lines of documentation
log_info "Calculating documentation volume..."

TOTAL_DOC_LINES=0
while IFS= read -r doc_file; do
    if [ -f "$doc_file" ]; then
        LINES=$(wc -l < "$doc_file" 2>/dev/null | tr -d ' ')
        TOTAL_DOC_LINES=$((TOTAL_DOC_LINES + LINES))
    fi
done < "$TEMP_DOCS_LIST"

log_info "  Total documentation lines: $TOTAL_DOC_LINES"

# Calculate basic doc coverage (will be more accurate in Phase 2)
if [ "$EXPORTED_SYMBOLS" -gt 0 ]; then
    # Count documented exports (use awk to avoid bash/grep issues)
    # This is a rough estimate - Phase 2 will do proper AST analysis
    DOCUMENTED_EXPORTS=$(find "$REPO_ROOT" -name "*.go" \
        ! -path "*/vendor/*" \
        ! -path "*/.git/*" \
        -exec awk '/^\/\// {doc=1} /^func [A-Z]/ && doc {count++; doc=0} /^[^\/]/ && !/^func/ {doc=0} END {print count+0}' {} + | awk '{sum+=$1} END {print sum+0}')
    DOCUMENTED_EXPORTS=${DOCUMENTED_EXPORTS:-0}

    DOC_COVERAGE_PCT=$((DOCUMENTED_EXPORTS * 100 / EXPORTED_SYMBOLS))
    log_info "  Documentation coverage: ${DOC_COVERAGE_PCT}% ($DOCUMENTED_EXPORTS/$EXPORTED_SYMBOLS)"
else
    DOC_COVERAGE_PCT=0
    DOCUMENTED_EXPORTS=0
fi

# Check for key documentation files
log_info "Checking for required documentation..."

check_file() {
    local file=$1
    local desc=$2
    if [ -f "$REPO_ROOT/$file" ]; then
        log_success "  ✓ $desc ($file)"
        return 0
    else
        log_warning "  ✗ $desc ($file) - MISSING"
        return 1
    fi
}

REQUIRED_DOCS_PRESENT=0
REQUIRED_DOCS_MISSING=0

check_file "README.md" "Project README" && REQUIRED_DOCS_PRESENT=$((REQUIRED_DOCS_PRESENT+1)) || REQUIRED_DOCS_MISSING=$((REQUIRED_DOCS_MISSING+1))
check_file "CONTRIBUTING.md" "Contributing guide" && REQUIRED_DOCS_PRESENT=$((REQUIRED_DOCS_PRESENT+1)) || REQUIRED_DOCS_MISSING=$((REQUIRED_DOCS_MISSING+1))
check_file "SECURITY.md" "Security policy" && REQUIRED_DOCS_PRESENT=$((REQUIRED_DOCS_PRESENT+1)) || REQUIRED_DOCS_MISSING=$((REQUIRED_DOCS_MISSING+1))
check_file "LICENSE" "License file" && REQUIRED_DOCS_PRESENT=$((REQUIRED_DOCS_PRESENT+1)) || REQUIRED_DOCS_MISSING=$((REQUIRED_DOCS_MISSING+1))

# Build final JSON report
log_info "Generating inventory report..."

# Build docs array for JSON
DOCS_JSON_ARRAY="["
FIRST=true
while IFS= read -r doc_file; do
    if [ -f "$doc_file" ]; then
        if [ "$FIRST" = false ]; then
            DOCS_JSON_ARRAY+=","
        fi
        FIRST=false
        DOCS_JSON_ARRAY+="\"$doc_file\""
    fi
done < "$TEMP_DOCS_LIST"
DOCS_JSON_ARRAY+="]"

# Update JSON with actual data using jq if available
if command -v jq &> /dev/null; then
    jq --arg timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
       --arg repo_root "$REPO_ROOT" \
       --argjson docs "$DOCS_JSON_ARRAY" \
       --argjson docs_count "$DOCS_COUNT" \
       --argjson readme_count "$README_FILES" \
       --argjson contrib_count "$CONTRIBUTING_FILES" \
       --argjson security_count "$SECURITY_FILES" \
       --argjson docgo_count "$DOC_GO_FILES" \
       --argjson pkg_count "$PACKAGE_COUNT" \
       --argjson exported_symbols "$EXPORTED_SYMBOLS" \
       --argjson has_doc "$DOCUMENTED_EXPORTS" \
       --argjson doc_coverage "$DOC_COVERAGE_PCT" \
       --argjson go_files "$GO_FILES" \
       --argjson doc_lines "$TOTAL_DOC_LINES" \
       '.timestamp = $timestamp |
        .repo_root = $repo_root |
        .documentation.files = $docs |
        .documentation.total_count = $docs_count |
        .documentation.by_type = {
          "README": $readme_count,
          "CONTRIBUTING": $contrib_count,
          "SECURITY": $security_count,
          "doc.go": $docgo_count,
          "other": ($docs_count - $readme_count - $contrib_count - $security_count - $docgo_count)
        } |
        .code.total_packages = $pkg_count |
        .code.exported_symbols = $exported_symbols |
        .code.has_doc = $has_doc |
        .metrics.doc_coverage_pct = $doc_coverage |
        .metrics.total_docs_files = $docs_count |
        .metrics.total_go_files = $go_files |
        .metrics.total_lines_of_docs = $doc_lines' \
       "$OUTPUT_FILE" > "$OUTPUT_FILE.tmp" && mv "$OUTPUT_FILE.tmp" "$OUTPUT_FILE"
else
    log_warning "jq not found, JSON output will be basic format"
fi

# Cleanup temp files
rm -f "$TEMP_DOCS_LIST" "$TEMP_CODE_INDEX"

# Print summary
echo ""
log_success "=== Inventory Collection Complete ==="
echo ""
echo "Documentation Files: $DOCS_COUNT"
echo "Go Packages: $PACKAGE_COUNT"
echo "Go Source Files: $GO_FILES"
echo "Exported Symbols: $EXPORTED_SYMBOLS"
echo "Documentation Coverage: ${DOC_COVERAGE_PCT}%"
echo "Total Doc Lines: $TOTAL_DOC_LINES"
echo ""
echo "Required docs present: $REQUIRED_DOCS_PRESENT"
echo "Required docs missing: $REQUIRED_DOCS_MISSING"
echo ""
log_info "Full inventory saved to: $OUTPUT_FILE"

# Exit with warning if doc coverage is below threshold
if [ "$DOC_COVERAGE_PCT" -lt 85 ]; then
    echo ""
    log_warning "Documentation coverage (${DOC_COVERAGE_PCT}%) is below recommended threshold (85%)"
fi

if [ "$REQUIRED_DOCS_MISSING" -gt 0 ]; then
    echo ""
    log_warning "$REQUIRED_DOCS_MISSING required documentation file(s) are missing"
fi

exit 0
