# Demo Screenshots Guide

Take these screenshots during your demo for documentation and presentations.

## Screenshot 1: Terminal - Initial Output

**File**: `terminal_startup.png`

**What to capture**: Terminal showing demo startup

**Should show**:
```
🚀 DATABASE PERFORMANCE OPTIMIZATION DEMO
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 Demo Configuration:
   • Connection Pooling: Enabled
   • Query Batching: Enabled
   • Test Requests: 50 per version
   • Visualization: http://localhost:9090

🎨 Starting visualization server...
✅ Visualization server ready
```

**Use for**: PR description, technical documentation

---

## Screenshot 2: Terminal - OLD Version Results

**File**: `terminal_old_version.png`

**What to capture**: Terminal showing OLD version test complete

**Should show**:
```
❌ OLD VERSION
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Progress: [██████████████████████████████████████████████████] 50/50

📊 Results:
   Average Response Time: 250ms
   Min Response Time: 230ms
   Max Response Time: 280ms
   Total Requests: 50
```

**Use for**: Before/after comparison, showing baseline performance

---

## Screenshot 3: Terminal - NEW Version Results

**File**: `terminal_new_version.png`

**What to capture**: Terminal showing NEW version test complete

**Should show**:
```
✅ NEW VERSION
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Progress: [██████████████████████████████████████████████████] 50/50

📊 Results:
   Average Response Time: 100ms
   Min Response Time: 85ms
   Max Response Time: 120ms
   Total Requests: 50
```

**Use for**: Showing improvement, celebrating success

---

## Screenshot 4: Terminal - Summary

**File**: `terminal_summary.png`

**What to capture**: Terminal showing final summary

**Should show**:
```
🎉 DEMO COMPLETE!

📈 Performance Summary:
   ⚡ Response Time Improvement: 60%
   🚀 Throughput Increase: 3x

💡 What This Means:
   • Users see 60% faster response times
   • Server can handle 3x more concurrent users
   • Database connections are efficiently pooled
   • Queries are batched to reduce round-trips
```

**Use for**: Executive summary, celebration post, PR highlights

---

## Screenshot 5: Browser - Full Dashboard

**File**: `browser_full_dashboard.png`

**What to capture**: Full browser window showing complete visualization

**Should show**:
- Title: "Database Performance Optimization"
- Green improvement banner: "60% Faster"
- Split-screen metrics (old vs new)
- Bar chart comparison at bottom
- All live data populated

**Use for**: Main demo image, blog posts, presentations, documentation cover

**Tips**:
- Maximize browser window
- Wait for all metrics to load
- Ensure green banner shows "60% Faster"

---

## Screenshot 6: Browser - Improvement Banner (Close-up)

**File**: `browser_improvement_banner.png`

**What to capture**: Just the top green banner

**Should show**:
```
⚡ Performance Improvement
    60% Faster
  3x More Throughput
```

**Use for**: Social media, Slack announcements, quick wins showcase

**Tips**:
- Crop tightly around the green banner
- This is the "hero" image
- Use in thumbnails and previews

---

## Screenshot 7: Browser - Side-by-Side Metrics

**File**: `browser_metrics_comparison.png`

**What to capture**: The two metric cards (old vs new)

**Should show**:
- Left card (red): "BEFORE (Old Version)"
- Right card (green): "AFTER (New Version)"
- All metrics populated
- Clear visual contrast

**Use for**: Technical documentation, detailed analysis

---

## Screenshot 8: Browser - Bar Chart

**File**: `browser_chart.png`

**What to capture**: Just the bar chart at bottom

**Should show**:
- "Response Time Comparison" title
- Two bars: BEFORE (tall, red) and AFTER (short, green)
- Response times labeled on bars

**Use for**: Simple visual explanation, presentations

**Why it works**: Universal language - anyone can see shorter bar = better

---

## Screenshot 9: Load Test - Concurrent Users

**File**: `terminal_load_test.png`

**What to capture**: Load test output (if using demo-load-test.sh)

**Should show**:
```
🚀 DATABASE LOAD TESTING DEMO
━━━━━━━━━━━━━━━━━━━━━━━━━━━
📋 Load Test Configuration:
   • Concurrent Users: 20
   • Requests Per User: 10
   • Total Requests: 200

👥 Simulating 20 concurrent users...
📊 Each user making 10 requests...
```

**Use for**: Technical deep-dives, architecture discussions

---

## Screenshot 10: Metrics Table (Generated)

**File**: `metrics_table.png`

**What to capture**: Output from generate-metrics-table.sh

**Should show**:
Markdown table with:
| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Avg Response | 250ms | 100ms | 60% faster |

**Use for**: Documentation, GitHub PR, wiki pages

---

## Quick Screenshot Checklist

During demo, capture these in order:

1. [ ] **Terminal**: Demo startup
2. [ ] **Terminal**: OLD version results
3. [ ] **Terminal**: NEW version results
4. [ ] **Terminal**: Final summary
5. [ ] **Browser**: Full dashboard (main image)
6. [ ] **Browser**: Improvement banner (hero image)
7. [ ] **Browser**: Side-by-side metrics
8. [ ] **Browser**: Bar chart
9. [ ] **Optional**: Load test results
10. [ ] **Optional**: Generated metrics table

---

## Screenshot Best Practices

### Terminal Screenshots
- **Font size**: Increase for readability (Cmd/Ctrl + +)
- **Colors**: Ensure colors are visible (avoid light themes)
- **Crop**: Remove unnecessary top/bottom whitespace
- **Format**: PNG for clarity

### Browser Screenshots
- **Window size**: Maximize or use consistent size
- **Resolution**: High-DPI if possible
- **Wait**: Let all animations complete
- **Clean**: Close dev tools, hide bookmarks bar

### File Naming Convention
```
demo_<location>_<what>_<when>.png

Examples:
- demo_terminal_old_version_2024-10-29.png
- demo_browser_full_dashboard_2024-10-29.png
- demo_browser_improvement_banner_2024-10-29.png
```

---

## Where to Use Screenshots

### GitHub PR Description
```markdown
## Performance Improvement Demo

![Improvement Summary](screenshots/terminal_summary.png)

### Before vs After
![Metrics Comparison](screenshots/browser_metrics_comparison.png)

### Visual Comparison
![Bar Chart](screenshots/browser_chart.png)
```

### Team Slack Announcement
```
🎉 Database optimization complete!

[browser_improvement_banner.png]

60% faster response times, 3x more capacity!
Full demo: ./examples/performance-testing/demo-performance/demo-run.sh
```

### Documentation Wiki
```markdown
# Database Performance Optimization

## Overview
[Full dashboard screenshot]

## Results
[Metrics comparison screenshot]

## How to Run
[Terminal startup screenshot]
```

### Blog Post / Article
- Hero image: Full browser dashboard
- Supporting images: Terminal results, bar chart
- Technical detail: Load test results

---

## Video Alternative

Instead of screenshots, record a screen video:

### macOS
```bash
# Record screen with QuickTime Player
# File > New Screen Recording
```

### Linux
```bash
# Use OBS Studio or recordmydesktop
recordmydesktop --on-the-fly-encoding -o demo.ogv
```

### Convert to GIF
```bash
# Use ffmpeg
ffmpeg -i demo.mp4 -vf "fps=10,scale=800:-1" demo.gif
```

**Advantage**: Shows live updates, more engaging
**Disadvantage**: Larger file size
**Best for**: README.md hero image, social media

---

## Screenshot Storage

### Option 1: Commit to Repo
```
ourocodus/
└── docs/
    └── images/
        └── performance-demo/
            ├── terminal_summary.png
            ├── browser_full_dashboard.png
            └── browser_improvement_banner.png
```

**Pros**: Version controlled, always available
**Cons**: Increases repo size

### Option 2: External Hosting
- GitHub Issues (paste screenshots in comments)
- Imgur / Cloudinary
- Team wiki / Confluence
- Documentation site

**Pros**: Doesn't bloat repo
**Cons**: Links can break

### Recommendation
- **Hero images**: Commit to repo (used in README)
- **Supporting images**: External hosting (used in issues/PRs)
- **Historical images**: Team wiki for reference

---

## Using Screenshots in Presentations

### Slide 1: Title
```
Database Performance Optimization
[Full dashboard screenshot as background, 50% opacity]
```

### Slide 2: The Problem
```
"No UI changes, but 60% faster"
[Terminal startup screenshot]
```

### Slide 3: The Results
```
Before vs After
[Side-by-side metrics screenshot]
```

### Slide 4: Visual Proof
```
[Bar chart screenshot - large, centered]
"Short bar = Better"
```

### Slide 5: Business Impact
```
[Improvement banner screenshot]
• 60% faster
• 3x capacity
• Same servers
```

---

## Animated Alternatives

Create animated demos with:

### Asciinema (Terminal)
```bash
# Record terminal session
asciinema rec demo.cast

# Upload and share
asciinema upload demo.cast
```

**Result**: Shareable, embeddable terminal recording

### Loom / CloudApp (Browser)
Record browser + narration

**Result**: Video demo with your voice explaining

**Use for**: Async presentations, training materials

---

**Pro Tip**: Take ALL screenshots during demo even if you don't use them all. You can always delete extras, but you can't go back and retake during live demo!
