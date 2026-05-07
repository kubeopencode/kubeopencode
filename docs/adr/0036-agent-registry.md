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

### Related ADRs

- **ADR 0015** — Repo as Agent: dynamic image building (superseded by this ADR)
- **ADR 0026** — Skills as a top-level Agent field
- **ADR 0034** — Plugin support

## Decision

### Registry CRD: A Namespace-Scoped Asset Catalog

The **Registry** is a new **namespace-scoped CRD** that serves as a catalog of agent assets. It is an index — it does not store skills or plugins itself, but references them using existing KubeOpenCode types and tracks their status.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Registry
metadata:
  name: team-alpha
  namespace: dev
spec:
  # Agent images — built and stored in Zot
  images:
    - name: go-dev
      dockerfile:
        context: "https://github.com/company/agent-images.git"
        path: "go/Dockerfile"
        ref: "main"
      metadata:
        description: "Go development environment"
        tags: [golang, backend]
        tools: [go, gopls, delve]

    - name: node-dev
      dockerfile:
        context: "https://github.com/company/agent-images.git"
        path: "node/Dockerfile"
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
  # Controller populates status for each asset
  images:
    - name: go-dev
      phase: Ready          # Pending | Building | Ready | Failed
      image: "zot.kubeopencode-system.svc:5000/dev/go-dev@sha256:abc123..."
      buildTime: "2026-05-07T10:00:00Z"
      digest: "sha256:abc123..."
    - name: node-dev
      phase: Building
      buildJobName: "registry-build-node-dev-1715000000"
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
      message: "git clone failed: authentication required"

  plugins:
    - name: slack-integration
      phase: Ready           # Ready | Unavailable
      resolvedVersion: "0.8.2"
      lastChecked: "2026-05-07T10:05:00Z"
    - name: otel-observability
      phase: Unavailable
      message: "npm view: package not found"
```

### What the Registry Controller Does

The Registry controller reconciles each Registry resource and performs status checks:

**For Images:**
1. If no built image exists (or Dockerfile changed) → create a **Kaniko Job** to build
2. Track build progress via Job status → update `status.images[].phase`
3. On success, record the image digest in status → image is available for use
4. On failure, surface the error message

**For Skills:**
1. Validate the Git repository is reachable (shallow `git ls-remote`)
2. Record the latest commit SHA
3. If `secretRef` is specified, verify the Secret exists
4. Update `status.skills[].phase` to Ready or Unavailable

**For Plugins:**
1. Validate the npm package exists (`npm view {package}`)
2. Resolve the actual version (handle semver ranges)
3. Update `status.plugins[].phase` to Ready or Unavailable

The controller re-checks periodically (configurable interval, default 10 minutes) to keep status fresh.

### Relationship to AgentTemplate and Agent

The Registry is a **catalog** — it does not directly configure Agents. The flow is:

```
Registry (catalog)                    AgentTemplate / Agent (runtime config)
┌─────────────────────┐              ┌──────────────────────────┐
│ images:             │              │ spec:                    │
│   - go-dev (Ready)  │──reference──→│   executorImage: zot/... │
│   - node-dev        │              │   skills:                │
│                     │              │     - name: my-skills    │
│ skills:             │──reference──→│       git: {same ref}    │
│   - golang (Ready)  │              │   plugins:               │
│   - code-review     │              │     - name: "slack@0.8"  │
│                     │──reference──→│                          │
│ plugins:            │              └──────────────────────────┘
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

### In-Cluster Image Building: Kaniko (Default) + BuildKit (Optional)

Image building is the only part that requires infrastructure (Zot + build engine). When `registry.enabled: true`, the Helm chart deploys:

- **Zot** — In-cluster OCI registry (Deployment + PVC) for storing built images
- **Kaniko** — Default build engine, creates K8s Jobs per build (PSS Restricted compatible)
- **BuildKit** — Optional alternative for clusters with relaxed Pod Security Standards

#### Why Kaniko as Default

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

| Aspect | Kaniko (default) | BuildKit (optional) |
|--------|-----------------|---------------------|
| **PSS Restricted** | Yes | No — requires `allowPrivilegeEscalation: true` |
| **Deployment** | Stateless K8s Jobs | Persistent Deployment (daemon) |
| **Multi-arch** | Not supported | Supported |
| **Maintenance** | Chainguard/osscontainertools forks | Official moby/buildkit |

#### Build Flow

```
User uploads Dockerfile (via UI or Registry spec)
       ↓
Registry Controller creates Kaniko Job
       ↓
Kaniko builds image → pushes to Zot
       ↓
Controller updates Registry status with image digest
       ↓
Image available for selection in Visual Assembler
```

Built images are stored in Zot with namespace-scoped paths:

```
zot.kubeopencode-system.svc:5000/{namespace}/{image-name}:latest
zot.kubeopencode-system.svc:5000/{namespace}/{image-name}@sha256:...
```

### Helm Integration

The Registry components are part of the unified KubeOpenCode Helm chart, gated behind `registry.enabled`:

```yaml
# charts/kubeopencode/values.yaml (additions)
registry:
  enabled: false          # Set to true to deploy Registry components
  zot:
    enabled: true
    image: ghcr.io/project-zot/zot-linux-amd64:v2.1.8
    storageSize: 50Gi
  build:
    engine: kaniko
    kaniko:
      image: registry.gitlab.com/gitlab-ci-utils/container-images/kaniko:debug
    buildkit:
      enabled: false
      image: moby/buildkit:v0.29.0-rootless
      cacheSize: 10Gi
```

When `registry.enabled: false` (default), no Registry-related resources are rendered. KubeOpenCode works exactly as today.

### Architecture

```
KubeOpenCode Helm Chart (kubeopencode-system namespace)
┌────────────────────────────────────────────────────────┐
│  Core components (always deployed):                     │
│  ┌──────────────────────────────────────────────┐      │
│  │  Controller (Deployment)                      │      │
│  │  • Agent, Task, CronTask reconcilers          │      │
│  │  • NEW: Registry reconciler                   │      │
│  └──────────────────────────────────────────────┘      │
│  ┌──────────────────────────────────────────────┐      │
│  │  Server + UI (Deployment, port 2746)          │      │
│  │  • Agents, Tasks, CronTasks pages             │      │
│  │  • NEW: Registry pages (CRUD + Assembler)     │      │
│  └──────────────────────────────────────────────┘      │
│                                                        │
│  Optional: Build infra (registry.enabled=true)         │
│  ┌──────────────┐  ┌──────────────┐                    │
│  │ Zot          │  │ BuildKit     │                    │
│  │ (Deployment  │  │ daemon       │                    │
│  │  + PVC)      │  │ (optional)   │                    │
│  │ built images │  │              │                    │
│  └──────────────┘  └──────────────┘                    │
└────────────────────────────────────────────────────────┘

Registry Controller actions:
  Images:  create Kaniko Job ──build──→ push to Zot
  Skills:  git ls-remote ──check──→ update status
  Plugins: npm view ──check──→ update status
```

**Note**: Verdaccio (in-cluster npm registry) is removed from this design. Plugins use the standard npm registry (or a corporate npm proxy configured externally). Air-gapped plugin support can use a pre-populated `node_modules` volume or an externally managed npm mirror — this is an infrastructure concern, not a KubeOpenCode concern.

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

type RegistrySpec struct {
    // Images defines container images to build and store.
    Images []RegistryImage `json:"images,omitempty"`
    // Skills defines skill references (reuses existing SkillSource types).
    Skills []RegistrySkill `json:"skills,omitempty"`
    // Plugins defines plugin references (reuses existing PluginSpec type).
    Plugins []RegistryPlugin `json:"plugins,omitempty"`
}

// RegistryImage defines a container image to build from a Dockerfile.
type RegistryImage struct {
    // Name is a unique identifier for this image within the Registry.
    Name string `json:"name"`
    // Dockerfile specifies how to build the image.
    Dockerfile DockerfileBuild `json:"dockerfile"`
    // Metadata provides human-readable information for the UI.
    Metadata AssetMetadata `json:"metadata,omitempty"`
}

type DockerfileBuild struct {
    // Context is a Git repository URL containing the Dockerfile.
    // Mutually exclusive with Inline.
    Context string `json:"context,omitempty"`
    // Path is the Dockerfile path within the context. Defaults to "Dockerfile".
    Path string `json:"path,omitempty"`
    // Ref is the Git ref to checkout. Defaults to "HEAD".
    Ref string `json:"ref,omitempty"`
    // Inline is an inline Dockerfile content. Mutually exclusive with Context.
    Inline string `json:"inline,omitempty"`
    // SecretRef references a Secret for Git credentials.
    SecretRef *GitSecretReference `json:"secretRef,omitempty"`
}

// RegistrySkill wraps SkillSource with catalog metadata.
// The Git field reuses the existing GitSkillSource type from SkillSource.
type RegistrySkill struct {
    // Name is a unique identifier for this skill within the Registry.
    Name string `json:"name"`
    // Git specifies the skill source (reuses existing GitSkillSource).
    Git *GitSkillSource `json:"git,omitempty"`
    // Metadata provides human-readable information for the UI.
    Metadata AssetMetadata `json:"metadata,omitempty"`
}

// RegistryPlugin wraps PluginSpec with catalog metadata.
type RegistryPlugin struct {
    // Name is a unique identifier for this plugin within the Registry.
    Name string `json:"name"`
    // Plugin specifies the plugin (reuses existing PluginSpec).
    Plugin PluginSpec `json:"plugin"`
    // Metadata provides human-readable information for the UI.
    Metadata AssetMetadata `json:"metadata,omitempty"`
}

// AssetMetadata provides human-readable information for UI display.
type AssetMetadata struct {
    Description string   `json:"description,omitempty"`
    Tags        []string `json:"tags,omitempty"`
    // Tools lists tools available in an image (only for images).
    Tools []string `json:"tools,omitempty"`
    // RequiredCredentials lists env vars that must be set (only for plugins).
    RequiredCredentials []CredentialRequirement `json:"requiredCredentials,omitempty"`
}

type CredentialRequirement struct {
    Env         string `json:"env"`
    Description string `json:"description,omitempty"`
}

// RegistryStatus tracks the status of all assets.
type RegistryStatus struct {
    Images  []ImageStatus  `json:"images,omitempty"`
    Skills  []SkillStatus  `json:"skills,omitempty"`
    Plugins []PluginStatus `json:"plugins,omitempty"`
    Summary StatusSummary  `json:"summary,omitempty"`
}

type ImageStatus struct {
    Name         string      `json:"name"`
    Phase        ImagePhase  `json:"phase"`             // Pending | Building | Ready | Failed
    Image        string      `json:"image,omitempty"`   // Full image reference with digest
    Digest       string      `json:"digest,omitempty"`
    BuildJobName string      `json:"buildJobName,omitempty"`
    BuildTime    *metav1.Time `json:"buildTime,omitempty"`
    Message      string      `json:"message,omitempty"`
}

type SkillStatus struct {
    Name         string      `json:"name"`
    Phase        AssetPhase  `json:"phase"`            // Ready | Unavailable
    LatestCommit string      `json:"latestCommit,omitempty"`
    LastChecked  *metav1.Time `json:"lastChecked,omitempty"`
    Message      string      `json:"message,omitempty"`
}

type PluginStatus struct {
    Name            string      `json:"name"`
    Phase           AssetPhase  `json:"phase"`          // Ready | Unavailable
    ResolvedVersion string      `json:"resolvedVersion,omitempty"`
    LastChecked     *metav1.Time `json:"lastChecked,omitempty"`
    Message         string      `json:"message,omitempty"`
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

**Decision**: The Registry CRD is a catalog/index that references existing storage (Git for skills, npm for plugins, Zot for images). It does not introduce new storage formats.

**Rationale**:
- Skills are SKILL.md files — Git is the natural home. No benefit from repackaging as OCI artifacts.
- Plugins are npm packages — the npm ecosystem already handles versioning, distribution, and caching.
- Only images need in-cluster storage (Zot) because they need to be **built** from Dockerfiles.
- Reusing existing types (`GitSkillSource`, `PluginSpec`) means zero migration — all existing Agent/AgentTemplate configurations work unchanged.

#### 2. AgentTemplate as Assembly Output

**Decision**: The Visual Assembler generates AgentTemplate YAML. No "recipe" or intermediate abstraction.

**Rationale**:
- AgentTemplate already exists with established merge semantics
- Generated YAML is self-contained and auditable
- Teams can share via `kubectl`, Git, or GitOps workflows

#### 3. Namespace-Scoped Registry CRD

**Decision**: Registry is namespace-scoped. Each team/namespace can have its own Registry.

**Rationale**:
- Teams manage their own asset catalogs independently
- RBAC naturally scoped — standard Kubernetes namespace permissions apply
- Multiple Registries per namespace are allowed (e.g., `team-alpha-dev`, `team-alpha-prod`)
- Built images are stored with namespace-scoped paths in Zot (`zot.svc:5000/{namespace}/{name}`)

#### 4. Status-Driven UI

**Decision**: The Registry controller actively checks asset availability and surfaces status. The UI shows ready/unavailable indicators.

**Rationale**:
- Users see immediately which assets are usable vs. broken
- Build failures are surfaced before they affect Agent deployment
- Git credential issues are caught at catalog time, not at Task runtime
- Periodic re-checks keep status fresh (stale Git refs, yanked npm packages)

#### 5. Kaniko as Default Build Engine

**Decision**: Kaniko is the default. BuildKit is opt-in. See the [In-Cluster Image Building](#in-cluster-image-building-kaniko-default--buildkit-optional) section for detailed analysis.

### Supersedes ADR 0015

ADR 0015 proposed embedding builds in the Agent controller via `Agent.spec.build`. This ADR takes a different approach: builds are managed through the **Registry CRD**, decoupled from Agent lifecycle.

| Aspect | ADR 0015 | ADR 0036 (this) |
|--------|----------|----------------|
| Build trigger | `Agent.spec.build` | `Registry.spec.images[].dockerfile` |
| Build engine | BuildKit only | Kaniko (default) + BuildKit (optional) |
| Image lifecycle | Tied to Agent | Independent — build once, reference in many Agents |
| Skill/Plugin management | Not addressed | Registry catalog with status checks |
| User experience | YAML-only | UI with CRUD + visual assembler |
| Controller state | Stateful (build tracking in Agent) | Stateless (build state in Job + Registry status) |

## Implementation Plan

### Phase 1: Registry CRD + Controller + CLI

**Goal**: Registry CRD with status checks for skills and plugins. No image building yet.

1. **API**: Add `Registry` CRD to `api/v1alpha1/`
2. **Controller**: `registry_controller.go` — reconcile skill/plugin status checks
3. **CLI**: `kubeoc registry list`, `kubeoc registry show <name>`
4. **Tests**: Unit + integration tests
5. **Documentation**: Architecture docs, YAML examples

### Phase 2: Image Building + Zot

**Goal**: In-cluster image building from Dockerfiles in Registry spec.

1. **Build orchestration**: Controller creates Kaniko Jobs for Registry images
2. **Zot deployment**: Helm templates under `charts/kubeopencode/templates/registry/`
3. **Build log streaming**: Server API for real-time build logs
4. **CLI**: `kubeoc registry build <name>`, `kubeoc registry logs <name>`

### Phase 3: UI + Visual Assembler

**Goal**: Web UI for asset management and agent assembly.

1. **Registry pages**: CRUD for images, skills, plugins with status indicators
2. **Build UI**: Upload Dockerfile → see build logs → success/fail
3. **Assembler**: Pick components → generate AgentTemplate YAML → copy button
4. **Server API**: REST endpoints for Registry CRUD + assembly

### Phase 4: Enterprise Features

1. **RBAC** — Who can create/edit Registries, who can build images
2. **Supply chain security** — Cosign signing for built images
3. **Audit trail** — Track asset changes
4. **Scheduled rebuilds** — Cron-based rebuild for security patches
5. **Multi-cluster** — Registry replication across clusters

## Consequences

### Positive

1. **Dramatically simpler design** — No OCI artifacts for skills, no Verdaccio, no custom packaging formats. Registry is a thin catalog layer over existing types.
2. **Zero migration** — Existing `SkillSource` and `PluginSpec` types are reused as-is. No changes to Agent or AgentTemplate CRDs.
3. **Status visibility** — Controller actively checks asset health. Users know immediately if a Git repo is unreachable or an npm package is missing.
4. **Namespace-scoped governance** — Teams manage their own catalogs. Standard Kubernetes RBAC applies.
5. **Visual assembly** — UI lowers the barrier to creating well-configured Agents.
6. **Single Helm chart** — Enable with `--set registry.enabled=true`. No separate install.
7. **PSS Restricted compatible** — Kaniko default build engine works in strict security environments.

### Negative

1. **No unified storage** — Skills stay in Git, plugins in npm, images in Zot. Three different systems to manage.
2. **Status checks are best-effort** — `git ls-remote` and `npm view` can fail for transient reasons. Status may flicker.
3. **No air-gap story for plugins** — Without Verdaccio, air-gapped clusters need an externally managed npm mirror. This is explicitly out of scope.
4. **New CRD** — Adds a new resource type that teams need to learn.

### Risks

| Risk | Mitigation |
|------|------------|
| UI development takes too long | Phase 1-2 deliver value via CLI and CRD before UI is ready |
| Kaniko fork maintenance | Track Chainguard and osscontainertools forks |
| Git credential issues in status checks | Use shallow `ls-remote` (no clone); clear error messages in status |
| Registry status becomes stale | Configurable check interval; manual trigger via annotation |

## Alternatives Considered

### Alternative 1: OCI Artifacts for Skills

Store skills as OCI artifacts in Zot. Add `skill-init` container to pull and unpack them.

**Rejected because:**
- Skills are SKILL.md files — Git is the natural, simpler storage
- OCI artifact packaging adds complexity (custom media types, crane dependency, new init container)
- Users already know Git; OCI artifacts are an unfamiliar concept for most
- No benefit over Git for text files that change infrequently

### Alternative 2: Verdaccio for In-Cluster npm

Deploy Verdaccio as an in-cluster npm registry for air-gapped plugin support.

**Deferred because:**
- Adds operational complexity (another Deployment + PVC to manage)
- Air-gapped npm is an infrastructure concern, not a KubeOpenCode concern
- Corporate npm proxies (Nexus, Artifactory, Verdaccio managed externally) already serve this purpose
- Can be added later if demand emerges

### Alternative 3: Recipe Abstraction Layer

Add a `Recipe` CRD that pre-combines image + skills + plugins.

**Rejected because:**
- AgentTemplate already serves this purpose
- Recipes introduce merge ambiguity (what if two recipes conflict?)
- Users want standard Kubernetes resources, not custom abstractions

### Alternative 4: Build Images in Agent Controller (ADR 0015)

Embed builds in the Agent controller via `Agent.spec.build`.

**Superseded because:**
- Couples image lifecycle to Agent lifecycle
- Controller becomes stateful (build tracking)
- BuildKit-only — no PSS Restricted support

### Alternative 5: Separate Helm Chart for Registry

Deploy Registry as an independent Helm chart.

**Rejected because:**
- Two Helm releases to manage
- Cross-namespace complexity
- Version compatibility concerns
- Community convention favors single charts with feature flags

### Alternative 6: Shipwright + Tekton for Builds

Use Shipwright CRDs for build lifecycle management.

**Rejected because:**
- Over-engineering for our simple requirement (Dockerfile → build → push)
- Two additional operator dependencies
- Fewer moving parts = easier to maintain

## References

- [Kaniko](https://github.com/GoogleContainerTools/kaniko) — Userspace container image builder
- [Chainguard Kaniko Fork](https://github.com/chainguard-forks/kaniko) — Actively maintained fork
- [BuildKit](https://github.com/moby/buildkit) — Container image builder (requires privilege escalation)
- [Zot Registry](https://zotregistry.dev/) — OCI-native container registry (CNCF Sandbox)
- [ADR 0015](0015-repo-as-agent-dynamic-image-building.md) — Dynamic image building (superseded)
- [ADR 0026](0026-skills.md) — Skills as a top-level Agent field
- [ADR 0034](0034-plugin-support-and-slack-integration.md) — Plugin support
- [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) — PSS Restricted profile
