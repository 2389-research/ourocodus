#!/bin/bash
set -e

# Cleanup PID file on exit
cleanup() {
    rm -f /tmp/claude-code.pid
}
trap cleanup EXIT

# Credential sourcing with fallback
# Priority:
#   1) ANTHROPIC_API_KEY already in environment (passed via -e flag)
#   2) .creds/.env file (mounted volume)
#   3) ~/.claude directory (OAuth credentials)
#   4) Error
#
# SECURITY NOTE: API key in environment is visible via /proc/$pid/environ
# to anyone with container exec access.
# Mitigations: read-only rootfs, drop capabilities, no-new-privileges.

if [ -n "$ANTHROPIC_API_KEY" ]; then
    echo "[claude-code] Using ANTHROPIC_API_KEY from environment" >&2
elif [ -f "/home/node/.creds/.env" ]; then
    echo "[claude-code] Sourcing credentials from .creds/.env" >&2
    # Validate .env format before sourcing (basic safety check)
    if grep -qE '^[A-Z_][A-Z0-9_]*=' /home/node/.creds/.env 2>/dev/null; then
        set -a
        source /home/node/.creds/.env
        set +a
    else
        echo "[claude-code] ERROR: Invalid .env format" >&2
        exit 1
    fi
elif [ -f "/home/node/.claude/.credentials.json" ]; then
    echo "[claude-code] Using existing Claude credentials from ~/.claude" >&2
    # claude-code-acp will read from standard location
else
    echo "[claude-code] ERROR: No credentials found" >&2
    echo "[claude-code] Provide credentials via one of:" >&2
    echo "[claude-code]   - ANTHROPIC_API_KEY environment variable" >&2
    echo "[claude-code]   - .creds/.env file mounted at /home/node/.creds" >&2
    echo "[claude-code]   - ~/.claude directory with OAuth credentials" >&2
    exit 1
fi

# Verify API key is available (unless using OAuth from ~/.claude)
if [ -z "$ANTHROPIC_API_KEY" ] && [ ! -f "/home/node/.claude/.credentials.json" ]; then
    echo "[claude-code] ERROR: ANTHROPIC_API_KEY not set and no Claude credentials found" >&2
    exit 1
fi

# Write PID file for health check (before exec replaces this process)
# The node process will inherit this PID
echo $$ > /tmp/claude-code.pid

# Start claude-code-acp
# stdin/stdout are used for ACP protocol communication
exec claude-code-acp --workspace /workspace "$@"
