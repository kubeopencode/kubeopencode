# ADR 0013: Defer Token Usage Tracking to Post-v0.1

## Status

Accepted

## Context

KubeOpenCode needs per-Task token usage tracking and cost reporting for enterprise governance, billing, and resource management. When an AI agent executes a Task, the underlying OpenCode tool consumes tokens from LLM providers. Currently, KubeOpenCode's `TaskExecutionStatus` only records execution phase (`Pending/Running/Completed/Failed`), Pod name, and timestamps — no token consumption or cost data.

A research report (Manus AI, 2026-02-05) investigated OpenCode's token tracking capabilities. We verified the report's claims against the OpenCode source code at `../opencode/` and analyzed the feasibility of integrating token tracking into KubeOpenCode.

### Research Report Verification

| Claim | Verification | Source |
|-------|-------------|--------|
| TUI displays real-time token usage | **Confirmed** | `packages/opencode/src/cli/cmd/tui/routes/session/header.tsx`, `sidebar.tsx` |
| `opencode stats` command exists | **Confirmed** | `packages/opencode/src/cli/cmd/stats.ts` |
| SDK `AssistantMessage` contains tokens/cost | **Confirmed** | `packages/opencode/src/session/message-v2.ts:352-394` |
| `opencode export` outputs token data as JSON | **Confirmed** | `packages/opencode/src/cli/cmd/export.ts` |
| File path `session-context-usage.tsx` | **Does not exist** | Actual implementation is in `header.tsx` and `sidebar.tsx` |

The report's core conclusions are correct — OpenCode has comprehensive internal token tracking. However, the report missed a critical integration method (`opencode run --format json`) and contained one fabricated file path.

### OpenCode Token Data Structure

Verified from `packages/opencode/src/session/message-v2.ts:380-389`:

```typescript
// Per AssistantMessage
cost: z.number(),
tokens: z.object({
  input: z.number(),
  output: z.number(),
  reasoning: z.number(),
  cache: z.object({
    read: z.number(),
    write: z.number(),
  }),
}),
```

Per-step token data is also tracked in `StepFinishPart` (`message-v2.ts:204-221`).

### Data Acquisition Methods and Trade-offs

We evaluated four methods for extracting token data from OpenCode:

| Method | UX Impact | Machine-Readable | Pod Mode | Server Mode |
|--------|-----------|-------------------|----------|-------------|
| `opencode run --format json` | Breaks `kubectl logs` readability | Yes (JSON events) | Works | Works |
| `opencode stats --project ""` | None (runs after task) | **No** (ASCII table only) | Works | **Fails** (data on server) |
| `opencode export <sessionID>` | None (runs after task) | Yes (JSON) | Works (needs sessionID) | **Fails** (data on server) |
| Server HTTP API query | None | Yes | N/A | Works (needs sessionID) |

**Key findings:**

1. **`opencode run --format json`**: Outputs all events as JSON to stdout, including `step_finish` events with token data. However, this **replaces the human-readable output entirely**, making `kubectl logs` useless for observing agent execution — a critical capability for users debugging or monitoring Tasks.

2. **`opencode stats`**: Aggregates token statistics across all sessions in the current project. Since each Task Pod runs a single session, the output would represent the Task's total usage. However, `stats` **only supports ASCII table output** (no `--format json` option) and reads from **local filesystem storage** (`Storage.list(["project"])` in `stats.ts:92`), not via SDK/HTTP.

3. **Server mode data locality problem**: In Server mode (`opencode run --attach <url>`), session data is created on the **remote server Pod** via HTTP API (`createOpencodeClient({ baseUrl: args.attach })` in `run.ts:571`). The `stats` and `export` commands read from **local filesystem storage**, so running them on the client Pod yields no data. This means Pod mode and Server mode would require different token extraction strategies.

4. **`opencode export`**: Exports complete session JSON including all token data. Requires `sessionID`, which is not output to stdout in default (non-JSON) mode.

### Current TaskExecutionStatus (No Token Fields)

From `api/v1alpha1/task_types.go:164-203`:

```go
type TaskExecutionStatus struct {
    ObservedGeneration int64              `json:"observedGeneration,omitempty"`
    Phase              TaskPhase          `json:"phase,omitempty"`
    AgentRef           *AgentReference    `json:"agentRef,omitempty"`
    PodName            string             `json:"podName,omitempty"`
    PodNamespace       string             `json:"podNamespace,omitempty"`
    StartTime          *metav1.Time       `json:"startTime,omitempty"`
    CompletionTime     *metav1.Time       `json:"completionTime,omitempty"`
    Conditions         []metav1.Condition `json:"conditions,omitempty"`
}
```

No token usage, cost, or model usage fields exist.

## Decision

**Defer token usage tracking to post-v0.1.** Do not implement it in the current project phase.

### Reason 1: No Clean Programmatic Extraction Without UX Degradation

The only machine-readable output method (`--format json`) replaces human-readable logs, breaking the primary observability path for users (`kubectl logs`). The alternative (`opencode stats`) lacks JSON output support. There is no way to get both human-readable execution logs AND machine-readable token statistics from a single OpenCode run.

### Reason 2: Pod Mode vs Server Mode Incompatibility

Token data acquisition would require fundamentally different strategies for Pod mode (local filesystem) and Server mode (HTTP API), increasing implementation complexity and maintenance burden. A unified approach requires upstream OpenCode changes.

### Reason 3: Upstream Dependency

The cleanest implementation path depends on OpenCode adding:
- `opencode stats --format json` for machine-readable output, OR
- `opencode run` outputting a token summary line/file upon completion, OR
- A post-run hook that writes token data to a known file path

These features do not exist today and require either upstream contribution or waiting for OpenCode to evolve.

### Reason 4: Project Maturity

KubeOpenCode is at v0.1.0. Core Task workflow (context resolution, credential mounting, cross-namespace, Server mode) needs production validation before expanding the status API surface. Adding token tracking fields to `TaskExecutionStatus` creates a backward-compatibility commitment for a feature whose data source is not yet stable.

## Consequences

### Positive

1. **No premature API commitment**: Avoid publishing `tokenUsage` fields in `TaskExecutionStatus` that may need to change based on upstream OpenCode evolution
2. **Focused engineering**: Team can concentrate on stabilizing core Task/Agent workflows
3. **Clean future implementation**: When OpenCode adds JSON stats output, the integration path is straightforward

### Negative

1. **No per-Task cost visibility**: Users cannot see how many tokens a Task consumed without manually checking OpenCode logs
2. **No Prometheus metrics**: No `kubeopencode_task_tokens_total` or `kubeopencode_task_cost_total` metrics for monitoring dashboards
3. **Limited governance**: Enterprise users cannot enforce token budgets or generate billing reports per Task

### What We Preserve

The future implementation path is clear and well-defined:

1. **Upstream contribution**: Add `opencode stats --format json` (or `opencode usage --format json`) to OpenCode
2. **Modified default command**: Append `opencode stats --format json > ${WORKSPACE_DIR}/.kubeopencode/token-usage.json` after `opencode run`
3. **TaskStatus extension**: Add `tokenUsage` field to `TaskExecutionStatus`:
   ```go
   type TokenUsage struct {
       Input      int64  `json:"input"`
       Output     int64  `json:"output"`
       Reasoning  int64  `json:"reasoning,omitempty"`
       CacheRead  int64  `json:"cacheRead,omitempty"`
       CacheWrite int64  `json:"cacheWrite,omitempty"`
       Cost       string `json:"cost,omitempty"`
   }
   ```
4. **Controller reads file**: After Pod completion, controller reads token JSON and updates TaskStatus
5. **Prometheus metrics**: Register counters/histograms based on TaskStatus token data
6. **Server mode**: Query Server's HTTP API (`GET /session/:sessionID/message`) for token data

## Alternative: Direct Filesystem Reading

Investigation of OpenCode's storage implementation (`packages/opencode/src/storage/storage.ts`) reveals that all session and message data is persisted as **plain JSON files** on the local filesystem. This opens a potential shortcut that does not require any upstream OpenCode changes.

### OpenCode Internal Storage Layout

Storage root (from `storage.ts:145` and `global/index.ts:8`):
```
${XDG_DATA_HOME}/opencode/storage/
```

Default path: `~/.local/share/opencode/storage/`

```
storage/
├── project/
│   └── <projectID>.json
├── session/
│   └── <projectID>/
│       └── <sessionID>.json
├── message/
│   └── <sessionID>/
│       └── <messageID>.json          # Contains cost + tokens for assistant messages
└── part/
    └── <messageID>/
        └── <partID>.json             # step-finish parts contain per-step cost + tokens
```

Each assistant message JSON file (written by `Session.updateMessage()` at `session/index.ts:377`) contains:

```json
{
  "role": "assistant",
  "cost": 0.0123,
  "tokens": {
    "input": 1500,
    "output": 200,
    "reasoning": 0,
    "cache": { "read": 0, "write": 0 }
  }
}
```

### How It Could Work

After `opencode run` completes in the Task Pod, a post-run script could traverse the message JSON files, filter for `role: "assistant"`, and aggregate `cost` and `tokens` fields into a summary file. The controller would then read this summary after Pod completion.

**Pod mode**: Feasible — data is on the same Pod's filesystem.
**Server mode**: Not directly feasible — data lives on the server Pod, not the client Pod.

### Why We Do Not Adopt This Approach

While technically feasible for Pod mode, this approach relies on **OpenCode's internal storage implementation details**, not a public API:

1. **No stability guarantee**: The filesystem layout (directory structure, JSON schema, storage path) is an implementation detail of OpenCode. It is not documented as a public interface and may change across versions without notice.
2. **Maintenance burden**: Any OpenCode upgrade could silently break the parsing logic if the storage format changes. This creates a fragile coupling between KubeOpenCode and OpenCode internals.
3. **Incomplete coverage**: Does not work for Server mode without additional mechanisms (e.g., controller `exec` into server Pod), further increasing complexity.

We prefer to wait for (or contribute) a **stable, public interface** from OpenCode — such as `opencode stats --format json` or a dedicated token summary output — that provides the same data with an explicit compatibility contract.

This approach remains documented here as a **fallback option** that could be used if upstream progress stalls and the need for token tracking becomes urgent.

## Prerequisites for Future Implementation

1. **OpenCode `stats --format json`**: Either contribute upstream or wait for OpenCode to add machine-readable stats output (preferred path)
2. **Session ID accessibility**: A way to retrieve the sessionID from a completed `opencode run` without using `--format json`
3. **Server mode API stability**: OpenCode's HTTP API for session/message queries needs to be stable
4. **Production usage feedback**: Real-world usage patterns to inform the token tracking API design (e.g., whether to track per-step or per-Task, whether to include model breakdown)
5. **Fallback consideration**: If upstream progress stalls, direct filesystem reading (documented above) can serve as an interim solution for Pod mode, with the understanding that it requires version-specific validation

## References

- OpenCode token tracking research report (Manus AI, 2026-02-05)
- OpenCode stats command: `../opencode/packages/opencode/src/cli/cmd/stats.ts`
- OpenCode message types: `../opencode/packages/opencode/src/session/message-v2.ts`
- OpenCode run command: `../opencode/packages/opencode/src/cli/cmd/run.ts`
- OpenCode storage: `../opencode/packages/opencode/src/storage/storage.ts`
- KubeOpenCode Task types: `api/v1alpha1/task_types.go`
- KubeOpenCode Task controller: `internal/controller/task_controller.go`
- KubeOpenCode Pod builder: `internal/controller/pod_builder.go`
- ADR 0011: Agent Server Mode (`docs/adr/0011-agent-server-mode.md`)
