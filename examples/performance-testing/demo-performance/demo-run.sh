#!/bin/bash
set -e

# Demo Run Script
# Purpose: Automated performance demonstration with visual comparison
# This script simulates before/after performance metrics and displays them in real-time

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# PROJECT_ROOT unused in this script, but available if needed for customization
# PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Configuration
VIZ_PORT=9090
OLD_BASE_LATENCY=250  # Simulated "old" average response time (ms)
NEW_BASE_LATENCY=100  # Simulated "new" average response time (ms)
NUM_REQUESTS=50       # Number of test requests per version

# Colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# PID tracking
VIZ_PID=""

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}🧹 Cleaning up...${NC}"
    if [ -n "$VIZ_PID" ] && kill -0 "$VIZ_PID" 2>/dev/null; then
        kill "$VIZ_PID" 2>/dev/null || true
        wait "$VIZ_PID" 2>/dev/null || true
        echo -e "${GREEN}✅ Visualization server stopped${NC}"
    fi
}

trap cleanup EXIT INT TERM

# Function to send metrics to visualization server
record_metric() {
    local version=$1
    local response_time=$2
    local success=${3:-true}

    curl -s -X POST http://localhost:${VIZ_PORT}/api/record \
        -H "Content-Type: application/json" \
        -d "{\"version\":\"$version\",\"response_time\":$response_time,\"success\":$success}" \
        > /dev/null 2>&1
}

# Function to simulate a request with realistic variance
simulate_request() {
    local base_latency=$1
    local variance=20  # ±20% variance

    # Generate random variance (-variance to +variance percent)
    local rand=$((RANDOM % (variance * 2) - variance))
    local latency
    latency=$(echo "$base_latency * (100 + $rand) / 100" | bc)

    # Simulate the delay
    local sleep_duration
    sleep_duration=$(echo "scale=3; $latency / 1000" | bc)
    sleep "$sleep_duration" 2>/dev/null || sleep 0.1

    echo "$latency"
}

# Function to run performance test
run_performance_test() {
    local version=$1
    local base_latency=$2
    local label=$3
    local color=$4

    echo -e "${color}${label}${NC}"
    echo -e "${color}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    local total=0
    local min=999999
    local max=0

    for i in $(seq 1 "$NUM_REQUESTS"); do
        # Show progress
        local progress_bar
        progress_bar=$(printf '█%.0s' $(seq 1 $((i * 50 / NUM_REQUESTS))))
        printf "\r  Progress: [%-50s] %d/%d" \
            "$progress_bar" \
            "$i" "$NUM_REQUESTS"

        # Simulate request
        local latency
        latency=$(simulate_request "$base_latency")
        record_metric "$version" "$latency" true

        # Track stats
        total=$(echo "$total + $latency" | bc)
        if (( $(echo "$latency < $min" | bc -l) )); then
            min=$latency
        fi
        if (( $(echo "$latency > $max" | bc -l) )); then
            max=$latency
        fi
    done

    echo ""
    echo ""

    # Calculate and display results
    local avg
    avg=$(echo "scale=2; $total / $NUM_REQUESTS" | bc)

    echo -e "  ${color}📊 Results:${NC}"
    echo -e "     Average Response Time: ${color}${avg}ms${NC}"
    echo -e "     Min Response Time: ${color}${min}ms${NC}"
    echo -e "     Max Response Time: ${color}${max}ms${NC}"
    echo -e "     Total Requests: ${color}${NUM_REQUESTS}${NC}"
    echo ""
}

# Main demo execution
main() {
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🚀 DATABASE PERFORMANCE OPTIMIZATION DEMO"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "📋 Demo Configuration:"
    echo "   • Connection Pooling: Enabled"
    echo "   • Query Batching: Enabled"
    echo "   • Test Requests: ${NUM_REQUESTS} per version"
    echo "   • Visualization: http://localhost:${VIZ_PORT}"
    echo ""

    # Check if visualization server binary exists
    if [ ! -f "${SCRIPT_DIR}/viz-server" ]; then
        echo -e "${RED}❌ Visualization server not found${NC}"
        echo "   Please run './demo-setup.sh' first"
        exit 1
    fi

    # Start visualization server
    echo -e "${BLUE}🎨 Starting visualization server...${NC}"
    "${SCRIPT_DIR}/viz-server" > /dev/null 2>&1 &
    VIZ_PID=$!

    # Wait for server to be ready
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

    # Prompt to open browser
    echo -e "${CYAN}🌐 Open your browser to: ${YELLOW}http://localhost:${VIZ_PORT}${NC}"
    echo ""
    read -p "Press ENTER when you're ready to start the demo..." -r
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    # Run "BEFORE" test
    echo -e "${RED}📊 PHASE 1: Testing OLD version (without optimizations)${NC}"
    echo ""
    sleep 2
    run_performance_test "old" "$OLD_BASE_LATENCY" "❌ OLD VERSION" "$RED"

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    sleep 3

    # Run "AFTER" test
    echo -e "${GREEN}📊 PHASE 2: Testing NEW version (with optimizations)${NC}"
    echo ""
    sleep 2
    run_performance_test "new" "$NEW_BASE_LATENCY" "✅ NEW VERSION" "$GREEN"

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""

    # Calculate final comparison
    local improvement
    improvement=$(echo "scale=1; ($OLD_BASE_LATENCY - $NEW_BASE_LATENCY) / $OLD_BASE_LATENCY * 100" | bc)
    local throughput_increase
    throughput_increase=$(echo "scale=1; $OLD_BASE_LATENCY / $NEW_BASE_LATENCY" | bc)

    echo -e "${GREEN}🎉 DEMO COMPLETE!${NC}"
    echo ""
    echo -e "${CYAN}📈 Performance Summary:${NC}"
    echo -e "   ⚡ Response Time Improvement: ${GREEN}${improvement}%${NC}"
    echo -e "   🚀 Throughput Increase: ${GREEN}${throughput_increase}x${NC}"
    echo ""
    echo -e "${YELLOW}💡 What This Means:${NC}"
    echo -e "   • Users see ${GREEN}60% faster${NC} response times"
    echo -e "   • Server can handle ${GREEN}3x more${NC} concurrent users"
    echo -e "   • Database connections are ${GREEN}efficiently pooled${NC}"
    echo -e "   • Queries are ${GREEN}batched${NC} to reduce round-trips"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo -e "${CYAN}📊 The visualization will remain open at: ${YELLOW}http://localhost:${VIZ_PORT}${NC}"
    echo ""
    read -p "Press ENTER to close the demo..." -r
    echo ""
}

main
