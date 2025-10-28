# Documentation Audit Skill

A comprehensive Claude Code skill for auditing and maintaining documentation quality in Go projects.

## Overview

This skill performs systematic documentation audits to ensure:
- **Accuracy**: Documentation matches actual code behavior
- **Currency**: No outdated or obsolete information
- **Completeness**: Appropriate documentation for all audiences
- **Balance**: "Just right" documentation volume

## Quick Start

### Prerequisites

**Required** (for basic functionality):
- Go 1.21+
- golangci-lint (install via `mise install` or see [installation](https://golangci-lint.run/usage/install/))

**Recommended** (for full Phase 1 checks):
```bash
# Link checking
cargo install lychee

# Markdown linting
npm install -g markdownlint-cli

# Spell checking
pip install codespell
```

### Running Your First Audit

```bash
# From repository root
cd .claude/skills/documentation-audit

# Run Phase 1 quick checks
./scripts/run-quick-checks.sh

# View the report
cat reports/quick-checks-report.md
```

## Usage

### Phase 1: Quick Checks (Current)

Fast automated checks that catch common issues:

```bash
# Full quick audit
./scripts/run-quick-checks.sh

# Just inventory
./scripts/collect-inventory.sh
```

**What Phase 1 Checks:**
- ✓ Broken links and anchors (via lychee)
- ✓ Markdown formatting issues (via markdownlint)
- ✓ Spelling errors (via codespell)
- ✓ Go Example test validation
- ✓ Documentation coverage of exported symbols (via golangci-lint)

### Phase 2: Deep Analysis (Coming Soon)

Code-driven analysis comparing code reality to documentation claims:
- Extract CLI flags, config keys, APIs from code
- Parse documentation claims
- Cross-validate for accuracy drift
- Check completeness by audience
- Identify over/under-documentation

**Status**: Phase 2 implementation coming in next iteration

### With Claude Code

From anywhere in your project, just ask Claude:

```
"Run a documentation audit"
"Check if our docs are up to date"
"Validate documentation quality"
```

Claude will automatically:
1. Use this skill
2. Run appropriate checks
3. Generate a report
4. Suggest fixes

## Configuration

Edit `config.yaml` to customize:

```yaml
thresholds:
  doc_coverage_min: 85          # Minimum % of documented exports
  drift_fail: true              # Fail CI on accuracy issues
  broken_links_max: 0           # Maximum allowed broken links

audiences:                      # Check completeness for:
  - user
  - contributor
  - maintainer

exclusions:                     # Paths to skip
  - "vendor/**"
  - "**/*.pb.go"
```

See [config.yaml](config.yaml) for all options.

## Output

All reports are generated in `reports/`:

- **quick-checks-report.md** - Human-readable findings from Phase 1
- **quick-checks-report.json** - Machine-readable results for CI
- **inventory.json** - Full documentation and code inventory

### Sample Report Structure

```markdown
# Documentation Audit - Quick Checks Report

## ✓ Inventory Collection
Status: PASSED
Successfully collected documentation and code inventory.

## ⚠ Link Checking
Status: WARNING
Found 3 broken link(s). These should be fixed or removed.

## Summary
Total Checks: 6
Passed: 4 ✓
Warnings: 2 ⚠
Failed: 0 ✗
```

## Integration

### CI Integration

**GitHub Actions** example:

```yaml
name: Documentation Audit

on: [pull_request, push]

jobs:
  docs-audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Install Tools
        run: |
          cargo install lychee
          npm install -g markdownlint-cli
          pip install codespell

      - name: Run Documentation Audit
        run: |
          cd .claude/skills/documentation-audit
          ./scripts/run-quick-checks.sh

      - name: Upload Report
        if: always()
        uses: actions/upload-artifact@v3
        with:
          name: doc-audit-report
          path: .claude/skills/documentation-audit/reports/
```

### Pre-commit Hook

Add to `.pre-commit-config.yaml`:

```yaml
- repo: local
  hooks:
    - id: doc-audit
      name: Documentation Audit
      entry: .claude/skills/documentation-audit/scripts/run-quick-checks.sh
      language: script
      pass_filenames: false
      stages: [manual]  # Run with: pre-commit run doc-audit
```

## Documentation Guidelines

### What to Document

**Always Document:**
- Exported APIs (types, functions, methods)
- CLI flags and commands
- Configuration options
- Non-obvious behavior
- Security considerations
- Performance implications

**Skip:**
- Trivial getters/setters
- Self-explanatory code
- Implementation details of private functions

See [SKILL.md](SKILL.md#what-to-document-vs-skip) for complete criteria.

## Templates

Use provided templates for consistency:

- [README.template.md](templates/README.template.md) - Project README structure
- [CONTRIBUTING.template.md](templates/CONTRIBUTING.template.md) - Contributing guide
- [SECURITY.template.md](templates/SECURITY.template.md) - Security policy

## Troubleshooting

### "command not found" errors

Install the missing tool:

```bash
# lychee (link checker)
cargo install lychee

# markdownlint (markdown linter)
npm install -g markdownlint-cli

# codespell (spell checker)
pip install codespell
```

The skill will skip checks for missing tools and warn you.

### Too many false positives

Adjust thresholds in `config.yaml` or add exclusions:

```yaml
exclusions:
  - "docs/legacy/**"  # Skip old docs
  - "vendor/**"       # Skip dependencies

tools:
  codespell:
    ignore_words:
      - "yourtechterm"  # Add technical terms
```

### Large backlog of issues

Start with blockers only:

1. Fix broken links first (quick wins)
2. Fix spelling errors
3. Address accuracy issues
4. Tackle completeness gaps
5. Refine volume issues

Use `.docauditignore` for legacy exceptions (coming in Phase 2).

## Roadmap

### Phase 1 (✓ Current)
- [x] Inventory collection
- [x] Link checking
- [x] Markdown linting
- [x] Spell checking
- [x] Example test validation
- [x] Basic doc coverage

### Phase 2 (Planned)
- [ ] Code fact extraction (CLI, config, APIs)
- [ ] Doc claim parsing
- [ ] Accuracy cross-validation
- [ ] Completeness by audience
- [ ] Over/under-documentation detection
- [ ] Auto-fix generation

### Phase 3 (Future)
- [ ] CI comment bot
- [ ] Trend tracking dashboard
- [ ] Auto-generated reference docs
- [ ] Documentation debt tracking

## Contributing

Improvements welcome! To contribute:

1. Test your changes on a real codebase
2. Update documentation
3. Add examples
4. Submit a PR

## FAQ

**Q: How often should I run this?**
A: Weekly scheduled runs + on every PR.

**Q: What if I have a lot of technical terms?**
A: Add them to `config.yaml` under `tools.codespell.ignore_words`.

**Q: Can I use this with non-Go projects?**
A: Phase 1 works for any project with markdown docs. Phase 2 is Go-specific.

**Q: Does this replace manual review?**
A: No, it complements manual review by catching mechanical issues.

**Q: What's the difference from golangci-lint?**
A: This is comprehensive (links, spelling, examples, cross-validation) vs. just code comments.

## References

- [SKILL.md](SKILL.md) - Complete skill documentation
- [config.yaml](config.yaml) - Configuration reference
- [Go Doc Comments](https://go.dev/doc/comment) - Go documentation style
- [Write the Docs](https://www.writethedocs.org/) - Documentation best practices

## License

Same as parent project.
