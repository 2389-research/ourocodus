#!/bin/bash
# Health check for claude-code-acp container
# Verifies: 1) PID file exists, 2) Process running, 3) Process is claude-code-acp

PID_FILE="/tmp/claude-code.pid"

# Check PID file exists
if [ ! -f "$PID_FILE" ]; then
    echo "PID file not found" >&2
    exit 1
fi

PID=$(cat "$PID_FILE")

# Validate PID is a number
if ! [[ "$PID" =~ ^[0-9]+$ ]]; then
    echo "Invalid PID in file" >&2
    exit 1
fi

# Check process is running
if ! kill -0 "$PID" 2>/dev/null; then
    echo "Process $PID not running" >&2
    exit 1
fi

# Verify process is claude-code-acp
# Primary check: verify executable path via /proc/$PID/exe symlink
# Fallback: check cmdline if exe is not readable (some environments)
VERIFIED=false

if [ -L "/proc/$PID/exe" ]; then
    EXE_PATH=$(readlink -f "/proc/$PID/exe" 2>/dev/null)
    if [ -n "$EXE_PATH" ] && [[ "$EXE_PATH" == *"/claude-code-acp" ]]; then
        VERIFIED=true
    fi
fi

# Fallback to cmdline check if exe verification didn't succeed
if [ "$VERIFIED" != "true" ] && [ -f "/proc/$PID/cmdline" ]; then
    CMDLINE=$(tr '\0' ' ' < "/proc/$PID/cmdline")
    if [[ "$CMDLINE" == *"claude-code-acp"* ]]; then
        VERIFIED=true
    fi
fi

if [ "$VERIFIED" != "true" ]; then
    echo "Process $PID is not claude-code-acp" >&2
    exit 1
fi

exit 0
