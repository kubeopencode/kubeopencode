<p align="center">
  <img src="docs/images/logo.png" alt="KubeOpenCode Logo" width="500">
</p>

<p align="center">
  <em>A Kubernetes-native system for executing AI-powered tasks</em>
</p>

> **Note**: This project uses [OpenCode](https://opencode.ai) as its AI coding agent. KubeOpenCode is not built by or affiliated with the OpenCode team.

<p align="center">
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/kubeopencode/kubeopencode"><img src="https://goreportcard.com/badge/github.com/kubeopencode/kubeopencode" alt="Go Report Card"></a>
</p>

## Overview

KubeOpenCode enables you to execute [OpenCode](https://opencode.ai) AI agent tasks using Kubernetes Custom Resources. It provides a simple, declarative, GitOps-friendly approach to running AI coding agents as Kubernetes Pods.

## Community & Dogfooding

> **We eat our own dog food!** KubeOpenCode uses itself in production:
> - GitHub webhooks trigger Tasks via [Argo Events](https://argoproj.github.io/argo-events/) for automated PR reviews, issue triage, and more
> - The **KubeOpenCode bot** in Slack and GitHub Discussions is powered by KubeOpenCode itself

**Get Help & Connect:**

- **Slack**: [Join KubeOpenCode Slack](https://join.slack.com/t/kubeopencode/shared_invite/zt-3o9qibz2b-PjJP4m2cHMcNT3cVg2TDhA) - Ask questions and get auto-answers from the KubeOpenCode bot
- **Discussions**: [GitHub Discussions](https://github.com/kubeopencode/kubeopencode/discussions) - Community Q&A with KubeOpenCode bot assistance
- **Issues**: [GitHub Issues](https://github.com/kubeopencode/kubeopencode/issues) - Bug reports and feature requests

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
│      KubeOpenCode Controller (Operator)     │
│  - Watch Task CRs                           │
│  - Create Kubernetes Pods for tasks         │
│  - Update CR status                         │
└─────────────────┬───────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────┐
│            Kubernetes Pods                  │
│  - Execute tasks using AI agents            │
│  - Context files mounted as volumes         │
└─────────────────────────────────────────────┘
```

### Core Concepts

- **Task**: Single task execution (the primary API)
- **TaskTemplate**: Reusable template for Task creation (similar to Argo WorkflowTemplate)
- **Agent**: AI agent configuration (HOW to execute)
- **KubeOpenCodeConfig**: System-level configuration (optional)

> **Note**: Workflow orchestration and webhook triggers have been delegated to Argo Workflows and Argo Events respectively. KubeOpenCode focuses on the core Task/Agent abstraction.

## Quick Start

### Prerequisites

- Kubernetes 1.25+
- Helm 3.8+

### Installation

```bash
# Create namespace
kubectl create namespace kubeopencode-system

# Install from OCI registry (with UI enabled)
helm install kubeopencode oci://quay.io/kubeopencode/helm-charts/kubeopencode \
  --namespace kubeopencode-system \
  --set server.enabled=true
```

### Minimal Example

```yaml
# Create an Agent
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: default
  namespace: kubeopencode-system
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  credentials:
    - name: api-key
      secretRef:
        name: ai-credentials
        key: api-key
      env: OPENCODE_API_KEY
---
# Create a Task
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: my-task
  namespace: kubeopencode-system
spec:
  agentRef:
    name: default
  description: |
    Update dependencies to latest versions.
    Run tests and create PR.
```

```bash
# Monitor progress
kubectl get tasks -n kubeopencode-system -w
kubectl logs $(kubectl get task my-task -o jsonpath='{.status.podName}') -n kubeopencode-system
```

See the [Getting Started Guide](docs/getting-started.md) for detailed examples including TaskTemplates, batch operations, and Web UI access.

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](docs/getting-started.md) | Installation, examples, and tutorials |
| [Features](docs/features.md) | Context system, concurrency, cross-namespace separation |
| [Agent Images](docs/agent-images.md) | Building and customizing agent images |
| [Security](docs/security.md) | RBAC, credentials, pod security |
| [Architecture](docs/architecture.md) | System design and API reference |
| [Troubleshooting](docs/troubleshooting.md) | Common issues and solutions |
| [Local Development](deploy/local-dev/local-development.md) | Development environment setup |
| [Helm Chart](charts/kubeopencode/README.md) | Deployment and configuration |
| [Agent Development](agents/README.md) | Building custom agent images |
| [ADRs](docs/adr/) | Architecture Decision Records |

## Roadmap

### Completed

- [x] Core Task/Agent abstraction
- [x] Inline ContextItem system (Text, ConfigMap, Git, Runtime, URL)
- [x] Argo Events integration for webhook-triggered Tasks
- [x] Task output capture system
- [x] Task stop feature (annotation-based)
- [x] Agent concurrency control (maxConcurrentTasks, quota)
- [x] Pod configuration (labels, scheduling, runtimeClassName)
- [x] Cross-namespace Task/Agent separation (credential isolation)
- [x] TaskTemplate for reusable configurations
- [x] Web UI for Task and Agent management

### Next Priorities

#### 1. Agent Skills Support

Enable Agents to declare and use skills (tools/capabilities) for enhanced task execution:

- **Skill Definition**: Define reusable skills that Agents can invoke (e.g., web search, code analysis, file operations)
- **MCP Integration**: Support for Model Context Protocol (MCP) servers as skill providers
- **Skill Composition**: Allow Agents to combine multiple skills for complex workflows
- **Built-in Skills**: Provide common skills out-of-the-box (kubectl, git, GitHub CLI, etc.)

#### 2. Metrics & Observability

Prometheus-native metrics for comprehensive Task/Agent observability:

- **Task Metrics**: Execution duration, success/failure rates, queue times, resource usage
- **Agent Metrics**: Concurrent task counts, quota utilization, error rates per Agent
- **Controller Metrics**: Reconciliation latency, API call rates, cache hit rates
- **Grafana Dashboards**: Pre-built dashboards for common monitoring scenarios

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on:

- Commit standards (signed commits required)
- Pull request process
- Code standards and testing requirements
- Development workflow

## License

Copyright Contributors to the KubeOpenCode project.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at:

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

## Acknowledgments

KubeOpenCode is inspired by:
- Tekton Pipelines
- Argo Workflows
- Kubernetes Batch API

Built with:
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)

---

Made with love by the KubeOpenCode community
