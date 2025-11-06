#!/bin/bash
# Verify no hard-coded references to old script locations
# This script helps ensure migration is complete

set -e

echo "🔍 Verifying no hard-coded script paths..."

FOUND_ISSUES=0

# Check for references to old script locations
# Exclude git directory, docs (which may reference old paths), and this script itself

if grep -r "scripts/demo" --exclude-dir=".git" --exclude-dir="docs" --exclude="verify-paths.sh" . 2>/dev/null; then
    echo "❌ Found references to scripts/demo"
    FOUND_ISSUES=1
fi

if grep -r "scripts/interactive" --exclude-dir=".git" --exclude-dir="docs" --exclude="verify-paths.sh" . 2>/dev/null; then
    echo "❌ Found references to scripts/interactive"
    FOUND_ISSUES=1
fi

if grep -r "scripts/demo-performance" --exclude-dir=".git" --exclude-dir="docs" --exclude="verify-paths.sh" . 2>/dev/null; then
    echo "❌ Found references to scripts/demo-performance"
    FOUND_ISSUES=1
fi

if grep -r "scripts/smoketest" --exclude-dir=".git" --exclude-dir="docs" --exclude="verify-paths.sh" . 2>/dev/null; then
    echo "❌ Found references to scripts/smoketest"
    FOUND_ISSUES=1
fi

if grep -r "scripts/container-race" --exclude-dir=".git" --exclude-dir="docs" --exclude="verify-paths.sh" . 2>/dev/null; then
    echo "❌ Found references to scripts/container-race"
    FOUND_ISSUES=1
fi

if [ $FOUND_ISSUES -eq 0 ]; then
    echo "✅ No hard-coded script paths found"
    exit 0
else
    echo "❌ Found hard-coded paths to old script locations"
    echo "Please update these references to point to examples/ directory"
    exit 1
fi
