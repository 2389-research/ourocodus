---
name: documentation-audit
description: Comprehensive documentation audit that verifies accuracy, identifies outdated content, ensures completeness for different audiences, and maintains optimal documentation volume - executable as a repeatable Claude Code skill
---

# Documentation Audit Skill

## Purpose

This skill performs systematic documentation audits to ensure:
1. **Accuracy**: All documentation claims match actual code behavior
2. **Currency**: No outdated or obsolete information
3. **Completeness**: Important aspects documented for all audiences (users, contributors, maintainers)
4. **Balance**: "Just right" documentation volume - not too much, not too little

## When to Use

- **Regular maintenance**: Run quarterly or before major releases
- **After significant refactoring**: Verify docs still match code
- **Pre-release**: Ensure all user-facing changes documented
- **Onboarding issues**: When new contributors struggle with docs
- **Manual trigger**: When requested by maintainers

## How It Works

This skill uses a **phased approach** with increasing sophistication:

### Phase 1: Quick Wins (Automated Checks)
Fast automated checks that catch common issues:
- Broken links and anchors
- Markdown formatting issues
- Spelling errors
- Stale code examples
- Basic doc coverage metrics

### Phase 2: Deep Analysis (Code-Driven Audit)
Sophisticated analysis comparing code reality to documentation claims:
- Extract "ground truth" from code (CLI flags, config, APIs)
- Parse documentation claims
- Cross-validate for accuracy drift
- Check completeness by audience
- Identify over/under-documentation

### Phase 3: Continuous Integration
Automation to prevent future drift:
- CI checks on PRs
- Scheduled audits
- Auto-generation of reference docs
- Trend tracking

## Workflow Steps

### Step 1: Collect Inventory

**Goal**: Discover all documentation and build code index

**Actions**:
```bash
cd .claude/skills/documentation-audit/scripts
./collect-inventory.sh
```

**Discovers**:
- Documentation files: README.md, CONTRIBUTING.md, docs/**/*.md, doc.go files
- Code structure: packages, exported APIs, CLI commands, config structs
- Current metrics: doc coverage, test count, link health

**Output**: `reports/inventory.json`

### Step 2: Run Quick Checks

**Goal**: Fast automated validation

**Actions**:
```bash
./run-quick-checks.sh
```

**Checks**:
- **Links**: Broken URLs, invalid anchors (via lychee)
- **Markdown**: Formatting, style consistency (via markdownlint)
- **Spelling**: Typos, common errors (via codespell)
- **Examples**: Go Example tests compile and run (via go test)
- **Coverage**: Exported symbol documentation (via golangci-lint)

**Output**: `reports/quick-checks-report.md`

### Step 3: Extract Code Facts (Phase 2+)

**Goal**: Build source of truth from code

**What it extracts**:
- **CLI flags**: From flag/pflag usage in cmd/
- **Config keys**: From struct tags and defaults
- **Exported APIs**: Public types, functions, methods
- **Environment variables**: os.Getenv patterns
- **Version requirements**: go.mod, build constraints

**Output**: `reports/code-facts.json`

### Step 4: Extract Doc Claims (Phase 2+)

**Goal**: Parse what documentation claims

**What it parses**:
- CLI reference sections
- Configuration documentation
- API documentation
- Quickstart/tutorial code blocks
- Compatibility statements

**Output**: `reports/doc-claims.json`

### Step 5: Cross-Validate (Phase 2+)

**Goal**: Find accuracy drift and completeness gaps

**Validation checks**:
- **Accuracy**: Code facts vs. doc claims
- **Completeness**: Required sections by audience
- **Volume**: Over/under-documentation heuristics

**Output**: `reports/validation-report.md` (with severity levels)

### Step 6: Generate Recommendations

**Goal**: Actionable remediation steps

**Provides**:
- **Blockers**: Must-fix accuracy issues (fail CI)
- **Warnings**: Completeness gaps
- **Suggestions**: Volume/duplication improvements
- **Auto-fixes**: Patches and generated content

**Output**: `reports/recommendations.md` + patches in `artifacts/`

## Integration with Zen

This skill is **designed for zen's multi-step workflows**:

```
Use zen's debug, analyze, or thinkdeep tools to:
1. Investigate complex documentation drift issues
2. Design documentation structure improvements
3. Plan large-scale documentation refactoring
4. Validate remediation approaches
```

**Example zen workflow**:
```
User: "Our API documentation seems out of sync"

Claude: I'll use the documentation-audit skill to investigate.
Step 1: Running inventory and quick checks...
Step 2: Extracting code facts from exported APIs...
Step 3: Using zen's analyze tool to cross-validate API docs against code...
Step 4: Generating prioritized remediation plan...
```

## What to Document vs. Skip

### ALWAYS DOCUMENT

**Code Elements**:
- Exported APIs (types, functions, methods, constants)
- CLI flags, subcommands, arguments
- Configuration keys, defaults, env vars
- HTTP endpoints, request/response formats
- Non-obvious invariants and contracts
- Concurrency patterns and goroutine usage
- Performance-critical paths and optimization notes
- Security-sensitive behavior
- Error handling patterns
- Public contracts and guarantees

**User Content**:
- Installation instructions
- Quickstart guide
- Common usage patterns
- Configuration examples
- Troubleshooting guide
- Migration guides (for breaking changes)
- FAQ

**Contributor Content**:
- Development setup
- Building and testing
- Code organization
- Contributing guidelines
- PR process and expectations
- Linting and formatting requirements
- Release process

**Maintainer Content**:
- Architecture decisions (ADRs)
- Security policy and reporting
- Release checklist
- Dependency management policy
- Backport policies
- Support lifecycle

### SKIP OR MINIMIZE

- Trivial getters/setters with obvious behavior
- Self-explanatory code with clear names
- Auto-generated code internals
- Implementation details of private functions
- Duplicated content (use links to canonical source)
- Obvious patterns (e.g., standard Go idioms)

## Documentation Volume Heuristics

### Target Metrics
- **Exported symbols**: ≥85% documented
- **Top-level packages**: 100% have package docs
- **Example tests**: ≥3 per major feature
- **CLI commands**: 100% documented
- **Config keys**: 100% documented

### Signs of Over-Documentation
- Same concept explained in 3+ places
- Line-by-line explanation of obvious code
- Documentation longer than the code it describes
- Repeating information available in godoc
- Excessive implementation details in user docs

### Signs of Under-Documentation
- Exported APIs with no comments
- CLI flags with empty help text
- Config keys with unclear purpose
- Missing installation instructions
- No quickstart or examples
- Implicit assumptions not stated
- No error handling guidance

## Configuration

See `config.yaml` for:
- Thresholds (coverage minimums, drift tolerance)
- Audiences to check (user, contributor, maintainer)
- Exclusions (vendor/, generated files)
- Tool paths and options

## Output Files

All outputs in `.claude/skills/documentation-audit/reports/`:
- `inventory.json` - Documentation and code index
- `quick-checks-report.md` - Phase 1 results
- `code-facts.json` - Extracted code ground truth
- `doc-claims.json` - Parsed documentation claims
- `validation-report.md` - Cross-validation findings
- `recommendations.md` - Actionable next steps
- `metrics.json` - Historical trend data

## CI Integration

### On Pull Requests
```yaml
- name: Documentation Audit
  run: |
    cd .claude/skills/documentation-audit
    ./scripts/run-quick-checks.sh
    # Fail on accuracy regressions
```

### Scheduled (Weekly)
```yaml
- name: Full Documentation Audit
  run: |
    cd .claude/skills/documentation-audit
    ./scripts/run-full-audit.sh
    # Generate report, track trends
```

## Maintenance

- **Weekly**: Review validation reports
- **Monthly**: Update thresholds based on trends
- **Quarterly**: Full deep audit with manual review
- **Per-release**: Mandatory audit before tagging

## Tool Dependencies

**Required** (Phase 1):
- `go` - Code analysis
- `golangci-lint` - Doc coverage
- Standard unix tools: grep, awk, jq

**Recommended** (better Phase 1):
- `lychee` - Link checking (install: `cargo install lychee`)
- `markdownlint-cli` - Markdown linting (install: `npm install -g markdownlint-cli`)
- `codespell` - Spell checking (install: `pip install codespell`)

**Optional** (Phase 2):
- `embedmd` - Code snippet sync (install: `go install github.com/campoy/embedmd@latest`)

## Examples

### Run Quick Audit (Phase 1)
```bash
cd .claude/skills/documentation-audit
./scripts/run-quick-checks.sh
cat reports/quick-checks-report.md
```

### Full Audit (Phase 2)
```bash
./scripts/run-full-audit.sh
cat reports/validation-report.md
cat reports/recommendations.md
```

### Check Specific Package
```bash
./scripts/check-package.sh ./pkg/session
```

### Generate Baseline
```bash
./scripts/collect-inventory.sh
cat reports/inventory.json | jq '.metrics'
```

## Troubleshooting

**Issue**: Script fails with "command not found"
- **Solution**: Check tool dependencies, install missing tools

**Issue**: Too many false positives
- **Solution**: Tune config.yaml thresholds, add exclusions

**Issue**: Reports say everything is fine but docs feel wrong
- **Solution**: Phase 1 only does basic checks. Run Phase 2 deep analysis or use zen's analyze tool

**Issue**: Overwhelming backlog of issues
- **Solution**: Start with blockers only, use .docauditignore for legacy exceptions, ratchet thresholds gradually

## Next Steps After Audit

1. **Triage findings**: Blockers → Warnings → Suggestions
2. **Quick wins**: Fix broken links, spelling, formatting first
3. **Accuracy issues**: Update docs to match code (or vice versa)
4. **Completeness gaps**: Prioritize by audience impact
5. **Volume issues**: De-duplicate, simplify, or expand as needed
6. **Automation**: Set up CI checks to prevent regression

## References

- [Go Doc Comments](https://go.dev/doc/comment)
- [Kubernetes Documentation Style Guide](https://kubernetes.io/docs/contribute/style/style-guide/)
- [Write the Docs](https://www.writethedocs.org/guide/)
