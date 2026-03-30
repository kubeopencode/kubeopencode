# ADR 0016: Human-in-the-Loop (HITL) Design for KubeOpenCode

> **Status: Partially Implemented.** A simplified model was adopted: Tasks are always
> non-interactive (all permissions auto-allowed), and HITL is handled via Web Terminal
> (ADR 0018) or `opencode attach`. The Phase 2 HITL REST API and UI (`hitl_handler.go`,
> `HITLPanel.tsx`) were implemented then removed in favor of the Web Terminal approach.
> Phase 3 (AG-UI translation layer) has not started.

## Status

Accepted (Simplified model implemented; Phase 2 HITL UI superseded by ADR 0018; Phase 3 not started)

## Context

KubeOpenCode currently operates in a "fire-and-forget" model: users create a Task, the controller
creates a Pod, and the agent runs to completion without human interaction. There is no mechanism for
humans to:

- Approve or deny tool executions (e.g., file edits, shell commands)
- Answer agent questions during execution
- Send follow-up messages mid-task
- Observe agent progress in real-time
- Interrupt execution gracefully (finer than the `kubeopencode.io/stop` annotation)

This ADR proposes a phased approach to add Human-in-the-Loop (HITL) capabilities, building on
OpenCode's existing server HTTP API and the Agent Server Mode (ADR 0011).

### Competitive Analysis: Ambient Code Platform

The [ambient-code/platform](https://github.com/ambient-code/platform) project implements a full
HITL system for Claude Code running in Kubernetes. Key design choices:

- **AG-UI protocol** for standardized event streaming
- **SSE** (not WebSocket) for real-time communication — simpler, stateless, proxy-friendly
- **Backend as proxy** — all traffic goes through a Go backend, never direct Pod communication
- **AskUserQuestion tool** — built-in HITL via Claude Agent SDK
- **Interrupt mechanism** — graceful signal to stop current tool call
- **Append-only event log** (JSONL) — persisted for reconnection and replay
- **Multi-user sessions** — per-user credentials in shared sessions

### OpenCode's Built-in HITL Capabilities

Analysis of the OpenCode source code (`../opencode/`) reveals **strong existing HITL support** via
its `opencode serve` HTTP API:

| Endpoint | Purpose | HITL Role |
|----------|---------|-----------|
| `GET /event` | SSE event stream (10s heartbeat) | Real-time event delivery |
| `POST /session/{id}/message` | Send message to session | Human sends follow-up |
| `POST /permission/{id}/reply` | Approve/deny tool execution | Tool approval workflow |
| `POST /question/{id}/reply` | Answer agent question | Structured Q&A |
| `POST /session/{id}/abort` | Interrupt execution | Graceful interruption |
| `GET /doc` | OpenAPI 3.1.1 spec | Self-documenting API |

**Key SSE event types** emitted by OpenCode:

| Event Type | Purpose |
|------------|---------|
| `permission.asked` | Agent requests approval to execute a tool |
| `permission.replied` | Permission response received |
| `question.asked` | Agent asks structured question (options, multi-select) |
| `question.replied` | Question answered |
| `session.status` | Session state change (busy, idle, etc.) |
| `message.part.delta` | Streaming text content |
| `message.updated` | Complete message with all parts |

**Critical limitation:** OpenCode is designed as an interactive coding assistant, not a fully
autonomous agent. Permission requests and questions **block indefinitely** if nobody responds.
There is no built-in timeout or auto-approve mechanism.

**Critical constraint:** OpenCode only accepts one message at a time per session. Sending a message
while the agent is busy returns `Session.BusyError`. The user must call
`POST /session/{id}/abort` first to interrupt, then send a new message.

### AG-UI Protocol Assessment

[AG-UI](https://docs.ag-ui.com/) is an open, event-based protocol standardizing agent-to-frontend
communication. Created by CopilotKit, it has gained significant adoption:

**Adoption (as of March 2026):**
- AWS (Bedrock AgentCore Runtime), Microsoft (Agent Framework), Google (ADK middleware), Oracle
- LangGraph, CrewAI, Mastra, Pydantic AI, AG2, LlamaIndex
- 12,600+ GitHub stars, MIT license

**Relevance:**
- Event-sourced design with ~25 typed events (lifecycle, text, tool calls, state, reasoning)
- Transport-agnostic (SSE, WebSocket, HTTP)
- Go community SDK available (`github.com/ag-ui-protocol/ag-ui/sdks/community/go`)
- HITL interrupt spec is still **draft** — not yet implemented in SDKs

**Assessment:** AG-UI is the emerging industry standard for agent-to-user communication. However,
its HITL spec is still draft, and OpenCode uses its own protocol (not AG-UI). We should design
our internal protocol to be translatable to AG-UI in the future, but not adopt it as a hard
dependency today.

## Decision

### Phased Implementation

Implement HITL in three phases, following a progressive approach — each phase delivers immediate
user value without waiting for the next:

```
Phase 1: Direct OpenCode TUI access via port-forward (zero development)
Phase 2: SSE Proxy + Web UI + CLI (native KubeOpenCode experience)
Phase 3: AG-UI translation layer + multi-framework support
```

---

### Phase 1: Direct OpenCode TUI Access (Zero Development)

#### Rationale

OpenCode already has a production-quality TUI with full HITL support (permission dialogs, question
forms, streaming output, interrupt). The `opencode --attach` command connects to a remote
OpenCode server. Combined with `kubectl port-forward`, this provides a complete HITL experience
with **zero new code**.

Building an SSE proxy backend without a client to consume it puts the cart before the horse.
Phase 1 focuses on making the existing tools accessible in a Kubernetes context.

#### How It Works

For Server-mode Agents (ADR 0011), the Agent controller already creates a Deployment running
`opencode serve` and a Service exposing the port:

```
User's Machine                          Kubernetes Cluster
┌──────────────────┐                    ┌──────────────────────────────┐
│                  │   port-forward     │  Agent Service               │
│  opencode        │ ─────────────────► │  {agent}.{ns}.svc:4096       │
│  --attach        │   localhost:4096   │          │                    │
│  http://localhost │                   │          ▼                    │
│  :4096           │   ◄── SSE ──────  │  OpenCode Server Pod          │
│                  │   ── HTTP POST ─►  │  (opencode serve --port 4096)│
│  Full TUI:       │                    │                              │
│  - Permissions   │                    │  Endpoints:                  │
│  - Questions     │                    │  GET  /event (SSE)           │
│  - Streaming     │                    │  POST /permission/*/reply    │
│  - Interrupt     │                    │  POST /question/*/reply      │
└──────────────────┘                    │  POST /session/*/message     │
                                        │  POST /session/*/abort       │
                                        └──────────────────────────────┘
```

#### User Workflow

```bash
# Step 1: Create a Task (as usual)
kubectl apply -f - <<EOF
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: fix-bug-123
spec:
  agentRef: my-agent          # Must be a Server-mode Agent
  instruction: "Fix the null pointer exception in handler.go"
EOF

# Step 2: Port-forward to the Agent's OpenCode server
kubectl port-forward svc/my-agent -n kubeopencode-system 4096:4096

# Step 3: Connect with OpenCode TUI
opencode --attach http://localhost:4096
```

Once connected, the user sees the **full OpenCode TUI**:
- Agent executes tools → TUI shows `[Allow Once] [Always] [Reject]` prompts
- Agent asks a question → TUI renders options with keyboard navigation
- User types follow-up messages in the input box
- `Ctrl+C` interrupts the current execution gracefully
- Streaming output shows agent's reasoning and tool results in real-time

#### Phase 1 Deliverables

1. **Task `WaitingInput` condition** (`api/v1alpha1/types.go`)
   - New condition type so `kubectl get tasks` shows when HITL is needed
   - Controller watches the OpenCode server's session status via lightweight polling
2. **Documentation** (`docs/features.md`)
   - HITL guide with step-by-step instructions
   - Port-forward workflow documentation
3. **Helm chart** — Ensure Agent Service is correctly configured for port-forward
   (commit `8a20bdf` already adds port-forward RBAC)

#### Task Status Extension

Add a new `WaitingInput` condition to Task status:

```go
// New condition type
const (
    TaskConditionWaitingInput = "WaitingInput"
)

// Condition reasons
const (
    WaitingInputReasonPermission = "PermissionRequired"
    WaitingInputReasonQuestion   = "QuestionAsked"
)
```

When the controller detects a pending permission or question on the OpenCode server,
it updates the Task's status conditions:

```yaml
status:
  phase: Running
  conditions:
    - type: WaitingInput
      status: "True"
      reason: PermissionRequired
      message: "Agent requests permission to edit /src/main.go"
      lastTransitionTime: "2026-03-25T10:30:00Z"
```

This enables standard Kubernetes tooling to detect HITL states:

```bash
# See which tasks are waiting for human input
kubectl get tasks -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,WAITING:.status.conditions[?(@.type=="WaitingInput")].reason

NAME           PHASE     WAITING
fix-bug-123    Running   PermissionRequired
migrate-db     Running   <none>
```

#### Limitations of Phase 1

| Limitation | Impact | Addressed In |
|-----------|--------|-------------|
| Requires `kubectl` + port-forward | Not accessible from web browsers or mobile | Phase 2 |
| Single user per TUI session | No multi-user collaboration | Phase 2 |
| No event persistence | Page refresh / disconnect loses context | Phase 2 |
| No RBAC for HITL actions | Anyone with port-forward access can approve | Phase 2 |
| Manual port-forward setup | Extra steps for users | Phase 2 (native UI) |

---

### Phase 2: SSE Proxy + Web UI + CLI (Native Experience)

#### Rationale

Phase 1 validates the HITL workflow but has friction (port-forward, requires `kubectl`, single
user). Phase 2 builds a native KubeOpenCode experience by adding:

1. An SSE proxy in `kubeopencode-server` (so no port-forward needed)
2. A Web UI for browser-based interaction
3. A CLI for terminal-based interaction

The SSE proxy and its clients (UI/CLI) are built together — the proxy only has value when there
are clients consuming its events.

#### SSE Proxy Architecture

```
External Client (Browser / CLI)
    │
    │  GET  /api/v1/namespaces/:ns/tasks/:name/events     (SSE stream)
    │  POST /api/v1/namespaces/:ns/tasks/:name/permission  (approve/deny)
    │  POST /api/v1/namespaces/:ns/tasks/:name/question    (answer)
    │  POST /api/v1/namespaces/:ns/tasks/:name/message     (follow-up)
    │  POST /api/v1/namespaces/:ns/tasks/:name/interrupt   (abort)
    │
    ▼
┌─────────────────────────────────────────────┐
│         kubeopencode-server (Go)             │
│  ┌───────────────────────────────────────┐  │
│  │          SSE Broker                    │  │
│  │  - Per-task client channels            │  │
│  │  - Ring buffer (last 1000 events)      │  │
│  │  - Fan-out to multiple clients         │  │
│  │  - 15s heartbeat                       │  │
│  │  - Last-Event-ID reconnection support  │  │
│  └───────────────────┬───────────────────┘  │
│                      │                       │
│  ┌───────────────────▼───────────────────┐  │
│  │      Upstream SSE Client               │  │
│  │  - Connects to Pod's /event endpoint   │  │
│  │  - Auto-reconnect (exponential backoff)│  │
│  │  - Per-session event filtering         │  │
│  │  - Basic auth (OPENCODE_SERVER_PASSWORD)│  │
│  └───────────────────┬───────────────────┘  │
│                      │                       │
│  ┌───────────────────▼───────────────────┐  │
│  │      Timeout Manager                   │  │
│  │  - Track pending permission/question   │  │
│  │  - Auto-reject on configurable timeout │  │
│  │  - Configurable default action         │  │
│  └───────────────────────────────────────┘  │
└──────────────────────┼───────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────┐
│     OpenCode Server Pod (per Agent)          │
│     http://{agent}.{ns}.svc.cluster.local    │
│                                              │
│     GET  /event              (SSE source)    │
│     POST /permission/{id}/reply              │
│     POST /question/{id}/reply                │
│     POST /session/{id}/message               │
│     POST /session/{id}/abort                 │
└─────────────────────────────────────────────┘
```

#### SSE Broker Implementation (Go)

```go
type SSEBroker struct {
    mu       sync.RWMutex
    clients  map[string]map[chan Event]struct{} // taskKey -> set of client channels
    buffer   map[string]*RingBuffer[Event]      // taskKey -> ring buffer (last 1000 events)
    upstream map[string]*UpstreamSSE            // taskKey -> upstream connection
}
```

**Upstream connection management:**
- Connect to OpenCode server's `/event` SSE endpoint
- Use `OPENCODE_SERVER_PASSWORD` for basic auth (generated per-Agent as a K8s Secret)
- Auto-reconnect with exponential backoff on disconnect
- Filter events by sessionID (OpenCode SSE is global, not per-session)

**Critical headers:**
```go
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")
w.Header().Set("X-Accel-Buffering", "no")  // disable nginx/ingress buffering
```

#### Permission Reply Flow

```
1. OpenCode emits permission.asked event
   → Upstream SSE Client receives it
   → SSE Broker forwards to all connected external clients

2. Client sends POST /api/v1/namespaces/ns/tasks/my-task/permission/P1
   Body: { "reply": "once" | "always" | "reject" }

3. kubeopencode-server:
   a. Validate user RBAC (SubjectAccessReview on Task resource, verb: "update")
   b. Resolve OpenCode server URL from Task's agentRef → Agent.status.serverStatus.url
   c. Forward to OpenCode: POST {serverURL}/permission/P1/reply
   d. Return result to client

4. OpenCode receives reply → agent unblocks → continues execution
5. permission.replied event flows back through SSE to all connected clients
```

#### Question Reply Flow

Same pattern as permission:

```
POST /api/v1/namespaces/ns/tasks/my-task/question/Q1
Body: { "answers": [["PostgreSQL"], ["dev"]] }
→ Forward to OpenCode: POST {serverURL}/question/Q1/reply
```

#### Authentication

Since SSE connections from browsers cannot set custom `Authorization` headers:

1. Client authenticates via normal REST API (K8s bearer token or OIDC)
2. Server issues a short-lived session token (JWT with taskName, namespace, user, 1h expiry)
3. Client connects to SSE with `?token=<session-token>` query parameter
4. All permission/question/message endpoints validate the token

For CLI usage, standard `Authorization: Bearer <token>` headers work.

#### Timeout for Unanswered Requests

Since OpenCode blocks forever on unanswered permissions/questions, add configurable timeouts
at the proxy layer:

```yaml
# In Agent spec (new field)
spec:
  hitlConfig:
    permissionTimeout: 300s     # Auto-reject after 5 minutes
    questionTimeout: 600s       # Auto-reject after 10 minutes
    defaultPermission: reject   # "reject" or "allow" on timeout
```

The Timeout Manager tracks pending requests and auto-replies on timeout:
- Permissions → configurable default (reject or allow)
- Questions → reject (cannot guess answers)

#### Client Interfaces

**Web UI** (primary) — embedded in kubeopencode-server (like ArgoCD's built-in UI):

```
/ui/                                   → Static SPA (React or Solid)
/api/v1/namespaces/:ns/tasks           → REST API (existing)
/api/v1/namespaces/:ns/tasks/:id/events → SSE stream (new)
/api/v1/namespaces/:ns/tasks/:id/...    → HITL endpoints (new)
```

The UI renders:
- Real-time agent output (streaming text via `message.part.delta` events)
- Permission request dialogs with Allow / Deny / Always buttons
- Question forms (text input, single-select, multi-select)
- Message input for follow-up instructions
- Interrupt button
- Session status indicator (working / waiting input / idle)

**CLI** (secondary) — for power users and automation:

```bash
# Stream events in terminal (read-only)
kocctl task watch fix-bug-123 -n kubeopencode-system

# Interactive HITL session (TUI with Bubble Tea)
kocctl task interact fix-bug-123 -n kubeopencode-system
```

**OpenCode TUI** (fallback, same as Phase 1) — always available via port-forward.

#### Phase 2 Deliverables

1. **SSE Proxy** (`internal/server/handlers/`)
   - `events_handler.go` — SSE streaming + broker + upstream client
   - `hitl_handler.go` — Permission, question, message, interrupt endpoints
2. **Auth Token System** (`internal/server/auth/`)
   - JWT session tokens for SSE connections
3. **Timeout Manager** (`internal/server/hitl/`)
   - Track pending permission/question requests; auto-reply on timeout
4. **Web UI** — React/Solid SPA embedded in kubeopencode-server
5. **CLI** — `kocctl task watch` and `kocctl task interact` commands
   - Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for TUI
6. **Agent `hitlConfig` field** (`api/v1alpha1/types.go`)
7. **Helm chart updates** — UI server configuration
8. **Integration Tests** (`internal/server/handlers/*_test.go`)
9. **Documentation** (`docs/features.md`, `docs/architecture.md`)

---

### Phase 3: AG-UI Translation Layer (Future)

#### Rationale

When AG-UI's HITL/interrupt spec stabilizes (currently draft), add a translation layer so
KubeOpenCode can serve standardized AG-UI events to any compatible frontend (CopilotKit, custom
dashboards, etc.), and support non-OpenCode agent frameworks natively.

#### Architecture

```
OpenCode SSE Events  →  KubeOpenCode Translator  →  AG-UI Events
                                                          ↓
                                                    Any AG-UI Client
                                                    (CopilotKit, custom)
```

**Event mapping (OpenCode → AG-UI):**

| OpenCode Event | AG-UI Event |
|---------------|-------------|
| `session.status` (busy) | `RUN_STARTED` |
| `session.status` (idle) | `RUN_FINISHED` |
| `session.error` | `RUN_ERROR` |
| `message.part.delta` | `TEXT_MESSAGE_CONTENT` |
| `message.updated` (start) | `TEXT_MESSAGE_START` |
| `message.updated` (end) | `TEXT_MESSAGE_END` |
| `permission.asked` | `TOOL_CALL_START` (workaround) or `RUN_FINISHED` with `outcome: "interrupt"` (when spec is final) |
| `question.asked` | `CUSTOM` event (until interrupt spec is final) |

This translation layer also enables future support for non-OpenCode agent frameworks:

```
Agent Framework         Protocol       KubeOpenCode API
─────────────────────────────────────────────────────────
OpenCode serve      →   OpenCode SSE   →  Translation → AG-UI
LangGraph           →   AG-UI native   →  Pass-through
Mastra              →   AG-UI native   →  Pass-through
pi-mono (RPC mode)  →   JSONL/stdin    →  Translation → AG-UI
```

#### Phase 3 Deliverables

1. **AG-UI translator** — OpenCode SSE → AG-UI event mapping
2. **Agent protocol field** — `spec.protocol: opencode | ag-ui | raw`
3. **Multi-framework support** — LangGraph, Mastra agent images with AG-UI native output

---

## Consequences

### Positive

1. **Immediate value (Phase 1)**: HITL works today with zero development via OpenCode TUI
2. **Progressive enhancement**: Each phase adds polish without blocking the previous one
3. **Real-time visibility**: Users can observe agent execution as it happens
4. **Safe execution**: Tool approvals prevent unintended modifications
5. **Interactive sessions**: Users can guide agents with follow-up instructions
6. **Standards-ready**: Designed to translate to AG-UI when the spec stabilizes
7. **Backward compatible**: Fire-and-forget Tasks work unchanged (no HITL overhead)

### Negative

1. **Server Mode only**: Pod-mode Tasks cannot use HITL (no persistent server to connect to)
2. **Phase 1 friction**: Port-forward + OpenCode CLI is not seamless
3. **OpenCode dependency**: Phase 1–2 are coupled to OpenCode's SSE protocol
4. **Phase 2 complexity**: SSE proxy, auth tokens, timeout management add moving parts

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| OpenCode changes SSE protocol | Pin to specific OpenCode versions; add integration tests |
| SSE connections overwhelm server (Phase 2) | Ring buffer with max 1000 events; connection limits per client |
| Permission timeout auto-rejects valid request | Make timeout configurable; default to safe values |
| AG-UI spec changes significantly | Translation layer is isolated; easy to update mapping |
| No one responds to HITL requests (Phase 1) | WaitingInput condition makes it visible; document the workflow |
| Port-forward is fragile (Phase 1) | Document reconnection; Phase 2 removes this dependency |

## Prerequisites

1. **Agent Server Mode stable** (ADR 0011) — Server Mode must be production-validated
2. **OpenCode `serve` stability** — The SSE/permission/question APIs must be reliable
3. **Port-forward RBAC** — Already in place (commit `8a20bdf`)

## Appendix A: OpenCode Permission Request Schema

```typescript
// permission.asked event payload
{
  id: string,                          // Request ID (UUID)
  sessionID: string,
  permission: string,                  // "edit", "write", "shell", etc.
  patterns: string[],                  // File paths/glob patterns
  metadata: Record<string, any>,       // Tool-specific context
  always: string[],                    // Patterns to remember for "always"
  tool?: {
    messageID: string,
    callID: string,
  }
}

// Reply: POST /permission/{id}/reply
{
  reply: "once" | "always" | "reject",
  message?: string                     // Feedback if rejecting
}
```

## Appendix B: OpenCode Question Request Schema

```typescript
// question.asked event payload
{
  id: string,
  sessionID: string,
  questions: [
    {
      question: string,                // "Which database should we use?"
      header: string,                  // Short label (max 30 chars)
      options: [
        { label: string, description: string }
      ],
      multiple?: boolean,              // Allow multi-select
      custom?: boolean,                // Allow free-text (default: true)
    }
  ],
  tool?: { messageID: string, callID: string }
}

// Reply: POST /question/{id}/reply
{
  answers: [
    ["PostgreSQL"],                    // Array of selected labels per question
  ]
}

// Reject: POST /question/{id}/reject
// (no body required)
```

## Appendix C: AG-UI Protocol Summary

AG-UI is an open, event-based protocol (MIT license, 12.6k GitHub stars) standardizing
agent-to-frontend communication. Created by CopilotKit, adopted by AWS, Microsoft, Google, Oracle.

**Core event categories (~25 types):**
- Lifecycle: `RUN_STARTED`, `RUN_FINISHED`, `RUN_ERROR`, `STEP_STARTED`, `STEP_FINISHED`
- Text: `TEXT_MESSAGE_START`, `TEXT_MESSAGE_CONTENT`, `TEXT_MESSAGE_END`
- Tool calls: `TOOL_CALL_START`, `TOOL_CALL_ARGS`, `TOOL_CALL_END`, `TOOL_CALL_RESULT`
- State: `STATE_SNAPSHOT`, `STATE_DELTA` (RFC 6902 JSON Patch)
- Reasoning: `REASONING_START`, `REASONING_MESSAGE_CONTENT`, `REASONING_END`
- Special: `RAW`, `CUSTOM`

**HITL status:** Draft spec (`docs.ag-ui.com/drafts/interrupts`). Uses `RUN_FINISHED` with
`outcome: "interrupt"` and resume payload. Not yet implemented in SDKs. Current workaround
uses tool calls as HITL mechanism.

**SDKs:** TypeScript (1st party), Python (1st party), Go (community), Kotlin (community).

**Assessment:** Emerging standard with strong industry momentum, but HITL spec is pre-production.
Monitor closely; adopt as translation target, not hard dependency.

## Appendix D: Comparison with Ambient Code Platform

| Aspect | KubeOpenCode (proposed) | Ambient Code Platform |
|--------|------------------------|----------------------|
| Agent runtime | OpenCode (TypeScript/Bun) | Claude Agent SDK (Python) |
| HITL protocol | OpenCode native SSE | AG-UI over SSE |
| Phase 1 interaction | OpenCode TUI via port-forward | Next.js Web UI |
| Phase 2 proxy | kubeopencode-server (Go) | Backend API (Go/Gin) |
| Event persistence | Ring buffer (Phase 2) | JSONL on disk |
| Auth | K8s RBAC + JWT tokens (Phase 2) | K8s user tokens (SSAR) |
| Multi-user | Phase 2 | Per-user credentials |
| UI | Embedded SPA (Phase 2) | Next.js + Shadcn UI |
| CLI | kocctl (Phase 2) | acpctl |
| Interrupt | OpenCode abort (Phase 1) / proxy (Phase 2) | Runner interrupt endpoint |
| External deps | None | PostgreSQL (optional), Langfuse |

## Appendix E: Alternative Agent Frameworks Evaluated

For future multi-framework support (Phase 3), the following were evaluated:

| Framework | HITL Mechanism | AG-UI | Server Mode | Language | Assessment |
|-----------|---------------|-------|-------------|----------|------------|
| OpenCode | Permission + Question system | No | `opencode serve` | TypeScript | **Current choice** — strongest coding ability |
| Claude Agent SDK | AskUserQuestion + canUseTool | No | Subprocess (wrappable) | Python/TS | Strong, but commercial ToS |
| pi-mono | Extension UI sub-protocol (RPC) | No | RPC mode / SDK mode | TypeScript | 27.7k stars, MIT, single maintainer risk |
| LangGraph | interrupt() + Command(resume=) | Yes (partner) | LangGraph Platform | Python | Best HITL design, production-proven |
| Mastra | suspend/resume in workflows | Yes (1st party) | Built-in server | TypeScript | Strong, Y Combinator backed |
| CrewAI | @human_feedback decorator | Yes (partner) | Webhook-based | Python | Good for multi-agent orchestration |

**Recommendation:** OpenCode for Phase 1–2 (strongest coding agent). Monitor LangGraph and Mastra
for Phase 3 AG-UI integration.

## Implementation Status & Decisions (2026-03-25)

### Simplified HITL Model

During implementation, the original 3-phase plan was refined into a **simpler, unified model**:

| Scenario | Method | Permissions | Output |
|----------|--------|-------------|--------|
| **Task** (automated) | `opencode run --attach` | All auto-allowed (`OPENCODE_PERMISSION={"*":"allow"}`) | OpenCode native TUI output in pod logs |
| **Interactive** (HITL) | `opencode attach` + port-forward | User approves in TUI | Full interactive TUI experience |

**Key decision: Tasks are always non-interactive.** This simplifies the user mental model:
- Tasks = fire-and-forget, all permissions auto-approved, no HITL overhead
- HITL = users explicitly choose to attach to a server agent via TUI

This avoids the complexity of the SSE broker, timeout manager, and JWT token system
originally planned for Phase 2. The Web UI HITL infrastructure (SSE proxy, permission/question
endpoints, HITLPanel component) was implemented but is secondary to the TUI-based workflow.

### Bugs Fixed During Implementation

1. **Credentials not mounted in Server mode** — `BuildServerDeployment` never called
   `buildCredentials()`, so API keys (e.g., `OPENCODE_API_KEY`) were missing from server pods.

2. **Missing HOME/SHELL env vars in Server mode** — SCC compatibility env vars were not set,
   causing failures on OpenShift where containers run with random UIDs.

3. **Agent contexts not loaded in Server mode** — Server-mode Deployments completely ignored
   `Agent.contexts` (Text, ConfigMap, Git, Runtime). Context resolution was extracted into
   shared `context_processor.go` functions used by both `TaskReconciler` and `AgentReconciler`.

4. **`OPENCODE_PERMISSION` overriding custom permissions** — Both `pod_builder.go` and
   `server_builder.go` unconditionally set `OPENCODE_PERMISSION={"*":"allow"}`, which would
   override the config file's `permission: {edit: "ask"}` settings. Fixed with
   `configHasPermission()` check (though this is now moot with the simplified model).

5. **`opencode run --attach` auto-rejecting permissions** — OpenCode's `run` command
   (`run.ts:544-556`) auto-rejects all `permission.asked` events since it's non-interactive.
   This blocked the original HITL-via-Web-UI approach. Resolved by the simplified model
   where Tasks always auto-allow all permissions.

6. **Config file not written to server pod** — `server_builder.go` set `OPENCODE_CONFIG` env
   var but never created the config file. Fixed by writing config inline via heredoc in the
   server command, or via context-init container when contexts are present.

### What Was Implemented

**Backend:**
- `context_processor.go` — Shared context resolution functions (extracted from TaskReconciler)
- `server_builder.go` — Full context support (init containers for Git, ConfigMap, Text, Runtime)
- `server_builder.go` — Credentials mounting (`buildCredentials`), HOME/SHELL env vars
- `hitl_handler.go` — SSE proxy, permission/question/message/interrupt endpoints
- `task_submit.go` — Utility subcommand for API-based task submission (kept as backup)
- `agent_controller.go` — Context ConfigMap reconciliation for server-mode agents
- `agent_handler.go` — Server port in API response

**Frontend (Web UI):**
- `HITLPanel.tsx` — SSE event streaming, permission/question UI, message input
- `AgentDetailPage.tsx` — Quick Connect section with KubeOpenCode CLI, manual steps, shell alias tips
- `TaskCreatePage.tsx` — Server-mode agent context limitation note
- `TaskDetailPage.tsx` — HITLPanel integration for running tasks

**CLI:**
- `cmd/cli/` — Independent `kubeopencode` CLI binary
- `kubeopencode agent attach` — One-click server agent attach (port-forward + opencode attach)
- `kubeopencode session watch/attach` — Task event streaming and HITL interaction

### Architecture Decision: task-submit vs opencode run --attach

We implemented `kubeopencode task-submit` as a HITL-compatible task submitter that uses the
OpenCode HTTP API directly (create session → submit prompt → poll status). Unlike
`opencode run --attach`, it does NOT auto-reject permissions.

However, with the simplified model (Tasks = always auto-allow), `opencode run --attach` works
correctly since no `permission.asked` events are generated. The `task-submit` command is kept
in the codebase as a utility for future HITL scenarios but is not used in the default flow.

## References

- ADR 0011: Agent Server Mode (`docs/adr/0011-agent-server-mode.md`)
- ADR 0012: Defer Session API (`docs/adr/0012-defer-session-api.md`)
- OpenCode server source: `../opencode/packages/opencode/src/server/server.ts`
- OpenCode permission system: `../opencode/packages/opencode/src/permission/index.ts`
- OpenCode question system: `../opencode/packages/opencode/src/question/index.ts`
- [AG-UI Protocol](https://docs.ag-ui.com/)
- [AG-UI GitHub](https://github.com/ag-ui-protocol/ag-ui)
- [AG-UI Interrupt Draft](https://docs.ag-ui.com/drafts/interrupts)
- [Ambient Code Platform](https://github.com/ambient-code/platform)
- [pi-mono](https://github.com/badlogic/pi-mono)
- [kubernetes-sigs/agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox)
- [tmaxmax/go-sse](https://github.com/tmaxmax/go-sse) — Go SSE library with replay support
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — Go TUI framework
