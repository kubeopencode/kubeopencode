---
sidebar_position: 8
title: Roadmap
description: Future directions for KubeOpenCode
---

# Roadmap

:::caution Alpha Project
KubeOpenCode is in **early alpha** (v0.1.x). The API (`v1alpha1`) may introduce breaking changes between releases. We do not guarantee backward compatibility at this stage. See [Getting Started](getting-started.md) for details.
:::

## Direction 1: Instant Messaging Integration

**Goal**: Make AI agents part of your team's daily communication workflow.

- **Slack integration** — Bi-directional: receive commands from Slack channels, push task results and notifications back
- **Other IM platforms** — Microsoft Teams, Lark/Feishu, and other enterprise messaging tools
- **ChatOps patterns** — Trigger tasks, check agent status, and review results without leaving your messaging app

This direction focuses on **usability** — reducing the friction between "I need an AI agent to do something" and getting the result.

## Direction 2: Kubernetes Ecosystem Integration

**Goal**: Production-grade stability and security through deeper integration with the Kubernetes ecosystem.

- **GitOps** — Native integration with ArgoCD, Flux for declarative agent management
- **Policy engines** — OPA/Gatekeeper, Kyverno integration for agent governance
- **Network security** — NetworkPolicy templates, service mesh integration
- **Multi-tenancy** — Namespace-level isolation, resource quotas, priority classes

This direction focuses on **stability and security** — making KubeOpenCode ready for production environments.

## Direction 3: Observability (OpenTelemetry)

**Goal**: Full observability for AI agent workloads — understand what agents are doing, how they perform, and where they fail.

- **OpenTelemetry integration** — Instrument controller and agent lifecycle with OTel traces, metrics, and logs
- **Task execution tracing** — End-to-end distributed traces from Task creation through agent execution to completion
- **Prometheus metrics** — Task duration, success/failure rates, queue depth, agent utilization, token usage
- **Structured logging** — Consistent, queryable log format across controller, init containers, and agents
- **Dashboard templates** — Pre-built Grafana dashboards for task throughput, agent health, and error analysis

This direction focuses on **operational insight** — giving platform teams the data they need to run AI agents reliably at scale.

## Direction 4: Container-in-Container (DinD) Support

**Goal**: Enable agent workloads that require running Docker, Podman, or Kind clusters inside containers — without compromising cluster security.

AI agents often need to run E2E tests against Kind clusters, build container images, or operate full development environments. These workloads require nested container runtimes inside the agent pod, which standard Kubernetes does not support out of the box.

KubeOpenCode itself does not need code changes — the Agent `podSpec` already supports `runtimeClassName` passthrough. What's needed is **cluster configuration guidance**: documentation and best practices for administrators to enable DinD capabilities using approaches like Sysbox, Rootless Podman, or Kata Containers, depending on their security and platform requirements.

See [ADR 0029](https://github.com/kubeopencode/kubeopencode/blob/main/docs/adr/0029-sysbox-dind-support.md) for the full evaluation of approaches and phased adoption strategy.

## Deferred

### Token Usage Tracking & Cost Reporting

**Status**: Waiting for upstream OpenCode support

Track per-Task token consumption and estimated cost. Blocked on OpenCode lacking machine-readable output for token statistics. See [ADR 0013](https://github.com/kubeopencode/kubeopencode/blob/main/docs/adr/0013-defer-token-usage-tracking.md) for details.
