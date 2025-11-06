#!/usr/bin/env bash
set -euo pipefail

RED=$'\033[31m'
GREEN=$'\033[32m'
YELLOW=$'\033[33m'
CYAN=$'\033[36m'
RESET=$'\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

say()  { printf "%s%s%s %s\n" "${CYAN}" "$1" "${RESET}" "$2"; }
warn() { printf "%s%s%s %s\n" "${YELLOW}" "$1" "${RESET}" "$2"; }
die()  { printf "%s%s%s %s\n" "${RED}" "$1" "${RESET}" "$2"; exit 1; }
yay()  { printf "%s%s%s %s\n" "${GREEN}" "$1" "${RESET}" "$2"; }

# Detect if mise is available
if command -v mise &> /dev/null; then
  MISE_EXEC="mise exec --"
else
  MISE_EXEC=""
fi

usage() {
  cat <<EOF
Usage: $0 [TEST_TYPE] [OPTIONS]

TEST_TYPE:
  relay      Run WebSocket relay integration tests (requires relay binary)
  session    Run session management layer tests (unit tests)
  all        Run all smoke tests (default)

OPTIONS:
  -v, --verbose       Enable verbose output
  --fuzz COUNT        Run fuzz testing with COUNT iterations (relay tests only)

Examples:
  $0 relay              # Run relay tests only
  $0 session            # Run session tests only
  $0 all                # Run all tests
  $0 relay --verbose    # Run relay tests with verbose output
  $0 relay --fuzz 1000  # Run relay with 1000 fuzz iterations
  $0 all --fuzz 100000  # Run all tests with 100k fuzz iterations
EOF
  exit 0
}

run_relay_test() {
  local verbose_flag=""
  local fuzz_flag=""
  [[ "$VERBOSE" == "true" ]] && verbose_flag="-verbose"
  [[ -n "$FUZZ_COUNT" ]] && fuzz_flag="-fuzz $FUZZ_COUNT"

  say "🚀" "Running WebSocket relay integration test..."

  if [[ ! -x "${REPO_ROOT}/bin/relay" ]]; then
    warn "🔧" "Relay binary missing. Building it..."
    (cd "${REPO_ROOT}" && ${MISE_EXEC} make build) || die "💥" "Build failed"
  fi

  if ${MISE_EXEC} go run "${REPO_ROOT}/examples/smoke-tests/relay" $verbose_flag $fuzz_flag; then
    yay "✅" "Relay integration test passed"
    return 0
  else
    die "❌" "Relay integration test failed"
  fi
}

run_session_test() {
  local verbose_flag=""
  [[ "$VERBOSE" == "true" ]] && verbose_flag="-verbose"

  say "🧪" "Running session management layer test..."

  if ${MISE_EXEC} go run "${REPO_ROOT}/examples/smoke-tests/session" $verbose_flag; then
    yay "✅" "Session management test passed"
    return 0
  else
    die "❌" "Session management test failed"
  fi
}

# Parse arguments
TEST_TYPE="${1:-all}"
VERBOSE="false"
FUZZ_COUNT=""

# Handle help
if [[ "$TEST_TYPE" == "-h" ]] || [[ "$TEST_TYPE" == "--help" ]]; then
  usage
fi

# Parse flags
shift_count=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    -v|--verbose)
      VERBOSE="true"
      shift
      ;;
    --fuzz)
      if [[ -n "$2" ]] && [[ "$2" =~ ^[0-9]+$ ]]; then
        FUZZ_COUNT="$2"
        shift 2
      else
        echo "Error: --fuzz requires a numeric argument"
        exit 1
      fi
      ;;
    *)
      # First non-flag argument is test type
      if [[ $shift_count -eq 0 ]]; then
        TEST_TYPE="$1"
      fi
      shift
      shift_count=$((shift_count + 1))
      ;;
  esac
done

# Run tests
case "$TEST_TYPE" in
  relay)
    run_relay_test
    ;;
  session)
    run_session_test
    ;;
  all)
    say "🧪" "Running all smoke tests..."
    run_session_test
    echo ""
    run_relay_test
    echo ""
    yay "🎉" "All smoke tests passed!"
    ;;
  *)
    echo "Unknown test type: $TEST_TYPE"
    usage
    ;;
esac
