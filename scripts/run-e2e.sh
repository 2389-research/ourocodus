#!/usr/bin/env bash
#
# run-e2e.sh - Run end-to-end integration tests
#
# This script runs the E2E test suite that validates the full system:
# PWA → Relay → Claude Code agents → back
#
# Prerequisites:
#   - ANTHROPIC_API_KEY environment variable set
#   - claude-code-acp installed and in PATH
#   - Go 1.23+ installed
#
# Usage:
#   ./scripts/run-e2e.sh [options]
#
# Options:
#   -v, --verbose    Enable verbose test output
#   -h, --help       Show this help message
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Default options
VERBOSE=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            grep '^#' "$0" | cut -c3-
            exit 0
            ;;
        *)
            echo -e "${RED}Error: Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Helper functions
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

# Check prerequisites
log_info "Checking prerequisites..."

# Check ANTHROPIC_API_KEY
if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
    log_error "ANTHROPIC_API_KEY environment variable is not set"
    echo ""
    echo "Please set your Anthropic API key:"
    echo "  export ANTHROPIC_API_KEY='your-api-key-here'"
    echo ""
    exit 1
fi
log_success "ANTHROPIC_API_KEY is set"

# Check Go installation
if ! command -v go &> /dev/null; then
    log_error "Go is not installed"
    echo ""
    echo "Please install Go 1.23 or later:"
    echo "  https://go.dev/doc/install"
    echo ""
    exit 1
fi
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
log_success "Go $GO_VERSION is installed"

# Check claude-code-acp (optional - test will skip if not available)
if ! command -v claude-code-acp &> /dev/null; then
    log_warning "claude-code-acp not found in PATH"
    log_warning "The test may fail if agents cannot be spawned"
else
    log_success "claude-code-acp is available"
fi

# Change to project root
cd "$PROJECT_ROOT"

# Setup worktrees if they don't exist
if [[ ! -d "agent" ]]; then
    log_info "Setting up worktrees..."
    if [[ -x "./scripts/setup-worktrees.sh" ]]; then
        ./scripts/setup-worktrees.sh || log_warning "Worktree setup had issues (may be okay)"
    else
        log_warning "Worktree setup script not found or not executable"
    fi
else
    log_info "Worktrees already exist at agent/"
fi

# Run the E2E tests
log_info "Running E2E tests..."
log_info "Test output will appear below:"
echo ""

# Build test flags - force output to not buffer
TEST_FLAGS="-v"
if [[ "$VERBOSE" == "true" ]]; then
    TEST_FLAGS="$TEST_FLAGS -test.v"
fi

# Run tests from project root with unbuffered output
# Using | cat forces line-buffered output instead of block-buffered
cd "$PROJECT_ROOT"
if go test $TEST_FLAGS -timeout 10m ./tests/e2e/... 2>&1 | cat; then
    echo ""
    log_success "E2E tests passed!"
    exit 0
else
    echo ""
    log_error "E2E tests failed"
    exit 1
fi
