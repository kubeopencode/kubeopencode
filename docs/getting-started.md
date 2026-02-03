# Getting Started with KubeOpenCode

This guide covers installation, configuration, and basic usage of KubeOpenCode.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.8+

## Installation

### Install from OCI Registry

```bash
# Create namespace
kubectl create namespace kubeopencode-system

# Install from OCI registry (with UI enabled)
helm install kubeopencode oci://quay.io/kubeopencode/helm-charts/kubeopencode \
  --namespace kubeopencode-system \
  --set server.enabled=true
```

### Install from Local Chart (Development)

```bash
# Create namespace
kubectl create namespace kubeopencode-system

# Install from local chart
helm install kubeopencode ./charts/kubeopencode \
  --namespace kubeopencode-system \
  --set server.enabled=true
```

## Access the Web UI

```bash
# Port forward to access the UI
kubectl port-forward -n kubeopencode-system svc/kubeopencode-server 2746:2746

# Open http://localhost:2746 in your browser
```

The Web UI provides:
- **Task List**: View and filter Tasks across namespaces
- **Task Detail**: Monitor Task execution with real-time log streaming
- **Task Creation**: Create new Tasks with Agent selection
- **Agent Browser**: View available Agents and their configurations

## Example Usage

### 1. Create an Agent

KubeOpenCode uses a **two-container pattern**:
- **Init Container** (`agentImage`): Copies OpenCode binary to `/tools` shared volume
- **Worker Container** (`executorImage`): Runs tasks using `/tools/opencode`

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: default
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
```

### 2. Create a Task

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: update-service-a
  namespace: kubeopencode-system
spec:
  # Task description (becomes /workspace/task.md)
  description: |
    Update dependencies to latest versions.
    Run tests and create PR.

  # Optional inline contexts
  contexts:
    - type: Text
      text: |
        # Coding Standards
        - Use descriptive names
        - Write unit tests
```

### 3. Monitor Progress

```bash
# Watch Task status
kubectl get tasks -n kubeopencode-system -w

# Check detailed status
kubectl describe task update-service-a -n kubeopencode-system

# View task logs
kubectl logs $(kubectl get task update-service-a -o jsonpath='{.status.podName}') -n kubeopencode-system
```

### 4. Use TaskTemplate for Reusable Configurations

TaskTemplates let you define common Task configurations that can be shared across multiple Tasks:

```yaml
# Create a TaskTemplate with shared configuration
apiVersion: kubeopencode.io/v1alpha1
kind: TaskTemplate
metadata:
  name: pr-task-template
  namespace: kubeopencode-system
spec:
  agentRef:
    name: default
  contexts:
    - type: ConfigMap
      configMap:
        name: coding-standards
---
# Create Tasks that reference the template
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: fix-issue-123
spec:
  taskTemplateRef:
    name: pr-task-template
  description: |
    Fix issue #123: Login button not working on mobile.
```

**Merge Strategy:**

| Field | Merge Behavior |
|-------|----------------|
| `agentRef` | Task takes precedence; if not specified, uses Template's |
| `description` | Task takes precedence; if not specified, uses Template's |
| `contexts` | Template contexts first, then Task contexts (both included) |

## Batch Operations with Helm

For running the same task across multiple targets, use Helm templating:

```yaml
# values.yaml
tasks:
  - name: update-service-a
    repo: service-a
  - name: update-service-b
    repo: service-b
  - name: update-service-c
    repo: service-c

# templates/tasks.yaml
{{- range .Values.tasks }}
---
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: {{ .name }}
spec:
  description: "Update dependencies for {{ .repo }}"
{{- end }}
```

```bash
# Generate and apply multiple tasks
helm template my-tasks ./chart | kubectl apply -f -
```

## Next Steps

- [Features](features.md) - Learn about the context system, concurrency control, and more
- [Agent Images](agent-images.md) - Build custom agent images
- [Security](security.md) - RBAC, credential management, and best practices
- [Architecture](architecture.md) - System design and API reference
