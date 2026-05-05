# ADR 0036: Agent Registry — Enterprise Agent Asset Management and Visual Agent Assembly

## Status

Proposed (supersedes [ADR 0015](0015-repo-as-agent-dynamic-image-building.md))

## Date

2026-05-05

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
  contexts:
    - name: coding-standards
      text:
        content: "???"                                # What's best practice?
  credentials: [...]                                   # How to configure?
```

Users must discover each component independently, understand how they fit together, and manually compose the YAML. There is no central catalog, no discovery mechanism, and no way to visually assemble an agent.

### The Enterprise Security Problem

In real enterprise environments, teams cannot freely pull skills from public GitHub repos or plugins from the public npm registry. There are serious concerns:

- **Supply chain security** — Public skills could contain malicious instructions; public plugins could exfiltrate data
- **Compliance** — Regulated industries require all assets to come from approved, audited sources
- **Governance** — Organizations need visibility into what agent capabilities are deployed across teams
- **Air-gapped environments** — Many enterprise clusters have no internet access at all

Teams need a **trusted, internal registry** where all agent assets (images, skills, plugins) are stored, reviewed, and managed by the organization.

### Inspiration: Rivet agentOS Registry

Rivet's agentOS project (https://rivet.dev/agent-os/) provides a package registry where users browse and compose agent components. We want to bring a similar "building blocks" experience to KubeOpenCode — but adapted for the enterprise Kubernetes context, with a visual UI as the assembler rather than code-level imports.

### Related ADRs

- **ADR 0015** — Repo as Agent: dynamic image building (superseded by this ADR)
- **ADR 0026** — Skills as a top-level Agent field (skills are Git-only, no discovery)
- **ADR 0034** — Plugin support (plugins are npm-only, no curated catalog)

## Decision

### Agent Registry as an Integrated Feature of KubeOpenCode

The **Agent Registry** is a built-in feature of KubeOpenCode — not a separate project. It is an **enterprise agent asset management and visual agent assembly** capability that provides:

1. **Centralized storage** — All agent assets (images, skills, plugins) in one managed location
2. **In-cluster image building** — Upload a Dockerfile, get a ready-to-use image (BuildKit + Zot as optional Helm sub-charts)
3. **Visual agent assembly** — A "Registry" tab in the existing KubeOpenCode UI where users pick components like Lego blocks and get a ready-to-apply Agent YAML
4. **Governance** — Visibility into what assets exist and who manages them

Since the infrastructure is just two lightweight Deployments (BuildKit daemon + Zot registry) — no external operators, no additional CRDs from third parties — it fits naturally into the existing KubeOpenCode Helm chart as optional components.

### Everything is a File

A unifying design principle: **all registry components are files**.

| Component | Files | What happens on upload |
|-----------|-------|----------------------|
| Skill | `SKILL.md` + `metadata.yaml` | Stored as-is. Available for selection in the UI. |
| Plugin | `metadata.yaml` + `README.md` | Stored as-is. Available for selection in the UI. |
| Image | `Dockerfile` + `metadata.yaml` | **Auto-built** via BuildKit → stored in Zot. User knows immediately if it works. |

Images are the special case: a Dockerfile is just a file, but when uploaded to the Registry, the system **immediately builds it** and stores the resulting container image in the cluster-internal Zot registry. The user gets instant feedback — build succeeded or failed — and the image becomes available for agent assembly.

### Three Component Types

#### 1. Images (Agent Runtimes)

Dockerfiles that define the execution environment for agents. The Registry includes built-in images and supports user-uploaded custom images.

**Built-in images** (shipped with the Registry):

| Image | Contents | Size |
|-------|----------|------|
| `minimal` | Shell + git + curl | ~200MB |
| `go` | Go + gopls + delve | ~800MB |
| `node` | Node.js + npm + tsc | ~600MB |
| `python` | Python + pip + venv | ~500MB |
| `k8s` | kubectl + helm + kustomize | ~400MB |
| `devbox` | Full stack (Go + Node + Python + tools) | ~2-3GB |

**Custom images**: Users upload a Dockerfile (or provide a Git repo URL + Dockerfile path). The Registry builds it immediately using BuildKit and pushes to the in-cluster Zot registry. The user sees the build log in real-time and knows instantly if the image is usable.

Image `metadata.yaml`:

```yaml
name: go
description: "Go development environment with gopls and delve"
tags: [golang, backend, development]
version: "1.0.0"
tools: [go, gopls, delve, golangci-lint]
```

#### 2. Skills (Agent Capabilities)

SKILL.md files that define what an agent can do. These are enterprise-internal assets — teams author skills specific to their organization's workflows, coding standards, and operational procedures.

Examples:
- Internal deployment procedures
- Company-specific code review guidelines
- Team coding standards and conventions
- Incident response runbooks
- Compliance and security audit procedures

Skill `metadata.yaml`:

```yaml
name: kubernetes-ops
description: "Kubernetes cluster operations, debugging, and resource management"
tags: [kubernetes, ops, sre]
version: "1.0.0"
```

Users can upload, edit, and version skills directly through the Registry UI. Skills are stored in a Git repository (internal or managed by the Registry).

#### 3. Plugins (Agent Extensions)

OpenCode plugin metadata and documentation. Plugins extend agent behavior with integrations (Slack, Jira, GitHub, etc.), safety guardrails, and observability.

In enterprise environments, plugins are typically mirrored to an internal npm registry. The Registry stores metadata about approved plugins:

Plugin `metadata.yaml`:

```yaml
name: slack-integration
description: "Slack bot integration — interact with agents via Slack messages"
tags: [slack, messaging, collaboration]
npmPackage: "@kubeopencode/opencode-slack-plugin"
target: server
requiredCredentials:
  - env: SLACK_BOT_TOKEN
    description: "Slack bot OAuth token"
  - env: SLACK_APP_TOKEN
    description: "Slack app-level token for Socket Mode"
```

### The Visual Agent Assembler (Core UX)

The heart of the Registry is its **UI-based agent assembly experience**. There is no intermediate "recipe" abstraction — the UI itself IS the assembler. The output is a standard KubeOpenCode Agent YAML.

#### Assembly Flow

```
┌─────────────────────────────────────────────────────────┐
│                   Registry UI                            │
│                                                          │
│  Step 1: Choose Image                                    │
│  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐          │
│  │  go  │ │ node │ │  py  │ │ k8s  │ │custom│          │
│  │  ✓   │ │      │ │      │ │      │ │      │          │
│  └──────┘ └──────┘ └──────┘ └──────┘ └──────┘          │
│                                                          │
│  Step 2: Choose Skills                                   │
│  ☑ golang-best-practices                                 │
│  ☑ code-review                                           │
│  ☐ security-review                                       │
│  ☐ documentation                                         │
│                                                          │
│  Step 3: Choose Plugins                                  │
│  ☑ slack-integration                                     │
│  ☐ otel-observability                                    │
│  ☐ github-integration                                    │
│                                                          │
│  Step 4: Review & Copy                                   │
│  ┌─────────────────────────────────────────────┐        │
│  │ apiVersion: kubeopencode.io/v1alpha1        │        │
│  │ kind: Agent                                  │        │
│  │ metadata:                                    │        │
│  │   name: my-go-agent                          │        │
│  │ spec:                                        │        │
│  │   executorImage: zot.svc:5000/go@sha256:...  │        │
│  │   skills:                                    │        │
│  │     - name: my-skills                        │        │
│  │       registry:                              │        │
│  │         url: http://registry.svc:8080        │        │
│  │         names: [golang, code-review]         │        │
│  │   plugins:                                   │        │
│  │     - name: slack-integration                │        │
│  │       registry:                              │        │
│  │         url: http://registry.svc:8080        │        │
│  │   ...                                        │        │
│  └─────────────────────────────────────────────┘        │
│                                    [ Copy YAML ]         │
│                                                          │
└──────────────────────────────────────────────────────────┘

                        ↓ User copies YAML

$ kubectl apply -f my-go-agent.yaml    # Agent runs on KubeOpenCode
```

**Key principle**: The generated YAML is a **standard Agent YAML**. KubeOpenCode does not know or care that it was generated by the Registry. The Agent CRD has no `recipe` field, no `registry` reference — it simply references images by digest, skills by Git URL, and plugins by npm package name.

### Registry Source Type in KubeOpenCode

KubeOpenCode is not entirely unaware of the Registry. The Agent spec gains a new **`registry` source type** — alongside the existing `git` source — for skills and plugins. This is a lightweight integration point: KubeOpenCode knows how to fetch assets from a Registry API endpoint, but has no dependency on the Registry being deployed.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  # Image: directly reference Zot digest (no special source type needed)
  executorImage: zot.kubeopencode-system.svc:5000/agents/go@sha256:abc123...

  skills:
    # Existing: Git source (unchanged)
    - name: external-skills
      git:
        repository: https://github.com/company/skills.git
        names: [golang, code-review]

    # NEW: Registry source
    - name: internal-skills
      registry:
        url: http://registry.kubeopencode-system.svc:8080
        names: [kubernetes-ops, security-review, incident-response]

  plugins:
    # Existing: npm package name (unchanged)
    - name: "@kubeopencode/opencode-slack-plugin"

    # NEW: Registry source (resolves to npm package + options)
    - name: slack-integration
      registry:
        url: http://registry.kubeopencode-system.svc:8080
```

**How it works:**

| Source | KubeOpenCode Controller Action |
|--------|-------------------------------|
| `skills[].git` | Clone Git repo → mount SKILL.md files (existing behavior) |
| `skills[].registry` | `GET {url}/api/v1/skills/{name}` → download SKILL.md → mount as file |
| `plugins[].registry` | `GET {url}/api/v1/plugins/{name}` → resolve npm package + options → install via plugin-init |
| `executorImage` | Standard image pull (unchanged — Zot URL is just a container registry address) |

**Benefits of the `registry` source type:**
- Skills: no Git clone overhead; Registry serves SKILL.md content directly via HTTP
- Plugins: Registry resolves the npm package name and validated options; KubeOpenCode doesn't need to know the npm details
- Agent YAML is self-contained — the Registry URL is in the spec, so the Agent is reproducible
- Graceful degradation — if Registry is down, the controller reports a clear error condition

**API changes in KubeOpenCode (minimal):**

```go
// SkillSource gains a Registry option alongside Git
type SkillSource struct {
    Name     string           `json:"name"`
    Git      *GitSkillSource  `json:"git,omitempty"`
    Registry *RegistrySource  `json:"registry,omitempty"`  // NEW
}

// PluginSpec gains an optional Registry source
type PluginSpec struct {
    Name     string                `json:"name"`
    Target   PluginTarget          `json:"target,omitempty"`
    Options  *runtime.RawExtension `json:"options,omitempty"`
    Registry *RegistrySource       `json:"registry,omitempty"`  // NEW
}

// RegistrySource points to a Registry API endpoint
type RegistrySource struct {
    // URL is the Registry server endpoint.
    // Example: "http://registry.kubeopencode-system.svc:8080"
    URL   string   `json:"url"`
    // Names selects specific items from the registry.
    // +optional
    Names []string `json:"names,omitempty"`
}
```

### Architecture

Everything lives in the existing KubeOpenCode Helm chart. BuildKit and Zot are optional sub-charts enabled by Helm values.

```
KubeOpenCode Helm Chart (kubeopencode-system namespace)
┌────────────────────────────────────────────────────────┐
│                                                         │
│  Existing components (unchanged):                       │
│  ┌──────────────────────────────────────────────┐      │
│  │  Controller (Deployment)                      │      │
│  │  • Agent, Task, CronTask reconcilers          │      │
│  │  • NEW: registry source resolver              │      │
│  │    (fetches skills/plugins from Registry API)  │      │
│  └──────────────────────────────────────────────┘      │
│  ┌──────────────────────────────────────────────┐      │
│  │  Server + UI (Deployment, port 2746)          │      │
│  │  • Existing: Agents, Tasks, CronTasks pages   │      │
│  │  • NEW: Registry tab                          │      │
│  │    - Browse / upload / manage assets           │      │
│  │    - Upload Dockerfile → trigger build         │      │
│  │    - Visual agent assembler (pick → copy YAML) │      │
│  │  • NEW: Registry REST API                     │      │
│  │    - GET /api/v1/registry/skills/{name}       │      │
│  │    - GET /api/v1/registry/plugins/{name}      │      │
│  │    - POST /api/v1/registry/images (build)     │      │
│  │  • NEW: BuildKit gRPC client (~200 lines)     │      │
│  └──────────────────────────────────────────────┘      │
│                                                         │
│  Optional components (registry.enabled: true):          │
│  ┌──────────────┐  ┌──────────────┐                    │
│  │ BuildKit     │  │ Zot          │                    │
│  │ daemon       │  │ registry     │                    │
│  │ (Deployment) │  │ (Deployment  │                    │
│  │ rootless     │  │  + PVC)      │                    │
│  │ + cache PVC  │  │              │                    │
│  └──────────────┘  └──────────────┘                    │
│                                                         │
└────────────────────────────────────────────────────────┘

Helm values:
  registry:
    enabled: true       # Enables Registry tab in UI + REST API
  build:
    enabled: true       # Deploys BuildKit daemon
  imageStore:
    enabled: true       # Deploys Zot registry
    storageSize: 50Gi   # PVC size for image storage
```

### In-Cluster Image Building: BuildKit + Zot

When a user uploads or adds a Dockerfile through the Registry UI:

1. **Registry server** receives the Dockerfile (or Git repo URL + Dockerfile path)
2. **Registry server** calls `BuildKit` daemon via Go gRPC SDK (`client.Solve()`)
3. **BuildKit** builds the image (rootless, secure) and pushes to the in-cluster **Zot** registry
4. **Registry server** streams build logs to the UI in real-time via the `statusCh` channel
5. On success, the image is listed as available with its digest
6. User can select this image in the visual assembler

#### Why BuildKit Direct (Not Shipwright)

The Registry's build needs are simple: take a Dockerfile, build it, push to Zot. The BuildKit Go SDK makes this a single function call:

```go
// Registry server's build function — ~200 lines total
buildkitClient, _ := client.New(ctx, "tcp://buildkitd.kubeopencode-system.svc:1234")

statusCh := make(chan *client.SolveStatus)
go streamToUI(statusCh)  // Real-time logs to Registry UI via WebSocket

resp, err := buildkitClient.Solve(ctx, nil, client.SolveOpt{
    Frontend: "dockerfile.v0",
    FrontendAttrs: map[string]string{
        "filename": "Dockerfile",
    },
    LocalDirs: map[string]string{
        "dockerfile": dockerfilePath,
        "context":    contextPath,
    },
    Exports: []client.ExportEntry{{
        Type: client.ExporterImage,
        Attrs: map[string]string{
            "name": "zot.kubeopencode-system.svc:5000/agents/my-image",
            "push": "true",
        },
    }},
    CacheExports: []client.CacheOptionsEntry{{
        Type: "inline",  // Cache embedded in image for faster rebuilds
    }},
}, statusCh)
// resp.ExporterResponse["containerimage.digest"] = "sha256:abc123..."
```

Shipwright + Tekton was considered but rejected as over-engineering:

| Aspect | BuildKit direct | Shipwright + Tekton |
|--------|----------------|---------------------|
| Core build call | `client.Solve()` — one function | Build CRD → BuildRun CRD → Tekton TaskRun → Pod → BuildKit |
| Build logs | `statusCh` channel — direct stream | Read from Tekton TaskRun Pod stdout |
| Build status | `Solve()` returns error or result | Poll BuildRun.status |
| Dependencies | 1 BuildKit Deployment | Tekton operator + Shipwright operator + CRDs |
| Implementation | ~200-300 lines Go | ~0 lines (but 2 operators to deploy and maintain) |
| Features we need | Build Dockerfile, push to registry | Build Dockerfile, push to registry |
| Features we DON'T need | — | Multi-strategy, Pipeline integration, Git triggers, multi-arch native |

The Registry's build requirements are simple and well-bounded. BuildKit's Go SDK gives us everything we need without the operational overhead of two additional operators.

#### BuildKit Daemon Deployment

BuildKit runs as a single Deployment in rootless mode within the Registry namespace:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: buildkitd
  namespace: kubeopencode-system
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: buildkitd
          image: moby/buildkit:v0.29.0-rootless
          args: ["--addr", "tcp://0.0.0.0:1234"]
          securityContext:
            runAsUser: 1000
            runAsGroup: 1000
            allowPrivilegeEscalation: true  # Required by rootlesskit
          ports:
            - containerPort: 1234
          volumeMounts:
            - name: buildkit-cache
              mountPath: /home/user/.local/share/buildkit
      volumes:
        - name: buildkit-cache
          persistentVolumeClaim:
            claimName: buildkit-cache  # Layer cache persists across restarts
```

Key characteristics:
- **Rootless mode** — No privileged containers (only `allowPrivilegeEscalation` for rootlesskit)
- **Persistent cache** — PVC preserves layer cache across Pod restarts for fast rebuilds
- **In-cluster only** — ClusterIP Service, Registry server connects via gRPC
- **Single replica** — Sufficient for agent image builds (not a CI system)

#### Zot In-Cluster Registry

Zot (CNCF Sandbox) runs as a Deployment + PVC within the Registry namespace:

- **PVC-based storage** — No S3 dependency. Works in any cluster.
- **Deduplication** — Agent images share base layers (debian:bookworm-slim); dedup saves significant storage.
- **Garbage collection** — Auto-cleanup of old image digests (configurable).
- **In-cluster only** — ClusterIP Service, no Ingress. Images never leave the cluster.
- **Single binary** — Minimal footprint, OCI-native.

KubeOpenCode references built images by their full in-cluster URL: `zot.kubeopencode-system.svc:5000/agents/go@sha256:...`

#### Total Additional Infrastructure: Just 2 Optional Deployments

```
kubeopencode-system namespace (existing)
├── Controller         (existing Deployment, unchanged)
├── Server + UI        (existing Deployment, adds Registry tab + BuildKit client)
├── BuildKit daemon    (NEW optional Deployment — build.enabled: true)
└── Zot registry       (NEW optional Deployment + PVC — imageStore.enabled: true)
```

No operators. No CRDs from third parties. No Tekton. The existing KubeOpenCode server directly orchestrates builds via BuildKit gRPC and manages images via Zot's OCI API.

### Supersedes ADR 0015

This design **supersedes ADR 0015** (Repo as Agent — Dynamic Image Building). ADR 0015 proposed the same BuildKit + Zot approach but as a controller-level concern with `Agent.spec.build`. This ADR takes a different angle: builds are managed through the **Registry UI and server API**, not through the Agent CRD.

| Aspect | ADR 0015 | ADR 0036 (this) |
|--------|----------|----------------|
| Build trigger | `Agent.spec.build` field (controller watches) | Registry UI / REST API (user-initiated) |
| Build engine | BuildKit gRPC (same) | BuildKit gRPC (same) |
| Image storage | Zot (same) | Zot (same) |
| User experience | YAML-only, build status in Agent.status | UI with real-time build logs + visual assembler |
| Agent CRD changes | New `BuildConfig` struct | None — Agent just references the built image digest |
| Helm deployment | `build.enabled` (same approach) | `registry.enabled` + `build.enabled` |

Key improvement: builds are decoupled from the Agent lifecycle. Users build images independently through the Registry, then reference them in any number of Agents. The Agent controller stays simple — it just pulls images, it never builds them.

### Key Design Decisions

#### 1. No Recipe Abstraction

**Decision**: There is no "recipe" CRD or file format. The Registry UI IS the assembler. The output is a standard Agent YAML.

**Rationale**:
- Adding a recipe layer between the UI and the Agent YAML introduces unnecessary abstraction
- The KubeOpenCode Agent spec already captures everything needed — image, skills, plugins, contexts, credentials
- Users want to see and own the final YAML, not depend on a recipe that resolves at runtime
- The UI provides a better "building blocks" experience than a static recipe file
- Recipes would need their own versioning, compatibility tracking, and maintenance — complexity without proportional value

#### 2. Registry is an Integrated Feature (Not a Separate Project)

**Decision**: The Registry is part of the KubeOpenCode project. BuildKit and Zot are optional Helm sub-charts (`build.enabled`, `imageStore.enabled`). The Registry UI is a tab in the existing KubeOpenCode web UI. The Registry REST API is part of the existing KubeOpenCode server.

**Rationale**:
- BuildKit daemon + Zot are just two Deployments — too lightweight to justify a separate project
- No external operators or third-party CRDs — stays within KubeOpenCode's "zero external dependencies" principle
- Unified UI — users don't need to switch between two web applications
- Single Helm install — `helm install kubeopencode --set registry.enabled=true` enables everything
- Users who don't need the Registry simply don't enable it — zero overhead

#### 3. Registry as a Source Type (Lightweight Integration)

**Decision**: KubeOpenCode Agent CRD gains a `registry` source type for skills and plugins — alongside the existing `git` (skills) and npm (plugins) sources. The controller knows how to fetch assets from a Registry HTTP API, but the Registry itself is optional.

**Rationale**:
- `registry` source is more natural than forcing everything through `git` — no Git clone overhead, direct HTTP fetch
- The Registry API contract is simple (REST endpoints returning SKILL.md content and plugin metadata) — minimal controller complexity
- The Agent spec is self-contained: `registry.url` tells the controller exactly where to fetch from
- Fully backward compatible — `git` and npm sources still work; `registry` is additive
- Users without a Registry simply don't use the `registry` source type — zero impact

**What KubeOpenCode knows:**
- How to call `GET {registry-url}/api/v1/skills/{name}` and mount the returned SKILL.md
- How to call `GET {registry-url}/api/v1/plugins/{name}` and resolve the npm package info

**What KubeOpenCode does NOT know:**
- How the Registry stores assets internally (Git, database, filesystem — doesn't matter)
- How images are built (BuildKit details — the server handles it internally)
- Registry UI specifics or build infrastructure

#### 4. Immediate Build Feedback

**Decision**: When a user uploads a Dockerfile, the build starts immediately and the user sees logs in real-time. The image is either usable or the build error is shown.

**Rationale**:
- "Upload Dockerfile → see if it works → use it" is the core UX promise
- Deferred or background builds break the interactive flow
- Build failures surfaced immediately prevent wasted time debugging Agent startup issues later

#### 5. Optional by Default

**Decision**: All registry components are disabled by default. Users opt in via Helm values.

**Rationale**:
- Users who just want KubeOpenCode for running Tasks/Agents should not be affected
- BuildKit + Zot add ~200MB image pulls and PVC requirements — only pay this cost if needed
- Progressive adoption: start with manual YAML → later enable Registry for UI-based management

## Implementation Plan

### Phase 1: Built-in Content + CLI

**Goal**: Ship built-in skills and image Dockerfiles in the repo. CLI for browsing. Images built by GitHub Actions and pushed to GHCR.

1. **Directory structure** (in KubeOpenCode repo)
   ```
   registry/
   ├── images/           # Built-in Dockerfiles + metadata
   │   ├── minimal/
   │   ├── go/
   │   ├── node/
   │   ├── python/
   │   ├── k8s/
   │   └── devbox/
   ├── skills/           # Built-in SKILL.md files + metadata
   │   ├── kubernetes-ops/
   │   ├── code-review/
   │   ├── security-review/
   │   ├── golang/
   │   └── documentation/
   └── plugins/          # Built-in plugin metadata + README
       ├── slack-integration/
       ├── otel-observability/
       └── github-integration/
   ```

2. **CLI: `kubeoc registry`**
   - `kubeoc registry list [--type images|skills|plugins]` — Browse components
   - `kubeoc registry show <name>` — Show component details

3. **CI**: Build registry Dockerfiles on merge → push to GHCR

### Phase 2: In-Cluster Build + Registry UI + `registry` Source Type

**Goal**: Full integrated experience — UI-based asset management, in-cluster image building, visual agent assembly.

1. **API: `registry` source type** (KubeOpenCode CRD change)
   - Add `RegistrySource` to `SkillSource` and `PluginSpec` in `api/v1alpha1/`
   - Implement registry HTTP client in controller
   - Run `make update` → CRD regeneration
   - Unit tests + documentation

2. **Server: Registry REST API** (add to existing `internal/server/`)
   - `GET /api/v1/registry/skills` — List skills
   - `GET /api/v1/registry/skills/{name}` — Get SKILL.md content
   - `GET /api/v1/registry/plugins` — List plugins
   - `GET /api/v1/registry/plugins/{name}` — Get plugin metadata
   - `GET /api/v1/registry/images` — List images (from Zot)
   - `POST /api/v1/registry/images` — Upload Dockerfile → trigger build
   - `GET /api/v1/registry/builds/{id}/logs` — Stream build logs (WebSocket/SSE)

3. **Server: BuildKit client** (add to existing server binary)
   - Go gRPC client connecting to BuildKit daemon (~200-300 lines)
   - `client.Solve()` with `statusCh` for real-time log streaming
   - Push to Zot via image exporter
   - Build result tracking (digest, duration, status)

4. **UI: Registry tab** (add to existing `ui/`)
   - **Assets** page: Browse/search/upload skills, plugins, images
   - **Images** page: Upload Dockerfile → see build logs → success/fail
   - **Assemble** page: Pick image → pick skills → pick plugins → generate YAML → copy button
   - **Builds** page: Build history with logs

5. **Helm chart: optional components** (add to existing `charts/kubeopencode/`)
   ```yaml
   # charts/kubeopencode/values.yaml
   registry:
     enabled: false       # Enable Registry UI + REST API
   build:
     enabled: false       # Deploy BuildKit daemon
     image: moby/buildkit:v0.29.0-rootless
     cacheSize: 10Gi      # PVC for layer cache
   imageStore:
     enabled: false       # Deploy Zot registry
     image: ghcr.io/project-zot/zot-linux-amd64:v2.1.8
     storageSize: 50Gi    # PVC for image storage
   ```

### Phase 3: Enterprise Features

1. **RBAC** — Role-based access: who can upload images, who can edit skills
2. **Audit trail** — Track asset changes with timestamps and user attribution
3. **Scheduled rebuilds** — Cron-based rebuild for base image security patches
4. **Plugin mirroring** — Mirror approved npm plugins to internal storage
5. **Multi-cluster sync** — Zot replication for multi-cluster deployments

## Consequences

### Positive

1. **Enterprise-ready agent governance** — All agent assets in one managed, auditable location
2. **Dramatically lower barrier to entry** — Visual assembler: pick components, copy YAML, apply
3. **Immediate build feedback** — Upload Dockerfile → see build result in seconds/minutes
4. **Supply chain security** — Teams use only approved, internal assets — no public dependencies at runtime
5. **Minimal KubeOpenCode changes** — Only a `registry` source type added to skills/plugins; no architectural changes
6. **Clean separation** — Registry handles infrastructure complexity; KubeOpenCode stays lean
7. **Native integration** — `registry` source type is a first-class citizen alongside `git` and npm; no workarounds needed

### Negative

1. **Scope expansion** — Adding UI pages, REST endpoints, and BuildKit/Zot integration increases KubeOpenCode's codebase and maintenance surface.
2. **Maintenance burden** — Built-in skills, images, and plugin metadata need ongoing updates.
3. **Initial investment** — Building the Registry UI is a significant development effort.
4. **BuildKit security** — Rootless BuildKit still requires `allowPrivilegeEscalation: true`. Clusters with strict Pod Security Standards may not allow this.

### Risks

| Risk | Mitigation |
|------|------------|
| Registry UI development takes too long | Start with CLI-only (Phase 1) to provide value before UI is ready |
| BuildKit rootless blocked by Pod Security policy | Document security requirements; provide alternative: build externally, push to Zot manually |
| Build failures frustrate users | Real-time build log streaming; clear error messages; Dockerfile linting before build |
| Zot PVC runs out of storage | GC policies; storage monitoring alerts; configurable PVC size in Helm values |
| Feature bloat in KubeOpenCode | All registry components are optional (`enabled: false` by default); core functionality unaffected |

## Alternatives Considered

### Alternative 1: Recipe Abstraction Layer

Add a `Recipe` file format that pre-combines image + skills + plugins into a named configuration.

**Rejected because:**
- The Agent spec already captures the full configuration — recipes add an unnecessary indirection
- The UI provides a better "building blocks" experience than static recipe files
- Recipes would need their own versioning and compatibility matrix — complexity without value
- Users want to see and own the final YAML, not depend on intermediate abstractions

### Alternative 2: Build Images in Agent Controller (ADR 0015)

Embed builds in the Agent controller via `Agent.spec.build` — every Agent with a `build` field triggers a BuildKit build.

**Superseded because:**
- Couples image lifecycle to Agent lifecycle — image should exist independently and be referenced by many Agents
- No UI for build management — users only see build status in `Agent.status`
- Registry approach is better: build once via UI, then reference the digest in any number of Agents

### Alternative 3: Helm-Based Distribution

Distribute each agent configuration as a Helm sub-chart.

**Rejected because:**
- Helm charts are opaque — users cannot easily see or modify the generated Agent spec
- Skills and plugins don't map to Helm concepts
- Adds Helm dependency management complexity

### Alternative 4: Separate Registry Project

Build the Registry as an independent project with its own repo, Helm chart, and release cycle.

**Rejected because:**
- Without Shipwright/Tekton, the infrastructure is just two Deployments (BuildKit + Zot) — too lightweight to justify a separate project
- Separate project means separate UI, separate install, separate auth — worse user experience
- Integrated approach gives unified UI, single Helm install, and consistent auth/RBAC

### Alternative 5: Shipwright + Tekton for Build Orchestration

Use Shipwright CRDs for build lifecycle management instead of calling BuildKit directly.

**Rejected because:**
- Our build requirements are simple: Dockerfile → build → push to Zot
- Shipwright adds two operator dependencies (Tekton + Shipwright) for capabilities we don't need (multi-strategy, Pipeline integration, Git triggers)
- BuildKit Go SDK handles the entire build lifecycle in ~200 lines of code
- Fewer moving parts = easier to debug, deploy, and maintain

### Alternative 6: Public Marketplace

Build a public marketplace where anyone can publish and discover agent components.

**Deferred because:**
- Enterprise users need internal, governed registries — not public marketplaces
- Supply chain security concerns make public assets risky for production use
- A public catalog can be built later on top of the same Registry feature

## References

- [Rivet agentOS Registry](https://rivet.dev/agent-os/registry) — Inspiration for the building-block approach
- [BuildKit](https://github.com/moby/buildkit) — Go SDK for container image building
- [Zot Registry](https://zotregistry.dev/) — OCI-native container registry (CNCF Sandbox)
- [Artifact Hub](https://artifacthub.io/) — Precedent for Kubernetes ecosystem package discovery
- [ADR 0015](0015-repo-as-agent-dynamic-image-building.md) — Dynamic image building (superseded)
- [ADR 0026](0026-skills.md) — Skills as a top-level Agent field
- [ADR 0034](0034-plugin-support-and-slack-integration.md) — Plugin support
