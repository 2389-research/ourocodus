# MDAP Documentation

This directory contains all MDAP (Massively Decomposed Agentic Processes) design documentation for the Ourocodus project.

## Primary Documents

### MDAP_IMPLEMENTATION_GUIDE.md

**The single source of truth** for MDAP implementation in Ourocodus.

This comprehensive guide consolidates all MDAP design work and provides complete documentation from principles through Milestone 5 implementation.

**Contents:**
- **Part 1: Foundations** - Core MDAP principles applied to coding
  - Rubric-based verification system
  - Verification hierarchy (L0/L1/L2/L3)
  - Philosophical foundation

- **Part 2: Architecture** - System design
  - Agent registry with schemas
  - Workflow definition format
  - Executor design with verification
  - Human steering integration
  - Voting strategies

- **Part 3: Milestone 5 Implementation** - Complete M5 roadmap
  - All 11 GitHub issues with detailed implementations
  - Week-by-week implementation plan
  - SQLite schemas and Go code examples

- **Part 4: Reference** - Practical guides
  - Complete workflow examples
  - Testing strategy
  - NATS subject design
  - Idempotency patterns
  - Deferred features

**Status:** Complete (3,595 lines)
**Created:** 2025-11-19

### CONSOLIDATION_PLAN.md

Design document explaining the consolidation strategy that produced MDAP_IMPLEMENTATION_GUIDE.md.

**Purpose:** Documents the analysis of overlapping source documents and the decision to consolidate into a single comprehensive guide.

## Archive

The `archive/` directory contains the original source documents that were consolidated:

1. `2025-11-19-mdap-milestone5-mapping.md` (1,581 lines) - M5 implementation roadmap
2. `2025-11-19-retry-and-approval-design.md` (1,053 lines) - Retry logic and approval gates
3. `2025-11-19-mdap-principles-audit.md` (918 lines) - Design audit against MDAP principles
4. `2025-11-19-mdap-for-coding.md` (1,228 lines) - MDAP adaptation for coding with rubrics

These documents are preserved for reference but have been superseded by MDAP_IMPLEMENTATION_GUIDE.md.

## Quick Start

1. **New to MDAP?** Start with MDAP_IMPLEMENTATION_GUIDE.md Part 1 (Foundations)
2. **Implementing M5?** Go to MDAP_IMPLEMENTATION_GUIDE.md Part 3 (M5 Implementation)
3. **Need code examples?** See MDAP_IMPLEMENTATION_GUIDE.md Part 4 (Reference)
4. **Understanding consolidation?** Read CONSOLIDATION_PLAN.md

## Key Concepts

### MDAP Core Insight

**MDAPs succeed not because steps are small, but because each step is *verifiable*.**

For coding: **Verification = rubrics, tests, constraints, specs**

### Milestone 5 Goals

- Sequential workflow execution with dependency management
- Crash recovery and idempotency guarantees
- Retry logic (2-layer design: transport + task)
- Approval gates for human-in-the-loop workflows
- NATS-based async coordination
- SQLite persistence
- HTTP API with authentication
- Full observability and debugging

### Architecture Highlights

- **Coordinator Service:** Orchestrates workflow execution
- **Workflow Engine:** Sequential execution (MVP), prepared for future parallelism
- **Agent Registry:** Schema-validated agents (deferred to M6)
- **Verification System:** L0 (syntax) and L1 (semantic) checks
- **Human Steering:** Pre/mid/post workflow integration points

## Implementation Timeline

**Milestone 5 (4 weeks):**
- Week 1: Foundation (#121, #122)
- Week 2: Parser + Executor (#126, #127)
- Week 3: NATS + API + Retry (#124, #123, #128, #59)
- Week 4: Operations + Observability + Approval (#129, #125, #53)

## Related Documents

- `../../CONTRIBUTING.md` - Development guidelines
- `../../../README.md` - Project overview
- GitHub Milestone 5: https://github.com/2389-research/ourocodus/milestone/6

## Questions?

For questions about MDAP implementation, refer to MDAP_IMPLEMENTATION_GUIDE.md or create a GitHub issue.

---

**Last Updated:** 2025-11-19
**Maintained By:** Ourocodus Team
