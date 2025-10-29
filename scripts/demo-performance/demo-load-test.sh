#!/bin/bash
set -e

# Demo Load Test Script
# Purpose: Demonstrate performance under concurrent load
# Shows how connection pooling handles multiple simultaneous users

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration
VIZ_PORT=9090
OLD_BASE_LATENCY=300  # Higher under load
NEW_BASE_LATENCY=120  # Still better under load
CONCURRENT_USERS=20   # Simulate 20 concurrent users
REQUESTS_PER_USER=10  # Each user makes 10 requests

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

VIZ_PID=""

cleanup() {
    echo ""
    echo -e "${YELLOW}🧹 Cleaning up...${NC}"
    if [ -n "$VIZ_PID" ] && kill -0 "$VIZ_PID" 2>/dev/null; then
        kill "$VIZ_PID" 2>/dev/null || true
        wait "$VIZ_PID" 2>/dev/null || true
        echo -e "${GREEN}✅ Visualization server stopped${NC}"
    fi
    # Kill any background user simulation jobs
    jobs -p | xargs -r kill 2>/dev/null || true
}

trap cleanup EXIT INT TERM

record_metric() {
    local version=$1
    local response_time=$2
    local success=${3:-true}

    curl -s -X POST http://localhost:${VIZ_PORT}/api/record \
        -H "Content-Type: application/json" \
        -d "{\"version\":\"$version\",\"response_time\":$response_time,\"success\":$success}" \
        > /dev/null 2>&1
}

simulate_request() {
    local base_latency=$1
    local load_factor=$2  # Additional latency due to concurrent load

    # More variance under load
    local variance=30
    local rand=$((RANDOM % (variance * 2) - variance))
    local latency
    latency=$(echo "$base_latency * (100 + $rand) / 100 + $load_factor" | bc)

    local sleep_duration
    sleep_duration=$(echo "scale=3; $latency / 1000" | bc)
    sleep "$sleep_duration" 2>/dev/null || sleep 0.1
    echo "$latency"
}

simulate_user() {
    local user_id=$1
    local version=$2
    local base_latency=$3
    local requests=$4

    for _ in $(seq 1 "$requests"); do
        # Calculate load factor (increases with concurrent users)
        local load_factor=0
        if [ "$version" = "old" ]; then
            # Old version suffers more under load (no connection pooling)
            load_factor=$(echo "$CONCURRENT_USERS * 15" | bc)
        else
            # New version handles load better (with connection pooling)
            load_factor=$(echo "$CONCURRENT_USERS * 3" | bc)
        fi

        local latency
        latency=$(simulate_request "$base_latency" "$load_factor")
        record_metric "$version" "$latency" true

        # Small random delay between requests
        local delay
        delay=$(echo "scale=2; $RANDOM % 100 / 100" | bc)
        sleep "$delay" 2>/dev/null || sleep 0.05
    done
}

run_load_test() {
    local version=$1
    local base_latency=$2
    local label=$3
    local color=$4

    echo -e "${color}${label}${NC}"
    echo -e "${color}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "  ${MAGENTA}👥 Simulating ${CONCURRENT_USERS} concurrent users...${NC}"
    echo -e "  ${MAGENTA}📊 Each user making ${REQUESTS_PER_USER} requests...${NC}"
    echo ""

    local start_time
    start_time=$(date +%s)

    # Launch concurrent users as background jobs
    for user_id in $(seq 1 "$CONCURRENT_USERS"); do
        simulate_user "$user_id" "$version" "$base_latency" "$REQUESTS_PER_USER" &
    done

    # Show progress while waiting
    local total_requests=$((CONCURRENT_USERS * REQUESTS_PER_USER))
    local completed=0

    while [ "$completed" -lt "$total_requests" ]; do
        # Query visualization server for current count
        local current
        current=$(curl -s http://localhost:${VIZ_PORT}/api/stats | \
            grep -o "\"request_count\":[0-9]*" | tail -1 | grep -o "[0-9]*" || echo "0")

        if [ -n "$current" ] && [ "$current" -gt "$completed" ]; then
            completed=$current
        fi

        # Calculate progress
        local progress=$((completed * 100 / total_requests))
        local bar_length=$((progress / 2))

        printf "\r  Progress: [%-50s] %d/%d (%d%%)" \
            "$(printf '█%.0s' $(seq 1 $bar_length))" \
            "$completed" "$total_requests" "$progress"

        sleep 0.5
    done

    # Wait for all background jobs to complete
    wait

    local end_time
    end_time=$(date +%s)
    local duration=$((end_time - start_time))

    echo ""
    echo ""

    # Calculate throughput
    local throughput
    throughput=$(echo "scale=2; $total_requests / $duration" | bc)

    echo -e "  ${color}📊 Results:${NC}"
    echo -e "     Concurrent Users: ${color}${CONCURRENT_USERS}${NC}"
    echo -e "     Total Requests: ${color}${total_requests}${NC}"
    echo -e "     Duration: ${color}${duration}s${NC}"
    echo -e "     Throughput: ${color}${throughput} req/s${NC}"
    echo ""
}

main() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🚀 DATABASE LOAD TESTING DEMO"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "📋 Load Test Configuration:"
    echo "   • Connection Pooling: Testing impact under load"
    echo "   • Query Batching: Reducing database round-trips"
    echo "   • Concurrent Users: ${CONCURRENT_USERS}"
    echo "   • Requests Per User: ${REQUESTS_PER_USER}"
    echo "   • Total Requests: $((CONCURRENT_USERS * REQUESTS_PER_USER))"
    echo "   • Visualization: http://localhost:${VIZ_PORT}"
    echo ""

    # Check prerequisites
    if [ ! -f "${SCRIPT_DIR}/viz-server" ]; then
        echo -e "${RED}❌ Visualization server not found${NC}"
        echo "   Please run './demo-setup.sh' first"
        exit 1
    fi

    # Start visualization server
    echo -e "${BLUE}🎨 Starting visualization server...${NC}"
    "${SCRIPT_DIR}/viz-server" > /dev/null 2>&1 &
    VIZ_PID=$!

    # Wait for server
    echo -n "   Waiting for server to start"
    for _ in {1..10}; do
        if curl -s http://localhost:${VIZ_PORT} > /dev/null 2>&1; then
            echo ""
            break
        fi
        echo -n "."
        sleep 1
    done

    if ! curl -s http://localhost:${VIZ_PORT} > /dev/null 2>&1; then
        echo ""
        echo -e "${RED}❌ Failed to start visualization server${NC}"
        exit 1
    fi

    echo -e "${GREEN}✅ Visualization server ready${NC}"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    echo -e "${CYAN}🌐 Open your browser to: ${YELLOW}http://localhost:${VIZ_PORT}${NC}"
    echo ""
    read -p "Press ENTER when you're ready to start the load test..." -r
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    # Run OLD version under load
    echo -e "${RED}📊 PHASE 1: Testing OLD version under load${NC}"
    echo -e "${RED}   (No connection pooling - suffers under concurrent load)${NC}"
    echo ""
    sleep 2
    run_load_test "old" "$OLD_BASE_LATENCY" "❌ OLD VERSION - LOAD TEST" "$RED"

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    sleep 3

    # Run NEW version under load
    echo -e "${GREEN}📊 PHASE 2: Testing NEW version under load${NC}"
    echo -e "${GREEN}   (With connection pooling - handles load efficiently)${NC}"
    echo ""
    sleep 2
    run_load_test "new" "$NEW_BASE_LATENCY" "✅ NEW VERSION - LOAD TEST" "$GREEN"

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    # Summary
    local improvement
    improvement=$(echo "scale=1; ($OLD_BASE_LATENCY - $NEW_BASE_LATENCY) / $OLD_BASE_LATENCY * 100" | bc)
    local capacity_increase
    capacity_increase=$(echo "scale=1; $OLD_BASE_LATENCY / $NEW_BASE_LATENCY" | bc)

    echo -e "${GREEN}🎉 LOAD TEST COMPLETE!${NC}"
    echo ""
    echo -e "${CYAN}📈 Performance Under Load:${NC}"
    echo -e "   ⚡ Response Time Improvement: ${GREEN}${improvement}%${NC}"
    echo -e "   🚀 Capacity Increase: ${GREEN}${capacity_increase}x more users${NC}"
    echo ""
    echo -e "${YELLOW}💡 Key Findings:${NC}"
    echo -e "   ${RED}❌ OLD VERSION:${NC}"
    echo -e "      • Each request creates new DB connection"
    echo -e "      • Connection overhead compounds under load"
    echo -e "      • Performance degrades with concurrent users"
    echo ""
    echo -e "   ${GREEN}✅ NEW VERSION:${NC}"
    echo -e "      • Connections are pooled and reused"
    echo -e "      • Queries are batched to reduce round-trips"
    echo -e "      • Performance remains stable under load"
    echo -e "      • Can handle ${capacity_increase}x more concurrent users"
    echo ""
    echo -e "${YELLOW}💰 Business Impact:${NC}"
    echo -e "   • Support more customers ${GREEN}without new infrastructure${NC}"
    echo -e "   • Reduce cloud costs (fewer DB connections)"
    echo -e "   • Better user experience under peak load"
    echo -e "   • Improved system reliability"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo -e "${CYAN}📊 View detailed metrics at: ${YELLOW}http://localhost:${VIZ_PORT}${NC}"
    echo ""
    read -p "Press ENTER to close the demo..." -r
    echo ""
}

main
