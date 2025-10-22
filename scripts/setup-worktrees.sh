#!/bin/bash
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Worktrees to create
WORKTREES=(
    "agent/auth"
    "agent/db"
    "agent/tests"
)

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo -e "${BLUE}=== Git Worktree Setup ===${NC}"
echo "Repository: $REPO_ROOT"
echo ""

# Function to check if worktree exists
worktree_exists() {
    local worktree_path="$1"
    git worktree list | grep -q "$worktree_path"
}

# Function to check if branch exists
branch_exists() {
    local branch_name="$1"
    git show-ref --verify --quiet "refs/heads/$branch_name"
}

# Function to create worktree
create_worktree() {
    local worktree_name="$1"
    local worktree_path="$REPO_ROOT/$worktree_name"
    local branch_name="$worktree_name"

    echo -e "${YELLOW}Processing worktree: ${worktree_name}${NC}"

    # Check if worktree already exists
    if worktree_exists "$worktree_path"; then
        echo -e "${GREEN}✓ Worktree already exists at: ${worktree_path}${NC}"
        return 0
    fi

    # Create directory if it doesn't exist
    mkdir -p "$(dirname "$worktree_path")"

    # Check if branch exists
    if branch_exists "$branch_name"; then
        echo -e "${YELLOW}  Branch '${branch_name}' already exists, using it${NC}"
        if git worktree add "$worktree_path" "$branch_name" 2>/dev/null; then
            echo -e "${GREEN}✓ Created worktree at: ${worktree_path}${NC}"
        else
            echo -e "${RED}✗ Failed to create worktree at: ${worktree_path}${NC}"
            return 1
        fi
    else
        # Create new branch and worktree
        if git worktree add -b "$branch_name" "$worktree_path" HEAD 2>/dev/null; then
            echo -e "${GREEN}✓ Created worktree at: ${worktree_path} with new branch '${branch_name}'${NC}"
        else
            echo -e "${RED}✗ Failed to create worktree at: ${worktree_path}${NC}"
            return 1
        fi
    fi

    echo ""
}

# Change to repository root
cd "$REPO_ROOT"

# Verify we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    echo -e "${RED}Error: Not in a git repository${NC}"
    exit 1
fi

# Create each worktree
failed=0
for worktree in "${WORKTREES[@]}"; do
    if ! create_worktree "$worktree"; then
        failed=$((failed + 1))
    fi
done

# Summary
echo -e "${BLUE}=== Summary ===${NC}"
git worktree list

if [ $failed -eq 0 ]; then
    echo -e "\n${GREEN}✓ All worktrees set up successfully!${NC}"
    exit 0
else
    echo -e "\n${RED}✗ ${failed} worktree(s) failed to set up${NC}"
    exit 1
fi
