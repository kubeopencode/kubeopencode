---
sidebar_position: 13
title: Task Cleanup
description: Automatic cleanup of finished Tasks via KubeOpenCodeConfig
---

# Task Cleanup

KubeOpenCodeConfig provides cluster-wide Task cleanup policies to prevent finished Tasks from accumulating indefinitely.

## Configuration

Task cleanup is configured via the `KubeOpenCodeConfig` resource (cluster-scoped singleton named `cluster`):

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeConfig
metadata:
  name: cluster
spec:
  cleanup:
    ttlSecondsAfterFinished: 3600   # Delete finished Tasks after 1 hour
    maxRetainedTasks: 100            # Keep at most 100 per namespace
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `cleanup.ttlSecondsAfterFinished` | *int32 | nil (disabled) | TTL for finished Tasks. nil = disabled |
| `cleanup.maxRetainedTasks` | *int32 | nil (unlimited) | Max completed Tasks per namespace. nil = unlimited |

## Behavior

### TTL-based Cleanup

Tasks are deleted after `ttlSecondsAfterFinished` seconds from completion. The TTL clock starts when the Task enters a terminal phase (`Completed` or `Failed`).

### Retention-based Cleanup

Only the most recent `maxRetainedTasks` completed Tasks are retained per namespace. Older completed Tasks are deleted. This prevents namespace clutter without setting a time-based TTL.

### Combined Cleanup

Both policies can be used together:
1. TTL is checked first — Tasks older than `ttlSecondsAfterFinished` are deleted
2. Retention is checked next — if more than `maxRetainedTasks` completed Tasks remain, the oldest are deleted

### Cascading Deletion

Deleting a Task automatically deletes its associated Pod and ConfigMap (via OwnerReferences).

### Default Behavior

Cleanup is **disabled by default**. Without a `KubeOpenCodeConfig` resource (or without the `cleanup` section), finished Tasks persist indefinitely.

:::tip
For production clusters, set `maxRetainedTasks` to prevent namespace clutter. For development, you may want a shorter TTL (e.g., `ttlSecondsAfterFinished: 3600`).
:::

## CronTask Interaction

CronTask has its own `maxRetainedTasks` field that blocks creation of new Tasks when the limit is reached. This is separate from the global cleanup policy:

- **CronTask `maxRetainedTasks`**: Blocks creation, does NOT delete old Tasks
- **KubeOpenCodeConfig `cleanup.maxRetainedTasks`**: Deletes old completed Tasks

Both can be used together — CronTask prevents over-creation, while global cleanup removes old Tasks.