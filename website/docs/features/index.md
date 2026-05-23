# Features

KubeOpenCode brings Agentic AI capabilities into the Kubernetes ecosystem. Here's an overview of key features.

## Core

- **[Live Agents](live-agents.md)** - Persistent, always-running AI agents as Kubernetes services
- **[Flexible Context System](context-system.md)** - Provide knowledge to agents via Text, ConfigMap, Git, URL, and Runtime contexts
- **[Agent Configuration](agent-configuration.md)** - Complete reference for all Agent spec fields
- **[Agent Templates](agent-templates.md)** - Reusable base configurations and ephemeral task blueprints
- **[Skills](skills.md)** - Reusable AI agent capabilities from Git repositories
- **[Plugins](plugins.md)** - OpenCode plugins for deep agent customization
- **[Multi-AI Support](multi-ai.md)** - Use different agent images for various AI backends

## Automation

- **[CronTask](crontask.md)** - Scheduled and recurring task execution
- **[Git Auto-Sync](git-auto-sync.md)** - Automatic sync with remote Git repositories
- **[Task Timeout](task-timeout.md)** - Automatic timeout for long-running tasks
- **[Task Stop](task-stop.md)** - Stop running tasks via annotation
- **[Task Cleanup](task-cleanup.md)** - Automatic cleanup of finished Tasks
- **[Task Session](task-session.md)** - OpenCode session info, token usage, and cost in Task status
- **[Concurrency & Quota](concurrency-quota.md)** - Limit concurrent tasks and rate of task starts

## Collaboration

- **[Agent Share Link](share-link.md)** - Share terminal access via URL — no Kubernetes credentials required

## Observability

- **[OpenTelemetry Observability](observability.md)** - LLM call traces, token usage, latency, and application-level spans via OpenTelemetry

## Infrastructure

- **[Persistence & Lifecycle](persistence.md)** - Session/workspace persistence, suspend/resume, and standby
- **[Enterprise (Proxy, CA, Registry)](enterprise.md)** - Corporate proxy, custom CA certificates, and private registry authentication
- **[Pod Configuration](pod-configuration.md)** - Pod security, scheduling, system containers, and advanced settings