# OpenCode Ecosystem Research — KubeOpenCode Integration Opportunities

> **Date**: 2026-03-26 | **Last verified**: 2026-03-26
>
> This document surveys OpenCode's official components, community plugins, and the MCP server ecosystem to identify integration opportunities for KubeOpenCode.

## 1. OpenCode Official Components

### 1.1 Current Utilization

| Component | Description | KubeOpenCode Status |
|-----------|-------------|---------------------|
| `opencode run` | Non-interactive task execution | **Used** — Pod Mode core |
| `opencode serve` | Headless HTTP server | **Used** — Server Mode core |
| `opencode run --attach` | Connect to running server | **Used** — Server Mode Task attach |
| Plugin System | JS/TS plugins with 10+ hook points | **Not used** |
| Custom Tools | `.opencode/tool/*.ts` custom tools | **Not used** |
| Custom Agents | `.opencode/agent/*.md` custom agents | **Not used** |
| Custom Commands | `.opencode/command/*.md` template commands | **Not used** |
| Skills System | `SKILL.md` reusable instruction sets | **Not used** |
| MCP Integration | Local/remote MCP servers | **Not used** (passable via Agent.spec.config) |
| LSP Support | 30+ language code intelligence | **Not used** (devbox has no LSP servers) |
| Official SDKs | JS/Go/Python SDKs | **Not used** |
| Session Sharing | Shareable session links | **Not used** |

### 1.2 Plugin System

OpenCode plugins are async JS/TS functions that receive `PluginInput` and return `Hooks`.

**Available Hooks:**

- `tool` — Define custom tools (Zod schema parameter validation)
- `auth` — OAuth / API Key authentication
- `chat.message` — Triggered on new message arrival
- `chat.params` — Modify LLM temperature/topP/topK
- `chat.headers` — Add custom HTTP headers
- `experimental.chat.messages.transform` — Transform messages before sending
- `experimental.chat.system.transform` — Modify system prompt
- `tool.execute.before` — Intercept/modify args before tool execution
- `tool.execute.after` — Post-process tool output
- `tool.definition` — Modify tool descriptions
- `permission.ask` — Custom permission request handling
- `command.execute.before` — Intercept command execution
- `shell.env` — Customize shell environment variables
- `event` — Subscribe to all bus events

**Loading methods:**

- NPM packages: `"@my-org/my-plugin@1.0.0"`
- Local files: `"file:///path/to/plugin.ts"`

### 1.3 Built-in Tools (14)

`bash`, `edit`, `write`, `read`, `grep`, `glob`, `list`, `lsp` (experimental), `patch`, `skill`, `todowrite`, `webfetch`, `websearch`, `question`

### 1.4 Built-in Agents

| Agent | Mode | Purpose |
|-------|------|---------|
| `build` | primary | Default full-access development agent |
| `plan` | primary | Read-only analysis/planning agent |
| `general` | subagent | Complex multi-step parallel tasks |
| `explore` | subagent | Fast codebase exploration |
| `compaction` | internal | Session compaction |
| `title` | internal | Generate session titles |
| `summary` | internal | Create summaries |

### 1.5 Provider Support (75+)

Anthropic, OpenAI, Google, Amazon Bedrock, Azure OpenAI, GitHub Copilot, GitLab Duo, OpenRouter, Together AI, Groq, DeepSeek, Fireworks AI, xAI, Ollama, LM Studio, llama.cpp, and any OpenAI-compatible endpoint.

### 1.6 LSP Support (30+ Languages)

The devbox image has Go/Node.js/Python runtimes but **no LSP servers installed**:

| Language | LSP Server | Install Command | Image Size Impact |
|----------|-----------|-----------------|-------------------|
| Go | `gopls` | `go install golang.org/x/tools/gopls@latest` | ~50MB |
| TypeScript/JS | `typescript-language-server` | `npm install -g typescript-language-server typescript` | ~30MB |
| Python | `pylsp` | `pip install python-lsp-server` | ~20MB |

OpenCode auto-detects file extensions and starts the corresponding LSP server, providing go-to-definition, find-references, diagnostics (compile/type errors fed back to LLM), and hover type information.

### 1.7 Server API Endpoints (`opencode serve`)

| Endpoint | Operations |
|----------|------------|
| `/config` | GET/PATCH configuration |
| `/session` | CRUD sessions, status, messages |
| `/project` | List/get/update projects |
| `/provider` | List providers and models |
| `/agent` | List agents |
| `/file` | Read/write/watch files |
| `/mcp` | Manage MCP servers |
| `/permission` | Permission requests/decisions |
| `/pty` | Terminal sessions |
| `/question` | User Q&A |

### 1.8 Config Schema Key Fields

```jsonc
{
  "model": "anthropic/claude-opus",
  "small_model": "anthropic/claude-haiku",
  "default_agent": "build",
  "agent": { /* custom agent configs */ },
  "command": { /* custom commands */ },
  "plugin": ["@org/plugin@1.0.0"],
  "provider": { /* LLM provider configs */ },
  "mcp": { /* MCP server configs */ },
  "permission": { /* permission rules */ },
  "lsp": { /* LSP server configs */ },
  "skills": { "paths": [], "urls": [] },
  "snapshot": true,
  "compaction": { "auto": true, "prune": true },
  "experimental": { "batch_tool": false }
}
```

### 1.9 `.opencode/` Directory Structure (Extension Points)

```
.opencode/
├── opencode.jsonc         # Configuration
├── agent/                 # Custom agents (.md frontmatter)
├── command/               # Custom command templates (.md)
├── tool/                  # Custom tools (.ts/.js)
├── skill/                 # Skill definitions (SKILL.md)
└── glossary/              # Glossaries
```

### 1.10 Official SDKs

- `@opencode-ai/sdk` (JavaScript/TypeScript) — npm package
- `opencode-sdk-go` (Go)
- `opencode-sdk-python` (Python)

---

## 2. Community Plugins (60+)

### 2.1 Tier 1 — High Value, Directly Integrable

| Plugin | Function | Integration Approach |
|--------|----------|---------------------|
| **opencode-plugin-otel** | OpenTelemetry telemetry | Pre-install in devbox image; Tasks auto-gain observability, feed into Jaeger/Tempo |
| **cc-safety-net** | Intercepts dangerous git/filesystem commands | Cluster environments need stronger safety; pre-install in Agent image |
| **opencode-quota** | Token/quota usage tracking | Link with Agent's `quota` field; LLM token-level cost tracking |
| **dynamic-context-pruning** | Optimize token consumption | Save costs in long-running task scenarios |
| **morph-fast-apply** | 10,500+ tokens/sec code editing | Accelerate large file edits; cluster tasks often involve large codebases |

### 2.2 Tier 2 — Scenario-Based Integration

| Plugin | Function | Integration Approach |
|--------|----------|---------------------|
| **agent-memory** | Self-editable memory blocks (Letta-style) | Server Mode persistent Agent retains memory across Tasks |
| **background-agents** | Async agent delegation | Parallelize sub-tasks within a Task |
| **opencode-worktree** | Git worktree management | Avoid conflicts when multiple Tasks concurrently operate on the same repo |
| **envsitter-guard** | Prevent .env leaks | Essential for security compliance |
| **pilot** | Poll GitHub Issues/Linear tickets | DevOps automation (but we already have Argo Events) |

### 2.3 Tier 3 — Nice to Have

| Plugin | Function | Notes |
|--------|----------|-------|
| **opencode-snip** | Reduce 60-90% token consumption | Need to evaluate output quality impact |
| **opencode-daytona** | Isolated sandbox | KubeOpenCode already provides Pod isolation |
| **micode** | Brainstorm-Plan-Implement workflow | Could be provided as a Skill template |

---

## 3. MCP Server Ecosystem

### 3.1 Kubernetes Ecosystem (Highest Priority)

| MCP Server | Stars | Language | Key Features |
|------------|-------|----------|--------------|
| **containers/kubernetes-mcp-server** | 1.3k | Go | **Best choice** — native Go, OpenShift support, no kubectl dependency |
| **argoproj-labs/mcp-for-argocd** | 378 | TS | Complements our Argo Events ecosystem; Agent directly manages ArgoCD apps |
| **kagent-dev/kmcp** | 443 | Go | CRD-based MCP server deployment; aligns with KubeOpenCode philosophy |
| **Flux159/mcp-server-kubernetes** | 1.4k | TS | Most feature-complete, but requires Node runtime |
| **strowk/mcp-k8s-go** | 377 | Go | Lightweight K8s MCP server |

### 3.2 Infrastructure MCP Servers

| MCP Server | Stars | Description |
|------------|-------|-------------|
| **agentgateway/agentgateway** | 2.2k | Agentic proxy (Rust) |
| **stacklok/toolhive** | 1.7k | Enterprise MCP server management platform |
| **microsoft/mcp-gateway** | 547 | MCP reverse proxy |

### 3.3 Official Reference MCP Servers

| Server | Purpose |
|--------|---------|
| `filesystem` | Secure file operations |
| `git` | Git repository operations |
| `memory` | Knowledge-graph persistent memory |
| `fetch` | Web content retrieval |
| `sequential-thinking` | Structured reasoning |

### 3.4 Dev Tool MCP Servers

| Server | Integration Value |
|--------|-------------------|
| PostgreSQL / Redis | Database operation scenarios |
| Sentry | Error tracking — Agent auto-fixes bugs |
| Slack / Jira | DevOps workflow integration |
| Apollo GraphQL | API development |

---

## 4. Brainstorm: Integration Opportunities

### A. KubeOpenCode MCP Server — Agent Autonomy

Create a KubeOpenCode-specific MCP server enabling Agents to:

- View status of other Tasks in the cluster
- Create sub-Tasks (Agent delegates work to other Agents)
- View Agent list and quotas
- Read other Tasks' output

```yaml
config:
  mcp:
    kubeopencode:
      type: local
      command:
        - /tools/kubeopencode-mcp
      environment:
        KUBERNETES_SERVICE_HOST: "..."
        NAMESPACE: kubeopencode-system
```

**Value**: Agent-to-Agent orchestration without external workflow engines.

### B. MCP Sidecar Injection — Declarative Tool Capabilities

Add `mcpServers` field to the Agent CRD:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: devops-agent
spec:
  mcpServers:
    - name: kubernetes
      image: ghcr.io/containers/kubernetes-mcp-server:latest
    - name: argocd
      image: ghcr.io/argoproj-labs/mcp-for-argocd:latest
      env:
        - name: ARGOCD_SERVER
          value: argocd-server.argocd.svc:443
```

Controller auto-injects MCP servers as sidecar containers into Task Pods and generates OpenCode MCP configuration.

**Value**: Users don't need to understand MCP configuration; declaratively manage Agent tool capabilities.

### C. OpenCode Plugin Pre-installation

Agent CRD declares required plugins:

```yaml
spec:
  plugins:
    - "@opencode-ai/plugin-otel@latest"
    - "@opencode-ai/plugin-safety-net@latest"
    - "file:///tools/plugins/kubeopencode-plugin.js"
```

Controller installs plugins in init container and injects into OpenCode config.

### D. SkillSet CRD — Organizational AI Instruction Library

New CRD `SkillSet` manages Skills as ConfigMaps:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: SkillSet
metadata:
  name: devops-skills
spec:
  skills:
    - name: k8s-troubleshoot
      content: |
        ---
        name: k8s-troubleshoot
        description: Troubleshoot Kubernetes issues
        ---
        # Steps: check pods, events, logs, describe...
```

Agent references SkillSet; Skills auto-mount to `.opencode/skill/`.

### E. Go SDK Deep Integration

Use `opencode-sdk-go` to replace HTTP proxy approach for Server Mode Agent communication:

- Stronger type safety
- Better error handling
- Session management (create, resume, list)
- Direct streaming of Agent output

### F. Structured Task Results

Inject a custom OpenCode Tool `report` for structured JSON task output:

```typescript
// .opencode/tool/report.ts
export default {
  name: "report",
  description: "Submit structured task result",
  parameters: z.object({
    status: z.enum(["success", "failure", "partial"]),
    summary: z.string(),
    files_changed: z.array(z.string()),
    pr_url: z.string().optional(),
  }),
  execute: async (args) => {
    fs.writeFileSync("/workspace/.kubeopencode/result.json", JSON.stringify(args));
    return { output: "Result recorded" };
  }
}
```

Controller reads `result.json` and writes to Task status — **machine-readable task output**.

### G. OTel Observability Pipeline

Combine with `opencode-plugin-otel`:

- LLM API call count, latency, token consumption
- Tool execution count and duration
- Total task duration and cost
- Expose as Prometheus metrics or send to OTLP collector

---

## 5. Priority Ranking

| Priority | Integration Item | Effort | Value | Notes |
|----------|-----------------|--------|-------|-------|
| **P0** | MCP Sidecar Injection (B) | Medium | Very High | Differentiating feature; declarative tool capabilities |
| **P0** | KubeOpenCode MCP Server (A) | Medium | Very High | Agent autonomy; Agent-to-Agent orchestration |
| **P1** | Structured Task Results (F) | Small | High | Machine-readable output; workflow integration foundation |
| **P1** | Safety Net Plugin Pre-install | Small | High | Security compliance |
| **P1** | OTel Observability (G) | Medium | High | Enterprise requirement |
| **P1** | LSP Servers Installation | Small | High | Three LSP servers ~100MB; significantly improves code intelligence |
| **P2** | Go SDK Integration (E) | Medium | Medium | Code quality improvement |
| **P2** | SkillSet CRD (D) | Medium | Medium | Organizational reuse |
| **P2** | Plugin Pre-install Mechanism (C) | Small | Medium | Flexibility |
| **P3** | kubernetes-mcp-server Integration | Small | Depends | Links with approach B |
| **P3** | Agent Memory Integration | Medium | Medium | Server Mode specific |

---

## 6. Key Conclusions

KubeOpenCode currently only leverages OpenCode's most basic `opencode run` and `opencode serve` capabilities. OpenCode's five major extension systems — **Plugin System, Custom Tools, MCP, Skills, Custom Agents/Commands** — are entirely untapped.

The biggest opportunities are **MCP Sidecar Injection** (B) and **KubeOpenCode MCP Server** (A) — the former gives Agents declarative tool capabilities, the latter gives Agents cluster autonomy. Together they enable truly **Agentic Kubernetes**.
