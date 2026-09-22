# Git Auto-Sync

Git contexts can be configured to automatically sync with the remote repository.
This enables GitOps workflows where pushing to Git triggers Agent updates.

## Sync Policies

| Policy | Mechanism | Pod Restart | Best For |
|--------|-----------|-------------|----------|
| **HotReload** (default) | Sidecar `git-sync` pulls in-place | No | Prompts, docs, context files |
| **Rollout** | Controller detects change, triggers rolling update | Yes | Workspace root, configs loaded at startup |

## HotReload Example

```yaml
spec:
  contexts:
  - name: team-prompts
    type: Git
    git:
      repository: https://github.com/org/prompts.git
      ref: main
      sync:
        enabled: true
        interval: 5m        # default
        policy: HotReload   # default
    mountPath: prompts/
```

## Rollout Example

```yaml
spec:
  contexts:
  - name: agent-config
    type: Git
    git:
      repository: https://github.com/org/agent-config.git
      ref: main
      sync:
        enabled: true
        interval: 10m
        policy: Rollout
    mountPath: "."
```

## Task Protection (Rollout Policy)

When the controller detects a remote change with Rollout policy, it checks for active Tasks:
- **No active Tasks**: proceeds with rolling update immediately
- **Active Tasks exist**: sets `GitSyncPending` condition and waits
- **Safety timeout (1 hour)**: forces rollout even if Tasks are still running

New Tasks are **not blocked** during `GitSyncPending` — only the Deployment rollout is delayed.

## Reloading the Server (HotReload)

HotReload updates files in place, which is enough for content the agent reads at
use time (prompts, docs, context files). Content that OpenCode snapshots at
instance start — most notably **skills** — needs more: the file on disk can be
current while the running server still holds the old version.

Set `sync.reload: true` to have the sidecar ask the server to re-scan after an
update lands:

```yaml
contexts:
- name: shared-config
  type: Git
  git:
    repository: https://github.com/org/config.git
    sync:
      enabled: true
      policy: HotReload
      reload: true    # re-scan the server after an update
  mountPath: config/
```

The reload is **deferred while any session is busy**, because re-scanning
re-initializes the server instance and would abort a turn in progress. It is
retried every cycle, and an owed reload survives a sidecar restart.

Skill sources reload automatically when synced and do not need this flag — see
[Skills](skills.md#keeping-skills-up-to-date).

## Sync Status

Agent status tracks the sync state:

```yaml
status:
  gitSyncStatuses:
  - name: team-prompts
    commitHash: "a1b2c3d4e5f6..."
    lastSynced: "2026-04-02T10:30:00Z"
```
