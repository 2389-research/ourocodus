# MDAP Foundation → Milestone 5 Implementation Roadmap

## Executive Summary

This document maps the foundational MDAP (Massively Decomposed Agentic Processes) design to specific GitHub Milestone 5 issues, providing a concrete implementation roadmap that bridges MDAP theory with the Ourocodus project's autonomous coordination goals.

**Key Insight:** The foundational MDAP components directly address 7 of 9 Milestone 5 issues, with clear paths for the remaining 2.

---

## Architecture Alignment

### MDAP Core Principles → Milestone 5 Deliverables

| MDAP Principle | Milestone 5 Deliverable | GitHub Issues |
|----------------|-------------------------|---------------|
| **WorkflowDAG** (task dependencies) | Sequential workflow execution engine | #127, #126 |
| **WorkflowExecutor** (orchestration) | Coordinator service foundation | #121, #127 |
| **StateStore** (persistence) | Workflow persistence (SQLite) | #122 |
| **MicroAgent** (single-purpose agents) | Task lifecycle & monitoring | #128 |
| **Verification** (L0/L1 checks) | Workflow operations (audit) | #129 |
| **Red-flagging** (suspicious outputs) | Basic observability | #125 |
| **NATS Integration** (event-driven) | NATS event handlers | #124 |
| **REST API** (workflow management) | HTTP API for workflow management | #123 |

---

## Issue-by-Issue Implementation Guide

### ✅ Issue #121: Coordinator Service Foundation

**MDAP Component:** `oos/core/executor.py` → `pkg/coordinator/coordinator.go`

**Mapping:**
```python
# MDAP Foundation (Python conceptual)
class WorkflowExecutor:
    def __init__(self, agent_registry, state_store, config):
        self.agent_registry = agent_registry
        self.state_store = state_store
        self.config = config
```

**Go Implementation:**
```go
// pkg/coordinator/coordinator.go
package coordinator

type Coordinator struct {
    sessionMgr  *session.Manager      // From M2
    natsClient  *nats.Conn            // From M4
    stateStore  *storage.SQLiteStore  // Issue #122
    config      *Config
}

func New(cfg *Config, sessionMgr *session.Manager, nc *nats.Conn) (*Coordinator, error) {
    store, err := storage.NewSQLiteStore(cfg.DBPath)
    if err != nil {
        return nil, err
    }
    return &Coordinator{
        sessionMgr: sessionMgr,
        natsClient: nc,
        stateStore: store,
        config:     cfg,
    }, nil
}

func (c *Coordinator) Start(ctx context.Context) error {
    // Lifecycle management
    // Restore in-progress workflows from SQLite
    // Setup NATS subscriptions
    return nil
}

func (c *Coordinator) Shutdown(ctx context.Context) error {
    // Graceful shutdown
    // Save all workflow state
    return c.stateStore.Close()
}
```

**Key Integration Points:**
- Session manager from M2 (multi-session tracking)
- NATS client from M4 (event-driven communication)
- SQLite state store (new, see #122)

**Acceptance Criteria Mapping:**
- ✅ Service lifecycle → `Start()` / `Shutdown()` methods
- ✅ Configuration loading → `Config` struct with env vars
- ✅ Relay architecture integration → Use existing `sessionMgr`
- ✅ Health check endpoint → `/health` in HTTP API (see #123)

---

### ✅ Issue #122: Workflow Persistence with SQLite

**MDAP Component:** `oos/storage/state.py` → `pkg/coordinator/storage/sqlite.go`

**Mapping:**
```python
# MDAP Foundation
class WorkflowStateStore:
    def save_workflow(self, workflow: WorkflowDAG) -> str:
        """Persist workflow definition and state"""
        pass

    def load_workflow(self, workflow_id: str) -> Optional[WorkflowDAG]:
        """Restore workflow from storage"""
        pass

    def update_node_status(self, workflow_id: str, node_id: str, status: NodeStatus):
        """Update individual node status"""
        pass
```

**Go Implementation:**
```go
// pkg/coordinator/storage/sqlite.go
package storage

type SQLiteStore struct {
    db *sql.DB
}

// Schema matches MDAP concepts
// workflow_definitions (id, name, yaml_def, created_at)
// workflow_instances (id, definition_id, status, started_at, completed_at)
// task_executions (id, instance_id, task_id, status, result, started_at, completed_at)
// workflow_events (id, instance_id, event_type, timestamp, data, correlation_id)

func (s *SQLiteStore) SaveWorkflowDefinition(ctx context.Context, def *WorkflowDefinition) error
func (s *SQLiteStore) GetWorkflowDefinition(ctx context.Context, id string) (*WorkflowDefinition, error)
func (s *SQLiteStore) CreateInstance(ctx context.Context, defID string) (*WorkflowInstance, error)
func (s *SQLiteStore) UpdateInstanceStatus(ctx context.Context, id string, status Status) error
func (s *SQLiteStore) RecordTaskExecution(ctx context.Context, exec *TaskExecution) error
func (s *SQLiteStore) RecordEvent(ctx context.Context, event *WorkflowEvent) error
```

**Schema Design:**
```sql
-- Mirrors MDAP WorkflowDAG structure
CREATE TABLE workflow_definitions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    yaml_definition TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE workflow_instances (
    id TEXT PRIMARY KEY,
    definition_id TEXT NOT NULL,
    status TEXT NOT NULL,  -- PENDING, RUNNING, COMPLETED, FAILED, CANCELLED
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (definition_id) REFERENCES workflow_definitions(id)
);

CREATE TABLE task_executions (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL,
    task_id TEXT NOT NULL,  -- From WorkflowNode.id
    agent_id TEXT,
    status TEXT NOT NULL,   -- PENDING, ASSIGNED, RUNNING, COMPLETED, FAILED
    result TEXT,            -- JSON-encoded result
    error TEXT,
    attempts INTEGER DEFAULT 1,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES workflow_instances(id)
);

CREATE TABLE workflow_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    correlation_id TEXT,    -- Critical for debugging async flows
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    data TEXT,              -- JSON-encoded event data
    FOREIGN KEY (instance_id) REFERENCES workflow_instances(id)
);

CREATE INDEX idx_instances_status ON workflow_instances(status);
CREATE INDEX idx_tasks_instance ON task_executions(instance_id);
CREATE INDEX idx_events_instance ON workflow_events(instance_id);
CREATE INDEX idx_events_correlation ON workflow_events(correlation_id);
```

**MDAP Integration:**
- Stores WorkflowDAG structure (definitions table)
- Tracks WorkflowNode status (task_executions table)
- Enables idempotency (check task status before execution)
- Supports replay from checkpoint (query last completed task)

---

### ✅ Issue #126: Workflow Definition Parser

**MDAP Component:** `oos/core/dag.py` → `pkg/coordinator/parser/parser.go`

**Mapping:**
```python
# MDAP Foundation
@dataclass
class WorkflowNode:
    id: str
    agent_type: str
    dependencies: List[str] = field(default_factory=list)
    config: Dict[str, Any] = field(default_factory=dict)

@dataclass
class WorkflowDAG:
    id: str
    name: str
    nodes: List[WorkflowNode]

    def get_ready_nodes(self) -> List[WorkflowNode]:
        """Return nodes where all dependencies are completed"""
        pass
```

**Go Implementation:**
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

**Example YAML Format:**
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

**MDAP Alignment:**
- Each task = MicroAgent (single responsibility)
- Dependencies = DAG edges
- Sequential execution enforced by dependency chain
- Validation prevents malformed workflows

---

### ✅ Issue #127: Sequential Workflow Execution Engine

**MDAP Component:** `oos/core/executor.py` → `pkg/coordinator/engine/executor.go`

**Mapping:**
```python
# MDAP Foundation
class WorkflowExecutor:
    async def execute(self, dag: WorkflowDAG) -> Dict[str, Any]:
        while not dag.is_complete():
            ready_nodes = dag.get_ready_nodes()
            for node in ready_nodes:
                await self._execute_node(dag, node)
        return results

    async def _execute_node(self, dag: WorkflowDAG, node: WorkflowNode):
        for attempt in range(1, max_retries + 1):
            try:
                result = await agent.execute(input_data)
                node.status = NodeStatus.COMPLETED
                node.result = result
                return
            except Exception as e:
                # Retry logic
                pass
```

**Go Implementation:**
```go
// pkg/coordinator/engine/executor.go
package engine

type ExecutionContext struct {
    WorkflowID   string
    InstanceID   string
    CurrentTask  string
    CompletedTasks map[string]*TaskResult
    Status       Status
    StartedAt    time.Time
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
        WorkflowID:    def.ID,
        InstanceID:    instance.ID,
        CompletedTasks: make(map[string]*TaskResult),
        Status:        StatusRunning,
        StartedAt:     time.Now(),
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

**MDAP Features Implemented:**
1. **Idempotency:** Check `CompletedTasks` before execution
2. **Explicit Serialization:** `serializeContext()` after each task
3. **Crash Recovery:** `RestoreAndResume()` loads context and continues
4. **Sequential Execution:** Simple for-loop over tasks
5. **Timeout Handling:** Per-task timeouts with default fallback
6. **Event Logging:** Record all state transitions

**Key Differences from Parallel MDAP:**
- Sequential execution (MVP constraint)
- No voting (single agent per task)
- Red-flagging deferred to observability (#125)

---

### ✅ Issue #128: Task Lifecycle & Monitoring

**MDAP Component:** `oos/core/agents.py` + `oos/core/verification.py` → `pkg/coordinator/engine/task.go`

**Mapping:**
```python
# MDAP Foundation
class MicroAgent(ABC):
    @abstractmethod
    async def execute(self, input_data: Dict[str, Any]) -> Any:
        pass

    async def validate_output(self, output: Any) -> bool:
        """L0/L1 verification"""
        pass

class NodeStatus(Enum):
    PENDING = "pending"
    RUNNING = "running"
    COMPLETED = "completed"
    FAILED = "failed"
```

**Go Implementation:**
```go
// pkg/coordinator/engine/task.go
package engine

type TaskState string

const (
    TaskStatePending   TaskState = "PENDING"
    TaskStateAssigned  TaskState = "ASSIGNED"   // New state: task assigned to specific agent
    TaskStateRunning   TaskState = "RUNNING"
    TaskStateCompleted TaskState = "COMPLETED"
    TaskStateFailed    TaskState = "FAILED"
)

type TaskExecution struct {
    ID          string
    InstanceID  string
    TaskID      string
    AgentID     string    // Which agent is executing this task
    State       TaskState
    Result      any
    Error       string
    Attempts    int
    StartedAt   time.Time
    CompletedAt time.Time
    Duration    time.Duration
}

type TaskMonitor struct {
    store   *storage.SQLiteStore
    metrics *metrics.Registry
}

func (m *TaskMonitor) TrackTaskExecution(ctx context.Context, exec *TaskExecution) error {
    // Record state transition
    if err := m.store.RecordTaskExecution(ctx, exec); err != nil {
        return err
    }

    // Update metrics
    m.metrics.TaskDuration.Observe(exec.Duration.Seconds())
    if exec.State == TaskStateFailed {
        m.metrics.TaskFailures.Inc()
    }

    return nil
}

func (m *TaskMonitor) HandleTimeout(ctx context.Context, taskID string) error {
    // Mark task as failed
    exec := &TaskExecution{
        TaskID: taskID,
        State:  TaskStateFailed,
        Error:  "task timeout",
    }

    // Update SQLite
    if err := m.store.UpdateTaskStatus(ctx, taskID, TaskStateFailed); err != nil {
        return err
    }

    // Publish timeout event
    // ... NATS publish

    return nil
}
```

**Monitoring Metrics (Prometheus):**
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

**MDAP Alignment:**
- Task state machine mirrors MDAP NodeStatus
- Agent assignment tracking enables future voting features
- Timeout handling critical for reliability
- Metrics support error budget tracking (MDAP Section 4.2)

---

### ✅ Issue #124: NATS Event Handlers

**MDAP Component:** Message Queue Integration → `pkg/coordinator/events/handlers.go`

**Mapping:**
```python
# MDAP Foundation (async communication)
# Agents publish completion events
# Executor subscribes and updates workflow state
```

**Go Implementation:**
```go
// pkg/coordinator/events/schemas.go
package events

// Shared event schemas
type AgentTaskCompleted struct {
    TaskID        string         `json:"task_id"`
    InstanceID    string         `json:"instance_id"`
    CorrelationID string         `json:"correlation_id"`  // Critical for debugging
    Result        any            `json:"result"`
    Timestamp     time.Time      `json:"timestamp"`
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
        log.Error("failed to unmarshal event", "error", err, "correlation_id", event.CorrelationID)
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

    // Update workflow state
    ctx := context.Background()
    if err := h.store.UpdateTaskStatus(ctx, event.TaskID, engine.TaskStateFailed); err != nil {
        log.Error("failed to update task status", "error", err, "correlation_id", event.CorrelationID)
        return
    }

    // Record event
    h.store.RecordEvent(ctx, &storage.WorkflowEvent{
        InstanceID:    event.InstanceID,
        EventType:     "task.failed",
        CorrelationID: event.CorrelationID,
        Data:          event.Error,
    })

    log.Error("task failed", "task_id", event.TaskID, "error", event.Error, "correlation_id", event.CorrelationID)
}

func (h *EventHandler) handleStatusRequest(msg *nats.Msg) {
    var req WorkflowStatusRequested
    if err := json.Unmarshal(msg.Data, &req); err != nil {
        log.Error("failed to unmarshal status request", "error", err)
        return
    }

    // Get workflow status
    ctx := context.Background()
    instance, err := h.store.GetWorkflowInstance(ctx, req.InstanceID)
    if err != nil {
        log.Error("failed to get workflow instance", "error", err, "correlation_id", req.CorrelationID)
        return
    }

    // Publish status response
    statusMsg := &WorkflowStatusResponse{
        InstanceID:    req.InstanceID,
        CorrelationID: req.CorrelationID,
        Status:        instance.Status,
        CompletedTasks: instance.CompletedTasks,
    }

    responseSubject := fmt.Sprintf("workflow.status.response.%s", req.CorrelationID)
    if err := h.nc.Publish(responseSubject, statusMsg); err != nil {
        log.Error("failed to publish status response", "error", err, "correlation_id", req.CorrelationID)
    }
}
```

**MDAP Alignment:**
- Correlation IDs enable tracing across async flows
- Event handlers update workflow state (maintain consistency)
- All events logged to SQLite (audit trail)
- NATS subjects map to agent types (decoupled communication)

**NATS Subject Design:**
```
agents.<agent_type>              → Agent subscribes for tasks
agents.<task_id>.result          → Coordinator subscribes for results
agents.<task_id>.completed       → Broadcast task completion
agents.<task_id>.failed          → Broadcast task failure
workflow.status.requested        → Request workflow status
workflow.status.response.<cid>   → Status response (correlation ID)
```

---

### ✅ Issue #123: HTTP API - Workflow Management

**MDAP Component:** `oos/api.py` → `pkg/coordinator/api/handlers.go`

**Mapping:**
```python
# MDAP Foundation
@app.post("/workflows/execute")
async def execute_workflow(workflow_def: WorkflowDefinition):
    dag = WorkflowDAG(...)
    executor = WorkflowExecutor(...)
    result = await executor.execute(dag)
    return result
```

**Go Implementation:**
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
        // Workflow already running
        writeJSON(w, http.StatusConflict, StartWorkflowResponse{
            InstanceID: existingInstance.ID,
            Status:     "already_running",
        })
        return
    }

    // Start execution (async)
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

    // Get task details
    tasks, err := a.store.GetTaskExecutions(r.Context(), instanceID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to get tasks")
        return
    }

    writeJSON(w, http.StatusOK, WorkflowStatusResponse{
        InstanceID:     instance.ID,
        WorkflowID:     instance.DefinitionID,
        Status:         instance.Status,
        StartedAt:      instance.StartedAt,
        CompletedAt:    instance.CompletedAt,
        Tasks:          tasks,
    })
}

// DELETE /v1/workflows/:id/instances/:instance_id - Cancel workflow
func (a *API) CancelWorkflow(w http.ResponseWriter, r *http.Request) {
    instanceID := chi.URLParam(r, "instance_id")

    // Mark workflow as cancelled
    if err := a.store.UpdateInstanceStatus(r.Context(), instanceID, engine.StatusCancelled); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to cancel workflow")
        return
    }

    // Publish cancellation event
    a.executor.CancelWorkflow(r.Context(), instanceID)

    writeJSON(w, http.StatusOK, CancelWorkflowResponse{Status: "cancelled"})
}

// Middleware: API key authentication
func (a *API) AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := r.Header.Get("X-API-Key")
        if apiKey == "" {
            writeError(w, http.StatusUnauthorized, "missing API key")
            return
        }

        // Validate API key
        if !a.validateAPIKey(apiKey) {
            writeError(w, http.StatusUnauthorized, "invalid API key")
            return
        }

        next.ServeHTTP(w, r)
    })
}

// Middleware: Rate limiting
func (a *API) RateLimitMiddleware(next http.Handler) http.Handler {
    // Token bucket implementation
    // 100 req/min per API key
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

**MDAP Alignment:**
- Replay protection prevents duplicate workflow starts
- Status endpoint exposes workflow DAG state
- Cancellation supports graceful shutdown
- Rate limiting prevents abuse

---

### ✅ Issue #129: Workflow Operations (Cancellation, Replay Protection, Audit)

**MDAP Component:** Verification + State Management → `pkg/coordinator/ops/operations.go`

**Mapping:**
```python
# MDAP Foundation
# Replay protection = idempotency checks
# Audit trail = event logging
# Cancellation = graceful shutdown
```

**Go Implementation:**
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
    // Get current instance
    instance, err := o.store.GetWorkflowInstance(ctx, instanceID)
    if err != nil {
        return err
    }

    // Check if already terminal state
    if instance.Status == engine.StatusCompleted ||
       instance.Status == engine.StatusFailed ||
       instance.Status == engine.StatusCancelled {
        return fmt.Errorf("workflow already in terminal state: %s", instance.Status)
    }

    // Mark as cancelled
    if err := o.store.UpdateInstanceStatus(ctx, instanceID, engine.StatusCancelled); err != nil {
        return err
    }

    // Publish cancellation event (agents should check and stop)
    cancelEvent := &events.WorkflowCancelled{
        InstanceID:    instanceID,
        CorrelationID: generateCorrelationID(),
        Timestamp:     time.Now(),
    }

    if err := o.nc.Publish("workflow.cancelled", cancelEvent); err != nil {
        return err
    }

    // Record audit event
    o.store.RecordEvent(ctx, &storage.WorkflowEvent{
        InstanceID:    instanceID,
        EventType:     "workflow.cancelled",
        CorrelationID: cancelEvent.CorrelationID,
        Data:          "user_requested",
    })

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
        // Return existing instance ID
        return existingInstance.ID, nil
    }

    if existingInstance != nil && forceRestart {
        // Cancel existing instance first
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

    // Apply filters
    if filters != nil {
        events = o.applyFilters(events, filters)
    }

    return events, nil
}

func (o *Operations) applyFilters(events []*storage.WorkflowEvent, filters *AuditFilters) []*storage.WorkflowEvent {
    var filtered []*storage.WorkflowEvent
    for _, event := range events {
        if filters.EventType != "" && event.EventType != filters.EventType {
            continue
        }
        if filters.StartTime != nil && event.Timestamp.Before(*filters.StartTime) {
            continue
        }
        if filters.EndTime != nil && event.Timestamp.After(*filters.EndTime) {
            continue
        }
        filtered = append(filtered, event)
    }
    return filtered
}
```

**MDAP Alignment:**
- Replay protection = MDAP idempotency guarantee
- Audit trail = Full event history for debugging
- Cancellation = Graceful failure mode

---

### ✅ Issue #125: Basic Workflow Observability

**MDAP Component:** Red-flagging + Verification → `pkg/coordinator/api/debug.go`

**Mapping:**
```python
# MDAP Foundation
class RedFlagChecker:
    def check(self, output: Any) -> Dict[str, bool]:
        # Detect suspicious outputs
        pass

class Verifier:
    async def verify(self, output: Any) -> List[VerificationResult]:
        # L0/L1 verification
        pass
```

**Go Implementation:**
```go
// pkg/coordinator/api/debug.go
package api

// GET /v1/debug/workflows/:id/status - Detailed workflow state
func (a *API) DebugWorkflowStatus(w http.ResponseWriter, r *http.Request) {
    instanceID := chi.URLParam(r, "id")

    // Get full workflow state
    instance, err := a.store.GetWorkflowInstance(r.Context(), instanceID)
    if err != nil {
        writeError(w, http.StatusNotFound, "instance not found")
        return
    }

    // Get all tasks
    tasks, err := a.store.GetTaskExecutions(r.Context(), instanceID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to get tasks")
        return
    }

    // Get execution context (for crash recovery debugging)
    execCtx, err := a.store.LoadInstanceContext(r.Context(), instanceID)
    if err != nil {
        // Context may not exist yet
        execCtx = nil
    }

    writeJSON(w, http.StatusOK, DebugStatusResponse{
        Instance:         instance,
        Tasks:            tasks,
        ExecutionContext: execCtx,
        SystemTime:       time.Now(),
    })
}

// GET /v1/debug/workflows/:id/events - Event history
func (a *API) DebugWorkflowEvents(w http.ResponseWriter, r *http.Request) {
    instanceID := chi.URLParam(r, "id")

    // Get all events with correlation IDs
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

// GET /v1/debug/workflows/:id/tasks - Task execution log
func (a *API) DebugWorkflowTasks(w http.ResponseWriter, r *http.Request) {
    instanceID := chi.URLParam(r, "id")

    // Get detailed task execution log
    tasks, err := a.store.GetTaskExecutions(r.Context(), instanceID)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to get tasks")
        return
    }

    // Include red-flag checks (MDAP Section 3.3)
    for _, task := range tasks {
        task.RedFlags = a.checkRedFlags(task)
    }

    writeJSON(w, http.StatusOK, DebugTasksResponse{
        InstanceID: instanceID,
        Tasks:      tasks,
    })
}

// Red-flag checks (MDAP-inspired)
func (a *API) checkRedFlags(task *storage.TaskExecution) []string {
    var flags []string

    // Too long check (Section 3.3)
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

// Prometheus metrics endpoint
// GET /metrics
func (a *API) Metrics(w http.ResponseWriter, r *http.Request) {
    promhttp.Handler().ServeHTTP(w, r)
}
```

**Logging Configuration:**
```go
// pkg/coordinator/logging/logger.go
package logging

func SetupLogger(config *Config) *slog.Logger {
    var level slog.Level
    switch config.LogLevel {
    case "DEBUG":
        level = slog.LevelDebug
    case "INFO":
        level = slog.LevelInfo
    case "WARN":
        level = slog.LevelWarn
    case "ERROR":
        level = slog.LevelError
    }

    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: level,
        AddSource: true,
    })

    return slog.New(handler)
}

// All workflow operations include correlation IDs
log.Info("workflow started",
    "workflow_id", workflowID,
    "instance_id", instanceID,
    "correlation_id", correlationID)
```

**MDAP Alignment:**
- Red-flag checks detect suspicious outputs (Section 3.3)
- Debug endpoints expose full workflow state
- Correlation IDs enable async flow tracing
- Structured logging supports debugging

---

## Integration with Milestone 4 (NATS)

### NATS Subjects from M4 → Coordinator M5

| M4 Component | NATS Subject | M5 Consumer | Purpose |
|--------------|--------------|-------------|---------|
| Relay NATS Publisher | `relay.events.*` | Coordinator Event Handler | Relay events trigger workflows |
| Agent Task Results | `agents.<type>.result` | Executor | Task completion updates |
| Load Balancing | `agents.<type>` | Task Dispatch | Distribute tasks to available agents |
| Event Logger | `*.events.*` | (Passive) | All events logged for audit |

### Key Integration Points

1. **Coordinator subscribes to agent results:**
   ```go
   nc.Subscribe("agents.*.result", executor.handleTaskResult)
   ```

2. **Coordinator publishes task assignments:**
   ```go
   nc.Publish("agents.http-fetch-agent", taskMsg)
   ```

3. **Relay events can trigger workflows:**
   ```go
   nc.Subscribe("relay.events.approval.approved", func(msg *nats.Msg) {
       // Start approval workflow
       workflowID := extractWorkflowID(msg)
       coordinator.StartWorkflow(workflowID)
   })
   ```

4. **Event logger captures all coordinator events:**
   - `workflow.started`
   - `workflow.completed`
   - `workflow.failed`
   - `workflow.cancelled`
   - `task.assigned`
   - `task.completed`
   - `task.failed`

---

## Roadmap: Week-by-Week Implementation

### Week 1: Foundation (Issues #121, #122)
- [ ] Create `pkg/coordinator` package structure
- [ ] Implement coordinator service lifecycle
- [ ] Setup SQLite schema and repository pattern
- [ ] Integration: Hook into existing relay architecture
- [ ] **Deliverable:** Coordinator service starts, SQLite initialized

### Week 2: Parser + Executor (Issues #126, #127)
- [ ] Implement workflow definition parser (YAML/JSON)
- [ ] Create internal DAG representation
- [ ] Build sequential execution engine
- [ ] Add explicit context serialization
- [ ] Implement crash recovery (`RestoreAndResume`)
- [ ] **Deliverable:** Can execute simple sequential workflow from YAML

### Week 3: NATS + API (Issues #124, #123, #128)
- [ ] Implement NATS event handlers
- [ ] Setup HTTP API with authentication and rate limiting
- [ ] Add task lifecycle tracking
- [ ] Implement timeout management
- [ ] **Deliverable:** End-to-end workflow via API + NATS

### Week 4: Operations + Observability (Issues #129, #125)
- [ ] Implement workflow cancellation
- [ ] Add replay protection
- [ ] Setup audit logging
- [ ] Create debug endpoints
- [ ] Add Prometheus metrics
- [ ] **Deliverable:** Production-ready M5 with observability

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

## MDAP Features Deferred to Future Milestones

The following MDAP features are intentionally **not** included in Milestone 5:

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

## Migration from MDAP Foundation (Python) → Ourocodus (Go)

### Type Mappings

| Python (MDAP Foundation) | Go (Ourocodus M5) |
|--------------------------|-------------------|
| `WorkflowDAG` | `parser.WorkflowDefinition` |
| `WorkflowNode` | `parser.Task` |
| `NodeStatus` | `engine.TaskState` |
| `WorkflowExecutor` | `engine.Executor` |
| `MicroAgent` | Agent (subscribes to NATS) |
| `WorkflowStateStore` | `storage.SQLiteStore` |
| `ExecutionContext` | `engine.ExecutionContext` |
| `VerificationResult` | `ops.RedFlagResult` |

### Configuration Translation

**Python (MDAP Foundation):**
```python
config = ExecutorConfig(
    max_retries=3,
    timeout_seconds=300,
    fail_fast=True
)
```

**Go (Ourocodus M5):**
```go
type ExecutorConfig struct {
    MaxRetries      int           `env:"EXECUTOR_MAX_RETRIES" default:"3"`
    DefaultTimeout  time.Duration `env:"EXECUTOR_TIMEOUT" default:"5m"`
    FailFast        bool          `env:"EXECUTOR_FAIL_FAST" default:"true"`
}
```

---

## Appendix: Example Workflow YAML

```yaml
apiVersion: ourocodus.dev/v1
kind: Workflow
metadata:
  name: data-ingestion-pipeline
  description: Fetch, validate, transform, and store data
  version: "1.0"

spec:
  tasks:
    # Task 1: Fetch data from API
    - id: fetch-raw-data
      type: http-fetch-agent
      params:
        url: "https://api.example.com/data"
        method: GET
        headers:
          Authorization: "Bearer ${API_TOKEN}"
      timeout: 30s

    # Task 2: Validate data structure
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

    # Task 3: Transform data
    - id: transform-data
      type: jq-transform-agent
      dependencies:
        - validate-schema
      params:
        expression: '.data[] | {id: .id, value: .value, timestamp: now}'
      timeout: 30s

    # Task 4: Store in database
    - id: store-data
      type: postgres-writer-agent
      dependencies:
        - transform-data
      params:
        connection: "postgresql://localhost/mydb"
        table: "ingested_data"
        conflict_strategy: "upsert"
      timeout: 60s

    # Task 5: Send notification
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

---

## Questions for Review

1. **SQLite vs. PostgreSQL:** Issue #122 specifies SQLite. Is this acceptable for production, or should we plan migration to PostgreSQL?

2. **Sequential Execution Constraint:** Issue #127 enforces sequential execution. When should we add parallelism? (Milestone 6? 7?)

3. **Agent Registry:** MDAP foundation includes agent registry. Should this be part of M5 or deferred?

4. **Retry Logic:** Issue #59 (M4) mentions retry logic. Should this be part of Issue #127 executor, or separate?

5. **Approval Gates:** Issue #53 (M4) mentions approval integration. How does this interact with workflow execution? Should workflows pause at approval tasks?

---

## Next Steps

1. **Review this mapping with team**
2. **Prioritize issues within M5** (suggested order: #121 → #122 → #126 → #127 → #124 → #123 → #128 → #129 → #125)
3. **Create detailed implementation tickets** for each issue with code examples from this document
4. **Setup development environment** with NATS from M4
5. **Begin Week 1 implementation** (Coordinator foundation + SQLite persistence)

---

## Document Metadata

- **Created:** 2025-11-18
- **MDAP Paper:** "Solving a Million-Step LLM Task with Zero Errors" (Anthropic, 2024)
- **Target Milestone:** Milestone 5: Autonomous Coordination (GitHub #6)
- **Dependencies:** Milestone 4: NATS Integration (GitHub #5)
- **Blocks:** Milestone 6: Production Polish (GitHub #8)
