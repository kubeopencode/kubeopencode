# KubeOpenCode Features

This document covers the key features of KubeOpenCode.

## Live Agents

Every Agent in KubeOpenCode is a persistent, running service on Kubernetes — available for interactive use anytime.

### Why Live Agents?

- **Zero cold start**: The agent is always running. No waiting for container startup when you need help.
- **Shared context**: Pre-load codebases, documentation, and organizational standards. All tasks share the same context.
- **Interactive access**: Connect via web terminal or CLI for real-time pair programming.
- **Session persistence**: Conversation history survives pod restarts (crashes, node drains, upgrades).
- **Team-shared agents**: One agent serves your entire team — consistent configuration, centralized credential management.

### Use Cases

| Use Case | Description |
|----------|-------------|
| **Team coding assistant** | A shared agent pre-loaded with your monorepo and coding standards. Team members attach via CLI to get interactive help. |
| **Slack/ChatOps bot** | An always-on agent that responds to Slack messages, creating PRs and fixing issues on demand. |
| **Code review agent** | A persistent agent that reviews PRs as they come in, leveraging shared context about your codebase. |
| **On-call assistant** | An agent with production runbooks and monitoring dashboards pre-loaded, ready to help debug incidents. |

### Setup

Creating an Agent automatically creates a persistent Deployment + Service:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: team-agent
spec:
  profile: "Team development agent"
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  port: 4096                      # OpenCode server port (default: 4096)
  persistence:
    sessions:
      size: "2Gi"                 # Persist conversation history
  contexts:
    - name: codebase
      type: Git
      git:
        repository: https://github.com/your-org/your-repo.git
        ref: main
      mountPath: code
  credentials:
    - name: api-key
      secretRef:
        name: ai-credentials
        key: api-key
      env: OPENCODE_API_KEY
```

The controller automatically creates:
- A **Deployment** running `opencode serve` (persistent server)
- A **Service** for in-cluster access (e.g., `http://team-agent.kubeopencode-system.svc.cluster.local:4096`)

### Interacting with Live Agents

**CLI attach** (connects through kube-apiserver service proxy — no Ingress or port-forward needed):

```bash
kubeoc agent attach team-agent -n kubeopencode-system
```

**Web Terminal**: Access the agent's OpenCode TUI directly from the KubeOpenCode dashboard at `http://localhost:2746`.

**Programmatic Tasks**: Submit Tasks referencing the Agent — they run on the persistent server via `--attach` flag:

```bash
kubectl apply -f task.yaml
```

### Agent vs Template Tasks

| Aspect | `agentRef` (Agent) | `templateRef` (AgentTemplate) |
|--------|-------------------|-------------------------------|
| Lifecycle | Persistent Deployment + lightweight Pod per Task | Ephemeral Pod per Task |
| Command | `opencode run --attach <url> "task"` | `opencode run "task"` |
| Cold start | No (server already running) | Yes (container startup) |
| Context sharing | Shared across Tasks via server | Isolated per Task |
| Interaction | Web Terminal, CLI attach, API | Logs only |
| Best for | Interactive coding, team agents | Batch operations, CI/CD, one-off tasks |
| Concurrency/Quota | Enforced by Agent | Not enforced |

### Agent Status

Monitor your live agent's health:

```bash
kubectl get agent team-agent -o wide
# NAME         PROFILE                  STATUS
# team-agent   Team development agent   Ready
```

The Agent status includes deployment details:
```yaml
status:
  deploymentName: team-agent-server
  serviceName: team-agent
  url: http://team-agent.kubeopencode-system.svc.cluster.local:4096
  ready: true
  conditions:
    - type: ServerReady
      status: "True"
    - type: ServerHealthy
      status: "True"
```

See [Getting Started](getting-started.md) for a complete walkthrough.

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
  - name: private-repo
    type: Git
    git:
      repository: https://github.com/org/private-repo.git
      ref: main
      secretRef:
        name: github-git-credentials  # Secret with username + password (PAT)
    mountPath: private-source
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
  profile: "Default development agent with org standards and GitHub access"
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

## Custom CA Certificates

When accessing private Git servers or internal HTTPS services that use self-signed or private CA certificates, configure `caBundle` on the Agent to mount custom CA certificates into all containers.

### ConfigMap Example (trust-manager Compatible)

If you use [cert-manager trust-manager](https://cert-manager.io/docs/trust/trust-manager/), it can automatically populate a ConfigMap with your organization's CA bundle. KubeOpenCode's default key (`ca-bundle.crt`) matches trust-manager's convention.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: internal-agent
spec:
  profile: "Agent with custom CA for internal services"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  caBundle:
    configMapRef:
      name: custom-ca-bundle       # ConfigMap containing the CA certificate
      key: ca-bundle.crt           # Optional, defaults to "ca-bundle.crt"
```

### Secret Example

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: internal-agent
spec:
  profile: "Agent with custom CA from Secret"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  caBundle:
    secretRef:
      name: custom-ca-secret       # Secret containing the CA certificate
      key: ca.crt                  # Optional, defaults to "ca.crt"
```

### How It Works

- The CA certificate is mounted at `/etc/ssl/certs/custom-ca/tls.crt` in **all** containers (init containers and the worker container)
- The `CUSTOM_CA_CERT_PATH` environment variable is set in all containers
- **git-init**: Concatenates the custom CA with system CAs and sets `GIT_SSL_CAINFO` so `git clone` trusts the private server
- **url-fetch**: Appends the custom CA to Go's x509 system certificate pool for HTTPS URL fetching

This is the recommended approach for private HTTPS servers. Avoid disabling TLS verification (`InsecureSkipTLSVerify`) in favor of proper CA bundle configuration.

### Multiple CA Certificates

The `caBundle` field accepts a single ConfigMap or Secret reference, but PEM format supports multiple certificates in one file. To trust multiple CAs, concatenate them into a single bundle:

```bash
# Combine multiple CA certificates into one PEM bundle
cat internal-ca.crt partner-ca.crt > combined-ca-bundle.crt
kubectl create configmap custom-ca-bundle --from-file=ca-bundle.crt=combined-ca-bundle.crt
```

If you use [cert-manager trust-manager](https://cert-manager.io/docs/trust/trust-manager/), it handles multi-source aggregation automatically:

```yaml
apiVersion: trust.cert-manager.io/v1alpha1
kind: Bundle
metadata:
  name: custom-ca-bundle
spec:
  sources:
    - useDefaultCAs: true          # Include public CAs
    - secret:
        name: internal-ca
        key: ca.crt
    - configMap:
        name: partner-ca
        key: ca-bundle.crt
  target:
    configMap:
      key: ca-bundle.crt           # Matches KubeOpenCode's default key
```

> **Note**: `git-init` automatically concatenates the custom CA bundle with the container's system CAs, so public HTTPS (e.g., github.com) continues working even when `caBundle` is configured. You do not need to include public CAs in your bundle unless you want to explicitly control the full trust chain.

## HTTP/HTTPS Proxy Configuration

Enterprise networks often require all outbound traffic to pass through a corporate proxy server. KubeOpenCode supports proxy configuration at both the Agent level and the cluster level via `KubeOpenCodeConfig`.

### Agent-Level Proxy

Configure proxy settings directly on an Agent. These settings are injected as environment variables into all init containers and the worker container.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: enterprise-agent
spec:
  profile: "Agent with corporate proxy configuration"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  proxy:
    httpProxy: "http://proxy.corp.example.com:8080"
    httpsProxy: "http://proxy.corp.example.com:8080"
    noProxy: "localhost,127.0.0.1,10.0.0.0/8,.corp.example.com"
```

### Cluster-Level Proxy

For organizations where all agents should use the same proxy, configure it once in `KubeOpenCodeConfig`:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeConfig
metadata:
  name: cluster
spec:
  proxy:
    httpProxy: "http://proxy.corp.example.com:8080"
    httpsProxy: "http://proxy.corp.example.com:8080"
    noProxy: "localhost,127.0.0.1,10.0.0.0/8,.corp.example.com"
```

### How It Works

- Both uppercase and lowercase environment variables are set: `HTTP_PROXY`/`http_proxy`, `HTTPS_PROXY`/`https_proxy`, `NO_PROXY`/`no_proxy`
- The `.svc` and `.cluster.local` suffixes are always appended automatically to `noProxy` to prevent proxying in-cluster traffic
- **Agent-level proxy overrides cluster-level proxy**: If an Agent has `proxy` configured, it takes precedence over the `KubeOpenCodeConfig` proxy settings
- Proxy environment variables are injected into all containers (init containers and the worker container)

## Private Registry Authentication

When using container images from private registries (e.g., Harbor, AWS ECR, GCR), configure `imagePullSecrets` on the Agent to provide registry authentication credentials.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: private-registry-agent
spec:
  profile: "Agent using images from private registries"
  agentImage: registry.corp.example.com/kubeopencode/agent-opencode:latest
  executorImage: registry.corp.example.com/kubeopencode/agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  imagePullSecrets:
    - name: harbor-registry-secret
    - name: gcr-secret
```

### Prerequisites

1. The referenced Secrets must exist in the **same namespace** as the Agent
2. Secrets must be of type `kubernetes.io/dockerconfigjson`

Create registry credentials:

```bash
kubectl create secret docker-registry harbor-registry-secret \
  --docker-server=registry.corp.example.com \
  --docker-username=myuser \
  --docker-password=mypassword \
  -n kubeopencode-system
```

The `imagePullSecrets` are added to the Pod spec of all generated Pods, enabling Kubernetes to authenticate when pulling `agentImage`, `executorImage`, or `attachImage` from private registries.

## Pod Security

KubeOpenCode applies a restricted security context by default to all agent containers, following the Kubernetes Pod Security Standards (Restricted profile).

### Default Security Context

When no `securityContext` is specified in `podSpec`, the controller applies these defaults to all containers (init containers and the worker container):

- `allowPrivilegeEscalation: false`
- `capabilities: drop: ["ALL"]`
- `seccompProfile: type: RuntimeDefault`

These defaults align with the Kubernetes Restricted Pod Security Standard and are suitable for most workloads.

### Custom Container Security Context

Override the default security context for tighter or workload-specific settings using `podSpec.securityContext`:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: hardened-agent
spec:
  profile: "Security-hardened agent with strict container settings"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  podSpec:
    securityContext:
      runAsNonRoot: true
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
          - ALL
      seccompProfile:
        type: RuntimeDefault
```

> **Note**: When using `readOnlyRootFilesystem: true`, ensure the agent image supports it. You may need to use `emptyDir` volumes for writable paths (e.g., `/tmp`, `/home/agent`).

### Pod-Level Security Context

Use `podSpec.podSecurityContext` to configure security attributes that apply to the entire Pod (all containers):

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: uid-agent
spec:
  profile: "Agent running as specific user and group"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  podSpec:
    podSecurityContext:
      runAsUser: 1000
      runAsGroup: 1000
      fsGroup: 1000
```

`podSecurityContext` is useful for:
- Enforcing a specific UID/GID for all containers
- Setting `fsGroup` for shared volume permissions
- Meeting namespace-level Pod Security Admission requirements

## OpenCode Configuration

The `config` field allows you to provide OpenCode configuration as an inline JSON string:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: opencode-agent
spec:
  profile: "OpenCode agent with custom model configuration"
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

## Skills

Skills are reusable AI agent capabilities defined as SKILL.md files (Markdown with YAML frontmatter). KubeOpenCode supports referencing external skills from Git repositories, allowing teams to share and reuse skills across Agents.

Skills are semantically different from Contexts:
- **Contexts** provide knowledge ("what the agent knows")
- **Skills** provide capabilities ("what the agent can do")

### Basic Usage

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: skilled-agent
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  skills:
  - name: official-skills
    git:
      repository: https://github.com/anthropics/skills.git
      ref: main
      path: skills/
      names:
      - frontend-design
      - webapp-testing
```

### Select Specific Skills

Use the `names` field to include only specific skills from a repository. If omitted, all skills under `path` are included:

```yaml
skills:
- name: team-skills
  git:
    repository: https://github.com/my-org/ai-skills.git
    path: engineering/
    names:
    - code-review
    - testing-strategy
```

### Private Repositories

Use `secretRef` for authentication (same Secret format as Git contexts):

```yaml
skills:
- name: internal-skills
  git:
    repository: https://github.com/my-org/private-skills.git
    ref: v2.0.0
    secretRef:
      name: github-pat
```

### How It Works

1. The controller clones skill Git repositories via `git-init` init containers
2. Skills are mounted at `/skills/{source-name}/` in the agent pod
3. The controller auto-injects `skills.paths` into `opencode.json`
4. OpenCode discovers SKILL.md files and makes them available as slash commands

### Skills in Templates

Skills can be defined in AgentTemplates and inherited by Agents. Agent-level skills replace template-level skills (same merge strategy as contexts):

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: AgentTemplate
metadata:
  name: base-template
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  skills:
  - name: org-standards
    git:
      repository: https://github.com/my-org/standards-skills.git
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | (required) | Unique identifier for this skill source |
| `git.repository` | string | (required) | Git URL (https://, http://, or git@) |
| `git.ref` | string | HEAD | Branch, tag, or commit SHA |
| `git.path` | string | (root) | Base directory in repo where skills are located |
| `git.names` | []string | (all) | Specific skill directories to include |
| `git.depth` | int | 1 | Clone depth (1=shallow, 0=full) |
| `git.recurseSubmodules` | bool | false | Clone submodules recursively |
| `git.secretRef.name` | string | - | Secret for Git authentication |

## Git Auto-Sync

Git contexts can be configured to automatically sync with the remote repository.
This enables GitOps workflows where pushing to Git triggers Agent updates.

### Sync Policies

| Policy | Mechanism | Pod Restart | Best For |
|--------|-----------|-------------|----------|
| **HotReload** (default) | Sidecar `git-sync` pulls in-place | No | Prompts, docs, context files |
| **Rollout** | Controller detects change, triggers rolling update | Yes | Workspace root, configs loaded at startup |

### HotReload Example

```yaml
spec:
  contexts:
  - name: team-prompts
    type: Git
    git:
      repository: https://github.com/org/prompts.git
      ref: main
      sync:
        enabled: true
        interval: 5m        # default
        policy: HotReload   # default
    mountPath: prompts/
```

### Rollout Example

```yaml
spec:
  contexts:
  - name: agent-config
    type: Git
    git:
      repository: https://github.com/org/agent-config.git
      ref: main
      sync:
        enabled: true
        interval: 10m
        policy: Rollout
    mountPath: "."
```

### Task Protection (Rollout Policy)

When the controller detects a remote change with Rollout policy, it checks for active Tasks:
- **No active Tasks**: proceeds with rolling update immediately
- **Active Tasks exist**: sets `GitSyncPending` condition and waits
- **Safety timeout (1 hour)**: forces rollout even if Tasks are still running

New Tasks are **not blocked** during `GitSyncPending` — only the Deployment rollout is delayed.

### Sync Status

Agent status tracks the sync state:

```yaml
status:
  gitSyncStatuses:
  - name: team-prompts
    commitHash: "a1b2c3d4e5f6..."
    lastSynced: "2026-04-02T10:30:00Z"
```

## Multi-AI Support

Use different Agents with different executorImages for various use cases:

```yaml
# Standard OpenCode agent with devbox
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: opencode-devbox
spec:
  profile: "Standard OpenCode agent with devbox environment"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
---
# Task sent to a running Agent
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: task-with-opencode
spec:
  agentRef:
    name: opencode-devbox
  description: "Update dependencies and create a PR"
```

Tasks can also reference an AgentTemplate for ephemeral execution without a running Agent:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: ephemeral-task
spec:
  templateRef:
    name: team-config
  description: "Run linting and formatting checks"
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

## CronTask (Scheduled Execution)

CronTask provides scheduled/recurring task execution — analogous to Kubernetes CronJob creating Jobs, CronTask creates Tasks on a cron schedule.

### Basic Usage

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: CronTask
metadata:
  name: daily-vuln-scan
spec:
  schedule: "0 9 * * 1-5"           # Every weekday at 09:00
  timeZone: "Asia/Shanghai"          # IANA timezone (default: UTC)
  concurrencyPolicy: Forbid          # Skip if previous is still running
  maxRetainedTasks: 10               # Max child Tasks (blocks creation when reached)
  taskTemplate:
    spec:
      agentRef:
        name: security-agent
      description: |
        Scan all Go dependencies for CVEs.
        If critical/high found, create a PR with the fix.
```

### Concurrency Policy

Controls behavior when a new schedule fires while a previous Task is still running:

| Policy | Behavior |
|--------|----------|
| **Forbid** (default) | Skip the new Task creation |
| **Allow** | Create new Task regardless |
| **Replace** | Stop the running Task, create new |

### maxRetainedTasks

Safety valve that counts ALL child Tasks (active + finished). When the limit is reached, the controller **blocks new Task creation** (does NOT delete old Tasks). Deletion is handled by the global `KubeOpenCodeConfig.cleanup` mechanism.

This clean separation ensures:
- CronTask is responsible for **creating** Tasks (with a cap)
- Global cleanup is responsible for **deleting** Tasks

### Manual Trigger

Trigger a CronTask to create a Task immediately:

```bash
# Via annotation (kubectl)
kubectl annotate crontask daily-vuln-scan kubeopencode.io/trigger=true

# Via API (UI uses this)
POST /api/v1/namespaces/{ns}/crontasks/{name}/trigger
```

### Suspend/Resume

```bash
# Suspend (stop creating new Tasks, existing ones continue)
kubectl patch crontask daily-vuln-scan --type merge -p '{"spec":{"suspend":true}}'

# Resume
kubectl patch crontask daily-vuln-scan --type merge -p '{"spec":{"suspend":false}}'
```

### Generated Task Naming

Tasks created by CronTask follow the pattern: `{crontask-name}-{unix-timestamp}`

Each generated Task includes:
- Label: `kubeopencode.io/crontask={crontask-name}`
- OwnerReference pointing to the CronTask (for garbage collection)

### CronTask Spec Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `schedule` | string | (required) | Cron expression (5-field: min hour dom month dow) |
| `timeZone` | string | UTC | IANA timezone |
| `concurrencyPolicy` | string | Forbid | Allow, Forbid, or Replace |
| `suspend` | bool | false | Pause scheduling |
| `startingDeadlineSeconds` | int64 | nil | Grace period for missed schedules |
| `maxRetainedTasks` | int32 | 10 | Max child Tasks before blocking creation |
| `taskTemplate` | object | (required) | Template for created Tasks (metadata + spec) |

## Agent Templates

AgentTemplate serves two purposes:

1. **Reusable base configuration for Agents**: Teams define shared settings (images, contexts, credentials) in one template. Individual users create Agents that reference it via `templateRef`.
2. **Blueprint for ephemeral tasks**: Tasks can reference a template directly via `templateRef` to run one-off, ephemeral Pods without a persistent Agent.

### Creating a Template

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: AgentTemplate
metadata:
  name: team-config
spec:
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
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
```

### Creating an Agent from a Template

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  templateRef:
    name: team-config
  profile: "My personal development agent"
  # Required fields (even with template):
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  # Instance-specific settings:
  maxConcurrentTasks: 3
```

### Running Ephemeral Tasks from a Template

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

### Merge Behavior

When an Agent references a template:
- **Scalar fields** (images, workspaceDir, config, etc.): Agent wins if set, otherwise template value
- **List fields** (contexts, credentials, imagePullSecrets): Agent's list **replaces** the template's (not appended)
- **Agent-only fields** (profile, port, persistence, suspend): Always from Agent

### Tracking

Agents using a template automatically get the label `kubeopencode.io/agent-template: <name>`,
enabling template-based queries:

```bash
# List all agents using a template
kubectl get agents -l kubeopencode.io/agent-template=team-config
```

## Concurrency Control

Limit concurrent tasks per Agent when using rate-limited AI services:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: rate-limited-agent
spec:
  profile: "Rate-limited agent for API-quota-constrained backends"
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
  profile: "Rate-limited agent with sliding window quota"
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

## Pod Configuration

Configure advanced Pod settings using `podSpec`:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: advanced-agent
spec:
  profile: "Advanced agent with gVisor isolation and GPU scheduling"
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

## Persistence

By default, Agents use ephemeral storage. When the server pod restarts (due to crashes, node drains, or upgrades), session data and workspace files are lost.

Persistence stores data on PersistentVolumeClaims (PVCs), so it survives pod restarts. Session and workspace persistence are configured independently. See [Live Agents](#live-agents) above for the full overview.

### Configuration

Add `persistence` to your Agent spec:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: persistent-agent
spec:
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  port: 4096
  persistence:
    sessions:
      size: "2Gi"               # default: 1Gi
    workspace:
      size: "20Gi"              # default: 10Gi
```

### How It Works

**Session persistence** (`persistence.sessions`):
- A PVC (`{agent-name}-server-sessions`) is created and mounted at `/data/sessions`
- `OPENCODE_DB` env var is set to `/data/sessions/opencode.db`
- Conversation history survives pod restarts

**Workspace persistence** (`persistence.workspace`):
- The workspace EmptyDir is replaced with a PVC (`{agent-name}-server-workspace`)
- Git-cloned repos, AI-modified files, and in-progress work survive pod restarts
- git-init skips cloning when the repository already exists on the PVC

### PVC Lifecycle

- The PVC is created with an `OwnerReference` to the Agent
- When the Agent is deleted, the PVC is automatically garbage-collected
- To retain data after Agent deletion, configure the StorageClass with `reclaimPolicy: Retain`
- PVC specs are immutable after creation; to change size or storage class, delete and recreate the Agent

### Limitations

- When workspace persistence is enabled and git-init detects an existing repository, it skips cloning.
  If the Agent's git ref changes, the existing checkout is not automatically updated.
- Sessions and workspace can be configured independently (one, both, or neither)

## Suspend/Resume

Agents can be suspended to save compute resources. `spec.suspend` is the single switch — both humans and the controller operate on it.

### Manual Suspend

Set `spec.suspend: true` to scale the Deployment to 0 replicas. PVCs and Service are retained.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: suspendable-agent
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  suspend: true    # scales deployment to 0
  persistence:
    sessions:
      size: "1Gi"
```

- Tasks targeting a suspended agent enter `Queued` phase with reason `AgentSuspended`
- Set `suspend: false` to resume — queued tasks start automatically
- API: `POST /api/v1/namespaces/{ns}/agents/{name}/suspend` and `.../resume`
- Cannot suspend while tasks are running (API returns 409 Conflict)

### Standby (Automatic Suspend/Resume)

Configure `spec.standby` for automatic lifecycle management. The controller manages `spec.suspend` automatically — suspending after idle timeout and resuming when new Tasks arrive.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: auto-scaling-agent
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  standby:
    idleTimeout: "30m"    # auto-suspend after 30 minutes with no tasks
  persistence:
    sessions:
      size: "1Gi"
    workspace:
      size: "10Gi"
```

**How it works:**

1. Agent starts running normally
2. All Tasks complete and no active connections → idle timer starts (`status.idleSince` is set)
3. After 30 minutes with no new Tasks and no active connections → controller sets `spec.suspend = true` → Deployment scales to 0
4. New Task arrives → controller sets `spec.suspend = false` → Deployment scales back to 1
5. Agent becomes ready (~30-60s cold start) → queued Task executes

**Connection-aware idle detection:** The standby system considers both active Tasks and active user connections (web terminal, `kubeoc agent attach`). While a user is connected, the idle timer is deferred — your interactive session won't be interrupted by auto-suspend. Connection activity is tracked via the `kubeopencode.io/last-connection-active` annotation, updated every 60 seconds by the web terminal handler and CLI. See [ADR 0028](adr/0028-connection-aware-standby.md) for details.

**Manual override:** Even with standby configured, you can still manually suspend/resume. The controller will resume automatically when new Tasks arrive.

**Condition reasons:**
- `Suspended: True, reason: UserRequested` — suspended (no standby configured)
- `Suspended: True, reason: Standby` — suspended with standby configured
- `Suspended: False, reason: Active` — running normally
- `StandbyConfigWarning: True, reason: IdleTimeoutTooShort` — `idleTimeout` is less than 2 minutes (connection heartbeat staleness is automatically degraded)

**Best used with `persistence`**: PVCs survive restarts, so session history and workspace files don't need to be re-initialized on resume.

### UI

The Agent detail page shows a **Suspend/Resume** button, and the agents list shows a "Suspended" badge. When standby is configured, the detail page shows the standby configuration and current idle duration.

## Next Steps

- [Getting Started](getting-started.md) - Installation and basic usage
- [Agent Images](agent-images.md) - Build custom agent images
- [Security](security.md) - RBAC, credential management, and best practices
- [Architecture](architecture.md) - System design and API reference
