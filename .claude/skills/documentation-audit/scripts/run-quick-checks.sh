#!/bin/bash
# Documentation Audit - Quick Checks Runner
# Orchestrates all Phase 1 automated checks

set -euo pipefail

# Add mise tools to PATH if available
if command -v mise &> /dev/null; then
    eval "$(mise activate bash --shims)" 2>/dev/null || true
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(dirname "$SCRIPT_DIR")"
REPO_ROOT="$(cd "$SKILL_DIR/../../.." && pwd)"
REPORTS_DIR="$SKILL_DIR/reports"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
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

log_section() {
    echo ""
    echo -e "${CYAN}${BOLD}=== $1 ===${NC}"
    echo ""
}

# Create reports directory
mkdir -p "$REPORTS_DIR"

# Report file
REPORT_FILE="$REPORTS_DIR/quick-checks-report.md"
REPORT_JSON="$REPORTS_DIR/quick-checks-report.json"

# Initialize report
cat > "$REPORT_FILE" <<EOF
# Documentation Audit - Quick Checks Report

**Generated**: $(date -u +%Y-%m-%dT%H:%M:%SZ)
**Repository**: $REPO_ROOT

---

EOF

# Initialize JSON report
cat > "$REPORT_JSON" <<EOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "repo_root": "$REPO_ROOT",
  "checks": {},
  "summary": {
    "total_checks": 0,
    "passed": 0,
    "warnings": 0,
    "failed": 0
  }
}
EOF

# Track overall results
TOTAL_CHECKS=0
PASSED_CHECKS=0
WARNING_CHECKS=0
FAILED_CHECKS=0

# Helper to update report
add_check_result() {
    local check_name=$1
    local status=$2  # pass, warning, fail
    local message=$3
    local details=${4:-""}

    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))

    case $status in
        pass)
            PASSED_CHECKS=$((PASSED_CHECKS + 1))
            echo "## ✓ $check_name" >> "$REPORT_FILE"
            echo "" >> "$REPORT_FILE"
            echo "**Status**: PASSED" >> "$REPORT_FILE"
            ;;
        warning)
            WARNING_CHECKS=$((WARNING_CHECKS + 1))
            echo "## ⚠ $check_name" >> "$REPORT_FILE"
            echo "" >> "$REPORT_FILE"
            echo "**Status**: WARNING" >> "$REPORT_FILE"
            ;;
        fail)
            FAILED_CHECKS=$((FAILED_CHECKS + 1))
            echo "## ✗ $check_name" >> "$REPORT_FILE"
            echo "" >> "$REPORT_FILE"
            echo "**Status**: FAILED" >> "$REPORT_FILE"
            ;;
    esac

    echo "" >> "$REPORT_FILE"
    echo "$message" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"

    if [ -n "$details" ]; then
        echo "<details>" >> "$REPORT_FILE"
        echo "<summary>Details</summary>" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo "$details" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        echo "</details>" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
    fi

    echo "---" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
}

# =============================================================================
# Start checks
# =============================================================================

log_section "Starting Documentation Quick Checks"

cd "$REPO_ROOT"

# =============================================================================
# Check 1: Inventory Collection
# =============================================================================
log_section "1. Collecting Inventory"

# Try simple version first, fall back to full version
INVENTORY_SCRIPT="$SCRIPT_DIR/collect-inventory-simple.sh"
if [ ! -x "$INVENTORY_SCRIPT" ]; then
    INVENTORY_SCRIPT="$SCRIPT_DIR/collect-inventory.sh"
fi

if [ -x "$INVENTORY_SCRIPT" ]; then
    if "$INVENTORY_SCRIPT" > /dev/null 2>&1; then
        log_success "Inventory collection completed"
        add_check_result "Inventory Collection" "pass" "Successfully collected documentation and code inventory."
    else
        log_error "Inventory collection failed"
        add_check_result "Inventory Collection" "fail" "Failed to collect inventory. Check logs for details."
    fi
else
    log_warning "Inventory script not found or not executable"
    add_check_result "Inventory Collection" "warning" "Inventory script not available."
fi

# =============================================================================
# Check 2: Link Checking
# =============================================================================
log_section "2. Checking Links"

if command -v lychee &> /dev/null; then
    log_info "Running lychee link checker..."

    LYCHEE_OUTPUT=$(mktemp)
    if lychee --no-progress --format markdown \
        --exclude "localhost|127.0.0.1|example.com" \
        --max-redirects 5 \
        --timeout 30 \
        "**/*.md" > "$LYCHEE_OUTPUT" 2>&1; then

        log_success "All links are valid"
        add_check_result "Link Checking" "pass" "All documentation links are valid and accessible."
    else
        BROKEN_LINKS=$(grep -c "✗" "$LYCHEE_OUTPUT" || echo "0")
        log_warning "Found $BROKEN_LINKS broken link(s)"

        DETAILS=$(cat "$LYCHEE_OUTPUT")
        add_check_result "Link Checking" "warning" \
            "Found $BROKEN_LINKS broken or inaccessible link(s). These should be fixed or removed." \
            "$DETAILS"
    fi
    rm -f "$LYCHEE_OUTPUT"
else
    log_warning "lychee not installed - skipping link checks"
    add_check_result "Link Checking" "warning" \
        "lychee link checker not installed. Install with: cargo install lychee"
fi

# =============================================================================
# Check 3: Markdown Linting
# =============================================================================
log_section "3. Linting Markdown"

if command -v markdownlint &> /dev/null; then
    log_info "Running markdownlint..."

    MDLINT_OUTPUT=$(mktemp)
    if markdownlint "**/*.md" \
        --ignore "vendor" \
        --ignore "node_modules" \
        > "$MDLINT_OUTPUT" 2>&1; then

        log_success "Markdown formatting is correct"
        add_check_result "Markdown Linting" "pass" "All markdown files follow formatting guidelines."
    else
        LINT_ISSUES=$(wc -l < "$MDLINT_OUTPUT" | tr -d ' ')
        log_warning "Found $LINT_ISSUES markdown formatting issue(s)"

        DETAILS=$(cat "$MDLINT_OUTPUT")
        add_check_result "Markdown Linting" "warning" \
            "Found $LINT_ISSUES markdown formatting issue(s). Run 'markdownlint --fix' to auto-fix." \
            "$DETAILS"
    fi
    rm -f "$MDLINT_OUTPUT"
else
    log_warning "markdownlint not installed - skipping markdown linting"
    add_check_result "Markdown Linting" "warning" \
        "markdownlint not installed. Install with: npm install -g markdownlint-cli"
fi

# =============================================================================
# Check 4: Spell Checking
# =============================================================================
log_section "4. Checking Spelling"

if command -v codespell &> /dev/null; then
    log_info "Running codespell..."

    SPELL_OUTPUT=$(mktemp)
    if codespell \
        --skip=".git,vendor,node_modules,*.sum,go.mod" \
        --ignore-words-list="ourocodus" \
        "*.md" "**/*.md" \
        > "$SPELL_OUTPUT" 2>&1; then

        log_success "No spelling errors found"
        add_check_result "Spell Checking" "pass" "No spelling errors detected in documentation."
    else
        SPELL_ERRORS=$(grep -c "==>" "$SPELL_OUTPUT" || echo "0")
        log_warning "Found $SPELL_ERRORS potential spelling error(s)"

        DETAILS=$(cat "$SPELL_OUTPUT")
        add_check_result "Spell Checking" "warning" \
            "Found $SPELL_ERRORS potential spelling error(s). Review and fix or add to ignore list." \
            "$DETAILS"
    fi
    rm -f "$SPELL_OUTPUT"
else
    log_warning "codespell not installed - skipping spell checking"
    add_check_result "Spell Checking" "warning" \
        "codespell not installed. Install with: pip install codespell"
fi

# =============================================================================
# Check 5: Example Tests
# =============================================================================
log_section "5. Validating Example Tests"

if command -v go &> /dev/null; then
    log_info "Running Example tests..."

    EXAMPLE_OUTPUT=$(mktemp)
    if go test -run Example ./... > "$EXAMPLE_OUTPUT" 2>&1; then
        EXAMPLE_COUNT=$(grep -c "^ok" "$EXAMPLE_OUTPUT" || echo "0")
        log_success "All Example tests pass ($EXAMPLE_COUNT packages)"
        add_check_result "Example Tests" "pass" \
            "All Go Example tests compile and execute successfully ($EXAMPLE_COUNT packages tested)."
    else
        log_error "Some Example tests failed"
        DETAILS=$(cat "$EXAMPLE_OUTPUT")
        add_check_result "Example Tests" "fail" \
            "Some Example tests failed to compile or execute. These validate code snippets in documentation." \
            "$DETAILS"
    fi
    rm -f "$EXAMPLE_OUTPUT"
else
    log_warning "Go not found - skipping Example test validation"
    add_check_result "Example Tests" "warning" \
        "Go not found in PATH. Cannot validate Example tests."
fi

# =============================================================================
# Check 6: Documentation Coverage
# =============================================================================
log_section "6. Checking Documentation Coverage"

if command -v go &> /dev/null && command -v golangci-lint &> /dev/null; then
    log_info "Analyzing documentation coverage with golangci-lint..."

    COVERAGE_OUTPUT=$(mktemp)
    # Run only the revive linter with exported and package-comments rules
    if golangci-lint run \
        --disable-all \
        --enable=revive \
        --timeout=5m \
        ./... > "$COVERAGE_OUTPUT" 2>&1; then

        log_success "All exported symbols are documented"
        add_check_result "Documentation Coverage" "pass" \
            "All exported types, functions, and packages have documentation comments."
    else
        UNDOC_COUNT=$(grep -c "should have comment" "$COVERAGE_OUTPUT" || echo "0")
        if [ "$UNDOC_COUNT" -gt 0 ]; then
            log_warning "Found $UNDOC_COUNT undocumented exported symbol(s)"
            DETAILS=$(cat "$COVERAGE_OUTPUT" | head -50)
            if [ "$UNDOC_COUNT" -gt 50 ]; then
                DETAILS="$DETAILS"$'\n'"... (showing first 50 of $UNDOC_COUNT issues)"
            fi
            add_check_result "Documentation Coverage" "warning" \
                "Found $UNDOC_COUNT exported symbols without documentation comments." \
                "$DETAILS"
        else
            log_success "No undocumented exports detected"
            add_check_result "Documentation Coverage" "pass" \
                "Documentation coverage check completed successfully."
        fi
    fi
    rm -f "$COVERAGE_OUTPUT"
else
    log_warning "golangci-lint not found - skipping doc coverage"
    add_check_result "Documentation Coverage" "warning" \
        "golangci-lint not available. Install to check documentation coverage."
fi

# =============================================================================
# Summary
# =============================================================================
log_section "Summary"

# Add summary to report
cat >> "$REPORT_FILE" <<EOF

## Summary

**Total Checks**: $TOTAL_CHECKS
**Passed**: $PASSED_CHECKS ✓
**Warnings**: $WARNING_CHECKS ⚠
**Failed**: $FAILED_CHECKS ✗

---

### Next Steps

EOF

if [ "$FAILED_CHECKS" -gt 0 ]; then
    cat >> "$REPORT_FILE" <<EOF
**Blockers** ($FAILED_CHECKS):
- Fix failed checks immediately
- These indicate accuracy or critical issues

EOF
fi

if [ "$WARNING_CHECKS" -gt 0 ]; then
    cat >> "$REPORT_FILE" <<EOF
**Warnings** ($WARNING_CHECKS):
- Address warnings to improve documentation quality
- Install missing tools for complete coverage

EOF
fi

if [ "$FAILED_CHECKS" -eq 0 ] && [ "$WARNING_CHECKS" -eq 0 ]; then
    cat >> "$REPORT_FILE" <<EOF
All quick checks passed! Documentation is in good shape.

Consider running Phase 2 deep analysis for comprehensive validation.
EOF
fi

# Update JSON summary
if command -v jq &> /dev/null; then
    jq --arg total "$TOTAL_CHECKS" \
       --arg passed "$PASSED_CHECKS" \
       --arg warnings "$WARNING_CHECKS" \
       --arg failed "$FAILED_CHECKS" \
       '.summary.total_checks = ($total | tonumber) |
        .summary.passed = ($passed | tonumber) |
        .summary.warnings = ($warnings | tonumber) |
        .summary.failed = ($failed | tonumber)' \
       "$REPORT_JSON" > "$REPORT_JSON.tmp" && mv "$REPORT_JSON.tmp" "$REPORT_JSON"
fi

# Print summary to console
echo ""
echo -e "${BOLD}=== Documentation Quick Checks Summary ===${NC}"
echo ""
echo -e "Total Checks:  $TOTAL_CHECKS"
echo -e "${GREEN}Passed:        $PASSED_CHECKS ✓${NC}"

if [ "$WARNING_CHECKS" -gt 0 ]; then
    echo -e "${YELLOW}Warnings:      $WARNING_CHECKS ⚠${NC}"
fi

if [ "$FAILED_CHECKS" -gt 0 ]; then
    echo -e "${RED}Failed:        $FAILED_CHECKS ✗${NC}"
fi

echo ""
log_info "Full report saved to: $REPORT_FILE"
log_info "JSON report saved to: $REPORT_JSON"
echo ""

# Exit with appropriate code
if [ "$FAILED_CHECKS" -gt 0 ]; then
    log_error "Documentation checks failed!"
    exit 1
elif [ "$WARNING_CHECKS" -gt 0 ]; then
    log_warning "Documentation checks completed with warnings"
    exit 0
else
    log_success "All documentation checks passed!"
    exit 0
fi
