# ADR 0027: Git Context Auto-Sync for Agents

## Status

Accepted

## Date

2026-04-02

## Context

Currently, Git contexts in KubeOpenCode are **clone-once**: the `git-init` init container clones the repository at Pod startup, and the content is never updated afterwards. If the remote repository changes (e.g., new commits pushed to the tracked branch), the Agent continues using the stale content until the Pod is manually restarted.

This creates friction for GitOps workflows where teams manage prompts, context files, SKILL.md, or configuration in Git repositories and expect Agents to automatically pick up changes when commits are pushed.

### Requirements

1. Agent should detect remote Git changes and update local content automatically
2. No external image dependencies — the sync functionality must be built into the `kubeopencode` binary
3. Minimize disruption to running Tasks
4. Opt-in per Git context (not all Git contexts need auto-sync)

### Constraints

- KubeOpenCode's architecture delegates event-driven triggers to Argo Events — we should NOT build webhook receivers
- The `kubeopencode` binary already has `git-init` subcommand; extending it is natural
- Agent always runs as a Deployment, so sidecar and rollout patterns both apply

## Decision

We add a `sync` field to `GitContext` and a new `kubeopencode git-sync` subcommand. Users choose between two sync policies — `HotReload` (sidecar in-place update) and `Rollout` (controller-driven Pod restart) — both delivered in a single implementation.

### API Changes

```go
type GitSyncPolicy string

const (
    // GitSyncPolicyHotReload updates files in-place without restarting the Pod.
    // A git-sync sidecar periodically pulls changes into the shared volume.
    GitSyncPolicyHotReload GitSyncPolicy = "HotReload"

    // GitSyncPolicyRollout triggers a Deployment rolling update when changes are detected.
    // The Agent controller periodically checks the remote ref and triggers rollout on change.
    GitSyncPolicyRollout GitSyncPolicy = "Rollout"
)

type GitSync struct {
    // Enabled enables periodic sync of the Git repository.
    // +optional
    Enabled bool `json:"enabled,omitempty"`

    // Interval is the polling interval for checking remote changes.
    // Format: Go duration string (e.g., "5m", "1h").
    // Default: "5m".
    // +optional
    Interval string `json:"interval,omitempty"`

    // Policy determines how changes are applied.
    // HotReload (default): sidecar pulls changes in-place, no Pod restart.
    // Rollout: controller detects changes and triggers Deployment rolling update.
    // +kubebuilder:validation:Enum=HotReload;Rollout
    // +kubebuilder:default=HotReload
    // +optional
    Policy GitSyncPolicy `json:"policy,omitempty"`
}

type GitContext struct {
    // ... existing fields (Repository, Path, Ref, Depth, etc.) ...

    // Sync configures automatic synchronization of the Git repository.
    // Only applicable to Agent contexts (ignored for Task contexts).
    // +optional
    Sync *GitSync `json:"sync,omitempty"`
}
```

### YAML Examples

#### HotReload — sidecar pulls changes, no restart

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  contexts:
  - name: team-prompts
    type: Git
    git:
      repository: https://github.com/org/prompts.git
      ref: main
      sync:
        enabled: true
        interval: 5m
        policy: HotReload   # default, can be omitted
    mountPath: prompts/
```

#### Rollout — controller triggers Pod restart on change

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  contexts:
  - name: agent-workspace
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

### Implementation

#### 1. `kubeopencode git-sync` Subcommand

A new long-lived subcommand that runs as a sidecar container, serving the **HotReload** policy.

Environment variables (same pattern as `git-init`):
- `GIT_REPO` — repository URL
- `GIT_REF` — branch/tag to track
- `GIT_ROOT` — local clone root directory (default: `/git`)
- `GIT_LINK` — subdirectory name (default: `repo`)
- `GIT_SYNC_INTERVAL` — polling interval in seconds (default: `300`)
- Auth variables: same as `git-init` (`GIT_USERNAME`, `GIT_PASSWORD`, `GIT_SSH_KEY`, etc.)

Behavior:
1. On startup, verify the clone exists (created by `git-init` init container)
2. Loop every `GIT_SYNC_INTERVAL` seconds:
   a. Run `git fetch origin <ref>`
   b. Compare local HEAD with `origin/<ref>`
   c. If different, run `git reset --hard origin/<ref>`
   d. Log the old and new commit hashes
3. On error, log and continue (do not crash — stale content is better than no content)

#### 2. Pod Construction (HotReload)

When `sync.enabled == true && sync.policy == HotReload`:
- The existing `git-init` init container still runs (initial clone)
- An additional `git-sync` sidecar container is added to the Deployment
- The Git volume remains `emptyDir` (shared between init container, sidecar, and worker container)
- Credentials are mounted into the sidecar (unlike `git-init` which cleans them up after clone)

```
┌─────────────────────────────────────────────────────┐
│ Pod                                                 │
│                                                     │
│  ┌─────────────┐   init    ┌─────────────────────┐  │
│  │  git-init   │ ────────► │   emptyDir volume    │  │
│  │ (clone once)│           │  /git/repo/...       │  │
│  └─────────────┘           └──────┬──────┬────────┘  │
│                                   │      │           │
│  ┌─────────────┐   pull    ───────┘      │           │
│  │  git-sync   │ ◄────────               │           │
│  │ (sidecar)   │   loop                  │           │
│  └─────────────┘                         │           │
│                                          ▼           │
│  ┌─────────────────────────────────────────────────┐ │
│  │  worker (opencode serve)                        │ │
│  │  reads from /workspace/prompts/                 │ │
│  └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

#### 3. Controller-Driven Rollout

When `sync.enabled == true && sync.policy == Rollout`:
- No sidecar is added (no `git-sync` container)
- The Agent controller handles change detection and triggers rollout

**Controller loop:**
1. During reconcile, for each Git context with `sync.policy == Rollout`:
   - Run `git ls-remote <repo> <ref>` to get the latest remote commit hash
   - Compare with the hash stored in a Pod template annotation: `kubeopencode.io/git-hash-<context-name>`
2. If the hash differs:
   - Check for running Tasks (Task protection — see below)
   - If safe to proceed, update the annotation on the Deployment's Pod template
   - Kubernetes automatically triggers a rolling update
3. Store sync status in `Agent.status`:

```go
type GitSyncStatus struct {
    // Name is the context name.
    Name string `json:"name"`
    // CommitHash is the last observed remote commit hash.
    CommitHash string `json:"commitHash"`
    // LastSynced is the timestamp of the last successful sync check.
    LastSynced metav1.Time `json:"lastSynced"`
}
```

**RequeueAfter:** The controller uses `ctrl.Result{RequeueAfter: interval}` to implement periodic polling. If multiple Git contexts have different intervals, use the shortest one.

**Task Protection:** Before triggering a rollout, the controller checks for active Tasks bound to this Agent:
- If active Tasks exist → set condition `GitSyncPending=True` with message, delay rollout
- Requeue after a short interval (e.g., 30s) to re-check
- Once no active Tasks remain → proceed with rollout, clear `GitSyncPending`
- Safety timeout: if Tasks run longer than 1 hour after sync detected, proceed with rollout anyway and log a warning

```
┌─────────────────────────────────────────────────────────┐
│ Agent Controller                                        │
│                                                         │
│  reconcile()                                            │
│    │                                                    │
│    ├─► git ls-remote <repo> <ref>                       │
│    │     └─► compare with annotation hash               │
│    │                                                    │
│    ├─► if changed:                                      │
│    │     ├─► check running Tasks                        │
│    │     │     ├─► Tasks running → set GitSyncPending   │
│    │     │     │   requeue after 30s                    │
│    │     │     └─► No tasks → proceed                   │
│    │     │                                              │
│    │     └─► update pod template annotation             │
│    │           └─► Deployment rolling update triggered   │
│    │                                                    │
│    └─► RequeueAfter: sync.interval                      │
└─────────────────────────────────────────────────────────┘
```

#### 4. Controller Git Execution

The controller needs to execute `git ls-remote` for the Rollout policy. Since the `kubeopencode` image already includes `git` (used by `git-init`), we shell out to `git ls-remote` directly. This keeps the approach consistent and avoids adding a Go Git library dependency.

For authenticated repos, the controller must have access to the referenced Secret. This is already handled by the existing RBAC (controller has read access to Secrets in managed namespaces).

### Policy Comparison

| Aspect | HotReload | Rollout |
|--------|-----------|---------|
| Mechanism | Sidecar `git-sync` pulls in-place | Controller detects change, triggers Deployment rollout |
| Pod restart | No | Yes (rolling update) |
| Task disruption | None | Delayed until Tasks complete (with timeout) |
| Latency | `interval` | `interval` + rollout time |
| Resource overhead | One sidecar container per Git context | None (controller-side) |
| Best for | Prompts, docs, context files | Workspace root, SKILL.md, configs that need fresh init |

### When to Use Which

- **HotReload**: The Git context provides supplementary files (prompts, docs, reference material) that the Agent reads on demand. Files update transparently; the Agent does not need to restart.
- **Rollout**: The Git context provides the workspace root (`mountPath: "."`) or files that are loaded once at Agent startup (e.g., `opencode.json`, SKILL.md). A full Pod restart is needed for changes to take effect.

## Consequences

### Positive

- Enables GitOps workflow: push to Git → Agent automatically updates
- Built into `kubeopencode` binary — no external image dependencies
- Opt-in per context — existing behavior unchanged by default
- Two policies cover all use cases: hot reload for speed, rollout for correctness
- Consistent with existing patterns (`git-init` subcommand, env var config, Pod builder)
- Task protection prevents disruption to running work

### Negative

- HotReload sidecar adds resource overhead per Agent (minimal — one lightweight container)
- Credentials must persist in sidecar (cannot cleanup after init like `git-init` does)
- Polling introduces latency (bounded by `interval`); for real-time needs, users can use Argo Events to annotate the Agent externally
- Rollout policy adds complexity to the Agent controller (git command execution, Task protection logic)
- Controller executing `git ls-remote` requires network access to Git repos from the controller Pod

### Neutral

- Does not replace Argo Events for event-driven triggers — complementary approach
- Task contexts are unaffected (Tasks are ephemeral, always clone fresh)

## Alternatives Considered

### Use `registry.k8s.io/git-sync/git-sync` Image

The Kubernetes community maintains a mature `git-sync` sidecar image. However:
- Adds an external image dependency, conflicting with KubeOpenCode's single-binary philosophy
- Less control over behavior and logging format
- Credential handling differs from our `git-init` pattern

**Rejected**: prefer building into `kubeopencode` binary for consistency and zero external dependencies.

### Webhook Receiver

Build a webhook endpoint in the kubeopencode server to receive GitHub/GitLab push events.

**Rejected**: KubeOpenCode's architecture explicitly delegates event-driven triggers to Argo Events. Building webhook receivers would duplicate that responsibility and require handling multiple Git platform APIs.

### Annotation-Based Manual Trigger Only

Require users to manually annotate Agents (e.g., `kubeopencode.io/git-refresh=true`) or use Argo Events to do so.

**Not rejected but insufficient alone**: this is a valid escape hatch and should work alongside auto-sync. However, requiring external tooling for basic "pull latest" is too much friction for the common case.

## Manual Testing Scenarios

The following scenarios require a real Git repository and a running cluster (local-dev Kind).
They cannot be fully automated in unit tests because they depend on real `git push` events.

### Scenario 1: Rollout on Remote Change

**Precondition**: Agent with `sync.policy: Rollout` pointing to a repo you can push to.

1. Create Agent with Rollout sync (interval ~30-60s)
2. Wait for initial `status.gitSyncStatuses[].commitHash` to be set
3. Push a new commit to the remote repo
4. Wait for one sync interval
5. **Verify**: commit hash in status and pod template annotation updates to the new hash
6. **Verify**: old Pod enters `Terminating`, new Pod starts `Running`

### Scenario 2: Task Protection Delays Rollout

**Precondition**: Agent with `sync.policy: Rollout` and a way to create a long-running Task (e.g., Agent with `command: ["sleep", "300"]`).

1. Create Agent with Rollout sync
2. Create a Task against the Agent (Task stays in Running state via sleep)
3. Push a new commit to the remote repo
4. Wait for one sync interval
5. **Verify**: `GitSyncPending` condition becomes `True` with message "Waiting for N active task(s)..."
6. **Verify**: Pod is NOT restarted (annotation keeps old hash)
7. Stop the Task (`kubectl annotate task <name> kubeopencode.io/stop=true`)
8. **Verify**: within seconds, `GitSyncPending` becomes `False`, hash updates, rollout executes
