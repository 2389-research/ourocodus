# CI Integration Guide

## Value Proposition

### Problems Solved

| Problem | Without CI | With CI |
|---------|------------|---------|
| **Broken Examples** | Users copy code that doesn't compile | PR blocked until example works |
| **Broken Links** | 404 errors frustrate users | Caught before merge |
| **Outdated Info** | Docs drift from code over months | Detected immediately |
| **Inconsistent Style** | Varies by contributor | Enforced standards |
| **Missing Docs** | New APIs undocumented | Warning on low coverage |
| **Spelling Errors** | Unprofessional appearance | Auto-detected |

### Cost/Benefit Analysis

**Time Cost:**
- Setup: 5 minutes (workflows already created)
- Per PR: 2-3 minutes CI runtime
- Fix issues: Same time you'd spend anyway (but earlier)

**Time Saved:**
- Support questions: 30-60 min per outdated doc issue
- User frustration: Priceless
- Reputation damage: Hard to quantify

**Break-even:** Catches just 1 outdated example per month → You save time

## What You Get

### 1. PR Comment with Full Report

Every PR gets an automatic comment:

```markdown
## 📚 Documentation Audit

### Results
Total Checks: 6
Passed: 4 ✓
Warnings: 2 ⚠
Failed: 0 ✗

---

### Details

✓ Inventory Collection - 89 docs, 10 packages
✓ Link Checking - All 47 links valid
✓ Example Tests - All examples compile
⚠ Spell Checking - Found 3 errors (see details)
✓ Markdown Linting - No issues
✓ Doc Coverage - 87% (target: 85%)

---

💡 Review warnings before merge
```

### 2. Artifacts for Investigation

Every run uploads detailed reports:
- `quick-checks-report.md` - Human-readable
- `quick-checks-report.json` - Machine-readable
- `inventory.json` - Full documentation index

**Access:** Actions tab → Workflow run → Artifacts

### 3. Weekly Health Checks

Scheduled audit creates issues when problems accumulate:

```markdown
Title: 📚 Weekly Documentation Audit Found Issues

Body:
Found 3 broken links and 1 failing example.

Links:
- docs/old-api.md → 404 (referenced in README.md:45)

Examples:
- README.md:142 - Example "Basic Usage" fails to compile

Action: Fix these issues or close if no longer relevant
```

## Configuration Options

### Strict Mode (Recommended for mature projects)

Block merges on any failure:

```yaml
- name: Run Documentation Audit
  run: |
    cd .claude/skills/documentation-audit
    ./scripts/run-quick-checks.sh
  # No continue-on-error → Fails PR on issues
```

**Result:** Zero tolerance for documentation problems

### Advisory Mode (Recommended for new adoption)

Allow merges but warn:

```yaml
- name: Run Documentation Audit
  continue-on-error: true
  run: |
    cd .claude/skills/documentation-audit
    ./scripts/run-quick-checks.sh
```

**Result:** Visibility without blocking workflow

### Selective Checks

Only run on documentation changes:

```yaml
on:
  pull_request:
    paths:
      - '**.md'
      - 'docs/**'
      # Not triggered by code-only changes
```

**Result:** Faster CI for code-only PRs

## Tuning for Your Project

### Start Conservative

Week 1-2: **Advisory mode** - Get baseline, no blocking
```yaml
continue-on-error: true
```

Week 3-4: **Block on failures** - Fix critical issues only
```yaml
continue-on-error: false  # But adjust thresholds in config.yaml
```

Month 2+: **Enforce standards** - Warnings become blockers
```yaml
thresholds:
  doc_coverage_min: 85  # Gradually increase
  broken_links_max: 0   # Zero tolerance
  spelling_errors_max: 0
```

### Adjust Thresholds

Edit `.claude/skills/documentation-audit/config.yaml`:

```yaml
# Relaxed for starting out
thresholds:
  doc_coverage_min: 50      # Low bar initially
  broken_links_max: 5       # Allow some legacy issues
  spelling_errors_max: 10   # Focus on bigger problems

# Strict for mature projects
thresholds:
  doc_coverage_min: 90      # High standards
  broken_links_max: 0       # Zero tolerance
  spelling_errors_max: 0    # Professional polish
```

## Real Output Examples

### Scenario 1: Developer Updates API

**PR:** Change function signature

```diff
-func Connect(url string) error
+func Connect(ctx context.Context, url string) error
```

**CI Output:**
```
✗ Example Tests
FAILED: README.md:45 - Example "Quick Start"

Error:
  not enough arguments in call to Connect
  have (string)
  want (context.Context, string)

Fix: Update example to include context.Context
```

**Developer:** Ah! Forgot to update README. Fixes it.

**Result:** Users never see broken example.

### Scenario 2: Documentation Refactoring

**PR:** Reorganize docs folder

```
docs/
  api.md → reference/api.md
  guide.md → tutorials/quickstart.md
```

**CI Output:**
```
✗ Link Checking
FAILED: 2 broken internal links

1. docs/api.md → 404
   Referenced in: README.md:34, CONTRIBUTING.md:67

2. docs/guide.md → 404
   Referenced in: README.md:56

Fix: Update references to new paths
```

**Developer:** Right, need to update all references.

**Result:** No broken links after merge.

### Scenario 3: Clean PR

**PR:** Add new feature with docs

**CI Output:**
```
✅ All checks passed!

Total Checks: 6
Passed: 6 ✓

Details:
✓ Inventory - Added 2 new docs
✓ Links - All 52 links valid
✓ Examples - New example compiles and runs
✓ Spelling - No errors
✓ Markdown - Proper formatting
✓ Coverage - 89% (+2% from new docs)

Great work! Documentation is thorough and accurate.
```

**Maintainer:** Instant confidence to merge.

## Installation

Already done! The workflows are at:
- `.github/workflows/docs-audit.yml` - PR checks
- `.github/workflows/docs-audit-scheduled.yml` - Weekly audits

### Test It

**Trigger manually:**
1. Go to Actions tab
2. Select "Documentation Audit"
3. Click "Run workflow"

**Or make a test PR:**
```bash
# Create test branch
git checkout -b test-docs-ci

# Make a trivial change
echo "\nTest CI" >> README.md

# Commit and push
git add README.md
git commit -m "test: trigger docs CI"
git push origin test-docs-ci

# Create PR and watch CI run
gh pr create --title "Test docs CI" --body "Testing doc audit"
```

## Monitoring

### GitHub Actions Dashboard

See all runs: `https://github.com/{org}/{repo}/actions/workflows/docs-audit.yml`

### Metrics to Track

- **Pass rate**: What % of PRs pass first try?
- **Time to fix**: How long to resolve issues?
- **Coverage trend**: Is doc coverage improving?
- **Issue detection**: How many real issues caught?

### Success Indicators

**After 1 month:**
- ✓ Zero broken links in main branch
- ✓ All examples compile
- ✓ Doc coverage > 80%

**After 3 months:**
- ✓ 90%+ PRs pass first try
- ✓ Zero user-reported doc issues
- ✓ Doc coverage > 85%

**After 6 months:**
- ✓ Documentation trusted by users
- ✓ Lower support burden
- ✓ Faster onboarding

## FAQ

**Q: Will this slow down PRs?**
A: Runtime is 2-3 minutes. Fixes are usually quick (update a link, fix spelling).

**Q: What if I need to merge urgently?**
A: Use advisory mode (`continue-on-error: true`) or skip CI with `[skip ci]` in commit message (not recommended).

**Q: What about false positives?**
A: Adjust exclusions in `config.yaml` or add to ignore list. Very rare with current tooling.

**Q: Do I need all the optional tools?**
A: No, but you get more value:
- Just Go → Example validation
- + lychee → Link checking
- + markdownlint → Style consistency
- + codespell → Professional polish

**Q: Can I run this locally before pushing?**
A: Yes! `cd .claude/skills/documentation-audit && ./scripts/run-quick-checks.sh`

**Q: What if CI fails on old legacy docs?**
A: Add to exclusions temporarily, tackle in dedicated cleanup PR.

## Next Steps

1. **Test it**: Trigger workflow manually to see it in action
2. **Create test PR**: See the PR comment format
3. **Adjust thresholds**: Start lenient, tighten over time
4. **Add pre-commit hook**: Catch issues before push (optional)

## Support

Issues with CI integration? Check:
- Workflow logs in Actions tab
- Tool installation steps (most common issue)
- Config exclusions if too many false positives
- Open issue with `[docs-ci]` tag
