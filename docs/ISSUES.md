# GitHub Issues - Phase 1 Implementation

## Overview

Phase 1 is broken down into 13 iterative issues, ordered by dependency. Work top-down for optimal progress.

## Issue Dependency Graph

```
#1 Project Initialization (no deps)
  ├─> #2 Git Worktree Script
  ├─> #3 ACP Client Package
  │    └─> #4 ACP Client Testing
  └─> #5 Relay WebSocket Server
       └─> #6 Relay Session Management
            └─> #7 Relay ACP Integration (needs #3, #4, #6)
                 ├─> #8 Relay Routing: WS → ACP
                 │    └─> #9 Relay Routing: ACP → WS
                 │         └─> #12 PWA Chat Interface (needs #11)
                 └─> #10 PWA Scaffold (needs #5, #6)
                      └─> #11 PWA Agent Cards
                           └─> #12 PWA Chat Interface (needs #9)

#13 End-to-End Testing (needs #2, #9, #12)
```

## Critical Path

The critical path (longest dependency chain) is:
```
#1 → #5 → #6 → #7 → #8 → #9 → #12 → #13
```

This path takes approximately **32-42 hours** of work.

## Parallel Work Opportunities

After completing foundations, these can be worked in parallel:

**After #7 (Relay ACP Integration):**
- Track A: #8 → #9 (Relay routing)
- Track B: #10 → #11 (PWA scaffold and cards)

**Merge point:** #12 (PWA Chat) requires both tracks complete

## Issue Labels

- **component:*** - Which part of system (setup, acp, relay, pwa, git)
- **type:*** - What kind of work (feature, test, integration)
- **priority:*** - Importance (high=critical path, medium, low)

## Time Estimates

| Issue # | Title | Estimate | Cumulative |
|---------|-------|----------|------------|
| #1 | Project Initialization | 4-6h | 4-6h |
| #2 | Git Worktree Script | 2-3h | 6-9h |
| #3 | ACP Client Package | 6-8h | 12-17h |
| #4 | ACP Client Testing | 4-6h | 16-23h |
| #5 | Relay WebSocket Server | 4-6h | 20-29h |
| #6 | Relay Session Management | 3-4h | 23-33h |
| #7 | Relay ACP Integration | 4-6h | 27-39h |
| #8 | Relay Routing: WS→ACP | 4-5h | 31-44h |
| #9 | Relay Routing: ACP→WS | 3-4h | 34-48h |
| #10 | PWA Scaffold | 4-6h | 38-54h |
| #11 | PWA Agent Cards | 3-4h | 41-58h |
| #12 | PWA Chat Interface | 6-8h | 47-66h |
| #13 | E2E Testing | 6-8h | 53-74h |

**Total: 53-74 hours (~2 weeks at 40h/week)**

## How to Use This

1. **Start with #1** - Must complete first
2. **Work sequentially** - Check "Dependencies" in each issue
3. **Update issue** - Check off tasks as you complete them
4. **Link PRs** - Reference issue number in PR title
5. **Close with PR** - Use "Closes #N" in PR description

## Issue Templates

Each issue follows this structure:

```markdown
## Goal
[One sentence: what does this accomplish?]

## Tasks
- [ ] Specific task 1
- [ ] Specific task 2

## Acceptance Criteria
- [ ] Testable criterion 1
- [ ] Testable criterion 2

## References
[Links to docs]

## Dependencies
[Links to other issues]

## Time Estimate
[Hours range]
```

## Milestone

All issues are in the **"Phase 1: Foundation"** milestone with due date: Dec 15, 2025.

Track progress: https://github.com/2389-research/ourocodus/milestone/1

## Detailed Briefs

- [#6 Relay Session Management](issues/06-relay-session-management.md)
- [#7 Relay ACP Integration](issues/07-relay-acp-integration.md)

## Questions?

See [docs/PHASE1.md](https://github.com/2389-research/ourocodus/blob/main/docs/PHASE1.md) for detailed implementation specification.
