#!/bin/bash
set -e

# Cleanup PID file on exit
cleanup() {
    rm -f /tmp/claude-code.pid
}
trap cleanup EXIT

# Credential sourcing with fallback
# Priority: 1) .creds/.env file  2) ~/.claude directory  3) Error
#
# SECURITY NOTE: Sourcing .env exposes API key to process environment.
# This is visible via /proc/$pid/environ to anyone with container exec access.
# Mitigations: read-only rootfs, drop capabilities, no-new-privileges.

if [ -f "/home/node/.creds/.env" ]; then
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
    echo "[claude-code] Provide ANTHROPIC_API_KEY via .creds/.env or mount ~/.claude" >&2
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
