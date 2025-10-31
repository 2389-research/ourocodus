#!/bin/sh
# NATS JetStream Initialization Script
# This script ensures JetStream streams are created idempotently
# Safe to run multiple times - will only create streams if they don't exist

set -e

echo "==================================="
echo "NATS JetStream Initialization"
echo "==================================="

# NATS server connection (using docker service name)
NATS_SERVER="nats://nats:4222"

# Wait for NATS server to be fully ready
echo "Waiting for NATS server to be ready..."
MAX_RETRIES=30
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if wget -q --spider http://nats:8222/healthz 2>/dev/null; then
        echo "✓ NATS server is healthy"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    echo "  Waiting for NATS... ($RETRY_COUNT/$MAX_RETRIES)"
    sleep 2
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo "✗ ERROR: NATS server failed to become healthy"
    exit 1
fi

# Additional wait to ensure JetStream is fully initialized
sleep 2

echo ""
echo "Verifying JetStream configuration..."

# Function to check if a stream exists
stream_exists() {
    nats --server="$NATS_SERVER" stream info "$1" >/dev/null 2>&1
    return $?
}

# Verify SESSION_EVENTS stream
echo ""
echo "Checking SESSION_EVENTS stream..."
if stream_exists "SESSION_EVENTS"; then
    echo "✓ SESSION_EVENTS stream exists"
else
    echo "  Creating SESSION_EVENTS stream..."
    nats --server="$NATS_SERVER" stream add SESSION_EVENTS \
        --subjects="sessions.*.events" \
        --storage=file \
        --retention=limits \
        --max-age=168h \
        --max-msgs=100000 \
        --max-bytes=1GB \
        --max-msg-size=1MB \
        --discard=old \
        --dupe-window=2m \
        --no-allow-rollup \
        --no-deny-delete \
        --no-deny-purge \
        --defaults
    echo "✓ SESSION_EVENTS stream created"
fi

# Verify WORK_RESULTS stream
echo ""
echo "Checking WORK_RESULTS stream..."
if stream_exists "WORK_RESULTS"; then
    echo "✓ WORK_RESULTS stream exists"
else
    echo "  Creating WORK_RESULTS stream..."
    nats --server="$NATS_SERVER" stream add WORK_RESULTS \
        --subjects="sessions.*.results.*" \
        --storage=file \
        --retention=limits \
        --max-age=168h \
        --max-msgs=100000 \
        --max-bytes=1GB \
        --max-msg-size=5MB \
        --discard=old \
        --dupe-window=2m \
        --no-allow-rollup \
        --no-deny-delete \
        --no-deny-purge \
        --defaults
    echo "✓ WORK_RESULTS stream created"
fi

# List all streams for verification
echo ""
echo "==================================="
echo "Current JetStream Streams:"
echo "==================================="
nats --server="$NATS_SERVER" stream list

echo ""
echo "✓ NATS JetStream initialization complete"
echo "==================================="

exit 0
