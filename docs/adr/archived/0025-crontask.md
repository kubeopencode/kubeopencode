# ADR 0025: CronTask — Scheduled Task Execution

## Status

Accepted

## Date

2026-04-02

## Context

KubeOpenCode has migrated many features to the UI, making the Agent's "proactive" capability — the ability to act autonomously on a schedule — a critical gap. Currently, scheduled execution is delegated to Kubernetes CronJob, which requires users to create a CronJob that shells out to kubectl or the API to create Tasks. This is clunky, not discoverable in the UI, and breaks the native KubeOpenCode experience.

CronTask is essential for use cases like:
- Daily dependency vulnerability scanning
- Periodic code review or report generation
- Scheduled environment cleanup or health checks
- Any recurring AI-powered task

The relationship mirrors Kubernetes' CronJob → Job pattern: CronTask is a Task factory that creates Task objects on a cron schedule.

## Decision

### New CRD: CronTask

Introduce `CronTask` as a namespaced CRD (shortName: `ct`) that creates Task objects on a cron schedule.

### API Design

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: CronTask
metadata:
  name: daily-vuln-scan
spec:
  schedule: "0 9 * * 1-5"           # Standard 5-field cron (required)
  timeZone: "Asia/Shanghai"          # IANA timezone (optional, default UTC)
  concurrencyPolicy: Forbid          # Allow | Forbid | Replace
  suspend: false                     # Pause scheduling
  startingDeadlineSeconds: 300       # Grace period for missed schedules
  maxRetainedTasks: 10               # Max child Tasks that can exist (blocks creation when reached)
  taskTemplate:
    metadata:
      labels:
        team: alpha
    spec:
      agentRef:
        name: security-agent
      description: "Scan dependencies for CVEs"
```

### Key Design Choices

**1. ConcurrencyPolicy defaults to Forbid**

Unlike Kubernetes CronJob (defaults to Allow), CronTask defaults to Forbid because AI tasks are resource-intensive (LLM API calls, GPU usage). Concurrent AI tasks can cause cost spikes. Forbid is the safest default — skip if previous is still running.

**2. maxRetainedTasks blocks creation, does NOT delete**

CronTask does not implement its own cleanup. Instead, `maxRetainedTasks` (default 10) counts ALL child Tasks (active + finished). When the limit is reached, the controller skips creating new Tasks and records an Event. Deletion of completed Tasks is handled entirely by the global `KubeOpenCodeConfig.cleanup` mechanism.

This separation of concerns:
- CronTask: responsible for CREATING Tasks (with a cap)
- KubeOpenCodeConfig cleanup: responsible for DELETING Tasks

If global cleanup is not configured, CronTask hits its cap and stops creating — a natural safety valve that signals admins to configure cleanup.

**3. Manual trigger via both API and annotation**

- API endpoint `POST /trigger` for UI "Run Now" button
- Annotation `kubeopencode.io/trigger=true` for kubectl users
- Consistent with existing patterns (Task Stop uses annotation, Agent Suspend/Resume uses API)

**4. No interaction with Agent Standby required**

CronTask creates standard Tasks. The existing Task Controller already handles resuming suspended Agents (standby auto-resume). CronTask does not need to know about standby.

### Generated Task Naming

Tasks created by CronTask follow the pattern: `{crontask-name}-{unix-timestamp}`

Each generated Task includes:
- Label: `kubeopencode.io/crontask={crontask-name}` (for querying)
- OwnerReference: pointing to CronTask (for garbage collection)

## Consequences

### Positive

- Native scheduled execution without external CronJob dependency
- Full UI integration: create, edit, suspend/resume, trigger, view history
- Clean separation: CronTask creates, global cleanup deletes
- Works naturally with existing Agent features (standby, quota, capacity)

### Negative

- New CRD and controller add complexity
- Requires `robfig/cron` dependency for cron expression parsing
- Users must configure global cleanup when using CronTask to prevent Task accumulation
