#!/bin/bash
# Documentation Audit - Simple Inventory Collection
# Simplified version for reliability

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="$(dirname "$SCRIPT_DIR")"
REPO_ROOT="$(cd "$SKILL_DIR/../../.." && pwd)"
REPORTS_DIR="$SKILL_DIR/reports"

echo "Collecting documentation inventory..."
echo "Repository: $REPO_ROOT"

mkdir -p "$REPORTS_DIR"

cd "$REPO_ROOT"

# Count documentation files
DOC_COUNT=$(find . -type f \( -name "*.md" -o -name "doc.go" \) \
  ! -path "*/vendor/*" \
  ! -path "*/node_modules/*" \
  ! -path "*/.git/*" \
  ! -path "*/build/*" \
  ! -path "*/dist/*" | wc -l | tr -d ' ')

# Count Go packages
PKG_COUNT=0
if command -v go &> /dev/null; then
    PKG_COUNT=$(go list ./... 2>/dev/null | wc -l | tr -d ' ' || echo "0")
fi

# Basic summary
cat > "$REPORTS_DIR/inventory.json" <<EOF
{
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "repo_root": "$REPO_ROOT",
  "documentation": {
    "total_count": $DOC_COUNT
  },
  "code": {
    "total_packages": $PKG_COUNT
  }
}
EOF

echo "Documentation files: $DOC_COUNT"
echo "Go packages: $PKG_COUNT"
echo ""
echo "Inventory saved to: $REPORTS_DIR/inventory.json"
