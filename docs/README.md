# Ourocodus Documentation Index

Welcome to the Ourocodus documentation! This index will help you find what you need based on your role and goals.

## 🚀 Start Here

New to Ourocodus? Start with these documents:

- [**README.md**](../README.md) - Project overview, quick start, and architecture summary
- [**PRD.md**](../PRD.md) - Product requirements and vision
- [**ROADMAP.md**](../ROADMAP.md) - Development roadmap, milestones, and timeline

## 👥 For Contributors

Contributing to Ourocodus? These docs will help you get started:

- [**CONTRIBUTING.md**](../CONTRIBUTING.md) - How to contribute, development workflow, and standards
- [**AGENTS.md**](../AGENTS.md) - AI agent development guide (for Claude Code, Cursor, etc.)
- [**DOCUMENTATION.md**](../DOCUMENTATION.md) - Documentation strategy and maintenance guide
- [**CHANGELOG.md**](../CHANGELOG.md) - Version history and changes

## 🏗️ Architecture & Design

Understanding how Ourocodus works:

### Core Architecture
- [**ARCHITECTURE.md**](architecture/ARCHITECTURE.md) - System architecture overview (Phase 1 vs Long-term)
- [**ACP.md**](architecture/ACP.md) - Agent Client Protocol integration and transport layer
- [**AGENT_RUNTIME.md**](architecture/AGENT_RUNTIME.md) - Runtime components, session types, and lifecycle
- [**NATS.md**](architecture/NATS.md) - Message bus architecture and JetStream integration
- [**PROTOCOLS.md**](architecture/PROTOCOLS.md) - Communication patterns and message flows
- [**PWA.md**](architecture/PWA.md) - Progressive Web App frontend architecture

### Detailed Design Documents
- [**acp-launcher-selection.md**](architecture/acp-launcher-selection.md) - ACP launcher selection logic (technical deep-dive)

## 💻 Developer Guides

Day-to-day development resources:

- [**SESSION_LIFECYCLE.md**](development/SESSION_LIFECYCLE.md) - Session and agent lifecycle management
- [**ERROR_HANDLING.md**](development/ERROR_HANDLING.md) - Error codes, handling strategies, and recoverable vs non-recoverable errors
- [**TESTING.md**](development/TESTING.md) - Testing strategy, test suites, and guidelines
- [**TERMINOLOGY.md**](development/TERMINOLOGY.md) - Project terminology and definitions

## 🔐 Operations

Deployment, security, and production operations:

- [**SECURITY.md**](operations/SECURITY.md) - Security threat models, workspace isolation, and mitigation strategies
- [**STATIC_FILE_SERVING.md**](operations/STATIC_FILE_SERVING.md) - Static file serving strategy for the PWA

## 📋 Component PRDs

Detailed requirements for individual components:

- [**prd/api.md**](prd/api.md) - API Server requirements
- [**prd/cli.md**](prd/cli.md) - CLI tool requirements
- [**prd/coordinator.md**](prd/coordinator.md) - Coordinator service requirements
- [**prd/relay.md**](prd/relay.md) - Relay server requirements

## 🗂️ Additional Resources

### Issue Tracking & Design
- [**ISSUES.md**](ISSUES.md) - Issue dependency tracking and relationships
- [**issues/**](issues/) - Detailed design documents for specific issues

### Testing Documentation
- [**testing/**](testing/) - Test plans and integration test documentation

### Implementation Plans
- [**plans/**](plans/) - Current and future implementation plans

### Implementation History
- [**history/**](history/) - Archived implementation plans and completed designs
  - [2025 Q4 Milestones 1-2](history/2025-Q4-milestone1-2/) - Historical execution logs and completed features

## 📦 Package Documentation

Package-specific documentation is located alongside the code:

- [**pkg/containersession/README.md**](../pkg/containersession/README.md) - Container session management
- [**pkg/relay/session/README.md**](../pkg/relay/session/README.md) - Relay session management
- [**examples/README.md**](../examples/README.md) - Example applications and demos
- [**tests/e2e/README.md**](../tests/e2e/README.md) - End-to-end testing

## 🔍 Finding What You Need

**I want to...**

- **Understand the overall system** → Start with [README.md](../README.md) then [ARCHITECTURE.md](architecture/ARCHITECTURE.md)
- **Set up my development environment** → [CONTRIBUTING.md](../CONTRIBUTING.md)
- **Learn about agent containers** → [AGENT_RUNTIME.md](architecture/AGENT_RUNTIME.md) and [ACP.md](architecture/ACP.md)
- **Understand error handling** → [ERROR_HANDLING.md](development/ERROR_HANDLING.md)
- **Deploy to production** → [SECURITY.md](operations/SECURITY.md) and operations docs
- **Write tests** → [TESTING.md](development/TESTING.md)
- **Find historical context** → [history/](history/) directory

## 📝 Documentation Standards

When contributing documentation:

1. Follow the guidelines in [DOCUMENTATION.md](../DOCUMENTATION.md)
2. Use markdown with GitHub-flavored extensions
3. Include diagrams using Mermaid when helpful
4. Keep code examples up-to-date with implementation
5. Update this index when adding new major documents

---

**Questions or suggestions?** Open an issue with the `documentation` label.

**Last Updated:** 2025-01-12
