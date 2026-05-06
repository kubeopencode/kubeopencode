# ADR 0037: KubePi Feasibility Analysis — Pi as Alternative Agent Runtime

## Status

Research / Informational

## Date

2026-05-06

## Context

### Motivation

The [Pi coding agent](https://pi.dev) (by Mario Zechner) has gained significant momentum: 31.7K GitHub stars, 1M+ npm weekly downloads, and a rapidly growing ecosystem. Pi's design philosophy — minimal core (4 tools, <1K token system prompt), aggressive extensibility via TypeScript extensions, and multi-provider support (15+ LLM providers including OAuth subscriptions) — represents a fundamentally different approach from OpenCode.

Several developments make Pi increasingly compelling:

1. **Extensibility** — Pi's extension system (TypeScript modules with access to tools, commands, keyboard shortcuts, events, and the full TUI) is more flexible than OpenCode's plugin model
2. **Package ecosystem** — Pi packages (bundling extensions, skills, prompts, themes) distributed via npm or Git enable community-driven capability sharing
3. **Multi-provider with subscriptions** — OAuth-based authentication for Claude Pro/Max, ChatGPT Plus/Pro, GitHub Copilot, Google Gemini CLI
4. **agentOS integration** — Rivet's agentOS has first-class Pi support with ~6ms cold starts via WebAssembly/V8 isolates
5. **Context engineering** — Minimal system prompt leaves more context window for actual work; extensions can inject/filter messages dynamically

This ADR analyzes the feasibility of building **KubePi** — a product with the same mission as KubeOpenCode (bringing agentic AI into Kubernetes) but using Pi as the agent runtime.

### Pi vs OpenCode: Technical Comparison

| Dimension | OpenCode | Pi |
|---|---|---|
| Language | Go (single binary, ~30MB) | TypeScript/Node.js (npm package) |
| Core tools | 10+ built-in tools | 4 tools (read, write, edit, bash); extras opt-in |
| System prompt | ~5K-10K tokens | <1K tokens |
| Server mode | `opencode serve` — full HTTP REST API (OpenAPI 3.1) | **None built-in**. RPC mode (stdin/stdout JSONL) + SDK |
| Session management | SQLite DB, HTTP API for CRUD | File-based sessions, RPC commands |
| Extension model | Plugins (npm packages, server/tui targets) | Extensions (TypeScript modules, full lifecycle hooks) |
| Skills | SKILL.md convention, Git sources | AGENTS.md + skills directory, Git/npm sources |
| Operating modes | TUI, `run`, `serve`, `attach`, `web` | Interactive, Print/JSON, RPC, SDK |
| Health endpoint | `GET /session/status` | None |
| LLM providers | ~10 providers (API key only) | 15+ providers (API key + OAuth subscriptions) |
| MCP support | Built-in | None (by design; use CLI tools with READMEs instead) |
| Permission model | Configurable (5 modes) | YOLO by default (no permission popups) |
| License | Source-available | MIT |

### The Critical Gap: No HTTP Server Mode

Pi's most significant limitation for Kubernetes orchestration is the **absence of a built-in HTTP server mode**. KubeOpenCode's architecture fundamentally relies on `opencode serve`:

- Agent runs as a Deployment with `opencode serve --port 4096 --hostname 0.0.0.0`
- Task Pods connect via `opencode run --attach http://<agent-svc>:<port>`
- Controller queries `GET /session/status` for health probes
- Controller queries `GET /session?search=<title>` to resolve session info after task completion
- `opencode attach` provides interactive terminal access

Pi offers four modes — none of which is an HTTP server:

1. **Interactive** — full TUI (requires terminal)
2. **Print/JSON** — one-shot, non-interactive
3. **RPC** — headless JSON protocol over stdin/stdout (designed for embedding)
4. **SDK** — library import for Node.js applications

### Community Solutions for Pi Server Mode

Two relevant projects address this gap:

**pi-server** ([@marcfargas/pi-server](https://github.com/marcfargas/pi-server)):
- Wraps Pi RPC mode in a WebSocket server
- Detachable sessions — disconnect/reconnect with full state restoration
- Status: M1 complete (connect, chat, reconnect), but no authentication, no TLS, no Docker deployment yet
- Architecture: `TUI Client ←WS→ pi-server ←stdio→ pi --mode rpc`

**pi-acp** ([@svkozak/pi-acp](https://github.com/svkozak/pi-acp)):
- ACP (Agent Client Protocol) adapter for Pi
- Bridges ACP JSON-RPC 2.0 over stdio to Pi's RPC mode
- Designed for Zed editor integration
- Supports session history, slash commands, skill commands

### Current KubeOpenCode-OpenCode Coupling Depth

Analysis of the codebase reveals **very tight, monolithic coupling** with no abstraction layer:

| Layer | Coupling Points | Files Affected |
|---|---|---|
| Binary distribution | Init container downloads OpenCode binary, copies to `/tools/opencode` | `pod_builder.go`, `agents/opencode/` |
| Command interface | Hardcoded `opencode run`, `opencode serve`, `opencode run --attach` | `pod_builder.go`, `server_builder.go` |
| HTTP API client | Dedicated client for `/session`, `/session/{id}/message` | `opencode_client.go` |
| Configuration schema | `opencode.json` with `skills.paths`, `plugin`, `instructions`, `permission` | `skill_processor.go` |
| Environment variables | 5+ OpenCode-specific env vars (`OPENCODE_CONFIG`, `OPENCODE_DB`, `OPENCODE_PERMISSION`, `OPENCODE_CONFIG_CONTENT`, `OPENCODE_TUI_CONFIG`) | `pod_builder.go` |
| Health monitoring | Probes against `/session/status` | `server_builder.go` |
| Session lifecycle | Task status mapped to OpenCode session data (token usage, cost, file changes) | `task_controller.go`, `opencode_client.go` |
| Plugin system | `PluginSpec` with npm-based plugins, server/tui targets | `agent_types.go`, `skill_processor.go` |
| Agent images | 3 purpose-built images: `kubeopencode-agent-opencode`, `kubeopencode-agent-devbox`, `kubeopencode-agent-attach` | `agents/` |

**Estimated migration effort**: 15-20 files, ~2000-3000 lines of production code, ~3000+ lines of tests.

## Analysis

### Architecture: kubepid as HTTP-to-RPC Bridge

The key architectural insight: **do not try to make Pi into an HTTP server. Instead, build a Go sidecar (`kubepid`) that bridges HTTP to Pi's RPC protocol.** This preserves KubeOpenCode's proven Kubernetes orchestration model while leveraging Pi's runtime.

```
                    ┌── Agent Deployment (long-running) ──┐
                    │                                     │
 Task Pod ──HTTP──► │  kubepid ──RPC──► pi --mode rpc     │
                    │  (Go)    stdin/   (Node.js)         │
 kubepi attach ─WS─►│         stdout                      │
                    │                                     │
                    └─────────────────────────────────────┘
```

**kubepid responsibilities:**

| Responsibility | Implementation |
|---|---|
| Process lifecycle | Spawn `pi --mode rpc`, monitor process health, graceful shutdown (abort RPC → SIGTERM → SIGKILL) |
| HTTP API | Expose REST endpoints that translate to Pi RPC commands |
| Health checks | `GET /healthz` — Pi process alive + recent RPC heartbeat; `GET /readyz` — extensions loaded |
| Session management | `GET /session/status` (compatibility), `GET /session/:id` → Pi `get_session_stats` RPC |
| Task submission | `POST /prompt` → Pi `prompt` RPC; `POST /prompt_async` → non-blocking |
| Steering | `POST /steer` → Pi `prompt` RPC with `streamingBehavior: "steer"` |
| Interactive access | `WS /attach` — bidirectional bridge to Pi RPC event stream |
| Metrics collection | Subscribe to Pi RPC events, extract token/cost, expose `/metrics` (Prometheus) |
| Context injection | Mount contexts as files, inject via Pi's `AGENTS.md` and extension hooks |

**kubepid API** (deliberately aligned with OpenCode's API to minimize controller changes):

```
GET  /healthz                    → Pi process alive + RPC heartbeat
GET  /session/status             → compatibility shim for K8s probes
GET  /session                    → list Pi sessions
GET  /session/:id                → Pi get_session_stats → translated response
GET  /session/:id/message        → Pi session messages (if supported by RPC)
POST /session/:id/prompt         → Pi prompt RPC
POST /session/:id/prompt_async   → Pi prompt RPC (non-blocking)
POST /session/:id/abort          → Pi abort RPC
POST /session/:id/steer          → Pi prompt RPC + streamingBehavior: "steer"
WS   /attach                    → bidirectional Pi RPC event bridge
```

### Container Architecture

**Two-container pattern (adapted from KubeOpenCode):**

| Container | KubeOpenCode | KubePi |
|---|---|---|
| Init | `kubeopencode-agent-opencode` (copies Go binary `/opencode` → `/tools/opencode`) | `kubeopencode-agent-pi` (Node.js with Pi installed, copies to `/tools/`) |
| Main (server mode) | `opencode serve --port 4096` | `kubepid --port 4096` (spawns `pi --mode rpc` internally) |
| Main (task mode) | `opencode run --attach <url>` | `kubepid run --server <url> --prompt <file>` (connects to server kubepid, submits prompt via HTTP) |

**Alternative: single-binary approach.** Since kubepid is the bridge anyway, it could embed a minimal Node.js runtime or use Pi's RPC mode directly by spawning the `pi` CLI. The init container would install Pi globally (`npm install -g @mariozechner/pi-coding-agent`) and kubepid would spawn it.

### CRD API Changes

#### Agent Spec — What Changes

```yaml
apiVersion: kubeopencode.io/v1alpha1   # or kubepi.io/v1alpha1
kind: Agent
metadata:
  name: code-reviewer
spec:
  # UNCHANGED — generic Kubernetes configuration
  executorImage: "ghcr.io/kubepi/devbox:latest"
  workspaceDir: /workspace
  credentials:
    - secretRef: anthropic-api-key
      env: ANTHROPIC_API_KEY
  persistence:
    workspace:
      storageClassName: standard
      size: 10Gi
  maxConcurrentTasks: 5
  quota:
    maxTokensPerTask: 500000
  
  # CHANGED — Pi-native fields replace OpenCode-specific ones
  provider: anthropic                          # NEW: Pi's --provider flag
  model: claude-sonnet-4-20250514                        # NEW: Pi's --model flag  
  thinking: high                               # NEW: Pi's --thinking flag
  
  extensions:                                  # REPLACES: plugins
    - name: security-policy
      git:
        repository: https://github.com/org/pi-extensions.git
        path: security/
    - name: custom-tools
      inline: |
        module.exports = function(pi) {
          pi.on("before_tool_call", async (event) => {
            if (event.tool === "bash" && event.args.command.includes("rm -rf")) {
              return { blocked: true, reason: "Destructive command blocked" };
            }
          });
        };
  
  packages:                                    # REPLACES: plugins (npm packages)
    - "@anthropic/pi-test-runner"
    - "git+https://github.com/org/pi-custom-tools.git"
  
  skills:                                      # SIMILAR: Git-based skills (Pi also uses skills)
    - name: k8s-ops
      git:
        repository: https://github.com/org/skills.git
        path: kubernetes/
  
  # REMOVED fields:
  # - agentImage (Pi installed via npm, not binary copy)
  # - attachImage (kubepid handles attach)
  # - config (replaced by provider/model/thinking + extensions)
  # - plugins (replaced by extensions + packages)
```

#### Task Spec — What Changes

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: fix-auth-bug
spec:
  agentRef:
    name: code-reviewer
  description: |
    Fix the authentication bypass in src/middleware/auth.ts
  contexts:
    - type: Git
      git:
        url: https://github.com/org/webapp.git
        ref: main
  
  # NEW: Pi-native capabilities
  steering:                                    # NEW: mid-execution intervention
    - trigger: afterMinutes(10)
      message: "Wrap up with a summary of changes made."
      mode: followUp
  
  timeout: 30m                                 # NEW: explicit timeout (vs K8s activeDeadlineSeconds)
```

**Task status remains largely unchanged** — `SessionInfo`, `TokenUsage`, `SessionSummary` are generic enough. The kubepid bridge translates Pi's stats to the same format.

### Feature Comparison: What Changes, What Stays, What's New

#### Features to KEEP (runtime-agnostic)

| Feature | Reason |
|---|---|
| Task CRD lifecycle (Pending → Queued → Running → Completed/Failed) | Pure Kubernetes concern |
| Agent Deployment + Service pattern | kubepid makes this work with Pi |
| Context system (Text, ConfigMap, Git, Runtime, URL) | Content sourcing is runtime-independent |
| Credential injection (Secrets → env vars) | Kubernetes-native |
| Persistence (workspace PVC) | Kubernetes-native |
| Suspend / Resume / Standby | Deployment lifecycle management |
| Share link (token-based terminal) | WebSocket via kubepid instead of HTTP |
| CronTask (scheduled execution) | Creates Tasks on schedule, runtime-independent |
| Quota and concurrency control | Rate limiting is runtime-independent |
| KubeOpenCodeConfig (cluster-scoped) | Cleanup, proxy, system config |
| Git auto-sync (sidecar) | Runtime-independent sidecar |
| Agent capacity management | Controller logic, not runtime-specific |
| CLI (`kubeoc` / `kubepi`) | Adapts to new API fields |
| UI (task list, agent management) | Adapts to new API fields |

#### Features to CHANGE

| Feature | KubeOpenCode | KubePi | Effort |
|---|---|---|---|
| Agent server process | `opencode serve` | `kubepid` (Go) + `pi --mode rpc` | High (new component) |
| Runtime API client | `opencode_client.go` (HTTP to OpenCode REST) | `pi_client.go` or kubepid internal (HTTP to Pi RPC) | High (rewrite) |
| Health probes | `GET /session/status` | kubepid `GET /healthz` | Low |
| Config injection | `opencode.json` with skills.paths, plugin arrays | Pi `settings.json` + `AGENTS.md` + extension files | Medium |
| Plugin/extension system | `PluginSpec` (npm, server/tui targets) | `ExtensionSpec` (TypeScript modules, Git/inline) + `PackageSpec` (npm/Git Pi packages) | Medium |
| Init container | Copy Go binary `/opencode` → `/tools/` | Install Pi via npm or copy Node.js + Pi to `/tools/` | Medium |
| Interactive attach | `opencode attach <url>` (HTTP) | kubepid WebSocket `/attach` → Pi RPC bridge | Medium |
| Session persistence | SQLite DB (`opencode.db`) on PVC | Pi session files (JSON) on PVC | Low |
| Skill injection | `skills.paths` in `opencode.json` | Files in `~/.pi/agent/skills/` or `.pi/skills/` | Low |
| Permission model | `OPENCODE_PERMISSION={"*":"allow"}` env var | Pi default YOLO mode (no change needed); or extension-based policy | Low |

#### Features to ADD (Pi-native capabilities)

| Feature | Description | Value |
|---|---|---|
| **Steering API** | Mid-execution intervention via Pi's `steer` and `followUp` RPC modes | Operators can guide running tasks without restarting |
| **Extension hooks** | `before_tool_call`, `after_agent_turn`, `before_agent_start` lifecycle hooks | Security policies, custom logging, context injection — without modifying runtime code |
| **Inline extensions** | Define extensions directly in Agent YAML (no Git repo needed) | Quick customization for simple policies |
| **Multi-provider with OAuth** | Claude Pro/Max, ChatGPT Plus/Pro, GitHub Copilot subscriptions | Cost savings — use existing subscriptions instead of API keys |
| **Live task streaming** | kubepid subscribes to Pi RPC events, forwards via WebSocket/SSE | Real-time visibility into agent reasoning and tool calls |
| **Model switching** | Pi supports runtime model changes via `config` RPC command | Phase-based tasks using different models for different steps |
| **Package ecosystem** | Pi packages from npm/Git | Community-driven capability sharing |

#### Features to REMOVE or SIMPLIFY

| Feature | Reason |
|---|---|
| `PluginSpec` with server/tui targets | Replaced by Pi's simpler extension model (no target distinction) |
| `agentImage` / `attachImage` separation | Pi is installed via npm; kubepid handles attach; two separate images not needed |
| OpenCode TUI config (`tui.json`) | Pi's TUI is configured via settings.json and extensions; no separate TUI config |
| `OPENCODE_CONFIG_CONTENT` instructions injection | Pi uses `AGENTS.md` files and `before_agent_start` extension hooks |

### Pi RPC Protocol: Key Capabilities for KubePi

Pi's RPC mode provides the following commands relevant to KubePi orchestration:

| RPC Command | Purpose | KubePi Usage |
|---|---|---|
| `prompt` | Send user message | Task submission, steering |
| `abort` | Cancel current operation | Task cancellation, timeout |
| `get_session_stats` | Token usage, cost, message count | Quota enforcement, task status enrichment |
| `config` | Change model/provider at runtime | Multi-model task phases |
| `switch_session` | Switch to different session | Session management |
| `reload` | Reload extensions and skills | Hot-reload after config changes |

Pi RPC events relevant to monitoring:

| Event Type | Data | KubePi Usage |
|---|---|---|
| `message_update` | Text deltas, tool calls, tool results | Live streaming, progress tracking |
| `session_update` | Session metadata changes | Status sync |
| `error` | Error details | Failure detection |
| `idle` | Agent finished processing | Task completion detection |

### Risk Assessment

| Risk | Severity | Mitigation |
|---|---|---|
| **Pi RPC protocol instability** — RPC is not a stable public API; may change between versions | High | Pin Pi version; kubepid abstracts protocol details; version compatibility matrix |
| **Node.js in containers** — larger image footprint (~150MB vs ~30MB for Go binary), slower startup | Medium | Multi-stage builds; pre-installed Pi in agent image; Node.js is standard in dev containers |
| **pi-server immaturity** — if we depend on it, it's early-stage | Medium | Build kubepid ourselves instead of depending on pi-server; kubepid is Go, full control |
| **No official server mode** — Pi may never ship built-in HTTP server | Low | kubepid is our abstraction; we don't depend on Pi adding server mode |
| **Extension security** — Pi's YOLO-by-default permission model in multi-tenant K8s | Medium | kubepid can implement permission enforcement; security-policy extension for tool-call filtering |
| **Session persistence format** — Pi's file-based sessions may change format | Low | kubepid abstracts session access; PVC ensures data durability |
| **Community/maintenance risk** — Pi is largely a single-maintainer project (Mario Zechner) | Medium | MIT license allows forking; TypeScript codebase is accessible; growing contributor base |

### Build vs Migrate Decision

This analysis considers two approaches:

**Option A: New project (KubePi)** — Build from scratch, clean API design, no legacy constraints.

**Option B: Runtime adapter in KubeOpenCode** — Add a `RuntimeAdapter` interface to KubeOpenCode, implement Pi as a second runtime alongside OpenCode.

| Criterion | Option A (New Project) | Option B (Runtime Adapter) |
|---|---|---|
| Development speed | Slower (rebuild everything) | Faster (incremental) |
| API cleanliness | Clean, Pi-native API | Compromise API serving two runtimes |
| Maintenance burden | Two separate projects | One project, more complex |
| User migration | Users must switch projects | Users configure `spec.runtime: "pi"` |
| Community perception | Clear signal, dedicated focus | More features in one project |
| Risk isolation | Pi issues don't affect KubeOpenCode | Pi issues could impact stability |

**Recommendation**: If the goal is to evaluate Pi as a runtime, **Option B (Runtime Adapter)** is more pragmatic. It introduces:

```go
// RuntimeAdapter abstracts the agent runtime specifics
type RuntimeAdapter interface {
    // Container construction
    BuildInitContainer(image string) corev1.Container
    BuildServerCommand(port int32) []string
    BuildRunCommand(serverURL, sessionTitle, workspaceDir string) []string
    
    // Health and monitoring
    GetHealthPath() string
    GetDefaultPort() int32
    
    // Configuration
    GetEnvironmentVars(config RuntimeConfig) []corev1.EnvVar
    GenerateConfigFiles(skills []string, plugins []interface{}, config json.RawMessage) (map[string]string, error)
    
    // Session management
    FindSession(ctx context.Context, serverURL, identifier string) (*RuntimeSession, error)
    GetSessionStats(ctx context.Context, serverURL, sessionID string) (*SessionStats, error)
}
```

This allows KubeOpenCode to support both runtimes without a full rewrite. The `kubepid` bridge would implement the Pi side of this interface.

## Decision

**No decision to act at this time.** This ADR is an informational feasibility analysis. Key conclusions:

1. **Pi as a runtime is technically feasible** — the kubepid HTTP-to-RPC bridge pattern solves Pi's lack of HTTP server mode while preserving KubeOpenCode's proven Kubernetes orchestration model.

2. **The migration effort is substantial but bounded** — estimated 15-20 files, 2000-3000 lines of production code. The kubepid component (~1500-2000 lines of Go) is the primary new work.

3. **Pi offers genuine architectural advantages** — extension hooks for security policies, steering API for mid-execution intervention, multi-provider OAuth support, and a growing package ecosystem.

4. **The recommended path is a RuntimeAdapter interface** — not a full rewrite or separate project. This allows incremental adoption and risk isolation.

5. **Key dependency to monitor** — Pi's RPC protocol stability. If Pi stabilizes its RPC protocol (or adopts ACP as a standard), the integration becomes significantly lower risk.

## Next Steps (if pursued)

1. **Phase 0: RuntimeAdapter interface** — Refactor KubeOpenCode internals to introduce the abstraction layer (no Pi code yet, just cleaner OpenCode integration)
2. **Phase 1: kubepid prototype** — Build the HTTP-to-RPC bridge as a standalone Go binary, validate with Pi RPC protocol
3. **Phase 2: Pi runtime implementation** — Implement `RuntimeAdapter` for Pi, build `kubeopencode-agent-pi` container image
4. **Phase 3: Feature additions** — Steering API, extension system, multi-model tasks
5. **Phase 4: Documentation and migration guide** — If Pi proves viable in production

## References

- [Pi coding agent](https://pi.dev) — Official site
- [badlogic/pi-mono](https://github.com/badlogic/pi-mono) — Pi monorepo (coding agent, LLM API, TUI, agent runtime)
- [Pi RPC protocol](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/rpc.md) — RPC mode documentation
- [Pi extensions](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/docs/extensions.md) — Extension system
- [marcfargas/pi-server](https://github.com/marcfargas/pi-server) — Detachable WebSocket server for Pi
- [svkozak/pi-acp](https://github.com/svkozak/pi-acp) — ACP adapter for Pi
- [Rivet agentOS](https://rivet.dev/agent-os) — Agent OS with first-class Pi support
- [Rivet agentOS Pi docs](https://rivet.dev/docs/agent-os/agents/pi/) — Running Pi in agentOS
- [1Panel-dev/KubePi](https://github.com/1Panel-dev/KubePi) — Existing "KubePi" project (K8s panel, 2.9K stars, name conflict)
