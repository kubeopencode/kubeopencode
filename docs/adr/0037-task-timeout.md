# ADR 0037: Task Timeout

## Status

Accepted

## Date

2026-05-06

## Context

Without timeouts, a stuck AI agent consumes resources indefinitely. This is identified as the highest-priority reliability gap in ADR 0020 (Enterprise Readiness Roadmap, Section 5.1).

Currently, the only way to stop a running Task is manual intervention via `kubectl annotate task <name> kubeopencode.io/stop=true`. There is no automatic enforcement of execution time limits.

### Why Task-level only?

ADR 0020 proposed a three-level resolution: Task > Agent > KubeOpenCodeConfig. After analysis, we choose to implement timeout **only at the Task level**:

1. **Agent is infrastructure, Task is the work unit.** Timeout is a constraint on a specific execution, not on the Agent itself. Agent already has `standby.idleTimeout` for its own lifecycle.
2. **Different Tasks have vastly different reasonable durations.** A "fix a typo" Task might need 5 minutes; a "refactor the entire module" Task might need 2 hours. A single Agent-level default is either too loose (no protection) or too strict (kills legitimate work).
3. **The user creating the Task knows best** how long it should take. This is the same principle as Kubernetes Job's `activeDeadlineSeconds` being on the Job, not on the controller.
4. **CronTask gets it for free.** CronTask's `taskTemplate.spec` embeds `TaskSpec`, so timeout flows through naturally — each scheduled Task can have its own timeout.
5. **Simplicity.** One field, one place, no resolution hierarchy to document or debug.

If a cluster-wide safety net is needed later, it can be added to `KubeOpenCodeConfig` as a separate follow-up without breaking this design.

### What counts as "execution time"?

**Only Running phase time counts.** The timeout clock starts at `status.startTime` (when the Task enters Running phase and a Pod is created), not at creation time. Queue time (Pending/Queued phases) is excluded because:

- Queue time is not under the Task's control — it depends on Agent capacity, standby resume, etc.
- A Task with `timeout: 30m` should get 30 minutes of actual execution, regardless of how long it waited in queue.

## Decision

Add a `timeout` field to `TaskSpec`. When a running Task exceeds its timeout, the controller stops it by reusing the existing stop mechanism (`kubeopencode.io/stop` annotation + `handleStop()`).

### API Change

```go
type TaskSpec struct {
    // ... existing fields ...

    // Timeout specifies the maximum duration for task execution.
    // The timeout clock starts when the Task enters the Running phase (status.startTime),
    // not when the Task is created. Queue time (Pending/Queued phases) is excluded.
    //
    // If the Task is still running after this duration, the controller stops it
    // by setting the kubeopencode.io/stop annotation, which triggers Pod deletion
    // via SIGTERM. The Task transitions to Completed with condition Stopped,
    // reason "Timeout".
    //
    // If not set, the Task runs indefinitely (no timeout).
    //
    // Example: "30m", "1h", "2h30m"
    // +optional
    Timeout *metav1.Duration `json:"timeout,omitempty"`
}
```

### Controller Behavior

In the reconcile loop, before checking Pod status for Running Tasks:

```go
// Check timeout before checking stop annotation and pod status
if task.Status.Phase == TaskPhaseRunning && task.Status.StartTime != nil {
    if task.Spec.Timeout != nil {
        elapsed := time.Since(task.Status.StartTime.Time)
        if elapsed >= task.Spec.Timeout.Duration {
            return r.handleTimeout(ctx, task)
        }
        // Requeue at the exact timeout expiry
        remaining := task.Spec.Timeout.Duration - elapsed
        return ctrl.Result{RequeueAfter: remaining}, nil
    }
}
```

`handleTimeout()` reuses the same logic as `handleStop()` but with `Reason: "Timeout"` instead of `Reason: "UserStopped"`:

```go
func (r *TaskReconciler) handleTimeout(ctx context.Context, task *Task) (ctrl.Result, error) {
    // Delete Pod (same as handleStop)
    // Set Phase = Completed
    // Set Condition: Stopped=True, Reason=Timeout, Message="Task timed out after <duration>"
    // Emit Event: reason=Timeout
}
```

### User Experience

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: fix-bug
spec:
  agentRef:
    name: my-agent
  description: "Fix the null pointer exception in auth module"
  timeout: 30m
```

After timeout:
```
$ kubectl get task fix-bug
NAME      PHASE       AGENT      AGE
fix-bug   Completed   my-agent   35m

$ kubectl describe task fix-bug
Conditions:
  Type:    Stopped
  Status:  True
  Reason:  Timeout
  Message: Task timed out after 30m0s
```

### CronTask Example

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: CronTask
metadata:
  name: daily-review
spec:
  schedule: "0 9 * * *"
  taskTemplate:
    spec:
      agentRef:
        name: code-reviewer
      description: "Review yesterday's PRs"
      timeout: 1h
```

## Consequences

### Positive

1. **Prevents resource leaks** — stuck Tasks are automatically cleaned up.
2. **Simple mental model** — one field on Task, starts counting from Running phase.
3. **Backward compatible** — no timeout by default, existing Tasks unaffected.
4. **Reuses existing stop mechanism** — no new Pod deletion logic needed.
5. **CronTask support is automatic** — `TaskSpec` is embedded in `TaskTemplateSpec`.

### Negative

1. **No cluster-wide safety net** — an administrator cannot enforce a global maximum timeout. This can be added later to `KubeOpenCodeConfig` if needed.
2. **No Agent-level default** — users must set timeout on each Task individually. This is intentional (see Context section) but could be revisited.

### Future Extensions

- `KubeOpenCodeConfig.spec.taskTimeout` as a cluster-wide default/maximum
- `KubeOpenCodeNamespaceConfig` for per-namespace defaults (ADR 0020, Section 4.1)
