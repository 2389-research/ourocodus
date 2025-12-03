#!/bin/bash
set -e

# Cleanup PID file on exit (only if it belongs to this process)
cleanup() {
    if [ -f /tmp/claude-code.pid ]; then
        if [ "$(cat /tmp/claude-code.pid 2>/dev/null)" = "$$" ]; then
            rm -f /tmp/claude-code.pid
        fi
    fi
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
    echo "[claude-code] Loading credentials from .creds/.env" >&2
    # SECURITY: Safe .env loading without shell execution
    # Only export allowed keys (ANTHROPIC_API_KEY), ignore all others
    # This prevents command injection via malicious .env files
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in
            \#*|"") continue ;;  # skip comments and blank lines
            ANTHROPIC_API_KEY=*)
                # Extract value after '=' and export
                # shellcheck disable=SC2163  # We're exporting the variable contained in $line
                export "${line}"
                ;;
            *) continue ;;       # ignore any unexpected keys
        esac
    done < /home/node/.creds/.env
    # Verify that ANTHROPIC_API_KEY was actually set from the file
    if [ -z "$ANTHROPIC_API_KEY" ]; then
        echo "[claude-code] ERROR: .creds/.env exists but contains no ANTHROPIC_API_KEY" >&2
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

# Write PID file for health check (before exec replaces this process)
# The node process will inherit this PID
echo $$ > /tmp/claude-code.pid

# Start claude-code-acp
# stdin/stdout are used for ACP protocol communication
exec claude-code-acp --workspace /workspace "$@"
