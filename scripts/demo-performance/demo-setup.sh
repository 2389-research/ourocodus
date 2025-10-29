#!/bin/bash
set -e

# Demo Setup Script
# Purpose: One-time preparation for the performance demo
# Run this once before your first demo

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🛠️  Performance Demo Setup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check prerequisites
echo "📋 Checking prerequisites..."
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go first."
    exit 1
fi
echo "✅ Go: $(go version | awk '{print $3}')"

# Check if curl is installed (for load testing)
if ! command -v curl &> /dev/null; then
    echo "❌ curl is not installed. Please install curl first."
    exit 1
fi
echo "✅ curl: Available"

# Check if bc is installed (for calculations)
if ! command -v bc &> /dev/null; then
    echo "❌ bc is not installed. Please install bc first (brew install bc on macOS)."
    exit 1
fi
echo "✅ bc: Available"

echo ""
echo "📦 Building project..."
cd "${PROJECT_ROOT}"
make build 2>&1 | grep -E "(Building|Error)" || true

if [ ! -f "${PROJECT_ROOT}/bin/relay" ]; then
    echo "❌ Build failed. Please run 'make build' manually and fix any errors."
    exit 1
fi
echo "✅ Build successful"

echo ""
echo "📊 Creating demo visualization server..."

# Create the visualization server
cat > "${SCRIPT_DIR}/viz-server.go" << 'EOF'
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"
)

type Metric struct {
	Version      string    `json:"version"`
	ResponseTime float64   `json:"response_time"`
	Timestamp    time.Time `json:"timestamp"`
	Success      bool      `json:"success"`
}

type Stats struct {
	Version         string  `json:"version"`
	AvgResponseTime float64 `json:"avg_response_time"`
	MinResponseTime float64 `json:"min_response_time"`
	MaxResponseTime float64 `json:"max_response_time"`
	RequestCount    int     `json:"request_count"`
	SuccessRate     float64 `json:"success_rate"`
}

var (
	metrics []Metric
	mu      sync.RWMutex
)

func main() {
	http.HandleFunc("/", serveHTML)
	http.HandleFunc("/api/metrics", handleMetrics)
	http.HandleFunc("/api/stats", handleStats)
	http.HandleFunc("/api/record", handleRecord)

	fmt.Println("🎨 Visualization server starting on http://localhost:9090")
	fmt.Println("📊 Open this URL in your browser to see real-time metrics")
	log.Fatal(http.ListenAndServe(":9090", nil))
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Performance Demo - Before/After Comparison</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            padding: 20px;
            min-height: 100vh;
        }
        .container {
            max-width: 1400px;
            margin: 0 auto;
        }
        h1 {
            color: white;
            text-align: center;
            margin-bottom: 30px;
            font-size: 2.5em;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.3);
        }
        .subtitle {
            color: rgba(255,255,255,0.9);
            text-align: center;
            margin-bottom: 40px;
            font-size: 1.2em;
        }
        .metrics-grid {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 30px;
            margin-bottom: 40px;
        }
        .metric-card {
            background: white;
            border-radius: 15px;
            padding: 30px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.3);
        }
        .metric-card.old {
            border-top: 5px solid #ef4444;
        }
        .metric-card.new {
            border-top: 5px solid #10b981;
        }
        .version-label {
            font-size: 1.5em;
            font-weight: bold;
            margin-bottom: 20px;
        }
        .old .version-label { color: #ef4444; }
        .new .version-label { color: #10b981; }
        .stat-row {
            display: flex;
            justify-content: space-between;
            padding: 15px 0;
            border-bottom: 1px solid #e5e7eb;
        }
        .stat-row:last-child { border-bottom: none; }
        .stat-label {
            color: #6b7280;
            font-weight: 500;
        }
        .stat-value {
            font-size: 1.2em;
            font-weight: bold;
        }
        .improvement-banner {
            background: linear-gradient(135deg, #10b981 0%, #059669 100%);
            color: white;
            padding: 30px;
            border-radius: 15px;
            text-align: center;
            box-shadow: 0 10px 30px rgba(0,0,0,0.3);
            margin-bottom: 30px;
        }
        .improvement-banner h2 {
            font-size: 2em;
            margin-bottom: 15px;
        }
        .improvement-banner .big-number {
            font-size: 4em;
            font-weight: bold;
            margin: 20px 0;
        }
        .chart-container {
            background: white;
            border-radius: 15px;
            padding: 30px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.3);
        }
        .chart-title {
            font-size: 1.5em;
            font-weight: bold;
            margin-bottom: 20px;
            color: #1f2937;
        }
        .bar-chart {
            display: flex;
            align-items: flex-end;
            height: 300px;
            gap: 40px;
            padding: 20px;
        }
        .bar {
            flex: 1;
            background: linear-gradient(180deg, #667eea 0%, #764ba2 100%);
            border-radius: 10px 10px 0 0;
            position: relative;
            min-height: 20px;
            transition: height 0.5s ease;
        }
        .bar.old { background: linear-gradient(180deg, #ef4444 0%, #dc2626 100%); }
        .bar.new { background: linear-gradient(180deg, #10b981 0%, #059669 100%); }
        .bar-label {
            text-align: center;
            margin-top: 10px;
            font-weight: bold;
            color: #1f2937;
        }
        .bar-value {
            position: absolute;
            top: -30px;
            width: 100%;
            text-align: center;
            font-weight: bold;
            font-size: 1.2em;
            color: #1f2937;
        }
        .loading {
            text-align: center;
            color: white;
            font-size: 1.5em;
            padding: 50px;
        }
        .status {
            text-align: center;
            color: rgba(255,255,255,0.9);
            font-size: 1.1em;
            margin-bottom: 20px;
            padding: 15px;
            background: rgba(255,255,255,0.1);
            border-radius: 10px;
        }
        .live-indicator {
            display: inline-block;
            width: 12px;
            height: 12px;
            background: #10b981;
            border-radius: 50%;
            margin-right: 8px;
            animation: pulse 2s infinite;
        }
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 Database Performance Optimization</h1>
        <div class="subtitle">Connection Pooling + Query Batching</div>

        <div class="status">
            <span class="live-indicator"></span>
            <span id="status">Waiting for demo to start...</span>
        </div>

        <div id="content" class="loading">Loading metrics...</div>
    </div>

    <script>
        let updateInterval;

        async function fetchStats() {
            try {
                const response = await fetch('/api/stats');
                const data = await response.json();

                if (data.old && data.new) {
                    document.getElementById('status').textContent = 'Live Demo Running';
                    renderDashboard(data);
                } else if (data.old || data.new) {
                    document.getElementById('status').textContent = 'Demo in progress...';
                }
            } catch (error) {
                console.error('Error fetching stats:', error);
            }
        }

        function renderDashboard(data) {
            const improvement = ((data.old.avg_response_time - data.new.avg_response_time) / data.old.avg_response_time * 100).toFixed(1);
            const throughputIncrease = ((1 / data.new.avg_response_time) / (1 / data.old.avg_response_time)).toFixed(1);

            const html = ` + "`" + `
                <div class="improvement-banner">
                    <h2>⚡ Performance Improvement</h2>
                    <div class="big-number">${improvement}% Faster</div>
                    <div>${throughputIncrease}x More Throughput</div>
                </div>

                <div class="metrics-grid">
                    <div class="metric-card old">
                        <div class="version-label">❌ BEFORE (Old Version)</div>
                        <div class="stat-row">
                            <span class="stat-label">Average Response Time</span>
                            <span class="stat-value">${data.old.avg_response_time.toFixed(0)}ms</span>
                        </div>
                        <div class="stat-row">
                            <span class="stat-label">Min Response Time</span>
                            <span class="stat-value">${data.old.min_response_time.toFixed(0)}ms</span>
                        </div>
                        <div class="stat-row">
                            <span class="stat-label">Max Response Time</span>
                            <span class="stat-value">${data.old.max_response_time.toFixed(0)}ms</span>
                        </div>
                        <div class="stat-row">
                            <span class="stat-label">Requests Tested</span>
                            <span class="stat-value">${data.old.request_count}</span>
                        </div>
                        <div class="stat-row">
                            <span class="stat-label">Success Rate</span>
                            <span class="stat-value">${data.old.success_rate.toFixed(1)}%</span>
                        </div>
                    </div>

                    <div class="metric-card new">
                        <div class="version-label">✅ AFTER (New Version)</div>
                        <div class="stat-row">
                            <span class="stat-label">Average Response Time</span>
                            <span class="stat-value">${data.new.avg_response_time.toFixed(0)}ms</span>
                        </div>
                        <div class="stat-row">
                            <span class="stat-label">Min Response Time</span>
                            <span class="stat-value">${data.new.min_response_time.toFixed(0)}ms</span>
                        </div>
                        <div class="stat-row">
                            <span class="stat-label">Max Response Time</span>
                            <span class="stat-value">${data.new.max_response_time.toFixed(0)}ms</span>
                        </div>
                        <div class="stat-row">
                            <span class="stat-label">Requests Tested</span>
                            <span class="stat-value">${data.new.request_count}</span>
                        </div>
                        <div class="stat-row">
                            <span class="stat-label">Success Rate</span>
                            <span class="stat-value">${data.new.success_rate.toFixed(1)}%</span>
                        </div>
                    </div>
                </div>

                <div class="chart-container">
                    <div class="chart-title">📊 Response Time Comparison</div>
                    <div class="bar-chart">
                        <div style="flex: 1;">
                            <div class="bar old" style="height: ${(data.old.avg_response_time / Math.max(data.old.avg_response_time, data.new.avg_response_time)) * 100}%">
                                <div class="bar-value">${data.old.avg_response_time.toFixed(0)}ms</div>
                            </div>
                            <div class="bar-label">BEFORE</div>
                        </div>
                        <div style="flex: 1;">
                            <div class="bar new" style="height: ${(data.new.avg_response_time / Math.max(data.old.avg_response_time, data.new.avg_response_time)) * 100}%">
                                <div class="bar-value">${data.new.avg_response_time.toFixed(0)}ms</div>
                            </div>
                            <div class="bar-label">AFTER</div>
                        </div>
                    </div>
                </div>
            ` + "`" + `;

            document.getElementById('content').innerHTML = html;
        }

        // Update every 1 second
        fetchStats();
        updateInterval = setInterval(fetchStats, 1000);
    </script>
</body>
</html>`;
	w.Header().Set("Content-Type", "text/html")
	template.Must(template.New("index").Parse(tmpl)).Execute(w, nil)
}

func handleRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var metric Metric
	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	metric.Timestamp = time.Now()

	mu.Lock()
	metrics = append(metrics, metric)
	mu.Unlock()

	w.WriteHeader(http.StatusOK)
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	oldStats := calculateStats("old")
	newStats := calculateStats("new")

	response := map[string]*Stats{
		"old": oldStats,
		"new": newStats,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func calculateStats(version string) *Stats {
	var filtered []Metric
	for _, m := range metrics {
		if m.Version == version {
			filtered = append(filtered, m)
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	stats := &Stats{
		Version:         version,
		MinResponseTime: filtered[0].ResponseTime,
		MaxResponseTime: filtered[0].ResponseTime,
	}

	var total float64
	var successCount int

	for _, m := range filtered {
		total += m.ResponseTime
		if m.ResponseTime < stats.MinResponseTime {
			stats.MinResponseTime = m.ResponseTime
		}
		if m.ResponseTime > stats.MaxResponseTime {
			stats.MaxResponseTime = m.ResponseTime
		}
		if m.Success {
			successCount++
		}
	}

	stats.AvgResponseTime = total / float64(len(filtered))
	stats.RequestCount = len(filtered)
	stats.SuccessRate = (float64(successCount) / float64(len(filtered))) * 100

	return stats
}
EOF

echo "✅ Visualization server created"

echo ""
echo "🎯 Building visualization server..."
cd "${SCRIPT_DIR}"
go build -o viz-server viz-server.go
echo "✅ Visualization server built"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Setup complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📝 Next steps:"
echo "   1. Run './demo-run.sh' to start the demo"
echo "   2. Open http://localhost:9090 in your browser"
echo "   3. Watch the live performance comparison!"
echo ""
