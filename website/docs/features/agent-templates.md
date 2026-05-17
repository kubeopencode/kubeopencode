# Agent Templates

AgentTemplate serves two purposes:

1. **Reusable base configuration for Agents**: Teams define shared settings (images, contexts, credentials) in one template. Individual users create Agents that reference it via `templateRef`.
2. **Blueprint for ephemeral tasks**: Tasks can reference a template directly via `templateRef` to run one-off, ephemeral Pods without a persistent Agent.

## Creating a Template

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: AgentTemplate
metadata:
  name: team-config
spec:
  agentImage: ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
  attachImage: ghcr.io/kubeopencode/kubeopencode-agent-attach:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  command: ["opencode", "serve"]
  contexts:
    - name: coding-standards
      type: Text
      text: "Follow team coding standards..."
  credentials:
    - name: github-token
      secretRef:
        name: shared-github-creds
        key: token
      env: GITHUB_TOKEN
  skills:
    - name: team-skills
      git:
        repository: https://github.com/my-org/ai-skills.git
        path: skills/
  plugins:
    - name: cc-safety-net
  config:
    $schema: https://opencode.ai/config.json
    model: anthropic/claude-sonnet-4-5
  caBundle:
    configMapRef:
      name: corporate-ca-bundle
      key: ca-bundle.crt
  proxy:
    httpProxy: "http://proxy.corp.example.com:8080"
    httpsProxy: "http://proxy.corp.example.com:8080"
    noProxy: "localhost,127.0.0.1,.svc,.cluster.local"
  imagePullSecrets:
    - name: my-registry-secret
  podSpec:
    labels:
      team: platform
    securityContext:
      runAsNonRoot: true
  extraPorts:
    - name: docker
      port: 2375
      targetPort: 2375
      protocol: TCP
  maxConcurrentTasks: 5
  quota:
    maxTaskStarts: 20
    windowSeconds: 3600
```

## Creating an Agent from a Template

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  templateRef:
    name: team-config
  profile: "My personal development agent"
  # Agent-only fields (not in template):
  persistence:
    sessions:
      size: "1Gi"
  standby:
    idleTimeout: "30m"
  share:
    enabled: true
    expiresAt: "2026-12-31T23:59:59Z"
```

## Running Ephemeral Tasks from a Template

Tasks can reference a template directly instead of a running Agent. This creates an ephemeral Pod that runs standalone and terminates when done — ideal for batch operations and CI/CD:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: one-off-task
spec:
  templateRef:
    name: team-config
  description: |
    Update all dependencies and run tests.
```

The Task controller creates a standalone Pod using the template's configuration. No persistent Agent is needed. Exactly one of `agentRef` or `templateRef` must be set on a Task.

## Template Fields Reference

### Shareable Fields (Available in Both AgentTemplate and Agent)

| Field | Type | Description |
|-------|------|-------------|
| `agentImage` | string | OpenCode init container image |
| `executorImage` | string | Main worker container image |
| `attachImage` | string | Lightweight image for Task Pods |
| `workspaceDir` | string | Agent working directory (default: `/workspace`) |
| `command` | []string | Command to run in the worker container |
| `serviceAccountName` | string | Kubernetes ServiceAccount |
| `contexts` | []ContextItem | Inline context definitions |
| `credentials` | []Credential | Secret mounts (env vars or files) |
| `skills` | []SkillSource | External SKILL.md from Git repos |
| `plugins` | []PluginSpec | OpenCode plugins to install and load |
| `config` | *runtime.RawExtension | Inline OpenCode configuration |
| `caBundle` | *CABundleConfig | Custom CA certificates for TLS |
| `proxy` | *ProxyConfig | HTTP/HTTPS proxy settings |
| `imagePullSecrets` | []LocalObjectReference | Private registry authentication |
| `podSpec` | *AgentPodSpec | Pod-level customization (security, scheduling, volumes) |
| `extraPorts` | []ExtraPort | Additional Service/Deployment ports |
| `maxConcurrentTasks` | *int32 | Max concurrent Tasks |
| `quota` | *QuotaConfig | Rate limiting for Task starts |

### Agent-Only Fields (Not Available in Templates)

| Field | Description |
|-------|-------------|
| `profile` | Human-readable agent summary |
| `port` | OpenCode server port (default: 4096) |
| `persistence` | Session/workspace PVCs |
| `suspend` | Manual suspend flag (scale to 0) |
| `standby` | Auto suspend/resume config |
| `share` | Shareable terminal link config |
| `templateRef` | Reference to AgentTemplate |

## Merge Behavior

When an Agent references a template with `templateRef`, fields are merged as follows:

| Field Type | Merge Behavior | Example |
|-----------|---------------|---------|
| **Scalar/pointer fields** | Agent wins if set, otherwise template value | `agentImage`, `workspaceDir`, `command`, `config`, `podSpec`, `caBundle`, `proxy` |
| **List fields** | Agent's list **replaces** the template's (not appended) | `contexts`, `credentials`, `skills`, `plugins`, `imagePullSecrets`, `extraPorts` |
| **Agent-only fields** | Always from Agent, ignored in template | `profile`, `port`, `persistence`, `suspend`, `standby`, `share` |

### Merge Examples

**Scalar override** — Agent uses its own image but inherits everything else from the template:
```yaml
# Template
spec:
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: shared-sa

# Agent
spec:
  templateRef:
    name: team-config
  executorImage: my-registry/custom-devbox:v2  # Overrides template
  # workspaceDir and serviceAccountName inherited from template
```

**List replacement** — Agent's `contexts` replace the template's entirely:
```yaml
# Template
spec:
  contexts:
    - name: org-standards
      type: Text
      text: "Follow org standards..."

# Agent
spec:
  templateRef:
    name: team-config
  contexts:  # Replaces template's contexts entirely
    - name: my-context
      type: Text
      text: "My personal context..."
  # org-standards context is NOT included
```

To include both template and Agent contexts, specify all of them in the Agent:
```yaml
spec:
  templateRef:
    name: team-config
  contexts:
    - name: org-standards      # Manually include from template
      type: Text
      text: "Follow org standards..."
    - name: my-context
      type: Text
      text: "My personal context..."
```

## Tracking

Agents using a template automatically get the label `kubeopencode.io/agent-template: <name>`,
enabling template-based queries:

```bash
# List all agents using a template
kubectl get agents -l kubeopencode.io/agent-template=team-config
```