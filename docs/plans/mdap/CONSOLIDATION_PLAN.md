# MDAP Documents Consolidation Plan

## Current State Analysis

We have 4 documents totaling ~4,780 lines:

1. **mdap-milestone5-mapping.md** (1,581 lines) - Implementation roadmap mapping MDAP to M5 issues
2. **mdap-principles-audit.md** (918 lines) - Audit of current design against MDAP principles
3. **mdap-for-coding.md** (1,228 lines) - MDAP adaptation for coding with rubrics
4. **retry-and-approval-design.md** (1,053 lines) - Retry logic and approval gate design

## Problem: Significant Overlap and Redundancy

### Overlap Analysis

| Topic | Document 1 (M5 Mapping) | Document 2 (Audit) | Document 3 (Coding) | Document 4 (Retry) |
|-------|------------------------|-------------------|--------------------|--------------------|
| **Core MDAP principles** | ✅ Brief summary | ✅ **Detailed audit** | ✅ **Deep dive** | ❌ Not covered |
| **Verification layers (L0/L1/L2/L3)** | ✅ Mentioned | ✅ **Detailed design** | ✅ **Most comprehensive** | ❌ Not covered |
| **Schema validation** | ✅ SQLite schemas | ✅ **Design proposal** | ✅ **Full implementation** | ❌ Not covered |
| **Micro-decomposition** | ✅ Example YAML | ✅ **Problem identified** | ✅ **40-task example** | ❌ Not covered |
| **Rubric system** | ❌ Not covered | ❌ Not covered | ✅ **Complete design** | ❌ Not covered |
| **Retry logic** | ✅ Brief in #127 | ✅ Mentioned | ❌ Not covered | ✅ **Complete design** |
| **Approval gates** | ❌ Question only | ❌ Not covered | ✅ **Implementation** | ✅ **Complete design** |
| **M5 issue mapping** | ✅ **Complete (9 issues)** | ❌ Not covered | ❌ Not covered | ✅ Issues #59, #53 |
| **Week-by-week roadmap** | ✅ Included | ❌ Not covered | ✅ Alternative plan | ❌ Not covered |
| **Human steering** | ❌ Not covered | ✅ **Design points** | ✅ **Complete design** | ❌ Not covered |

### Key Redundancies

1. **Verification design appears in 3 documents:**
   - M5 Mapping: Brief mention in Issue #125
   - Audit: Action items to add verifiers
   - Coding: Complete L0/L1/L2/L3 hierarchy ← **Most authoritative**

2. **Decomposition examples appear in 3 documents:**
   - M5 Mapping: Simple 5-task workflow
   - Audit: 15-task decomposed example
   - Coding: 40-task executor implementation ← **Most thorough**

3. **Approval gates appear in 2 documents:**
   - Retry: Complete Go implementation
   - Coding: YAML spec + agent design ← **More aligned with MDAP**

4. **Idempotency discussion appears in all 4 documents:**
   - Different perspectives but same core concept

## Proposed Consolidation Structure

### Option A: Single Comprehensive Document (Recommended)

**Create:** `MDAP_IMPLEMENTATION_GUIDE.md` (~2,500 lines)

**Structure:**
```markdown
# MDAP Implementation Guide for Ourocodus

## Part 1: Foundations (from Audit + Coding)
- Core MDAP principles applied to coding
- Rubric-based verification (the breakthrough)
- Verification hierarchy (L0/L1/L2/L3)
- Micro-decomposition philosophy

## Part 2: Architecture (from Coding + Audit)
- Agent registry with schemas
- Workflow definition format
- Executor design
- Human steering integration points
- Voting for ambiguity

## Part 3: Milestone 5 Implementation (from M5 Mapping + Retry)
### Issue-by-Issue Implementation
- #121: Coordinator Service Foundation
- #122: Workflow Persistence (SQLite)
- #126: Workflow Parser
- #127: Sequential Executor (includes retry from doc 4)
- #124: NATS Event Handlers
- #123: HTTP API
- #128: Task Lifecycle
- #129: Workflow Operations
- #125: Observability

### Special Features (from Retry + Coding)
- Issue #59: Retry Logic (2-layer design)
- Issue #53: Approval Gates

### Week-by-Week Roadmap
- Week 1: Foundation + Rubric system
- Week 2: Parser + Executor + Verification
- Week 3: NATS + API + Retry
- Week 4: Operations + Human steering

## Part 4: Reference
- Complete workflow examples
- Schema definitions
- Agent library (20-30 primitives)
- NATS subject design
- Testing strategy
```

**Benefits:**
- Single source of truth
- Clear narrative arc: principles → architecture → implementation
- No duplication
- Easy to maintain

**Drawbacks:**
- Long document (~2,500 lines)
- Harder to navigate without good TOC

---

### Option B: Layered Document Set (Alternative)

**Keep 3 documents, delete 1:**

1. **MDAP_PRINCIPLES.md** (merge Audit + Coding foundations)
   - Core principles applied to coding
   - Rubric-based verification
   - Verification hierarchy
   - Philosophical foundation
   - ~600 lines

2. **MDAP_ARCHITECTURE.md** (extract from Coding)
   - Agent registry design
   - Workflow definition format
   - Executor design
   - Human steering
   - Voting strategies
   - ~600 lines

3. **MILESTONE5_IMPLEMENTATION.md** (merge M5 Mapping + Retry)
   - Issue-by-issue implementation guide
   - Retry logic (#59)
   - Approval gates (#53)
   - Week-by-week roadmap
   - Complete code examples
   - ~1,300 lines

**Delete:** `mdap-principles-audit.md` (content absorbed into PRINCIPLES and ARCHITECTURE)

**Benefits:**
- Logical separation of concerns
- Easier to read individual documents
- Can link between documents

**Drawbacks:**
- Risk of future divergence
- Need to maintain cross-references
- Overlap still possible

---

### Option C: Keep Current, Add Index (Minimal)

**Keep all 4 documents, create:**

**MDAP_INDEX.md** - Navigation guide

```markdown
# MDAP Documentation Index

## Start Here
- New to MDAP? Read **mdap-for-coding.md** first
- Implementing M5? Go to **mdap-milestone5-mapping.md**
- Need retry design? See **retry-and-approval-design.md**

## By Topic

### Principles & Philosophy
- Rubric-based verification: **mdap-for-coding.md** (lines 1-200)
- MDAP vs traditional: **mdap-principles-audit.md** (lines 1-100)

### Verification
- L0/L1/L2/L3 hierarchy: **mdap-for-coding.md** (lines 201-500)
- Implementation checklist: **mdap-principles-audit.md** (lines 300-400)

### Retry Logic
- Two-layer design: **retry-and-approval-design.md** (Part 1)
- Integration with executor: **mdap-milestone5-mapping.md** (Issue #127)

### Approval Gates
- Complete design: **retry-and-approval-design.md** (Part 2)
- YAML spec: **mdap-for-coding.md** (lines 800-900)

[etc...]
```

**Benefits:**
- No restructuring needed
- Preserves all context
- Index clarifies navigation

**Drawbacks:**
- Redundancy remains
- Maintenance burden increases
- Risk of documents drifting

---

## Recommendation: Option A (Single Comprehensive Document)

### Why?

1. **Clear narrative arc:** Principles → Architecture → Implementation
2. **No duplication:** Each concept covered once, thoroughly
3. **Single source of truth:** No ambiguity about which document is authoritative
4. **Easier maintenance:** One file to update when plans change
5. **Better for newcomers:** One document to read for complete picture

### Migration Strategy

1. **Extract best content from each document:**
   - Principles: Take from `mdap-for-coding.md` (most thorough)
   - Verification: Take from `mdap-for-coding.md` (most complete)
   - M5 mapping: Take from `mdap-milestone5-mapping.md` (most detailed)
   - Retry: Take from `retry-and-approval-design.md` (most complete)
   - Approval: Merge both approaches (Retry has Go code, Coding has YAML)

2. **Resolve conflicts:**
   - Decomposition: Use 40-task example from Coding (most thorough)
   - Roadmap: Merge both week-by-week plans
   - Schemas: Use M5 Mapping SQLite schemas (most detailed)

3. **Create new document:**
   - `MDAP_IMPLEMENTATION_GUIDE.md`

4. **Archive old documents:**
   - Move to `docs/plans/mdap/archive/` for reference

---

## Detailed Consolidation Mapping

### Section 1: Foundations

| Content | Source Document | Lines | Keep? |
|---------|----------------|-------|-------|
| Core insight (verifiability) | mdap-for-coding.md | 1-50 | ✅ PRIMARY |
| Philosophical foundation | mdap-for-coding.md | 51-100 | ✅ Keep |
| Tower of Hanoi vs Coding | mdap-for-coding.md | 101-150 | ✅ Keep |
| Rubric system design | mdap-for-coding.md | 151-300 | ✅ Keep |
| 10 MDAP principles | mdap-principles-audit.md | 1-800 | ⚠️ Summarize (too verbose) |

**Recommendation:** Use Coding as primary source (clearer, more concise). Add principle summaries from Audit.

---

### Section 2: Architecture

| Content | Source Document | Lines | Keep? |
|---------|----------------|-------|-------|
| Verification hierarchy (L0-L3) | mdap-for-coding.md | 301-600 | ✅ PRIMARY |
| Agent registry design | mdap-for-coding.md | 601-700 | ✅ Keep |
| Workflow definition format | mdap-principles-audit.md | 200-400 | ⚠️ Merge with Coding |
| Human steering integration | mdap-for-coding.md | 701-900 | ✅ Keep |
| Voting strategies | mdap-for-coding.md | 901-1000 | ✅ Keep |
| Micro-agent library | mdap-for-coding.md | 1001-1100 | ✅ Keep |

**Recommendation:** Use Coding as backbone, supplement with Audit's action items.

---

### Section 3: Milestone 5 Implementation

| Content | Source Document | Lines | Keep? |
|---------|----------------|-------|-------|
| Issue #121 (Coordinator) | mdap-milestone5-mapping.md | 30-90 | ✅ Keep (most detailed) |
| Issue #122 (SQLite) | mdap-milestone5-mapping.md | 96-194 | ✅ Keep (has schemas) |
| Issue #126 (Parser) | mdap-milestone5-mapping.md | 197-313 | ✅ Keep |
| Issue #127 (Executor) | mdap-milestone5-mapping.md | 316-494 | ✅ Keep |
| Issue #59 (Retry) | retry-and-approval-design.md | 1-400 | ✅ Keep (most complete) |
| Issue #53 (Approval) | retry-and-approval-design.md | 401-800 | ⚠️ Merge with Coding YAML |
| Issue #124 (NATS) | mdap-milestone5-mapping.md | 638-797 | ✅ Keep |
| Issue #123 (API) | mdap-milestone5-mapping.md | 800-964 | ✅ Keep |
| Issue #128 (Monitoring) | mdap-milestone5-mapping.md | 511-630 | ✅ Keep |
| Issue #129 (Operations) | mdap-milestone5-mapping.md | 975-1114 | ✅ Keep |
| Issue #125 (Observability) | mdap-milestone5-mapping.md | 1117-1282 | ✅ Keep |

**Recommendation:** Use M5 Mapping as primary, insert Retry doc content into #127 and #53.

---

### Section 4: Examples & Reference

| Content | Source Document | Lines | Keep? |
|---------|----------------|-------|-------|
| 40-task executor example | mdap-for-coding.md | 400-1000 | ✅ Keep (most thorough) |
| Simple 5-task workflow | mdap-milestone5-mapping.md | 1477-1547 | ⚠️ Keep as "simple example" |
| Week-by-week roadmap | mdap-milestone5-mapping.md | 1329-1359 | ✅ Merge with Coding roadmap |
| Testing strategy | mdap-milestone5-mapping.md | 1363-1382 | ✅ Keep |
| NATS subject design | mdap-milestone5-mapping.md | 789-797 | ✅ Keep |
| Idempotency patterns | retry-and-approval-design.md | 900-1053 | ✅ Keep (comprehensive) |

**Recommendation:** Combine examples (simple + complex), use M5 Mapping testing strategy.

---

## Action Plan

### Step 1: Create New Document Structure
```bash
# Create consolidated document
touch docs/plans/mdap/MDAP_IMPLEMENTATION_GUIDE.md
```

### Step 2: Extract and Merge Content

**Part 1: Foundations (500 lines)**
- [ ] Copy core insight from mdap-for-coding.md (lines 1-50)
- [ ] Copy philosophical foundation (lines 51-150)
- [ ] Copy rubric system design (lines 151-300)
- [ ] Summarize 10 principles from mdap-principles-audit.md

**Part 2: Architecture (600 lines)**
- [ ] Copy verification hierarchy from mdap-for-coding.md (lines 301-600)
- [ ] Copy agent registry design (lines 601-700)
- [ ] Copy human steering (lines 701-900)
- [ ] Copy voting strategies (lines 901-1000)
- [ ] Copy micro-agent library (lines 1001-1100)

**Part 3: Milestone 5 Implementation (1,200 lines)**
- [ ] Copy all 9 M5 issues from mdap-milestone5-mapping.md
- [ ] Insert retry logic from retry-and-approval-design.md into Issue #127
- [ ] Insert approval gates from retry-and-approval-design.md into new section
- [ ] Merge week-by-week roadmaps

**Part 4: Reference (200 lines)**
- [ ] Copy 40-task example from mdap-for-coding.md
- [ ] Copy simple example from mdap-milestone5-mapping.md
- [ ] Copy testing strategy
- [ ] Copy NATS subject design
- [ ] Copy idempotency patterns from retry-and-approval-design.md

### Step 3: Archive Old Documents
```bash
mkdir -p docs/plans/mdap/archive
mv docs/plans/mdap/2025-11-19-*.md docs/plans/mdap/archive/
```

### Step 4: Update References
- [ ] Update any links in other docs to point to new guide
- [ ] Create README.md in mdap/ folder explaining structure

---

## Estimated Consolidation Size

**Target:** ~2,500 lines (down from 4,780)

**Breakdown:**
- Part 1 (Foundations): 500 lines
- Part 2 (Architecture): 600 lines
- Part 3 (M5 Implementation): 1,200 lines
- Part 4 (Reference): 200 lines

**Savings:** ~2,280 lines (48% reduction) through de-duplication

---

## Timeline

**Estimated time:** 2-3 hours

**Breakdown:**
- Structure creation: 30 min
- Content extraction: 1 hour
- Conflict resolution: 30 min
- Review and polish: 30 min
- Archive and cleanup: 30 min

---

## Decision

**Recommendation:** Proceed with Option A (Single Comprehensive Document)

**Rationale:**
1. Clearest for implementation teams
2. Eliminates all redundancy
3. Creates authoritative reference
4. Easier to maintain long-term
5. Better narrative flow

**Alternative:** If document feels too long after consolidation, can split into Option B (3 documents) later.

**Next step:** Get user approval, then execute consolidation plan.
