# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the KubeOpenCode project.
ADRs document significant architectural and design decisions along with their context and consequences.

## Index by Status

### Implemented

| # | Title | Date |
|---|-------|------|
| 0001 | [Record Architecture Decisions](0001-record-architecture-decisions.md) | |
| 0006 | [Environment Configuration for Agent Containers in SCC Environments](0006-home-directory-for-agent-containers.md) | |
| 0007 | [Migrate Workflow and Webhook Functionality to Argo](0007-migrate-to-argo-events-workflows.md) | |
| 0008 | [Rebrand KubeTask to KubeOpenCode](0008-rebrand-kubetask-to-kubeopencode.md) | |
| 0009 | [Task Execution Migration from Job to Pod](0009-job-to-pod-migration.md) | 2026-01-05 |
| 0010 | [OpenCode Permission Configuration for Automated Execution](0010-opencode-permission-for-automated-execution.md) | |
| 0022 | [Agent Always Running — Unified Execution Model](0022-agent-always-running.md) | 2026-03-31 |
| 0023 | [Agent Idle Timeout for Auto-Suspend/Auto-Resume](0023-agent-idle-timeout.md) | 2026-04-01 |
| 0014 | [Remove TaskTemplate CRD](0014-remove-tasktemplate.md) | 2026-03-10 |
| 0018 | [Web Terminal Replaces OpenCode Web UI](0018-web-terminal-replaces-web-ui.md) | 2026-03-27 |
| 0019 | [Web Terminal Credential Security Strategy](0019-web-terminal-credential-security.md) | 2026-03-27 |
| 0021 | [Custom CA Bundle Design for Init Containers](0021-custom-ca-bundle-design.md) | |

### Partially Implemented

| # | Title | Notes |
|---|-------|-------|
| 0016 | [Human-in-the-Loop (HITL) Design](0016-human-in-the-loop-design.md) | Simplified model adopted; Phase 2 UI superseded by ADR 0018; Phase 3 not started |

### Accepted (Deferred)

| # | Title | Notes |
|---|-------|-------|
| 0012 | [Defer Session API to Post-v0.1](0012-defer-session-api.md) | Blocked on Server Mode production validation and security model |
| 0013 | [Defer Token Usage Tracking to Post-v0.1](0013-defer-token-usage-tracking.md) | Blocked on upstream OpenCode `stats --format json` support |

### Accepted

| # | Title | Date |
|---|-------|------|
| 0024 | [Agent Standby — Unified Suspend/Resume Lifecycle](0024-agent-standby.md) | |
| 0025 | [CronTask — Scheduled Task Execution](0025-crontask.md) | |
| 0026 | [Skills as a Top-Level Agent Field](0026-skills.md) | |
| 0026-mcp | [MCP Server Support in Agent API](0026-mcp-support.md) | |
| 0027 | [Git Context Auto-Sync for Agents](0027-git-sync.md) | |
| 0028 | [Connection-Aware Standby — Heartbeat-based Idle Detection](0028-connection-aware-standby.md) | |
| 0029 | [Container-in-Container (DinD) Support for Agent Workloads](0029-sysbox-dind-support.md) | |

### Proposed

| # | Title | Date |
|---|-------|------|
| 0020 | [Enterprise Readiness Roadmap](0020-enterprise-readiness-roadmap.md) | CA certs, metrics, proxy, imagePullSecrets, pod security done; rest pending |
| 0030 | [Graceful Task Termination on Deletion](0030-task-deletion-graceful-stop.md) | |
| 0031 | [OpenTelemetry Observability for Tasks and Agents](0031-opentelemetry-observability.md) | |
| 0032 | [Harness Strategy — Lessons from Anthropic Managed Agents](0032-harness-strategy-managed-agents-analysis.md) | 2026-04-11 |
| 0036 | [Agent Registry — Enterprise Agent Asset Management and Visual Agent Assembly](0036-agent-registry.md) | 2026-05-05 |

### Superseded

| # | Title | Superseded By |
|---|-------|---------------|
| 0002 | [Task CRD vs Kubernetes Job](0002-task-crd-vs-kubernetes-job.md) | [ADR 0009](0009-job-to-pod-migration.md) |
| 0011 | [Agent Server Mode for Persistent OpenCode Servers](0011-agent-server-mode.md) | [ADR 0022](0022-agent-always-running.md) |
| 0015 | [Repo as Agent — Dynamic Image Building](0015-repo-as-agent-dynamic-image-building.md) | [ADR 0036](0036-agent-registry.md) |
| 0017 | [OpenCode Web UI Integration via Self-Hosted Reverse Proxy](0017-opencode-web-ui-integration.md) | [ADR 0018](0018-web-terminal-replaces-web-ui.md) |
