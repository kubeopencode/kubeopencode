# ADR 0015: Repo as Agent — Dynamic Image Building for Agent Environments

## Status

Proposed

## Context

KubeOpenCode currently requires pre-built container images (`agentImage` + `executorImage`) pushed to a registry before an Agent can be used. The typical workflow is:

1. Write a Dockerfile for the agent environment
2. Build the image locally or in CI
3. Push to a container registry
4. Reference the image in the Agent spec

### The Problem

1. **Manual Build-Push Cycle**: Every agent environment change requires building and pushing images outside KubeOpenCode. This creates friction, especially for teams without established CI/CD pipelines for agent images.

2. **Environment Diversity**: Different companies, teams, and use cases require different agent environments (Python ML stack, Node.js frontend tools, Java enterprise tooling, etc.). Pre-building every variant is impractical.

3. **GitOps Gap**: While Tasks and Agents are declarative Kubernetes resources, the agent environment (container image) is managed outside the Kubernetes lifecycle. This breaks the GitOps model.

### Use Case: Custom Agent Environment

A typical scenario driving this decision:
- A data science team needs an agent with specific Python packages (numpy, pandas, scikit-learn, transformers)
- They maintain a Dockerfile in their team's Git repository
- They want to create an Agent that automatically builds from this Dockerfile
- When they update the Dockerfile (e.g., add a new package), the Agent environment should update accordingly

### Vision: "Repo as Agent"

A Git repository IS the agent definition. The repo contains a Dockerfile (or build instructions), and KubeOpenCode dynamically builds the image. This eliminates the manual build-push cycle and makes agents truly declarative and GitOps-friendly.

### Current Architecture

The existing codebase provides strong integration points for this feature:

- **Two-Container Pattern** (`pod_builder.go`): Init containers already support ordered multi-step initialization. A build step fits naturally.
- **Git Context System** (`context_types.go`): `GitContext` already supports repo cloning with auth, depth, ref, and submodules — directly reusable for Dockerfile sources.
- **Lazy Image Resolution** (`pod_builder.go`): `ResolveAgentConfig()` resolves images at Pod creation time. A build-then-resolve pattern fits cleanly.
- **Agent Controller** (`agent_controller.go`): Already manages infrastructure for Server mode (Deployment + Service). Adding build management is a natural extension.

## Decision

### Build Backend: BuildKit gRPC

We choose **BuildKit** as the build backend for the following reasons:

1. **Docker's official build engine** — actively maintained, production-proven
2. **Go gRPC SDK** (`github.com/moby/buildkit` client package) — enables programmatic builds from our controller without CRD abstractions
3. **Rootless mode** — reduces security concerns compared to alternatives
4. **Excellent caching** — layer-based caching reduces rebuild times significantly
5. **Kaniko is archived** (June 2025) — no longer a viable option
6. **kpack requires additional CRDs** — adds dependency complexity without proportional benefit for our use case

### In-Cluster Registry: Zot

We choose **Zot** as the in-cluster registry:

1. **Lightweight, OCI-native** — purpose-built for OCI artifacts
2. **Helm chart available** — easy to deploy as a sub-chart
3. **Production-ready** — used in production by multiple organizations
4. **Minimal footprint** — lower resource requirements than Docker Distribution

### Both Are Optional

BuildKit and Zot are deployed as optional Helm sub-charts (`build.enabled: false` by default). This preserves KubeOpenCode's "no external dependencies" philosophy. Users who want dynamic builds opt in explicitly.

### API Design

#### New Types on Agent Spec

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: ml-agent
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent

  # NEW: Build executor image from source
  build:
    git:
      repository: https://github.com/company/agent-envs.git
      ref: main
      secretRef:
        name: github-creds
    # Path to Dockerfile within the repo (default: "Dockerfile")
    dockerfile: agents/python-ml/Dockerfile
    # Build context directory within the repo (default: ".")
    contextDir: agents/python-ml/
    # Registry to push built image (required)
    registry:
      url: registry.internal.svc.cluster.local:5000
      secretRef:
        name: registry-creds
    # Rebuild if cache older than this (default: 24h)
    cacheTTL: 24h
```

When `build` is set, `executorImage` is ignored — the controller builds the image and uses the resulting digest.

#### Build Status on Agent Status

```yaml
status:
  buildStatus:
    # Resolved image reference with digest
    image: registry.internal.svc.cluster.local:5000/kubeopencode/ml-agent@sha256:abc123...
    # When the image was last built
    lastBuildTime: "2026-03-23T10:30:00Z"
    # Source commit that was built
    sourceCommit: "a1b2c3d"
  conditions:
    - type: ImageReady
      status: "True"
      reason: BuildSucceeded
      message: "Image built successfully from commit a1b2c3d"
```

### Key Design Decisions

#### 1. Build Owned by Agent Controller (Not Task Controller)

**Decision**: The Agent controller manages builds. The Task controller only consumes the built image.

**Rationale**:
- Separation of concerns — building is an Agent lifecycle concern, not a Task concern
- Image is built once and reused across many Tasks
- Matches the existing pattern where Agent controller manages Server mode infrastructure
- Avoids blocking Task creation on build completion

#### 2. Content-Addressable Caching

**Decision**: Cache key = hash of (Dockerfile content + build context tree hash + base image digest). Only rebuild when the cache key changes or `cacheTTL` expires.

**Rationale**:
- Eliminates redundant builds when nothing changed
- `cacheTTL` provides an escape hatch for base image security updates
- Content-addressable approach is deterministic and debuggable

#### 3. Task Queuing During Build

**Decision**: When an Agent has `build` config but no `buildStatus.image` yet, Tasks referencing this Agent enter `Queued` phase with reason `ImageBuilding`.

**Rationale**:
- Reuses existing queuing mechanism (same as `maxConcurrentTasks` queuing)
- Tasks automatically start when the build completes
- Clear status signal to users about why their Task is waiting

#### 4. Build Logs as Events

**Decision**: Build progress and errors are surfaced as Kubernetes Events on the Agent resource.

**Rationale**:
- Standard Kubernetes pattern for operational events
- Visible via `kubectl describe agent <name>`
- No need for custom log streaming infrastructure

## Consequences

### Positive

1. **Eliminates manual build-push cycle** — users define a Dockerfile in Git, KubeOpenCode handles the rest
2. **True GitOps for agent environments** — version control, PR review, and audit trail for agent environments come for free
3. **Lower barrier to entry** — new users start with a Dockerfile instead of understanding registries and tagging
4. **Reuses existing Git infrastructure** — `GitContext` secret management, shallow clone, and submodule support are reusable
5. **Competitive differentiator** — no other Kubernetes AI agent platform offers "point to a repo and go"
6. **Backward compatible** — existing Agents with `executorImage` work unchanged

### Negative

1. **First-run latency** — image builds take minutes (downloading base images, installing packages). Mitigated by caching and `cacheTTL`.
2. **Infrastructure dependencies** — requires BuildKit + Zot. Mitigated by making them optional Helm sub-charts.
3. **New failure mode** — build errors (bad Dockerfiles, network issues) are harder to debug than "image not found". Mitigated by surfacing build logs as Events.
4. **Security surface** — executing arbitrary Dockerfiles in-cluster. Mitigated by rootless BuildKit and optional base image allowlisting.
5. **Resource consumption** — builds consume CPU/memory. Mitigated by resource limits on BuildKit pods.
6. **Registry storage** — built images accumulate. Mitigated by garbage collection policies (future phase).

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Build latency frustrates users | Content-addressable caching; only rebuild on actual changes; clear `ImageBuilding` status |
| BuildKit security concerns | Rootless mode; network policies; resource limits |
| Registry storage grows unbounded | Garbage collection policy (Phase 2); `cacheTTL` limits stale images |
| Scope creep dilutes core focus | Phased implementation; `build.enabled: false` by default; clear boundaries |
| Build failures block all Tasks | Tasks fail gracefully with clear error; existing `executorImage` still works as fallback |

## Implementation Roadmap

### Phase 1: MVP

1. **API Changes** (`api/v1alpha1/agent_types.go`)
   - Add `BuildConfig` struct with `git`, `dockerfile`, `contextDir`, `registry`, `cacheTTL` fields
   - Add `Build *BuildConfig` to `AgentSpec`
   - Add `BuildStatus` (image, lastBuildTime, sourceCommit) to `AgentStatus`
   - Add `ImageReady` condition type

2. **Build Logic** (`internal/controller/agent_controller.go`)
   - Extend Agent controller to watch for `build` config
   - Use BuildKit gRPC client to trigger builds
   - Store resulting image digest in Agent status
   - Requeue periodically to check for source changes

3. **Task Integration** (`internal/controller/pod_builder.go`, `task_controller.go`)
   - `ResolveAgentConfig()` checks `buildStatus.image` when `build` is configured
   - If image not ready, queue Task with reason `ImageBuilding`

4. **Infrastructure** (`charts/kubeopencode/`)
   - Optional BuildKit Deployment sub-chart
   - Optional Zot registry Deployment sub-chart
   - `build.enabled: false` by default

### Phase 2: Caching & Optimization

- Content-addressable caching (Dockerfile hash + context tree hash)
- Pre-pull DaemonSet for frequently used built images
- Build log streaming to Task events
- Registry garbage collection

### Phase 3: Advanced Features

- Inline Dockerfile support (no Git repo needed)
- Build args from Task spec (per-task customization)
- Webhook-triggered rebuilds on Git push
- Base image allowlisting policy

## Alternatives Considered

### Alternative 1: Recommend External CI Only

Document how to use GitHub Actions, Tekton, or ArgoCD to build agent images. No code changes.

**Rejected because**: Does not achieve the "Repo as Agent" vision. Users still manage a separate build pipeline.

### Alternative 2: CLI Tool for Local Builds

Provide `koc agent build --from-repo <url>` that builds locally and pushes.

**Rejected because**: Requires local Docker/BuildKit installation. Not GitOps-friendly. Manual step in the workflow.

### Alternative 3: kpack CRDs

Controller creates kpack `Image` CRD resources; kpack controller handles builds.

**Rejected because**: Adds external CRD dependency. kpack is less actively maintained than BuildKit. Requires separate kpack operator installation.

### Alternative 4: Kaniko Init Container

Add kaniko as an init container that builds the image before the agent container starts.

**Rejected because**: Kaniko was archived in June 2025 by GoogleContainerTools. No longer maintained.

## References

- [BuildKit GitHub Repository](https://github.com/moby/buildkit)
- [BuildKit Kubernetes Driver](https://docs.docker.com/build/builders/drivers/kubernetes/)
- [BuildKit Go SDK](https://pkg.go.dev/github.com/moby/buildkit)
- [Zot Registry](https://github.com/project-zot/zot)
- [Zot Kubernetes Installation](https://zotregistry.dev/v2.1.0/install-guides/install-guide-k8s/)
- [Kaniko Archived (June 2025)](https://thehapyone.com/the-end-of-an-era-kaniko-has-been-archived/)
- Existing KubeOpenCode patterns: `agent_controller.go` (Server mode), `context_types.go` (Git context)
