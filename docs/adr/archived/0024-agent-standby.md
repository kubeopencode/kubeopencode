# ADR 0024: Agent Standby — Unified Suspend/Resume Lifecycle

## Status

Accepted

## Date

2026-04-01

## Context

ADR 0023 introduced `spec.idleTimeout` for auto-suspend, but kept it separate from `spec.suspend`. The auto-suspend mechanism only modified `status.suspended` without changing `spec.suspend`, creating a split between spec and status. This caused a bug: the task controller only checked `spec.suspend`, so tasks targeting an auto-suspended agent would fail instead of being queued.

The fundamental issue was that Agent had two parallel suspend mechanisms (manual `spec.suspend` and auto `status.suspended`) with different semantics, making the state machine complex and error-prone.

## Decision

Replace `spec.idleTimeout` with `spec.standby`, a structured field that configures automatic lifecycle management. The key design change: **Agent has a single switch (`spec.suspend`) that both humans and the controller operate on.**

### API

```yaml
spec:
  suspend: false              # Single suspend switch (used by both humans and controller)
  standby:
    idleTimeout: "30m"        # Enable automatic lifecycle management

status:
  suspended: true             # Read-only mirror of spec.suspend
  idleSince: "2026-04-01T10:00:00Z"
```

### Behavior

| `spec.standby` | Behavior |
|---|---|
| nil | Manual-only: only humans control `spec.suspend` |
| `{idleTimeout: "30m"}` | Standby mode: controller manages `spec.suspend` automatically |

**In standby mode:**
- No active tasks + idle timeout expires → controller sets `spec.suspend = true`
- New task arrives on suspended agent → controller sets `spec.suspend = false`
- Human can still manually suspend (if no running tasks) or resume at any time

### Implementation

The agent controller patches `spec.suspend` directly (similar to how HPA patches `spec.replicas`). The `reconcileStandby()` function is idempotent — it only patches when the value actually needs to change, preventing infinite reconcile loops.

The task controller requires no special handling — it checks `cfg.suspend` (from `spec.suspend`), which is now always correct regardless of who set it.

## Consequences

### Positive

- Single source of truth (`spec.suspend`) eliminates spec/status inconsistency bugs
- Task controller works correctly without needing to know about standby
- Clearer mental model: Agent is a black box with one switch, multiple actors can flip it

### Negative

- Controller modifying spec is unconventional (but precedented by HPA)
- Must guard against reconcile loops (solved by conditional patching)

### Supersedes

- [ADR 0023: Agent Idle Timeout](archived/0023-agent-idle-timeout.md)
