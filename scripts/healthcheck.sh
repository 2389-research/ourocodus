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

# Verify process is claude-code-acp (check cmdline)
if [ -f "/proc/$PID/cmdline" ]; then
    CMDLINE=$(tr '\0' ' ' < "/proc/$PID/cmdline")
    if [[ "$CMDLINE" != *"claude-code-acp"* ]]; then
        echo "Process $PID is not claude-code-acp" >&2
        exit 1
    fi
fi

exit 0
