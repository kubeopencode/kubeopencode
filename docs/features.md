# KubeOpenCode Features

This document covers the key features of KubeOpenCode.

## Flexible Context System

Tasks and Agents use inline **ContextItem** to provide additional context to AI agents.

### Context Types

- **Text**: Inline text content
- **ConfigMap**: Content from ConfigMap
- **Git**: Content from Git repository
- **Runtime**: KubeOpenCode platform awareness system prompt
- **URL**: Content fetched from remote HTTP/HTTPS URL

### Example

```yaml
contexts:
  - type: Text
    text: |
      # Rules for AI Agent
      Always use signed commits...
  - type: ConfigMap
    configMap:
      name: my-scripts
    mountPath: .scripts
    fileMode: 493  # 0755 in decimal
  - type: Git
    git:
      repository: https://github.com/org/repo.git
      ref: main
    mountPath: source-code
  - type: URL
    url:
      source: https://api.example.com/openapi.yaml
    mountPath: specs/openapi.yaml
```

### Content Aggregation

Contexts without `mountPath` are written to `.kubeopencode/context.md` with XML tags. OpenCode loads this via `OPENCODE_CONFIG_CONTENT`, preserving any existing `AGENTS.md` in the repository.

## Agent Configuration

Agent centralizes execution environment configuration:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: default
spec:
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent

  # Default contexts for all tasks (inline ContextItems)
  contexts:
    - type: Text
      text: |
        # Organization Standards
        - Use signed commits
        - Follow Go conventions

  # Credentials (secrets as env vars or file mounts)
  credentials:
    - name: github-token
      secretRef:
        name: github-creds
        key: token
      env: GITHUB_TOKEN

    - name: ssh-key
      secretRef:
        name: ssh-keys
        key: id_rsa
      mountPath: /home/agent/.ssh/id_rsa
      fileMode: 0400
```

## OpenCode Configuration

The `config` field allows you to provide OpenCode configuration as an inline JSON string:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: opencode-agent
spec:
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  config: |
    {
      "$schema": "https://opencode.ai/config.json",
      "model": "google/gemini-2.5-pro",
      "small_model": "google/gemini-2.5-flash"
    }
```

The configuration is written to `/tools/opencode.json` and the `OPENCODE_CONFIG` environment variable is set automatically. See [OpenCode configuration schema](https://opencode.ai/config.json) for available options.

## Multi-AI Support

Use different Agents with different executorImages for various use cases:

```yaml
# Standard OpenCode agent with devbox
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: opencode-devbox
spec:
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
---
# Task using specific agent
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: task-with-opencode
spec:
  agentRef:
    name: opencode-devbox
  description: "Update dependencies and create a PR"
```

## Task Stop

Stop a running task using the stop annotation:

```bash
kubectl annotate task my-task kubeopencode.io/stop=true
```

When this annotation is detected:
- The controller deletes the Pod (with graceful termination period)
- Task status is set to `Completed` with a `Stopped` condition
- The `Stopped` condition has reason `UserStopped`

**Note:** Logs are lost when a Task is stopped. For log persistence, use an external log aggregation system.

## Concurrency Control

Limit concurrent tasks per Agent when using rate-limited AI services:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: rate-limited-agent
spec:
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  maxConcurrentTasks: 3  # Only 3 Tasks can run at once
```

When the limit is reached:
- New Tasks enter `Queued` phase instead of `Running`
- Queued Tasks automatically transition to `Running` when capacity becomes available
- Tasks are processed in approximate FIFO order

## Quota (Rate Limiting)

In addition to `maxConcurrentTasks` (which limits simultaneous running Tasks), you can configure `quota` to limit the rate at which Tasks can start using a sliding time window:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: rate-limited-agent
spec:
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  quota:
    maxTaskStarts: 10     # Maximum 10 task starts
    windowSeconds: 3600   # Per hour (sliding window)
```

### Quota vs MaxConcurrentTasks

| Feature | `maxConcurrentTasks` | `quota` |
|---------|----------------------|---------|
| What it limits | Simultaneous running Tasks | Rate of new Task starts |
| Time component | No (instant check) | Yes (sliding window) |
| Queued Reason | `AgentAtCapacity` | `QuotaExceeded` |
| Use case | Limit resource usage | API rate limiting |

Both can be used together for comprehensive control. When quota is exceeded, new Tasks enter `Queued` phase with reason `QuotaExceeded`.

## Cross-Namespace Task/Agent Separation

Enable separation of concerns between platform and dev teams:

```
┌─────────────────────────────────────────────────────────────────┐
│                    Task Namespace (dev-team-a)                  │
│  ┌──────────┐                                                   │
│  │   Task   │  agentRef:                                        │
│  │          │    name: opencode-agent                           │
│  │          │    namespace: platform-agents                     │
│  └──────────┘                                                   │
└─────────────────────────────────────────────────────────────────┘
        │ references
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Agent Namespace (platform-agents)              │
│  ┌──────────┐     ┌──────────┐     ┌──────────┐                │
│  │  Agent   │     │ Secrets  │     │   Pod    │◄── runs here   │
│  │          │     │(API keys)│     │          │                │
│  └──────────┘     └──────────┘     └──────────┘                │
└─────────────────────────────────────────────────────────────────┘
```

### Example

```yaml
# In platform-agents namespace - managed by Infra team
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: opencode-agent
  namespace: platform-agents
spec:
  # Restrict which namespaces can use this Agent
  allowedNamespaces:
    - "dev-*"
    - "staging"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  serviceAccountName: kubeopencode-agent
  workspaceDir: /workspace
  credentials:
    - name: anthropic-key
      secretRef:
        name: anthropic-credentials
        key: api-key
      env: ANTHROPIC_API_KEY
---
# In dev-team-a namespace - created by Dev team
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: fix-bug-123
  namespace: dev-team-a
spec:
  agentRef:
    name: opencode-agent
    namespace: platform-agents
  description: "Fix the bug in authentication module"
```

### Result

- Task exists in `dev-team-a` namespace
- Pod runs in `platform-agents` namespace (where Agent lives)
- Credentials stay in `platform-agents` - never exposed to dev team
- Task status shows `podNamespace: platform-agents`

### AllowedNamespaces Patterns

`allowedNamespaces` supports glob patterns:
- `"dev-*"` - matches `dev-team-a`, `dev-team-b`, etc.
- `"staging"` - exact match
- Empty list (default) - all namespaces allowed

## Pod Configuration

Configure advanced Pod settings using `podSpec`:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: advanced-agent
spec:
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  podSpec:
    # Labels for NetworkPolicy, monitoring, etc.
    labels:
      network-policy: agent-restricted
    # Enhanced isolation with gVisor or Kata
    runtimeClassName: gvisor
    # Scheduling configuration
    scheduling:
      nodeSelector:
        node-type: ai-workload
      tolerations:
        - key: "dedicated"
          operator: "Equal"
          value: "ai-workload"
          effect: "NoSchedule"
```

## Next Steps

- [Getting Started](getting-started.md) - Installation and basic usage
- [Agent Images](agent-images.md) - Build custom agent images
- [Security](security.md) - RBAC, credential management, and best practices
- [Architecture](architecture.md) - System design and API reference
