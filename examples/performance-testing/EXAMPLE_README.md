# Performance Testing Examples

## Purpose

This directory contains scripts for performance testing and benchmarking the Ourocodus system. These examples demonstrate:

- **Load testing**: Simulating multiple concurrent users
- **Performance measurement**: Capturing response times and throughput
- **Before/after comparisons**: Validating optimization improvements
- **Visualization**: Real-time metrics dashboards
- **Reporting**: Generating performance metrics tables

## Educational Value

Learn how to:
- Design effective load tests
- Measure system performance under stress
- Identify performance bottlenecks
- Visualize performance data
- Compare optimization results

## Prerequisites

1. **Built binaries**: Run `make build` from repository root
2. **Docker** (optional): For containerized testing
3. **Available ports**: Various ports for visualization servers
4. **Go installed**: Required for test scripts

## Available Scripts

### demo-setup.sh
**Purpose**: One-time setup for performance testing environment

**What it does**:
- Builds visualization server
- Verifies prerequisites
- Prepares test environment

**Usage**:
```bash
./demo-setup.sh
```

### demo-run.sh
**Purpose**: Main automated performance demo with browser visualization

**What it does**:
- Runs "before" performance test
- Runs "after" performance test (with optimizations)
- Generates side-by-side comparison
- Serves results on http://localhost:9090

**Usage**:
```bash
./demo-run.sh
# Open browser to http://localhost:9090
```

### demo-load-test.sh
**Purpose**: Advanced concurrent load testing

**What it does**:
- Simulates multiple concurrent users
- Measures system behavior under heavy load
- Tests scalability limits
- Generates detailed performance metrics

**Usage**:
```bash
./demo-load-test.sh
```

### demo-reset.sh
**Purpose**: Clean up test environment

**What it does**:
- Stops running services
- Cleans up test data
- Resets environment to initial state

**Usage**:
```bash
./demo-reset.sh
```

### generate-metrics-table.sh
**Purpose**: Generate formatted metrics tables from test results

**What it does**:
- Parses raw performance data
- Calculates statistics (avg, min, max, percentiles)
- Formats results as markdown tables
- Useful for documentation and reports

**Usage**:
```bash
./generate-metrics-table.sh <results-file>
```

## Quick Start Guide

### Basic Performance Demo

```bash
# 1. One-time setup
cd examples/performance-testing
./demo-setup.sh

# 2. Run the demo
./demo-run.sh

# 3. Open browser to http://localhost:9090
#    Watch live before/after comparison

# 4. When done
./demo-reset.sh
```

### Load Testing

```bash
# Run concurrent load test
cd examples/performance-testing
./demo-load-test.sh

# Results will be displayed in terminal
# and optionally saved to file
```

## Understanding the Metrics

### Response Time
- **Average**: Mean time for requests
- **Min/Max**: Fastest and slowest requests
- **P50/P95/P99**: Percentile measurements

### Throughput
- **Requests/sec**: Rate of successful requests
- **Concurrent users**: Simultaneous connections
- **Success rate**: Percentage of successful requests

### Resource Usage
- **CPU**: Processor utilization
- **Memory**: RAM consumption
- **Connections**: Active network connections

## Use Cases

### 1. Validate Optimizations

Test before and after code changes:

```bash
# Record baseline
./demo-run.sh > baseline.txt

# Make optimization changes
# Rebuild: make build

# Test again
./demo-run.sh > optimized.txt

# Compare results
diff baseline.txt optimized.txt
```

### 2. Capacity Planning

Determine system limits:

```bash
# Start with low load
./demo-load-test.sh --users 10

# Gradually increase
./demo-load-test.sh --users 50
./demo-load-test.sh --users 100
./demo-load-test.sh --users 500

# Find breaking point
```

### 3. Regression Testing

Ensure performance doesn't degrade:

```bash
# Run regularly in CI/CD
./demo-run.sh > current-performance.txt

# Compare against baseline
if performance_degraded; then
    alert "Performance regression detected"
fi
```

### 4. Demo for Stakeholders

Show improvement impact:

```bash
# Run visualization demo
./demo-run.sh

# Open http://localhost:9090 in browser
# Present split-screen comparison
# Highlight key metrics
```

## Interpreting Results

### Good Performance Indicators
- ✅ Response times < 100ms for simple operations
- ✅ > 95% success rate under load
- ✅ Linear scaling with added resources
- ✅ Stable memory usage over time

### Warning Signs
- ⚠️  Response times increasing over time
- ⚠️  Success rate dropping under load
- ⚠️  Memory leaks (increasing usage)
- ⚠️  High error rates

### Critical Issues
- ❌ System crashes under load
- ❌ Response times > 1 second
- ❌ Success rate < 90%
- ❌ Complete service unavailability

## Troubleshooting

### Setup fails

**Cause**: Missing dependencies

**Solution**:
```bash
# Install required tools
go mod tidy
make build

# Verify prerequisites
./demo-setup.sh --check
```

### Test hangs or times out

**Cause**: System under too much load

**Solution**:
- Reduce concurrent users
- Increase timeout values
- Check system resources

### Metrics don't match expectations

**Cause**: Incorrect test configuration

**Solution**:
- Verify test parameters
- Check for interference from other processes
- Run tests multiple times for consistency

### Visualization not loading

**Cause**: Port conflicts or server issues

**Solution**:
```bash
# Check if port is available
lsof -i :9090

# Try different port
./demo-run.sh --port 9091
```

## Best Practices

1. **Consistent environment**: Run tests on same hardware/configuration
2. **Multiple runs**: Average results across 3-5 runs
3. **Warm-up period**: Discard first run to avoid cold-start effects
4. **Isolated testing**: Close other applications during tests
5. **Document changes**: Record configuration between test runs

## Advanced Topics

### Custom Test Scenarios

Edit scripts to test specific scenarios:
- Different request patterns
- Variable payload sizes
- Mixed read/write operations
- Error handling under load

### Integration with CI/CD

```yaml
# Example GitHub Actions workflow
performance-test:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v2
    - name: Run performance tests
      run: |
        cd examples/performance-testing
        ./demo-setup.sh
        ./demo-run.sh
    - name: Upload results
      uses: actions/upload-artifact@v2
      with:
        name: performance-results
        path: results/
```

## Related Documentation

- [Basic Demo](../basic-demo/README.md) - Start with basic functionality
- [Architecture](../../docs/architecture.md) - Understand system design
- [QUICK_START.md](./QUICK_START.md) - Condensed quick start
- [PRESENTATION_GUIDE.md](./PRESENTATION_GUIDE.md) - Presenting performance results

## Detailed Documentation

This directory contains additional detailed documentation:

- **README.md**: Original comprehensive guide
- **DEMO_PACKAGE_OVERVIEW.md**: Package structure and implementation details
- **PRESENTATION_GUIDE.md**: Tips for presenting performance improvements
- **QUICK_START.md**: Condensed getting started guide
- **SCREENSHOTS.md**: Visual guide to the demo

## Notes

- Performance results vary by hardware and system load
- These are examples, not production benchmarking tools
- Customize scripts for your specific use cases
- Consider adding more realistic workloads for your scenarios
- Use external tools (Apache Bench, wrk, k6) for production load testing
