# WebSocket Demo - Quick Reference Card

**Print this or keep it visible during your meeting**

---

## 30-Second Elevator Pitch

"I built automatic reconnection for our PWA. When users lose network connectivity, the application automatically reconnects when the network comes back - no manual refresh needed. This is the same reliability pattern used by Slack and Discord."

---

## Demo Commands

```bash
# You already ran this:
./demo-setup.sh ✓

# Run this to start demo:
./demo-run.sh

# If something breaks:
./demo-reset.sh
```

---

## 4 Key Talking Points

### 1. **User Experience** (Most Important for VP)
- Users don't need to refresh or retry manually
- Clear visual feedback (green = connected, gray = reconnecting)
- Session preserved across reconnections

### 2. **Reliability**
- Handles network blips, server restarts, WiFi switching
- Especially valuable for mobile users
- Industry-standard exponential backoff

### 3. **Business Value**
- Reduced support tickets ("why did I get disconnected?")
- Professional-grade user experience
- Foundation for real-time collaboration features

### 4. **What's Next**
- Phase 2: Session recovery (resume work after reconnection)
- Phase 3: Real-time updates and offline queue

---

## What to Show (Follow Script Pauses)

1. **Baseline**: Open http://localhost:8080, click "New Project", show green status
2. **Problem**: Server dies → status turns gray, shows "Reconnecting..."
3. **Recovery**: Server returns → status turns green automatically
4. **Resilience**: Multiple failures handled gracefully

---

## If Asked Technical Questions

**"How does it work?"**
→ WebSocket connection with automatic retry and exponential backoff

**"What if network is down for hours?"**
→ After 10 attempts (~8 min), shows "Connection failed" - user can manually refresh

**"Performance impact?"**
→ Minimal - only activates when disconnected

**"Mobile support?"**
→ Yes - especially valuable for WiFi/cellular switching

**"What's next?"**
→ Session recovery, real-time updates, offline queue

---

## If Demo Breaks

**Fallback plan:**
1. Run `./demo-reset.sh`
2. Show the code in `web/app.js` (ReconnectConnection class)
3. Talk through the feature instead of demonstrating

**Key point:** VP cares about outcomes, not perfect demos.

---

## Your Confidence Boosters

✓ This is industry-standard practice (not experimental)
✓ PR #42 already merged and reviewed
✓ Code is tested and production-ready
✓ Foundation for future real-time features
✓ Directly improves user experience

---

## After Demo

- Run `./demo-reset.sh` to clean up
- Share DEMO-GUIDE.md if VP wants details
- Offer to demo again for other stakeholders

---

**Remember:** You focus on explaining WHY it matters. The script handles WHAT happens.

**Good luck!**
