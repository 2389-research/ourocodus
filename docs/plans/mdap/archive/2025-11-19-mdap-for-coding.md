# MDAP for Coding: Rubric-Based Verification

## The Core Insight

**MDAPs succeed not because steps are small, but because each step is *verifiable*.**

For coding: **Verification = rubrics, tests, constraints, specs**

This is our ground truth.

---

## Philosophical Foundation

### Tower of Hanoi (Original MDAP)
- **Truth:** External, objective, deterministic
- **Verification:** "Is disk A on peg Y?" (boolean check)
- **Correctness:** Single right answer

### Coding (Ourocodus MDAP)
- **Truth:** Constructed through rubrics and constraints
- **Verification:** "Does this satisfy the rubric?" (multi-dimensional check)
- **Correctness:** Acceptance criteria + quality thresholds

**Key realization:** We're not cheating by constructing truth. We're **engineering correctness** the same way compilers, linters, and CI systems do.

---

## The Verification Hierarchy for Coding

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

---

## Rubric System Design

### What is a Rubric?

A rubric is a **multi-dimensional scoring system** that defines "acceptable" for a given task.

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

---

## Massive Decomposition for Coding

### Example: "Add HTTP endpoint for workflow status"

**Traditional approach (single agent):**
```yaml
tasks:
  - id: add-status-endpoint
    type: code-generator
    prompt: "Add GET /v1/workflows/:id/status endpoint"
```

**MDAP approach (20+ micro-agents):**
```yaml
tasks:
  # 1. Understanding phase
  - id: parse-requirement
    type: requirement-parser
    input:
      requirement: "Add GET /v1/workflows/:id/status endpoint"
    output_schema: ParsedRequirement

  - id: extract-entities
    type: entity-extractor
    dependencies: [parse-requirement]
    output_schema: Entities

  - id: identify-dependencies
    type: dependency-analyzer
    dependencies: [extract-entities]
    output_schema: Dependencies

  # 2. Interface design phase
  - id: design-request-schema
    type: schema-designer
    dependencies: [extract-entities]
    output_schema: RequestSchema

  - id: design-response-schema
    type: schema-designer
    dependencies: [extract-entities]
    output_schema: ResponseSchema

  - id: validate-schemas
    type: schema-validator
    dependencies: [design-request-schema, design-response-schema]
    verifiers:
      - type: json-schema-validator
        fail_on_error: true

  # 3. Code generation phase (skeleton)
  - id: generate-handler-skeleton
    type: code-skeleton-generator
    dependencies: [validate-schemas]
    output_schema: GoCode
    verifiers:
      - type: syntax-validator
        params: {language: go}

  - id: generate-validation-logic
    type: validation-generator
    dependencies: [generate-handler-skeleton, design-request-schema]
    output_schema: GoCode

  - id: generate-business-logic
    type: logic-generator
    dependencies: [generate-validation-logic, identify-dependencies]
    output_schema: GoCode

  - id: generate-error-handling
    type: error-handler-generator
    dependencies: [generate-business-logic]
    output_schema: GoCode

  - id: generate-response-marshaling
    type: response-generator
    dependencies: [generate-error-handling, design-response-schema]
    output_schema: GoCode

  # 4. Assembly phase
  - id: assemble-handler
    type: code-assembler
    dependencies:
      - generate-handler-skeleton
      - generate-validation-logic
      - generate-business-logic
      - generate-error-handling
      - generate-response-marshaling
    output_schema: CompleteGoCode

  # 5. Test generation phase
  - id: generate-unit-tests
    type: test-generator
    dependencies: [assemble-handler]
    params:
      test_type: unit
    output_schema: GoTestCode

  - id: generate-integration-tests
    type: test-generator
    dependencies: [assemble-handler]
    params:
      test_type: integration
    output_schema: GoTestCode

  # 6. Documentation phase
  - id: generate-godoc
    type: doc-generator
    dependencies: [assemble-handler]
    params:
      format: godoc
    output_schema: Documentation

  - id: generate-openapi
    type: openapi-generator
    dependencies: [design-request-schema, design-response-schema]
    output_schema: OpenAPISpec

  # 7. Verification phase (L0)
  - id: verify-syntax
    type: syntax-validator
    dependencies: [assemble-handler]
    verifiers:
      - type: go-compiler
        fail_on_error: true

  # 8. Verification phase (L1)
  - id: run-unit-tests
    type: test-runner
    dependencies: [generate-unit-tests, verify-syntax]
    verifiers:
      - type: test-pass-checker
        fail_on_error: true

  - id: run-linter
    type: linter
    dependencies: [assemble-handler]
    verifiers:
      - type: golangci-lint
        fail_on_error: true

  - id: check-test-coverage
    type: coverage-checker
    dependencies: [run-unit-tests]
    params:
      threshold: 80%
    verifiers:
      - type: coverage-threshold
        fail_on_error: true

  # 9. Verification phase (L2)
  - id: run-integration-tests
    type: test-runner
    dependencies: [generate-integration-tests, verify-syntax]
    verifiers:
      - type: test-pass-checker
        fail_on_error: true

  - id: check-api-compatibility
    type: api-compatibility-checker
    dependencies: [generate-openapi]
    verifiers:
      - type: breaking-change-detector
        fail_on_error: false  # Warn only

  # 10. Verification phase (L3)
  - id: ai-code-review
    type: code-reviewer
    dependencies: [assemble-handler, run-unit-tests, run-linter]
    params:
      focus: [maintainability, security, performance]

  - id: human-approval
    type: approval-gate
    dependencies: [ai-code-review]
    params:
      title: "Review generated endpoint"
      timeout: 600s
```

**Result:** 20+ micro-agents, each with a tiny, verifiable job.

---

## Repair Loops with Rubrics

### Problem: Traditional Retry is Dumb

```yaml
# Traditional approach: just retry the same agent
- id: generate-code
  type: code-generator
  retry: 3  # ❌ Same agent, same mistakes
```

### Solution: Repair Loop with Different Strategies

```yaml
- id: generate-code
  type: code-generator
  output_schema: GoCode
  verifiers:
    - type: syntax-validator
      fail_on_error: false
    - type: test-runner
      fail_on_error: false
    - type: linter
      fail_on_error: false

  # Repair policy: different strategy for each failure type
  on_failure:
    - condition: "syntax_error"
      repair:
        - agent: syntax-repair-agent
          max_attempts: 2

    - condition: "tests_fail"
      repair:
        - agent: test-fixing-agent
          max_attempts: 3

    - condition: "linting_issues"
      repair:
        - agent: linting-fix-agent
          max_attempts: 2

    - condition: "rubric_score < 0.75"
      repair:
        - agent: quality-improvement-agent
          max_attempts: 2
        - escalate: human-review
```

### Repair Agent Example

```go
type SyntaxRepairAgent struct {
    llm LLMClient
}

func (a *SyntaxRepairAgent) Execute(ctx context.Context, input *RepairInput) (*RepairOutput, error) {
    // Input: broken code + syntax error
    brokenCode := input.Code
    syntaxError := input.Error

    // Prompt is TINY and FOCUSED
    prompt := fmt.Sprintf(`Fix this Go syntax error:

Code:
%s

Error:
%s

Return ONLY the corrected code, no explanation.`, brokenCode, syntaxError)

    // Call LLM with short context
    fixedCode, err := a.llm.Complete(ctx, prompt)
    if err != nil {
        return nil, err
    }

    return &RepairOutput{
        Code: fixedCode,
    }, nil
}
```

**Key:** Repair agents are specialized, focused, and have tiny context.

---

## Human Steering Integration

### Where Humans Intervene

**1. Pre-Workflow (Planning Phase)**
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

**2. Mid-Workflow (Steering Points)**
```yaml
- id: generate-code
  type: code-generator
  steering_point: true  # Human can review intermediate result

- id: human-steering-check
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

**3. Post-Verification (Override Failures)**
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

**4. Post-Workflow (Learning Phase)**
```yaml
- id: human-feedback
  type: feedback-collector
  dependencies: [all-tasks-complete]
  params:
    questions:
      - "Was the generated code acceptable? (1-5)"
      - "Which task produced the worst output?"
      - "What could be improved?"
```

### Interactive Steering API

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

**Use voting when:**
- Task output is subjective (naming, code style)
- Multiple valid solutions exist (algorithm choice)
- Risk is high (security-critical code)
- Correctness is ambiguous (UX decisions)

**Don't use voting when:**
- Task is deterministic (parsing JSON, running tests)
- Output is verifiable objectively (syntax checking)
- Cost is prohibitive (expensive LLM calls)

### Voting Strategy

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

### Implementation

```go
func (e *Executor) executeWithVoting(ctx context.Context, task *Task) error {
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

    task.Output = winner
    return nil
}

func (e *Executor) vote(results []*TaskResult, config *VotingConfig) (*TaskResult, bool) {
    switch config.Method {
    case "majority":
        // Simple majority: find most common result
        counts := make(map[string]int)
        for _, result := range results {
            hash := hashResult(result)
            counts[hash]++
        }

        maxCount := 0
        var winner *TaskResult
        for _, result := range results {
            hash := hashResult(result)
            if counts[hash] > maxCount {
                maxCount = counts[hash]
                winner = result
            }
        }

        // Check consensus threshold
        consensus := maxCount >= parseConsensus(config.RequireConsensus, len(results))
        return winner, consensus

    case "weighted":
        // Weighted vote: score each result by rubric
        // ... (implementation details)

    case "unanimous":
        // All results must be identical
        // ... (implementation details)
    }

    return nil, false
}
```

---

## Practical Example: Ourocodus Workflow for Issue #127

**Task:** Implement sequential workflow execution engine

### Traditional Approach (1 agent)
```yaml
tasks:
  - id: implement-executor
    type: code-generator
    prompt: "Implement pkg/coordinator/engine/executor.go with sequential execution"
```

### MDAP Approach (40+ micro-agents)

```yaml
apiVersion: ourocodus.dev/v1
kind: Workflow
metadata:
  name: implement-executor-m5-127

spec:
  rubric:
    name: "Go Executor Quality"
    dimensions:
      - name: correctness
        weight: 40%
        criteria:
          - "Compiles without errors"
          - "All tests pass"
          - "Handles edge cases"
      - name: code_quality
        weight: 30%
        criteria:
          - "golangci-lint passes"
          - "Test coverage > 80%"
          - "Functions < 50 lines"
      - name: reliability
        weight: 20%
        criteria:
          - "Crash recovery works"
          - "Idempotency guaranteed"
          - "Context serialization correct"
      - name: maintainability
        weight: 10%
        criteria:
          - "Clear variable names"
          - "Documented public APIs"
          - "No code duplication"
    passing_threshold: 0.75

  tasks:
    # Phase 1: Understanding (5 tasks)
    - id: parse-issue
      type: github-issue-parser
      input:
        issue_number: 127
      output_schema: ParsedIssue

    - id: extract-requirements
      type: requirement-extractor
      dependencies: [parse-issue]
      output_schema: Requirements

    - id: identify-interfaces
      type: interface-identifier
      dependencies: [extract-requirements]
      output_schema: Interfaces

    - id: map-dependencies
      type: dependency-mapper
      dependencies: [extract-requirements]
      input:
        codebase_path: "pkg/coordinator"
      output_schema: Dependencies

    - id: generate-test-scenarios
      type: scenario-generator
      dependencies: [extract-requirements]
      output_schema: TestScenarios

    # Phase 2: Interface Design (3 tasks)
    - id: design-executor-interface
      type: interface-designer
      dependencies: [identify-interfaces]
      output_schema: GoInterface
      verifiers:
        - type: syntax-validator
          params: {language: go}

    - id: design-context-struct
      type: struct-designer
      dependencies: [extract-requirements]
      output_schema: GoStruct

    - id: design-config-struct
      type: struct-designer
      dependencies: [extract-requirements]
      output_schema: GoStruct

    # Phase 3: Implementation (10 tasks)
    - id: generate-executor-skeleton
      type: skeleton-generator
      dependencies: [design-executor-interface]
      output_schema: GoCode

    - id: implement-execute-method
      type: method-implementer
      dependencies: [generate-executor-skeleton, design-context-struct]
      params:
        method_name: "Execute"
      output_schema: GoCode

    - id: implement-execute-task-method
      type: method-implementer
      dependencies: [implement-execute-method]
      params:
        method_name: "executeTask"
      output_schema: GoCode

    - id: implement-serialize-context
      type: method-implementer
      dependencies: [implement-execute-task-method, design-context-struct]
      params:
        method_name: "serializeContext"
      output_schema: GoCode

    - id: implement-restore-resume
      type: method-implementer
      dependencies: [implement-serialize-context]
      params:
        method_name: "RestoreAndResume"
      output_schema: GoCode

    - id: implement-idempotency-check
      type: code-snippet-generator
      dependencies: [implement-execute-task-method]
      params:
        snippet_type: "idempotency_check"
      output_schema: GoCode

    - id: implement-retry-logic
      type: code-snippet-generator
      dependencies: [implement-execute-task-method]
      params:
        snippet_type: "retry_with_backoff"
      output_schema: GoCode

    - id: implement-timeout-handling
      type: code-snippet-generator
      dependencies: [implement-execute-task-method]
      params:
        snippet_type: "context_timeout"
      output_schema: GoCode

    - id: implement-nats-dispatch
      type: code-snippet-generator
      dependencies: [implement-execute-task-method, map-dependencies]
      params:
        snippet_type: "nats_request_reply"
      output_schema: GoCode

    - id: assemble-executor
      type: code-assembler
      dependencies:
        - generate-executor-skeleton
        - implement-execute-method
        - implement-execute-task-method
        - implement-serialize-context
        - implement-restore-resume
        - implement-idempotency-check
        - implement-retry-logic
        - implement-timeout-handling
        - implement-nats-dispatch
      output_schema: CompleteGoCode

    # Phase 4: Test Generation (5 tasks)
    - id: generate-unit-test-skeleton
      type: test-skeleton-generator
      dependencies: [assemble-executor]
      params:
        test_type: unit
      output_schema: GoTestCode

    - id: generate-execute-tests
      type: test-case-generator
      dependencies: [generate-unit-test-skeleton, generate-test-scenarios]
      params:
        method_name: "Execute"
      output_schema: GoTestCode

    - id: generate-crash-recovery-tests
      type: test-case-generator
      dependencies: [generate-unit-test-skeleton]
      params:
        test_scenario: "crash_recovery"
      output_schema: GoTestCode

    - id: generate-idempotency-tests
      type: test-case-generator
      dependencies: [generate-unit-test-skeleton]
      params:
        test_scenario: "idempotency"
      output_schema: GoTestCode

    - id: assemble-tests
      type: test-assembler
      dependencies:
        - generate-unit-test-skeleton
        - generate-execute-tests
        - generate-crash-recovery-tests
        - generate-idempotency-tests
      output_schema: CompleteGoTestCode

    # Phase 5: Documentation (2 tasks)
    - id: generate-godoc
      type: doc-generator
      dependencies: [assemble-executor]
      params:
        format: godoc
      output_schema: Documentation

    - id: update-readme
      type: readme-updater
      dependencies: [assemble-executor]
      params:
        section: "Executor"
      output_schema: Markdown

    # Phase 6: L0 Verification (3 tasks)
    - id: verify-syntax
      type: syntax-validator
      dependencies: [assemble-executor, assemble-tests]
      verifiers:
        - type: go-build
          command: "go build ./pkg/coordinator/engine"
          fail_on_error: true

    - id: verify-imports
      type: import-validator
      dependencies: [verify-syntax]
      verifiers:
        - type: goimports
          fail_on_error: true

    - id: verify-formatting
      type: formatter
      dependencies: [verify-syntax]
      verifiers:
        - type: gofumpt
          fail_on_error: true

    # Phase 7: L1 Verification (5 tasks)
    - id: run-unit-tests
      type: test-runner
      dependencies: [verify-syntax, assemble-tests]
      params:
        command: "go test ./pkg/coordinator/engine"
      verifiers:
        - type: test-pass-checker
          fail_on_error: true

    - id: check-coverage
      type: coverage-checker
      dependencies: [run-unit-tests]
      params:
        threshold: 80%
      verifiers:
        - type: coverage-threshold
          fail_on_error: true

    - id: run-linter
      type: linter
      dependencies: [verify-syntax]
      params:
        tool: golangci-lint
        config: .golangci.yml
      verifiers:
        - type: linter-pass-checker
          fail_on_error: true

    - id: run-staticcheck
      type: static-analyzer
      dependencies: [verify-syntax]
      params:
        tool: staticcheck
      verifiers:
        - type: staticcheck-pass-checker
          fail_on_error: true

    - id: check-complexity
      type: complexity-checker
      dependencies: [verify-syntax]
      params:
        max_complexity: 10
      verifiers:
        - type: complexity-threshold
          fail_on_error: false  # Warn only

    # Phase 8: L2 Verification (3 tasks)
    - id: run-integration-tests
      type: test-runner
      dependencies: [run-unit-tests]
      params:
        command: "go test ./tests/integration/executor_test.go"
      verifiers:
        - type: test-pass-checker
          fail_on_error: true

    - id: check-interface-compatibility
      type: interface-checker
      dependencies: [assemble-executor]
      params:
        verify_against: "pkg/coordinator/interfaces.go"
      verifiers:
        - type: interface-match-checker
          fail_on_error: true

    - id: detect-regressions
      type: regression-detector
      dependencies: [run-integration-tests]
      params:
        baseline_branch: main
      verifiers:
        - type: no-breaking-changes
          fail_on_error: false  # Warn only

    # Phase 9: Rubric Evaluation (1 task)
    - id: evaluate-rubric
      type: rubric-evaluator
      dependencies:
        - run-unit-tests
        - check-coverage
        - run-linter
        - run-staticcheck
        - check-complexity
        - run-integration-tests
      params:
        rubric: ${workflow.spec.rubric}
      verifiers:
        - type: rubric-threshold
          params:
            threshold: 0.75
          fail_on_error: false

    # Phase 10: L3 Verification (2 tasks)
    - id: ai-code-review
      type: code-reviewer
      dependencies: [evaluate-rubric]
      params:
        focus:
          - crash_recovery_correctness
          - idempotency_guarantees
          - error_handling
          - maintainability

    - id: human-approval
      type: approval-gate
      dependencies: [ai-code-review]
      params:
        title: "Review Executor Implementation"
        description: |
          Executor implementation complete:
          - Rubric score: ${tasks.evaluate-rubric.output.score}
          - Test coverage: ${tasks.check-coverage.output.coverage}%
          - AI review: ${tasks.ai-code-review.output.summary}
        reviewers: ["user:clint"]
        timeout: 600s
      on_rejection:
        - provide_feedback: true
        - retry_from: implement-execute-method
```

**Result:**
- 40+ micro-agents
- Each agent: tiny, verifiable job
- Multi-layer verification (L0 → L1 → L2 → L3)
- Rubric-based quality scoring
- Human steering at critical points
- Repair loops for common failures

---

## Summary: MDAP for Ourocodus

### Core Principles Applied

1. **Massive Decomposition**
   - 20-40 micro-agents per coding task
   - Each agent: single responsibility, tiny context

2. **Rubric-Based Verification**
   - L0: Syntax (compile, parse, schema)
   - L1: Semantic (tests, lints, constraints)
   - L2: Integration (interfaces, dependencies, regressions)
   - L3: Human judgment (quality, maintainability, strategy)

3. **Repair Loops, Not Retries**
   - Syntax error → syntax-repair-agent
   - Test failure → test-fixing-agent
   - Lint issues → linting-fix-agent
   - Low rubric score → quality-improvement-agent

4. **Human Steering**
   - Pre-workflow: plan review
   - Mid-workflow: intermediate checkpoints
   - Post-verification: override failures
   - Post-workflow: feedback collection

5. **Voting for Ambiguity**
   - Subjective decisions: ensemble vote
   - High-risk operations: multi-sample consensus
   - Disagreement: escalate to human

6. **Instrumentation**
   - Log every agent call (input, output, duration, cost)
   - Track rubric scores over time
   - Identify flaky agents
   - Measure verification failure rates

---

## Implementation Roadmap for Milestone 5

**Week 1: Foundation**
- [ ] Agent registry with schema support
- [ ] Rubric definition format (YAML)
- [ ] Rubric evaluator implementation

**Week 2: Verification Infrastructure**
- [ ] L0 verifiers (syntax, schema, compile)
- [ ] L1 verifiers (tests, lints, coverage)
- [ ] L2 verifiers (integration, interfaces)

**Week 3: Micro-Agent Library**
- [ ] 10 deterministic agents (parsers, validators, assemblers)
- [ ] 5 LLM agents (generators, fixers, reviewers)
- [ ] Agent composition patterns

**Week 4: Human Steering**
- [ ] Steering points in workflow engine
- [ ] Interactive API endpoints
- [ ] Feedback collection system

This is how we build MDAP-level reliability for coding tasks in Ourocodus.
