# ADR 0012: Defer Session API to Post-v0.1

## Status

Accepted

## Context

A design document was proposed to introduce a new `Session` CRD for KubeOpenCode, aiming to provide interactive, persistent OpenCode environments with Web Terminal access. The proposed `Session` API would:

1. Define a new CRD `Session` (`session.kubeopencode.io`)
2. Create a per-session Deployment + Service running `opencode serve`
3. Expose a WebSocket-based Web Terminal via PTY proxy in `kubeopencode-server`
4. Allow users to interactively connect to long-running OpenCode environments

We conducted a thorough analysis of the design against the existing codebase, covering architecture, API design, security, scalability, and project maturity.

### The Proposed Design

**SessionSpec:**
- `agentRef`: Reference to an Agent (required to have `serverConfig`)
- `title`: Optional display name
- `ttlSecondsAfterFinished`: Auto-cleanup timer

**SessionStatus:**
- `phase`: Pending, Running, Failed, Terminating
- `deploymentName`, `serviceName`, `podName`, `url`
- `conditions`: Standard Kubernetes conditions

**Controller:** A new `SessionReconciler` that creates and manages Deployment + Service per Session.

**Web Terminal:** WebSocket proxy in `kubeopencode-server` forwarding to OpenCode's PTY API (`POST /pty`, `GET /pty/:ptyID/connect`).

## Decision

**Defer the Session API.** Do not implement it in the current project phase (v0.1.x).

### Reason 1: Architectural Overlap with Agent Server Mode

The proposed Session controller would create a Deployment + Service running `opencode serve` — which is exactly what the existing Agent Server Mode already does (ADR 0011, `internal/controller/agent_controller.go`, `internal/controller/server_builder.go`).

This creates two parallel infrastructure management paths:

| Concern | Agent Server Mode | Proposed Session |
|---------|-------------------|-----------------|
| Creates Deployment | Yes (`AgentReconciler`) | Yes (`SessionReconciler`) |
| Creates Service | Yes (`AgentReconciler`) | Yes (`SessionReconciler`) |
| Runs `opencode serve` | Yes | Yes |
| Health checks | Yes (liveness + readiness probes) | Yes (same probes) |
| Status tracking | `Agent.status.serverStatus` | `Session.status` |

The `server_builder.go` functions are tightly coupled to the Agent type:

```go
func BuildServerDeployment(agent *kubeopenv1alpha1.Agent, ...) *appsv1.Deployment
func BuildServerService(agent *kubeopenv1alpha1.Agent) *corev1.Service
```

Reusing them for Session would require either significant refactoring or code duplication. Additionally, if a Session references an Agent that already has `serverConfig`, the AgentReconciler has already created the Deployment/Service — creating a naming conflict and OwnerReference ambiguity.

A more natural design would be for Session to act as a lightweight logical session on top of an existing Agent server, rather than managing its own infrastructure.

### Reason 2: Resource Scalability Concern

The proposed design maps **1 Session = 1 Deployment = 1 Service = 1 Pod**. For multi-user scenarios, this means:

- 100 users = 100 Deployments + 100 Services + 100 Pods
- Each OpenCode server Pod requires ~500Mi+ memory
- 100 users ≈ 50-100 GB memory constantly allocated, mostly idle

While Kubernetes can handle hundreds of Services and Deployments technically, this resource model is inefficient. The correct pattern for interactive sessions is:

- **1 Agent = 1 Deployment** (shared infrastructure)
- **N Sessions = N logical sessions** within the OpenCode server (via `/session` API)
- Web Terminal connections go through PTY sessions on the shared server

This is O(1) infrastructure per Agent instead of O(n) per user.

### Reason 3: Security Design Absent

Web Terminal exposes direct shell access from the browser to a container. The design document contains no security considerations:

- **Authentication/Authorization**: No RBAC model for who can connect to which Session's terminal
- **Audit logging**: Terminal I/O should be recorded for compliance
- **Transport security**: WebSocket connections require TLS
- **Session isolation**: Multi-user scenarios need user-level isolation
- **Network policy**: The WebSocket proxy path needs network security controls

Shipping Web Terminal without a security model would create an unacceptable attack surface.

### Reason 4: Project Maturity

KubeOpenCode is at v0.1.0 with a v1alpha1 API. The core value proposition — Task-based AI agent execution — needs production validation before expanding the API surface. Specifically:

- **Agent Server Mode** (ADR 0011) is newly implemented and not yet battle-tested in production
- **Core Task workflow** (context resolution, credential mounting, cross-namespace) needs user feedback
- Adding a 4th CRD increases API surface by ~33% and maintenance burden proportionally
- Once a CRD is published, backward compatibility constraints limit future redesign

### Reason 5: Design Inconsistencies

The proposed design has several inconsistencies with the existing codebase:

1. **Phase design mismatch**: Task uses `Pending/Queued/Running/Completed/Failed`; Session proposes `Pending/Running/Failed/Terminating` — missing `Completed`, adding `Terminating` (Task uses a `Stopped` condition instead)
2. **Incomplete API**: Only List/Get/Exec endpoints; missing Create/Delete/Update — users would need `kubectl` for lifecycle management, degrading the UI experience
3. **Build system references**: Design references `make generate` and `make manifests` which don't exist; the project uses `make update` and `make verify`
4. **Missing finalizer design**: Task uses `kubeopencode.io/task-cleanup` finalizer; Session design relies solely on OwnerReference without considering cleanup of OpenCode server state

## Consequences

### Positive

1. **Focused engineering**: Team can concentrate on stabilizing Task workflow and Agent Server Mode
2. **Avoid premature API commitment**: No backward-compatibility burden for an under-designed CRD
3. **Better future design**: Real-world usage of Agent Server Mode will inform a more accurate Session design
4. **Smaller attack surface**: No Web Terminal exposure without proper security model

### Negative

1. **No interactive sessions**: Users who want interactive OpenCode environments must use `kubectl exec` or `opencode attach` directly
2. **No Web Terminal**: Browser-based terminal access is deferred

### What We Preserve

The path to interactive sessions remains open. When the time comes, the recommended approach is:

**Session as a logical concept on top of Agent Server Mode** (not a parallel infrastructure path):

1. Session references a running Server-mode Agent
2. Session controller creates a logical session on the Agent's OpenCode server (via `/session` API)
3. Web Terminal connects to the Agent's OpenCode server PTY (via `/pty` API)
4. No additional Deployments or Services per Session

This approach leverages the existing Agent Server Mode infrastructure and OpenCode's built-in session/PTY management, avoiding resource multiplication.

## Prerequisites for Future Implementation

Before implementing Session API, the following should be in place:

1. **Agent Server Mode production validation**: Real users running Server-mode Agents at scale
2. **Security model**: RBAC for terminal access, audit logging, TLS for WebSocket
3. **OpenCode session persistence**: Understanding how OpenCode handles session state across server restarts
4. **User demand signal**: Concrete user feedback requesting interactive sessions beyond `kubectl exec`

## References

- Session API design document (Manus AI, 2026-02-05)
- ADR 0011: Agent Server Mode (`docs/adr/0011-agent-server-mode.md`)
- Agent controller: `internal/controller/agent_controller.go`
- Server builder: `internal/controller/server_builder.go`
- OpenCode PTY implementation: `../opencode/packages/opencode/src/pty/index.ts`
- OpenCode serve command: `../opencode/packages/opencode/src/cli/cmd/serve.ts`
