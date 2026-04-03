# ADR 0028: Connection-Aware Standby — Heartbeat-based Idle Detection

## Status

Accepted

## Date

2026-04-03

## Context

ADR 0024 introduced the standby feature with a single `spec.suspend` switch and automatic lifecycle management. The controller auto-suspends an Agent after `idleTimeout` when no active Tasks exist, and auto-resumes when a new Task arrives.

However, the idle detection only considers Task count. Users can connect to an Agent via:

1. **Web terminal** — WebSocket connection through `kubeopencode-server`
2. **CLI attach** — `kubeoc agent attach` via service proxy or port-forward

If a user is interactively working with an Agent (browsing sessions, editing code) but no Task is actively running, the Agent is considered idle and gets suspended after `idleTimeout`, disconnecting the user mid-session.

### Why Not Query OpenCode Directly?

OpenCode (the AI coding tool inside the Agent pod) does not expose any metrics or API endpoints for active connections. Its `/session/status` endpoint only tracks whether sessions are processing AI requests (busy/retry), not whether users have attached TUI clients.

## Decision

### Short-term: Annotation Heartbeat (this ADR)

Use annotation `kubeopencode.io/last-connection-active` as a heartbeat signal from connection endpoints to the controller.

**Writers** (heartbeat producers):
- **Web terminal handler** (`agent_terminal_handler.go`) — updates the annotation every 60 seconds while a WebSocket terminal session is active
- **CLI attach** (`kubeoc agent attach`) — updates the annotation every 60 seconds while the opencode attach subprocess is running

**Reader** (heartbeat consumer):
- **Agent controller** (`reconcileStandby()`) — checks the annotation timestamp before advancing the idle timer. If the annotation is within the staleness threshold, the Agent is considered "connected" and idle timer is deferred.

### Constants

| Constant | Value | Purpose |
|---|---|---|
| `ConnectionHeartbeatInterval` | 60s | How often clients update the annotation |
| `ConnectionHeartbeatStaleness` | 2min | How long a heartbeat is considered fresh |

### Constraint Validation

If `idleTimeout < ConnectionHeartbeatStaleness` (2 minutes), the controller degrades the staleness threshold to `idleTimeout / 2` and sets a `StandbyConfigWarning` condition on the Agent. This ensures the feature works correctly even with very short idle timeouts.

### Annotation Format

```yaml
metadata:
  annotations:
    kubeopencode.io/last-connection-active: "2026-04-03T10:30:00Z"
```

Value is an RFC3339 timestamp in UTC.

### Modified Idle Logic

```
reconcileStandby():
  if activeTasks > 0:
    clear idle timer, auto-resume if suspended
  elif hasActiveConnection (annotation within staleness):
    clear idle timer (defer suspension)
  else:
    start/advance idle timer → auto-suspend when expired
```

### Long-term: OpenCode `/connections` Endpoint (future)

Propose to the OpenCode project a new endpoint:

```
GET /connections

Response:
{
  "active": 2,
  "connections": [
    {
      "type": "attach",
      "connectedAt": "2026-04-03T10:00:00Z",
      "lastActiveAt": "2026-04-03T10:30:00Z"
    }
  ]
}
```

This would allow the controller to directly query the Agent's OpenCode server for connection state, eliminating the need for annotation heartbeats. The annotation mechanism can be retained as a fallback.

## Consequences

### Positive

- Users' interactive sessions are not interrupted by standby auto-suspend
- No CRD changes required (annotation-based)
- Backward compatible: no annotation = original behavior unchanged
- Works with all connection types that go through KubeOpenCode's own endpoints

### Negative

- Each active connection generates one etcd write per minute (acceptable at typical scale of 1-3 concurrent connections per Agent)
- 2-minute detection delay after connection closes before idle timer starts
- CLI heartbeat requires users to have `patch` permission on Agents (added to web-user ClusterRole)
- Direct connections that bypass KubeOpenCode (e.g., raw `kubectl port-forward` + `opencode attach`) are not detected

### Does Not Cover

- Connections made directly to the Agent pod (bypassing kubeopencode-server and kubeoc CLI)
- OpenCode's internal session activity (addressed by long-term `/connections` endpoint proposal)

## Supersedes

None (extends ADR 0024)
