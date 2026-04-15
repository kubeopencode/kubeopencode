---
sidebar_position: 2
title: Setting Up an Agent
description: Step-by-step guide to configure a production-ready AI agent on Kubernetes
---

# Setting Up an Agent

This guide walks you through configuring a fully functional Agent — from choosing your AI model to enabling persistence, skills, and context. By the end, you'll have a production-ready Agent running on Kubernetes.

:::tip Prerequisites
Make sure KubeOpenCode is installed on your cluster. See [Getting Started](getting-started.md) for installation instructions.
:::

## Overview

An Agent in KubeOpenCode is a persistent, running AI service on Kubernetes. Configuring one involves:

1. **Model & Provider** — Which AI model powers the agent (most important)
2. **Images** — The runtime environment (init container + executor)
3. **Workspace** — Where the agent works
4. **Features** — Persistence, standby, skills, context, and more

## Step 1: Configure the AI Model

This is the most critical step. KubeOpenCode runs [OpenCode](https://opencode.ai) under the hood, so model configuration follows the OpenCode config format. You provide this via the Agent's `config` field as an inline YAML object.

### Minimal Configuration

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  config:
    $schema: https://opencode.ai/config.json
    model: anthropic/claude-sonnet-4-5
    small_model: anthropic/claude-haiku-4-5
```

The `config` field is serialized to `/tools/opencode.json` inside the container. The `OPENCODE_CONFIG` environment variable is set automatically — you don't need to do anything else.

### Model Format

Models use the format **`provider/model-id`**:

| Provider | Example Model ID | API Key Env Var |
|----------|-----------------|-----------------|
| Anthropic | `anthropic/claude-sonnet-4-5` | `ANTHROPIC_API_KEY` |
| Google | `google/gemini-2.5-pro` | `GOOGLE_API_KEY` |
| OpenAI | `openai/gpt-5.1-codex` | `OPENAI_API_KEY` |
| DeepSeek | `deepseek/deepseek-chat` | `DEEPSEEK_API_KEY` |
| OpenCode Zen (free) | `opencode/big-pickle` | *(none required)* |

For the full list of 75+ supported providers, see the [OpenCode Providers documentation](https://opencode.ai/docs/providers).

For model selection and configuration options, see the [OpenCode Models documentation](https://opencode.ai/docs/models).

### Providing API Keys

Most providers require an API key. Use the `credentials` field to mount a Kubernetes Secret as environment variables:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ai-credentials
  namespace: test
type: Opaque
stringData:
  ANTHROPIC_API_KEY: "sk-ant-xxxxx"
---
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  config:
    $schema: https://opencode.ai/config.json
    model: anthropic/claude-sonnet-4-5
    small_model: anthropic/claude-haiku-4-5
  credentials:
    - name: anthropic
      secretRef:
        name: ai-credentials
```

When no `key` or `mountPath` is specified, all keys in the Secret are injected as environment variables. This is the simplest way to provide API keys.

### Advanced Config Options

The `config` field supports the full [OpenCode configuration schema](https://opencode.ai/config.json). Common options:

```yaml
config:
  $schema: https://opencode.ai/config.json
  model: anthropic/claude-sonnet-4-5
  small_model: anthropic/claude-haiku-4-5
  share: disabled
  autoupdate: false
  provider:
    anthropic:
      options:
        baseURL: https://your-proxy.example.com/v1
```

For full configuration reference, see the [OpenCode Config documentation](https://opencode.ai/docs/config).

## Step 2: Choose Your Images

KubeOpenCode uses a **two-container pattern**:

| Container | Field | Role |
|-----------|-------|------|
| **Init Container** | `agentImage` | Contains the OpenCode CLI binary; copies it to a shared `/tools` volume at startup |
| **Worker Container** | `executorImage` | Your development environment — runs the AI agent using `/tools/opencode` |

:::info How the two-container pattern works
At pod startup, the init container copies the OpenCode binary from `agentImage` to the shared `/tools` volume. The worker container (`executorImage`) then uses `/tools/opencode` to run the AI agent. This separation means you can update OpenCode independently from your development environment.
:::

### Available Images

| Image | Type | Description |
|-------|------|-------------|
| `opencode` | Init Container | OpenCode CLI binary |
| `devbox` | Worker (Executor) | Universal development environment with Go, Node.js, Python, kubectl, helm |
| `echo` | Testing | Minimal Alpine image for E2E testing |

### Default Images

If you don't specify images, KubeOpenCode uses these defaults:

- **OpenCode init**: `ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest`
- **Devbox executor**: `ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest`

For most users, the defaults work out of the box — you don't need to set `agentImage` or `executorImage`.

### Image Resolution

When configuring an Agent, the controller resolves images as follows:

| Configuration | Init Container | Worker Container |
|--------------|----------------|------------------|
| Both `agentImage` and `executorImage` set | `agentImage` | `executorImage` |
| Only `agentImage` set (legacy) | Default OpenCode image | `agentImage` |
| Neither set | Default OpenCode image | Default devbox image |

### Custom Executor Images

The default `devbox` image includes Go, Node.js, and Python. If your project needs a different runtime (Rust, Java, Ruby, etc.), build a custom executor image:

```dockerfile
FROM ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest

# Add your language runtime
RUN apt-get update && apt-get install -y \
    rustc cargo

# Add project-specific tools
RUN pip install your-framework
```

Then reference it in your Agent:

```yaml
spec:
  executorImage: your-registry.example.com/custom-devbox:v1.0
```

### Building Agent Images

For local development (Kind clusters):

```bash
# Build OpenCode init container
make agent-build AGENT=opencode

# Build executor containers
make agent-build AGENT=devbox

# Customize registry and version
make agent-build AGENT=devbox IMG_REGISTRY=docker.io IMG_ORG=myorg VERSION=v1.0.0
```

For remote/production clusters (OpenShift, EKS, GKE, etc.):

```bash
# Multi-arch build and push
make agent-buildx AGENT=opencode
make agent-buildx AGENT=devbox
```

For detailed guidance on building custom agent images, see the [Agent Developer Guide](https://github.com/kubeopencode/kubeopencode/tree/main/agents).

## Step 3: Set the Workspace Directory

The `workspaceDir` field defines where the agent works inside the container. This is where code gets cloned, files get created, and the agent does its work.

```yaml
spec:
  workspaceDir: /workspace
```

**Relationship with Context:** When you add [Git contexts](#context), they are cloned into subdirectories of `workspaceDir`. For example, a Git context with `mountPath: my-repo` will be available at `/workspace/my-repo`.

**Relationship with Persistence:** If you enable [workspace persistence](#persistence), the `workspaceDir` is backed by a PersistentVolumeClaim. Files in this directory survive pod restarts.

## Step 4: Enable Agent Features

With model, images, and workspace configured, your Agent is ready to run. The following features are optional but highly recommended for production use.

### Persistence

By default, Agent pods use ephemeral storage (EmptyDir) — everything is lost on restart. Enable persistence to preserve:

- **Sessions** — Conversation history (OpenCode's SQLite database)
- **Workspace** — Cloned repos, generated files, build artifacts

```yaml
spec:
  persistence:
    sessions:
      size: "1Gi"       # Conversation history survives restarts
    workspace:
      size: "10Gi"      # Workspace files survive restarts
```

Both fields are optional — you can persist sessions only, workspace only, or both. The controller automatically creates a PersistentVolumeClaim (PVC) for each, using your cluster's default StorageClass. To use a specific StorageClass, set `storageClassName`:

```yaml
spec:
  persistence:
    workspace:
      storageClassName: "fast-ssd"
      size: "10Gi"
```

See [Persistence & Lifecycle](features/persistence.md) for the full explanation of how Kubernetes storage works with KubeOpenCode.

### Standby (Auto Suspend/Resume)

Save cluster resources by automatically suspending idle Agents:

```yaml
spec:
  standby:
    idleTimeout: "30m"   # Suspend after 30 minutes with no active Tasks
```

When a new Task arrives, the Agent resumes automatically. Active web terminal or CLI sessions prevent auto-suspend.

You can also manually suspend/resume an Agent:

```bash
# Suspend
kubectl patch agent my-agent --type merge -p '{"spec":{"suspend":true}}'
# Resume
kubectl patch agent my-agent --type merge -p '{"spec":{"suspend":false}}'
```

See [Persistence & Lifecycle — Suspend/Resume](features/persistence.md#suspendresume) for details.

### Context

Contexts provide knowledge to the agent — codebases, documentation, configuration. KubeOpenCode supports multiple context types:

```yaml
spec:
  contexts:
    # Clone a Git repository into the workspace
    - name: my-codebase
      description: "Main application codebase"
      type: Git
      git:
        repository: https://github.com/your-org/your-repo.git
        ref: main
      mountPath: code/         # Available at /workspace/code/

    # Inline text context (written to .kubeopencode/context.md)
    - name: coding-standards
      description: "Team coding guidelines"
      type: Text
      text: |
        Always use TypeScript strict mode.
        Follow the repository's existing patterns.

    # Reference a ConfigMap
    - name: project-config
      description: "Shared project configuration"
      type: ConfigMap
      configMap:
        name: project-docs
        key: guidelines.md
```

**Context vs Workspace:** Contexts are *inputs* — they provide information the agent needs. The workspace is the *working area* — it's where the agent reads contexts and creates output.

Git contexts support **auto-sync** to keep code up-to-date without pod restarts:

```yaml
contexts:
  - name: my-repo
    type: Git
    git:
      repository: https://github.com/your-org/repo.git
      ref: main
      sync:
        enabled: true
        interval: "5m"
        policy: HotReload    # Update in-place, no restart
    mountPath: repo/
```

See [Flexible Context System](features/context-system.md) for all context types and examples.

### Skills

Skills are reusable AI capabilities defined as `SKILL.md` files. They teach the agent *how to do things* (as opposed to contexts, which tell the agent *what it knows*).

```yaml
spec:
  skills:
    - name: community-skills
      git:
        repository: https://github.com/anthropics/skills.git
        ref: main
        path: skills/
        names:              # Optional: pick specific skills
          - frontend-design
          - skill-creator
```

KubeOpenCode automatically clones the skill repos and injects the paths into the OpenCode config. See [Skills](features/skills.md) for private repos and advanced usage.

### Plugins

Plugins extend Agent behavior at a deeper level — custom tools, auth providers, message interception, event handling, and more. They are OpenCode TypeScript plugins declared on the Agent spec. The controller creates a plugin-init container that runs `npm install` to download plugins into a shared volume — the executor container loads them without needing npm:

```yaml
spec:
  plugins:
    - name: "cc-safety-net"
    - name: "@nicholasgriffintn/opencode-plugin-otel"
      options:
        endpoint: "http://otel-collector:4318"
```

The controller installs plugins to `/plugins/node_modules/` and merges `file://` paths into the OpenCode `plugin` array. Plugin credentials are provided via `spec.credentials`. See [Plugins](features/plugins.md) for options, templates, and field reference.

### Concurrency & Quota

Control how many Tasks can run simultaneously and limit task creation rate:

```yaml
spec:
  maxConcurrentTasks: 3      # Max 3 tasks running at once
  quota:
    maxTaskStarts: 20        # Max 20 tasks per hour
    windowSeconds: 3600
```

See [Concurrency & Quota](features/concurrency-quota.md) for details.

## Complete Example

Here's a production-ready Agent combining all the key features:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ai-credentials
  namespace: my-team
type: Opaque
stringData:
  ANTHROPIC_API_KEY: "sk-ant-xxxxx"
---
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: team-agent
  namespace: my-team
spec:
  # Profile: human-readable description (informational only)
  profile: "Full-stack development agent for the platform team"

  # Step 1: AI Model Configuration
  config:
    $schema: https://opencode.ai/config.json
    model: anthropic/claude-sonnet-4-5
    small_model: anthropic/claude-haiku-4-5
    share: disabled
    autoupdate: false
  credentials:
    - name: anthropic
      secretRef:
        name: ai-credentials

  # Step 2: Images (optional — defaults work for most cases)
  # agentImage: ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest
  # executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest

  # Step 3: Workspace
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent

  # Step 4: Features
  persistence:
    sessions:
      size: "1Gi"
    workspace:
      size: "10Gi"

  standby:
    idleTimeout: "30m"

  maxConcurrentTasks: 3

  contexts:
    - name: platform-repo
      description: "Platform service codebase"
      type: Git
      git:
        repository: https://github.com/your-org/platform.git
        ref: main
        sync:
          enabled: true
          interval: "5m"
          policy: HotReload
      mountPath: platform/

  skills:
    - name: team-skills
      git:
        repository: https://github.com/your-org/ai-skills.git
        ref: main
        path: skills/
```

## Using AgentTemplate for Team Sharing

If multiple Agents share common configuration (images, credentials, model), extract it into an AgentTemplate:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: AgentTemplate
metadata:
  name: team-base
  namespace: my-team
spec:
  agentImage: ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  config:
    $schema: https://opencode.ai/config.json
    model: anthropic/claude-sonnet-4-5
    small_model: anthropic/claude-haiku-4-5
  credentials:
    - name: anthropic
      secretRef:
        name: ai-credentials
---
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: frontend-agent
  namespace: my-team
spec:
  templateRef:
    name: team-base
  profile: "Frontend development specialist"
  # Override or extend template settings
  contexts:
    - name: frontend-repo
      type: Git
      git:
        repository: https://github.com/your-org/frontend.git
        ref: main
      mountPath: frontend/
  persistence:
    sessions:
      size: "1Gi"
  standby:
    idleTimeout: "1h"
```

Agent fields override template fields for scalars. For lists (contexts, credentials), the Agent **replaces** the template list entirely.

See [Agent Templates](features/agent-templates.md) for merge behavior details.

## Submitting Tasks to Your Agent

Once your Agent is running, submit Tasks to it:

```bash
kubectl apply -n my-team -f - <<EOF
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: fix-login-bug
spec:
  agentRef:
    name: team-agent
  description: "Fix the login timeout bug in src/auth/handler.go"
EOF

# Watch progress
kubectl get task -n my-team -w
```

## Further Reading: Repo-as-Agent Pattern

The KubeOpenCode team's own agent — [kubeopencode-agent](https://github.com/kubeopencode/kubeopencode-agent) — follows the **Repo-as-Agent** pattern: a dedicated Git repository that *is* the agent, containing its identity, cross-repo context, team knowledge, skills, and scheduled workflows. The Agent you configure with KubeOpenCode is designed to work this way.

Read more about this pattern and how we built it: [Whatever You Want to Build, Build an Agent First](/blog/repo-as-agent).

## Next Steps

- [Features](features/index.md) — Deep dive into all Agent capabilities
- [Security](security.md) — RBAC, credential management, and network proxy
- [Architecture](architecture.md) — System design and complete API reference
