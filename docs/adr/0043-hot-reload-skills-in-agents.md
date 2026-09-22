# ADR 0043: Hot-Reloading Skills in Long-Running Agents

## Status

Proposed

## Date

2026-09-22

## Context

KubeOpenCode Agent pods run `opencode serve` as a long-lived process. Task pods
are ephemeral and restart per Task, so they always discover current content. An
Agent, however, can run for days.

Skills are loaded and **cached when the server instance starts**. A change to a
skill repository therefore does not reach a running Agent, even though the
cloned files on disk may be current. Git contexts already had a `sync` option
(HotReload / Rollout); skill sources had none at all, so a skill catalog could
only be updated by restarting the Agent Deployment.

Two facts, established by probing a running `opencode serve`, shape the design:

1. **File updates alone are insufficient.** After editing and even adding a
   `SKILL.md` on disk, `GET /skill` returned the cached list: a newly added skill
   was not discovered, and an edited skill still served its old description.
   Discovery happens once per instance.

2. **`POST /instance/dispose` forces a re-scan without restarting the process.**
   After disposal the new skill appeared and edited content was current. The
   server stayed healthy, and **sessions survived** — the session list and
   per-session reads were unchanged.

The same probe showed one hazard: **disposing during an active turn aborts it**.
The assistant message was terminated with `MessageAbortedError`. Disposal
releases the whole instance, not just skill state, so it must not run while a
turn is generating.

Measured cost of a reload (pinned OpenCode 1.17.11, several repos, with and
without MCP):

| Measurement | Result |
|---|---|
| `POST /instance/dispose` | ~3ms |
| Turn after dispose vs. warm | ~+3.5–4.5s on the first turn |
| Idle gap of 20s, no dispose | ~+0.5–1s (so the cost is the dispose, not idleness) |
| Memory over 30 consecutive reloads | oscillates, then settles below baseline — churn, not a leak |

The cost lands on the turn *after* the reload and is small at a 15m interval
(roughly 0.4% of one turn). There is no lighter re-scan endpoint; disposal is the
only mechanism the server exposes.

## Decision

Add `sync` to `GitSkillSource`, reusing the existing `GitSync` type so the field
shape matches Git contexts, and implement the re-scan in the existing `git-sync`
sidecar.

1. **Skills sync via a sidecar that reloads.** When `sync.enabled` is set, the
   mount gets a `git-sync` sidecar (as synced Git contexts do). On a detected
   commit change it updates the files and then notifies the server.

2. **Reloads are gated on session idleness.** The sidecar reads
   `GET /session/status` and only disposes when no session is `busy`. A busy
   agent defers the reload to the next cycle; the file update still happens
   immediately.

3. **An owed reload is persisted, not held in memory.** The commit hash last
   scanned by the server is written to the shared volume, so a restarted sidecar
   still knows the server is behind. Without this, a restart leaves the server
   stale until the repository happens to change again — possibly days.

4. **Skill sources always reload when syncing**; Git contexts opt in via
   `sync.reload`. Re-scanning is the entire point of syncing skills, so making it
   opt-in there would be a footgun. Contexts keep today's behavior by default.

5. **Reload applies to Agent servers only.** Task pods are ephemeral and never
   build `git-sync` sidecars, so the concept does not apply.

6. **Skills default to a 15m interval** versus 5m for contexts, since each
   applied change costs a reload.

7. **`Rollout` is rejected for skills.** It would require comparing remote refs
   in the controller, which is not implemented for authenticated repositories;
   allowing it would present an option that silently does nothing.

## Consequences

### Positive

- Skill catalogs can be updated on running Agents without a restart, closing a
  gap where a change required an operator to bounce a Deployment.
- The mechanism reuses the sidecar, GitHub App credential refresh, CA bundle, and
  proxy wiring that Git contexts already have.
- The idle gate means a reload never aborts a turn in progress, and the
  persisted state means a deferred reload is not lost.

### Negative

- A reload briefly re-initializes the server instance, adding a few seconds to
  the first turn after it. This is the accepted cost of live skill updates and is
  why the skill interval defaults to 15m.
- The reload is an entire-instance operation, so it also discards unrelated
  instance state (MCP connections, language servers), which then re-initialize.
- In-cluster behavior may differ from the loopback measurements above; memory
  should be watched after rollout.

### Limitations

- **`names` filtering blocks additions.** A mount using `names` maps fixed
  subpaths, so a newly added skill directory is not mounted until the Pod
  restarts. Only edits to already-mounted skills hot-reload. Omitting `names`
  makes additions dynamic. This is documented rather than worked around, as
  fixing it needs a different mount strategy.
- **Plugins are out of scope.** Plugin packages are installed into an emptyDir
  by `plugin-init`, so a new plugin's `node_modules` would not exist at reload
  time. Reloading cannot make a new plugin available; that needs a separate
  install mechanism.
- Reload depends on the `/instance/dispose` and `/session/status` endpoints, which
  are present in the pinned OpenCode version but are internal-ish surfaces.

## Alternatives Considered

- **Watch the filesystem from the server.** OpenCode does not re-scan on file
  change, so this would require upstream changes.
- **Always dispose on every sync cycle.** Simpler, but wasteful and aborts
  in-flight turns.
- **Rollout (recreate the Pod).** Already available implicitly via a Deployment
  change, but defeats the purpose for long-running Agents and interrupts work.
- **Track pending state in memory only.** Rejected after testing showed a
  restarted sidecar could leave the server permanently stale.
- **Pre-warming the instance after dispose.** Measured as ineffective — a `/skill`
  call after disposal did not reduce the next turn's latency.
