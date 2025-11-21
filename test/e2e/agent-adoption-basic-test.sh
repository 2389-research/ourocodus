#!/usr/bin/env bash
# Basic end-to-end test for Phase 1 Agent Adoption
# Tests: spawn agent and verify Docker labels
# This script has minimal dependencies (only requires Docker and Go)

set -e  # Exit on error
set -o pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
AGENT_ID="basic-e2e-test-agent-$(date +%s)"
TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$TEST_DIR/../.." && pwd)"

# Test state
CONTAINER_ID=""

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"

    # Stop agent if running
    if [ -n "$AGENT_ID" ]; then
        echo "Stopping agent: $AGENT_ID"
        "$ROOT_DIR/agentd" stop "$AGENT_ID" 2>/dev/null || true
    fi

    # Remove any leftover containers
    docker ps -a --filter "label=agent-id=$AGENT_ID" -q | xargs -r docker rm -f 2>/dev/null || true

    echo -e "${GREEN}Cleanup complete${NC}"
}

trap cleanup EXIT

# Helper functions
log_step() {
    echo -e "\n${BLUE}==>${NC} ${1}"
}

log_success() {
    echo -e "${GREEN}✓${NC} ${1}"
}

log_error() {
    echo -e "${RED}✗${NC} ${1}"
}

# Build binaries
build_binaries() {
    log_step "Building agentd binary..."
    cd "$ROOT_DIR"
    go build -o agentd ./cmd/agentd
    log_success "Binary built"
}

# Test 1: Spawn agent with spawn-source label
test_spawn_with_label() {
    log_step "Test 1: Spawn agent with spawn-source label"

    # Check if agent already exists
    if docker ps -a --filter "label=agent-id=$AGENT_ID" -q | grep -q .; then
        log_error "Agent $AGENT_ID already exists. Cleaning up..."
        docker ps -a --filter "label=agent-id=$AGENT_ID" -q | xargs docker rm -f
    fi

    # Spawn agent
    if ! ANTHROPIC_API_KEY="test-key-basic" "$ROOT_DIR/agentd" spawn "$AGENT_ID" --image "ourocodus/agent:latest"; then
        log_error "Failed to spawn agent"
        return 1
    fi

    log_success "Agent spawned successfully"
    return 0
}

# Test 2: Verify container and labels
test_verify_container() {
    log_step "Test 2: Verify container exists with correct labels"

    # Get container ID
    CONTAINER_ID=$(docker ps --filter "label=agent-id=$AGENT_ID" --format "{{.ID}}")
    if [ -z "$CONTAINER_ID" ]; then
        log_error "Agent container not found"
        return 1
    fi
    log_success "Container found: $CONTAINER_ID"

    # Verify ourocodus.agent label
    AGENT_LABEL=$(docker inspect "$CONTAINER_ID" --format '{{index .Config.Labels "ourocodus.agent"}}')
    if [ "$AGENT_LABEL" != "true" ]; then
        log_error "Expected ourocodus.agent=true, got: $AGENT_LABEL"
        return 1
    fi
    log_success "Label ourocodus.agent=true verified"

    # Verify agent-id label
    AGENT_ID_LABEL=$(docker inspect "$CONTAINER_ID" --format '{{index .Config.Labels "agent-id"}}')
    if [ "$AGENT_ID_LABEL" != "$AGENT_ID" ]; then
        log_error "Expected agent-id=$AGENT_ID, got: $AGENT_ID_LABEL"
        return 1
    fi
    log_success "Label agent-id=$AGENT_ID verified"

    # Verify spawn-source label (NEW in Phase 1)
    SPAWN_SOURCE=$(docker inspect "$CONTAINER_ID" --format '{{index .Config.Labels "ourocodus.agent/spawn-source"}}')
    if [ "$SPAWN_SOURCE" != "cli" ]; then
        log_error "Expected spawn-source=cli, got: $SPAWN_SOURCE"
        return 1
    fi
    log_success "Label ourocodus.agent/spawn-source=cli verified ⭐"

    return 0
}

# Test 3: Verify workspace mount
test_verify_workspace() {
    log_step "Test 3: Verify workspace mount"

    # Check workspace mount
    WORKSPACE_MOUNT=$(docker inspect "$CONTAINER_ID" --format '{{range .Mounts}}{{if eq .Destination "/workspace"}}{{.Source}}{{end}}{{end}}')
    if [ -z "$WORKSPACE_MOUNT" ]; then
        log_error "Workspace mount not found"
        return 1
    fi

    # Verify workspace directory exists
    if [ ! -d "$WORKSPACE_MOUNT" ]; then
        log_error "Workspace directory does not exist: $WORKSPACE_MOUNT"
        return 1
    fi

    log_success "Workspace mount verified: $WORKSPACE_MOUNT"
    return 0
}

# Test 4: Verify credentials mount
test_verify_credentials() {
    log_step "Test 4: Verify credentials mount"

    # Check for .creds mount
    CREDS_MOUNT=$(docker inspect "$CONTAINER_ID" --format '{{range .Mounts}}{{if eq .Destination "/root/.creds"}}{{.Source}}{{end}}{{end}}')

    if [ -z "$CREDS_MOUNT" ]; then
        log_success "No .creds mount (expected for basic spawn)"
    else
        # Verify it's read-only
        IS_READONLY=$(docker inspect "$CONTAINER_ID" --format '{{range .Mounts}}{{if eq .Destination "/root/.creds"}}{{.RW}}{{end}}{{end}}')
        if [ "$IS_READONLY" = "false" ]; then
            log_success ".creds mount is read-only: $CREDS_MOUNT"
        else
            log_error ".creds mount should be read-only"
            return 1
        fi
    fi

    return 0
}

# Test 5: Test stop command
test_stop_agent() {
    log_step "Test 5: Stop agent and verify cleanup"

    # Stop agent
    if ! "$ROOT_DIR/agentd" stop "$AGENT_ID"; then
        log_error "Failed to stop agent"
        return 1
    fi

    # Verify container is removed
    sleep 1
    REMAINING=$(docker ps -a --filter "label=agent-id=$AGENT_ID" -q)
    if [ -n "$REMAINING" ]; then
        log_error "Container still exists after stop"
        return 1
    fi

    log_success "Agent stopped and cleaned up"
    return 0
}

# Test 6: List command (spawn new agent first)
test_list_command() {
    log_step "Test 6: Test list command"

    # Spawn a fresh agent for list test
    LIST_TEST_AGENT="list-test-$(date +%s)"
    if ! ANTHROPIC_API_KEY="test-key-list" "$ROOT_DIR/agentd" spawn "$LIST_TEST_AGENT"; then
        log_error "Failed to spawn agent for list test"
        return 1
    fi

    # Run list command
    LIST_OUTPUT=$("$ROOT_DIR/agentd" list)

    # Verify our agent is in the list
    if ! echo "$LIST_OUTPUT" | grep -q "$LIST_TEST_AGENT"; then
        log_error "Agent not found in list output"
        echo "List output:"
        echo "$LIST_OUTPUT"
        # Cleanup
        "$ROOT_DIR/agentd" stop "$LIST_TEST_AGENT" || true
        return 1
    fi

    # Verify spawn-source is shown
    if ! echo "$LIST_OUTPUT" | grep -q "cli"; then
        log_error "spawn-source not shown in list output"
        echo "List output:"
        echo "$LIST_OUTPUT"
        # Cleanup
        "$ROOT_DIR/agentd" stop "$LIST_TEST_AGENT" || true
        return 1
    fi

    log_success "List command works correctly"

    # Cleanup
    "$ROOT_DIR/agentd" stop "$LIST_TEST_AGENT" || true

    return 0
}

# Setup Docker socket for Colima
setup_docker_socket() {
    # Check if Colima is running
    if command -v colima &> /dev/null && colima status &> /dev/null; then
        # Colima uses a different socket location
        export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"
        log_step "Detected Colima, using socket: $DOCKER_HOST"
    fi
}

# Main test execution
main() {
    echo -e "${BLUE}╔════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  Phase 1 Agent Adoption - Basic Integration Test  ║${NC}"
    echo -e "${BLUE}╚════════════════════════════════════════════════════╝${NC}"

    # Check Docker is available
    if ! command -v docker &> /dev/null; then
        log_error "Docker not found. Please install Docker."
        exit 1
    fi

    # Setup Docker socket (Colima support)
    setup_docker_socket

    # Build
    build_binaries

    # Run tests
    FAILED=0

    test_spawn_with_label || FAILED=$((FAILED + 1))
    test_verify_container || FAILED=$((FAILED + 1))
    test_verify_workspace || FAILED=$((FAILED + 1))
    test_verify_credentials || FAILED=$((FAILED + 1))
    test_stop_agent || FAILED=$((FAILED + 1))
    test_list_command || FAILED=$((FAILED + 1))

    # Summary
    echo -e "\n${BLUE}════════════════════════════════════════════════════${NC}"
    if [ $FAILED -eq 0 ]; then
        echo -e "${GREEN}✓ All basic tests passed!${NC}"
        echo -e "\n${YELLOW}Note:${NC} For full WebSocket attach/detach testing, run:"
        echo -e "  ${BLUE}./test/e2e/agent-adoption-test.sh${NC}"
        return 0
    else
        echo -e "${RED}✗ $FAILED test(s) failed${NC}"
        return 1
    fi
}

# Run main
main
exit $?
