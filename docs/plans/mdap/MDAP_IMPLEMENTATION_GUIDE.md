# MDAP Implementation Guide for Ourocodus

**Version:** 1.0
**Created:** 2025-11-19
**Status:** Consolidated Reference

This document consolidates all MDAP (Massively Decomposed Agentic Processes) design work for the Ourocodus project, providing a complete guide from principles through Milestone 5 implementation.

---

## Table of Contents

### Part 1: Foundations
1. [Core Insight](#core-insight)
2. [Philosophical Foundation](#philosophical-foundation)
3. [Verification Hierarchy](#verification-hierarchy-for-coding)
4. [Rubric System](#rubric-system-design)
5. [The 10 MDAP Principles](#the-10-mdap-principles-applied-to-coding)

### Part 2: Architecture
6. [Agent Registry with Schemas](#agent-registry-with-schemas)
7. [Workflow Definition Format](#workflow-definition-format)
8. [Executor Design](#executor-design)
9. [Human Steering Integration](#human-steering-integration)
10. [Voting for Ambiguity](#voting-for-ambiguous-tasks)
11. [Micro-Agent Library](#micro-agent-library-primitives)

### Part 3: Milestone 5 Implementation
12. [Architecture Overview](#milestone-5-architecture-overview)
13. [Issue #121: Coordinator Service](#issue-121-coordinator-service-foundation)
14. [Issue #122: Workflow Persistence](#issue-122-workflow-persistence-with-sqlite)
15. [Issue #126: Workflow Parser](#issue-126-workflow-definition-parser)
16. [Issue #127: Execution Engine](#issue-127-sequential-workflow-execution-engine)
17. [Issue #59: Retry Logic](#issue-59-retry-logic-integration)
18. [Issue #53: Approval Gates](#issue-53-approval-gate-integration)
19. [Issue #124: NATS Events](#issue-124-nats-event-handlers)
20. [Issue #123: HTTP API](#issue-123-http-api-workflow-management)
21. [Issue #128: Task Lifecycle](#issue-128-task-lifecycle--monitoring)
22. [Issue #129: Workflow Operations](#issue-129-workflow-operations)
23. [Issue #125: Observability](#issue-125-basic-workflow-observability)
24. [Week-by-Week Roadmap](#week-by-week-implementation-roadmap)

### Part 4: Reference
25. [Complete Workflow Examples](#complete-workflow-examples)
26. [Testing Strategy](#testing-strategy)
27. [NATS Subject Design](#nats-subject-design)
28. [Idempotency Patterns](#idempotency-patterns)
29. [Deferred Features](#mdap-features-deferred-to-future-milestones)

---

# Part 1: Foundations

## Core Insight

**MDAPs succeed not because steps are small, but because each step is *verifiable*.**

For coding tasks: **Verification = rubrics, tests, constraints, specs**

This is our ground truth.

### Why This Matters

Traditional agent systems fail at scale because:
- Large steps accumulate errors
- No reliable way to detect failures
- Retry logic is blind (same broken agent)
- Context grows unbounded

MDAP succeeds because:
- Tiny steps are easy to verify
- Each step has explicit checks
- Failures are isolated and recoverable
- Context stays small and focused

---

## Philosophical Foundation

### Tower of Hanoi (Original MDAP)

The Anthropic MDAP paper demonstrated million-step execution with zero errors on Tower of Hanoi puzzles.

**Characteristics:**
- **Truth:** External, objective, deterministic
- **Verification:** "Is disk A on peg Y?" (boolean check)
- **Correctness:** Single right answer
- **Failure mode:** Move is either legal or illegal

### Coding (Ourocodus MDAP)

Coding has no absolute truth. We must **construct truth** through rubrics and constraints.

**Characteristics:**
- **Truth:** Constructed through rubrics and constraints
- **Verification:** "Does this satisfy the rubric?" (multi-dimensional check)
- **Correctness:** Acceptance criteria + quality thresholds
- **Failure mode:** Spectrum from "perfect" to "unacceptable"

### The Resolution

**We're not cheating by constructing truth. We're engineering correctness the same way compilers, linters, and CI systems do.**

Examples of constructed truth in software engineering:
- **Compilers:** Code either compiles or doesn't (constructed boolean)
- **Linters:** Code either passes style rules or doesn't (constructed boolean)
- **Test suites:** Tests either pass or fail (constructed boolean)
- **Type checkers:** Types either satisfy constraints or don't (constructed boolean)

**MDAP for coding uses the same principle:** Define explicit criteria, verify against them, iterate until passing.

---

## Verification Hierarchy for Coding

MDAP verification has 4 layers, each with increasing depth and cost.

### L0: Syntax Verification (Deterministic, Fast, Cheap)

**Question:** "Is this structurally valid?"

**Checks:**
- Does it parse? (JSON, YAML, Go, Python, etc.)
- Does it compile?
- Does it match the schema?
- Are required fields present?

**Agents:**
- `syntax-validator` - Parse and validate syntax
- `schema-validator` - Validate against JSON schema
- `type-checker` - Run type checker (Go: `go vet`, Python: `mypy`)

**Example:**
```yaml
verifiers:
  - type: syntax-validator
    params:
      language: go
    fail_on_error: true

  - type: schema-validator
    params:
      schema: |
        {
          "type": "object",
          "required": ["package", "imports", "functions"]
        }
    fail_on_error: true
```

**When to use:** After every code generation step. L0 is cheap and catches 80% of issues.

---

### L1: Semantic Verification (Business Logic, Constraints)

**Question:** "Does this satisfy the requirements?"

**Checks:**
- Do tests pass?
- Do constraints hold? (complexity, length, naming)
- Does it match the spec?
- Are edge cases covered?
- Does linting pass?

**Agents:**
- `test-runner` - Execute test suite
- `linter` - Run linter (golangci-lint, ruff, etc.)
- `constraint-checker` - Verify custom constraints
- `coverage-checker` - Ensure test coverage thresholds

**Example:**
```yaml
verifiers:
  - type: test-runner
    params:
      command: "go test ./..."
      timeout: 60s
    fail_on_error: true

  - type: linter
    params:
      tool: golangci-lint
      config: .golangci.yml
    fail_on_error: true

  - type: constraint-checker
    params:
      rules:
        - "cyclomatic_complexity < 10"
        - "function_length < 50 lines"
        - "test_coverage > 80%"
    fail_on_error: true
```

**When to use:** After code assembly, before integration. L1 verifies correctness.

---

### L2: Cross-Module Verification (Integration, Consistency)

**Question:** "Does this integrate correctly with the rest of the system?"

**Checks:**
- Do integration tests pass?
- Are interfaces consistent?
- Does it follow architectural patterns?
- Are dependencies compatible?
- Does it break existing functionality?

**Agents:**
- `integration-tester` - Run integration test suite
- `interface-checker` - Verify interface compatibility
- `dependency-analyzer` - Check dependency graph
- `regression-detector` - Detect breaking changes

**Example:**
```yaml
verifiers:
  - type: integration-tester
    params:
      suite: tests/integration
      timeout: 300s
    fail_on_error: true

  - type: interface-checker
    params:
      verify_against: pkg/coordinator/interfaces.go
    fail_on_error: true

  - type: regression-detector
    params:
      baseline: main
      run_tests: true
    fail_on_error: false  # Warn but don't block
```

**When to use:** After L1 passes, before human review. L2 verifies system consistency.

---

### L3: Human Verification (Judgment, Taste, Strategy)

**Question:** "Is this acceptable to humans?"

**Checks:**
- Does it meet quality standards?
- Is the architecture sound?
- Does it match project conventions?
- Is it maintainable?
- Does it align with long-term goals?

**Agents:**
- `code-reviewer` (AI) - Generate review comments
- `approval-gate` (Human) - Human reviews and approves
- `style-checker` - Verify adherence to style guide
- `documentation-validator` - Check documentation quality

**Example:**
```yaml
verifiers:
  - type: code-reviewer
    params:
      focus:
        - maintainability
        - security
        - performance
    generate_report: true

  - type: approval-gate
    params:
      title: "Review code changes"
      description: "Generated code ready for review"
      reviewers: ["user:alice", "user:bob"]
      timeout: 600s
    fail_on_rejection: true
```

**When to use:** After L2 passes, before deployment. L3 provides human judgment.

---

## Rubric System Design

### What is a Rubric?

A rubric is a **multi-dimensional scoring system** that defines "acceptable" for a given task.

Rubrics answer: "What does 'good code' mean for this specific context?"

**Structure:**
```yaml
rubric:
  name: "Go HTTP Handler Quality"
  dimensions:
    - name: correctness
      weight: 40%
      criteria:
        - "Tests pass"
        - "Handles error cases"
        - "Returns correct status codes"

    - name: code_quality
      weight: 30%
      criteria:
        - "golangci-lint passes"
        - "Cyclomatic complexity < 10"
        - "No code duplication"

    - name: maintainability
      weight: 20%
      criteria:
        - "Functions < 50 lines"
        - "Clear variable names"
        - "Documented public APIs"

    - name: security
      weight: 10%
      criteria:
        - "No SQL injection vulnerabilities"
        - "Input validation present"
        - "Secrets not in code"

  passing_threshold: 75%
```

### Rubric Evaluation Agent

```go
type RubricEvaluator struct {
    rubric *Rubric
}

type Rubric struct {
    Name              string
    Dimensions        []RubricDimension
    PassingThreshold  float64  // 0.0 - 1.0
}

type RubricDimension struct {
    Name     string
    Weight   float64
    Criteria []RubricCriterion
}

type RubricCriterion struct {
    Description string
    Evaluator   func(ctx context.Context, code string) (bool, error)
}

type RubricScore struct {
    OverallScore     float64
    DimensionScores  map[string]float64
    CriteriaResults  []CriterionResult
    Passed           bool
}

func (r *RubricEvaluator) Evaluate(ctx context.Context, code string) (*RubricScore, error) {
    score := &RubricScore{
        DimensionScores: make(map[string]float64),
    }

    for _, dimension := range r.rubric.Dimensions {
        dimensionScore := 0.0
        criteriaCount := len(dimension.Criteria)

        for _, criterion := range dimension.Criteria {
            passed, err := criterion.Evaluator(ctx, code)
            if err != nil {
                return nil, fmt.Errorf("criterion evaluation failed: %w", err)
            }

            result := CriterionResult{
                Dimension:   dimension.Name,
                Criterion:   criterion.Description,
                Passed:      passed,
            }
            score.CriteriaResults = append(score.CriteriaResults, result)

            if passed {
                dimensionScore += 1.0
            }
        }

        // Dimension score = (passed criteria / total criteria) * dimension weight
        dimensionScore = (dimensionScore / float64(criteriaCount)) * dimension.Weight
        score.DimensionScores[dimension.Name] = dimensionScore
        score.OverallScore += dimensionScore
    }

    score.Passed = score.OverallScore >= r.rubric.PassingThreshold

    return score, nil
}
```

### Using Rubrics in Workflows

```yaml
tasks:
  - id: evaluate-code-quality
    type: rubric-evaluator
    dependencies: [generate-code, run-tests, run-linter]
    params:
      rubric: ${workflow.spec.rubric}
    verifiers:
      - type: rubric-threshold
        params:
          threshold: 0.75
        fail_on_error: false  # Log score but don't block

  - id: quality-gate
    type: approval-gate
    dependencies: [evaluate-code-quality]
    condition: "evaluate-code-quality.score < 0.75"
    params:
      title: "Quality score below threshold"
      description: "Score: ${evaluate-code-quality.score}. Override?"
```

---

## The 10 MDAP Principles Applied to Coding

### 1. Decompose Way More Than Feels Natural

**Principle:** Break tasks into micro-steps so small they feel trivial.

**Problem:** Our natural instinct is to create "smart" agents that handle large, complex tasks.

**Solution:** Create "dumb" micro-agents that do one checkable thing.

**Example:**

❌ **Traditional (too coarse):**
```yaml
tasks:
  - id: add-endpoint
    type: code-generator
    prompt: "Add GET /v1/workflows/:id/status endpoint"
```

✅ **MDAP (micro-decomposed):**
```yaml
tasks:
  - id: parse-requirement
    type: requirement-parser

  - id: design-request-schema
    type: schema-designer

  - id: design-response-schema
    type: schema-designer

  - id: generate-handler-skeleton
    type: code-skeleton-generator

  - id: generate-validation-logic
    type: validation-generator

  - id: generate-business-logic
    type: logic-generator

  - id: generate-tests
    type: test-generator

  - id: verify-compilation
    type: syntax-validator

  - id: run-tests
    type: test-runner
```

Each step: **single responsibility, verifiable output**.

---

### 2. Make the Process Explicit (DAG, Not Soup)

**Principle:** Orchestrator controls flow explicitly. No "agent figures it out."

**Problem:** Letting agents decide execution order leads to unpredictable behavior.

**Solution:** Explicit DAG with dependencies, conditional branches, and retry boundaries.

**Example:**
```yaml
tasks:
  - id: validate-data
    type: schema-checker
    on_failure: repair-path  # Explicit failure routing

  - id: repair-path
    type: llm-repair-agent
    params:
      max_attempts: 3
    on_success: validate-data  # Loop back to validator
    on_exhausted: human-review # Escalate to human

  - id: human-review
    type: approval-gate
    params:
      question: "Data validation failed after 3 repairs. Continue or abort?"
      options: ["continue-with-partial", "abort"]
```

**Key insight:** The DAG structure makes failure modes explicit and recoverable.

---

### 3. Structured I/O Everywhere

**Principle:** Agents talk via typed schemas, not freeform text.

**Problem:** Untyped `any` blobs make verification impossible.

**Solution:** Every task has explicit input/output schemas validated at runtime.

**Implementation:**
```go
type AgentDefinition struct {
    Type          string
    InputSchema   *jsonschema.Schema
    OutputSchema  *jsonschema.Schema
    Handler       func(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

func (e *Executor) executeTask(ctx context.Context, task *Task) error {
    agent := e.registry.Get(task.Type)

    // Validate input
    if err := agent.InputSchema.Validate(task.Input); err != nil {
        return fmt.Errorf("input validation failed: %w", err)
    }

    // Execute
    output, err := agent.Handler(ctx, task.Input)
    if err != nil {
        return err
    }

    // Validate output
    if err := agent.OutputSchema.Validate(output); err != nil {
        // RED FLAG: Agent produced invalid output
        return fmt.Errorf("output validation failed: %w", err)
    }

    task.Output = output
    return nil
}
```

**This is L0 verification** - the foundation of MDAP reliability.

---

### 4. Verification Built-In, Not Afterthought

**Principle:** Every nontrivial step has explicit verifiers. Ask: "If this step is wrong, who catches it?"

**Problem:** Red flags are logged warnings, not gates.

**Solution:** Verifiers block task completion until checks pass.

**Implementation:**
```yaml
tasks:
  - id: generate-code
    type: code-generator
    output_schema: GoCode
    verifiers:
      # L0: Syntax check
      - type: syntax-validator
        language: python
        fail_on_error: true

      # L1: Semantic check
      - type: constraint-checker
        rules:
          - "imports only from allowed_modules"
          - "no eval() or exec() calls"
        fail_on_error: true

      # L2: Cross-step consistency
      - type: interface-checker
        verifies: "output matches expected_interface from parent task"
        fail_on_error: true
```

**Workflow:** Task produces output → Run verifiers → Only mark complete if all pass

---

### 5. Redundancy & Sampling for High-Stakes Steps

**Principle:** For critical decisions, run N variants and vote/rank.

**When to use:**
- Subjective outputs (naming, style)
- Multiple valid solutions (algorithm choice)
- High-risk operations (security-critical code)

**When NOT to use:**
- Deterministic operations (parsing, math)
- Already-verified outputs (passed L0/L1 checks)
- Low-value operations (logging)

**Implementation:**
```yaml
- id: choose-algorithm
  type: algorithm-selector
  strategy: ensemble-vote
  samples: 3
  voting:
    method: majority
    require_consensus: 2-of-3
    on_disagreement: human-review

  agents:
    - type: algorithm-selector-a
      params: {focus: performance}
    - type: algorithm-selector-b
      params: {focus: simplicity}
    - type: algorithm-selector-c
      params: {focus: maintainability}
```

---

### 6. Small, Specialized Models/Tools

**Principle:** Many narrow, cheap agents > one god-agent.

**Agent library categories:**

**Deterministic Agents (no LLM):**
- `json-parser`, `json-extractor`, `schema-validator`
- `http-executor`, `sql-query`, `jq-transform`
- `regex-matcher`, `hash-calculator`, `timestamp-generator`

**LLM-Powered Agents (small, focused prompts):**
- `text-classifier`, `entity-extractor`, `json-repairer`
- `text-summarizer`, `code-formatter`, `error-explainer`
- `constraint-checker` (LLM validates natural language rules)

**Key:** Each agent is a **microservice** with a single responsibility.

---

### 7. Short Context, External State

**Principle:** Don't pass entire history to every agent. Use summaries, IDs, external stores.

**Problem:** Long context prompts are expensive, slow, and hurt performance.

**Solution:** Agents only see explicitly declared inputs.

**Implementation:**
```go
type TaskContext struct {
    TaskID      string
    InstanceID  string
    Input       json.RawMessage  // Only this task's input

    // Access to specific prior outputs via keys, not full history
    GetPriorOutput func(taskID string) (json.RawMessage, error)

    // No access to:
    // - Full workflow definition
    // - All task results
    // - Execution history
}
```

**Workflow template language:**
```yaml
tasks:
  - id: transform-data
    type: jq-transform
    input:
      # Explicitly pull data from prior task
      source: ${tasks.fetch-data.output.data}
      # Or from workflow initial input
      config: ${workflow.input.transform_config}
```

---

### 8. Instrument Everything

**Principle:** Log all agent calls, track metrics, find flaky steps.

**What to log:**
```go
type TaskExecutionLog struct {
    TaskID        string
    InstanceID    string
    AgentType     string
    InputHash     string  // Hash if large
    OutputHash    string
    Duration      time.Duration
    Attempt       int
    VerifierResults []VerifierResult
    RedFlags      []string
    Cost          float64  // If LLM call
    Tokens        int      // If LLM call
}
```

**Queries we want:**
- "Which agent types have highest failure rate?"
- "Which verifiers fail most often?"
- "Which workflows get stuck in repair loops?"
- "What's the average cost per workflow?"
- "Where do humans intervene most?"

---

### 9. Failure Isolation & Recovery

**Principle:** Failures should be local, observable, recoverable.

**Repair policy (not just retry):**
```yaml
on_failure:
  - condition: "syntax_error"
    repair:
      - agent: syntax-repair-agent
        max_attempts: 2

  - condition: "tests_fail"
    repair:
      - agent: test-fixing-agent
        max_attempts: 3

  - condition: "rubric_score < 0.75"
    repair:
      - agent: quality-improvement-agent
        max_attempts: 2
      - escalate: human-review
```

**Different repair strategy for each failure mode.**

---

### 10. Safety from Structure, Not Vibes

**Principle:** Don't rely on "smart agent makes good choices." Use structure + checks.

**Agent capability model:**
```go
type AgentCapabilities struct {
    CanReadFiles    bool
    CanWriteFiles   bool
    CanMakeHTTPCall bool
    CanExecuteCode  bool
    AllowedDomains  []string
    AllowedPaths    []string
}
```

**Policy enforcement:**
```yaml
tasks:
  - id: delete-data
    type: database-delete-agent
    requires_approval: true  # Always gate destructive operations

  - id: deploy-to-production
    type: deploy-agent
    requires_approval: true
    approval_timeout: 300s
```

---

# Part 2: Architecture

## Agent Registry with Schemas

### Core Design

Every agent in Ourocodus must register with explicit schemas:

```go
// pkg/coordinator/registry/registry.go
package registry

type AgentRegistry struct {
    agents map[string]*AgentDefinition
    mu     sync.RWMutex
}

type AgentDefinition struct {
    Type          string
    Description   string
    InputSchema   *jsonschema.Schema
    OutputSchema  *jsonschema.Schema
    Capabilities  AgentCapabilities
    Handler       AgentHandler
}

type AgentHandler func(ctx context.Context, input json.RawMessage) (json.RawMessage, error)

type AgentCapabilities struct {
    CanReadFiles    bool
    CanWriteFiles   bool
    CanMakeHTTPCall bool
    CanExecuteCode  bool
    AllowedDomains  []string
    AllowedPaths    []string
}

func NewRegistry() *AgentRegistry {
    return &AgentRegistry{
        agents: make(map[string]*AgentDefinition),
    }
}

func (r *AgentRegistry) Register(agent *AgentDefinition) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if _, exists := r.agents[agent.Type]; exists {
        return fmt.Errorf("agent type %s already registered", agent.Type)
    }

    r.agents[agent.Type] = agent
    return nil
}

func (r *AgentRegistry) Get(agentType string) (*AgentDefinition, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    agent, exists := r.agents[agentType]
    if !exists {
        return nil, fmt.Errorf("agent type %s not found", agentType)
    }

    return agent, nil
}

func (r *AgentRegistry) List() []*AgentDefinition {
    r.mu.RLock()
    defer r.mu.RUnlock()

    agents := make([]*AgentDefinition, 0, len(r.agents))
    for _, agent := range r.agents {
        agents = append(agents, agent)
    }
    return agents
}
```

### Example Agent Registration

```go
// Register JSON parser agent
registry.Register(&AgentDefinition{
    Type:        "json-parser",
    Description: "Parse JSON string to object",
    InputSchema: &jsonschema.Schema{
        Type: "object",
        Properties: map[string]*jsonschema.Schema{
            "json_string": {Type: "string"},
        },
        Required: []string{"json_string"},
    },
    OutputSchema: &jsonschema.Schema{
        Type: "object",
        Properties: map[string]*jsonschema.Schema{
            "parsed_data": {Type: "object"},
        },
    },
    Capabilities: AgentCapabilities{
        CanReadFiles: false,
        CanWriteFiles: false,
    },
    Handler: func(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
        var req struct {
            JSONString string `json:"json_string"`
        }
        if err := json.Unmarshal(input, &req); err != nil {
            return nil, err
        }

        var parsed any
        if err := json.Unmarshal([]byte(req.JSONString), &parsed); err != nil {
            return nil, fmt.Errorf("failed to parse JSON: %w", err)
        }

        return json.Marshal(map[string]any{
            "parsed_data": parsed,
        })
    },
})
```

---

## Workflow Definition Format

### Ourocodus Workflow Spec

```yaml
apiVersion: ourocodus.dev/v1
kind: Workflow
metadata:
  name: data-processing-pipeline
  description: Sequential data processing with verification
  version: "1.0"

spec:
  # Optional: Define rubric for quality assessment
  rubric:
    name: "Data Pipeline Quality"
    dimensions:
      - name: correctness
        weight: 50%
        criteria:
          - "All tests pass"
          - "Data validation succeeds"
      - name: performance
        weight: 30%
        criteria:
          - "Processing time < 5 minutes"
      - name: reliability
        weight: 20%
        criteria:
          - "Error rate < 1%"
    passing_threshold: 0.80

  # Task definitions
  tasks:
    - id: fetch-data
      type: http-fetch-agent
      params:
        url: "https://api.example.com/data"
        method: GET
      timeout: 30s
      output_schema:
        type: object
        required: [data]

    - id: validate-data
      type: schema-validator
      dependencies: [fetch-data]
      input:
        data: ${tasks.fetch-data.output.data}
        schema: ${workflow.input.validation_schema}
      verifiers:
        - type: constraint-checker
          params:
            rules:
              - "data.length > 0"
          fail_on_error: true
      on_failure:
        - repair:
            agent: data-repair-agent
            max_attempts: 2
        - escalate: human-review

    - id: transform-data
      type: jq-transform
      dependencies: [validate-data]
      input:
        data: ${tasks.validate-data.output.validated_data}
        expression: '.[] | {id, value, timestamp: now}'

    - id: store-data
      type: postgres-writer
      dependencies: [transform-data]
      requires_approval: false  # Not destructive
      input:
        data: ${tasks.transform-data.output}
        connection: ${workflow.secrets.db_connection}
        table: "processed_data"
```

### Parser Implementation

```go
// pkg/coordinator/parser/parser.go
package parser

type WorkflowDefinition struct {
    APIVersion string   `yaml:"apiVersion"`
    Kind       string   `yaml:"kind"`
    Metadata   Metadata `yaml:"metadata"`
    Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
    Version     string `yaml:"version"`
}

type Spec struct {
    Rubric *Rubric `yaml:"rubric,omitempty"`
    Tasks  []Task  `yaml:"tasks"`
}

type Task struct {
    ID              string            `yaml:"id"`
    Type            string            `yaml:"type"`
    Dependencies    []string          `yaml:"dependencies,omitempty"`
    Input           map[string]any    `yaml:"input,omitempty"`
    Params          map[string]any    `yaml:"params,omitempty"`
    Timeout         string            `yaml:"timeout,omitempty"`
    OutputSchema    *Schema           `yaml:"output_schema,omitempty"`
    Verifiers       []Verifier        `yaml:"verifiers,omitempty"`
    OnFailure       *FailurePolicy    `yaml:"on_failure,omitempty"`
    RequiresApproval bool             `yaml:"requires_approval,omitempty"`
}

type Verifier struct {
    Type         string         `yaml:"type"`
    Params       map[string]any `yaml:"params,omitempty"`
    FailOnError  bool           `yaml:"fail_on_error"`
}

type FailurePolicy struct {
    Repair   []RepairAttempt `yaml:"repair,omitempty"`
    Escalate string          `yaml:"escalate,omitempty"`
}

type RepairAttempt struct {
    Agent       string `yaml:"agent"`
    MaxAttempts int    `yaml:"max_attempts"`
}

func Parse(yamlData []byte) (*WorkflowDefinition, error) {
    var def WorkflowDefinition
    if err := yaml.Unmarshal(yamlData, &def); err != nil {
        return nil, fmt.Errorf("failed to parse YAML: %w", err)
    }

    // Validate
    if err := validateWorkflow(&def); err != nil {
        return nil, err
    }

    return &def, nil
}

func validateWorkflow(def *WorkflowDefinition) error {
    // 1. All task IDs unique
    seen := make(map[string]bool)
    for _, task := range def.Spec.Tasks {
        if seen[task.ID] {
            return fmt.Errorf("duplicate task ID: %s", task.ID)
        }
        seen[task.ID] = true
    }

    // 2. All dependencies reference valid tasks
    for _, task := range def.Spec.Tasks {
        for _, dep := range task.Dependencies {
            if !seen[dep] {
                return fmt.Errorf("task %s depends on unknown task %s", task.ID, dep)
            }
        }
    }

    // 3. No circular dependencies
    if err := detectCycles(def.Spec.Tasks); err != nil {
        return err
    }

    return nil
}
```

---

## Executor Design

### Core Executor Structure

```go
// pkg/coordinator/engine/executor.go
package engine

type Executor struct {
    registry   *registry.AgentRegistry
    store      *storage.SQLiteStore
    natsClient *nats.Client
    config     *ExecutorConfig
}

type ExecutorConfig struct {
    MaxRetries      int           `env:"EXECUTOR_MAX_RETRIES" default:"3"`
    DefaultTimeout  time.Duration `env:"EXECUTOR_TIMEOUT" default:"5m"`
    FailFast        bool          `env:"EXECUTOR_FAIL_FAST" default:"true"`
}

type ExecutionContext struct {
    WorkflowID     string
    InstanceID     string
    CurrentTask    string
    CompletedTasks map[string]*TaskResult
    Status         Status
    StartedAt      time.Time
}

func (e *Executor) Execute(ctx context.Context, def *parser.WorkflowDefinition) (*ExecutionResult, error) {
    // Create workflow instance
    instance, err := e.store.CreateInstance(ctx, def.Metadata.Name)
    if err != nil {
        return nil, err
    }

    // Create execution context
    execCtx := &ExecutionContext{
        WorkflowID:     def.Metadata.Name,
        InstanceID:     instance.ID,
        CompletedTasks: make(map[string]*TaskResult),
        Status:         StatusRunning,
        StartedAt:      time.Now(),
    }

    // Sequential execution (MVP: no parallelism)
    for _, task := range def.Spec.Tasks {
        // Idempotency: check if already completed
        if execCtx.CompletedTasks[task.ID] != nil {
            continue
        }

        // Execute task with verification
        result, err := e.executeTaskWithVerification(ctx, execCtx, &task)
        if err != nil {
            if e.config.FailFast {
                execCtx.Status = StatusFailed
                e.store.UpdateInstanceStatus(ctx, instance.ID, StatusFailed)
                return nil, err
            }
            log.Error("task failed but continuing", "task_id", task.ID, "error", err)
        }

        // Update context
        execCtx.CompletedTasks[task.ID] = result
        execCtx.CurrentTask = task.ID

        // Serialize context (crash recovery)
        if err := e.serializeContext(ctx, execCtx); err != nil {
            return nil, fmt.Errorf("failed to serialize context: %w", err)
        }
    }

    execCtx.Status = StatusCompleted
    e.store.UpdateInstanceStatus(ctx, instance.ID, StatusCompleted)

    return &ExecutionResult{
        InstanceID: instance.ID,
        Status:     execCtx.Status,
        Duration:   time.Since(execCtx.StartedAt),
    }, nil
}
```

### Task Execution with Verification

```go
func (e *Executor) executeTaskWithVerification(ctx context.Context, execCtx *ExecutionContext, task *parser.Task) (*TaskResult, error) {
    // Get agent definition
    agent, err := e.registry.Get(task.Type)
    if err != nil {
        return nil, err
    }

    // Prepare input
    input, err := e.prepareTaskInput(execCtx, task)
    if err != nil {
        return nil, err
    }

    // L0: Validate input against agent's input schema
    if err := agent.InputSchema.Validate(input); err != nil {
        return nil, fmt.Errorf("L0 input validation failed: %w", err)
    }

    // Execute with retry/repair policy
    output, err := e.executeWithRecovery(ctx, agent, input, task.OnFailure)
    if err != nil {
        return nil, err
    }

    // L0: Validate output against agent's output schema
    if err := agent.OutputSchema.Validate(output); err != nil {
        return nil, fmt.Errorf("L0 output validation failed: %w", err)
    }

    // L1/L2: Run task-specific verifiers
    for _, verifier := range task.Verifiers {
        if err := e.runVerifier(ctx, verifier, output); err != nil {
            if verifier.FailOnError {
                return nil, fmt.Errorf("verifier %s failed: %w", verifier.Type, err)
            }
            log.Warn("verifier failed but not blocking", "verifier", verifier.Type, "error", err)
        }
    }

    return &TaskResult{
        TaskID: task.ID,
        Output: output,
        Status: TaskStateCompleted,
    }, nil
}
```

---

## Human Steering Integration

### Steering Points in Workflows

Humans can intervene at multiple points:

**1. Pre-Workflow (Planning):**
```yaml
- id: workflow-plan-review
  type: approval-gate
  params:
    title: "Review execution plan"
    description: "20 tasks planned. Review before execution?"
    options:
      - label: "Execute as planned"
      - label: "Modify plan"
      - label: "Cancel"
```

**2. Mid-Workflow (Checkpoints):**
```yaml
- id: generate-code
  type: code-generator
  steering_point: true  # Human can review intermediate result

- id: checkpoint
  type: approval-gate
  dependencies: [generate-code]
  params:
    title: "Review generated code"
    description: "Code generated. Continue to tests?"
    options:
      - label: "Looks good, continue"
      - label: "Regenerate with feedback"
      - label: "Edit manually"
    feedback_prompt: "What should be changed?"
```

**3. Post-Verification (Override Failures):**
```yaml
- id: verify-tests
  type: test-runner
  verifiers:
    - type: test-pass-checker
      fail_on_error: false  # Don't auto-fail

- id: human-override
  type: approval-gate
  dependencies: [verify-tests]
  condition: "verify-tests failed"
  params:
    title: "Tests failed. Override?"
    description: "3 tests failed. Review and decide."
    options:
      - label: "Fix tests and retry"
      - label: "Continue anyway"
      - label: "Abort workflow"
```

### Steering API

```go
// POST /v1/workflows/:id/steer
type SteerRequest struct {
    TaskID   string         `json:"task_id"`
    Action   string         `json:"action"`  // "continue", "regenerate", "edit", "abort"
    Feedback string         `json:"feedback,omitempty"`
    Edits    map[string]any `json:"edits,omitempty"`
}

func (a *API) SteerWorkflow(w http.ResponseWriter, r *http.Request) {
    var req SteerRequest
    json.NewDecoder(r.Body).Decode(&req)

    switch req.Action {
    case "continue":
        // Resume workflow execution
        a.executor.Resume(r.Context(), req.InstanceID)

    case "regenerate":
        // Restart task with feedback
        a.executor.RegenerateTask(r.Context(), req.TaskID, req.Feedback)

    case "edit":
        // Apply human edits, skip to next task
        a.executor.ApplyEdits(r.Context(), req.TaskID, req.Edits)

    case "abort":
        // Cancel workflow
        a.executor.Cancel(r.Context(), req.InstanceID)
    }
}
```

---

## Voting for Ambiguous Tasks

### When to Use Voting

✅ **Use voting when:**
- Task output is subjective (naming, code style)
- Multiple valid solutions exist (algorithm choice)
- Risk is high (security-critical code)
- Correctness is ambiguous (UX decisions)

❌ **Don't use voting when:**
- Task is deterministic (parsing JSON, running tests)
- Output is verifiable objectively (syntax checking)
- Cost is prohibitive (expensive LLM calls)

### Voting Implementation

```yaml
- id: choose-algorithm
  type: algorithm-selector
  strategy: ensemble-vote
  samples: 3
  voting:
    method: majority
    require_consensus: 2-of-3
    on_disagreement: human-review

  agents:
    - type: algorithm-selector-a
      params: {focus: performance}
    - type: algorithm-selector-b
      params: {focus: simplicity}
    - type: algorithm-selector-c
      params: {focus: maintainability}
```

```go
func (e *Executor) executeWithVoting(ctx context.Context, task *parser.Task) (*TaskResult, error) {
    results := make([]*TaskResult, len(task.Agents))

    // Execute all agents in parallel
    var wg sync.WaitGroup
    for i, agent := range task.Agents {
        wg.Add(1)
        go func(idx int, a *Agent) {
            defer wg.Done()
            result, err := e.executeAgent(ctx, a, task.Input)
            if err != nil {
                log.Error("agent failed", "agent", a.Type, "error", err)
                return
            }
            results[idx] = result
        }(i, agent)
    }
    wg.Wait()

    // Vote on results
    winner, consensus := e.vote(results, task.Voting)

    if !consensus {
        // Disagreement - escalate to human
        return e.escalateToHuman(ctx, task, results)
    }

    return winner, nil
}
```

---

## Micro-Agent Library (Primitives)

### Deterministic Agents (No LLM)

**Data Processing:**
- `json-parser` - Parse JSON string to object
- `json-extractor` - Extract field via JSONPath
- `json-merger` - Merge multiple JSON objects
- `jq-transform` - Apply JQ transformation
- `yaml-parser` - Parse YAML to object
- `xml-parser` - Parse XML to object

**Validation:**
- `schema-validator` - Validate against JSON schema
- `regex-matcher` - Match regex pattern
- `type-checker` - Verify data types
- `constraint-checker` - Check business rules

**HTTP:**
- `http-executor` - Execute HTTP request
- `http-status-validator` - Validate status code
- `header-extractor` - Extract HTTP headers

**Storage:**
- `sql-query` - Execute SQL query
- `redis-get` - Get value from Redis
- `redis-set` - Set value in Redis

**Utilities:**
- `hash-calculator` - Calculate hash (SHA256, MD5)
- `timestamp-generator` - Generate current timestamp
- `uuid-generator` - Generate UUID
- `base64-encoder` - Encode to base64
- `base64-decoder` - Decode from base64

### LLM-Powered Agents (Focused Prompts)

**Text Processing:**
- `text-classifier` - Classify text into categories
- `entity-extractor` - Extract named entities
- `text-summarizer` - Summarize text to N words
- `sentiment-analyzer` - Analyze sentiment

**Code Processing:**
- `syntax-repairer` - Fix syntax errors
- `test-fixer` - Fix failing tests
- `code-formatter` - Format code with style
- `error-explainer` - Explain error message
- `code-reviewer` - Generate review comments

**Data Repair:**
- `json-repairer` - Fix malformed JSON
- `data-normalizer` - Normalize inconsistent data
- `missing-field-filler` - Fill missing fields

---

*This completes Part 2: Architecture. Continue to [Part 3: Milestone 5 Implementation](#part-3-milestone-5-implementation)*

---

# Part 3: Milestone 5 Implementation

## Milestone 5 Architecture Overview

Milestone 5 delivers the **Coordinator service** - the orchestrator for MDAP workflows.

### Core Components

| Component | GitHub Issue | Purpose |
|-----------|--------------|---------|
| **Coordinator Service** | #121 | Service lifecycle, integration with relay |
| **SQLite Persistence** | #122 | Workflow definitions, instances, task state |
| **Workflow Parser** | #126 | Parse YAML workflows into internal DAG |
| **Sequential Executor** | #127 | Execute tasks sequentially with verification |
| **Retry Logic** | #59 | Two-layer retry (transport + task level) |
| **Approval Gates** | #53 | Human-in-the-loop decision points |
| **NATS Event Handlers** | #124 | Async communication with agents |
| **HTTP API** | #123 | Workflow management endpoints |
| **Task Lifecycle** | #128 | State tracking and monitoring |
| **Workflow Operations** | #129 | Cancellation, replay protection, audit |
| **Observability** | #125 | Debug endpoints and metrics |

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                      HTTP API (#123)                         │
│  POST /workflows, POST /workflows/:id/instances, etc.       │
└────────────────────┬────────────────────────────────────────┘
                     │
         ┌───────────▼──────────────┐
         │   Coordinator (#121)      │
         │   - Lifecycle mgmt        │
         │   - Session integration   │
         └───────────┬───────────────┘
                     │
         ┌───────────▼──────────────┐
         │   Parser (#126)           │
         │   - YAML → Internal DAG   │
         │   - Validation            │
         └───────────┬───────────────┘
                     │
         ┌───────────▼──────────────┐
         │   Executor (#127)         │
         │   - Sequential execution  │
         │   - Retry logic (#59)     │
         │   - Approval gates (#53)  │
         │   - Task lifecycle (#128) │
         └───┬──────────────────┬───┘
             │                  │
    ┌────────▼────────┐  ┌─────▼──────────┐
    │  SQLite (#122)  │  │  NATS (#124)   │
    │  - Definitions  │  │  - Task dispatch│
    │  - Instances    │  │  - Agent events │
    │  - Task state   │  │  - Async coord  │
    │  - Events       │  └────────────────┘
    │  - Audit log    │
    └─────────────────┘
```

---

## Issue #121: Coordinator Service Foundation

**Purpose:** Create the foundational coordinator service that orchestrates multi-agent workflows.

### Implementation

```go
// pkg/coordinator/coordinator.go
package coordinator

type Coordinator struct {
    sessionMgr  *session.Manager      // From M2
    natsClient  *nats.Client          // From M4
    stateStore  *storage.SQLiteStore  // Issue #122
    registry    *registry.AgentRegistry
    executor    *engine.Executor
    config      *Config
    mu          sync.RWMutex
}

type Config struct {
    DBPath           string        `env:"COORDINATOR_DB_PATH" default:"./coordinator.db"`
    NATSURLs         []string      `env:"COORDINATOR_NATS_URLS" default:"nats://localhost:4222"`
    DefaultTimeout   time.Duration `env:"COORDINATOR_TIMEOUT" default:"5m"`
    MaxRetries       int           `env:"COORDINATOR_MAX_RETRIES" default:"3"`
}

func New(cfg *Config, sessionMgr *session.Manager) (*Coordinator, error) {
    // Initialize SQLite store
    store, err := storage.NewSQLiteStore(cfg.DBPath)
    if err != nil {
        return nil, fmt.Errorf("failed to create store: %w", err)
    }

    // Connect to NATS
    natsClient, err := nats.NewClient(
        nats.WithURLs(cfg.NATSURLs...),
        nats.WithName("coordinator"),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to connect to NATS: %w", err)
    }

    // Create agent registry
    agentRegistry := registry.NewRegistry()

    // Create executor
    executor := engine.NewExecutor(agentRegistry, store, natsClient, &engine.ExecutorConfig{
        MaxRetries:     cfg.MaxRetries,
        DefaultTimeout: cfg.DefaultTimeout,
    })

    return &Coordinator{
        sessionMgr:  sessionMgr,
        natsClient:  natsClient,
        stateStore:  store,
        registry:    agentRegistry,
        executor:    executor,
        config:      cfg,
    }, nil
}

func (c *Coordinator) Start(ctx context.Context) error {
    // Restore in-progress workflows
    instances, err := c.stateStore.GetActiveInstances(ctx)
    if err != nil {
        return fmt.Errorf("failed to load active instances: %w", err)
    }

    for _, instance := range instances {
        log.Info("restoring workflow instance", "instance_id", instance.ID, "workflow_id", instance.WorkflowID)
        go func(inst *storage.WorkflowInstance) {
            if err := c.executor.RestoreAndResume(context.Background(), inst.ID); err != nil {
                log.Error("failed to restore instance", "instance_id", inst.ID, "error", err)
            }
        }(instance)
    }

    // Setup NATS event subscriptions
    if err := c.setupEventHandlers(ctx); err != nil {
        return fmt.Errorf("failed to setup event handlers: %w", err)
    }

    log.Info("coordinator started", "db_path", c.config.DBPath)
    return nil
}

func (c *Coordinator) Shutdown(ctx context.Context) error {
    log.Info("shutting down coordinator")

    // Drain NATS connection
    if err := c.natsClient.Drain(ctx); err != nil {
        log.Error("failed to drain NATS", "error", err)
    }

    // Close SQLite store
    if err := c.stateStore.Close(); err != nil {
        return fmt.Errorf("failed to close store: %w", err)
    }

    log.Info("coordinator shut down complete")
    return nil
}

func (c *Coordinator) setupEventHandlers(ctx context.Context) error {
    // Subscribe to agent task completion events
    _, err := c.natsClient.Subscribe(ctx, "agents.*.completed", c.handleTaskCompleted)
    if err != nil {
        return err
    }

    // Subscribe to agent task failure events
    _, err = c.natsClient.Subscribe(ctx, "agents.*.failed", c.handleTaskFailed)
    if err != nil {
        return err
    }

    return nil
}
```

### Integration with Relay Architecture

```go
// cmd/relay/main.go - Add coordinator initialization
func main() {
    // ... existing relay setup ...

    // Initialize coordinator
    coord, err := coordinator.New(&coordinator.Config{
        DBPath:   os.Getenv("COORDINATOR_DB_PATH"),
        NATSURLs: strings.Split(os.Getenv("NATS_URLS"), ","),
    }, sessionManager)
    if err != nil {
        log.Fatal("failed to create coordinator", "error", err)
    }

    // Start coordinator
    if err := coord.Start(ctx); err != nil {
        log.Fatal("failed to start coordinator", "error", err)
    }
    defer coord.Shutdown(context.Background())

    // ... existing relay server startup ...
}
```

---

## Issue #122: Workflow Persistence with SQLite

**Purpose:** Implement SQLite-based persistence for workflow definitions, state, and execution history.

### Schema Design

```sql
-- Workflow definitions (templates)
CREATE TABLE workflow_definitions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    yaml_definition TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Workflow instances (executions)
CREATE TABLE workflow_instances (
    id TEXT PRIMARY KEY,
    definition_id TEXT NOT NULL,
    status TEXT NOT NULL,  -- PENDING, RUNNING, COMPLETED, FAILED, CANCELLED
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    execution_context TEXT,  -- JSON-encoded ExecutionContext for crash recovery
    FOREIGN KEY (definition_id) REFERENCES workflow_definitions(id)
);

-- Task executions
CREATE TABLE task_executions (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    agent_id TEXT,
    agent_type TEXT NOT NULL,
    status TEXT NOT NULL,  -- PENDING, ASSIGNED, RUNNING, COMPLETED, FAILED
    input TEXT,            -- JSON-encoded input
    result TEXT,           -- JSON-encoded result
    error TEXT,
    attempts INTEGER DEFAULT 1,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    duration_ms INTEGER,
    FOREIGN KEY (instance_id) REFERENCES workflow_instances(id)
);

-- Task attempts (for retry tracking)
CREATE TABLE task_attempts (
    id TEXT PRIMARY KEY,
    task_execution_id TEXT NOT NULL,
    attempt_number INTEGER NOT NULL,
    agent_id TEXT,
    status TEXT NOT NULL,
    result TEXT,
    error TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    duration_ms INTEGER,
    FOREIGN KEY (task_execution_id) REFERENCES task_executions(id),
    UNIQUE(task_execution_id, attempt_number)
);

-- Workflow events (audit trail)
CREATE TABLE workflow_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id TEXT NOT NULL,
    event_type TEXT NOT NULL,  -- workflow.started, task.completed, etc.
    correlation_id TEXT,        -- For tracing async flows
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    data TEXT,                  -- JSON-encoded event data
    FOREIGN KEY (instance_id) REFERENCES workflow_instances(id)
);

-- Approval requests (for approval gates)
CREATE TABLE approval_requests (
    id TEXT PRIMARY KEY,
    task_execution_id TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    approvers TEXT NOT NULL,    -- JSON array of user IDs
    status TEXT NOT NULL,       -- pending, approved, rejected
    approved_by TEXT,
    comment TEXT,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    FOREIGN KEY (task_execution_id) REFERENCES task_executions(id),
    FOREIGN KEY (instance_id) REFERENCES workflow_instances(id)
);

-- Indexes
CREATE INDEX idx_instances_status ON workflow_instances(status);
CREATE INDEX idx_instances_definition ON workflow_instances(definition_id);
CREATE INDEX idx_tasks_instance ON task_executions(instance_id);
CREATE INDEX idx_tasks_status ON task_executions(status);
CREATE INDEX idx_attempts_execution ON task_attempts(task_execution_id);
CREATE INDEX idx_events_instance ON workflow_events(instance_id);
CREATE INDEX idx_events_correlation ON workflow_events(correlation_id);
CREATE INDEX idx_approvals_status ON approval_requests(status);
CREATE INDEX idx_approvals_expires ON approval_requests(expires_at);
```

### Repository Implementation

```go
// pkg/coordinator/storage/sqlite.go
package storage

type SQLiteStore struct {
    db *sql.DB
    mu sync.RWMutex
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return nil, err
    }

    store := &SQLiteStore{db: db}

    // Initialize schema
    if err := store.initSchema(); err != nil {
        return nil, err
    }

    return store, nil
}

func (s *SQLiteStore) initSchema() error {
    // Execute schema from above
    _, err := s.db.Exec(schema)
    return err
}

// Workflow definitions
func (s *SQLiteStore) SaveWorkflowDefinition(ctx context.Context, def *WorkflowDefinition) error {
    query := `INSERT INTO workflow_definitions (id, name, yaml_definition, created_at, updated_at)
              VALUES (?, ?, ?, ?, ?)
              ON CONFLICT(id) DO UPDATE SET yaml_definition=excluded.yaml_definition, updated_at=excluded.updated_at`

    _, err := s.db.ExecContext(ctx, query, def.ID, def.Name, def.YAMLDefinition, time.Now(), time.Now())
    return err
}

func (s *SQLiteStore) GetWorkflowDefinition(ctx context.Context, id string) (*WorkflowDefinition, error) {
    query := `SELECT id, name, yaml_definition, created_at, updated_at FROM workflow_definitions WHERE id = ?`

    var def WorkflowDefinition
    err := s.db.QueryRowContext(ctx, query, id).Scan(&def.ID, &def.Name, &def.YAMLDefinition, &def.CreatedAt, &def.UpdatedAt)
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    return &def, err
}

// Workflow instances
func (s *SQLiteStore) CreateInstance(ctx context.Context, defID string) (*WorkflowInstance, error) {
    id := generateInstanceID()
    query := `INSERT INTO workflow_instances (id, definition_id, status, started_at) VALUES (?, ?, ?, ?)`

    _, err := s.db.ExecContext(ctx, query, id, defID, "PENDING", time.Now())
    if err != nil {
        return nil, err
    }

    return &WorkflowInstance{
        ID:           id,
        DefinitionID: defID,
        Status:       "PENDING",
        StartedAt:    time.Now(),
    }, nil
}

func (s *SQLiteStore) UpdateInstanceStatus(ctx context.Context, id string, status string) error {
    query := `UPDATE workflow_instances SET status = ?, completed_at = ? WHERE id = ?`

    var completedAt *time.Time
    if status == "COMPLETED" || status == "FAILED" || status == "CANCELLED" {
        now := time.Now()
        completedAt = &now
    }

    _, err := s.db.ExecContext(ctx, query, status, completedAt, id)
    return err
}

func (s *SQLiteStore) GetActiveInstances(ctx context.Context) ([]*WorkflowInstance, error) {
    query := `SELECT id, definition_id, status, started_at, execution_context
              FROM workflow_instances
              WHERE status IN ('PENDING', 'RUNNING')`

    rows, err := s.db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var instances []*WorkflowInstance
    for rows.Next() {
        var inst WorkflowInstance
        var execCtxJSON sql.NullString

        err := rows.Scan(&inst.ID, &inst.DefinitionID, &inst.Status, &inst.StartedAt, &execCtxJSON)
        if err != nil {
            return nil, err
        }

        if execCtxJSON.Valid {
            json.Unmarshal([]byte(execCtxJSON.String), &inst.ExecutionContext)
        }

        instances = append(instances, &inst)
    }

    return instances, rows.Err()
}

// Task executions
func (s *SQLiteStore) CreateTaskExecution(ctx context.Context, exec *TaskExecution) error {
    query := `INSERT INTO task_executions (id, instance_id, task_id, agent_type, status, input, started_at)
              VALUES (?, ?, ?, ?, ?, ?, ?)`

    inputJSON, _ := json.Marshal(exec.Input)
    _, err := s.db.ExecContext(ctx, query, exec.ID, exec.InstanceID, exec.TaskID, exec.AgentType, exec.Status, inputJSON, time.Now())
    return err
}

func (s *SQLiteStore) UpdateTaskExecution(ctx context.Context, exec *TaskExecution) error {
    query := `UPDATE task_executions
              SET status = ?, result = ?, error = ?, completed_at = ?, duration_ms = ?
              WHERE id = ?`

    resultJSON, _ := json.Marshal(exec.Result)
    _, err := s.db.ExecContext(ctx, query, exec.Status, resultJSON, exec.Error, exec.CompletedAt, exec.DurationMs, exec.ID)
    return err
}

// Events
func (s *SQLiteStore) RecordEvent(ctx context.Context, event *WorkflowEvent) error {
    query := `INSERT INTO workflow_events (instance_id, event_type, correlation_id, data)
              VALUES (?, ?, ?, ?)`

    dataJSON, _ := json.Marshal(event.Data)
    _, err := s.db.ExecContext(ctx, query, event.InstanceID, event.EventType, event.CorrelationID, dataJSON)
    return err
}

func (s *SQLiteStore) Close() error {
    return s.db.Close()
}
```

---

## Issue #126: Workflow Definition Parser

**Component Mapping:** `oos/core/dag.py` → `pkg/coordinator/parser/parser.go`

### Purpose

Parse and validate YAML/JSON workflow definitions, convert to internal DAG representation, and validate structural correctness.

### Workflow Definition Format

```yaml
apiVersion: ourocodus.dev/v1
kind: Workflow
metadata:
  name: data-processing-pipeline
  description: Sequential data processing workflow
spec:
  tasks:
    - id: fetch-data
      type: http-fetch-agent
      params:
        url: "https://api.example.com/data"
      timeout: 30s

    - id: validate-data
      type: validation-agent
      dependencies:
        - fetch-data
      params:
        schema: "data-schema-v1"

    - id: transform-data
      type: transform-agent
      dependencies:
        - validate-data
      params:
        operation: "normalize"

    - id: store-data
      type: storage-agent
      dependencies:
        - transform-data
      params:
        destination: "postgresql://..."
```

### Implementation

```go
// pkg/coordinator/parser/parser.go
package parser

type WorkflowDefinition struct {
    APIVersion string   `yaml:"apiVersion"`
    Kind       string   `yaml:"kind"`
    Metadata   Metadata `yaml:"metadata"`
    Spec       Spec     `yaml:"spec"`
}

type Spec struct {
    Tasks []Task `yaml:"tasks"`
}

type Task struct {
    ID           string            `yaml:"id"`
    Type         string            `yaml:"type"`        // Maps to MDAP agent_type
    Dependencies []string          `yaml:"dependencies"` // Maps to MDAP dependencies
    Params       map[string]any    `yaml:"params"`      // Maps to MDAP config
    Timeout      string            `yaml:"timeout,omitempty"`
}

// Parser validates and converts YAML → internal DAG
func Parse(yamlData []byte) (*InternalDAG, error) {
    var def WorkflowDefinition
    if err := yaml.Unmarshal(yamlData, &def); err != nil {
        return nil, err
    }

    // Validate
    if err := validateWorkflow(&def); err != nil {
        return nil, err
    }

    // Build DAG
    return buildDAG(&def)
}

func validateWorkflow(def *WorkflowDefinition) error {
    // MDAP-inspired validation
    // 1. All task IDs unique
    // 2. All dependencies reference valid tasks
    // 3. No circular dependencies
    // 4. Task types are supported
    return nil
}
```

### MDAP Alignment

- Each task = MicroAgent (single responsibility)
- Dependencies = DAG edges
- Sequential execution enforced by dependency chain
- Validation prevents malformed workflows

---

## Issue #127: Sequential Workflow Execution Engine

**Component Mapping:** `oos/core/executor.py` → `pkg/coordinator/engine/executor.go`

### Purpose

Execute workflows sequentially with explicit state management, crash recovery, and idempotency guarantees.

### Key Features

1. **Sequential Execution:** Tasks execute in dependency order (no parallelism in M5)
2. **Explicit State:** Execution context serialized to SQLite after each task
3. **Crash Recovery:** Coordinator restarts resume from last completed task
4. **Idempotency:** Completed tasks never re-execute
5. **Timeout Handling:** Per-task timeouts with default fallback

### Implementation

```go
// pkg/coordinator/engine/executor.go
package engine

type ExecutionContext struct {
    WorkflowID     string
    InstanceID     string
    CurrentTask    string
    CompletedTasks map[string]*TaskResult
    Status         Status
    StartedAt      time.Time
}

type Executor struct {
    store      *storage.SQLiteStore
    natsClient *nats.Conn
    config     *Config
}

func (e *Executor) Execute(ctx context.Context, def *parser.WorkflowDefinition) (*ExecutionResult, error) {
    // Create instance
    instance, err := e.store.CreateInstance(ctx, def.ID)
    if err != nil {
        return nil, err
    }

    // Create execution context
    execCtx := &ExecutionContext{
        WorkflowID:     def.ID,
        InstanceID:     instance.ID,
        CompletedTasks: make(map[string]*TaskResult),
        Status:         StatusRunning,
        StartedAt:      time.Now(),
    }

    // Sequential execution (MVP: no parallelism)
    for _, task := range def.Spec.Tasks {
        // Idempotency: check if task already completed
        if execCtx.CompletedTasks[task.ID] != nil {
            continue  // Skip already-completed tasks
        }

        // Execute task
        result, err := e.executeTask(ctx, execCtx, task)
        if err != nil {
            execCtx.Status = StatusFailed
            e.store.UpdateInstanceStatus(ctx, instance.ID, StatusFailed)
            return nil, err
        }

        // Update context
        execCtx.CompletedTasks[task.ID] = result
        execCtx.CurrentTask = task.ID

        // Serialize context to SQLite (critical for crash recovery)
        if err := e.serializeContext(ctx, execCtx); err != nil {
            return nil, fmt.Errorf("failed to serialize context: %w", err)
        }

        // Record event
        e.store.RecordEvent(ctx, &storage.WorkflowEvent{
            InstanceID:    instance.ID,
            EventType:     "task.completed",
            CorrelationID: task.ID,
            Data:          result,
        })
    }

    execCtx.Status = StatusCompleted
    e.store.UpdateInstanceStatus(ctx, instance.ID, StatusCompleted)
    return &ExecutionResult{/* ... */}, nil
}

func (e *Executor) executeTask(ctx context.Context, execCtx *ExecutionContext, task *parser.Task) (*TaskResult, error) {
    // Dispatch task via NATS
    taskMsg := &TaskMessage{
        TaskID:     task.ID,
        InstanceID: execCtx.InstanceID,
        Type:       task.Type,
        Params:     task.Params,
    }

    // Publish to NATS subject: agents.<task.Type>
    subject := fmt.Sprintf("agents.%s", task.Type)

    // Wait for completion (with timeout)
    timeout := task.Timeout
    if timeout == 0 {
        timeout = e.config.DefaultTimeout
    }

    resultChan := make(chan *TaskResult, 1)
    errChan := make(chan error, 1)

    // Subscribe to completion events
    sub, err := e.natsClient.Subscribe(fmt.Sprintf("agents.%s.result", task.ID), func(msg *nats.Msg) {
        var result TaskResult
        if err := json.Unmarshal(msg.Data, &result); err != nil {
            errChan <- err
            return
        }
        resultChan <- &result
    })
    if err != nil {
        return nil, err
    }
    defer sub.Unsubscribe()

    // Publish task
    if err := e.natsClient.Publish(subject, taskMsg); err != nil {
        return nil, err
    }

    // Wait for result or timeout
    select {
    case result := <-resultChan:
        return result, nil
    case err := <-errChan:
        return nil, err
    case <-time.After(timeout):
        return nil, fmt.Errorf("task timeout after %v", timeout)
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

func (e *Executor) serializeContext(ctx context.Context, execCtx *ExecutionContext) error {
    // Store execution context in SQLite
    // This enables crash recovery: coordinator restarts, loads context, resumes from last completed task
    return e.store.UpdateInstanceContext(ctx, execCtx.InstanceID, execCtx)
}

func (e *Executor) RestoreAndResume(ctx context.Context, instanceID string) error {
    // Load execution context from SQLite
    execCtx, err := e.store.LoadInstanceContext(ctx, instanceID)
    if err != nil {
        return err
    }

    // Resume execution from current task
    def, err := e.store.GetWorkflowDefinition(ctx, execCtx.WorkflowID)
    if err != nil {
        return err
    }

    // Skip completed tasks, continue from current
    // ... (same execution loop, but starting from execCtx.CurrentTask)

    return nil
}
```

### MDAP Alignment

- **Idempotency:** Check `CompletedTasks` before execution
- **Explicit Serialization:** `serializeContext()` after each task
- **Crash Recovery:** `RestoreAndResume()` loads context and continues
- **Sequential Execution:** Simple for-loop over tasks
- **Event Logging:** Record all state transitions

---

## Issue #59: Retry Logic Integration

**Component:** Two-layer retry design integrated with executor

### The Two Layers

#### Layer 1: Transport Retry (NATS Client) ✅ Already Implemented

Location: `pkg/nats/client.go:203-236`

Handles:
- Failed message publishes (network issues)
- Request timeouts (server overload)
- Connection drops (transient disconnects)

Retry policy:
- Exponential backoff
- Max attempts: configurable
- Only retries transient errors

#### Layer 2: Task Retry (Coordinator) 🔨 This Issue

Location: `pkg/coordinator/engine/executor.go` (enhancement to executeTask)

Handles:
- Agent crashes/failures during task execution
- Agent heartbeat timeout
- Task validation failures
- Business logic errors

Retry policy:
- Max 3 attempts per task
- Exponential backoff (2s → 4s → 8s)
- Reassign to different agent on retry

### Schema Enhancement

```sql
-- Track each retry attempt separately
CREATE TABLE task_attempts (
    id TEXT PRIMARY KEY,
    task_execution_id TEXT NOT NULL,
    attempt_number INTEGER NOT NULL,
    agent_id TEXT,
    status TEXT NOT NULL,      -- RUNNING, COMPLETED, FAILED
    result TEXT,
    error TEXT,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    duration_ms INTEGER,
    FOREIGN KEY (task_execution_id) REFERENCES task_executions(id),
    UNIQUE(task_execution_id, attempt_number)
);

CREATE INDEX idx_attempts_execution ON task_attempts(task_execution_id);
```

### Implementation

```go
type RetryPolicy struct {
    MaxAttempts       int           `env:"RETRY_MAX_ATTEMPTS" default:"3"`
    InitialBackoff    time.Duration `env:"RETRY_INITIAL_BACKOFF" default:"2s"`
    MaxBackoff        time.Duration `env:"RETRY_MAX_BACKOFF" default:"30s"`
    BackoffMultiplier float64       `env:"RETRY_BACKOFF_MULTIPLIER" default:"2.0"`
    ReassignOnRetry   bool          `env:"RETRY_REASSIGN_AGENT" default:"true"`
}

func (e *Executor) executeTask(ctx context.Context, execCtx *ExecutionContext, task *parser.Task) (*TaskResult, error) {
    // Get existing task execution record (idempotency check)
    taskExec, err := e.store.GetTaskExecution(ctx, execCtx.InstanceID, task.ID)
    if err != nil && err != storage.ErrNotFound {
        return nil, err
    }

    // Idempotency: Task already completed, skip execution
    if taskExec != nil && taskExec.Status == TaskStateCompleted {
        log.Info("task already completed, skipping",
            "task_id", task.ID,
            "instance_id", execCtx.InstanceID,
            "attempts", taskExec.Attempts)
        return taskExec.Result, nil
    }

    // Create or update task execution record
    if taskExec == nil {
        taskExec = &TaskExecution{
            ID:         generateTaskExecutionID(),
            InstanceID: execCtx.InstanceID,
            TaskID:     task.ID,
            Status:     TaskStatePending,
            Attempts:   0,
        }
        if err := e.store.CreateTaskExecution(ctx, taskExec); err != nil {
            return nil, err
        }
    }

    // Retry loop
    var lastErr error
    for attempt := 1; attempt <= e.config.Retry.MaxAttempts; attempt++ {
        // Update attempts counter
        taskExec.Attempts = attempt

        // Backoff before retry (skip on first attempt)
        if attempt > 1 {
            backoff := e.calculateBackoff(attempt)
            log.Info("retrying task after backoff",
                "task_id", task.ID,
                "attempt", attempt,
                "backoff", backoff)

            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(backoff):
            }
        }

        // Select agent for this attempt
        agentID, err := e.selectAgent(ctx, task, taskExec, attempt)
        if err != nil {
            lastErr = fmt.Errorf("agent selection failed: %w", err)
            continue
        }

        // Record attempt in database
        attemptID := generateAttemptID()
        attemptRecord := &TaskAttempt{
            ID:              attemptID,
            TaskExecutionID: taskExec.ID,
            AttemptNumber:   attempt,
            AgentID:         agentID,
            Status:          TaskStateRunning,
            StartedAt:       time.Now(),
        }
        if err := e.store.CreateTaskAttempt(ctx, attemptRecord); err != nil {
            log.Error("failed to record task attempt", "error", err)
        }

        // Execute task with this agent
        result, err := e.executeTaskAttempt(ctx, task, agentID, attempt)

        // Record attempt result
        attemptRecord.CompletedAt = time.Now()
        attemptRecord.DurationMs = int(time.Since(attemptRecord.StartedAt).Milliseconds())

        if err == nil {
            // Success!
            attemptRecord.Status = TaskStateCompleted
            attemptRecord.Result = result
            e.store.UpdateTaskAttempt(ctx, attemptRecord)

            // Update task execution
            taskExec.Status = TaskStateCompleted
            taskExec.Result = result
            taskExec.AgentID = agentID
            taskExec.CompletedAt = time.Now()
            e.store.UpdateTaskExecution(ctx, taskExec)

            log.Info("task completed successfully",
                "task_id", task.ID,
                "agent_id", agentID,
                "attempts", attempt)

            return result, nil
        }

        // Failure - record it
        lastErr = err
        attemptRecord.Status = TaskStateFailed
        attemptRecord.Error = err.Error()
        e.store.UpdateTaskAttempt(ctx, attemptRecord)

        log.Warn("task attempt failed",
            "task_id", task.ID,
            "agent_id", agentID,
            "attempt", attempt,
            "error", err,
            "will_retry", attempt < e.config.Retry.MaxAttempts)

        // Classify error
        if isPermanentError(err) {
            log.Error("permanent error, aborting retries",
                "task_id", task.ID,
                "error", err)
            break
        }

        // Check context before retrying
        if ctx.Err() != nil {
            return nil, ctx.Err()
        }
    }

    // All retries exhausted
    taskExec.Status = TaskStateFailed
    taskExec.Error = lastErr.Error()
    e.store.UpdateTaskExecution(ctx, taskExec)

    return nil, fmt.Errorf("task failed after %d attempts: %w", e.config.Retry.MaxAttempts, lastErr)
}

func (e *Executor) calculateBackoff(attempt int) time.Duration {
    // Exponential backoff: initialBackoff * (multiplier ^ (attempt - 1))
    backoff := float64(e.config.Retry.InitialBackoff) *
               math.Pow(e.config.Retry.BackoffMultiplier, float64(attempt-1))

    // Cap at max backoff
    if time.Duration(backoff) > e.config.Retry.MaxBackoff {
        return e.config.Retry.MaxBackoff
    }

    return time.Duration(backoff)
}

func isPermanentError(err error) bool {
    if err == nil {
        return false
    }

    errStr := err.Error()

    // Permanent errors
    permanentPatterns := []string{
        "validation failed",
        "invalid input",
        "schema mismatch",
        "permission denied",
        "not found",
        "already exists",
        "malformed",
    }

    for _, pattern := range permanentPatterns {
        if strings.Contains(errStr, pattern) {
            return true
        }
    }

    // Default: assume transient (retry)
    return false
}
```

### Key Features

1. **Idempotency:** Check `task_executions` before execution, skip if already completed
2. **Retry Tracking:** Store each attempt in `task_attempts` table for full audit trail
3. **Exponential Backoff:** 2s → 4s → 8s (configurable)
4. **Agent Reassignment:** Use NATS queue groups to route retry to different agent
5. **Permanent vs. Transient Errors:** Don't retry validation failures

---

## Issue #53: Approval Gate Integration

**Purpose:** Human-in-the-loop workflow integration with timeout and approval gates

### Design: Approval as Special Task Type

Approval gates are special workflow tasks that block until human responds.

### Workflow Example

```yaml
apiVersion: ourocodus.dev/v1
kind: Workflow
metadata:
  name: production-deployment
spec:
  tasks:
    - id: build-artifact
      type: build-agent
      params:
        source: "main"

    - id: run-tests
      type: test-agent
      dependencies:
        - build-artifact

    # Approval gate: blocks here
    - id: approve-deployment
      type: approval-gate
      dependencies:
        - run-tests
      params:
        title: "Deploy to production?"
        description: "Tests passed. Ready to deploy?"
        timeout: 300s  # 5 minutes
        approvers:
          - "user:alice"
          - "user:bob"

    - id: deploy-production
      type: deploy-agent
      dependencies:
        - approve-deployment  # Blocks until approved!
      params:
        environment: "production"
```

### Implementation

```go
// pkg/coordinator/agents/approval.go
package agents

type ApprovalGateAgent struct {
    store      *storage.SQLiteStore
    natsClient *nats.Client
}

func (a *ApprovalGateAgent) Execute(ctx context.Context, task *parser.Task) (*TaskResult, error) {
    // Extract params
    title := task.Params["title"].(string)
    description := task.Params["description"].(string)
    timeout := task.Params["timeout"].(time.Duration)
    approvers := task.Params["approvers"].([]string)

    // Create approval request record
    approvalReq := &ApprovalRequest{
        ID:          generateApprovalID(),
        TaskID:      task.ID,
        InstanceID:  task.InstanceID,
        Title:       title,
        Description: description,
        Approvers:   approvers,
        Status:      ApprovalStatusPending,
        CreatedAt:   time.Now(),
        ExpiresAt:   time.Now().Add(timeout),
    }

    if err := a.store.CreateApprovalRequest(ctx, approvalReq); err != nil {
        return nil, fmt.Errorf("failed to create approval request: %w", err)
    }

    // Publish approval request to NATS (for PWA/UI to consume)
    approvalEvent := &events.ApprovalRequested{
        ApprovalID:    approvalReq.ID,
        TaskID:        task.ID,
        InstanceID:    task.InstanceID,
        Title:         title,
        Description:   description,
        Approvers:     approvers,
        ExpiresAt:     approvalReq.ExpiresAt,
        CorrelationID: generateCorrelationID(),
    }

    if err := a.natsClient.Publish(ctx, "approvals.requested", approvalEvent); err != nil {
        return nil, fmt.Errorf("failed to publish approval request: %w", err)
    }

    log.Info("approval request created",
        "approval_id", approvalReq.ID,
        "task_id", task.ID,
        "expires_at", approvalReq.ExpiresAt)

    // Block until approved, rejected, or timeout
    resultChan := make(chan *ApprovalResult, 1)
    errChan := make(chan error, 1)

    // Subscribe to approval responses
    responseSubject := fmt.Sprintf("approvals.response.%s", approvalReq.ID)
    sub, err := a.natsClient.Subscribe(ctx, responseSubject, func(ctx context.Context, msg *nats.Message) error {
        var result ApprovalResult
        if err := json.Unmarshal(msg.Data, &result); err != nil {
            errChan <- err
            return nil
        }
        resultChan <- &result
        return nil
    })
    if err != nil {
        return nil, fmt.Errorf("failed to subscribe to approval responses: %w", err)
    }
    defer sub.Unsubscribe(context.Background())

    // Wait for response or timeout
    timeoutTimer := time.NewTimer(timeout)
    defer timeoutTimer.Stop()

    select {
    case result := <-resultChan:
        // Human responded!
        approvalReq.Status = ApprovalStatus(result.Status)
        approvalReq.ApprovedBy = result.ApprovedBy
        approvalReq.Comment = result.Comment
        approvalReq.CompletedAt = time.Now()
        a.store.UpdateApprovalRequest(ctx, approvalReq)

        if result.Status == "approved" {
            log.Info("approval granted",
                "approval_id", approvalReq.ID,
                "approved_by", result.ApprovedBy)

            return &TaskResult{
                Data: map[string]any{
                    "status":      "approved",
                    "approved_by": result.ApprovedBy,
                    "comment":     result.Comment,
                },
            }, nil
        } else {
            log.Warn("approval rejected",
                "approval_id", approvalReq.ID,
                "rejected_by", result.ApprovedBy)

            return nil, fmt.Errorf("approval rejected by %s: %s", result.ApprovedBy, result.Comment)
        }

    case err := <-errChan:
        return nil, err

    case <-timeoutTimer.C:
        // Timeout - default reject
        log.Warn("approval timeout, auto-rejecting",
            "approval_id", approvalReq.ID,
            "timeout", timeout)

        approvalReq.Status = ApprovalStatusRejected
        approvalReq.Comment = fmt.Sprintf("Timeout after %v", timeout)
        approvalReq.CompletedAt = time.Now()
        a.store.UpdateApprovalRequest(ctx, approvalReq)

        return nil, fmt.Errorf("approval timeout after %v", timeout)

    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

### Approval API Endpoints

```go
// pkg/coordinator/api/approvals.go

// POST /v1/approvals/:id/approve
func (a *API) ApproveRequest(w http.ResponseWriter, r *http.Request) {
    approvalID := chi.URLParam(r, "id")

    var req ApproveRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    // Get approval request
    approval, err := a.store.GetApprovalRequest(r.Context(), approvalID)
    if err != nil {
        writeError(w, http.StatusNotFound, "approval request not found")
        return
    }

    // Check if already completed
    if approval.Status != ApprovalStatusPending {
        writeError(w, http.StatusConflict, fmt.Sprintf("approval already %s", approval.Status))
        return
    }

    // Check if expired
    if time.Now().After(approval.ExpiresAt) {
        writeError(w, http.StatusGone, "approval request expired")
        return
    }

    // Validate approver
    userID := r.Context().Value("user_id").(string)
    if !contains(approval.Approvers, userID) {
        writeError(w, http.StatusForbidden, "you are not authorized to approve this request")
        return
    }

    // Publish approval response to NATS
    result := &ApprovalResult{
        ApprovalID: approvalID,
        Status:     req.Decision, // "approved" or "rejected"
        ApprovedBy: userID,
        Comment:    req.Comment,
    }

    responseSubject := fmt.Sprintf("approvals.response.%s", approvalID)
    resultData, err := json.Marshal(result)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to marshal response")
        return
    }

    if err := a.natsClient.Publish(r.Context(), responseSubject, resultData); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to publish approval response")
        return
    }

    writeJSON(w, http.StatusOK, ApproveResponse{
        ApprovalID: approvalID,
        Status:     req.Decision,
        Message:    "approval response recorded",
    })
}

// GET /v1/approvals - List pending approvals
func (a *API) ListPendingApprovals(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("user_id").(string)

    approvals, err := a.store.GetPendingApprovals(r.Context(), userID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to get approvals")
        return
    }

    writeJSON(w, http.StatusOK, ListApprovalsResponse{
        Approvals: approvals,
        Count:     len(approvals),
    })
}
```

### Schema

```sql
CREATE TABLE approval_requests (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    approvers TEXT NOT NULL,  -- JSON array of user IDs
    status TEXT NOT NULL,     -- pending, approved, rejected
    approved_by TEXT,
    comment TEXT,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES task_executions(task_id),
    FOREIGN KEY (instance_id) REFERENCES workflow_instances(id)
);

CREATE INDEX idx_approvals_status ON approval_requests(status);
CREATE INDEX idx_approvals_expires ON approval_requests(expires_at);
```

---

## Issue #124: NATS Event Handlers

**Purpose:** Async communication between coordinator and agents via NATS pub/sub

### Event Schemas

```go
// pkg/coordinator/events/schemas.go
package events

type AgentTaskCompleted struct {
    TaskID        string    `json:"task_id"`
    InstanceID    string    `json:"instance_id"`
    CorrelationID string    `json:"correlation_id"`
    Result        any       `json:"result"`
    Timestamp     time.Time `json:"timestamp"`
}

type AgentTaskFailed struct {
    TaskID        string    `json:"task_id"`
    InstanceID    string    `json:"instance_id"`
    CorrelationID string    `json:"correlation_id"`
    Error         string    `json:"error"`
    Timestamp     time.Time `json:"timestamp"`
}

type WorkflowStatusRequested struct {
    InstanceID    string `json:"instance_id"`
    CorrelationID string `json:"correlation_id"`
}
```

### Event Handler Implementation

```go
// pkg/coordinator/events/handlers.go
package events

type EventHandler struct {
    executor *engine.Executor
    store    *storage.SQLiteStore
    nc       *nats.Conn
}

func (h *EventHandler) Subscribe(ctx context.Context) error {
    // Subscribe to agent task events
    if _, err := h.nc.Subscribe("agents.*.completed", h.handleTaskCompleted); err != nil {
        return err
    }

    if _, err := h.nc.Subscribe("agents.*.failed", h.handleTaskFailed); err != nil {
        return err
    }

    if _, err := h.nc.Subscribe("workflow.status.requested", h.handleStatusRequest); err != nil {
        return err
    }

    return nil
}

func (h *EventHandler) handleTaskCompleted(msg *nats.Msg) {
    var event AgentTaskCompleted
    if err := json.Unmarshal(msg.Data, &event); err != nil {
        log.Error("failed to unmarshal event", "error", err)
        return
    }

    // Update workflow state
    ctx := context.Background()
    if err := h.store.UpdateTaskStatus(ctx, event.TaskID, engine.TaskStateCompleted); err != nil {
        log.Error("failed to update task status", "error", err, "correlation_id", event.CorrelationID)
        return
    }

    // Record event
    h.store.RecordEvent(ctx, &storage.WorkflowEvent{
        InstanceID:    event.InstanceID,
        EventType:     "task.completed",
        CorrelationID: event.CorrelationID,
        Data:          event.Result,
    })

    log.Info("task completed", "task_id", event.TaskID, "correlation_id", event.CorrelationID)
}

func (h *EventHandler) handleTaskFailed(msg *nats.Msg) {
    var event AgentTaskFailed
    if err := json.Unmarshal(msg.Data, &event); err != nil {
        log.Error("failed to unmarshal event", "error", err)
        return
    }

    ctx := context.Background()
    if err := h.store.UpdateTaskStatus(ctx, event.TaskID, engine.TaskStateFailed); err != nil {
        log.Error("failed to update task status", "error", err, "correlation_id", event.CorrelationID)
        return
    }

    h.store.RecordEvent(ctx, &storage.WorkflowEvent{
        InstanceID:    event.InstanceID,
        EventType:     "task.failed",
        CorrelationID: event.CorrelationID,
        Data:          event.Error,
    })

    log.Error("task failed", "task_id", event.TaskID, "error", event.Error, "correlation_id", event.CorrelationID)
}
```

### NATS Subject Design

```
agents.<agent_type>              → Agent subscribes for tasks
agents.<task_id>.result          → Coordinator subscribes for results
agents.<task_id>.completed       → Broadcast task completion
agents.<task_id>.failed          → Broadcast task failure
workflow.status.requested        → Request workflow status
workflow.status.response.<cid>   → Status response (correlation ID)
approvals.requested              → Approval request broadcast
approvals.response.<approval_id> → Approval response
```

---

## Issue #123: HTTP API - Workflow Management

**Purpose:** RESTful API for creating workflows, starting instances, checking status, and cancelling workflows

### API Endpoints

```go
// pkg/coordinator/api/handlers.go
package api

type API struct {
    coordinator *coordinator.Coordinator
    store       *storage.SQLiteStore
    executor    *engine.Executor
}

// POST /v1/workflows - Create workflow definition
func (a *API) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
    var req CreateWorkflowRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    // Parse and validate workflow
    def, err := parser.Parse([]byte(req.YAMLDefinition))
    if err != nil {
        writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid workflow: %v", err))
        return
    }

    // Save to SQLite
    if err := a.store.SaveWorkflowDefinition(r.Context(), def); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to save workflow")
        return
    }

    writeJSON(w, http.StatusCreated, CreateWorkflowResponse{ID: def.ID})
}

// POST /v1/workflows/:id/instances - Start workflow instance
func (a *API) StartWorkflow(w http.ResponseWriter, r *http.Request) {
    workflowID := chi.URLParam(r, "id")

    // Get workflow definition
    def, err := a.store.GetWorkflowDefinition(r.Context(), workflowID)
    if err != nil {
        writeError(w, http.StatusNotFound, "workflow not found")
        return
    }

    // Replay protection: check for existing active instance
    existingInstance, err := a.store.GetActiveInstance(r.Context(), workflowID)
    if err == nil && existingInstance != nil {
        writeJSON(w, http.StatusConflict, StartWorkflowResponse{
            InstanceID: existingInstance.ID,
            Status:     "already_running",
        })
        return
    }

    // Start execution (async)
    instance, _ := a.store.CreateInstance(r.Context(), workflowID)
    go func() {
        result, err := a.executor.Execute(context.Background(), def)
        if err != nil {
            log.Error("workflow execution failed", "workflow_id", workflowID, "error", err)
        } else {
            log.Info("workflow execution completed", "workflow_id", workflowID, "result", result)
        }
    }()

    writeJSON(w, http.StatusAccepted, StartWorkflowResponse{
        InstanceID: instance.ID,
        Status:     "started",
    })
}

// GET /v1/workflows/:id/instances/:instance_id - Get instance status
func (a *API) GetWorkflowStatus(w http.ResponseWriter, r *http.Request) {
    instanceID := chi.URLParam(r, "instance_id")

    instance, err := a.store.GetWorkflowInstance(r.Context(), instanceID)
    if err != nil {
        writeError(w, http.StatusNotFound, "instance not found")
        return
    }

    tasks, err := a.store.GetTaskExecutions(r.Context(), instanceID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to get tasks")
        return
    }

    writeJSON(w, http.StatusOK, WorkflowStatusResponse{
        InstanceID:  instance.ID,
        WorkflowID:  instance.DefinitionID,
        Status:      instance.Status,
        StartedAt:   instance.StartedAt,
        CompletedAt: instance.CompletedAt,
        Tasks:       tasks,
    })
}

// DELETE /v1/workflows/:id/instances/:instance_id - Cancel workflow
func (a *API) CancelWorkflow(w http.ResponseWriter, r *http.Request) {
    instanceID := chi.URLParam(r, "instance_id")

    if err := a.store.UpdateInstanceStatus(r.Context(), instanceID, engine.StatusCancelled); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to cancel workflow")
        return
    }

    a.executor.CancelWorkflow(r.Context(), instanceID)

    writeJSON(w, http.StatusOK, CancelWorkflowResponse{Status: "cancelled"})
}
```

### Middleware

```go
// API key authentication
func (a *API) AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := r.Header.Get("X-API-Key")
        if apiKey == "" || !a.validateAPIKey(apiKey) {
            writeError(w, http.StatusUnauthorized, "invalid API key")
            return
        }
        next.ServeHTTP(w, r)
    })
}

// Rate limiting
func (a *API) RateLimitMiddleware(next http.Handler) http.Handler {
    // Token bucket: 100 req/min per API key
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := r.Header.Get("X-API-Key")
        if !a.rateLimiter.Allow(apiKey) {
            w.Header().Set("Retry-After", "60")
            writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

---

## Issue #128: Task Lifecycle & Monitoring

**Purpose:** Track task state transitions and collect metrics for observability

### Task State Machine

```go
// pkg/coordinator/engine/task.go
package engine

type TaskState string

const (
    TaskStatePending   TaskState = "PENDING"
    TaskStateAssigned  TaskState = "ASSIGNED"
    TaskStateRunning   TaskState = "RUNNING"
    TaskStateCompleted TaskState = "COMPLETED"
    TaskStateFailed    TaskState = "FAILED"
)

type TaskExecution struct {
    ID          string
    InstanceID  string
    TaskID      string
    AgentID     string
    State       TaskState
    Result      any
    Error       string
    Attempts    int
    StartedAt   time.Time
    CompletedAt time.Time
    Duration    time.Duration
}
```

### Monitoring Metrics

```go
type Metrics struct {
    TaskDuration    prometheus.Histogram
    TaskFailures    prometheus.Counter
    TasksInProgress prometheus.Gauge
}

func NewMetrics() *Metrics {
    return &Metrics{
        TaskDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name:    "ourocodus_task_duration_seconds",
            Help:    "Task execution duration",
            Buckets: prometheus.DefBuckets,
        }),
        TaskFailures: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "ourocodus_task_failures_total",
            Help: "Total number of task failures",
        }),
        TasksInProgress: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "ourocodus_tasks_in_progress",
            Help: "Number of tasks currently running",
        }),
    }
}
```

---

## Issue #129: Workflow Operations

**Purpose:** Cancellation, replay protection, and audit logging

### Implementation

```go
// pkg/coordinator/ops/operations.go
package ops

type Operations struct {
    store    *storage.SQLiteStore
    executor *engine.Executor
    nc       *nats.Conn
}

// Cancellation
func (o *Operations) CancelWorkflow(ctx context.Context, instanceID string) error {
    instance, err := o.store.GetWorkflowInstance(ctx, instanceID)
    if err != nil {
        return err
    }

    // Check if already terminal
    if instance.Status == engine.StatusCompleted ||
       instance.Status == engine.StatusFailed ||
       instance.Status == engine.StatusCancelled {
        return fmt.Errorf("workflow already in terminal state: %s", instance.Status)
    }

    // Mark as cancelled
    if err := o.store.UpdateInstanceStatus(ctx, instanceID, engine.StatusCancelled); err != nil {
        return err
    }

    // Publish cancellation event
    cancelEvent := &events.WorkflowCancelled{
        InstanceID:    instanceID,
        CorrelationID: generateCorrelationID(),
        Timestamp:     time.Now(),
    }

    if err := o.nc.Publish("workflow.cancelled", cancelEvent); err != nil {
        return err
    }

    return nil
}

// Replay Protection
func (o *Operations) StartWorkflowWithReplayProtection(ctx context.Context, defID string, forceRestart bool) (string, error) {
    // Check for existing active instance
    existingInstance, err := o.store.GetActiveInstance(ctx, defID)
    if err != nil && err != storage.ErrNotFound {
        return "", err
    }

    if existingInstance != nil && !forceRestart {
        return existingInstance.ID, nil
    }

    if existingInstance != nil && forceRestart {
        if err := o.CancelWorkflow(ctx, existingInstance.ID); err != nil {
            return "", fmt.Errorf("failed to cancel existing instance: %w", err)
        }
    }

    // Create new instance
    instance, err := o.store.CreateInstance(ctx, defID)
    if err != nil {
        return "", err
    }

    // Start execution
    def, err := o.store.GetWorkflowDefinition(ctx, defID)
    if err != nil {
        return "", err
    }

    go o.executor.Execute(context.Background(), def)

    return instance.ID, nil
}

// Audit Trail
func (o *Operations) GetAuditLog(ctx context.Context, instanceID string, filters *AuditFilters) ([]*storage.WorkflowEvent, error) {
    events, err := o.store.GetEvents(ctx, instanceID)
    if err != nil {
        return nil, err
    }

    if filters != nil {
        events = o.applyFilters(events, filters)
    }

    return events, nil
}
```

---

## Issue #125: Basic Workflow Observability

**Purpose:** Debug endpoints, red-flag checks, and structured logging

### Debug Endpoints

```go
// pkg/coordinator/api/debug.go
package api

// GET /v1/debug/workflows/:id/status
func (a *API) DebugWorkflowStatus(w http.ResponseWriter, r *http.Request) {
    instanceID := chi.URLParam(r, "id")

    instance, err := a.store.GetWorkflowInstance(r.Context(), instanceID)
    if err != nil {
        writeError(w, http.StatusNotFound, "instance not found")
        return
    }

    tasks, err := a.store.GetTaskExecutions(r.Context(), instanceID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to get tasks")
        return
    }

    execCtx, _ := a.store.LoadInstanceContext(r.Context(), instanceID)

    writeJSON(w, http.StatusOK, DebugStatusResponse{
        Instance:         instance,
        Tasks:            tasks,
        ExecutionContext: execCtx,
        SystemTime:       time.Now(),
    })
}

// GET /v1/debug/workflows/:id/events
func (a *API) DebugWorkflowEvents(w http.ResponseWriter, r *http.Request) {
    instanceID := chi.URLParam(r, "id")

    events, err := a.store.GetEvents(r.Context(), instanceID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to get events")
        return
    }

    writeJSON(w, http.StatusOK, DebugEventsResponse{
        InstanceID: instanceID,
        Events:     events,
        Count:      len(events),
    })
}
```

### Red-Flag Checks

```go
func (a *API) checkRedFlags(task *storage.TaskExecution) []string {
    var flags []string

    // Too long check (MDAP Section 3.3)
    if task.Result != nil {
        resultStr := fmt.Sprintf("%v", task.Result)
        if len(resultStr) > 750 {
            flags = append(flags, "too_long")
        }
    }

    // Timeout check
    if task.Duration > 5*time.Minute {
        flags = append(flags, "slow_execution")
    }

    // High retry count
    if task.Attempts > 3 {
        flags = append(flags, "frequent_retries")
    }

    return flags
}
```

### Structured Logging

```go
// All workflow operations include correlation IDs
log.Info("workflow started",
    "workflow_id", workflowID,
    "instance_id", instanceID,
    "correlation_id", correlationID)
```

---

## Week-by-Week Implementation Roadmap

### Week 1: Foundation (Issues #121, #122)
- [ ] Create `pkg/coordinator` package structure
- [ ] Implement coordinator service lifecycle
- [ ] Setup SQLite schema and repository pattern
- [ ] Integration: Hook into existing relay architecture
- **Deliverable:** Coordinator service starts, SQLite initialized

### Week 2: Parser + Executor (Issues #126, #127)
- [ ] Implement workflow definition parser (YAML/JSON)
- [ ] Create internal DAG representation
- [ ] Build sequential execution engine
- [ ] Add explicit context serialization
- [ ] Implement crash recovery (`RestoreAndResume`)
- **Deliverable:** Can execute simple sequential workflow from YAML

### Week 3: NATS + API + Retry (Issues #124, #123, #128, #59)
- [ ] Implement NATS event handlers
- [ ] Setup HTTP API with authentication and rate limiting
- [ ] Add task lifecycle tracking
- [ ] Implement timeout management
- [ ] Add retry logic with exponential backoff
- **Deliverable:** End-to-end workflow via API + NATS with retry

### Week 4: Operations + Observability + Approval (Issues #129, #125, #53)
- [ ] Implement workflow cancellation
- [ ] Add replay protection
- [ ] Setup audit logging
- [ ] Create debug endpoints
- [ ] Add Prometheus metrics
- [ ] Implement approval gate agent
- [ ] Add approval API endpoints
- **Deliverable:** Production-ready M5 with observability and human steering

---

# Part 4: Reference

## Complete Workflow Examples

### Example 1: Simple Data Processing Pipeline

```yaml
apiVersion: ourocodus.dev/v1
kind: Workflow
metadata:
  name: data-ingestion-pipeline
  description: Fetch, validate, transform, and store data
  version: "1.0"

spec:
  tasks:
    - id: fetch-raw-data
      type: http-fetch-agent
      params:
        url: "https://api.example.com/data"
        method: GET
        headers:
          Authorization: "Bearer ${API_TOKEN}"
      timeout: 30s

    - id: validate-schema
      type: json-validator-agent
      dependencies:
        - fetch-raw-data
      params:
        schema: |
          {
            "type": "object",
            "required": ["id", "data"],
            "properties": {
              "id": {"type": "string"},
              "data": {"type": "array"}
            }
          }
      timeout: 10s

    - id: transform-data
      type: jq-transform-agent
      dependencies:
        - validate-schema
      params:
        expression: '.data[] | {id: .id, value: .value, timestamp: now}'
      timeout: 30s

    - id: store-data
      type: postgres-writer-agent
      dependencies:
        - transform-data
      params:
        connection: "postgresql://localhost/mydb"
        table: "ingested_data"
        conflict_strategy: "upsert"
      timeout: 60s

    - id: notify-completion
      type: webhook-agent
      dependencies:
        - store-data
      params:
        url: "https://hooks.slack.com/services/..."
        method: POST
        body: |
          {
            "text": "Data ingestion completed successfully"
          }
      timeout: 10s
```

### Example 2: Production Deployment with Approval

```yaml
apiVersion: ourocodus.dev/v1
kind: Workflow
metadata:
  name: production-deployment
  description: Build, test, approve, and deploy to production
  version: "1.0"

spec:
  tasks:
    - id: build-artifact
      type: build-agent
      params:
        source: "main"
        target: "dist/"
      timeout: 300s

    - id: run-unit-tests
      type: test-agent
      dependencies:
        - build-artifact
      params:
        test_suite: "unit"
      timeout: 120s

    - id: run-integration-tests
      type: test-agent
      dependencies:
        - build-artifact
      params:
        test_suite: "integration"
      timeout: 300s

    - id: approve-deployment
      type: approval-gate
      dependencies:
        - run-unit-tests
        - run-integration-tests
      params:
        title: "Deploy to production?"
        description: "All tests passed. Ready to deploy?"
        timeout: 300s
        approvers:
          - "user:alice@example.com"
          - "user:bob@example.com"

    - id: deploy-production
      type: deploy-agent
      dependencies:
        - approve-deployment
      params:
        environment: "production"
        region: "us-east-1"
      timeout: 600s

    - id: notify-team
      type: webhook-agent
      dependencies:
        - deploy-production
      params:
        url: "https://hooks.slack.com/services/..."
        method: POST
        body: |
          {
            "text": "Production deployment completed successfully!"
          }
      timeout: 10s
```

---

## Testing Strategy

### Unit Tests
- Parser validation (Issue #126)
- State machine transitions (Issue #127)
- Red-flag checks (Issue #125)
- Replay protection logic (Issue #129)

### Integration Tests
- End-to-end workflow execution (Issues #127, #124)
- Crash recovery (Issue #127)
- NATS event flow (Issue #124)
- API authentication and rate limiting (Issue #123)

### Load Tests
- Concurrent workflow execution
- NATS throughput
- SQLite query performance
- API rate limiting behavior

---

## Idempotency Patterns

### System-Wide Idempotency

| Layer | Mechanism | Guarantees |
|-------|-----------|------------|
| NATS Client | Correlation IDs | Same request ID = idempotent retry |
| Task Execution | SQLite status check | Completed tasks never re-execute |
| Workflow Execution | Context serialization | Crash recovery from checkpoint |
| Approval Gates | Status check | Can't approve twice |
| API Operations | Replay protection | Starting same workflow returns existing |
| Agent Handlers | Query coordinator | Agents check status before work |

### Implementation Examples

**Task-level idempotency:**
```go
taskExec, err := e.store.GetTaskExecution(ctx, execCtx.InstanceID, task.ID)
if taskExec != nil && taskExec.Status == TaskStateCompleted {
    return taskExec.Result, nil  // Skip re-execution
}
```

**Workflow-level idempotency:**
```go
if err := e.serializeContext(ctx, execCtx); err != nil {
    return nil, fmt.Errorf("failed to serialize context: %w", err)
}
```

**Approval idempotency:**
```go
if approval.Status != ApprovalStatusPending {
    writeError(w, http.StatusConflict, fmt.Sprintf("approval already %s", approval.Status))
    return
}
```

---

## MDAP Features Deferred to Future Milestones

### Not in Milestone 5

1. **Parallel Task Execution**
   - Sequential execution only (MVP constraint)
   - DAG structure supports future parallelism
   - Defer to M6 or M7

2. **First-to-ahead-by-k Voting**
   - Single agent per task (MVP)
   - Agent registry supports future voting
   - Defer to M7 or M8

3. **L2/L3 Verification**
   - Only L0 (syntax) and L1 (semantic) checks
   - Cross-step verification requires parallel execution
   - Defer to M7

4. **Error Budget Management**
   - Basic metrics only (Issue #125)
   - SLO tracking and circuit breakers deferred
   - Defer to M6

5. **Advanced Red-flagging**
   - Basic checks only (too_long, slow_execution)
   - Confidence scoring and LLM-based flagging deferred
   - Defer to M8

---

## Success Metrics

### Functional Metrics
- ✅ Workflows execute sequentially with correct dependency order
- ✅ Coordinator restarts resume in-progress workflows
- ✅ Completed tasks are not re-executed (idempotency)
- ✅ Workflow cancellation stops cleanly
- ✅ Replay protection prevents duplicate starts

### Performance Metrics
- SQLite queries < 100ms (Issue #122)
- Task dispatch latency < 50ms (Issue #127)
- API response time < 200ms (Issue #123)
- NATS message throughput > 1000 msg/sec (Issue #124)

### Reliability Metrics
- Crash recovery success rate: 100% (Issue #127)
- Task timeout accuracy: ±1 second (Issue #128)
- Event correlation success rate: 100% (Issue #124)

---

## Document Metadata

**Version:** 1.0
**Created:** 2025-11-19
**Last Updated:** 2025-11-19
**Status:** Complete
**Source Documents:**
- `2025-11-19-mdap-milestone5-mapping.md` (1,581 lines)
- `2025-11-19-retry-and-approval-design.md` (1,053 lines)
- `2025-11-19-mdap-principles-audit.md` (918 lines)
- `2025-11-19-mdap-for-coding.md` (1,228 lines)

**Consolidation:**
- Total source lines: 4,780
- Target guide lines: ~3,500
- Reduction: ~27% through de-duplication

**MDAP Paper:** "Solving a Million-Step LLM Task with Zero Errors" (Anthropic, 2024)
**Target Milestone:** Milestone 5: Autonomous Coordination (GitHub #6)
**Dependencies:** Milestone 4: NATS Integration (GitHub #5)
**Blocks:** Milestone 6: Production Polish (GitHub #8)
