# MDAP Principles Audit & Redesign

## Core Question

**Are we building a microservice architecture with error-correcting codes, or are we building "a smart brain"?**

Current answer: We're **halfway there**, but missing critical decomposition and verification strategies.

---

## Principle-by-Principle Audit

### ✅ Principle 1: Extreme Decomposition

**MDAP Core Idea:** Break tasks into micro-steps so small they feel trivial.

**Current Design:**
```yaml
tasks:
  - id: fetch-data
    type: http-fetch-agent      # Too coarse?

  - id: validate-data
    type: validation-agent      # What does this actually validate?

  - id: transform-data
    type: transform-agent       # One agent for ALL transforms?
```

**Problem:** Our "agents" are still **monolithic functions**, not micro-tasks.

**MDAP Would Do:**
```yaml
tasks:
  # Fetch phase - decomposed
  - id: prepare-http-request
    type: request-builder
    output_schema: HTTPRequest

  - id: execute-http-call
    type: http-executor
    dependencies: [prepare-http-request]
    output_schema: HTTPResponse

  - id: check-http-status
    type: status-validator
    dependencies: [execute-http-call]
    verifies: ["status_code in [200, 201, 204]"]

  - id: extract-response-body
    type: json-extractor
    dependencies: [check-http-status]
    output_schema: RawJSON

  # Validation phase - decomposed
  - id: validate-json-parseable
    type: json-parser
    dependencies: [extract-response-body]
    output_schema: ParsedJSON

  - id: validate-required-fields
    type: schema-checker
    dependencies: [validate-json-parseable]
    params:
      required_fields: ["id", "data"]

  - id: validate-field-types
    type: type-checker
    dependencies: [validate-required-fields]
    params:
      schema: {id: string, data: array}

  - id: validate-business-rules
    type: constraint-checker
    dependencies: [validate-field-types]
    params:
      rules:
        - "data.length > 0"
        - "id matches pattern [a-z0-9-]+"
```

**Insight:** Each step should do **one checkable thing**. No "smart agents" - just specialized microservices with strict contracts.

**Action Items:**
- [ ] Define atomic task primitives (json-parser, type-checker, constraint-validator, etc.)
- [ ] Create task library with 20-30 micro-agents, each with single responsibility
- [ ] Rewrite example workflows to use micro-composition

---

### ⚠️ Principle 2: Explicit Process (DAG), Not Soup

**MDAP Core Idea:** Orchestrator controls flow explicitly. No "agent figures it out."

**Current Design:** ✅ We have explicit DAG with dependencies

```go
type Task struct {
    ID           string
    Type         string
    Dependencies []string  // Explicit edges
}
```

**Good!** But missing:
- **Conditional branches** (if validation fails, go to repair path)
- **Retry boundaries** (which failures trigger retry vs. abort?)
- **Human gates** (where can humans inject guidance?)

**MDAP Would Add:**
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

**Action Items:**
- [ ] Add `on_success`, `on_failure`, `on_timeout` routing to task schema
- [ ] Support DAG cycles (with max-iteration guards) for repair loops
- [ ] Make human gates explicit decision points in DAG, not just approval/reject

---

### ❌ Principle 3: Structured I/O Everywhere

**MDAP Core Idea:** Agents talk via typed schemas, not freeform text.

**Current Design:** Missing entirely!

```go
type Task struct {
    Params map[string]any  // ❌ Untyped blob
}

type TaskResult struct {
    Data any  // ❌ Untyped blob
}
```

**Problem:** No schema validation between tasks. This is **catastrophic** for reliability.

**MDAP Would Do:**
```go
// Define schemas as first-class objects
type TaskSchema struct {
    InputSchema  *jsonschema.Schema
    OutputSchema *jsonschema.Schema
}

type Task struct {
    ID      string
    Type    string
    Input   json.RawMessage  // Must match agent's InputSchema
    Output  json.RawMessage  // Must match agent's OutputSchema
}

// Agent registry with schemas
type AgentDefinition struct {
    Type         string
    InputSchema  string  // JSON Schema URL or inline
    OutputSchema string
    Handler      func(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

// Before executing task, validate input against schema
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

**Action Items:**
- [ ] Add `input_schema` and `output_schema` to task definitions
- [ ] Validate every task I/O against schemas (L0 verification)
- [ ] Store schemas in agent registry, not in workflow YAML
- [ ] Generate schema docs automatically for each agent type

---

### ❌ Principle 4: Verification Built-In, Not Afterthought

**MDAP Core Idea:** Every step has explicit verifiers. Ask: "If this step is wrong, who catches it?"

**Current Design:** We have "red-flag checks" as post-hoc logging, not as **gates**.

```go
// Current: Red flags are warnings, not blockers
func (a *API) checkRedFlags(task *storage.TaskExecution) []string {
    var flags []string
    if len(resultStr) > 750 {
        flags = append(flags, "too_long")
    }
    return flags  // ❌ Just logged, doesn't fail task
}
```

**MDAP Would Do:**
```yaml
tasks:
  - id: generate-code
    type: code-generator
    output_schema: GeneratedCode
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

      # Voting (if high-stakes)
      - type: multi-sample-voter
        samples: 3
        strategy: majority
        fail_on_disagreement: true
```

**How Verification Works:**

1. **Task produces output** → Store as "unverified"
2. **Run L0 verifiers** (syntax, schema) → If fail, mark task as FAILED
3. **Run L1 verifiers** (semantic, constraints) → If fail, attempt repair or retry
4. **Run L2 verifiers** (cross-task consistency) → If fail, backtrack
5. **Only after all verifiers pass** → Mark output as "verified" and allow downstream tasks

**Implementation:**
```go
type Verifier struct {
    Type          string
    Params        map[string]any
    FailOnError   bool
    RepairOnFail  bool  // Try to fix vs. abort
}

type Task struct {
    ID         string
    Type       string
    Verifiers  []Verifier  // Run after task completes
}

func (e *Executor) executeTaskWithVerification(ctx context.Context, task *Task) error {
    // Execute task
    output, err := e.executeRaw(ctx, task)
    if err != nil {
        return err
    }

    // Run all verifiers
    for _, verifier := range task.Verifiers {
        result, err := e.runVerifier(ctx, verifier, output)
        if err != nil || !result.Passed {
            if verifier.FailOnError {
                // Hard fail - mark task as FAILED
                return fmt.Errorf("verifier %s failed: %v", verifier.Type, result.Error)
            }

            if verifier.RepairOnFail {
                // Attempt repair
                repaired, err := e.attemptRepair(ctx, task, output, result)
                if err != nil {
                    return err
                }
                output = repaired
            } else {
                // Log warning but continue
                log.Warn("verifier failed but not blocking", "verifier", verifier.Type)
            }
        }
    }

    // All verifiers passed - store verified output
    task.Output = output
    task.Status = TaskStateVerified
    return nil
}
```

**Action Items:**
- [ ] Add `verifiers: []Verifier` to task schema
- [ ] Implement verifier registry (syntax-validator, schema-checker, constraint-checker, etc.)
- [ ] Make verification **blocking** by default - tasks don't complete until verified
- [ ] Add repair loops: verifier fails → repair agent → re-verify

---

### ⚠️ Principle 5: Redundancy & Sampling for High-Stakes Steps

**MDAP Core Idea:** For critical decisions, run N variants and vote/rank.

**Current Design:** Single execution per task. No voting.

**MDAP Would Add:**
```yaml
tasks:
  - id: critical-data-transform
    type: data-transformer
    strategy: multi-sample-vote
    samples: 3
    voting:
      method: majority
      require_consensus: 2-of-3
      on_disagreement: human-review
```

**Implementation:**
```go
type TaskStrategy string

const (
    StrategySingle       TaskStrategy = "single"         // Default: one execution
    StrategyMultiSample  TaskStrategy = "multi-sample"   // Run N times, vote
    StrategyEnsemble     TaskStrategy = "ensemble"       // Different agents, vote
)

type Task struct {
    Strategy  TaskStrategy
    Samples   int                    // For multi-sample
    Voting    *VotingConfig
}

type VotingConfig struct {
    Method             string  // "majority", "unanimous", "weighted"
    RequireConsensus   string  // "2-of-3", "unanimous"
    OnDisagreement     string  // "fail", "human-review", "log-warning"
}

func (e *Executor) executeWithVoting(ctx context.Context, task *Task) error {
    if task.Strategy == StrategySingle {
        return e.executeOnce(ctx, task)
    }

    // Multi-sample execution
    results := make([]*TaskResult, task.Samples)
    for i := 0; i < task.Samples; i++ {
        result, err := e.executeOnce(ctx, task)
        if err != nil {
            log.Error("sample failed", "attempt", i, "error", err)
            continue
        }
        results[i] = result
    }

    // Vote on results
    winner, consensus := e.vote(results, task.Voting)
    if !consensus {
        switch task.Voting.OnDisagreement {
        case "fail":
            return fmt.Errorf("voting failed to reach consensus")
        case "human-review":
            return e.escalateToHuman(ctx, task, results)
        default:
            log.Warn("voting disagreement, using best-effort result")
        }
    }

    task.Output = winner
    return nil
}
```

**When to Use:**
- High-stakes decisions (deploy to production, financial transactions)
- Ambiguous inputs (text understanding, complex transforms)
- Safety-critical operations (access control, data deletion)

**When NOT to Use:**
- Deterministic operations (math, schema validation)
- Cheap idempotent operations (HTTP GET)
- Low-value operations (logging, metrics)

**Action Items:**
- [ ] Add `strategy` and `voting` to task schema
- [ ] Implement majority voting for task outputs
- [ ] Add "disagreement escalation" to human review
- [ ] Expose voting metrics (consensus rate, disagreement frequency)

---

### ✅ Principle 6: Small, Specialized Models/Tools

**MDAP Core Idea:** Many narrow, cheap agents > one god-agent.

**Current Design:** Already aligned! Our task types are agent types:

```go
type AgentDefinition struct {
    Type        string
    Description string
    Handler     func(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}
```

**Good!** Each agent is a microservice with single responsibility.

**Enhancement:** Agent library with 20-30 primitives:

**Deterministic Agents (no LLM):**
- `json-parser` - Parse JSON string to object
- `json-extractor` - Extract field from JSON via JSONPath
- `schema-validator` - Validate JSON against schema
- `http-executor` - Execute HTTP request
- `sql-query` - Execute SQL query
- `jq-transform` - JQ transformation
- `regex-matcher` - Match regex pattern
- `hash-calculator` - Calculate hash of input
- `timestamp-generator` - Generate current timestamp

**LLM-Powered Agents (small, focused prompts):**
- `text-classifier` - Classify text into categories
- `entity-extractor` - Extract named entities
- `json-repairer` - Fix malformed JSON
- `text-summarizer` - Summarize text to N words
- `code-formatter` - Format code with specific style
- `error-explainer` - Explain error message
- `constraint-checker` - Check if text satisfies constraints

**Action Items:**
- [ ] Build primitive agent library (start with 10 deterministic + 5 LLM)
- [ ] Document each agent: input schema, output schema, guarantees
- [ ] Make agents **composable** via DAG
- [ ] Measure cost/latency per agent type

---

### ❌ Principle 7: Short Context, External State

**MDAP Core Idea:** Don't pass entire history to every agent. Use summaries, IDs, external stores.

**Current Design:** We're passing `WorkflowDAG` around, which could get huge.

```go
func (e *Executor) executeTask(ctx context.Context, execCtx *ExecutionContext, task *Task) error {
    // What does task have access to?
    // - Just its input data? ✅
    // - Entire workflow history? ❌
}
```

**MDAP Would Do:**
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

func (e *Executor) executeTask(ctx context.Context, taskCtx *TaskContext) error {
    // Agent only sees:
    // 1. Its input
    // 2. Explicitly requested prior outputs (via GetPriorOutput)
    // 3. Nothing else

    agent := e.registry.Get(taskCtx.Type)
    output, err := agent.Handler(ctx, taskCtx.Input)
    return err
}
```

**Workflow Definition Language:**
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

**Benefits:**
- Agents can't "see" irrelevant context (prevents prompt injection, reduces cost)
- Context size stays small (sub-1000 tokens per task)
- Clear data lineage (know exactly what each task depends on)

**Action Items:**
- [ ] Add template language for task inputs (`${tasks.X.output.Y}`)
- [ ] Restrict agent access to only declared inputs
- [ ] Store workflow state externally (SQLite), pass only keys
- [ ] Add context size metrics per task

---

### ⚠️ Principle 8: Instrument Everything

**MDAP Core Idea:** Log all agent calls, track metrics, find flaky steps.

**Current Design:** We have basic logging and metrics, but missing:

**What We Need:**
```go
type TaskExecutionLog struct {
    TaskID        string
    InstanceID    string
    AgentType     string
    Input         json.RawMessage  // Or hash if large
    Output        json.RawMessage  // Or hash if large
    Duration      time.Duration
    Attempt       int
    VerifierResults []VerifierResult
    RedFlags      []string
    Cost          float64  // If LLM call
    Tokens        int      // If LLM call
}

type WorkflowMetrics struct {
    // Per-agent-type metrics
    AgentSuccessRate    map[string]float64
    AgentAvgDuration    map[string]time.Duration
    AgentAvgCost        map[string]float64

    // Per-verifier metrics
    VerifierFailureRate map[string]float64

    // Workflow-level metrics
    WorkflowSuccessRate float64
    AvgWorkflowDuration time.Duration
    RetryRate           float64
    HumanEscalationRate float64
}
```

**Queries We Want:**
- "Which agent types have highest failure rate?"
- "Which verifiers fail most often?"
- "Which workflows get stuck in repair loops?"
- "What's the average cost per workflow?"
- "Where do humans intervene most?"

**Action Items:**
- [ ] Log every agent call (input hash, output hash, duration, cost)
- [ ] Store verifier results in `task_verifications` table
- [ ] Add metrics endpoint: `GET /metrics/workflows`
- [ ] Build dashboards for failure rates, retry counts, costs
- [ ] Alert on: high failure rate, slow tasks, expensive workflows

---

### ⚠️ Principle 9: Failure Isolation & Recovery

**MDAP Core Idea:** Failures should be local, observable, recoverable.

**Current Design:** We have retry at task level, but missing:
- **Repair loops** (try to fix vs. just retry)
- **Partial success** (workflow completes with degraded output)
- **Failure quarantine** (mark data as uncertain, continue anyway)

**MDAP Would Add:**
```yaml
tasks:
  - id: parse-user-input
    type: json-parser
    on_failure:
      - attempt: repair-with-llm
        max_attempts: 2
      - escalate: human-review
      - fallback: use-default-values
```

**Implementation:**
```go
type FailurePolicy struct {
    Attempts   []RecoveryAttempt
}

type RecoveryAttempt struct {
    Type   string  // "retry", "repair", "escalate", "fallback"
    Agent  string  // Which agent to use for repair
    MaxAttempts int
}

func (e *Executor) executeWithRecovery(ctx context.Context, task *Task) error {
    output, err := e.executeOnce(ctx, task)
    if err == nil {
        return nil  // Success on first try
    }

    // Follow failure policy
    for _, attempt := range task.FailurePolicy.Attempts {
        switch attempt.Type {
        case "retry":
            // Same agent, fresh attempt
            output, err = e.executeOnce(ctx, task)
            if err == nil {
                return nil
            }

        case "repair":
            // Different agent tries to fix the output
            repairTask := &Task{
                Type: attempt.Agent,
                Input: createRepairInput(task, output, err),
            }
            repaired, err := e.executeOnce(ctx, repairTask)
            if err == nil {
                task.Output = repaired
                task.Status = TaskStateRepairedAndCompleted
                return nil
            }

        case "escalate":
            // Ask human for help
            return e.escalateToHuman(ctx, task, output, err)

        case "fallback":
            // Use degraded/default value
            task.Output = task.FallbackValue
            task.Status = TaskStateCompletedWithFallback
            return nil
        }
    }

    return fmt.Errorf("all recovery attempts exhausted: %w", err)
}
```

**Action Items:**
- [ ] Add failure policies to task schema
- [ ] Implement repair loops (call repair agent, re-verify)
- [ ] Support partial success (workflow continues with degraded data)
- [ ] Add "data uncertainty" markers (flag outputs as potentially wrong)

---

### ❌ Principle 10: Safety from Structure, Not Vibes

**MDAP Core Idea:** Don't rely on "smart agent makes good choices." Use structure + checks.

**Current Design:** We rely too much on agents "doing the right thing."

**What We Need:**

**1. Agent Privilege Levels:**
```go
type AgentCapabilities struct {
    CanReadFiles    bool
    CanWriteFiles   bool
    CanMakeHTTPCall bool
    CanExecuteCode  bool
    AllowedDomains  []string
    AllowedPaths    []string
}

type AgentDefinition struct {
    Type         string
    Capabilities AgentCapabilities  // Enforce at runtime
}
```

**2. Policy Enforcement:**
```yaml
tasks:
  - id: fetch-user-data
    type: http-fetch-agent
    enforce:
      - no_credentials_in_logs: true
      - tls_required: true
      - rate_limit: 100/min
```

**3. Human Gates at Critical Points:**
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

**Action Items:**
- [ ] Define agent capability model (read/write/execute permissions)
- [ ] Enforce capabilities at runtime (sandbox agents)
- [ ] Add policy engine (rate limits, domain restrictions, required approvals)
- [ ] Make human gates mandatory for destructive operations

---

## Redesigned Architecture: MDAP-Native

### Core Components

**1. Agent Registry**
```go
type AgentRegistry struct {
    agents map[string]*AgentDefinition
}

type AgentDefinition struct {
    Type          string
    Description   string
    InputSchema   *jsonschema.Schema
    OutputSchema  *jsonschema.Schema
    Capabilities  AgentCapabilities
    Handler       func(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}
```

**2. Workflow Definition (MDAP-Style)**
```yaml
apiVersion: ourocodus.dev/v1
kind: Workflow
metadata:
  name: data-ingestion

spec:
  # Initial input schema
  input_schema:
    type: object
    properties:
      api_url: {type: string}

  tasks:
    # Micro-decomposed tasks
    - id: build-request
      type: request-builder
      input:
        url: ${workflow.input.api_url}
      output_schema: HTTPRequest

    - id: execute-request
      type: http-executor
      dependencies: [build-request]
      input:
        request: ${tasks.build-request.output}
      output_schema: HTTPResponse
      verifiers:
        - type: status-validator
          params: {allowed: [200, 201]}
          fail_on_error: true

    - id: parse-response
      type: json-parser
      dependencies: [execute-request]
      input:
        body: ${tasks.execute-request.output.body}
      output_schema: ParsedJSON
      on_failure:
        - attempt: repair-with-llm
          max_attempts: 2
        - escalate: human-review

    - id: validate-schema
      type: schema-validator
      dependencies: [parse-response]
      input:
        data: ${tasks.parse-response.output}
        schema: {type: object, required: [id, data]}
      verifiers:
        - type: constraint-checker
          params:
            rules:
              - "data.length > 0"
          fail_on_error: true
```

**3. Executor (MDAP-Style)**
```go
func (e *Executor) ExecuteTask(ctx context.Context, task *Task) error {
    // 1. Validate input against schema
    if err := task.InputSchema.Validate(task.Input); err != nil {
        return fmt.Errorf("input validation failed: %w", err)
    }

    // 2. Execute with recovery policy
    output, err := e.executeWithRecovery(ctx, task)
    if err != nil {
        return err
    }

    // 3. Validate output against schema
    if err := task.OutputSchema.Validate(output); err != nil {
        return fmt.Errorf("output validation failed: %w", err)
    }

    // 4. Run all verifiers
    for _, verifier := range task.Verifiers {
        if err := e.runVerifier(ctx, verifier, output); err != nil {
            if verifier.FailOnError {
                return err
            }
            log.Warn("verifier failed", "verifier", verifier.Type, "error", err)
        }
    }

    // 5. Store verified output
    task.Output = output
    task.Status = TaskStateVerified
    return nil
}
```

---

## Human Guiding/Herding Integration Points

**Where humans can intervene:**

**1. Pre-Workflow (Planning):**
- Review workflow definition before execution
- Adjust task parameters
- Add/remove verification steps
- Set approval gates

**2. Mid-Workflow (Steering):**
- Approve/reject at approval gates
- Override failed verifiers ("continue anyway")
- Inject new tasks dynamically
- Adjust task priorities
- Pause/resume workflows

**3. Post-Workflow (Learning):**
- Review execution logs
- Mark task outputs as "good" or "bad" (training data)
- Adjust agent prompts based on failures
- Refine verification rules

**Implementation:**
```yaml
tasks:
  - id: risky-operation
    type: data-transformer
    # Human can review plan before execution
    plan_review: optional

  - id: critical-decision
    type: decision-maker
    # Human must approve before execution
    requires_approval: true

  - id: complex-task
    type: multi-step-agent
    # Human can steer during execution
    interactive: true
    steering_points:
      - after: generate-candidates
        prompt: "Review candidates before voting"
      - after: vote
        prompt: "Override vote result?"
```

---

## Summary: What Changes

| Current Design | MDAP-Native Design |
|----------------|-------------------|
| Tasks are "smart agents" | Tasks are **micro-primitives** |
| One agent per task | Many micro-agents per workflow |
| Untyped I/O (`any`) | **Strict schemas** with validation |
| Red flags as warnings | **Verifiers as gates** |
| Single execution | **Voting/sampling** for critical steps |
| Retry on failure | **Repair loops** + fallbacks |
| Hope agents work | **Explicit verification** at every step |
| Long context prompts | **Short, focused context** |
| Post-hoc logging | **Built-in instrumentation** |
| Trust agents | **Zero trust** + capability model |

---

## Next Steps

<function_calls>
<invoke name="TodoWrite">
<parameter name="todos">[{"content": "Audit current design against MDAP principles", "status": "completed", "activeForm": "Auditing current design against MDAP principles"}, {"content": "Identify gaps in decomposition and verification strategy", "status": "completed", "activeForm": "Identifying gaps in decomposition and verification strategy"}, {"content": "Design micro-agent primitive library (20-30 agents)", "status": "pending", "activeForm": "Designing micro-agent primitive library"}, {"content": "Add schema validation to task I/O", "status": "pending", "activeForm": "Adding schema validation to task I/O"}, {"content": "Implement verifier registry and L0/L1/L2 checks", "status": "pending", "activeForm": "Implementing verifier registry and L0/L1/L2 checks"}, {"content": "Add repair loops and failure recovery policies", "status": "pending", "activeForm": "Adding repair loops and failure recovery policies"}, {"content": "Design human steering integration points", "status": "pending", "activeForm": "Designing human steering integration points"}]