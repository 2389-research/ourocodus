#!/usr/bin/env bash
set -euo pipefail

RED=$'\033[31m'
GREEN=$'\033[32m'
YELLOW=$'\033[33m'
CYAN=$'\033[36m'
BOLD=$'\033[1m'
RESET=$'\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

say()  { printf "%s%s%s %s\n" "${CYAN}" "$1" "${RESET}" "$2"; }
warn() { printf "%s%s%s %s\n" "${YELLOW}" "$1" "${RESET}" "$2"; }
die()  { printf "%s%s%s %s\n" "${RED}" "$1" "${RESET}" "$2"; exit 1; }
yay()  { printf "%s%s%s %s\n" "${GREEN}" "$1" "${RESET}" "$2"; }

say "🧪" "Initiating smoke test. Because trust, apparently, must be earned."

if [[ ! -x "${REPO_ROOT}/bin/relay" ]]; then
  warn "🔧" "Relay binary missing. Fine. Building it for you…"
  (cd "${REPO_ROOT}" && mise exec -- make build) || die "💥" "Build exploded. Fix that then come back."
fi

say "🚀" "Launching the dramatic production known as 'relay smoke test'."
if mise exec -- go run ./scripts/smoketest; then
  yay "🎉" "Relay is alive, responsive, and only mildly sarcastic."
else
  die "😵" "Smoke test reported doom. Scroll up for the gory details."
fi
