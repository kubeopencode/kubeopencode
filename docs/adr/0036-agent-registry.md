# ADR 0036: Agent Registry — Enterprise Agent Asset Management and Visual Agent Assembly

## Status

Proposed (supersedes [ADR 0015](0015-repo-as-agent-dynamic-image-building.md))

## Date

2026-05-06

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

### Agent Registry as an Independent, Optional Component

The **Agent Registry** is an **independent project** within the KubeOpenCode ecosystem — deployed as a separate Helm chart with its own release cycle. It is an **enterprise agent asset management and visual agent assembly** capability that provides:

1. **Centralized storage** — All agent assets (images, skills, plugins) stored in OCI-native and npm-native registries
2. **In-cluster image building** — Upload a Dockerfile, get a ready-to-use image (Kaniko or BuildKit + Zot)
3. **Visual agent assembly** — A Registry UI where users pick components like Lego blocks and generate a ready-to-apply AgentTemplate or Agent YAML
4. **Governance** — Visibility into what assets exist, versioning, and access control

KubeOpenCode itself remains **stateless** and does **not depend on the Registry**. Without the Registry deployed, KubeOpenCode works exactly as it does today — skills from Git, plugins from public npm, images from any container registry. The Registry is a value-add for enterprise environments that need centralized asset management.

### Unified OCI Storage: Everything is an OCI Artifact

A unifying design principle: **all registry assets are stored as OCI artifacts in Zot**.

| Component | OCI Artifact Contents | What happens on upload |
|-----------|----------------------|----------------------|
| Skill | `SKILL.md` + `metadata.yaml` | Packed as OCI artifact → pushed to Zot with version tag |
| Plugin | npm package tarball | Mirrored to Verdaccio (in-cluster npm registry) |
| Image | Container image layers | **Built** via Kaniko/BuildKit → pushed to Zot as container image |

This unification means one storage backend (Zot) handles both container images and skill artifacts. Skills get the same versioning (tags), immutable references (digests), deduplication, garbage collection, and replication that container images already have.

**Why OCI artifacts for skills (not ConfigMap, not Git):**

| Approach | Pros | Cons |
|----------|------|------|
| **OCI artifact in Zot** | Unified backend with images; native versioning via tags; immutable digest references; Zot GC/dedup/replication for free; no size limits | Users can't `kubectl` skills directly; requires OCI tooling |
| ConfigMap | Pure K8s native; `kubectl` operable | 1MB etcd limit; no versioning; namespace-scoped (cross-namespace invisible); etcd pressure at scale |
| Git repo | Full version history; diff/blame | Needs a Git server component; sync complexity |

### Three Component Types

#### 1. Images (Agent Runtimes)

Container images that define the execution environment for agents. The Registry includes built-in images and supports user-uploaded custom images.

**Built-in images** (shipped with the Registry):

| Image | Contents | Size |
|-------|----------|------|
| `minimal` | Shell + git + curl | ~200MB |
| `go` | Go + gopls + delve | ~800MB |
| `node` | Node.js + npm + tsc | ~600MB |
| `python` | Python + pip + venv | ~500MB |
| `k8s` | kubectl + helm + kustomize | ~400MB |
| `devbox` | Full stack (Go + Node + Python + tools) | ~2-3GB |

**Custom images**: Users upload a Dockerfile (or provide a Git repo URL + Dockerfile path). The Registry builds it using Kaniko (default) or BuildKit and pushes to the in-cluster Zot registry. The user sees the build log in real-time and knows instantly if the image is usable.

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

Skills are stored as **OCI artifacts in Zot**. Each skill is a directory (`SKILL.md` + `metadata.yaml`) packed into an OCI artifact and pushed to Zot with a version tag:

```
zot.svc:5000/skills/kubernetes-ops:1.0.0
zot.svc:5000/skills/kubernetes-ops:1.0.0@sha256:abc123...
zot.svc:5000/skills/code-review:2.1.0
```

Users can upload, edit, and version skills through the Registry UI or CLI. The Registry server handles the OCI artifact packing/unpacking transparently.

#### 3. Plugins (Agent Extensions)

OpenCode plugins that extend agent behavior with integrations (Slack, Jira, GitHub, etc.), safety guardrails, and observability.

Plugins are **npm packages**. In enterprise/air-gapped environments, the Registry includes an **in-cluster npm registry (Verdaccio)** that mirrors approved plugins:

Plugin `metadata.yaml` (stored in the Registry for catalog/discovery purposes):

```yaml
name: slack-integration
description: "Slack bot integration — interact with agents via Slack messages"
tags: [slack, messaging, collaboration]
npmPackage: "@kubeopencode/opencode-slack-plugin"
version: "0.8.2"
target: server
requiredCredentials:
  - env: SLACK_BOT_TOKEN
    description: "Slack bot OAuth token"
  - env: SLACK_APP_TOKEN
    description: "Slack app-level token for Socket Mode"
```

**Air-gapped plugin workflow:**
1. While online: `npm install` required plugins through the in-cluster Verdaccio (auto-caches tarballs to PVC)
2. Transfer Verdaccio PVC content to air-gapped cluster
3. Air-gapped: remove `proxy` entries from Verdaccio config → fully offline npm registry
4. Plugin-init containers install from `http://verdaccio.kubeopencode-registry.svc:4873`

### The Visual Agent Assembler (Core UX)

The heart of the Registry is its **UI-based agent assembly experience**. There is no intermediate "recipe" abstraction — the UI itself IS the assembler. The output is a standard KubeOpenCode **AgentTemplate** (recommended for sharing across teams) or **Agent** YAML.

#### Why AgentTemplate as Default Output (Not Agent)

The Visual Assembler generates **AgentTemplate** YAML by default because:

- **Shareable** — AgentTemplate is a Kubernetes resource. Teams apply it once, then any team member creates Agents via `spec.templateRef.name`
- **Versionable** — Store the AgentTemplate YAML in Git for GitOps workflows
- **No new abstractions** — AgentTemplate already exists with established merge semantics (`template_merge.go`)
- **Avoids "recipe" complexity** — A recipe would need merge/override rules (what if two recipes conflict?). AgentTemplate's merge is already defined: scalar non-empty wins, lists replace entirely

A "Recipe" abstraction was considered and rejected — it would introduce merge ambiguity (merge two recipes? override? error?) without adding value beyond what AgentTemplate already provides.

#### Assembly Flow

```
┌─────────────────────────────────────────────────────────┐
│                   Registry UI                            │
│                                                          │
│  Step 1: Choose Executor Image                           │
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
│  Step 4: Review & Export                                  │
│  ┌─────────────────────────────────────────────┐        │
│  │ apiVersion: kubeopencode.io/v1alpha1        │        │
│  │ kind: AgentTemplate                          │        │
│  │ metadata:                                    │        │
│  │   name: go-dev-standard                      │        │
│  │ spec:                                        │        │
│  │   executorImage: zot.svc:5000/images/go      │        │
│  │     @sha256:abc123...                         │        │
│  │   skills:                                    │        │
│  │     - name: registry-skills                  │        │
│  │       oci:                                   │        │
│  │         images:                              │        │
│  │           - zot.svc:5000/skills/golang:1.0.0 │        │
│  │           - zot.svc:5000/skills/code-review  │        │
│  │             :2.1.0                            │        │
│  │   plugins:                                   │        │
│  │     - name: "@kubeopencode/opencode-slack-   │        │
│  │       plugin@0.8.2"                           │        │
│  │   workspaceDir: /workspace                   │        │
│  │   serviceAccountName: agent-sa               │        │
│  └─────────────────────────────────────────────┘        │
│                                                          │
│         [ Copy YAML ]  [ Generate Agent Instead ]        │
│                                                          │
└──────────────────────────────────────────────────────────┘

                    ↓ User copies YAML

$ kubectl apply -f go-dev-standard.yaml     # AgentTemplate available

# Any team member creates Agents referencing it:
$ cat my-agent.yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  templateRef:
    name: go-dev-standard
  port: 4096
  persistence:
    workspace:
      storageSize: 10Gi
```

### OCI Source Type in KubeOpenCode

KubeOpenCode gains a new **`oci` source type** for skills — alongside the existing `git` source. This is a standard OCI pull, not a custom Registry API. KubeOpenCode does not need to know anything about the Registry's internal implementation.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  executorImage: zot.kubeopencode-registry.svc:5000/images/go@sha256:abc123...

  skills:
    # Existing: Git source (unchanged)
    - name: external-skills
      git:
        repository: https://github.com/company/skills.git
        names: [golang, code-review]

    # NEW: OCI source — pull skill artifacts from any OCI registry
    - name: registry-skills
      oci:
        images:
          - zot.kubeopencode-registry.svc:5000/skills/kubernetes-ops:1.0.0
          - zot.kubeopencode-registry.svc:5000/skills/security-review:2.0.0
        # Optional: select specific skill subdirectories within the artifact
        names: [kubernetes-ops, security-review]

  plugins:
    # Existing: npm package name (unchanged — points to Verdaccio in air-gapped)
    - name: "@kubeopencode/opencode-slack-plugin@0.8.2"
```

**How it works:**

| Source | KubeOpenCode Controller Action |
|--------|-------------------------------|
| `skills[].git` | `git-init` container clones Git repo → mount SKILL.md files (existing behavior) |
| `skills[].oci` | `skill-init` container pulls OCI artifact (via `crane export`) → unpack to `/skills/{name}/` |
| `executorImage` | Standard image pull (unchanged — Zot URL is just a container registry address) |
| `plugins[].name` | `plugin-init` container runs `npm install` (unchanged — can point to Verdaccio via npm config) |

**Key design: controller uses OCI protocol, not a custom Registry API.** The controller doesn't call any Registry-specific REST endpoint. It pulls OCI artifacts the same way it pulls container images — via standard OCI Distribution API. This means:

- No coupling to the Registry server's API
- Any OCI-compliant registry works (Zot, Harbor, GHCR, ECR, etc.)
- The controller needs only one new init container type (`skill-init` using `crane`)
- If the Registry is not deployed, users can push skill OCI artifacts to any registry manually

**API changes in KubeOpenCode (minimal):**

```go
// SkillSource gains an OCI option alongside Git
type SkillSource struct {
    Name string           `json:"name"`
    Git  *GitSkillSource  `json:"git,omitempty"`
    OCI  *OCISkillSource  `json:"oci,omitempty"`  // NEW
}

// OCISkillSource pulls skills packaged as OCI artifacts
type OCISkillSource struct {
    // Images is a list of OCI artifact references containing SKILL.md files.
    // Each reference is an OCI image/artifact URL with optional tag or digest.
    // Example: "zot.svc:5000/skills/kubernetes-ops:1.0.0"
    // Example: "zot.svc:5000/skills/kubernetes-ops@sha256:abc123..."
    Images []string `json:"images"`
    // Names selects specific skill subdirectories within the artifacts.
    // If empty, all skills in the artifacts are mounted.
    // +optional
    Names []string `json:"names,omitempty"`
    // SecretRef references a Secret for OCI registry authentication.
    // +optional
    SecretRef *OCISecretReference `json:"secretRef,omitempty"`
}

// OCISecretReference references a Secret containing OCI registry credentials.
type OCISecretReference struct {
    // Name of the Secret in the same namespace.
    Name string `json:"name"`
}
```

**Plugin source for Verdaccio — no API change needed.** The existing `PluginSpec.Name` is an npm package specifier (e.g., `"@kubeopencode/opencode-slack-plugin@0.8.2"`). To point at an in-cluster Verdaccio instead of public npm, the controller sets the npm registry URL via environment variable in the plugin-init container. This is configurable via `KubeOpenCodeConfig`:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeConfig
metadata:
  name: cluster
spec:
  npmRegistry: "http://verdaccio.kubeopencode-registry.svc:4873"  # NEW optional field
```

When `npmRegistry` is set, all plugin-init containers use `npm install --registry={url}`. When unset, they use the default public npm registry. No change to Agent/Plugin CRD types.

### Versioning and Update Strategy

All three component types support explicit versioning:

#### Image Versioning
Standard OCI image tags and digests. Agent YAML references images by digest for immutability:
```yaml
executorImage: zot.svc:5000/images/go@sha256:abc123...  # Immutable
executorImage: zot.svc:5000/images/go:1.2.0             # Mutable tag
```

#### Skill Versioning
OCI artifact tags serve as version numbers. Three referencing strategies:

| Strategy | Example | Behavior | Use Case |
|----------|---------|----------|----------|
| **Digest** (immutable) | `skills/k8s-ops@sha256:abc...` | Never changes. Agent rebuild pulls same content. | Production |
| **Version tag** | `skills/k8s-ops:1.0.0` | Tag can be overwritten by Registry admin. Agent rebuild may get updated content. | Staging |
| **Latest tag** | `skills/k8s-ops:latest` | Always points to newest version. | Development |

The Registry UI can display an **"Update Available"** indicator by comparing the digest referenced in an AgentTemplate against the latest digest in the registry for that skill.

#### Plugin Versioning
npm semver — version is part of the package specifier (e.g., `"@kubeopencode/opencode-slack-plugin@0.8.2"`). Verdaccio caches specific versions. Updating means changing the version in the Agent spec.

### Architecture

The Registry is a **separate Helm chart** (`kubeopencode-registry`) deployed independently from KubeOpenCode. KubeOpenCode has no dependency on the Registry.

```
KubeOpenCode Helm Chart (kubeopencode-system namespace)
┌────────────────────────────────────────────────────────┐
│  Existing components (unchanged except where noted):    │
│  ┌──────────────────────────────────────────────┐      │
│  │  Controller (Deployment)                      │      │
│  │  • Agent, Task, CronTask reconcilers          │      │
│  │  • NEW: OCI skill source resolver             │      │
│  │    (skill-init container using crane pull)     │      │
│  └──────────────────────────────────────────────┘      │
│  ┌──────────────────────────────────────────────┐      │
│  │  Server + UI (Deployment, port 2746)          │      │
│  │  • Agents, Tasks, CronTasks pages (unchanged) │      │
│  └──────────────────────────────────────────────┘      │
│  ┌──────────────────────────────────────────────┐      │
│  │  KubeOpenCodeConfig (cluster singleton)       │      │
│  │  • NEW optional field: npmRegistry            │      │
│  └──────────────────────────────────────────────┘      │
└────────────────────────────────────────────────────────┘

KubeOpenCode Registry Helm Chart (kubeopencode-registry namespace)
┌────────────────────────────────────────────────────────┐
│  ┌──────────────────────────────────────────────┐      │
│  │  Registry Server (Deployment)                 │      │
│  │  • Registry REST API                          │      │
│  │    - Skill CRUD (pack/unpack OCI artifacts)   │      │
│  │    - Plugin catalog (metadata + Verdaccio)    │      │
│  │    - Image builds (Kaniko jobs / BuildKit)    │      │
│  │  • Visual Assembler UI                        │      │
│  │    - Browse / upload / manage assets           │      │
│  │    - Pick components → generate AgentTemplate │      │
│  │  • Build orchestrator                         │      │
│  │    - Kaniko: creates K8s Jobs                 │      │
│  │    - BuildKit: gRPC client (optional)         │      │
│  │  • OCI client (push skill artifacts to Zot)   │      │
│  └──────────────────────────────────────────────┘      │
│                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ Zot          │  │ Verdaccio    │  │ BuildKit     │  │
│  │ (Deployment  │  │ (Deployment  │  │ daemon       │  │
│  │  + PVC)      │  │  + PVC)      │  │ (optional    │  │
│  │ images +     │  │ npm packages │  │  Deployment) │  │
│  │ skill OCI    │  │              │  │              │  │
│  │ artifacts    │  │              │  │              │  │
│  └──────────────┘  └──────────────┘  └──────────────┘  │
└────────────────────────────────────────────────────────┘

Communication (all standard protocols):
  KubeOpenCode Controller ──OCI pull──→ Zot (images + skills)
  KubeOpenCode Controller ──npm install──→ Verdaccio (plugins)
  Registry Server ──OCI push──→ Zot
  Registry Server ──Kaniko Job / BuildKit gRPC──→ Build → Zot
```

**Helm values for the Registry chart:**
```yaml
# charts/kubeopencode-registry/values.yaml
registry:
  image: ghcr.io/kubeopencode/kubeopencode-registry:latest
  replicas: 1
  service:
    port: 8080

zot:
  enabled: true
  image: ghcr.io/project-zot/zot-linux-amd64:v2.1.8
  storageSize: 50Gi

verdaccio:
  enabled: true
  image: verdaccio/verdaccio:6
  storageSize: 10Gi

build:
  engine: kaniko          # "kaniko" (default, PSS Restricted safe) or "buildkit"
  kaniko:
    image: registry.gitlab.com/gitlab-ci-utils/container-images/kaniko:debug
  buildkit:               # Only used when engine: buildkit
    enabled: false
    image: moby/buildkit:v0.29.0-rootless
    cacheSize: 10Gi
```

### In-Cluster Image Building: Kaniko (Default) + BuildKit (Optional)

When a user uploads a Dockerfile through the Registry UI:

1. **Registry server** receives the Dockerfile (or Git repo URL + Dockerfile path)
2. **Registry server** creates a **Kaniko Job** (or calls BuildKit daemon) to build the image
3. The build engine pushes the image to the in-cluster **Zot** registry
4. **Registry server** streams build logs to the UI in real-time
5. On success, the image is listed as available with its digest
6. User can select this image in the visual assembler

#### Two Build Engines

| Aspect | Kaniko (default) | BuildKit (optional) |
|--------|-----------------|---------------------|
| **PSS Restricted compatible** | **Yes** — runs fully unprivileged | **No** — requires `allowPrivilegeEscalation: true` |
| **Security model** | Userspace-only, no kernel namespaces | Rootless with `newuidmap` setuid binary |
| **Deployment** | No daemon — Registry creates K8s Jobs on demand | Persistent Deployment (daemon) |
| **Multi-arch builds** | Not supported | Supported via QEMU or cross-compilation |
| **Build caching** | Registry cache in Zot (`--cache=true --cache-repo=zot.svc:5000/cache`) | Layer cache on PVC |
| **Build isolation** | Chroot (not container-isolated) | Full container isolation |
| **Performance** | Adequate for agent images | Faster for large/complex builds |
| **Maintenance** | Chainguard or osscontainertools fork (Google archived original June 2025) | Official moby/buildkit |

**Why Kaniko as default (not BuildKit):**

BuildKit rootless mode **requires `allowPrivilegeEscalation: true`** — this is a hard Linux kernel requirement, not a configuration choice. The `rootlesskit` binary needs `newuidmap` (setuid) to establish user namespace mappings with multiple UID entries. The `PR_SET_NO_NEW_PRIVS` flag set by `allowPrivilegeEscalation: false` blocks setuid execution entirely.

This means BuildKit cannot run under **Pod Security Standards Restricted profile**, which many enterprise clusters enforce. The BuildKit maintainer (AkihiroSuda) has confirmed this is an inherent limitation:

> `allowPrivilegeEscalation` has to be true for initializing the user namespace with `newuidmap` setuid binary.

Kubernetes User Namespaces (`hostUsers: false`, beta since K8s 1.30, enabled by default in 1.33) allow running rootful BuildKit safely, but PSS Restricted still syntactically rejects `privileged: true` even with user namespaces active.

Kaniko builds entirely in userspace without kernel namespace operations and runs cleanly under PSS Restricted:

```yaml
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]
  runAsNonRoot: true
  seccompProfile:
    type: RuntimeDefault
```

**We do not accept "build externally and push to Zot" as a fallback** — this defeats the core value proposition of in-cluster building and the Registry's ease-of-use promise. Kaniko ensures every cluster can build images regardless of security policy.

#### Kaniko Build (Default)

The Registry server creates a Kubernetes Job for each build:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: build-go-image-1715000000
  namespace: kubeopencode-registry
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 3600
  template:
    spec:
      restartPolicy: Never
      securityContext:
        runAsNonRoot: true
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: kaniko
          image: registry.gitlab.com/gitlab-ci-utils/container-images/kaniko:debug
          args:
            - "--dockerfile=Dockerfile"
            - "--context=dir:///workspace"
            - "--destination=zot.kubeopencode-registry.svc:5000/images/go:1.0.0"
            - "--cache=true"
            - "--cache-repo=zot.kubeopencode-registry.svc:5000/cache"
            - "--insecure"  # In-cluster Zot has no TLS
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
          volumeMounts:
            - name: build-context
              mountPath: /workspace
      volumes:
        - name: build-context
          configMap:
            name: build-go-image-1715000000-context
```

**Advantages of the Job-based approach:**
- **Stateless Registry server** — No build state to track; the Job IS the state. Query `Job.status` for build progress.
- **Concurrent builds** — Each build is an independent Job. No single-daemon bottleneck.
- **Resource isolation** — Each build gets its own resource limits. Builds don't compete with the Registry server for CPU/memory.
- **Crash recovery** — If the Registry server restarts mid-build, the Job continues. Server reconnects via `kubectl logs --follow`.
- **Cleanup** — `ttlSecondsAfterFinished` auto-deletes completed Jobs.

**Log streaming:** The Registry server streams build logs to the UI via `kubectl logs --follow` on the Job's Pod. This is simpler and more robust than BuildKit's `statusCh` gRPC channel.

#### BuildKit Daemon (Optional, for Advanced Users)

For clusters with relaxed Pod Security Standards (Baseline or Privileged) that need multi-architecture builds or faster performance:

```yaml
build:
  engine: buildkit
  buildkit:
    enabled: true
```

BuildKit runs as a single Deployment in rootless mode:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: buildkitd
  namespace: kubeopencode-registry
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
            seccompProfile:
              type: Unconfined
            appArmorProfile:
              type: Unconfined
            # allowPrivilegeEscalation defaults to true (required by rootlesskit)
          ports:
            - containerPort: 1234
          volumeMounts:
            - name: buildkit-cache
              mountPath: /home/user/.local/share/buildkit
      volumes:
        - name: buildkit-cache
          persistentVolumeClaim:
            claimName: buildkit-cache
```

The Registry server calls BuildKit via Go gRPC SDK (`client.Solve()`) with `statusCh` for real-time log streaming.

#### Why Not Shipwright + Tekton

Shipwright + Tekton was considered but rejected as over-engineering:

| Aspect | Kaniko Job / BuildKit direct | Shipwright + Tekton |
|--------|------------------------------|---------------------|
| Core build | `kubectl create job` / `client.Solve()` | Build CRD → BuildRun CRD → Tekton TaskRun → Pod |
| Dependencies | 0 operators | Tekton operator + Shipwright operator + CRDs |
| Features we need | Build Dockerfile, push to registry | Build Dockerfile, push to registry |
| Features we DON'T need | — | Multi-strategy, Pipeline integration, Git triggers |

#### Zot In-Cluster Registry

Zot (CNCF Sandbox) runs as a Deployment + PVC, serving as the unified storage for both container images and skill OCI artifacts:

- **PVC-based storage** — No S3 dependency. Works in any cluster.
- **Deduplication** — Agent images share base layers (debian:bookworm-slim); skills share metadata layers. Dedup saves significant storage.
- **Garbage collection** — Auto-cleanup of old digests (configurable).
- **In-cluster only** — ClusterIP Service, no Ingress. Assets never leave the cluster.
- **Dual purpose** — Stores both container images (`/images/*`) and skill OCI artifacts (`/skills/*`).
- **Single binary** — Minimal footprint, OCI-native.

#### Verdaccio In-Cluster npm Registry

Verdaccio runs as a Deployment + PVC, providing an in-cluster npm registry for plugin packages:

- **On-demand caching** — Proxies requests to public npm; caches tarballs locally on first install.
- **Offline mode** — Remove `proxy` entries from config for fully air-gapped operation.
- **Proven at scale** — Used by create-react-app, babel, pnpm, storybook in their CI systems.
- **Standard npm protocol** — `npm install`, `npm publish`, scoped packages, audit — fully compatible.
- **Simple config** — YAML-based configuration, htpasswd auth.

### Cluster-Scoped Deployment, Namespace-Level Access Control

The Registry is deployed **cluster-scoped** — one Registry instance serves the entire cluster.

**Rationale:**

| Dimension | Per-Namespace | Cluster-Scoped |
|-----------|---------------|----------------|
| Zot images | N copies of same base images | One set, shared across all namespaces |
| Verdaccio npm | N identical caches | One cache |
| Skills | Invisible across namespaces | Shared catalog, RBAC controls visibility |
| Build infra | N Kaniko service accounts | One build namespace |
| Ops cost | N sets of PVCs, Deployments | 1 set |

**Access control via OCI path conventions + Kubernetes RBAC:**

```
zot.svc:5000/images/{name}          # Shared images (all teams)
zot.svc:5000/skills/{name}          # Shared skills (all teams)
zot.svc:5000/ns/{namespace}/images/ # Namespace-scoped images (team-specific)
zot.svc:5000/ns/{namespace}/skills/ # Namespace-scoped skills (team-specific)
```

Registry server RBAC checks the requesting user's Kubernetes permissions (via TokenReview/SubjectAccessReview) before allowing push/pull operations on namespace-scoped paths.

### Supersedes ADR 0015

This design **supersedes ADR 0015** (Repo as Agent — Dynamic Image Building). ADR 0015 proposed BuildKit + Zot as a controller-level concern with `Agent.spec.build`. This ADR takes a different angle: builds are managed through the **Registry UI and server API**, not through the Agent CRD.

| Aspect | ADR 0015 | ADR 0036 (this) |
|--------|----------|----------------|
| Build trigger | `Agent.spec.build` field (controller watches) | Registry UI / REST API (user-initiated) |
| Build engine | BuildKit only | Kaniko (default, PSS safe) + BuildKit (optional) |
| Image storage | Zot | Zot (same) |
| Skill storage | Not addressed | OCI artifacts in Zot |
| Plugin storage | Not addressed | Verdaccio in-cluster npm registry |
| User experience | YAML-only, build status in Agent.status | UI + CLI with real-time build logs + visual assembler |
| Agent CRD changes | New `BuildConfig` struct | Minimal: `OCI` source for skills only |
| KubeOpenCode coupling | Tight (controller builds images) | Loose (standard OCI + npm protocols) |
| Deployment | Part of KubeOpenCode Helm chart | Separate Helm chart |

Key improvements:
- Builds are decoupled from the Agent lifecycle — build once, reference in many Agents
- Kaniko enables building in PSS Restricted clusters (BuildKit cannot)
- Registry is independent — KubeOpenCode stays stateless and lean
- Unified OCI storage for both images and skills

### Key Design Decisions

#### 1. AgentTemplate as Assembler Output (No Recipe Abstraction)

**Decision**: The Visual Assembler generates **AgentTemplate YAML** (or optionally Agent YAML). There is no "recipe" CRD, file format, or intermediate abstraction.

**Rationale**:
- AgentTemplate already exists as a shareable, reusable configuration resource
- Merge semantics are already implemented in `template_merge.go` (scalar: non-empty wins; lists: replace)
- A Recipe would introduce ambiguity: if a user selects two Recipes, do we merge or override? With AgentTemplate, this question doesn't exist — an Agent references exactly one template
- Users can share AgentTemplates via `kubectl`, Git, or any standard K8s tooling
- The generated YAML is self-contained and auditable — no runtime resolution

#### 2. Registry as an Independent Project (Not Integrated in KubeOpenCode)

**Decision**: The Registry is a separate Helm chart (`kubeopencode-registry`) with its own release cycle. KubeOpenCode has no compile-time or runtime dependency on the Registry.

**Rationale**:
- KubeOpenCode stays **stateless** — no build orchestration, no artifact management, no additional PVCs
- Clean separation of concerns — KubeOpenCode manages workloads (Agent/Task); Registry manages assets (images/skills/plugins)
- Independent release cycles — Registry can iterate quickly without affecting KubeOpenCode stability
- Users who don't need the Registry get zero overhead — not even disabled Helm values to maintain
- The communication uses **standard protocols** (OCI pull, npm install) — no custom API coupling

#### 3. OCI Source Type for Skills (Not Custom Registry API)

**Decision**: KubeOpenCode gains an `oci` source type for skills. The controller pulls skill OCI artifacts using standard OCI Distribution protocol (`crane export`), not a custom Registry REST API.

**Rationale**:
- **No coupling to Registry internals** — the controller doesn't need to know the Registry's API contract
- **Any OCI registry works** — Zot, Harbor, GHCR, ECR, ACR. Users without the Registry can push skill artifacts to any OCI registry
- **Consistent with existing patterns** — `git-init` clones repos; `skill-init` pulls OCI artifacts. Same init-container pattern
- **No new protocol to maintain** — OCI Distribution is a CNCF standard with mature Go libraries (`crane`, `oras`)
- **Plugin source unchanged** — npm protocol handles plugins; the `npmRegistry` field in KubeOpenCodeConfig directs to Verdaccio when deployed

#### 4. Kaniko as Default Build Engine (PSS Restricted Compatible)

**Decision**: Kaniko is the default build engine. BuildKit is available as an opt-in alternative for clusters with relaxed Pod Security Standards.

**Rationale**:
- BuildKit rootless **requires `allowPrivilegeEscalation: true`** — a hard Linux kernel limitation due to `newuidmap` setuid requirement. This blocks PSS Restricted clusters.
- "Build externally and push to Zot" is **not an acceptable fallback** — it defeats the ease-of-use promise
- Kaniko runs fully unprivileged: `allowPrivilegeEscalation: false`, `drop: ALL`, `runAsNonRoot: true`, `seccomp: RuntimeDefault`
- Job-based approach (Kaniko) keeps the Registry server stateless — no build state, no gRPC sessions, no crash recovery concerns
- BuildKit remains available for users who need multi-arch builds or better performance on PSS Baseline/Privileged clusters
- Google archived Kaniko in June 2025, but active forks exist: Chainguard (`chainguard-forks/kaniko`), osscontainertools (`osscontainertools/kaniko` v1.27.2), GitLab mirror

#### 5. Immediate Build Feedback

**Decision**: When a user uploads a Dockerfile, the build starts immediately and the user sees logs in real-time. The image is either usable or the build error is shown.

**Rationale**:
- "Upload Dockerfile → see if it works → use it" is the core UX promise
- Deferred or background builds break the interactive flow
- Build failures surfaced immediately prevent wasted time debugging Agent startup issues later

#### 6. Optional by Default

**Decision**: The entire Registry is a separate Helm chart, disabled by default. KubeOpenCode works without it.

**Rationale**:
- Users who just want KubeOpenCode for running Tasks/Agents should not be affected at all
- Progressive adoption: start with manual YAML → later deploy Registry for UI-based management
- Zero overhead for non-Registry users — not even conditional Helm values

## Implementation Plan

### Phase 1: Built-in Content + CLI + OCI Source Type

**Goal**: Ship built-in skills and image Dockerfiles in the repo. Add OCI skill source type to KubeOpenCode. CLI for browsing. Images built by GitHub Actions and pushed to GHCR. Skills pushed to GHCR as OCI artifacts.

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

2. **API: `oci` source type** (KubeOpenCode CRD change)
   - Add `OCISkillSource` to `SkillSource` in `api/v1alpha1/skill_types.go`
   - Implement `skill-init` container in `cmd/kubeopencode/skill_init.go` (uses `crane export`)
   - Update `skill_processor.go` to handle OCI source alongside Git
   - Update `pod_builder.go` to create skill-init containers for OCI sources
   - Add `NpmRegistry` field to KubeOpenCodeConfig for Verdaccio support
   - Run `make update` → CRD regeneration
   - Unit tests + integration tests + documentation

3. **CLI: `kubeoc registry`**
   - `kubeoc registry list [--type images|skills|plugins]` — Browse built-in components
   - `kubeoc registry show <name>` — Show component details

4. **CI**: Build registry Dockerfiles on merge → push to GHCR. Pack skills as OCI artifacts → push to GHCR.

### Phase 2: Registry Server + UI + In-Cluster Build

**Goal**: Full integrated experience — Registry server with UI, in-cluster image building, visual agent assembly.

1. **Registry Server** (new binary: `kubeopencode-registry`)
   - REST API:
     - `GET /api/v1/skills` — List skills (from Zot catalog)
     - `GET /api/v1/skills/{name}` — Get skill details (from Zot)
     - `POST /api/v1/skills` — Upload skill (pack as OCI artifact → push to Zot)
     - `PUT /api/v1/skills/{name}` — Update skill
     - `DELETE /api/v1/skills/{name}` — Delete skill
     - `GET /api/v1/plugins` — List plugins (from Verdaccio)
     - `GET /api/v1/images` — List images (from Zot catalog)
     - `POST /api/v1/images` — Upload Dockerfile → create Kaniko Job → build → push to Zot
     - `GET /api/v1/builds` — List builds (from K8s Jobs)
     - `GET /api/v1/builds/{id}/logs` — Stream build logs (SSE/WebSocket via `kubectl logs --follow`)
     - `POST /api/v1/assemble` — Generate AgentTemplate YAML from selected components

2. **Registry UI**
   - **Assets** page: Browse/search/upload skills, plugins, images
   - **Images** page: Upload Dockerfile → see build logs → success/fail
   - **Assemble** page: Pick image → pick skills → pick plugins → generate AgentTemplate YAML → copy button
   - **Builds** page: Build history with logs

3. **Helm chart**: `charts/kubeopencode-registry/`
   - Registry Server Deployment + Service
   - Zot Deployment + PVC + Service
   - Verdaccio Deployment + PVC + Service + ConfigMap
   - Optional BuildKit Deployment + PVC + Service
   - RBAC (ServiceAccount, ClusterRole for Job creation)

### Phase 3: Enterprise Features

1. **RBAC** — Role-based access: who can upload images, who can edit skills, namespace-scoped paths
2. **Audit trail** — Track asset changes with timestamps and user attribution
3. **Scheduled rebuilds** — Cron-based rebuild for base image security patches
4. **Multi-cluster sync** — Zot replication for multi-cluster deployments
5. **Update notifications** — Detect when AgentTemplate references outdated skill/image digests

## Consequences

### Positive

1. **Enterprise-ready agent governance** — All agent assets in one managed, auditable, versioned location
2. **Dramatically lower barrier to entry** — Visual assembler: pick components, generate AgentTemplate, apply
3. **Air-gapped support (complete)** — Images in Zot, skills as OCI artifacts in Zot, plugins in Verdaccio — no public internet dependency at runtime
4. **PSS Restricted compatible** — Kaniko default build engine works in the strictest security environments
5. **KubeOpenCode stays stateless** — No build orchestration, no artifact management, no additional state in the core project
6. **Standard protocols** — OCI Distribution for images/skills, npm for plugins — no custom APIs between KubeOpenCode and Registry
7. **AgentTemplate as output** — Shareable, versionable, auditable — teams standardize on approved configurations
8. **Unified OCI storage** — One Zot instance for images and skills; consistent versioning, GC, and replication

### Negative

1. **Two Helm charts to manage** — Registry is a separate install. Users must deploy and maintain it independently.
2. **Maintenance burden** — Built-in skills, images, and plugin metadata need ongoing updates. Kaniko fork needs tracking.
3. **Initial investment** — Building the Registry server + UI is a significant development effort.
4. **OCI artifact complexity** — Users unfamiliar with OCI concepts may find skill storage less intuitive than ConfigMap or Git.

### Risks

| Risk | Mitigation |
|------|------------|
| Registry UI development takes too long | Start with OCI source type + CLI (Phase 1) to provide value before UI is ready |
| Kaniko fork maintenance uncertainty | Track Chainguard and osscontainertools forks; they are actively maintained and used by GitLab |
| Kaniko build isolation weaker than BuildKit | Kaniko uses chroot (not container isolation). Only build trusted Dockerfiles. For untrusted builds, use BuildKit on PSS Baseline clusters |
| Verdaccio as additional component to maintain | Verdaccio is mature, low-maintenance (single Deployment + PVC). Can be disabled if air-gap npm is not needed |
| Zot PVC runs out of storage | GC policies; storage monitoring alerts; configurable PVC size in Helm values |
| OCI skill artifacts unfamiliar to users | Registry UI abstracts OCI details; users upload SKILL.md files, UI handles packing |

## Alternatives Considered

### Alternative 1: Recipe Abstraction Layer

Add a `Recipe` file format that pre-combines image + skills + plugins into a named configuration.

**Rejected because:**
- AgentTemplate already serves this purpose as a Kubernetes-native resource
- Recipes introduce merge ambiguity: what happens when two recipes conflict? AgentTemplate avoids this — Agent references exactly one template
- Recipes would need their own versioning and compatibility matrix — complexity without value
- Users want standard Kubernetes resources, not custom file formats

### Alternative 2: Build Images in Agent Controller (ADR 0015)

Embed builds in the Agent controller via `Agent.spec.build` — every Agent with a `build` field triggers a build.

**Superseded because:**
- Couples image lifecycle to Agent lifecycle — image should exist independently and be referenced by many Agents
- Controller becomes stateful (build tracking) — violates KubeOpenCode's stateless principle
- No UI for build management — users only see build status in `Agent.status`
- BuildKit-only — no PSS Restricted support

### Alternative 3: ConfigMap for Skill Storage

Store skills as ConfigMap data in the cluster.

**Rejected because:**
- 1MB etcd object size limit — large skills may exceed it
- Namespace-scoped — cross-namespace sharing requires duplication
- No native versioning — requires naming conventions (`skill-k8s-ops-v1`)
- etcd pressure — skills are static content that shouldn't occupy etcd storage
- No GC, dedup, or replication

### Alternative 4: Custom Registry REST API for Skills (Instead of OCI)

Have the controller fetch skills via `GET {registry-url}/api/v1/skills/{name}` (custom HTTP API).

**Rejected because:**
- Creates a coupling between KubeOpenCode controller and Registry server API
- Controller needs a custom HTTP client for a Registry-specific contract
- Only works with the KubeOpenCode Registry — not portable to other OCI registries
- OCI Distribution is a CNCF standard with mature tooling; custom APIs add maintenance burden

### Alternative 5: Integrated Registry in KubeOpenCode (Not Separate)

Build the Registry into the KubeOpenCode Helm chart as optional components.

**Rejected because:**
- KubeOpenCode Server would become stateful (build orchestration, artifact management)
- Build state tracking across server replicas requires shared state
- Server crashes during builds lose state (BuildKit continues but status is gone)
- Independent release cycles allow Registry to iterate without affecting KubeOpenCode stability
- Clean separation is more maintainable long-term

### Alternative 6: Shipwright + Tekton for Build Orchestration

Use Shipwright CRDs for build lifecycle management instead of Kaniko/BuildKit directly.

**Rejected because:**
- Our build requirements are simple: Dockerfile → build → push to Zot
- Shipwright adds two operator dependencies (Tekton + Shipwright) for capabilities we don't need
- Kaniko Jobs and BuildKit direct calls handle the entire lifecycle simply
- Fewer moving parts = easier to debug, deploy, and maintain

### Alternative 7: Public Marketplace

Build a public marketplace where anyone can publish and discover agent components.

**Deferred because:**
- Enterprise users need internal, governed registries — not public marketplaces
- Supply chain security concerns make public assets risky for production use
- A public catalog can be built later on top of the same Registry feature

### Alternative 8: Zot for npm Packages (Unified Registry)

Store npm packages as OCI artifacts in Zot to avoid deploying Verdaccio.

**Rejected because:**
- Zot does not implement the npm registry API — `npm install` cannot talk to Zot natively
- npm clients and OCI registries speak completely different protocols (npm Registry API vs OCI Distribution API)
- No off-the-shelf npm-to-OCI translation proxy exists
- Building a custom protocol translator is more effort than deploying Verdaccio
- Verdaccio is a mature, lightweight (single Deployment + PVC) solution purpose-built for this

## References

- [Rivet agentOS Registry](https://rivet.dev/agent-os/registry) — Inspiration for the building-block approach
- [Kaniko](https://github.com/GoogleContainerTools/kaniko) — Userspace container image builder (PSS Restricted compatible)
- [Chainguard Kaniko Fork](https://github.com/chainguard-forks/kaniko) — Actively maintained Kaniko fork
- [BuildKit](https://github.com/moby/buildkit) — Go SDK for container image building (requires privilege escalation)
- [Zot Registry](https://zotregistry.dev/) — OCI-native container registry (CNCF Sandbox)
- [Verdaccio](https://verdaccio.org/) — Lightweight npm proxy registry
- [ORAS](https://oras.land/) — OCI Registry As Storage — tooling for OCI artifacts
- [crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane) — OCI image/artifact manipulation tool
- [Artifact Hub](https://artifacthub.io/) — Precedent for Kubernetes ecosystem package discovery
- [ADR 0015](0015-repo-as-agent-dynamic-image-building.md) — Dynamic image building (superseded)
- [ADR 0026](0026-skills.md) — Skills as a top-level Agent field
- [ADR 0034](0034-plugin-support-and-slack-integration.md) — Plugin support
- [BuildKit Rootless Security Analysis](https://github.com/moby/buildkit/blob/master/docs/rootless.md) — Why `allowPrivilegeEscalation: true` is required
- [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) — PSS Restricted profile requirements
