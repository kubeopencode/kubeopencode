# Features

KubeOpenCode brings Agentic AI capabilities into the Kubernetes ecosystem. Here's an overview of key features.

## Core

- **[Live Agents](live-agents.md)** - Persistent, always-running AI agents as Kubernetes services
- **[Flexible Context System](context-system.md)** - Provide knowledge to agents via Text, ConfigMap, Git, URL, and Runtime contexts
- **[Agent Configuration](agent-configuration.md)** - Centralized execution environment and OpenCode configuration
- **[Agent Templates](agent-templates.md)** - Reusable base configurations and ephemeral task blueprints
- **[Skills](skills.md)** - Reusable AI agent capabilities from Git repositories
- **[Multi-AI Support](multi-ai.md)** - Use different agent images for various AI backends

## Automation

- **[CronTask](crontask.md)** - Scheduled and recurring task execution
- **[Git Auto-Sync](git-auto-sync.md)** - Automatic sync with remote Git repositories
- **[Task Stop](task-stop.md)** - Stop running tasks via annotation
- **[Concurrency & Quota](concurrency-quota.md)** - Limit concurrent tasks and rate of task starts

## Collaboration

- **[Agent Share Link](share-link.md)** - Share terminal access via URL — no Kubernetes credentials required

## Infrastructure

- **[Persistence & Lifecycle](persistence.md)** - Session/workspace persistence, suspend/resume, and standby
- **[Enterprise (Proxy, CA, Registry)](enterprise.md)** - Corporate proxy, custom CA certificates, and private registry authentication
- **[Pod Configuration](pod-configuration.md)** - Pod security, scheduling, and advanced settings
