# Getting Started with KubeOpenCode

## Quick Start (AI-Assisted)

The fastest way to get started — let your AI coding agent set up everything for you:

```bash
git clone https://github.com/kubeopencode/kubeopencode.git
cd kubeopencode
```

Then tell your AI agent (Claude Code, Cursor, Windsurf, etc.):

> Follow `deploy/local-dev/local-development.md` to set up a local development environment for me.

The agent will handle Kind cluster creation, image builds, Helm installation, and test resource deployment automatically.

## Manual Setup

### Prerequisites

- Kubernetes 1.25+ (or [Kind](https://kind.sigs.k8s.io/) for local development)
- Helm 3.8+

### Install

```bash
kubectl create namespace kubeopencode-system

helm install kubeopencode oci://quay.io/kubeopencode/helm-charts/kubeopencode \
  --namespace kubeopencode-system \
  --set server.enabled=true
```

### Access the Web UI

```bash
kubectl port-forward -n kubeopencode-system svc/kubeopencode-server 2746:2746
# Open http://localhost:2746
```

## Choose Your Approach

| | **Agent (Persistent)** | **AgentTemplate (Ephemeral)** |
|---|---|---|
| **What** | Running AI agent as a Kubernetes service | Blueprint for one-off task Pods |
| **Best for** | Interactive coding, team-shared agents | Batch operations, CI/CD pipelines |
| **Cold start** | None (always running) | Yes (container startup per Task) |
| **Interaction** | Web Terminal, CLI attach, API | Logs only |
| **Task reference** | `agentRef` | `templateRef` |

## Live Agent (Recommended)

### 1. Create an Agent

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: dev-agent
  namespace: kubeopencode-system
spec:
  profile: "Interactive development agent"
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  port: 4096
  persistence:
    sessions:
      size: "2Gi"

  credentials:
    - name: api-key
      secretRef:
        name: ai-credentials
        key: api-key
      env: OPENCODE_API_KEY

  # Optional: pre-load your codebase
  contexts:
    - name: source-code
      type: Git
      git:
        repository: https://github.com/your-org/your-repo.git
        ref: main
      mountPath: code
```

### 2. Wait for Ready

```bash
kubectl get agents -n kubeopencode-system -w
# NAME        PROFILE                         STATUS
# dev-agent   Interactive development agent    Ready
```

### 3. Interact

**Web Terminal:** Open http://localhost:2746, navigate to the agent, and click "Terminal".

**Submit Tasks programmatically:**

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: fix-bug-123
  namespace: kubeopencode-system
spec:
  agentRef:
    name: dev-agent
  description: |
    Fix the null pointer exception in UserService.java.
    The bug is reported in issue #123.
```

## Ephemeral Tasks (with AgentTemplate)

For batch operations and CI/CD pipelines:

```yaml
# 1. Create a template
apiVersion: kubeopencode.io/v1alpha1
kind: AgentTemplate
metadata:
  name: batch-template
  namespace: kubeopencode-system
spec:
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  credentials:
    - name: opencode-api-key
      secretRef:
        name: ai-credentials
        key: opencode-key
      env: OPENCODE_API_KEY
---
# 2. Create a task referencing the template
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: update-deps
  namespace: kubeopencode-system
spec:
  templateRef:
    name: batch-template
  description: |
    Update dependencies to latest versions.
    Run tests and create PR.
```

```bash
# Monitor progress
kubectl get tasks -n kubeopencode-system -w
```

## Next Steps

- [Features](features.md) - Context system, concurrency control, and more
- [Agent Images](agent-images.md) - Build custom agent images
- [Security](security.md) - RBAC, credential management
- [Architecture](architecture.md) - System design and API reference
- [Local Development](../deploy/local-dev/local-development.md) - Full local dev environment setup
