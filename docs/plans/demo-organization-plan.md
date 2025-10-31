# Demo Organization Plan

## Problem Statement

The current demo/script/example ecosystem is disorganized, making it difficult for:
- **New contributors** to find relevant examples
- **Team members** to understand what demos each feature
- **Presenters** to know which demo to use for which audience
- **Maintainers** to keep documentation synchronized

### Current Issues

1. **Inconsistent Naming**:
   - `scripts/demo/`, `scripts/interactive/`, `scripts/container-race/`, `scripts/demo-performance/`
   - No clear naming convention or pattern

2. **Scattered Documentation**:
   - Root-level demo guides (`DEMO-GUIDE.md`, `DEMO-QUICK-REF.md`)
   - Per-demo docs (`container-race/DEMO_GUIDE.md`)
   - No central index or feature matrix

3. **Mixed Purposes**:
   - User-facing demos (container-race, PWA)
   - Developer tools (interactive REPL)
   - Testing infrastructure (smoke tests, e2e)
   - Setup scripts (nats-init, setup-worktrees)
   - Code examples (NATS patterns)

4. **Unclear Mapping**:
   - Which demo shows which feature?
   - What's the difference between `demo/` and `interactive/`?
   - Are smoke tests demos or tests?

## Current Inventory

### scripts/
```
scripts/
├── container-race/         # PackNplay parallel containers demo (4 files, 844 lines)
│   ├── main.go
│   ├── README.md
│   ├── DEMO_GUIDE.md
│   └── SUMMARY.md
├── demo/                   # PR #27 WebSocket features showcase
│   └── main.go
├── demo-performance/       # Database optimization visualizer (12 files)
│   ├── demo-run.sh
│   ├── demo-setup.sh
│   ├── DEMO_PACKAGE_OVERVIEW.md
│   └── ...
├── interactive/            # Interactive REPL for manual testing
│   └── main.go
├── smoketest/              # Smoke test fixtures
│   ├── relay/
│   └── session/
├── nats-init.sh           # NATS setup script
├── run-e2e.sh             # E2E test runner
├── setup-worktrees.sh     # Git worktree initialization
└── smoke-test.sh          # Unified smoke test runner
```

### examples/
```
examples/
└── nats/                   # NATS client patterns
    ├── basic/
    ├── graceful-shutdown/
    ├── jetstream/
    └── request-reply/
```

### web/
```
web/                        # PWA with Protocol Inspector
├── index.html
├── demo.html
├── app.js
├── demo.js
├── README.md (detailed)
└── ...
```

### Root Documentation
```
DEMO-GUIDE.md              # WebSocket reconnection demo guide
DEMO-QUICK-REF.md          # Quick reference card for demos
```

## Proposed Organization

### New Directory Structure

```
ourocodus/
├── demos/                          # User-facing feature demonstrations
│   ├── README.md                   # Central demo index + feature matrix
│   ├── container-race/             # PackNplay: parallel containers, worktrees, I/O
│   │   ├── README.md               # How to run + what it shows
│   │   ├── GUIDE.md                # Presentation talking points
│   │   ├── main.go
│   │   └── ...
│   ├── websocket-reconnection/     # Resilient WebSocket connections
│   │   ├── README.md
│   │   ├── GUIDE.md
│   │   ├── QUICK-REF.md
│   │   ├── demo-run.sh
│   │   └── demo-reset.sh
│   └── performance-viz/            # Database optimization visualizer
│       ├── README.md
│       ├── demo-run.sh
│       ├── demo-setup.sh
│       └── ...
│
├── examples/                       # Code examples for developers
│   ├── README.md                   # Examples index + use cases
│   ├── nats/                       # NATS client patterns
│   │   ├── README.md
│   │   ├── basic/
│   │   ├── graceful-shutdown/
│   │   ├── jetstream/
│   │   └── request-reply/
│   └── agent-lifecycle/            # Basic agent spawning/stopping
│       ├── README.md
│       ├── spawn-simple/
│       ├── spawn-with-config/
│       └── attach-existing/
│
├── web/                            # PWA application (SPECIAL CASE - see below)
│   ├── README.md                   # PWA + Protocol Inspector docs
│   ├── index.html                  # Main PWA interface
│   ├── app.js                      # PWA application logic
│   ├── styles.css                  # PWA styles
│   ├── demo.html                   # Protocol Inspector (dev tool)
│   ├── demo.js                     # Inspector logic
│   ├── demo.css                    # Inspector styles
│   ├── demo-shim.js                # Iframe communication
│   └── ...
│
├── scripts/                        # Development/testing scripts
│   ├── README.md                   # Script index + descriptions
│   ├── setup-worktrees.sh          # Git worktree initialization
│   ├── nats-init.sh                # NATS infrastructure setup
│   ├── smoke-test.sh               # Unified smoke test runner
│   ├── run-e2e.sh                  # E2E test orchestration
│   └── smoketest/                  # Smoke test fixtures
│       ├── relay/
│       └── session/
│
└── tools/                          # Developer tools (new)
    ├── README.md                   # Tools index
    └── interactive-repl/           # Manual testing REPL
        ├── README.md
        └── main.go
```

### Naming Conventions

#### Demos (`demos/`)
**Pattern**: `<feature-name>` or `<descriptive-name>`

**Examples**:
- `container-race` - Shows PackNplay parallel execution
- `websocket-reconnection` - Shows resilient WebSocket handling
- `performance-viz` - Shows database optimization results

**Rules**:
- Use kebab-case
- Name describes WHAT is being demonstrated
- Include target audience in README (technical/executive)

#### Examples (`examples/`)
**Pattern**: `<technology>` or `<pattern-name>`

**Examples**:
- `nats` - NATS client patterns
- `agent-lifecycle` - Agent spawning patterns
- `packnplay-basic` - Basic Packnplay usage

**Rules**:
- Use kebab-case
- Name describes the technology or pattern
- Focused on code patterns, not presentations

#### Scripts (`scripts/`)
**Pattern**: `<action>-<target>.sh` for shell scripts

**Examples**:
- `setup-worktrees.sh` - Initialize git worktrees
- `nats-init.sh` - Setup NATS infrastructure
- `smoke-test.sh` - Run smoke tests

**Rules**:
- Use kebab-case
- Verb-noun structure
- Must have usage/help text

#### Tools (`tools/`)
**Pattern**: `<tool-purpose>`

**Examples**:
- `interactive-repl` - Manual testing REPL
- `log-analyzer` - Parse and analyze logs (hypothetical)

**Rules**:
- Use kebab-case
- Name describes the tool's purpose
- Include README with usage examples

### Special Case: web/ Directory

**Decision**: Keep `web/` as-is, but improve discoverability.

#### Why web/ Stays in Place

The `web/` directory contains both production code and demo/debug tools that are **tightly coupled**:

**Production PWA**:
- `index.html` - Main application interface
- `app.js` - Application logic (33KB)
- `styles.css` - Application styles

**Developer Tools** (embedded in production):
- `demo.html` - **Protocol Inspector** - Visual WebSocket message debugger
- `demo.js` - Inspector logic
- `demo.css` - Inspector styles
- `demo-shim.js` - Iframe communication shim

#### Why Not Move to demos/?

1. **Tight coupling**: Protocol Inspector embeds the PWA via iframe (`<iframe src="/?demo=true">`)
2. **Same server**: Both served from relay web server at `localhost:8080`
3. **Shared protocol**: Both use same WebSocket connection and message format
4. **Development tool**: Protocol Inspector is for debugging, not presentations
5. **Already documented**: `web/README.md` has detailed protocol inspector docs

#### How We Improve Discoverability

**1. Update demos/README.md** feature matrix:
```markdown
| Feature | Location | Description |
|---------|----------|-------------|
| Protocol Inspector | `web/demo.html` | Visual WebSocket message debugger (dev tool) |
```

**2. Update web/README.md** structure:
```markdown
## Contents

### Production Application
- `index.html` - Main PWA interface
- `app.js` - Application logic
- `styles.css` - Application styles

### Developer Tools
- `demo.html` - **Protocol Inspector** - Debug WebSocket messages
- `demo.js` - Inspector logic with message visualization
- `demo.css` - Inspector layout (split-pane view)
- `demo-shim.js` - Iframe communication bridge

## Running the Protocol Inspector

1. Start relay: `make run`
2. Open: http://localhost:8080/demo.html
3. Use PWA in left pane, watch messages in right pane
```

**3. Add Makefile target** for easy access:
```makefile
# Open Protocol Inspector for debugging WebSocket messages
demo-protocol-inspector:
	@echo "🔍 Opening Protocol Inspector..."
	@if ! curl -s http://localhost:8080 > /dev/null 2>&1; then \
		echo "❌ Error: Relay server not running"; \
		echo "Start it with: make run"; \
		exit 1; \
	fi
	open http://localhost:8080/demo.html
```

**4. Add .mise.toml task**:
```toml
[tasks."demo:protocol-inspector"]
description = "🔍 Open Protocol Inspector (WebSocket debugger)"
run = "make demo-protocol-inspector"
```

#### Result

Users can discover the Protocol Inspector through:
- **demos/README.md** - Listed in feature matrix
- **Makefile** - `make demo-protocol-inspector`
- **mise** - `mise run demo:protocol-inspector`
- **web/README.md** - Detailed usage docs

But the code stays in `web/` where it belongs with the PWA.

### Documentation Standards

#### demos/README.md (Central Index)

**Required sections**:
1. **Overview** - What demos exist and their purposes
2. **Feature Matrix** - Which demo shows which capability
3. **Quick Start** - One-liner to run each demo
4. **Prerequisites** - Common requirements (Docker, NATS, etc.)
5. **Troubleshooting** - Common issues across demos

**Feature Matrix Example**:
```markdown
| Feature | container-race | websocket-reconnection | performance-viz |
|---------|----------------|------------------------|-----------------|
| Parallel execution | ✅ | | |
| Git worktrees | ✅ | | |
| Real-time I/O | ✅ | | |
| WebSocket resilience | | ✅ | |
| Session recovery | | ✅ | |
| Performance metrics | | | ✅ |
| Visual dashboards | | | ✅ |
```

#### Per-Demo Documentation

**demos/<name>/README.md**:
- **What It Shows** - Features demonstrated
- **Target Audience** - Who should see this (technical/executive/both)
- **Requirements** - Docker, NATS, API keys, etc.
- **Quick Start** - One command to run
- **How It Works** - Brief technical overview
- **Troubleshooting** - Demo-specific issues

**demos/<name>/GUIDE.md** (optional):
- **Pre-Demo Checklist** - What to run beforehand
- **Talking Points** - What to say and when
- **Expected Output** - What audience should see
- **Q&A Prep** - Common questions and answers
- **Fallback Plan** - What to do if demo breaks

#### examples/README.md

**Required sections**:
1. **Overview** - What examples exist
2. **Use Cases** - When to use which example
3. **Getting Started** - Basic setup
4. **Contributing** - How to add new examples

#### scripts/README.md

**Required sections**:
1. **Overview** - Available scripts
2. **Common Workflows** - Script sequences for common tasks
3. **Environment Requirements** - What each script needs
4. **Maintenance** - How to update scripts

#### tools/README.md

**Required sections**:
1. **Available Tools** - What tools exist
2. **Installation** - How to install/build
3. **Common Use Cases** - When to use each tool

## Migration Plan

### Phase 1: Create New Structure (No Breaking Changes)

```bash
# Create new directories
mkdir -p demos examples tools

# Create index READMEs
touch demos/README.md examples/README.md scripts/README.md tools/README.md
```

**Tasks**:
1. Create skeleton directory structure
2. Write central README files
3. Create feature matrix in `demos/README.md`
4. Document migration plan in `CONTRIBUTING.md`

**No files moved yet - purely additive**

### Phase 2: Migrate Demos

**Move and Rename**:
```bash
# Container race stays mostly the same
git mv scripts/container-race demos/

# WebSocket reconnection consolidation
mkdir demos/websocket-reconnection
git mv DEMO-GUIDE.md demos/websocket-reconnection/GUIDE.md
git mv DEMO-QUICK-REF.md demos/websocket-reconnection/QUICK-REF.md
# Create wrapper that references web/demo.html

# Performance visualizer
git mv scripts/demo-performance demos/performance-viz

# Move interactive to tools
git mv scripts/interactive tools/interactive-repl

# Move basic demo (decide on its future)
# scripts/demo -> either merge into another demo or deprecate
```

**Update Makefile**:
```makefile
# Update targets to point to new locations
demo-container-race:
    go run demos/container-race/main.go

demo-websocket:
    @echo "Opening web/demo.html in browser..."
    open web/demo.html

demo-performance:
    cd demos/performance-viz && ./demo-run.sh

tool-repl:
    go run tools/interactive-repl/main.go
```

**Update .mise.toml**:
```toml
[tasks."demo:container-race"]
description = "🏁 PackNplay parallel containers demo"
run = "make demo-container-race"

[tasks."demo:websocket"]
description = "🔌 WebSocket reconnection demo"
run = "make demo-websocket"

[tasks."demo:performance"]
description = "📊 Database performance demo"
run = "make demo-performance"
```

### Phase 3: Update Documentation

**Tasks**:
1. Update `README.md` to reference `demos/README.md`
2. Update `CONTRIBUTING.md` with new structure
3. Add demo organization section to docs
4. Update all internal references to old paths
5. Update GitHub workflows if they reference old paths

### Phase 4: Cleanup

**Tasks**:
1. Remove deprecated files/directories
2. Update `.gitignore` if needed
3. Archive old demo scripts (if any) to `archive/` for reference
4. Update all documentation cross-references
5. Verify CI/CD still works

## Benefits

### For New Contributors
- **Single entry point**: `demos/README.md` shows all demos
- **Clear purposes**: Each directory name explains its purpose
- **Consistent structure**: Same layout in every demo
- **Easy discovery**: Feature matrix shows what to explore

### For Presenters
- **Find the right demo**: Feature matrix maps capabilities to demos
- **Presentation ready**: Each demo has GUIDE.md with talking points
- **Quick start**: One command to run each demo
- **Fallback plans**: GUIDE.md includes "what if it breaks"

### For Maintainers
- **Clear ownership**: demos/ vs examples/ vs scripts/ vs tools/
- **Standard structure**: Same README format everywhere
- **Easier updates**: All related docs in one place
- **Less duplication**: Central feature matrix prevents overlap

### For Users
- **Learn by example**: Progressive examples/ directory
- **See it in action**: Polished demos/ directory
- **Self-service tools**: tools/ directory for power users

## Success Metrics

### Quantitative
- Reduce demo discovery time from ~10 minutes to <2 minutes
- Centralize demo docs from 6+ locations to 1 entry point
- Standardize README structure across 100% of demos

### Qualitative
- New contributors can find relevant demo in <2 minutes
- Presenters can prepare for demo in <5 minutes
- Maintainers can add new demo following clear template

## Rollout

### Week 1: Planning & Setup
- [ ] Create this plan document
- [ ] Get feedback from team
- [ ] Create GitHub issue
- [ ] Get approval to proceed

### Week 2: Structure Creation
- [ ] Create skeleton directories
- [ ] Write central README files
- [ ] Create feature matrix
- [ ] Update CONTRIBUTING.md

### Week 3: Migration
- [ ] Move demos one-by-one
- [ ] Update Makefile targets
- [ ] Update .mise.toml tasks
- [ ] Test each demo still works

### Week 4: Documentation & Cleanup
- [ ] Update all documentation references
- [ ] Update GitHub workflows
- [ ] Archive old structure
- [ ] Verify CI/CD
- [ ] Announce completion

## Open Questions

1. **scripts/demo/main.go**: What does this demo vs scripts/interactive? Should we merge or keep both?
   - Investigate: What unique value does each provide?

2. **Smoke tests**: Are these demos or tests?
   - Proposal: Keep in scripts/ as they're testing infrastructure

3. **E2E tests**: Similar question to smoke tests
   - Proposal: Keep in scripts/ as orchestration for test suite

4. **NATS examples**: Should these stay in examples/ or move to docs/?
   - Proposal: Keep in examples/ as runnable code

5. **Web demos**: Should demos that use web/ reference it or duplicate?
   - **Decision**: Keep web/ as-is. Protocol Inspector is tightly coupled to PWA (iframe embed).
   - Improve discoverability via demos/README.md feature matrix + Makefile targets.
   - See "Special Case: web/ Directory" section above for full rationale.

6. **Demo-specific scripts**: Keep with demo or in scripts/?
   - Proposal: Keep with demo (e.g., `demos/performance-viz/demo-run.sh`)

## Related Work

- Issue #XX: Improve documentation structure (if exists)
- PR #XX: Add CONTRIBUTING.md (if exists)
- Docs restructuring discussion (if exists)

## Next Steps

1. Create GitHub issue from this plan
2. Get team feedback on proposed structure
3. Iterate on plan based on feedback
4. Begin Phase 1 implementation
