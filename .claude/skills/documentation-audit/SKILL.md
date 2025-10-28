---
name: documentation-audit
description: Systematic documentation audit that validates every documentation claim against code and identifies undocumented features - executable as a repeatable Claude Code skill
---

# Documentation Audit Skill

## Purpose

This skill performs systematic documentation audits to ensure:
1. **Accuracy**: Every claim in documentation matches actual code behavior (90-95% of effort)
2. **Completeness**: All features, protocols, and behaviors in code are documented
3. **Quality**: Basic formatting/link/spelling checks (5% of effort)

## Core Workflow

### Forward Pass: Documentation → Code Validation (70% of work)

**Goal**: Verify every claim in documentation is accurate

**Process**:
1. Create TODO for each documentation section
2. Read documentation section carefully
3. Extract factual claims (what does the doc SAY the code does?)
4. Read relevant code to verify each claim
5. Update documentation if claims are inaccurate or outdated

**Example**:
```
Doc says: "Manager.Create() returns an error if the role already exists"
→ Read pkg/relay/session/manager.go Create() method
→ Verify error handling for duplicate roles
→ Update doc if behavior differs
```

### Reverse Pass: Code → Documentation Gap Finding (25% of work)

**Goal**: Find features/behaviors in code that aren't documented

**Process**:
1. Create TODO for each Go package/major file
2. Read the code to understand what it does
3. Identify key features, protocols, state machines, error handling
4. Check if these behaviors are explained in documentation
5. Add documentation for undocumented features

**Example**:
```
Read: pkg/relay/session/manager.go
Find: SessionClockAdapter for time injection
Check: Is this pattern documented?
→ If no, add section to docs/TESTING.md
```

### Quick Quality Checks (5% of work)

**Goal**: Catch obvious issues with minimal effort

**Process**:
- Run markdownlint to check formatting
- Run link checker for broken links
- Quick spelling check with codespell

## How to Use This Skill

### Step 1: List All Documentation Sections

Create a TODO for each major section that makes factual claims:

```bash
# List all markdown files
find . -name "*.md" -not -path "*/vendor/*" -not -path "*/.git/*"

# For each file, identify sections:
# - README: Quick Start, Environment Variables, Architecture, etc.
# - ARCHITECTURE: Phase 1, Long-term, Components
# - SESSION_LIFECYCLE: States, Transitions, Cleanup
# etc.
```

Create one TODO per section to validate.

### Step 2: Validate Each Section Against Code

For each TODO:
1. Read the documentation section
2. List every factual claim it makes
3. Use Read/Grep/Glob to find relevant code
4. Verify claim is accurate
5. Update docs if wrong/outdated

### Step 3: List All Code Packages/Files

```bash
# List Go packages
go list ./...

# Or list key files
find pkg cmd -name "*.go" -not -name "*_test.go"
```

Create TODO for each package/file to check for documentation gaps.

### Step 4: Find Undocumented Features

For each TODO:
1. Read the code file/package
2. Understand what it does
3. Ask: "Is this behavior explained anywhere in docs?"
4. If no, add documentation

### Step 5: Quick Quality Checks

Run automated checks (optional, least important):
```bash
# Markdown linting
markdownlint '**/*.md' --ignore .worktrees

# Link checking
lychee '**/*.md'

# Spelling
codespell docs/ *.md
```

## What to Validate

### Types of Claims to Check

1. **Existence Claims**
   - "The Manager type provides session lifecycle management"
   - "Use OUROCODUS_ACP_BINARY to specify custom ACP binary"
   - → Verify type/variable exists

2. **Behavior Claims**
   - "Create() returns error if role already exists"
   - "Sessions progress through CREATED → SPAWNING → ACTIVE → TERMINATING → CLEANED"
   - → Read code to verify behavior

3. **Configuration Claims**
   - "Default timeout is 30 seconds"
   - "ANTHROPIC_API_KEY is required"
   - → Check default values in code

4. **API/Protocol Claims**
   - "Messages use JSON-RPC format"
   - "WebSocket handshake returns connection:established"
   - → Verify message formats and protocol details

5. **Example Claims**
   - "Run `make build` to compile"
   - "Use `mise run smoke` for smoke tests"
   - → Verify commands actually work

### What Counts as Undocumented

Code features that should be documented but aren't:

1. **Public APIs**: Exported functions, types, methods
2. **Configuration**: Environment variables, config files, flags
3. **Protocols**: Message formats, state machines, handshakes
4. **Patterns**: Dependency injection, testing patterns, error handling
5. **Workflows**: Build process, deployment, development setup
6. **Architecture**: Major design decisions, component relationships

## Tools You Have

**No fancy automation needed**. Use your normal tools:

- **Read**: Read documentation and code files
- **Grep**: Search for specific functions, types, patterns
- **Glob**: Find files matching patterns
- **Edit/Write**: Update documentation

## Output

**DO NOT generate reports**. Instead:
- Update documentation files directly as you find issues
- Keep TODO list updated with progress
- Summarize findings at the end (brief, conversational)

## Example Workflow

```
User: Run documentation audit

Claude:
1. Lists all doc files and creates TODOs for major sections
2. For each section:
   - Reads doc section
   - Extracts claims
   - Reads relevant code
   - Updates docs if inaccurate
3. Lists all Go packages and creates TODOs
4. For each package:
   - Reads code
   - Identifies features/behaviors
   - Checks if documented
   - Adds missing documentation
5. Runs quick quality checks (optional)
6. Summarizes findings conversationally
```

## Anti-Patterns (What NOT to Do)

❌ **Don't generate reports** - Update docs directly instead
❌ **Don't build NLP extraction pipelines** - Just read and validate manually
❌ **Don't focus on formatting/linting** - That's only 5% of the work
❌ **Don't batch updates** - Fix docs as you find issues
❌ **Don't skip code reading** - Every claim must be verified against code

## Summary

This skill is simple:
1. Read doc → Read code → Verify match → Fix docs
2. Read code → Check if documented → Add docs
3. Quick automated checks (least important)

90-95% of effort is reading and validating, not automation.