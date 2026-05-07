# ADR 0036: Agent Registry — Agent Asset Catalog and Visual Agent Assembly

## Status

Proposed (supersedes [ADR 0015](0015-repo-as-agent-dynamic-image-building.md))

## Date

2026-05-07

## Context

### The Assembly Problem

Building a useful KubeOpenCode Agent today requires manually assembling multiple independent pieces:

```yaml
spec:
  agentImage: "ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest"    # Where to find alternatives?
  executorImage: "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest"   # One size fits all?
  skills:
    - name: my-skills
      git:
        repository: "https://github.com/???/???"    # How to discover?
  plugins:
    - name: "???"                                     # Which ones exist?
  credentials: [...]                                   # How to configure?
```

Users must discover each component independently, understand how they fit together, and manually compose the YAML. There is no central catalog, no discovery mechanism, and no way to visually assemble an agent.

### The Enterprise Governance Problem

In enterprise environments, teams need visibility into what agent assets (images, skills, plugins) are available, what versions are in use, and whether upstream sources are reachable. There is no centralized place to manage and browse these assets.

### Design Principle: Reuse, Don't Reinvent

KubeOpenCode already has well-defined types for skills (`SkillSource` with `GitSkillSource`) and plugins (`PluginSpec` with npm packages). These work. The problem is not storage or format — it is **discovery, management, and assembly**. The Registry should be a **catalog layer on top of existing types**, not a new storage system.

All three asset types follow the same principle — **index only, never store**:

| Asset  | Storage                           |
|--------|-----------------------------------|
| Skill  | User's Git repository             |
| Plugin | npm registry (public or corporate)|
| Image  | User's container registry (Harbor, ACR, ECR, GHCR, etc.) |

### Related ADRs

- **ADR 0015** — Repo as Agent: dynamic image building (superseded by this ADR)
- **ADR 0026** — Skills as a top-level Agent field
- **ADR 0034** — Plugin support

## Decision

### Registry CRD: A Namespace-Scoped Asset Catalog

The **Registry** is a new **namespace-scoped CRD** that serves as a catalog of agent assets. It is an index — it does not store skills, plugins, or images itself. Skills reference Git repos, plugins reference npm, and images are built from Dockerfiles and pushed to an **external container registry** provided by the user.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Registry
metadata:
  name: team-alpha
  namespace: dev
spec:
  # Check interval for skill/plugin status validation (default: 10m)
  checkInterval: 15m

  # External container registry for built images
  imageRegistry:
    # Default prefix — images without explicit target use this
    prefix: "harbor.company.com/kubeopencode"
    # Push credentials (must contain .dockerconfigjson)
    secretRef:
      name: registry-push-credentials

  # Agent images — built from Dockerfile, pushed to external registry
  images:
    - name: go-dev
      dockerfile:
        context: "https://github.com/company/agent-images.git"
        path: "go/Dockerfile"
        ref: "main"
      # Optional: override imageRegistry.prefix for this image
      target: "harbor.company.com/team-alpha/go-dev"
      metadata:
        description: "Go development environment"
        tags: [golang, backend]
        tools: [go, gopls, delve]

    - name: node-dev
      dockerfile:
        context: "https://github.com/company/agent-images.git"
        path: "node/Dockerfile"
      # Uses imageRegistry.prefix → harbor.company.com/kubeopencode/node-dev
      metadata:
        description: "Node.js development environment"
        tags: [nodejs, frontend]

    - name: custom-ml
      dockerfile:
        # Inline Dockerfile (stored in ConfigMap by controller)
        inline: |
          FROM python:3.12-slim
          RUN pip install torch transformers
      metadata:
        description: "ML/AI development environment"
        tags: [python, ml]

  # Build configuration for Kaniko Jobs
  build:
    # Resource limits for build Jobs (prevents unbounded memory usage)
    resources:
      requests:
        cpu: "500m"
        memory: "1Gi"
      limits:
        cpu: "2"
        memory: "4Gi"
    # Number of retries on build failure (default: 2)
    retryLimit: 2
    # TTL for completed/failed build Jobs (default: 3600 = 1 hour)
    ttlSecondsAfterFinished: 3600

  # Skills — references using existing SkillSource types
  skills:
    - name: golang-best-practices
      git:
        repository: "https://github.com/company/skills.git"
        names: [golang]
      metadata:
        description: "Go coding standards and best practices"
        tags: [golang, standards]

    - name: code-review
      git:
        repository: "https://github.com/company/skills.git"
        names: [code-review]
      metadata:
        description: "Code review guidelines"
        tags: [review, quality]

    - name: k8s-ops
      git:
        repository: "https://github.com/company/sre-skills.git"
        names: [kubernetes-ops]
        secretRef:
          name: git-credentials
      metadata:
        description: "Kubernetes operations runbook"
        tags: [kubernetes, sre]

  # Plugins — references using existing PluginSpec type
  plugins:
    - name: slack-integration
      plugin:
        name: "@kubeopencode/opencode-slack-plugin@0.8.2"
        target: server
      metadata:
        description: "Slack bot integration"
        tags: [slack, messaging]
        requiredCredentials:
          - env: SLACK_BOT_TOKEN
          - env: SLACK_APP_TOKEN

    - name: otel-observability
      plugin:
        name: "@kubeopencode/opencode-otel-plugin@1.0.0"
        target: server
      metadata:
        description: "OpenTelemetry tracing for agent operations"
        tags: [observability, tracing]

status:
  # Reflects the generation that status was last reconciled for
  observedGeneration: 3

  # Controller populates status for each asset
  images:
    - name: go-dev
      phase: Ready          # Pending | Building | Ready | Failed
      image: "harbor.company.com/team-alpha/go-dev@sha256:abc123..."
      digest: "sha256:abc123..."
      buildTime: "2026-05-07T10:00:00Z"
    - name: node-dev
      phase: Building
      buildJobName: "registry-team-alpha-build-node-dev-1715000000"
    - name: custom-ml
      phase: Failed
      message: "pip install torch: network timeout"

  skills:
    - name: golang-best-practices
      phase: Ready           # Ready | Unavailable
      latestCommit: "a1b2c3d"
      lastChecked: "2026-05-07T10:05:00Z"
    - name: k8s-ops
      phase: Unavailable
      message: "git ls-remote failed: authentication required"

  plugins:
    - name: slack-integration
      phase: Ready           # Ready | Unavailable
      resolvedVersion: "0.8.2"
      lastChecked: "2026-05-07T10:05:00Z"
    - name: otel-observability
      phase: Unavailable
      message: "npm registry: package not found"
```

### What the Registry Controller Does

The Registry controller reconciles each Registry resource and performs status checks. All checks run **in-process** using Go libraries — no external binaries required.

**For Images:**
1. If no built image exists (or Dockerfile changed) → create a **Kaniko Job** to build and push to the external registry
2. Track build progress via Job status → update `status.images[].phase`
3. On success, record the image reference and digest in status → image is available for use
4. On failure, surface the error message; retry up to `spec.build.retryLimit` times
5. If spec is updated while a build is in-flight → delete the existing Job, create a new one
6. Completed/failed Jobs are cleaned up after `spec.build.ttlSecondsAfterFinished`
7. On Registry deletion → ownerReferences on Jobs trigger garbage collection

**For Skills:**
1. Validate the Git repository is reachable via `go-git` library (`ls-remote`, no binary dependency)
2. Record the latest commit SHA
3. If `secretRef` is specified, verify the Secret exists
4. Update `status.skills[].phase` to Ready or Unavailable

**For Plugins:**
1. Validate the npm package exists via HTTP call to npm registry API (`GET https://registry.npmjs.org/{package}`)
2. Resolve the actual version (handle semver ranges)
3. Update `status.plugins[].phase` to Ready or Unavailable

All status checks use goroutines with `context.WithTimeout` to prevent blocking the reconcile loop. The controller re-checks periodically based on `spec.checkInterval` (default 10 minutes).

#### Controller Execution: Why In-Process

| Approach | Pros | Cons |
|----------|------|------|
| In-process (chosen) | No extra binaries, no Pod scheduling overhead | Controller process needs network access |
| Temporary Jobs per check | Full isolation | Massive overhead — scheduling a Pod every 10 min per asset |
| Sidecar container | Dedicated network context | Complicates Controller Deployment |

Git checks use `go-git` (already proven in the ecosystem — Flux, Argo CD use the same approach). npm checks are a single HTTP GET to the registry API — no `npm` CLI needed. Both are non-blocking with context timeouts.

### Relationship to AgentTemplate and Agent

The Registry is a **catalog** — it does not directly configure Agents. The flow is:

```
Registry (catalog)                    AgentTemplate / Agent (runtime config)
┌─────────────────────┐              ┌──────────────────────────────┐
│ images:             │              │ spec:                        │
│   - go-dev (Ready)  │──reference──→│   executorImage: harbor/...  │
│   - node-dev        │              │   skills:                    │
│                     │              │     - name: my-skills        │
│ skills:             │──reference──→│       git: {same ref}        │
│   - golang (Ready)  │              │   plugins:                   │
│   - code-review     │              │     - name: "slack@0.8"      │
│                     │──reference──→│                              │
│ plugins:            │              └──────────────────────────────┘
│   - slack (Ready)   │
└─────────────────────┘
         ↑
    UI assembles from Registry
    → generates AgentTemplate YAML
```

The Visual Assembler UI reads Registry resources to show available assets, then generates an **AgentTemplate** (or Agent) YAML that directly embeds the skill/plugin/image references. The generated YAML is self-contained — it does not reference the Registry at runtime. This means:

- Deleting a Registry does not affect running Agents
- AgentTemplate works independently of Registry
- Registry is purely a management and discovery layer

**Trade-off acknowledged**: Because the generated YAML is decoupled from the Registry, changes to the Registry (e.g., updating a skill's Git URL) do NOT automatically propagate to existing AgentTemplates. Users must re-generate the YAML to pick up changes. This is intentional — it prevents unexpected configuration drift in production Agents.

### Visual Assembler UI

The Registry UI provides two capabilities:

**1. Asset Management (CRUD)**
- **Images**: Upload Dockerfile (inline or Git URL) → build → see build logs → manage versions
- **Skills**: Add/edit/remove skill references → see Git reachability and latest commit
- **Plugins**: Add/edit/remove plugin references → see npm package availability and version

**2. Agent Assembly**
Users pick components from the Registry catalog and the UI generates AgentTemplate YAML:

```
┌─────────────────────────────────────────────────┐
│            Registry: team-alpha                  │
│                                                  │
│  Images (pick one):                              │
│  ┌────────┐ ┌────────┐ ┌────────┐               │
│  │ go-dev │ │node-dev│ │  ml    │               │
│  │   ✓    │ │        │ │        │               │
│  │ Ready  │ │Building│ │ Failed │               │
│  └────────┘ └────────┘ └────────┘               │
│                                                  │
│  Skills (pick any):                              │
│  ☑ golang-best-practices  ● Ready  a1b2c3d      │
│  ☑ code-review            ● Ready  f4e5d6a      │
│  ☐ k8s-ops                ○ Unavailable          │
│                                                  │
│  Plugins (pick any):                             │
│  ☑ slack-integration      ● Ready  0.8.2         │
│  ☐ otel-observability     ○ Unavailable          │
│                                                  │
│  [ Generate AgentTemplate YAML ]                 │
└─────────────────────────────────────────────────┘
```

Only assets with **Ready** status can be selected for assembly. The status indicators help users immediately see which assets are usable.

### In-Cluster Image Building: Kaniko

When a Registry defines images, the controller creates **Kaniko Jobs** to build Dockerfiles and push the resulting images to the user's external container registry.

#### Why Kaniko

BuildKit rootless mode **requires `allowPrivilegeEscalation: true`** — a hard Linux kernel requirement (`newuidmap` setuid binary). This blocks PSS Restricted clusters. Kaniko builds entirely in userspace:

```yaml
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]
  runAsNonRoot: true
  seccompProfile:
    type: RuntimeDefault
```

| Aspect | Kaniko | BuildKit |
|--------|--------|----------|
| **PSS Restricted** | Yes | No — requires `allowPrivilegeEscalation: true` |
| **Deployment** | Stateless K8s Jobs | Persistent Deployment (daemon) |
| **Multi-arch** | Not supported | Supported |
| **Maintenance** | [Chainguard fork](https://github.com/chainguard-forks/kaniko) actively maintained | Official moby/buildkit |

**Note on Kaniko status**: The [original GoogleContainerTools/kaniko](https://github.com/GoogleContainerTools/kaniko) repository is archived. This design depends on the actively maintained [Chainguard fork](https://github.com/chainguard-forks/kaniko). The Helm default image should reference the Chainguard fork, not third-party builds.

#### Build Flow

```
User uploads Dockerfile (via UI or Registry spec)
       ↓
Registry Controller creates Kaniko Job
  → --destination={target or prefix/name}
  → --digest-file=/dev/termination-log
  → Push credentials mounted from imageRegistry.secretRef
       ↓
Kaniko builds image → pushes to external registry
       ↓
Controller reads Job termination message → extracts digest
       ↓
Controller updates Registry status with image reference + digest
       ↓
Image available for selection in Visual Assembler
```

#### Build Job Lifecycle

| Scenario | Behavior |
|----------|----------|
| Build succeeds | Job retained for `ttlSecondsAfterFinished`, status updated to Ready |
| Build fails | Retried up to `retryLimit` times, then marked Failed with error message |
| Spec updated during build | In-flight Job deleted, new Job created |
| Registry deleted | Jobs garbage-collected via ownerReferences |
| Registry deleted during build | ownerReferences trigger Job deletion; partial images may remain in external registry (user's responsibility to clean up) |

### Helm Integration

The Registry components are part of the unified KubeOpenCode Helm chart. Since Registry requires no infrastructure (no in-cluster registry, no persistent daemons), it only needs:

```yaml
# charts/kubeopencode/values.yaml (additions)
registry:
  enabled: false          # Set to true to enable Registry CRD reconciler
  build:
    kaniko:
      image: cgr.dev/chainguard/kaniko:latest
```

When `registry.enabled: false` (default), the Registry reconciler is not started. No additional Deployments, PVCs, or Services are created. KubeOpenCode works exactly as today.

When `registry.enabled: true`, the only new component is the Registry reconciler running inside the existing Controller process. Build Jobs are created on-demand and cleaned up automatically.

### Architecture

```
KubeOpenCode Helm Chart (kubeopencode-system namespace)
┌────────────────────────────────────────────────────────┐
│  Core components (always deployed):                     │
│  ┌──────────────────────────────────────────────┐      │
│  │  Controller (Deployment)                      │      │
│  │  • Agent, Task, CronTask reconcilers          │      │
│  │  • Registry reconciler (if registry.enabled)  │      │
│  └──────────────────────────────────────────────┘      │
│  ┌──────────────────────────────────────────────┐      │
│  │  Server + UI (Deployment, port 2746)          │      │
│  │  • Agents, Tasks, CronTasks pages             │      │
│  │  • Registry pages (CRUD + Assembler)          │      │
│  └──────────────────────────────────────────────┘      │
└────────────────────────────────────────────────────────┘

No additional infrastructure components required.

Registry Controller actions:
  Images:  create Kaniko Job ──build──→ push to external registry
  Skills:  go-git ls-remote ──check──→ update status
  Plugins: HTTP GET npm registry API ──check──→ update status
```

**Note**: No in-cluster registry (Zot), no persistent build daemon (BuildKit), no npm registry (Verdaccio) is deployed. The Registry is a pure CRD + Controller feature with zero infrastructure dependencies.

### API Definition

```go
// Registry is a namespace-scoped catalog of agent assets.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Images",type=integer,JSONPath=`.status.summary.images`
// +kubebuilder:printcolumn:name="Skills",type=integer,JSONPath=`.status.summary.skills`
// +kubebuilder:printcolumn:name="Plugins",type=integer,JSONPath=`.status.summary.plugins`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.summary.ready`
type Registry struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              RegistrySpec   `json:"spec,omitempty"`
    Status            RegistryStatus `json:"status,omitempty"`
}

// RegistryList contains a list of Registry resources.
// +kubebuilder:object:root=true
type RegistryList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []Registry `json:"items"`
}

type RegistrySpec struct {
    // CheckInterval defines how often the controller re-validates
    // skill and plugin availability. Defaults to "10m".
    // +optional
    // +kubebuilder:default="10m"
    CheckInterval *metav1.Duration `json:"checkInterval,omitempty"`

    // ImageRegistry configures the external container registry for built images.
    // Required if spec.images is non-empty.
    // +optional
    ImageRegistry *ImageRegistryConfig `json:"imageRegistry,omitempty"`

    // Build configures Kaniko build Job defaults.
    // +optional
    Build *BuildConfig `json:"build,omitempty"`

    // Images defines container images to build and push.
    Images []RegistryImage `json:"images,omitempty"`
    // Skills defines skill references (reuses existing SkillSource types).
    Skills []RegistrySkill `json:"skills,omitempty"`
    // Plugins defines plugin references (reuses existing PluginSpec type).
    Plugins []RegistryPlugin `json:"plugins,omitempty"`
}

// ImageRegistryConfig configures the external container registry for pushing built images.
type ImageRegistryConfig struct {
    // Prefix is the default registry/repository prefix for built images.
    // Images without an explicit Target will be pushed to {Prefix}/{ImageName}:{tag}.
    // Example: "harbor.company.com/kubeopencode", "gcr.io/my-project/agents"
    // +required
    Prefix string `json:"prefix"`

    // SecretRef references a Secret containing registry push credentials.
    // The Secret must be of type kubernetes.io/dockerconfigjson.
    // +required
    SecretRef RegistrySecretReference `json:"secretRef"`
}

// RegistrySecretReference references a Secret for container registry authentication.
type RegistrySecretReference struct {
    // Name of the Secret containing registry credentials.
    // +required
    Name string `json:"name"`
}

// BuildConfig defines defaults for Kaniko build Jobs.
type BuildConfig struct {
    // Resources specifies compute resources for build Jobs.
    // Builds (e.g., compiling Go, installing PyTorch) can be resource-intensive.
    // If not set, no resource limits are applied (inherits namespace defaults).
    // +optional
    Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

    // RetryLimit is the number of retries on build failure. Defaults to 2.
    // +optional
    // +kubebuilder:default=2
    // +kubebuilder:validation:Minimum=0
    // +kubebuilder:validation:Maximum=10
    RetryLimit *int32 `json:"retryLimit,omitempty"`

    // TTLSecondsAfterFinished defines how long completed/failed build Jobs
    // are retained before cleanup. Defaults to 3600 (1 hour).
    // +optional
    // +kubebuilder:default=3600
    // +kubebuilder:validation:Minimum=0
    TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`
}

// RegistryImage defines a container image to build from a Dockerfile.
type RegistryImage struct {
    // Name is a unique identifier for this image within the Registry.
    // +required
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=63
    // +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
    Name string `json:"name"`
    // Dockerfile specifies how to build the image.
    Dockerfile DockerfileBuild `json:"dockerfile"`
    // Target overrides imageRegistry.prefix for this image.
    // If set, the image is pushed to this exact reference (minus tag/digest).
    // Example: "harbor.company.com/team-alpha/go-dev"
    // +optional
    Target string `json:"target,omitempty"`
    // Metadata provides human-readable information for the UI.
    // +optional
    Metadata AssetMetadata `json:"metadata,omitempty"`
}

// +kubebuilder:validation:XValidation:rule="has(self.context) || has(self.inline)",message="either context or inline must be specified"
// +kubebuilder:validation:XValidation:rule="!(has(self.context) && has(self.inline))",message="context and inline are mutually exclusive"
type DockerfileBuild struct {
    // Context is a Git repository URL containing the Dockerfile.
    // Mutually exclusive with Inline.
    // +optional
    Context string `json:"context,omitempty"`
    // Path is the Dockerfile path within the context. Defaults to "Dockerfile".
    // +optional
    Path string `json:"path,omitempty"`
    // Ref is the Git ref to checkout. Defaults to "HEAD".
    // +optional
    Ref string `json:"ref,omitempty"`
    // Inline is an inline Dockerfile content. Mutually exclusive with Context.
    // +optional
    Inline string `json:"inline,omitempty"`
    // SecretRef references a Secret for Git credentials (used when Context is a private repo).
    // Reuses the existing GitSecretReference type.
    // +optional
    SecretRef *GitSecretReference `json:"secretRef,omitempty"`
}

// RegistrySkill wraps SkillSource with catalog metadata.
// The Git field reuses the existing GitSkillSource type from SkillSource.
type RegistrySkill struct {
    // Name is a unique identifier for this skill within the Registry.
    // +required
    // +kubebuilder:validation:MinLength=1
    Name string `json:"name"`
    // Git specifies the skill source (reuses existing GitSkillSource).
    Git *GitSkillSource `json:"git,omitempty"`
    // Metadata provides human-readable information for the UI.
    // +optional
    Metadata AssetMetadata `json:"metadata,omitempty"`
}

// RegistryPlugin wraps PluginSpec with catalog metadata.
type RegistryPlugin struct {
    // Name is a unique identifier for this plugin within the Registry.
    // +required
    // +kubebuilder:validation:MinLength=1
    Name string `json:"name"`
    // Plugin specifies the plugin package (reuses existing PluginSpec).
    Plugin PluginSpec `json:"plugin"`
    // Metadata provides human-readable information for the UI.
    // +optional
    Metadata AssetMetadata `json:"metadata,omitempty"`
}

// AssetMetadata provides human-readable information for UI display.
type AssetMetadata struct {
    Description string   `json:"description,omitempty"`
    Tags        []string `json:"tags,omitempty"`
    // Tools lists tools available in an image (only meaningful for images).
    Tools []string `json:"tools,omitempty"`
    // RequiredCredentials lists env vars that must be set (only meaningful for plugins).
    RequiredCredentials []CredentialRequirement `json:"requiredCredentials,omitempty"`
}

type CredentialRequirement struct {
    Env         string `json:"env"`
    Description string `json:"description,omitempty"`
}

// RegistryStatus tracks the status of all assets.
type RegistryStatus struct {
    // ObservedGeneration reflects the generation of the spec that was last reconciled.
    ObservedGeneration int64          `json:"observedGeneration,omitempty"`
    Images             []ImageStatus  `json:"images,omitempty"`
    Skills             []SkillStatus  `json:"skills,omitempty"`
    Plugins            []PluginStatus `json:"plugins,omitempty"`
    Summary            StatusSummary  `json:"summary,omitempty"`
}

type ImageStatus struct {
    Name         string       `json:"name"`
    Phase        ImagePhase   `json:"phase"`             // Pending | Building | Ready | Failed
    Image        string       `json:"image,omitempty"`   // Full image reference with digest
    Digest       string       `json:"digest,omitempty"`
    BuildJobName string       `json:"buildJobName,omitempty"`
    BuildTime    *metav1.Time `json:"buildTime,omitempty"`
    Message      string       `json:"message,omitempty"`
}

type SkillStatus struct {
    Name         string       `json:"name"`
    Phase        AssetPhase   `json:"phase"`            // Ready | Unavailable
    LatestCommit string       `json:"latestCommit,omitempty"`
    LastChecked  *metav1.Time `json:"lastChecked,omitempty"`
    Message      string       `json:"message,omitempty"`
}

type PluginStatus struct {
    Name            string       `json:"name"`
    Phase           AssetPhase   `json:"phase"`          // Ready | Unavailable
    ResolvedVersion string       `json:"resolvedVersion,omitempty"`
    LastChecked     *metav1.Time `json:"lastChecked,omitempty"`
    Message         string       `json:"message,omitempty"`
}

type StatusSummary struct {
    Images  int    `json:"images"`
    Skills  int    `json:"skills"`
    Plugins int    `json:"plugins"`
    Ready   string `json:"ready"`  // e.g., "8/10"
}
```

### Key Design Decisions

#### 1. Registry as Catalog, Not Storage

**Decision**: The Registry CRD is a catalog/index. It never stores assets. Skills stay in Git, plugins stay in npm, images are pushed to an external registry provided by the user.

**Rationale**:
- Skills are SKILL.md files — Git is the natural home. No benefit from repackaging as OCI artifacts.
- Plugins are npm packages — the npm ecosystem already handles versioning, distribution, and caching.
- Images are pushed to the user's existing container registry — enterprises already run Harbor, Nexus, ACR, ECR, or GHCR. Adding an in-cluster registry (Zot) creates operational burden with no unique value.
- Reusing existing types (`GitSkillSource`, `PluginSpec`) means zero migration — all existing Agent/AgentTemplate configurations work unchanged.

#### 2. External Registry, Not In-Cluster Zot

**Decision**: Built images are pushed to an external container registry. KubeOpenCode does not deploy an in-cluster registry.

**Rationale**:
- Enterprises already have container registries with access control, audit logging, vulnerability scanning, and HA.
- An in-cluster Zot would require managing: PVC storage, garbage collection, TLS, access control, backup/restore, and high availability.
- Zot's namespace-scoped paths (`zot:5000/{namespace}/...`) are a naming convention, not an authorization boundary — no multi-tenant isolation without additional configuration.
- The "index only, never store" principle is applied consistently across all three asset types.
- Users who lack a container registry likely also don't need custom image builds — the default devbox image suffices.

#### 3. AgentTemplate as Assembly Output

**Decision**: The Visual Assembler generates AgentTemplate YAML. No "recipe" or intermediate abstraction.

**Rationale**:
- AgentTemplate already exists with established merge semantics
- Generated YAML is self-contained and auditable
- Teams can share via `kubectl`, Git, or GitOps workflows

#### 4. Namespace-Scoped Registry CRD

**Decision**: Registry is namespace-scoped. Each team/namespace can have its own Registry.

**Rationale**:
- Teams manage their own asset catalogs independently
- RBAC naturally scoped — standard Kubernetes namespace permissions apply
- Multiple Registries per namespace are allowed (e.g., `team-alpha-dev`, `team-alpha-prod`)

**Cross-Registry image name conflicts**: Two Registries in the same namespace could define images with the same name but different targets. Since images are pushed to external registries at user-specified paths, there is no storage-level conflict. However, if both use the same `imageRegistry.prefix` and same image name, the last build wins. The controller does **not** enforce cross-Registry uniqueness — this is by design, as each Registry is an independent catalog.

#### 5. Status-Driven UI

**Decision**: The Registry controller actively checks asset availability and surfaces status. The UI shows ready/unavailable indicators.

**Rationale**:
- Users see immediately which assets are usable vs. broken
- Build failures are surfaced before they affect Agent deployment
- Git credential issues are caught at catalog time, not at Task runtime
- Periodic re-checks keep status fresh (stale Git refs, yanked npm packages)

#### 6. In-Process Status Checks

**Decision**: Skill and plugin status checks run in the Controller process using Go libraries (`go-git` for Git, HTTP client for npm registry API). No external binaries or temporary Jobs.

**Rationale**:
- `go-git` is battle-tested in the Kubernetes ecosystem (Flux CD, Argo CD)
- npm registry API is a simple JSON REST endpoint (`GET https://registry.npmjs.org/{package}`)
- Controller image stays minimal — no `git` or `npm` binaries needed
- Goroutines with context timeouts prevent blocking the reconcile loop
- Creating temporary Pods for periodic checks would be excessive (scheduling overhead every 10 minutes per asset)

#### 7. Kaniko as Build Engine

**Decision**: Kaniko is the only supported build engine. BuildKit is not offered as an alternative.

**Rationale**:
- PSS Restricted compatibility is a hard requirement for enterprise clusters
- Offering two engines adds testing and documentation burden with marginal benefit
- Multi-arch builds (BuildKit's advantage) can be handled externally via CI/CD pipelines
- The Chainguard fork of Kaniko is actively maintained

### Supersedes ADR 0015

ADR 0015 proposed embedding builds in the Agent controller via `Agent.spec.build`. This ADR takes a different approach: builds are managed through the **Registry CRD**, decoupled from Agent lifecycle.

| Aspect | ADR 0015 | ADR 0036 (this) |
|--------|----------|----------------|
| Build trigger | `Agent.spec.build` | `Registry.spec.images[].dockerfile` |
| Build engine | BuildKit only | Kaniko (PSS Restricted compatible) |
| Image storage | In-cluster registry | External registry (user-provided) |
| Image lifecycle | Tied to Agent | Independent — build once, reference in many Agents |
| Skill/Plugin management | Not addressed | Registry catalog with status checks |
| User experience | YAML-only | UI with CRUD + visual assembler |
| Infrastructure deps | BuildKit daemon + in-cluster registry | None (Kaniko Jobs are ephemeral) |

## Implementation Plan

### Phase 1: Registry CRD + Controller + CLI

**Goal**: Registry CRD with status checks for skills and plugins. No image building yet.

1. **API**: Add `Registry` and `RegistryList` CRDs to `api/v1alpha1/`
2. **Controller**: `registry_controller.go` — reconcile skill/plugin status checks (go-git + npm HTTP)
3. **CLI**: `kubeoc registry list`, `kubeoc registry show <name>`
4. **RBAC**: Update Helm ClusterRoles (controller, server, web-user)
5. **Tests**: Unit + integration tests
6. **Documentation**: Architecture docs, YAML examples

### Phase 2: Image Building

**Goal**: In-cluster image building from Dockerfiles, pushing to external registries.

1. **Build orchestration**: Controller creates Kaniko Jobs with `--destination` pointing to external registry
2. **Credential mounting**: `imageRegistry.secretRef` mounted as `/kaniko/.docker/config.json`
3. **Job lifecycle**: ownerReferences, TTL cleanup, retry logic, in-flight cancellation
4. **Build log streaming**: Server API for real-time build logs
5. **CLI**: `kubeoc registry build <name>`, `kubeoc registry logs <name>`

### Phase 3: UI + Visual Assembler

**Goal**: Web UI for asset management and agent assembly.

1. **Registry pages**: CRUD for images, skills, plugins with status indicators
2. **Build UI**: Upload Dockerfile → see build logs → success/fail
3. **Assembler**: Pick components → generate AgentTemplate YAML → copy button
4. **Server API**: REST endpoints for Registry CRUD + assembly

### Phase 4: Enterprise Features

1. **Supply chain security** — Cosign signing for built images
2. **Audit trail** — Track asset changes
3. **Scheduled rebuilds** — Cron-based rebuild for security patches
4. **Multi-cluster** — Registry replication across clusters
5. **Observability** — Prometheus metrics for build duration, failure rate, status check latency

## Consequences

### Positive

1. **Zero infrastructure dependencies** — No in-cluster registry, no persistent daemons. Registry is a pure CRD + Controller feature. Build Jobs are ephemeral.
2. **Consistent "index only" design** — All three asset types (skills, plugins, images) follow the same pattern: catalog references, never store.
3. **Enterprise-friendly** — Works with existing container registries (Harbor, ACR, ECR, GHCR). No new infrastructure to manage.
4. **Zero migration** — Existing `SkillSource` and `PluginSpec` types are reused as-is. No changes to Agent or AgentTemplate CRDs.
5. **Status visibility** — Controller actively checks asset health. Users know immediately if a Git repo is unreachable or an npm package is missing.
6. **Namespace-scoped governance** — Teams manage their own catalogs. Standard Kubernetes RBAC applies.
7. **Visual assembly** — UI lowers the barrier to creating well-configured Agents.
8. **Single Helm chart** — Enable with `--set registry.enabled=true`. No separate install, no additional Deployments.
9. **PSS Restricted compatible** — Kaniko build engine works in strict security environments.

### Negative

1. **Requires external registry** — Users who want image builds must provide a container registry with push credentials. No "zero config" image building experience.
2. **Status checks are best-effort** — `git ls-remote` and npm API calls can fail for transient reasons. Status may flicker.
3. **No air-gap story for plugins** — Without an in-cluster npm registry, air-gapped clusters need an externally managed npm mirror. This is explicitly out of scope.
4. **New CRD** — Adds a new resource type that teams need to learn.
5. **Configuration drift** — Changes to Registry (e.g., updated Git URL for a skill) do not automatically propagate to previously generated AgentTemplates. Users must re-generate.
6. **No multi-arch builds** — Kaniko does not support multi-arch. Mixed ARM/x86 clusters need external CI/CD for multi-arch images.

### Risks

| Risk | Mitigation |
|------|------------|
| UI development takes too long | Phase 1-2 deliver value via CLI and CRD before UI is ready |
| Chainguard Kaniko fork abandoned | Track [chainguard-forks/kaniko](https://github.com/chainguard-forks/kaniko) and [osscontainertools/kaniko](https://github.com/osscontainertools/kaniko); low risk given active maintenance |
| Git credential issues in status checks | Use `go-git` shallow ls-remote (no clone); clear error messages in status |
| Registry status becomes stale | Configurable `checkInterval`; manual trigger via annotation |
| Build Jobs consume excessive resources | `spec.build.resources` allows setting limits; namespace ResourceQuotas also apply |

## Alternatives Considered

### Alternative 1: In-Cluster Zot Registry

Deploy Zot as an in-cluster OCI registry for storing built images.

**Rejected because:**
- Introduces significant operational burden: PVC storage management, garbage collection, TLS, HA, backup/restore
- Namespace-scoped paths are a naming convention, not an authorization boundary — no multi-tenant isolation without additional Zot configuration
- Enterprises already have container registries with access control, audit, and vulnerability scanning
- Violates the "index only, never store" principle that works well for skills (Git) and plugins (npm)
- A team without a container registry likely doesn't need custom image builds

### Alternative 2: OCI Artifacts for Skills

Store skills as OCI artifacts in an in-cluster registry. Add `skill-init` container to pull and unpack them.

**Rejected because:**
- Skills are SKILL.md files — Git is the natural, simpler storage
- OCI artifact packaging adds complexity (custom media types, crane dependency, new init container)
- Users already know Git; OCI artifacts are an unfamiliar concept for most
- No benefit over Git for text files that change infrequently

### Alternative 3: Verdaccio for In-Cluster npm

Deploy Verdaccio as an in-cluster npm registry for air-gapped plugin support.

**Deferred because:**
- Adds operational complexity (another Deployment + PVC to manage)
- Air-gapped npm is an infrastructure concern, not a KubeOpenCode concern
- Corporate npm proxies (Nexus, Artifactory, Verdaccio managed externally) already serve this purpose
- Can be added later if demand emerges

### Alternative 4: Recipe Abstraction Layer

Add a `Recipe` CRD that pre-combines image + skills + plugins.

**Rejected because:**
- AgentTemplate already serves this purpose
- Recipes introduce merge ambiguity (what if two recipes conflict?)
- Users want standard Kubernetes resources, not custom abstractions

### Alternative 5: Build Images in Agent Controller (ADR 0015)

Embed builds in the Agent controller via `Agent.spec.build`.

**Superseded because:**
- Couples image lifecycle to Agent lifecycle
- Controller becomes stateful (build tracking)
- BuildKit-only — no PSS Restricted support

### Alternative 6: Separate Helm Chart for Registry

Deploy Registry as an independent Helm chart.

**Rejected because:**
- Two Helm releases to manage
- Cross-namespace complexity
- Version compatibility concerns
- Community convention favors single charts with feature flags

### Alternative 7: Shipwright + Tekton for Builds

Use Shipwright CRDs for build lifecycle management.

**Rejected because:**
- Over-engineering for our simple requirement (Dockerfile → build → push)
- Two additional operator dependencies
- Fewer moving parts = easier to maintain

### Alternative 8: BuildKit as Alternative Build Engine

Offer BuildKit alongside Kaniko as an optional build engine.

**Rejected because:**
- BuildKit requires `allowPrivilegeEscalation: true`, incompatible with PSS Restricted
- Supporting two engines doubles testing and documentation effort
- Multi-arch (BuildKit's main advantage over Kaniko) can be handled by external CI/CD
- Single engine simplifies troubleshooting

## References

- [Kaniko](https://github.com/GoogleContainerTools/kaniko) — Userspace container image builder (archived)
- [Chainguard Kaniko Fork](https://github.com/chainguard-forks/kaniko) — Actively maintained fork
- [BuildKit](https://github.com/moby/buildkit) — Container image builder (requires privilege escalation)
- [go-git](https://github.com/go-git/go-git) — Pure Go Git implementation
- [ADR 0015](0015-repo-as-agent-dynamic-image-building.md) — Dynamic image building (superseded)
- [ADR 0026](0026-skills.md) — Skills as a top-level Agent field
- [ADR 0034](0034-plugin-support-and-slack-integration.md) — Plugin support
- [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) — PSS Restricted profile
