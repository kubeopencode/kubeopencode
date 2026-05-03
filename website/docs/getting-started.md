---
sidebar_position: 1
title: Getting Started
description: Install KubeOpenCode and run your first AI task
---

# Getting Started

:::caution Alpha Project
KubeOpenCode is in **early alpha** (v0.1.x). It is **not recommended for production use**. The API (`v1alpha1`) may introduce breaking changes between releases — backward compatibility is not guaranteed at this stage. We welcome contributions and feedback!
:::

This guide covers installing KubeOpenCode on a Kubernetes cluster and running your first AI task. The default setup uses the free `opencode/big-pickle` model — **no API key required**.

> **Looking for local development setup?** If you want to build from source and run on a local Kind cluster, see the [Contributing Guide](https://github.com/kubeopencode/kubeopencode/blob/main/CONTRIBUTING.md#local-development-environment).

## Prerequisites

- A Kubernetes cluster (1.28+)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/) 3.8+

## Installation

### Install from OCI Registry

```bash
kubectl create namespace kubeopencode-system

helm install kubeopencode oci://ghcr.io/kubeopencode/helm-charts/kubeopencode \
  --namespace kubeopencode-system \
  --set server.enabled=true
```

### Verify Deployment

```bash
# Controller and UI server
kubectl get pods -n kubeopencode-system
```

Expected output:

```
NAME                                       READY   STATUS    RESTARTS   AGE
kubeopencode-controller-xxxxxxxxx-xxxxx    1/1     Running   0          30s
kubeopencode-server-xxxxxxxxx-xxxxx        1/1     Running   0          30s
```

Check CRDs are installed:

```bash
kubectl get crds | grep kubeopencode
```

## Access the Web UI

```bash
kubectl port-forward -n kubeopencode-system svc/kubeopencode-server 2746:2746
```

Open [http://localhost:2746](http://localhost:2746). The UI provides:
- **Task List** — View and filter Tasks across namespaces
- **Task Detail** — Monitor execution with real-time log streaming
- **Task Create** — Submit new Tasks to Agents
- **Agent Browser** — View Agents and AgentTemplates

## Try It Out

### Create an Agent

Create a namespace and a simple Agent:

```bash
kubectl create namespace test

kubectl apply -n test -f - <<EOF
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: dev-agent
spec:
  profile: "A lightweight development agent"
  workspaceDir: /workspace
EOF
```

### Submit a Task

```bash
kubectl apply -n test -f - <<EOF
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: hello-world
spec:
  agentRef:
    name: dev-agent
  description: "Say hello and tell me what tools you have available"
EOF

# Watch the task
kubectl get task -n test -w
```

### Using a Paid Model

The default setup uses the free `opencode/big-pickle` model. To switch to a paid model (Anthropic, Google, etc.), create a Secret with your API key and reference it in the Agent's `credentials` field. See [Security](security.md) for details.

## Upgrade

```bash
helm upgrade kubeopencode oci://ghcr.io/kubeopencode/helm-charts/kubeopencode \
  --namespace kubeopencode-system \
  --set server.enabled=true
```

See [Operations](operations/upgrading.md) for upgrade and maintenance guides.

## Next Steps

- [Setting Up an Agent](setting-up-agent.md) — Configure your own Agent: model selection, images, persistence, and more
- [Features](features/index.md) — Learn about Live Agents (human-in-the-loop) and automated workflows
- [Security](security.md) — RBAC, credential management, and best practices
- [Architecture](architecture.md) — System design and API reference
