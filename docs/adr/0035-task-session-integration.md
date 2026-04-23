# ADR 0035: Exposing OpenCode Sessions in Task Status

## Status

Proposed

## Date

2026-04-22

## Context

When a Task runs against a live Agent (`agentRef`), the Task Pod executes `opencode run --attach <serverURL>`, which creates a **session** on the Agent's OpenCode server. This session contains the full conversation history: user prompts, AI responses, tool calls, file changes, token usage, and cost.

Currently, the Task CRD status has **no visibility** into the underlying session:

```go
type TaskExecutionStatus struct {
    Phase          TaskPhase
    PodName        string
    StartTime      *metav1.Time
    CompletionTime *metav1.Time
    Conditions     []metav1.Condition
    // ... no session information
}
```

This creates several problems:

1. **Debugging**: When a Task fails, operators must `kubectl exec` into the Agent Pod and use OpenCode's TUI `/session` command to find and inspect the relevant session. There is no way to correlate a Task to its session from Kubernetes.
2. **Observability**: Token usage, cost, and message count are invisible at the Kubernetes layer. Cluster admins cannot answer "how many tokens did this Task consume?" without accessing the Agent's internal database.
3. **UI integration**: The KubeOpenCode UI shows Task lifecycle (phase, duration) but cannot display conversation details. Users must open the web terminal and navigate OpenCode's TUI to browse session history.

### How OpenCode Sessions Work

OpenCode stores sessions in a SQLite database (`~/.local/share/opencode/opencode.db`). Each session has:

- **SessionID** (`ses_` prefix, e.g., `ses_ff34a1b2c3d4ABCDefghijklmn`)
- **Title, slug, timestamps** (created, updated, archived)
- **Messages** (user/assistant turns with role, tokens, cost, model info)
- **Parts** (text, tool calls, file diffs, reasoning, snapshots)
- **Summary** (additions, deletions, files changed)

OpenCode exposes a full HTTP API for sessions:

| Endpoint | Description |
|----------|-------------|
| `GET /session` | List sessions |
| `GET /session/:id` | Get session details |
| `GET /session/:id/message` | Get messages (paginated) |
| `POST /session` | Create session |
| `DELETE /session/:id` | Delete session |

The `opencode run --attach` command creates a session via `POST /session`, sends the prompt via `POST /session/:id/message`, waits for completion via SSE (`GET /event`), then exits. With `--format json`, every emitted event includes the `sessionID` field.

### Key Insight

The session ID is already available during Task execution — it is created by the `opencode run --attach` process inside the Task Pod. The challenge is **surfacing it** to the Kubernetes layer.

## Decision

### Phase 1: Capture Session ID in Task Status

Add session information to `TaskExecutionStatus`:

```go
type SessionInfo struct {
    // ID is the OpenCode session ID (e.g., "ses_ff34a1b2...")
    // +optional
    ID string `json:"id,omitempty"`

    // Title is the session title used when creating the session.
    // Format: "kubeopencode/<namespace>/<task-name>"
    // +optional
    Title string `json:"title,omitempty"`
}

type TaskExecutionStatus struct {
    // ... existing fields ...

    // Session contains information about the OpenCode session created for this Task.
    // Only set for agentRef Tasks.
    // +optional
    Session *SessionInfo `json:"session,omitempty"`
}
```

**How to capture the session ID:**

Use `--format json` in the `opencode run --attach` command. The Task Pod's command becomes:

```bash
/tools/opencode run --attach <serverURL> --format json --title "Task: <name>" "$(cat /workspace/task.md)" \
  | tee /dev/stderr \
  | grep -m1 '"sessionID"' \
  | jq -r '.sessionID' > /tmp/session-id
```

However, this approach requires the Task controller to read the session ID from the Pod. Alternative approaches:

**Option A: Pod annotation (recommended)**. Modify the `opencode run --attach` command to write the session ID to a well-known annotation on itself:

The Task Pod command wraps `opencode run --format json` and extracts the session ID from the first JSON output line, then patches the Pod annotation. The Task controller watches Pod annotation changes and copies the session ID to `task.status.session`.

This requires the Task Pod's ServiceAccount to have `patch` permission on its own Pod, which adds RBAC complexity.

**Option B: Controller polls Agent API**. The Task controller, during reconciliation of a Running Task, queries the Agent's OpenCode API (`GET /session?search=<task-name>`) to find the matching session by title (since we set `--title "Task: <name>"`).

This is simpler (no Pod RBAC changes) but has timing issues (session may not exist yet) and depends on title-based matching which is fragile.

**Option C: Sidecar reporter**. Add a lightweight sidecar container to the Task Pod that monitors the main container's stdout (JSON format), extracts the session ID, and reports it back via a Kubernetes API call (annotation or status).

This adds Pod complexity but cleanly separates concerns.

**Option D: OpenCode plugin**. Create an OpenCode plugin that fires on session creation and reports the session ID back to the KubeOpenCode controller via an API call or Pod annotation.

This is the most elegant but adds a dependency on the plugin system.

**Recommended: Option B (Controller polls Agent API)** for initial implementation due to simplicity. The controller already knows the Agent's server URL and can query sessions. Title-based matching can be made reliable by using a deterministic title format (e.g., `"kubeopencode/<namespace>/<task-name>"`) and adding a `--title` flag that is always set.

### Phase 2: Session Proxy API in KubeOpenCode Server

Add proxy endpoints to the KubeOpenCode API server that forward requests to the Agent's OpenCode session API:

```
GET /api/v1/namespaces/{ns}/tasks/{name}/session           → Agent's GET /session/{id}
GET /api/v1/namespaces/{ns}/tasks/{name}/session/messages   → Agent's GET /session/{id}/message
```

This enables:
- KubeOpenCode UI to display conversation history without requiring direct Agent access
- Users to browse Task conversations through the KubeOpenCode dashboard
- Read-only debugging without opening a web terminal

### Phase 3: Session Summary in Task Status (Future)

Once session ID is reliably captured, the controller can periodically (or on Task completion) fetch session summary data and store it in status:

```go
type SessionInfo struct {
    ID  string `json:"id,omitempty"`
    URL string `json:"url,omitempty"`

    // Summary is populated when the Task completes.
    // +optional
    Summary *SessionSummary `json:"summary,omitempty"`
}

type SessionSummary struct {
    // MessageCount is the total number of messages in the session.
    // +optional
    MessageCount int32 `json:"messageCount,omitempty"`

    // TokenUsage is the total token consumption.
    // +optional
    TokenUsage *TokenUsage `json:"tokenUsage,omitempty"`

    // Cost is the total cost in USD.
    // +optional
    Cost string `json:"cost,omitempty"`

    // FilesChanged is the number of files modified.
    // +optional
    FilesChanged int32 `json:"filesChanged,omitempty"`

    // Additions is the total lines added.
    // +optional
    Additions int32 `json:"additions,omitempty"`

    // Deletions is the total lines deleted.
    // +optional
    Deletions int32 `json:"deletions,omitempty"`
}

type TokenUsage struct {
    Input    int64 `json:"input,omitempty"`
    Output   int64 `json:"output,omitempty"`
    Reasoning int64 `json:"reasoning,omitempty"`
    Cache    int64  `json:"cache,omitempty"`
}
```

## Consequences

### Positive

- Tasks become self-describing: `kubectl get task <name> -o jsonpath='{.status.session.id}'` returns the session ID
- KubeOpenCode UI can display conversation history via the proxy API without embedding OpenCode's TUI
- Cluster admins gain visibility into token usage and cost at the Kubernetes layer
- Debugging workflow simplified: Task → Session ID → conversation details, all through `kubectl` or UI
- No new CRD required — session data is either in Task status (summary) or proxied from Agent (details)

### Negative

- Controller-to-Agent API dependency: Task controller must be able to reach the Agent's OpenCode HTTP API (already true for server readiness checks)
- Title-based session matching (Option B) requires a strict naming convention; must handle edge cases (duplicate titles, session not yet created)
- Session data is ephemeral: if the Agent Pod is deleted (no persistence), session data is lost. Task status summary would be the only surviving record
- Proxy API adds a new responsibility to the KubeOpenCode server

### Risks

- **OpenCode API stability**: The session API is part of OpenCode's internal server, not a public API contract. Breaking changes could affect the integration.
- **Performance**: Proxying large conversation histories (hundreds of messages) through the KubeOpenCode server could be slow. Pagination is essential.
- **templateRef Tasks**: Template-based Tasks create ephemeral Pods with standalone OpenCode instances. Session data is lost when the Pod terminates unless we add persistence or extract session summary before Pod deletion.

## Alternatives Considered

### Alternative 1: Session as a Separate CRD

Create a `Session` CRD that mirrors OpenCode's session data in Kubernetes:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Session
metadata:
  name: ses-ff34a1b2c3d4
  labels:
    kubeopencode.io/task: my-task
spec: {}
status:
  messages: [...]
  tokenUsage: ...
```

**Rejected** because:
- Duplicates data already stored in OpenCode's SQLite database
- Message-level data in CRD status would hit etcd size limits (1.5MB per object)
- Adds CRD management complexity with no clear benefit over the proxy approach

### Alternative 2: Rename Task to Session

Replace the Task CRD with a Session CRD to align terminology with OpenCode.

**Rejected** because:
- Task and Session are different abstractions at different layers (see Context section)
- "Session" lacks the lifecycle semantics needed for Kubernetes orchestration (scheduling, queuing, quota)
- `CronSession` is semantically awkward
- Non-interactive Tasks (CronTask-triggered, API-triggered) have no interactive "session" from the user's perspective

### Alternative 3: Custom GUI Replacing OpenCode TUI

Build a fully custom chat GUI in KubeOpenCode UI, bypassing OpenCode's TUI entirely.

**Deferred** — requires significantly more effort and may involve switching to a lighter agent base. Will be reconsidered if user feedback shows the TUI-based approach is insufficient. See future ADR if needed.
