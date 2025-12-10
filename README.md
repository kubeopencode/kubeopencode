# KubeTask

A Kubernetes-native system for executing AI-powered tasks.

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/stolostron/kubetask)](https://goreportcard.com/report/github.com/stolostron/kubetask)

## Overview

KubeTask enables you to execute AI agent tasks (like Claude, Gemini) using Kubernetes Custom Resources. It provides a simple, declarative, GitOps-friendly approach to running AI agents as Kubernetes Jobs.

**Key Features:**

- **Kubernetes-Native**: Built on CRDs and the Operator pattern
- **Simple API**: Only two CRDs - Task and Agent
- **AI-Agnostic**: Works with any AI agent (Claude, Gemini, etc.)
- **No External Dependencies**: Uses etcd for state, Jobs for execution
- **GitOps Ready**: Fully declarative resource definitions
- **Flexible Context System**: Support for files from inline content or ConfigMaps
- **Batch Operations**: Use Helm/Kustomize for multiple Tasks (Kubernetes-native approach)

## Architecture

```
┌─────────────────────────────────────────────┐
│         Kubernetes API Server               │
│  - Custom Resource Definitions (CRDs)       │
│  - RBAC & Authentication                    │
└─────────────────┬───────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────┐
│      KubeTask Controller (Operator)         │
│  - Watch Task CRs                           │
│  - Create Kubernetes Jobs for tasks         │
│  - Update CR status                         │
└─────────────────┬───────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────┐
│         Kubernetes Jobs/Pods                │
│  - Execute tasks using AI agents            │
│  - Context files mounted as volumes         │
└─────────────────────────────────────────────┘
```

### Core Concepts

- **Task**: Single task execution (the primary API)
- **Agent**: AI agent configuration (HOW to execute)

## Quick Start

### Prerequisites

- Kubernetes 1.25+
- Helm 3.8+
- GitHub Personal Access Token (optional)
- Anthropic Claude API key or Vertex AI credentials

### Installation

```bash
# Create namespace
kubectl create namespace kubetask-system

# Install with Helm
helm install kubetask ./charts/kubetask \
  --namespace kubetask-system
```

### Example Usage

#### 1. Create an Agent

```yaml
apiVersion: kubetask.io/v1alpha1
kind: Agent
metadata:
  name: default
  namespace: kubetask-system
spec:
  agentImage: quay.io/myorg/claude-agent:v1.0
  serviceAccountName: kubetask-agent
  credentials:
    - name: anthropic-api-key
      secretRef:
        name: ai-credentials
        key: anthropic-key
      env: ANTHROPIC_API_KEY
```

#### 2. Create a Task

```yaml
apiVersion: kubetask.io/v1alpha1
kind: Task
metadata:
  name: update-service-a
  namespace: kubetask-system
spec:
  contexts:
    # Task description
    - type: File
      file:
        filePath: /workspace/task.md
        source:
          inline: |
            Update dependencies to latest versions.
            Run tests and create PR.

    # Workflow guide from ConfigMap
    - type: File
      file:
        filePath: /workspace/guide.md
        source:
          configMapKeyRef:
            name: workflow-guides
            key: pr-workflow.md
```

#### 3. Monitor Progress

```bash
# Watch Task status
kubectl get tasks -n kubetask-system -w

# Check detailed status
kubectl describe task update-service-a -n kubetask-system

# View task logs
kubectl logs job/$(kubectl get task update-service-a -o jsonpath='{.status.jobName}') -n kubetask-system
```

### Batch Operations with Helm

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
apiVersion: kubetask.io/v1alpha1
kind: Task
metadata:
  name: {{ .name }}
spec:
  contexts:
    - type: File
      file:
        filePath: /workspace/task.md
        source:
          inline: "Update dependencies for {{ .repo }}"
{{- end }}
```

```bash
# Generate and apply multiple tasks
helm template my-tasks ./chart | kubectl apply -f -
```

## Key Features

### Flexible Context System

KubeTask supports multiple context types:

- **Single File with Inline Content**:
  ```yaml
  type: File
  file:
    filePath: /workspace/task.md
    source:
      inline: "Task description"
  ```

- **Single File from ConfigMap Key**:
  ```yaml
  type: File
  file:
    filePath: /workspace/guide.md
    source:
      configMapKeyRef:
        name: guides
        key: workflow.md
  ```

- **Directory from ConfigMap** (all keys become files):
  ```yaml
  type: File
  file:
    dirPath: /workspace/configs
    source:
      configMapRef:
        name: my-configs
  ```

- **Content Aggregation**: Multiple contexts with the same `filePath` are aggregated into a single file

### Agent Configuration

Agent centralizes execution environment configuration:

```yaml
apiVersion: kubetask.io/v1alpha1
kind: Agent
metadata:
  name: default
spec:
  agentImage: quay.io/myorg/claude-agent:v1.0
  toolsImage: quay.io/myorg/dev-tools:latest
  serviceAccountName: kubetask-agent

  # Default contexts for all tasks
  defaultContexts:
    - type: File
      file:
        filePath: /workspace/org-standards.md
        source:
          configMapKeyRef:
            name: org-configs
            key: standards.md

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

  # Pod scheduling
  scheduling:
    nodeSelector:
      workload-type: ai-agent
```

### Multi-AI Support

Use different Agents for different AI agents:

```yaml
# Claude workspace
apiVersion: kubetask.io/v1alpha1
kind: Agent
metadata:
  name: claude-workspace
spec:
  agentImage: quay.io/myorg/claude-agent:v1.0
  serviceAccountName: kubetask-agent
---
# Gemini workspace
apiVersion: kubetask.io/v1alpha1
kind: Agent
metadata:
  name: gemini-workspace
spec:
  agentImage: quay.io/myorg/gemini-agent:v1.0
  serviceAccountName: kubetask-agent
---
# Task using specific agent
apiVersion: kubetask.io/v1alpha1
kind: Task
metadata:
  name: task-with-claude
spec:
  agentRef: claude-workspace
  contexts:
    - type: File
      file:
        filePath: /workspace/task.md
        source:
          inline: "Update dependencies"
```

## Agent Images

KubeTask provides **template agent images** that serve as starting points for building your own customized agents. These templates demonstrate the agent interface pattern and include common development tools, but are designed to be customized based on your specific requirements.

**Important**: The provided agent images (gemini, goose, echo) are examples/templates. You should build and customize your own agent images according to your needs:

- Choose which AI CLI to include (Gemini, Claude Code, Goose, etc.)
- Install the specific tools your tasks require
- Configure credentials and environment variables
- Optimize image size for your use case

### Available Templates

| Template | Description | Use Case |
|----------|-------------|----------|
| `gemini` | Google Gemini CLI with Go, git, kubectl | General development tasks |
| `goose` | Block's Goose agent with Go, git, kubectl | Multi-provider AI tasks |
| `echo` | Minimal Alpine image | E2E testing and debugging |

### Building Your Agent

```bash
# Build from template
make agent-build AGENT=gemini

# Customize registry and version
make agent-build AGENT=gemini IMG_REGISTRY=docker.io IMG_ORG=myorg VERSION=v1.0.0
```

For detailed guidance on building custom agent images, see the [Agent Developer Guide](agents/README.md).

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/stolostron/kubetask.git
cd kubetask

# Build the controller
make build

# Run tests
make test

# Run linter
make lint
```

### Local Development

```bash
# Run controller locally (requires kubeconfig)
make run

# Format code
make fmt

# Update generated code
make update
```

### E2E Testing

```bash
# Setup e2e environment
make e2e-setup

# Run e2e tests
make e2e-test

# Teardown e2e environment
make e2e-teardown
```

### Docker Images

```bash
# Build docker image
make docker-build

# Build and push multi-arch images
make docker-buildx
```

## Configuration

### Helm Chart Values

See [charts/kubetask/README.md](charts/kubetask/README.md) for complete configuration options.

Key configurations:

```yaml
controller:
  image:
    repository: quay.io/zhaoxue/kubetask-controller
    tag: v0.1.0
  resources:
    limits:
      cpu: 500m
      memory: 512Mi
```

## Documentation

- [Architecture](docs/architecture.md) - Detailed architecture and design decisions
- [Agent Context Spec](docs/agent-context-spec.md) - How contexts are mounted
- [Helm Chart](charts/kubetask/README.md) - Deployment and configuration guide
- [ADRs](docs/adr/) - Architecture Decision Records

## Security

### RBAC

KubeTask follows the principle of least privilege:

- **Controller**: Manages CRs and Jobs only

### Secrets Management

Never commit secrets to Git. Use:
- Kubernetes Secrets
- External Secrets Operator
- Sealed Secrets
- HashiCorp Vault

### Pod Security

- Runs with non-root user
- Dropped capabilities
- Read-only root filesystem (where applicable)

## Troubleshooting

### Controller Issues

```bash
# Check controller logs
kubectl logs -n kubetask-system deployment/kubetask-controller

# Verify RBAC
kubectl auth can-i create tasks \
  --as=system:serviceaccount:kubetask-system:kubetask-controller
```

### Job Failures

```bash
# List failed jobs
kubectl get jobs -n kubetask-system --field-selector status.successful=0

# Check job logs
kubectl logs job/<job-name> -n kubetask-system

# Describe job for events
kubectl describe job/<job-name> -n kubetask-system
```

## Contributing

We welcome contributions! Please follow these guidelines:

1. **Commit Standards**: Use signed commits with `-s` flag
   ```bash
   git commit -s -m "feat: add new feature"
   ```

2. **Pull Requests**:
   - Check for upstream repositories first
   - Create PRs against upstream, not forks
   - Use descriptive titles and comprehensive descriptions

3. **Code Standards**:
   - Write code comments in English
   - Follow Go conventions
   - Run `make lint` before submitting

4. **Testing**:
   - Write tests for new features
   - Ensure `make test` passes
   - Test e2e changes with `make e2e-test`

See [CLAUDE.md](CLAUDE.md) for detailed development guidelines.

## Roadmap

- [ ] Enhanced status reporting and observability
- [ ] Support for additional context types (MCP)
- [ ] Advanced retry and failure handling
- [ ] Integration with more AI providers
- [ ] Web UI for monitoring and management
- [ ] GitOps integration examples (Flux, ArgoCD)

## Community

- **Issues**: [GitHub Issues](https://github.com/stolostron/kubetask/issues)
- **Discussions**: [GitHub Discussions](https://github.com/stolostron/kubetask/discussions)

## License

Copyright Contributors to the KubeTask project.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at:

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

## Acknowledgments

KubeTask is inspired by:
- Tekton Pipelines
- Argo Workflows
- Kubernetes Batch API

Built with:
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)

---

Made with love by the KubeTask community
