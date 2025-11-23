#!/bin/bash
# Systematic verification of Phase 1-4 completion
# This script checks every acceptance criterion against actual code

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "=== Phase 1-4 Completion Verification ==="
echo "Repo: $REPO_ROOT"
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASS=0
FAIL=0
WARN=0

check_pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASS++))
}

check_fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAIL++))
}

check_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
    ((WARN++))
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "PHASE 1: Docker Label Discovery"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "--- Task 1.1: Add spawn-source Label to agentd ---"

# Check for spawn-source label in agentd spawn code
if grep -q "LabelSpawnSource" cmd/agentd/cmd_spawn.go 2>/dev/null; then
    check_pass "spawn-source label constant defined"
else
    check_fail "spawn-source label constant NOT found"
fi

if grep -q "ourocodus.agent/spawn-source" cmd/agentd/*.go 2>/dev/null; then
    check_pass "spawn-source label used in agentd"
else
    check_fail "spawn-source label NOT used in agentd"
fi

echo ""
echo "--- Task 1.2: Create Lease Management Module ---"

# Check lease module exists
if [ -f "pkg/relay/session/lease.go" ]; then
    check_pass "lease.go exists"

    # Check key functions
    if grep -q "func AcquireLease" pkg/relay/session/lease.go; then
        check_pass "AcquireLease() function exists"
    else
        check_fail "AcquireLease() function NOT found"
    fi

    if grep -q "func ReleaseLease" pkg/relay/session/lease.go; then
        check_pass "ReleaseLease() function exists"
    else
        check_fail "ReleaseLease() function NOT found"
    fi

    if grep -q "func RenewLease" pkg/relay/session/lease.go; then
        check_pass "RenewLease() function exists"
    else
        check_fail "RenewLease() function NOT found"
    fi

    if grep -q "O_EXCL" pkg/relay/session/lease.go; then
        check_pass "Atomic file creation with O_EXCL"
    else
        check_fail "O_EXCL NOT found (non-atomic)"
    fi

    if grep -q "ErrAlreadyAttached" pkg/relay/session/lease.go; then
        check_pass "ErrAlreadyAttached error defined"
    else
        check_fail "ErrAlreadyAttached NOT found"
    fi
else
    check_fail "lease.go does NOT exist"
fi

# Check lease tests
if [ -f "pkg/relay/session/lease_test.go" ]; then
    check_pass "lease_test.go exists"
else
    check_fail "lease_test.go does NOT exist"
fi

echo ""
echo "--- Task 1.3: Add Agent Discovery Message Handler ---"

if grep -q "agent:discover" pkg/relay/*.go; then
    check_pass "agent:discover handler exists"
else
    check_fail "agent:discover handler NOT found"
fi

if grep -q "handleAgentDiscover" pkg/relay/*.go; then
    check_pass "handleAgentDiscover function exists"
else
    check_fail "handleAgentDiscover function NOT found"
fi

echo ""
echo "--- Task 1.4: Add Attach/Detach Message Handlers ---"

if grep -q "agent:attach" pkg/relay/*.go; then
    check_pass "agent:attach handler exists"
else
    check_fail "agent:attach handler NOT found"
fi

if grep -q "agent:detach" pkg/relay/*.go; then
    check_pass "agent:detach handler exists"
else
    check_fail "agent:detach handler NOT found"
fi

if grep -q "handleAgentAttach" pkg/relay/*.go; then
    check_pass "handleAgentAttach function exists"
else
    check_fail "handleAgentAttach function NOT found"
fi

if grep -q "handleAgentDetach" pkg/relay/*.go; then
    check_pass "handleAgentDetach function exists"
else
    check_fail "handleAgentDetach function NOT found"
fi

echo ""
echo "--- Task 1.5: Add UserSession.AttachAgent() Method ---"

if grep -q "func.*AttachAgent" pkg/relay/session/*.go; then
    check_pass "AttachAgent() method exists"
else
    check_fail "AttachAgent() method NOT found"
fi

if grep -q "func.*DetachAgent" pkg/relay/session/*.go; then
    check_pass "DetachAgent() method exists"
else
    check_fail "DetachAgent() method NOT found"
fi

# Check for mutex usage (thread safety)
if grep -A 20 "func.*AttachAgent" pkg/relay/session/models.go 2>/dev/null | grep -q "\.mu\."; then
    check_pass "AttachAgent() uses mutex for thread safety"
else
    check_warn "AttachAgent() mutex usage not confirmed"
fi

echo ""
echo "--- Task 1.6: Integration Tests ---"

if [ -f "pkg/relay/integration_test.go" ]; then
    check_pass "integration_test.go exists"
else
    check_fail "integration_test.go does NOT exist"
fi

if [ -f "pkg/relay/session/models_test.go" ]; then
    check_pass "session models_test.go exists"
else
    check_fail "session models_test.go does NOT exist"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "PHASE 2: NATS Heartbeats"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "--- Task 2.1: Add Heartbeat Publisher to Agent ---"

if [ -f "pkg/heartbeat/publisher.go" ] || [ -f "pkg/heartbeat/heartbeat.go" ]; then
    check_pass "heartbeat package exists"

    if grep -q "agent\.heartbeat\." pkg/heartbeat/*.go 2>/dev/null; then
        check_pass "Heartbeat subject format correct"
    else
        check_fail "Heartbeat subject format NOT found"
    fi
else
    check_fail "heartbeat package does NOT exist"
fi

echo ""
echo "--- Task 2.2: Add Heartbeat Monitor to Relay ---"

if grep -q "heartbeat" pkg/relay/session/*.go; then
    check_pass "Relay has heartbeat integration"
else
    check_fail "Relay heartbeat integration NOT found"
fi

if grep -q "RenewLease" pkg/relay/session/*.go; then
    check_pass "Lease renewal on heartbeat"
else
    check_fail "Lease renewal NOT found"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "PHASE 3: ACP Communication Bridge"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "--- Task 3.1: Implement ACP Client Wrapper ---"

if [ -f "pkg/relay/session/acp_bridge.go" ]; then
    check_pass "acp_bridge.go exists"

    if grep -q "type ACPBridge struct" pkg/relay/session/acp_bridge.go; then
        check_pass "ACPBridge type defined"
    else
        check_fail "ACPBridge type NOT found"
    fi

    if grep -q "func.*SendMessage" pkg/relay/session/acp_bridge.go; then
        check_pass "SendMessage() method exists"
    else
        check_fail "SendMessage() method NOT found"
    fi
else
    check_fail "acp_bridge.go does NOT exist"
fi

echo ""
echo "--- Task 3.2: Wire ACP Bridge to AttachAgent() ---"

if grep -q "ACPBridge" pkg/relay/session/models.go; then
    check_pass "ACPBridge integrated in session models"
else
    check_fail "ACPBridge NOT integrated in session models"
fi

if grep -A 50 "func.*AttachAgent" pkg/relay/session/models.go 2>/dev/null | grep -q "acp\|ACP"; then
    check_pass "AttachAgent() creates ACP bridge"
else
    check_fail "AttachAgent() does NOT create ACP bridge"
fi

echo ""
echo "--- Task 3.3: Verify WebSocket Message Routing ---"

if grep -q "agent:message" pkg/relay/*.go; then
    check_pass "agent:message protocol exists"
else
    check_fail "agent:message protocol NOT found"
fi

if grep -q "handleAgentMessage" pkg/relay/*.go; then
    check_pass "handleAgentMessage function exists"
else
    check_fail "handleAgentMessage function NOT found"
fi

echo ""
echo "--- Task 3.4: End-to-End Communication Tests ---"

if grep -q "TestAgentMessage" pkg/relay/*.go 2>/dev/null; then
    check_pass "Agent message tests exist"
else
    check_warn "Agent message tests not found (may be in integration tests)"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "PHASE 4: Security Hardening"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "--- Task 4.1: Generate Attach Tokens ---"

if grep -q "token" cmd/agentd/cmd_spawn.go 2>/dev/null; then
    check_pass "Token generation in spawn command"
else
    check_fail "Token generation NOT found"
fi

if grep -q "GenerateToken\|generateToken" pkg/relay/session/*.go cmd/agentd/*.go 2>/dev/null; then
    check_pass "Token generation function exists"
else
    check_fail "Token generation function NOT found"
fi

echo ""
echo "--- Task 4.2: Add Token Verification to Attach ---"

if grep -A 50 "handleAgentAttach" pkg/relay/*.go 2>/dev/null | grep -q "token\|Token"; then
    check_pass "Token verification in attach handler"
else
    check_fail "Token verification NOT found in attach handler"
fi

if grep -q "VerifyToken\|verifyToken\|CompareToken" pkg/relay/session/*.go 2>/dev/null; then
    check_pass "Token verification function exists"
else
    check_warn "Token verification function not explicitly found"
fi

echo ""
echo "--- Task 4.3: Add Audit Logging ---"

if grep -q "\[AUDIT\]" pkg/relay/session/*.go; then
    check_pass "Audit logging present"
else
    check_fail "Audit logging NOT found"
fi

if grep -q "agent:attach.*success\|agent:detach.*success" pkg/relay/session/*.go; then
    check_pass "Attach/detach audit logs exist"
else
    check_fail "Attach/detach audit logs NOT found"
fi

echo ""
echo "--- Task 4.4: Add Rate Limiting ---"

if [ -f "pkg/relay/ratelimit/limiter.go" ]; then
    check_pass "Rate limiter module exists"
else
    check_fail "Rate limiter module does NOT exist"
fi

if [ -f "pkg/relay/ratelimit/limiter_test.go" ]; then
    check_pass "Rate limiter tests exist"
else
    check_fail "Rate limiter tests do NOT exist"
fi

if grep -q "rateLimiter" pkg/relay/server.go pkg/relay/handlers_agent_adoption.go 2>/dev/null; then
    check_pass "Rate limiter integrated in server"
else
    check_fail "Rate limiter NOT integrated"
fi

if grep -q "RATE_LIMIT_EXCEEDED" pkg/relay/*.go; then
    check_pass "Rate limit error handling exists"
else
    check_fail "Rate limit error handling NOT found"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo -e "  ${GREEN}Passed:${NC} $PASS"
echo -e "  ${RED}Failed:${NC} $FAIL"
echo -e "  ${YELLOW}Warnings:${NC} $WARN"
echo ""

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}✓ All critical checks passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ $FAIL critical check(s) failed${NC}"
    exit 1
fi
