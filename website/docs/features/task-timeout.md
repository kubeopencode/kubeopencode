# Task Timeout

Tasks support an optional `timeout` field that limits execution duration. When the timeout is exceeded, the controller automatically stops the Task.

## Usage

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

The `timeout` field accepts Go duration strings: `"30m"`, `"1h"`, `"2h30m"`, etc.

## Behavior

- **Clock starts at Running phase**: The timeout is measured from `status.startTime`, not creation time. Queue time (Pending/Queued phases) is excluded.
- **When timeout is exceeded**: The controller deletes the Pod (SIGTERM), and the Task transitions to `Completed` with condition `Stopped`, reason `Timeout`.
- **No timeout by default**: If `timeout` is not set, the Task runs indefinitely (backward compatible).

## Checking timeout status

```bash
kubectl describe task fix-bug
```

A timed-out Task shows:

```
Status:
  Phase: Completed
  Conditions:
    Type:    Stopped
    Status:  True
    Reason:  Timeout
    Message: Task timed out after 30m0s
```

## CronTask

The `timeout` field works naturally with CronTask since `TaskSpec` is embedded in the task template:

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
