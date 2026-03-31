<p align="center">
  <img src="docs/images/logo.png" alt="KubeOpenCode Logo" width="500">
</p>

<p align="center">
  <em>Kubernetes-native Agent Platform for Teams and Enterprise</em>
</p>

> **Note**: KubeOpenCode builds on the excellent [OpenCode](https://opencode.ai) AI coding agent. OpenCode is great for individual developers — KubeOpenCode makes it work for **teams and enterprises** by adding governance, shared agent configurations, scale, and enterprise infrastructure integration. This is an independent project, not affiliated with the OpenCode team.

<p align="center">
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <a href="https://goreportcard.com/report/github.com/kubeopencode/kubeopencode"><img src="https://goreportcard.com/badge/github.com/kubeopencode/kubeopencode" alt="Go Report Card"></a>
</p>

## Demo

[![Watch Demo](https://img.shields.io/badge/▶_Watch_Demo-black?style=for-the-badge&logo=x&logoColor=white)](https://x.com/Xzhaojun/status/2035704972642517133)

## Overview

KubeOpenCode is a Kubernetes-native agent platform that enables teams to deploy, manage, and govern AI coding agents at scale. Built on [OpenCode](https://opencode.ai), it turns individual AI coding capabilities into a shared, enterprise-ready platform.

**Why KubeOpenCode?**

- **Live Agents on Kubernetes**: Run AI agents as persistent services with Server Mode — interactive terminal access, shared context across tasks, zero cold start. Perfect for team-shared coding assistants, Slack bots, and always-on development agents.
- **For Teams**: Shared agent configurations, batch operations across repositories, concurrency control, and centralized credential management — so your entire team can leverage AI agents with consistent standards.
- **For Enterprise**: RBAC, private registry support, corporate proxy integration, custom CA certificates, pod security policies, and audit-ready infrastructure — meeting the governance and compliance requirements of enterprise environments.
- **Kubernetes-Native**: Declarative CRDs, GitOps-friendly, works with Helm/Kustomize/ArgoCD — no new tools to learn, just `kubectl apply`.

## Community

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
- **Agent**: AI agent configuration (HOW to execute) — supports two modes:
  - **Pod Mode** (default): Ephemeral Pod per Task, ideal for batch operations
  - **Server Mode**: Persistent agent service with interactive access, ideal for live coding assistants
- **AgentTemplate**: Reusable base configuration for Agents — teams maintain shared settings, individuals create personal Agents that inherit from the template
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
  profile: "General-purpose development agent"
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

### Live Agent (Server Mode)

Deploy a persistent AI agent that your team can interact with anytime:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: team-agent
  namespace: kubeopencode-system
spec:
  profile: "Always-on development agent for the team"
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  serverConfig:
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
```

```bash
# The controller automatically creates a Deployment + Service
kubectl get agents -n kubeopencode-system
# NAME         PROFILE                                  MODE     STATUS
# team-agent   Always-on development agent for the team Server   Ready

# Attach to the live agent from your terminal
kubeoc agent attach team-agent -n kubeopencode-system
```

See the [Getting Started Guide](docs/getting-started.md) for detailed examples including Server Mode setup, batch operations, and interactive access.

### CLI

The KubeOpenCode CLI lets you list agents and interactively attach to server-mode agents from your terminal — no `kubectl port-forward` needed.

**Install:**

```bash
go install github.com/kubeopencode/kubeopencode/cmd/kubeoc@latest
```

**Configure** (optional — defaults to `KUBECONFIG` / `~/.kube/config`):

```bash
# Point to the cluster where KubeOpenCode agents run
export KUBEOPENCODE_KUBECONFIG=/path/to/agent-cluster.kubeconfig
```

**Usage:**

```bash
# List available agents
kubeoc get agents

# NAMESPACE    NAME           PROFILE                          MODE     STATUS
# test         my-agent       General-purpose dev agent         Server   Ready
# prod         review-bot     Automated code review agent       Server   Ready

# Attach to an agent (connects via kube-apiserver service proxy)
kubeoc agent attach my-agent -n test
```

The CLI authenticates using your kubeconfig credentials and connects through the kube-apiserver's built-in service proxy — no Ingress or port-forward required.

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](docs/getting-started.md) | Installation, examples, and tutorials |
| [Features](docs/features.md) | Context system, concurrency, pod configuration |
| [Agent Images](docs/agent-images.md) | Building and customizing agent images |
| [Security](docs/security.md) | RBAC, credentials, pod security |
| [Architecture](docs/architecture.md) | System design and API reference |
| [Troubleshooting](docs/troubleshooting.md) | Common issues and solutions |
| [Local Development](deploy/local-dev/local-development.md) | Development environment setup |
| [Helm Chart](charts/kubeopencode/README.md) | Deployment and configuration |
| [Agent Development](agents/README.md) | Building custom agent images |
| [Roadmap](docs/roadmap.md) | Planned features and improvements |
| [ADRs](docs/adr/) | Architecture Decision Records |

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
