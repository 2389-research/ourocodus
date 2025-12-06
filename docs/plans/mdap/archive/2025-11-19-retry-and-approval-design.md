# Retry Logic & Approval Gate Integration Design

## Executive Summary

This document addresses two critical Milestone 5 integration questions:

1. **Issue #59 (Retry Logic):** How retry/recovery integrates with the workflow engine
2. **Issue #53 (Approval Gates):** How human-in-the-loop approval blocks workflow execution

**Key Insight:** Both features are **workflow-level concerns** that the coordinator orchestrates, not low-level transport concerns handled by NATS client.

---

## Part 1: Retry Logic Integration (#59)

### Current Retry Infrastructure

Your codebase already has **excellent** retry infrastructure in `pkg/nats/client.go`:

```go
// pkg/nats/client.go:203-236
// Publish with retry
var lastErr error
for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
    if attempt > 0 {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(c.config.RetryBackoff.Next(attempt)):
        }
    }

    err := c.conn.PublishMsg(msg)
    if err == nil {
        c.metrics.recordPublish(subject, time.Since(start), nil)
        return nil
    }

    lastErr = err

    // Check if error is retryable
    if !isTransientError(err) {
        c.metrics.recordPublish(subject, time.Since(start), err)
        return WrapPermanentError("publish", subject, err)
    }
```

**What this handles:** Transient NATS transport failures (network blips, server restarts)

**What it doesn't handle:**
- Agent crashes during task execution
- Agent heartbeat timeouts
- Task-level business logic failures
- Reassigning failed tasks to different agents

### The Two Layers of Retry

There are **two distinct retry layers** in a robust orchestration system:

#### Layer 1: Transport Retry (NATS Client) ✅ Already Implemented

**Location:** `pkg/nats/client.go:203-236`

**What it retries:**
- Failed message publishes (network issues)
- Request timeouts (server overload)
- Connection drops (transient disconnects)

**Retry policy:**
- Exponential backoff via `c.config.RetryBackoff.Next(attempt)`
- Max attempts: `c.config.RetryAttempts`
- Only retries transient errors: `isTransientError(err)`

**Idempotency:** NATS request-reply is idempotent (same request ID)

#### Layer 2: Task Retry (Coordinator) 🔨 Issue #59

**Location:** `pkg/coordinator/engine/executor.go` (to be implemented)

**What it retries:**
- Agent crashes/failures during task execution
- Agent heartbeat timeout (agent unresponsive)
- Task validation failures (red-flag checks fail)
- Business logic errors (agent returns error result)

**Retry policy:**
- Max 3 attempts per task (Issue #59 spec)
- Exponential backoff (2^attempt seconds)
- Reassign to **different agent** on retry (load balance, avoid bad agents)

**Idempotency:**
- Check `task_executions` table before retry
- Skip already-completed tasks
- Store attempt count in SQLite

---

### Design: Task-Level Retry in Coordinator

#### Schema Changes

Update `task_executions` table to track retry metadata:

```sql
-- Existing schema from MDAP_MILESTONE5_MAPPING.md
CREATE TABLE task_executions (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    agent_id TEXT,              -- Which agent executed this attempt
    status TEXT NOT NULL,
    result TEXT,
    error TEXT,
    attempts INTEGER DEFAULT 1, -- Already exists!
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES workflow_instances(id)
);

-- New: Track each retry attempt separately
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

#### Executor Changes

```go
// pkg/coordinator/engine/executor.go

type RetryPolicy struct {
    MaxAttempts      int           `env:"RETRY_MAX_ATTEMPTS" default:"3"`
    InitialBackoff   time.Duration `env:"RETRY_INITIAL_BACKOFF" default:"2s"`
    MaxBackoff       time.Duration `env:"RETRY_MAX_BACKOFF" default:"30s"`
    BackoffMultiplier float64      `env:"RETRY_BACKOFF_MULTIPLIER" default:"2.0"`
    ReassignOnRetry  bool          `env:"RETRY_REASSIGN_AGENT" default:"true"`
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
            // Continue anyway - don't fail task due to audit failure
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

            // Record success event
            e.store.RecordEvent(ctx, &storage.WorkflowEvent{
                InstanceID:    execCtx.InstanceID,
                EventType:     "task.completed",
                CorrelationID: generateCorrelationID(),
                Data: map[string]any{
                    "task_id":  task.ID,
                    "agent_id": agentID,
                    "attempts": attempt,
                },
            })

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

        // Log failure with reason
        log.Warn("task attempt failed",
            "task_id", task.ID,
            "agent_id", agentID,
            "attempt", attempt,
            "error", err,
            "will_retry", attempt < e.config.Retry.MaxAttempts)

        // Classify error
        if isPermanentError(err) {
            // Don't retry permanent errors (validation failures, malformed input, etc.)
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

    // Record final failure event
    e.store.RecordEvent(ctx, &storage.WorkflowEvent{
        InstanceID:    execCtx.InstanceID,
        EventType:     "task.failed",
        CorrelationID: generateCorrelationID(),
        Data: map[string]any{
            "task_id": task.ID,
            "error":   lastErr.Error(),
            "attempts": taskExec.Attempts,
        },
    })

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

func (e *Executor) selectAgent(ctx context.Context, task *parser.Task, taskExec *TaskExecution, attempt int) (string, error) {
    // If reassignOnRetry is false, use same agent
    if !e.config.Retry.ReassignOnRetry && attempt > 1 && taskExec.AgentID != "" {
        return taskExec.AgentID, nil
    }

    // Otherwise, select available agent from NATS queue
    // This leverages NATS queue groups for automatic load balancing
    // Agents subscribe to: "agents.<task.Type>" with queue group
    // NATS will route to a random available subscriber

    // For retry, we want to AVOID the agent that failed (if possible)
    // We can use NATS request/reply with metadata to exclude agents

    // Simple approach: Let NATS queue group handle it (random agent)
    // Advanced approach: Maintain agent health tracking and blacklist failures

    return "", nil // No explicit agent selection - let NATS queue groups handle it
}

func (e *Executor) executeTaskAttempt(ctx context.Context, task *parser.Task, agentID string, attempt int) (any, error) {
    // Same implementation as before, but with timeout and heartbeat checks
    taskMsg := &TaskMessage{
        TaskID:     task.ID,
        Type:       task.Type,
        Params:     task.Params,
        Attempt:    attempt,
        InstanceID: execCtx.InstanceID,
    }

    // Determine timeout
    timeout := task.Timeout
    if timeout == 0 {
        timeout = e.config.DefaultTimeout
    }

    // Create cancellable context with timeout
    taskCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    // Publish task to NATS queue (agents.<task.Type>)
    subject := fmt.Sprintf("agents.%s", task.Type)

    // Subscribe to result (correlation ID pattern)
    resultSubject := fmt.Sprintf("agents.result.%s", taskMsg.TaskID)
    resultChan := make(chan *TaskResult, 1)
    errChan := make(chan error, 1)

    sub, err := e.natsClient.Subscribe(taskCtx, resultSubject, func(ctx context.Context, msg *nats.Message) error {
        var result TaskResult
        if err := json.Unmarshal(msg.Data, &result); err != nil {
            errChan <- err
            return nil
        }
        resultChan <- &result
        return nil
    })
    if err != nil {
        return nil, fmt.Errorf("failed to subscribe to result: %w", err)
    }
    defer sub.Unsubscribe(context.Background())

    // Publish task
    taskData, err := json.Marshal(taskMsg)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal task: %w", err)
    }

    if err := e.natsClient.Publish(taskCtx, subject, taskData); err != nil {
        return nil, fmt.Errorf("failed to publish task: %w", err)
    }

    // Start heartbeat monitoring (optional enhancement)
    go e.monitorHeartbeat(taskCtx, task.ID, cancel)

    // Wait for result or timeout
    select {
    case result := <-resultChan:
        // Success! Validate result
        if err := e.validateTaskResult(result); err != nil {
            return nil, fmt.Errorf("task result validation failed: %w", err)
        }
        return result.Data, nil

    case err := <-errChan:
        return nil, fmt.Errorf("task result unmarshaling failed: %w", err)

    case <-taskCtx.Done():
        if taskCtx.Err() == context.DeadlineExceeded {
            return nil, fmt.Errorf("task timeout after %v", timeout)
        }
        return nil, taskCtx.Err()
    }
}

func (e *Executor) monitorHeartbeat(ctx context.Context, taskID string, cancelFunc context.CancelFunc) {
    // Subscribe to agent heartbeats
    heartbeatSubject := fmt.Sprintf("agents.heartbeat.%s", taskID)

    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    lastHeartbeat := time.Now()

    for {
        select {
        case <-ctx.Done():
            return

        case <-ticker.C:
            // Check if heartbeat is stale
            if time.Since(lastHeartbeat) > 30*time.Second {
                log.Error("agent heartbeat timeout, cancelling task",
                    "task_id", taskID)
                cancelFunc()
                return
            }
        }
    }
}

func (e *Executor) validateTaskResult(result *TaskResult) error {
    // Red-flag checks (MDAP Section 3.3)
    if result.Data == nil {
        return fmt.Errorf("task result data is nil")
    }

    // Too long check
    resultStr := fmt.Sprintf("%v", result.Data)
    if len(resultStr) > 750 {
        return fmt.Errorf("task result too long: %d chars (red flag)", len(resultStr))
    }

    // Malformed check
    if result.Error != "" {
        return fmt.Errorf("task reported error: %s", result.Error)
    }

    return nil
}

func isPermanentError(err error) bool {
    // Classify errors as permanent (don't retry) or transient (retry)
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

#### Key Features

1. **Idempotency:** Check `task_executions` before execution, skip if already completed
2. **Retry Tracking:** Store each attempt in `task_attempts` table for full audit trail
3. **Exponential Backoff:** 2s → 4s → 8s (configurable)
4. **Agent Reassignment:** Use NATS queue groups to route retry to different agent
5. **Heartbeat Monitoring:** Detect agent crashes mid-execution
6. **Red-flag Validation:** Validate task results before marking complete
7. **Permanent vs. Transient Errors:** Don't retry validation failures

---

## Part 2: Approval Gate Integration (#53)

### The Challenge

**Approval gates** are a special task type that **blocks workflow execution** until a human approves or rejects.

**Requirements from Issue #53:**
- Workflow blocks until approved/rejected
- Timeout after 5 minutes (default reject)
- Approval API endpoint: `POST /workflows/{id}/approve`
- NATS request/reply for approval requests

### Design: Approval as Special Task Type

Approval gates are **just another task type** in the workflow, but with special handling:

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

#### Approval Gate Agent

The approval gate is handled by a special **built-in agent** that doesn't execute business logic, but waits for human input:

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

            // Publish approval granted event
            a.natsClient.Publish(ctx, "approvals.granted", &events.ApprovalGranted{
                ApprovalID:    approvalReq.ID,
                TaskID:        task.ID,
                ApprovedBy:    result.ApprovedBy,
                CorrelationID: generateCorrelationID(),
            })

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

            // Publish approval rejected event
            a.natsClient.Publish(ctx, "approvals.rejected", &events.ApprovalRejected{
                ApprovalID:    approvalReq.ID,
                TaskID:        task.ID,
                RejectedBy:    result.ApprovedBy,
                CorrelationID: generateCorrelationID(),
            })

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

        // Publish timeout event
        a.natsClient.Publish(ctx, "approvals.timeout", &events.ApprovalTimeout{
            ApprovalID:    approvalReq.ID,
            TaskID:        task.ID,
            CorrelationID: generateCorrelationID(),
        })

        return nil, fmt.Errorf("approval timeout after %v", timeout)

    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

type ApprovalRequest struct {
    ID          string
    TaskID      string
    InstanceID  string
    Title       string
    Description string
    Approvers   []string
    Status      ApprovalStatus
    ApprovedBy  string
    Comment     string
    CreatedAt   time.Time
    ExpiresAt   time.Time
    CompletedAt time.Time
}

type ApprovalStatus string

const (
    ApprovalStatusPending  ApprovalStatus = "pending"
    ApprovalStatusApproved ApprovalStatus = "approved"
    ApprovalStatusRejected ApprovalStatus = "rejected"
)

type ApprovalResult struct {
    ApprovalID string `json:"approval_id"`
    Status     string `json:"status"` // "approved" or "rejected"
    ApprovedBy string `json:"approved_by"`
    Comment    string `json:"comment"`
}
```

#### Approval API Endpoint

```go
// pkg/coordinator/api/approvals.go
package api

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

    // Validate approver (check if user is in approvers list)
    userID := r.Context().Value("user_id").(string) // From auth middleware
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

// POST /v1/approvals/:id/reject
func (a *API) RejectRequest(w http.ResponseWriter, r *http.Request) {
    // Same as ApproveRequest, but with decision="rejected"
}

// GET /v1/approvals - List pending approvals
func (a *API) ListPendingApprovals(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("user_id").(string)

    // Get all pending approvals for this user
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

type ApproveRequest struct {
    Decision string `json:"decision"` // "approved" or "rejected"
    Comment  string `json:"comment"`
}
```

#### Schema

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

## Idempotency Throughout the System

You asked: "We should strive that everything be idempotent and have support to enable idempotency wherever possible."

Here's how **every layer** achieves idempotency:

### Layer 1: NATS Client (Transport)

**Mechanism:** Request correlation IDs

```go
// pkg/nats/client.go:593-606
func (c *client) addCorrelationHeaders(ctx context.Context, msg *nats.Msg, opts *pubOptions) {
    correlationID := opts.correlationID
    if correlationID == "" {
        correlationID = c.config.GenerateID()
    }
    msg.Header.Set(c.config.CorrelationHeader, correlationID)
}
```

**Idempotency:** Same correlation ID = same request, even if retried

### Layer 2: Task Execution

**Mechanism:** Check `task_executions` table before execution

```go
// Idempotency check in executeTask()
taskExec, err := e.store.GetTaskExecution(ctx, execCtx.InstanceID, task.ID)
if taskExec != nil && taskExec.Status == TaskStateCompleted {
    log.Info("task already completed, skipping")
    return taskExec.Result, nil  // Return cached result
}
```

**Idempotency:** Tasks never re-execute if already completed

### Layer 3: Workflow Execution

**Mechanism:** Execution context serialization

```go
// After each task completes
if err := e.serializeContext(ctx, execCtx); err != nil {
    return nil, fmt.Errorf("failed to serialize context: %w", err)
}
```

**Idempotency:** Coordinator crash → restart → resume from last completed task

### Layer 4: Approval Gates

**Mechanism:** Approval request status check

```go
if approval.Status != ApprovalStatusPending {
    writeError(w, http.StatusConflict, fmt.Sprintf("approval already %s", approval.Status))
    return
}
```

**Idempotency:** Can't approve/reject twice

### Layer 5: API Operations

**Mechanism:** Replay protection (Issue #129)

```go
// StartWorkflowWithReplayProtection
existingInstance, err := o.store.GetActiveInstance(ctx, defID)
if existingInstance != nil && !forceRestart {
    return existingInstance.ID, nil  // Return existing instance ID
}
```

**Idempotency:** Starting same workflow twice returns existing instance

### Layer 6: Agent Task Handlers

**Mechanism:** Agents should check task status before starting work

```go
// Example: Agent receiving task
func (a *MyAgent) HandleTask(ctx context.Context, msg *nats.Message) error {
    var task TaskMessage
    json.Unmarshal(msg.Data, &task)

    // Query coordinator: Is this task still PENDING?
    status, err := a.client.GetTaskStatus(ctx, task.TaskID)
    if err != nil {
        return err
    }

    if status != "PENDING" {
        log.Info("task no longer pending, skipping", "task_id", task.TaskID, "status", status)
        return nil  // Idempotent: don't re-execute completed tasks
    }

    // Proceed with execution...
}
```

**Idempotency:** Agents check coordinator before starting work

---

## Summary & Recommendations

### Retry Logic (#59)

**Where it belongs:** Coordinator executor (`pkg/coordinator/engine/executor.go`)

**Key features:**
- 3 attempts max
- Exponential backoff (2s → 4s → 8s)
- Reassign to different agent on retry (via NATS queue groups)
- Store each attempt in `task_attempts` table
- Detect permanent vs. transient errors
- Heartbeat monitoring (optional enhancement)

**Idempotency:**
- Check `task_executions.status` before retry
- Skip completed tasks
- Store attempt count

### Approval Gates (#53)

**Where it belongs:** Built-in approval agent + API endpoints

**Key features:**
- Special task type: `approval-gate`
- Blocks workflow execution until human responds
- Timeout with auto-reject (5 minutes default)
- API: `POST /v1/approvals/:id/approve`
- NATS events for PWA/UI to consume

**Idempotency:**
- Check `approval_requests.status` before processing
- Can't approve/reject twice
- Status transitions: `pending` → `approved` | `rejected`

### Idempotency Everywhere

| Layer | Mechanism | Guarantees |
|-------|-----------|------------|
| NATS Client | Correlation IDs | Same request ID = idempotent retry |
| Task Execution | SQLite status check | Completed tasks never re-execute |
| Workflow Execution | Context serialization | Crash recovery from checkpoint |
| Approval Gates | Status check | Can't approve twice |
| API Operations | Replay protection | Starting same workflow returns existing |
| Agent Handlers | Query coordinator | Agents check status before work |

---

## Open Questions

1. **Agent Health Tracking:** Should we maintain a "blacklist" of failing agents to avoid reassigning to known-bad agents?

2. **Manual Retry API:** Issue #59 mentions "manual retry trigger via API". Should we expose `POST /v1/workflows/:id/tasks/:task_id/retry`?

3. **Approval Escalation:** If primary approver doesn't respond, should we escalate to secondary approvers?

4. **Retry Circuit Breaker:** Should we stop retrying a task type if >50% failure rate across all workflows?

5. **Agent Heartbeat Protocol:** Should agents send periodic heartbeats during long-running tasks? What's the schema?

---

## Next Steps

1. Implement retry logic in executor (Issue #59)
2. Implement approval gate agent (Issue #53)
3. Add approval API endpoints (Issue #53)
4. Create `task_attempts` and `approval_requests` tables
5. Write integration tests for retry scenarios
6. Write integration tests for approval timeout
7. Document retry configuration (env vars)
8. Document approval gate YAML syntax
