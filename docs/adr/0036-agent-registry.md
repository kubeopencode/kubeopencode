# ADR 0036: Agent Registry — Agent Asset Catalog and Visual Agent Assembly

## Status

Proposed (supersedes [ADR 0015](archived/0015-repo-as-agent-dynamic-image-building.md))

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

All three asset types follow the same principle — **index only, never store, never build**:

| Asset  | Storage                           | Registry's Role         |
|--------|-----------------------------------|-------------------------|
| Skill  | User's Git repository             | Index + status check    |
| Plugin | npm registry (public or corporate)| Index + status check    |
| Image  | User's container registry (Harbor, ACR, ECR, GHCR, etc.) | Index + status check |

The Registry is like building blocks — each piece (image, skill, plugin) already exists somewhere else. The Registry simply catalogues them so users can discover and assemble agents visually. **Image building is explicitly out of scope** — it is delegated to external CI/CD pipelines or a future dedicated build controller.

### Related ADRs

- **ADR 0015** — Repo as Agent: dynamic image building (superseded by this ADR)
- **ADR 0026** — Skills as a top-level Agent field
- **ADR 0034** — Plugin support

## Decision

### Registry CRD: A Namespace-Scoped Asset Catalog

The **Registry** is a new **namespace-scoped CRD** that serves as a catalog of agent assets. It is a pure index — it does not store or build anything. Skills reference Git repos, plugins reference npm, and images reference already-built container images in external registries.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Registry
metadata:
  name: team-alpha
  namespace: dev
spec:
  # Check interval for asset status validation (default: 10m)
  checkInterval: 15m

  # Executor images — references to pre-built container images
  images:
    - name: go-dev
      # Full image reference (required) — must already exist in an external registry
      image: "harbor.company.com/kubeopencode/go-dev:1.23"
      # Optional: credentials for pulling from private registries (for status checks)
      secretRef:
        name: harbor-pull-credentials
      metadata:
        description: "Go 1.23 development environment with LSP support"
        category: backend
        tags: [golang, backend, grpc]
        tools: [go, gopls, delve, golangci-lint, protoc]
        baseImage: "ubuntu:24.04"
        maintainer: "platform-team@company.com"

    - name: node-dev
      image: "harbor.company.com/kubeopencode/node-dev:22"
      metadata:
        description: "Node.js 22 with TypeScript, React, and testing tools"
        category: frontend
        tags: [nodejs, typescript, frontend, react]
        tools: [node, npm, pnpm, tsx, playwright]
        baseImage: "ubuntu:24.04"

    - name: python-ml
      image: "ghcr.io/company/agent-images/python-ml:3.12"
      metadata:
        description: "Python 3.12 with PyTorch, Transformers, and Jupyter"
        category: data-science
        tags: [python, ml, ai, pytorch]
        tools: [python, pip, jupyter, pytest]
        baseImage: "nvidia/cuda:12.4-runtime-ubuntu24.04"

    - name: devbox-default
      # The default KubeOpenCode devbox image — no secretRef needed for public images
      image: "ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest"
      metadata:
        description: "General-purpose development environment (default)"
        category: general
        tags: [general, multi-language]
        tools: [git, docker, curl, jq, python, node, go]

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
      phase: Ready           # Ready | Unavailable
      image: "harbor.company.com/kubeopencode/go-dev:1.23"
      digest: "sha256:abc123..."
      lastChecked: "2026-05-07T10:05:00Z"
    - name: python-ml
      phase: Unavailable
      message: "image not found: ghcr.io/company/agent-images/python-ml:3.12"
      lastChecked: "2026-05-07T10:05:00Z"

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

  summary:
    images: 4
    skills: 3
    plugins: 2
    readyCount: 5
    totalCount: 9
```

### What the Registry Controller Does

The Registry controller reconciles each Registry resource and performs status checks. All checks run **in-process** using Go libraries — no external binaries required. The controller **never builds anything** — it only validates that referenced assets exist and are reachable.

**For Images:**
1. Validate the container image exists in the registry via **registry API** (`HEAD` manifest check)
2. Record the resolved digest in status
3. If `secretRef` is specified, use credentials for private registry access
4. Update `status.images[].phase` to Ready or Unavailable

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

Git checks use `go-git` (already proven in the ecosystem — Flux, Argo CD use the same approach). npm checks are a single HTTP GET to the registry API — no `npm` CLI needed. Image checks use the [go-containerregistry](https://github.com/google/go-containerregistry) library (crane) — a single `HEAD` request to the manifest endpoint. All are non-blocking with context timeouts.

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
- **Images**: Add/edit/remove image references with rich descriptions → see registry reachability and digest
- **Skills**: Add/edit/remove skill references → see Git reachability and latest commit
- **Plugins**: Add/edit/remove plugin references → see npm package availability and version

**2. Agent Assembly**
Users pick components from the Registry catalog and the UI generates AgentTemplate YAML:

```
┌─────────────────────────────────────────────────┐
│            Registry: team-alpha                  │
│                                                  │
│  Executor Images (pick one):                     │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐   │
│  │  go-dev    │ │  node-dev  │ │ python-ml  │   │
│  │    ✓       │ │            │ │            │   │
│  │  Ready     │ │   Ready    │ │Unavailable │   │
│  │  Go 1.23   │ │  Node 22   │ │ Python 3.12│   │
│  │  backend   │ │  frontend  │ │data-science│   │
│  └────────────┘ └────────────┘ └────────────┘   │
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

Only assets with **Ready** status can be selected for assembly. The status indicators help users immediately see which assets are usable. Image cards display rich metadata (description, category, tools) to help users choose the right executor environment.

### Helm Integration

The Registry components are part of the unified KubeOpenCode Helm chart. Since Registry requires no infrastructure (no in-cluster registry, no build daemons), it only needs:

```yaml
# charts/kubeopencode/values.yaml (additions)
registry:
  enabled: false          # Set to true to enable Registry CRD reconciler
```

When `registry.enabled: false` (default), the Registry reconciler is not started. No additional Deployments, PVCs, or Services are created. KubeOpenCode works exactly as today.

When `registry.enabled: true`, the only new component is the Registry reconciler running inside the existing Controller process. No additional Pods, Jobs, or infrastructure are created.

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

Registry Controller actions (all in-process, no external Jobs):
  Images:  HEAD manifest check ──validate──→ update status
  Skills:  go-git ls-remote    ──check────→ update status
  Plugins: HTTP GET npm API    ──check────→ update status
```

**Note**: No in-cluster registry (Zot), no build daemon (Kaniko/BuildKit), no npm registry (Verdaccio) is deployed. The Registry is a pure CRD + Controller feature with zero infrastructure dependencies. Image building is handled externally (CI/CD, dedicated build controller, or manual `docker push`).

### API Definition

```go
// Registry is a namespace-scoped catalog of agent assets.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Images",type=integer,JSONPath=`.status.summary.images`
// +kubebuilder:printcolumn:name="Skills",type=integer,JSONPath=`.status.summary.skills`
// +kubebuilder:printcolumn:name="Plugins",type=integer,JSONPath=`.status.summary.plugins`
// +kubebuilder:printcolumn:name="Ready",type=integer,JSONPath=`.status.summary.readyCount`
// +kubebuilder:printcolumn:name="Total",type=integer,JSONPath=`.status.summary.totalCount`
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
    // asset availability (images, skills, plugins).
    // If not specified, the controller defaults to 10 minutes.
    // +optional
    CheckInterval *metav1.Duration `json:"checkInterval,omitempty"`

    // Images defines executor image references available for agent assembly.
    // Each entry references a pre-built container image in an external registry.
    // The Registry does not build images — it only indexes and validates them.
    Images []RegistryImage `json:"images,omitempty"`
    // Skills defines skill references (reuses existing SkillSource types).
    Skills []RegistrySkill `json:"skills,omitempty"`
    // Plugins defines plugin references (reuses existing PluginSpec type).
    Plugins []RegistryPlugin `json:"plugins,omitempty"`
}

// RegistryImage defines a reference to a pre-built container image.
// The image must already exist in an external container registry.
// The Registry controller validates that the image is reachable and records its digest.
type RegistryImage struct {
    // Name is a unique identifier for this image within the Registry.
    // +required
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=63
    // +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
    Name string `json:"name"`
    // Image is the full container image reference (e.g., "harbor.company.com/team/go-dev:1.23").
    // Must include registry, repository, and tag or digest.
    // +required
    Image string `json:"image"`
    // SecretRef references a Secret containing registry credentials for private images.
    // The Secret must be of type kubernetes.io/dockerconfigjson.
    // Used by the controller for image existence validation (HEAD manifest check).
    // +optional
    SecretRef *RegistrySecretReference `json:"secretRef,omitempty"`
    // Metadata provides human-readable information for the UI,
    // helping users choose the right executor image for their use case.
    // +optional
    Metadata ImageMetadata `json:"metadata,omitempty"`
}

// ImageMetadata provides rich descriptive information for executor images.
// This metadata helps users browse and select the right image in the Visual Assembler UI.
type ImageMetadata struct {
    // Description is a human-readable summary of what this image provides.
    // Example: "Go 1.23 development environment with LSP support and debugging tools"
    // +optional
    Description string `json:"description,omitempty"`
    // Category classifies the image for filtering in the UI.
    // Examples: "backend", "frontend", "data-science", "devops", "general"
    // +optional
    Category string `json:"category,omitempty"`
    // Tags are searchable labels for discovery.
    // Examples: ["golang", "backend", "grpc"]
    // +optional
    Tags []string `json:"tags,omitempty"`
    // Tools lists the key tools/binaries available in the image.
    // Displayed in the UI to help users understand what the image provides.
    // Examples: ["go", "gopls", "delve", "golangci-lint"]
    // +optional
    Tools []string `json:"tools,omitempty"`
    // BaseImage indicates the base OS/runtime image (informational).
    // Example: "ubuntu:24.04", "nvidia/cuda:12.4-runtime-ubuntu24.04"
    // +optional
    BaseImage string `json:"baseImage,omitempty"`
    // Maintainer is the contact for this image (informational).
    // Example: "platform-team@company.com"
    // +optional
    Maintainer string `json:"maintainer,omitempty"`
}

// RegistrySecretReference references a Secret for container registry authentication.
// This is a separate type from GitSecretReference intentionally:
// - GitSecretReference expects username/password or ssh-privatekey (Git credentials)
// - RegistrySecretReference expects kubernetes.io/dockerconfigjson (Docker credentials)
// Keeping them separate allows independent evolution (e.g., adding namespace or
// credential rotation fields) without affecting Git credential semantics.
type RegistrySecretReference struct {
    // Name of the Secret containing registry credentials.
    // +required
    Name string `json:"name"`
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
// Note: PluginSpec includes an Options field (runtime configuration).
// In the Registry context, Options is ignored — it exists only because
// we reuse the PluginSpec type as-is. Plugin options are runtime concerns
// and belong in Agent/AgentTemplate, not in the catalog.
type RegistryPlugin struct {
    // Name is a unique identifier for this plugin within the Registry.
    // +required
    // +kubebuilder:validation:MinLength=1
    Name string `json:"name"`
    // Plugin specifies the plugin package (reuses existing PluginSpec).
    // Only name and target are meaningful in the Registry; options is ignored.
    Plugin PluginSpec `json:"plugin"`
    // Metadata provides human-readable information for the UI.
    // +optional
    Metadata AssetMetadata `json:"metadata,omitempty"`
}

// AssetMetadata provides human-readable information for UI display (skills and plugins).
type AssetMetadata struct {
    Description string   `json:"description,omitempty"`
    Tags        []string `json:"tags,omitempty"`
    // RequiredCredentials lists env vars that must be set (only meaningful for plugins).
    RequiredCredentials []CredentialRequirement `json:"requiredCredentials,omitempty"`
}

type CredentialRequirement struct {
    Env         string `json:"env"`
    Description string `json:"description,omitempty"`
}

// AssetPhase represents the availability status of a registry asset.
// +kubebuilder:validation:Enum=Ready;Unavailable
type AssetPhase string

const (
    AssetPhaseReady       AssetPhase = "Ready"
    AssetPhaseUnavailable AssetPhase = "Unavailable"
)

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
    Name        string       `json:"name"`
    Phase       AssetPhase   `json:"phase"`             // Ready | Unavailable
    Image       string       `json:"image,omitempty"`   // Full image reference
    Digest      string       `json:"digest,omitempty"`  // Resolved digest from registry
    LastChecked *metav1.Time `json:"lastChecked,omitempty"`
    Message     string       `json:"message,omitempty"`
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
    Images     int `json:"images"`
    Skills     int `json:"skills"`
    Plugins    int `json:"plugins"`
    ReadyCount int `json:"readyCount"`
    TotalCount int `json:"totalCount"`
}
```

### Key Design Decisions

#### 1. Registry as Catalog, Not Storage or Builder

**Decision**: The Registry CRD is a pure catalog/index. It never stores or builds assets. Skills stay in Git, plugins stay in npm, images stay in the user's container registry. The Registry only references and validates them.

**Rationale**:
- Skills are SKILL.md files — Git is the natural home. No benefit from repackaging as OCI artifacts.
- Plugins are npm packages — the npm ecosystem already handles versioning, distribution, and caching.
- Images already exist in the user's container registry — enterprises already run Harbor, Nexus, ACR, ECR, or GHCR.
- Building images is a separate concern (CI/CD, GitOps, or a dedicated build controller). Mixing build and catalog responsibilities creates unnecessary complexity.
- Reusing existing types (`GitSkillSource`, `PluginSpec`) means zero migration — all existing Agent/AgentTemplate configurations work unchanged.

#### 2. No Image Building in Registry

**Decision**: The Registry does not build images. It only references pre-built images. Image building is delegated to external systems (CI/CD pipelines, a future dedicated build controller, or manual `docker build && docker push`).

**Rationale**:
- Separation of concerns: catalog (discovery) vs. build (construction) are fundamentally different responsibilities
- Build infrastructure (Kaniko Jobs, build context management, retry logic, credential mounting for push) adds significant controller complexity
- Enterprise teams already have CI/CD pipelines that build and push images
- A future dedicated build CRD/controller can handle image building if needed, without polluting the Registry's clean catalog semantics
- Keeping Registry simple makes it faster to implement and easier to maintain

#### 3. Rich Image Metadata for User Selection

**Decision**: Executor images include rich metadata (description, category, tags, tools, baseImage, maintainer) beyond what skills and plugins need.

**Rationale**:
- Choosing an executor image is the most impactful decision when assembling an agent — it determines the entire development environment
- Users need to understand what tools are available, what language runtimes are installed, and what the image is designed for
- Category-based filtering (backend, frontend, data-science, devops) enables quick narrowing in the UI
- Tool lists help users verify that required tooling is available before selecting
- BaseImage and maintainer provide traceability for enterprise governance

#### 4. AgentTemplate as Assembly Output

**Decision**: The Visual Assembler generates AgentTemplate YAML. No "recipe" or intermediate abstraction.

**Rationale**:
- AgentTemplate already exists with established merge semantics
- Generated YAML is self-contained and auditable
- Teams can share via `kubectl`, Git, or GitOps workflows

#### 5. Namespace-Scoped Registry CRD

**Decision**: Registry is namespace-scoped. Each team/namespace can have its own Registry.

**Rationale**:
- Teams manage their own asset catalogs independently
- RBAC naturally scoped — standard Kubernetes namespace permissions apply
- Multiple Registries per namespace are allowed (e.g., `team-alpha-dev`, `team-alpha-prod`)

**Cross-Registry image name conflicts**: Two Registries in the same namespace could define images with the same name but different references. Since images are just references to external registries, there is no storage-level conflict. The controller does **not** enforce cross-Registry uniqueness — this is by design, as each Registry is an independent catalog.

#### 6. Status-Driven UI

**Decision**: The Registry controller actively checks asset availability and surfaces status. The UI shows ready/unavailable indicators.

**Rationale**:
- Users see immediately which assets are usable vs. broken
- Missing images are discovered at catalog time, not at Agent deployment time
- Git credential issues are caught at catalog time, not at Task runtime
- Periodic re-checks keep status fresh (deleted images, stale Git refs, yanked npm packages)

#### 7. In-Process Status Checks

**Decision**: All status checks (images, skills, plugins) run in the Controller process using Go libraries. No external binaries or temporary Jobs.

**Rationale**:
- `go-git` is battle-tested in the Kubernetes ecosystem (Flux CD, Argo CD)
- npm registry API is a simple JSON REST endpoint (`GET https://registry.npmjs.org/{package}`)
- Container registry manifest check uses `go-containerregistry` (crane) — a `HEAD` request to the manifest API
- Controller image stays minimal — no `git`, `npm`, or `crane` binaries needed
- Goroutines with context timeouts prevent blocking the reconcile loop
- Creating temporary Pods for periodic checks would be excessive (scheduling overhead every 10 minutes per asset)

### Supersedes ADR 0015

ADR 0015 proposed embedding builds in the Agent controller via `Agent.spec.build`. This ADR takes a fundamentally different approach: the Registry is a pure catalog with no build capability.

| Aspect | ADR 0015 | ADR 0036 (this) |
|--------|----------|----------------|
| Image handling | Build from Dockerfile | Reference pre-built images |
| Build engine | BuildKit | None (out of scope) |
| Image storage | In-cluster registry | External registry (user-provided) |
| Image lifecycle | Tied to Agent | Independent — reference in many Agents |
| Skill/Plugin management | Not addressed | Registry catalog with status checks |
| User experience | YAML-only | UI with CRUD + visual assembler |
| Infrastructure deps | BuildKit daemon + in-cluster registry | None |

## Implementation Plan

### Phase 1: Registry CRD + Controller + CLI

**Goal**: Registry CRD with status checks for all asset types (images, skills, plugins).

1. **API**: Add `Registry` and `RegistryList` CRDs to `api/v1alpha1/`
2. **Controller**: `registry_controller.go` — reconcile status checks:
   - Images: `go-containerregistry` manifest HEAD check
   - Skills: `go-git` ls-remote
   - Plugins: npm registry HTTP GET
3. **CLI**: `kubeoc registry list`, `kubeoc registry show <name>`
4. **RBAC**: Update Helm ClusterRoles (controller, server, web-user)
5. **Tests**: Unit + integration tests
6. **Documentation**: Architecture docs, YAML examples

### Phase 2: UI + Visual Assembler

**Goal**: Web UI for asset management and agent assembly.

1. **Registry pages**: CRUD for images, skills, plugins with status indicators
2. **Image browser**: Category filtering, tool search, rich metadata display
3. **Assembler**: Pick components → generate AgentTemplate YAML → copy button
4. **Server API**: REST endpoints for Registry CRUD + assembly

### Phase 3: Enterprise Features

1. **Audit trail** — Track asset changes
2. **Multi-cluster** — Registry replication across clusters
3. **Observability** — Prometheus metrics for status check latency, asset counts
4. **Image policy** — Allowlist/denylist for image registries or tags

## Consequences

### Positive

1. **Zero infrastructure dependencies** — No in-cluster registry, no build daemons, no Jobs. Registry is a pure CRD + Controller feature.
2. **Consistent "index only" design** — All three asset types (skills, plugins, images) follow the same pattern: catalog references, never store, never build.
3. **Enterprise-friendly** — Works with existing container registries (Harbor, ACR, ECR, GHCR). No new infrastructure to manage.
4. **Zero migration** — Existing `SkillSource` and `PluginSpec` types are reused as-is. No changes to Agent or AgentTemplate CRDs.
5. **Status visibility** — Controller actively checks asset health. Users know immediately if an image is missing, a Git repo is unreachable, or an npm package is gone.
6. **Namespace-scoped governance** — Teams manage their own catalogs. Standard Kubernetes RBAC applies.
7. **Visual assembly** — UI lowers the barrier to creating well-configured Agents. Rich image metadata helps users choose the right executor environment.
8. **Single Helm chart** — Enable with `--set registry.enabled=true`. No separate install, no additional Deployments.
9. **Simple controller** — No build orchestration, no Job management, no retry logic. Controller only performs lightweight status checks.
10. **Clean separation of concerns** — Build responsibility is explicitly out of scope. A future build controller can be added independently without modifying Registry.

### Negative

1. **No integrated build experience** — Users must build and push images externally before referencing them in the Registry. No "upload Dockerfile and build" workflow.
2. **Status checks are best-effort** — `git ls-remote`, npm API calls, and registry manifest checks can fail for transient reasons. Status may flicker.
3. **No air-gap story for plugins** — Without an in-cluster npm registry, air-gapped clusters need an externally managed npm mirror. This is explicitly out of scope.
4. **New CRD** — Adds a new resource type that teams need to learn.
5. **Configuration drift** — Changes to Registry (e.g., updated Git URL for a skill) do not automatically propagate to previously generated AgentTemplates. Users must re-generate.
6. **Image metadata is manual** — Description, tools, category must be manually maintained by the user. There is no automatic introspection of image contents.

### Risks

| Risk | Mitigation |
|------|------------|
| UI development takes too long | Phase 1 delivers value via CLI and CRD before UI is ready |
| Git credential issues in status checks | Use `go-git` shallow ls-remote (no clone); clear error messages in status |
| Registry status becomes stale | Configurable `checkInterval`; manual trigger via annotation |
| Image metadata becomes outdated | Metadata is informational — incorrect metadata does not affect agent functionality; UI can highlight `lastChecked` timestamps |
| Users want integrated build | Document recommended CI/CD patterns; consider a future dedicated build CRD |

## Alternatives Considered

### Alternative 1: In-Cluster Zot Registry

Deploy Zot as an in-cluster OCI registry for storing images.

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

### Alternative 7: Build Images in Registry Controller (Kaniko)

Include Kaniko-based image building in the Registry controller itself.

**Rejected (descoped) because:**
- Mixes catalog and build responsibilities — violates separation of concerns
- Build orchestration (Job lifecycle, retry logic, in-flight cancellation, credential mounting) adds significant controller complexity
- Enterprise teams already have CI/CD pipelines for image building
- A dedicated build controller can be added later as a separate CRD without polluting Registry semantics
- Keeping Registry as a pure catalog makes it simpler to implement, test, and maintain

### Alternative 8: Shipwright + Tekton for Builds

Use Shipwright CRDs for build lifecycle management.

**Rejected because:**
- Over-engineering for our use case
- Two additional operator dependencies
- Build is out of scope for Registry — if needed, a simpler dedicated build CRD is preferred

## References

- [go-containerregistry](https://github.com/google/go-containerregistry) — Go library for interacting with container registries
- [go-git](https://github.com/go-git/go-git) — Pure Go Git implementation
- [ADR 0015](archived/0015-repo-as-agent-dynamic-image-building.md) — Dynamic image building (superseded)
- [ADR 0026](0026-skills.md) — Skills as a top-level Agent field
- [ADR 0034](0034-plugin-support-and-slack-integration.md) — Plugin support
