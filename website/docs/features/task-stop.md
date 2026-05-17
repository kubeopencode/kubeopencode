---
sidebar_position: 12
title: Task Stop
description: Stop running tasks via annotation
---

# Task Stop

Stop a running Task using an annotation. This is the recommended way to terminate a Task — prefer this over `kubectl delete task`.

## Stopping a Task

```bash
kubectl annotate task my-task kubeopencode.io/stop=true
```

When this annotation is detected:
- The controller deletes the Pod (with graceful termination period)
- Task status is set to `Phase=Completed` with a `Stopped` condition
- The `Stopped` condition has reason `UserStopped`

## Checking Stopped Task Status

```bash
# Check the task phase
kubectl get task my-task -o jsonpath='{.status.phase}'
# Output: Completed

# Check the stopped condition
kubectl get task my-task -o jsonpath='{.status.conditions[?(@.type=="Stopped")]}'
```

## Important Notes

:::warning Logs are lost when a Task is stopped
When a Task is stopped, the underlying Pod is deleted. Pod logs are no longer accessible via `kubectl logs`. For log persistence, use an external log aggregation system (Fluentd, Loki, etc.).
:::

:::tip Prefer `kubectl annotate task` over `kubectl delete task`
Using the stop annotation preserves the Task object for status inspection and auditing. Deleting the Task removes it entirely.
:::

## Related

- [Task Timeout](task-timeout.md) — Automatic timeout for long-running tasks
- [Task Cleanup](task-cleanup.md) — Automatic cleanup of finished Tasks