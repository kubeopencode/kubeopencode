# ADR 0038: Extra Environment Variables and Volume Mounts for System Containers

## Status

Accepted (Implemented)

## Context

### Bug: git-init and git-sync containers crash on OpenShift with exit 255

ADR 0006 documented the decision to set `HOME=/tmp` and `SHELL=/bin/bash` in agent containers for
OpenShift SCC (Security Context Constraints) compatibility. The fix was correctly applied to the main
`opencode-server` container in `server_builder.go`, with an explicit comment explaining the rationale.

However, the same fix was **not applied** to the `git-init-*` and `git-sync-*` system containers built
in `pod_builder.go`. This caused the following failure chain on OpenShift:

1. OCP `restricted-v2` SCC assigns a random UID (e.g., `1000980000`) at runtime.
2. The `kubeopencode` image's `/etc/passwd` maps that UID to home dir `/` (the root filesystem).
3. `git-init` calls `os.UserHomeDir()` which returns `/` (from `/etc/passwd` lookup, since `HOME` is not set).
4. `setupAuth()` runs `git config --global credential.helper store`, which tries to write `/.gitconfig`.
5. Writing to `/` fails with **Permission denied** → `git config` exits with code **255**.
6. `setupAuth()` returns the error: `"failed to setup authentication: exit status 255"`.
7. `git-init-0` enters `CrashLoopBackOff` → all subsequent init containers and main containers are blocked.

The error message `"failed to setup authentication: exit status 255"` was misleading — it looked like
a credentials problem, but the root cause was a missing `HOME` environment variable.

### Feature Request: Granular env/volume injection for system containers

Users in enterprise environments (especially OpenShift) need to inject custom environment variables
and volume mounts into specific system containers. Common use cases:

- Setting `HOME=/tmp` on `git-init` / `git-sync` containers for SCC compatibility (the bug above)
- Mounting corporate CA bundles into specific init containers beyond what `caBundle` provides
- Injecting custom git configuration or proxy settings into only the git containers
- Providing secrets to the `plugin-init` container for private npm registry authentication
- Injecting variables into `opencode-init` for custom image entrypoint configuration

The existing API provided:
- `podSpec.extraVolumes` — extra volumes added to the pod (mounted on executor only)
- `podSpec.extraVolumeMounts` — extra mounts on the executor container only
- No way to inject env vars into any container
- No way to mount volumes into init containers

## Decision

### 1. Fix the bug: add HOME and SHELL to git-init and git-sync containers

Add `HOME=/tmp` and `SHELL=/bin/bash` directly to `buildGitInitContainer()` and `buildGitSyncSidecar()`
in `pod_builder.go`, following the same pattern already established for the main container.

This is a **behavioral change** but cannot cause regressions: `/tmp` is always writable, and the
containers previously crashed without this fix on SCC-enabled clusters.

### 2. Add two new types: InitContainerOverrides and SystemContainerOverrides

```go
// InitContainerOverrides allows injecting extra env vars and volume mounts
// into a specific KubeOpenCode-managed init or sidecar container.
type InitContainerOverrides struct {
    ExtraEnv          []corev1.EnvVar     `json:"extraEnv,omitempty"`
    ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`
}

// SystemContainerOverrides configures per-container-type overrides.
type SystemContainerOverrides struct {
    OpenCodeInit *InitContainerOverrides `json:"openCodeInit,omitempty"`
    ContextInit  *InitContainerOverrides `json:"contextInit,omitempty"`
    GitInit      *InitContainerOverrides `json:"gitInit,omitempty"`
    GitSync      *InitContainerOverrides `json:"gitSync,omitempty"`
    PluginInit   *InitContainerOverrides `json:"pluginInit,omitempty"`
}
```

### 3. Add two new fields to AgentPodSpec

```go
// ExtraEnv injects env vars into ALL containers (init + executor).
ExtraEnv []corev1.EnvVar `json:"extraEnv,omitempty"`

// SystemContainers provides per-container-type overrides.
SystemContainers *SystemContainerOverrides `json:"systemContainers,omitempty"`
```

Since `AgentPodSpec` is already shared between `Agent` and `AgentTemplate`, both resource types
gain these fields without any additional API surface changes.

### 4. Application order

Controller-managed values are applied first, user overrides last:

1. Controller-managed base env vars (HOME, SHELL, GIT_REPO, etc.)
2. CA bundle env/mounts (from `caBundle`)
3. Proxy env vars (from `proxy`)
4. `podSpec.extraEnv` — global, applied to ALL containers
5. `podSpec.systemContainers.*` — per-container-type, applied after global extraEnv

This ordering ensures users can override controller defaults (e.g., override `HOME` if they need
a different path) while controller-managed values remain the baseline.

### 5. Merge strategy for Agent + AgentTemplate

`podSpec` is already merged as `firstNonNilPtr(agent.Spec.PodSpec, tmpl.Spec.PodSpec)` — Agent
wins entirely if non-nil. The new `ExtraEnv` and `SystemContainers` fields inside `AgentPodSpec`
follow this same strategy automatically: if the Agent has a `podSpec`, its `extraEnv` and
`systemContainers` take precedence; if only the template has a `podSpec`, those values are inherited.

This is consistent with how all other list fields (`contexts`, `credentials`, etc.) work.

## Alternatives Considered

### Alternative: runAsUser SCC override

Setting a specific `runAsUser` in `podSpec.podSecurityContext` does not fix the root cause.
The issue is not which UID runs, but that the UID has no writable home directory in `/etc/passwd`.
Any UID assigned by OCP that isn't explicitly in the image's `/etc/passwd` with a writable home
will exhibit the same failure. This approach also requires `anyuid` SCC for fixed UIDs, which is
a security regression.

### Alternative: Patched system image with ENV HOME=/tmp

Adding `ENV HOME=/tmp` to the `kubeopencode` Dockerfile would fix the bug for all system containers.
This was considered as a workaround, but the controller-level fix (adding `HOME` to the env vars
built in `pod_builder.go`) is more aligned with the existing pattern from ADR 0006 and does not
require rebuilding the system image.

### Alternative: Selector-based extraEnv (targets: [gitInit, gitSync])

Instead of named struct fields, a list of `{env, targets: [gitInit, executor]}` entries was
considered. This is more flexible for combinations but harder to discover and more verbose for
the common case. Named struct fields are self-documenting and match the existing API style.

### Alternative: Global extraEnv only (no per-container targeting)

A simpler API with only `podSpec.extraEnv` (applied to all containers) was considered.
This is sufficient for the OCP HOME fix but insufficient for cases where users need to inject
a secret only into `git-init` (e.g., a private registry credential for `plugin-init` npm install)
without exposing it to the executor container. Per-container targeting is worth the additional
complexity.

## Consequences

### Positive

- **Bug fixed**: `git-init` and `git-sync` containers work correctly on OpenShift SCC environments
  without any user configuration required.
- **Backward compatible**: All new fields are `+optional` with `omitempty`. Zero behavior change
  for existing deployments that don't use the new fields.
- **Flexible**: Users can inject env vars and volume mounts at any granularity — global (all
  containers), per-container-type (gitInit, gitSync, etc.), or both.
- **Consistent**: The API follows the same patterns as `caBundle` and `proxy` (applied to all
  containers) and `extraVolumes`/`extraVolumeMounts` (executor-scoped).
- **Template-friendly**: Works through `AgentTemplate` inheritance with the same merge strategy
  as all other `podSpec` fields.

### Negative

- **podSpec merge is coarse-grained**: When an Agent defines `podSpec` to override any field,
  the entire `podSpec` (including `extraEnv` and `systemContainers`) from the template is discarded.
  Users who want to inherit template's `systemContainers` but override `resources` must duplicate
  the `systemContainers` in the Agent's `podSpec`. This is consistent with the existing behavior
  for `extraVolumes`, `labels`, etc., and is documented.

## References

- [ADR 0006: Environment Configuration for Agent Containers in SCC Environments](./0006-home-directory-for-agent-containers.md)
- [OpenShift SCC Documentation](https://docs.openshift.com/container-platform/latest/authentication/managing-security-context-constraints.html)
- [Go os.UserHomeDir() documentation](https://pkg.go.dev/os#UserHomeDir)
