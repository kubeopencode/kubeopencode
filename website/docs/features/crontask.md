# CronTask (Scheduled Execution)

CronTask provides scheduled/recurring task execution — analogous to Kubernetes CronJob creating Jobs, CronTask creates Tasks on a cron schedule.

## Basic Usage

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: CronTask
metadata:
  name: daily-vuln-scan
spec:
  schedule: "0 9 * * 1-5"           # Every weekday at 09:00
  timeZone: "Asia/Shanghai"          # IANA timezone (default: UTC)
  concurrencyPolicy: Forbid          # Skip if previous is still running
  maxRetainedTasks: 10               # Max child Tasks (blocks creation when reached)
  taskTemplate:
    spec:
      agentRef:
        name: security-agent
      description: |
        Scan all Go dependencies for CVEs.
        If critical/high found, create a PR with the fix.
```

## Concurrency Policy

Controls behavior when a new schedule fires while a previous Task is still running:

| Policy | Behavior |
|--------|----------|
| **Forbid** (default) | Skip the new Task creation |
| **Allow** | Create new Task regardless |
| **Replace** | Stop the running Task, create new |

## maxRetainedTasks

Safety valve that counts ALL child Tasks (active + finished). When the limit is reached, the controller **blocks new Task creation** (does NOT delete old Tasks). Deletion is handled by the global `KubeOpenCodeConfig.cleanup` mechanism.

This clean separation ensures:
- CronTask is responsible for **creating** Tasks (with a cap)
- Global cleanup is responsible for **deleting** Tasks

## Manual Trigger

Trigger a CronTask to create a Task immediately:

```bash
# Via annotation (kubectl)
kubectl annotate crontask daily-vuln-scan kubeopencode.io/trigger=true

# Via API (UI uses this)
POST /api/v1/namespaces/{ns}/crontasks/{name}/trigger
```

## Suspend/Resume

```bash
# Suspend (stop creating new Tasks, existing ones continue)
kubectl patch crontask daily-vuln-scan --type merge -p '{"spec":{"suspend":true}}'

# Resume
kubectl patch crontask daily-vuln-scan --type merge -p '{"spec":{"suspend":false}}'
```

## Generated Task Naming

Tasks created by CronTask follow the pattern: `{crontask-name}-{unix-timestamp}`

Each generated Task includes:
- Label: `kubeopencode.io/crontask={crontask-name}`
- OwnerReference pointing to the CronTask (for garbage collection)

## Spec Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `schedule` | string | (required) | Cron expression (5-field: min hour dom month dow) |
| `timeZone` | string | UTC | IANA timezone |
| `concurrencyPolicy` | string | Forbid | Allow, Forbid, or Replace |
| `suspend` | bool | false | Pause scheduling |
| `startingDeadlineSeconds` | int64 | nil | Grace period for missed schedules |
| `maxRetainedTasks` | int32 | 10 | Max child Tasks before blocking creation |
| `taskTemplate` | object | (required) | Template for created Tasks (metadata + spec) |

## Status Fields

The CronTask status provides information about scheduled execution:

```bash
kubectl get crontask daily-vuln-scan -o yaml
```

```yaml
status:
  active: 1
  activeRefs:
    - name: daily-vuln-scan-1715904000
      namespace: default
      uid: abc123-def456
  lastScheduleTime: "2026-05-17T09:00:00Z"
  lastSuccessfulTime: "2026-05-17T09:12:34Z"
  nextScheduleTime: "2026-05-18T09:00:00Z"
  totalExecutions: 42
  conditions:
    - type: Ready
      status: "True"
      reason: CronTaskReady
      lastTransitionTime: "2026-01-01T00:00:00Z"
```

### Status Field Reference

| Field | Type | Description |
|-------|------|-------------|
| `active` | int32 | Number of currently active (Pending/Queued/Running) child Tasks |
| `activeRefs` | []ObjectReference | References to currently active child Tasks (name, namespace, uid) |
| `lastScheduleTime` | *metav1.Time | Last time a Task was successfully scheduled |
| `lastSuccessfulTime` | *metav1.Time | Last time a scheduled Task completed successfully |
| `nextScheduleTime` | *metav1.Time | Next calculated schedule time |
| `totalExecutions` | int64 | Total number of Tasks created by this CronTask since creation |
| `conditions` | []Condition | Standard Kubernetes conditions (type `Ready`) |
