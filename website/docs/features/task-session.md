---
sidebar_position: 14
title: Task Session Integration
description: OpenCode session information in Task status
---

# Task Session Integration

Tasks that reference an Agent (`agentRef`) expose OpenCode session information in their status, enabling tracking of token usage, cost, and session details.

## Session Info in Task Status

When a Task runs via an Agent (using `agentRef`), the controller populates the `status.session` field:

```bash
kubectl get task my-task -o jsonpath='{.status.session}'
```

```json
{
  "id": "abc123-session-id",
  "url": "http://my-agent.kubeopencode-system.svc.cluster.local:4096",
  "title": "Fix login timeout bug",
  "summary": {
    "messageCount": 15,
    "tokenUsage": 45000,
    "cost": 0.23,
    "filesChanged": 3,
    "additions": 120,
    "deletions": 45
  }
}
```

### Session Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | OpenCode session identifier |
| `url` | string | Full URL to the session (for `agentRef` Tasks) |
| `title` | string | Session title set by OpenCode |
| `summary.messageCount` | int64 | Total messages in the session |
| `summary.tokenUsage` | int64 | Total tokens consumed |
| `summary.cost` | float64 | Estimated cost in USD |
| `summary.filesChanged` | int64 | Number of files modified |
| `summary.additions` | int64 | Lines added |
| `summary.deletions` | int64 | Lines removed |

:::info
The `summary` field is populated after the Task completes. It is not available during execution.
:::

## Template Tasks

Tasks that reference an AgentTemplate (`templateRef`) create ephemeral standalone Pods. These Tasks do **not** have session info in their status — the `session` field is only populated for `agentRef` tasks.

## Querying Session Info

### Get session details

```bash
kubectl get task my-task -o jsonpath='{.status.session}' | jq
```

### Get token usage

```bash
kubectl get task my-task -o jsonpath='{.status.session.summary.tokenUsage}'
```

### Get cost

```bash
kubectl get task my-task -o jsonpath='{.status.session.summary.cost}'
```

### List tasks sorted by cost

```bash
kubectl get tasks -o custom-columns=NAME:.metadata.name,COST:.status.session.summary.cost
```

## Related

- [Task Timeout](task-timeout.md) — Limit task execution time
- [Task Stop](task-stop.md) — Stop running tasks via annotation
- [Live Agents](live-agents.md) — Persistent agents with session support
- [Architecture](../architecture.md) — Complete API reference