# Quick Start - 30 Second Version

## First Time Setup

```bash
cd scripts/demo-performance
./demo-setup.sh
```

## Run Demo

```bash
./demo-run.sh
```

Open browser to: **http://localhost:9090**

## What You'll See

### Terminal
- Progress bars for OLD vs NEW
- Real-time metrics
- Improvement summary

### Browser
- Split-screen comparison
- Big numbers: **60% FASTER**
- Live updating charts

## Clean Up

```bash
./demo-reset.sh
```

## Advanced Options

```bash
# Load test with 20 concurrent users
./demo-load-test.sh
```

---

## Demo Checklist

Before presenting:
- [ ] Run `./demo-setup.sh` once
- [ ] Test with `./demo-run.sh`
- [ ] Open browser tab to http://localhost:9090
- [ ] Read PRESENTATION_GUIDE.md

During demo:
- [ ] Start script: `./demo-run.sh`
- [ ] Point to terminal metrics
- [ ] Switch to browser for visuals
- [ ] Explain business value
- [ ] Answer questions

After demo:
- [ ] Send summary email (template in guide)
- [ ] Clean up: `./demo-reset.sh`
- [ ] Update documentation

---

## Key Talking Points

**Technical**: "Connection pooling + query batching"

**User-Focused**: "60% faster response times"

**Business**: "3x more users, same infrastructure"

**Cost**: "Lower database costs, better reliability"

---

## Emergency Backup

If demo fails, show these metrics:

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Avg Response | 250ms | 100ms | 60% faster |
| Throughput | 1x | 3x | 3x capacity |
| Peak Users | 100 | 300 | 3x more |

**Key message**: "Backend optimizations = real business value"
