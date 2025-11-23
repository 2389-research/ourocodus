#!/bin/bash
# Phase 4 Security Hardening Demo
# Demonstrates token authentication, audit logging, and rate limiting

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Demo configuration
AGENT_ID="demo-secure-agent"
RELAY_URL="http://localhost:8080"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║     Phase 4 Security Hardening Demo                         ║"
echo "║     - Token Authentication                                   ║"
echo "║     - Audit Logging                                          ║"
echo "║     - Rate Limiting                                          ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up...${NC}"
    bin/agentd stop "$AGENT_ID" 2>/dev/null || true
    rm -f ".agentd/session/$AGENT_ID.token" 2>/dev/null || true
}
trap cleanup EXIT

echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}Step 1: Spawn Agent with Token Generation${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo "Running: bin/agentd spawn $AGENT_ID"
echo ""

# Spawn agent and capture output
SPAWN_OUTPUT=$(bin/agentd spawn "$AGENT_ID" 2>&1 || true)
echo "$SPAWN_OUTPUT"

# Extract token from spawn output
TOKEN=$(echo "$SPAWN_OUTPUT" | grep -A1 "🔐 Attach Token" | tail -1 | tr -d ' ')

if [ -z "$TOKEN" ]; then
    echo -e "${RED}✗ Failed to extract token from spawn output${NC}"
    echo "Spawn output was:"
    echo "$SPAWN_OUTPUT"
    exit 1
fi

echo ""
echo -e "${GREEN}✓ Agent spawned successfully${NC}"
echo -e "${GREEN}✓ Token generated: ${TOKEN:0:20}...${NC}"
echo ""
read -p "Press Enter to continue..."

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}Step 2: Verify Token File Created${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""

TOKEN_FILE=".agentd/session/$AGENT_ID.token"
if [ -f "$TOKEN_FILE" ]; then
    echo -e "${GREEN}✓ Token file exists: $TOKEN_FILE${NC}"
    echo "  Permissions: $(stat -f "%Sp" "$TOKEN_FILE" 2>/dev/null || stat -c "%a" "$TOKEN_FILE" 2>/dev/null)"
    echo "  Size: $(wc -c < "$TOKEN_FILE" | tr -d ' ') bytes (256-bit base64-encoded)"
else
    echo -e "${RED}✗ Token file not found: $TOKEN_FILE${NC}"
    exit 1
fi

echo ""
read -p "Press Enter to continue..."

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}Step 3: Test Authentication (Manual Web Browser Test)${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo "The relay server requires this token for agent attachment."
echo ""
echo -e "${BLUE}📋 Instructions:${NC}"
echo "  1. Start the relay: bin/relay"
echo "  2. Open browser: $RELAY_URL"
echo "  3. Try to attach to agent: $AGENT_ID"
echo ""
echo -e "${GREEN}✓ With valid token:${NC}"
echo "     Token: $TOKEN"
echo "     Expected: Attachment succeeds"
echo ""
echo -e "${RED}✗ With invalid token:${NC}"
echo "     Token: invalid-token-12345"
echo "     Expected: INVALID_TOKEN error"
echo ""
echo -e "${YELLOW}⚠ Without token:${NC}"
echo "     Token: (empty)"
echo "     Expected: MISSING_TOKEN error"
echo ""
read -p "Press Enter after testing in browser..."

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}Step 4: View Audit Logs${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo "Audit logs are written to the relay server output in structured JSON."
echo "Look for lines containing [AUDIT] in the relay server output."
echo ""
echo -e "${BLUE}Example audit log entries:${NC}"
echo ""
cat << 'EOF'
# Successful attachment:
[AUDIT] {"timestamp":"2025-11-23T02:00:00Z","type":"agent:attach",
         "userId":"user-session-123","agentId":"demo-secure-agent",
         "success":true}

# Failed attachment (invalid token):
[AUDIT] {"timestamp":"2025-11-23T02:00:01Z","type":"agent:attach",
         "userId":"user-session-456","agentId":"demo-secure-agent",
         "success":false,"error":"invalid attach token"}

# Detachment:
[AUDIT] {"timestamp":"2025-11-23T02:00:10Z","type":"agent:detach",
         "userId":"user-session-123","agentId":"demo-secure-agent",
         "success":true}
EOF
echo ""
echo -e "${GREEN}Tip: Pipe relay output through 'jq' to format JSON:${NC}"
echo "  bin/relay 2>&1 | grep AUDIT | jq ."
echo ""
read -p "Press Enter to continue..."

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}Step 5: Rate Limiting Demonstration${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo "The relay enforces rate limiting to prevent brute-force attacks:"
echo "  • Burst capacity: 10 requests"
echo "  • Refill rate: 1 request/second"
echo ""
echo -e "${BLUE}To test rate limiting:${NC}"
echo "  1. Make sure relay is running"
echo "  2. In browser DevTools, run this JavaScript:"
echo ""
cat << 'EOF'
// Rapid attachment attempts (will hit rate limit)
for (let i = 0; i < 15; i++) {
  fetch('/api/attach', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({
      type: 'agent:attach',
      agentId: 'demo-secure-agent',
      userSessionId: 'user-' + Date.now(),
      token: 'invalid-token-' + i
    })
  }).then(r => r.json())
    .then(d => console.log(`Attempt ${i+1}:`, d.code || 'success'));
}
EOF
echo ""
echo -e "${YELLOW}Expected result:${NC}"
echo "  • First ~10 attempts: INVALID_TOKEN errors (authentication fails)"
echo "  • Next 5 attempts: RATE_LIMIT_EXCEEDED errors"
echo ""
echo -e "${GREEN}This prevents attackers from brute-forcing tokens!${NC}"
echo ""
read -p "Press Enter to continue..."

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}Step 6: Security Properties Summary${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${GREEN}✓ Token Authentication:${NC}"
echo "  • 256-bit cryptographic tokens (32 bytes)"
echo "  • Generated using crypto/rand (cryptographically secure)"
echo "  • Base64-encoded for safe transmission"
echo "  • Stored with 0600 permissions"
echo "  • Constant-time comparison prevents timing attacks"
echo ""
echo -e "${GREEN}✓ Audit Logging:${NC}"
echo "  • All attach/detach operations logged"
echo "  • Authentication failures logged with reason"
echo "  • Structured JSON format for analysis"
echo "  • Includes timestamps, user IDs, agent IDs, outcomes"
echo ""
echo -e "${GREEN}✓ Rate Limiting:${NC}"
echo "  • Token bucket algorithm per user session"
echo "  • Prevents brute-force token guessing"
echo "  • Protects against DoS attacks"
echo "  • Thread-safe for concurrent users"
echo ""

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}Demo Complete!${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${GREEN}Phase 4 Security Features Demonstrated:${NC}"
echo "  ✓ Token generation and secure storage"
echo "  ✓ Token-based authentication enforcement"
echo "  ✓ Audit logging of security events"
echo "  ✓ Rate limiting to prevent attacks"
echo ""
echo -e "${BLUE}Next Steps:${NC}"
echo "  • Review audit logs in relay output"
echo "  • Test with multiple concurrent users"
echo "  • Monitor rate limit behavior under load"
echo ""
