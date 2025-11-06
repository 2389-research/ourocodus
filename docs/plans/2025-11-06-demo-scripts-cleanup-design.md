# Demo Scripts Cleanup and Organization Design

**Date:** 2025-11-06
**Issue:** #163 - Clean up demo scripts and move to examples/
**Approach:** Conservative audit and migrate

## Problem Statement

The `scripts/` directory currently contains a mix of:
- Demo scripts for showcasing features
- Interactive test scripts
- Performance testing tools
- Debug/prototype code
- Production infrastructure scripts (CI/CD, setup tools)

This mixing makes it unclear what's production infrastructure versus educational examples, and creates confusion for contributors and users.

## Design Goals

1. **Separate concerns**: Production infrastructure in `scripts/`, educational demos in `examples/`
2. **Maintain quality**: Only keep demos that work with current code and have educational value
3. **Enhance usability**: Comprehensive READMEs for each example
4. **Clean history**: Clear commit structure showing what moved where and why

## Evaluation Criteria

Scripts are kept as examples if they meet both criteria:
- ✅ **Current functionality**: Demonstrates features that still work in the current codebase
- ✅ **Educational value**: Helps users understand key concepts, patterns, or workflows

## Proposed Directory Structure

```
examples/
├── README.md                          # Overview of all examples
├── basic-demo/                        # Simple relay + agent demo
│   ├── README.md
│   └── main.go                        # From scripts/demo/main.go
├── interactive-repl/                  # Interactive testing
│   ├── README.md
│   └── main.go                        # From scripts/interactive/main.go
├── performance-testing/               # Load testing demos
│   ├── README.md
│   └── [demo-performance scripts]
├── smoke-tests/                       # Smoke test examples
│   ├── README.md
│   └── [smoketest scripts]
└── debugging/                         # Debug/test scenarios
    ├── README.md
    └── container-race/

scripts/                               # Production infrastructure only
├── nats-init.sh                       # NATS setup
├── setup-worktrees.sh                 # Dev tooling
├── run-e2e.sh                         # CI/CD
└── smoke-test.sh                      # Automated testing
```

## Script Categorization

### Demo/Test Scripts → Move to examples/

These scripts have educational/demo value:
- `scripts/demo/` - Basic relay+agent demo
- `scripts/interactive/` - Interactive REPL
- `scripts/demo-performance/` - Performance testing suite
- `scripts/smoketest/` - Smoke test examples
- `scripts/container-race/` - Debug scenario for race conditions

### Infrastructure Scripts → Keep in scripts/

These are production development/CI tools:
- `scripts/nats-init.sh` - NATS JetStream initialization for docker-compose
- `scripts/setup-worktrees.sh` - Git worktree management for multi-agent development
- `scripts/run-e2e.sh` - End-to-end test runner for CI/CD
- `scripts/smoke-test.sh` - Automated smoke testing

## Audit Process

For each script identified as a potential example:

1. **Examine** - Read code to understand purpose and dependencies
2. **Verify** - Check if it references current binaries/APIs
3. **Test** - Run the script to confirm functionality
4. **Evaluate** - Apply criteria (current + educational)
5. **Decide** - Keep as example, keep as infrastructure, or remove

## Migration Process

For each script/directory moving to examples/:

1. **Create subdirectory** in examples/ with descriptive name
2. **Copy script(s)** to new location
3. **Write README.md** with:
   - Purpose and educational goals
   - Prerequisites (binaries, services, environment variables)
   - Step-by-step execution instructions
   - Expected output and behavior
   - Key concepts demonstrated
   - Troubleshooting common issues
4. **Test from new location** - Verify all paths resolve correctly
5. **Update path references** in script if necessary
6. **Remove from scripts/** after successful testing

## Testing Strategy

Before each migration:
- ✅ Build project: `make build`
- ✅ Run from current location
- ✅ Move to examples/
- ✅ Update paths if needed
- ✅ Run from new location
- ✅ Verify relative paths to `bin/` directory work

## Documentation Updates

After migration:
- Update root `README.md` for any old script references
- Create `examples/README.md` as main entry point
- Verify `.gitignore` doesn't exclude examples/
- Update CI/CD references to moved scripts

## Commit Strategy

- One commit per script/directory migration for clean history
- Commit message format: `docs: migrate <script> to examples/`
- Include removal reason in commit message if script is deleted
- Final commit: `chore: clean up scripts/ directory structure`

## Success Metrics

- ✅ `scripts/` contains only production infrastructure
- ✅ `examples/` has clear, documented demos
- ✅ Each example has comprehensive README
- ✅ All examples are tested and work
- ✅ No references to old locations remain

## Risks and Mitigation

| Risk | Mitigation |
|------|------------|
| Breaking existing workflows | Audit all references before moving; update docs |
| Scripts don't work in new location | Test thoroughly; update paths as needed |
| Losing valuable examples | Conservative approach: audit before removal |
| Outdated examples misleading users | Only keep scripts that pass current functionality test |
