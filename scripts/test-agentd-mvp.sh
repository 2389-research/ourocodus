#!/bin/bash
# agentd MVP End-to-End Test Script
# Tests full lifecycle: spawn → list → stop

set -e

# Ensure DOCKER_HOST is set for Colima
export DOCKER_HOST=${DOCKER_HOST:-unix:///Users/clint/.colima/default/docker.sock}

echo "=== agentd MVP Test Suite ==="
echo ""

# Clean any previous test state
echo "Cleaning previous state..."
bin/agentd stop test-alice test-bob 2>/dev/null || true
git worktree remove .agentd/worktrees/agent-test-alice --force 2>/dev/null || true
git worktree remove .agentd/worktrees/agent-test-bob --force 2>/dev/null || true
rm -rf .agentd 2>/dev/null || true
echo ""

# Test 1: Doctor
echo "[Test 1] Doctor check..."
bin/agentd doctor
echo ""

# Test 2: List with no agents
echo "[Test 2] List (should be empty)..."
OUTPUT=$(bin/agentd list)
if echo "$OUTPUT" | grep -q "No agents running"; then
    echo "✓ Correctly shows no agents"
else
    echo "✗ Expected 'No agents running'"
    exit 1
fi
echo ""

# Test 3: Spawn single agent
echo "[Test 3] Spawn single agent..."
bin/agentd spawn test-alice
if [ ! -d ".agentd/worktrees/agent-test-alice" ]; then
    echo "✗ Worktree not created"
    exit 1
fi
echo "✓ Agent test-alice spawned"
echo ""

# Test 4: List shows agent
echo "[Test 4] List (should show test-alice)..."
OUTPUT=$(bin/agentd list)
if echo "$OUTPUT" | grep -q "test-alice"; then
    echo "✓ test-alice appears in list"
else
    echo "✗ test-alice not found in list"
    echo "$OUTPUT"
    exit 1
fi
echo ""

# Test 5: Spawn second agent
echo "[Test 5] Spawn second agent..."
bin/agentd spawn test-bob
COUNT=$(bin/agentd list | grep -c "test-" || true)
if [ "$COUNT" -ge 2 ]; then
    echo "✓ Both agents running"
else
    echo "✗ Expected 2 agents, found $COUNT"
    bin/agentd list
    exit 1
fi
echo ""

# Test 6: Verify isolation
echo "[Test 6] Verify workspace isolation..."
if [ -d ".agentd/worktrees/agent-test-alice" ] && [ -d ".agentd/worktrees/agent-test-bob" ]; then
    echo "✓ Both worktrees exist"
else
    echo "✗ Worktrees missing"
    exit 1
fi

# Verify git worktrees
WORKTREE_COUNT=$(git worktree list | grep -c "agent-test" || true)
if [ "$WORKTREE_COUNT" -ge 2 ]; then
    echo "✓ Git worktrees registered"
else
    echo "✗ Git worktrees not found"
    git worktree list
    exit 1
fi
echo ""

# Test 7: Stop single agent
echo "[Test 7] Stop single agent..."
bin/agentd stop test-alice
if bin/agentd list | grep -q "test-alice"; then
    echo "✗ test-alice still appears in list"
    bin/agentd list
    exit 1
fi
echo "✓ test-alice stopped"
echo ""

# Test 8: Stop remaining agent
echo "[Test 8] Stop remaining agent..."
bin/agentd stop test-bob
OUTPUT=$(bin/agentd list | grep "test-" || echo "none")
if [ "$OUTPUT" != "none" ]; then
    echo "✗ Agents still running"
    bin/agentd list
    exit 1
fi
echo "✓ All test agents stopped"
echo ""

# Test 9: Verify cleanup
echo "[Test 9] Verify cleanup..."
if [ -d ".agentd/worktrees/agent-test-alice" ] || [ -d ".agentd/worktrees/agent-test-bob" ]; then
    echo "✗ Worktrees not cleaned up"
    exit 1
fi
echo "✓ Worktrees removed"
echo ""

# Final cleanup
rm -rf .agentd 2>/dev/null || true

echo "✅ All MVP tests passed!"
echo ""
echo "Summary:"
echo "  ✓ Doctor validates environment"
echo "  ✓ Spawn creates isolated agents"
echo "  ✓ List shows active agents"
echo "  ✓ Stop cleans up resources"
echo "  ✓ Workspace isolation verified"
echo ""
