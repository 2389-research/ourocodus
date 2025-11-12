# Documentation Audit Skill Design

**Status**: Design
**Created**: 2025-11-12
**Type**: General-purpose skill for AI coding agents

## Purpose

This skill enables AI agents to audit and repair markdown documentation in any codebase, verifying that documentation claims match actual code behavior and maintaining documentation structure.

## Success Criteria

1. **Accuracy**: All documentation claims verified against code
2. **Navigation**: Clear structure with functioning cross-links and README files where needed
3. **Currency**: Obsolete documentation archived or removed
4. **Completeness**: All significant features documented

## Scope

**Included**:
- Markdown documentation files (*.md)
- Text-based diagrams (Mermaid, PlantUML, Graphviz, ASCII)
- Documentation structure and navigation

**Excluded**:
- Code comments and godoc
- Image-based diagrams
- Point-in-time documents (docs/plans/*)

## Architecture: Multi-Pass Progressive Refinement

The skill operates in four passes, each building on the previous.

### Pass 0: Repository Indexing

Build a comprehensive index of the repository before verification begins.

**Activities**:
- Extract symbol graph using symbolic code analysis tools
- Identify API contracts from canonical sources (OpenAPI, protobuf, GraphQL schemas, configuration files)
- Map documentation cross-links and navigation files
- Index CODEOWNERS, commit frequency per file, and last modification dates
- Cache results for token efficiency

**Output**: Repository index with symbols, canonical sources, documentation graph, and ownership metadata.

**Rationale**: Front-loading repository knowledge prevents redundant analysis in later passes and enables verification against authoritative sources.

### Pass 1: Discovery and Classification

Discover all markdown files and classify them by purpose and lifecycle.

**Activities**:
- Find markdown files recursively
- Classify using multiple signals:
  - Path heuristics (docs/archive/*, files with dates in names)
  - Commit recency and change velocity
  - Link graph centrality (frequently linked documents)
  - CODEOWNERS presence and ownership patterns
  - YAML frontmatter (status: living|archival|planned)
- Assign confidence scores to classifications
- Identify obsolete candidates for archival

**Output**: Classified document inventory with confidence scores.

**Decision Rules**:
- docs/plans/* are point-in-time (never updated)
- High velocity + clear ownership = living documentation
- Archived paths + low recency = obsolete candidate
- Low confidence classifications require user review before destructive actions

**Safety**: Move obsolete documents to docs/archive/ rather than delete. Never remove documents with low-confidence classifications without approval.

### Pass 2: Claim Extraction and Investigation Planning

Extract verifiable claims from living documentation and plan verification strategy.

**Activities**:
- Parse living documentation and extract claims by type:
  - **Behavioral**: "Service retries three times"
  - **Structural**: "Class implements Interface"
  - **API**: "Endpoint returns JSON with schema X"
  - **Configuration**: "Setting defaults to value Y"
  - **Usage**: "Run command with flag --foo"
- Record claim metadata: doc_path, span_offsets, type, referenced_symbols
- Identify text-based diagrams and classify:
  - **Normative**: Must match code structure exactly (UML class diagrams)
  - **Illustrative**: Conceptual, verify broad relationships only (architecture diagrams)
- Analyze codebase structure for documentation gaps:
  - Packages/modules without documentation
  - Exported APIs without usage examples
  - Directories lacking README files
- Build claims ledger: claim_id, doc_path, span, type, symbols, evidence, status, confidence, risk_tier, last_verified_at
- Create investigation plan grouping claims by symbol/module for batch efficiency

**Output**: Claims ledger and batched investigation plan.

**Optimization**: Group claims by topic to share context in verification calls, reducing token usage.

### Pass 3: Verification and Investigation

Verify claims against code using risk-appropriate methods.

**Verification Hierarchy** (highest confidence first):
1. **Canonical sources**: Verify against OpenAPI specs, protobuf definitions, GraphQL schemas, configuration schemas
2. **Symbolic analysis**: Verify structural claims using find_symbol, find_referencing_symbols, search_for_pattern
3. **Deep investigation**: Use zen:analyze, zen:debug, or zen:thinkdeep for complex behavioral claims

**Activities**:
- Process investigation batches
- For each claim:
  - Select verification method based on claim type and available evidence
  - Use symbolic analysis pre/post deep investigation to narrow scope
  - Document findings with evidence trails and confidence scores
  - Record corrections in claims ledger
- Verify diagrams:
  - Parse diagram syntax to extract assertions
  - Compare against symbol graph (normative) or verify broad structure (illustrative)
- Create documentation for identified gaps:
  - Missing README files in documented directories
  - Undocumented features found in Pass 0 indexing
- Handle conflicts:
  - Apply authority hierarchy: canonical sources > generated docs > top-level README > service READMEs
  - Record conflicts with context (version, environment, feature flag)
  - Annotate rather than delete when conflicts may be contextual

**Output**: Verified claims ledger with corrections, new documentation drafts, conflict records.

**Risk Management**:
- Assign confidence scores to all verifications
- Flag low-confidence verifications for user review
- Prefer static verification over runtime checks (safer, more deterministic)
- Never auto-fix based on weak evidence

### Pass 4: Risk-Tiered Repair and Reporting

Apply corrections based on risk assessment.

**Risk Tiers**:

**Auto-fix (no approval required)**:
- Broken internal links (update paths)
- Typos in code references (symbol renamed in codebase)
- Outdated paths (files moved)
- Missing table of contents
- Diagram syntax errors

**PR with evidence (approval required)**:
- Substantive technical corrections (behavior claims)
- Claim deletions or rewrites
- Document reclassifications
- Structural changes
- Conflict resolutions

**User review required**:
- Low-confidence verifications
- Conflicting claims across documents
- Planned features (add disclaimers, don't verify behavior)
- Destructive actions (archival, removal)

**Activities**:
- Apply auto-fixes
- Generate PRs for substantive changes with:
  - Diff preview
  - Evidence trails (code spans, symbols verified, canonical sources)
  - Confidence scores
  - Rationale
- Move obsolete documents to docs/archive/
- Create new README files for undocumented directories
- Insert gap documentation
- Update navigation indexes
- Generate summary report:
  - Changes by category and risk tier
  - Coverage metrics (claims verified / total claims)
  - Remaining manual review items
  - Confidence distribution

**Output**: Applied fixes, pending PRs, archived documents, comprehensive report.

## Claims Ledger Schema

The claims ledger persists across runs to enable incremental verification.

```yaml
claim_id: string
doc_path: string
span_offsets: {start: int, end: int}
type: behavioral | structural | api | config | usage
content: string
referenced_symbols: [string]
canonical_source: string | null
evidence: {
  files: [string],
  spans: [{file: string, line_start: int, line_end: int}],
  verification_method: canonical | symbolic | deep_investigation
}
status: verified | contradicted | unknown | requires_review
confidence: exploring | low | medium | high | very_high | almost_certain | certain
risk_tier: auto_fix | pr_required | user_review
last_verified_at: timestamp
proposed_change: string | null
```

## Authority Hierarchy

When multiple documents make conflicting claims:

1. Canonical sources (OpenAPI, protobuf, schemas)
2. Generated documentation (godoc, JSDoc)
3. Top-level README.md
4. Service/module READMEs
5. Design documents
6. Ad-hoc notes

## Document Classification Heuristics

**Living Documentation** (keep current):
- High commit frequency (updated within last 6 months)
- Clear CODEOWNERS
- Linked from README or navigation
- Path patterns: README.md, CONTRIBUTING.md, ARCHITECTURE.md, top-level docs

**Point-in-Time** (never update):
- Under docs/plans/
- Contains specific dates or version numbers
- Marked with status: plan or status: archival
- Low change frequency (<2 updates ever)

**Obsolete** (archive):
- No updates in 12+ months
- References removed code
- Superseded by newer documentation
- Marked with DRAFT or TODO

## Skill Parameters

The skill accepts these parameters when invoked:

- `dry_run` (default: true): Generate report without applying changes
- `risk_tier` (default: pr_required): Maximum risk tier to auto-apply
- `verification_method` (default: auto): Force specific verification approach
- `focus_paths` (default: all): Restrict to specific directories
- `skip_gap_analysis` (default: false): Skip documentation gap detection

## Token Efficiency Strategies

1. **Index once, reuse**: Pass 0 creates cached repository knowledge
2. **Batch similar claims**: Group by symbol/module to share context
3. **Retrieval-augmented**: Fetch only relevant code spans for verification
4. **Skip unchanged**: Use claims ledger to avoid re-verifying unchanged claims
5. **Prefer static**: Use symbolic analysis before deep investigation
6. **Budget caps**: Hard limits on tokens per run with graceful degradation

## Safety Mechanisms

1. **Evidence trails**: Every change documents source claim, verification method, evidence
2. **Confidence scoring**: Low confidence blocks auto-fixes
3. **Diff previews**: Show changes before applying
4. **Archival over deletion**: Move obsolete docs, don't delete
5. **Idempotence check**: Re-run pipeline on proposed changes to detect oscillation
6. **Ownership respect**: Avoid auto-edits outside CODEOWNERS paths

## Edge Cases

**Feature flags**: Claims may be contextual (true when flag enabled). Record context, don't mark as contradicted.

**Multiple versions**: Documentation for different versions may coexist. Partition claims by version.

**Planned features**: Add disclaimer banners, mark verification: not_applicable. Don't attempt behavioral verification.

**External dependencies**: Claims about external APIs may be stale. Verify against canonical sources if available, otherwise flag for review.

**Runtime behavior**: Claims requiring execution (performance, flakiness) are fragile. Prefer symbolic verification or mark for manual testing.

## Extensibility

Future enhancements:

- Support for additional canonical sources (Terraform, Kubernetes manifests)
- Integration with documentation site generators
- Automated diagram generation from code
- Machine learning for claim extraction refinement
- Continuous monitoring mode (run on every commit)

## Implementation Notes

The skill should be implemented as a Claude Code skill file with:

1. Clear announcement of skill usage
2. TodoWrite tracking for pass progress
3. Incremental user feedback between passes
4. Support for resumption (continuation_id for long-running audits)
5. Integration with existing superpowers skills (code-reviewer, systematic-debugging)

## References

- Zen model critical analysis (gpt-5, continuation: e0470165-64a3-41ed-a777-3c1bc7fe4b47)
- Strunk & White, The Elements of Style (clarity guidelines)
