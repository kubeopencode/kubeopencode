# ADR 0041: Agent-Sandbox API Gap Analysis

## Status

Research / Informational

## Date

2026-05-31

## Context

[Agent-Sandbox](https://github.com/kubernetes-sigs/agent-sandbox) (`kubernetes-sigs/agent-sandbox`) is a Kubernetes SIG Apps project that provides a `Sandbox` CRD for managing isolated, stateful, singleton workloads. It is designed for AI agent runtimes, development environments, and notebooks. Given its traction and capabilities, we evaluated whether KubeOpenCode could adopt Agent-Sandbox as its underlying runtime.

This ADR provides an objective analysis from two angles:
1. **What KubeOpenCode features can be implemented using Agent-Sandbox** (through direct mapping or adapter layer)
2. **What KubeOpenCode features cannot be implemented** (architectural or API limitations)

## KubeOpenCode Runtime Architecture

KubeOpenCode currently manages two runtime patterns:

**1. Agent (Server Mode) — Long-running instance:**
```
Agent CRD → Deployment (replicas: 0 or 1) + ClusterIP Service + optional PVCs
  │
  └── Pod with init containers:
      ├── opencode-init: copies OpenCode binary to /tools
      ├── context-init: copies ConfigMap content to workspace
      ├── git-init-*: clones Git repositories
      ├── plugin-init: npm install plugins to /plugins
      └──
      └── opencode-server container: runs `opencode serve`
```

**2. Task — Ephemeral execution:**
```
Task CRD → Pod
  │
  ├── agentRef path: lightweight Pod runs `opencode run --attach <agent-url>`
  └── templateRef path: standalone Pod runs `opencode run` with full devbox
```

## Agent-Sandbox Architecture

Agent-Sandbox provides three core CRDs:

```
SandboxTemplate → defines PodTemplate, VolumeClaimTemplates, NetworkPolicy, Service config
       │
       ▼
SandboxClaim ──► Sandbox ──► Pod + Service + PVCs
       │
       └── SandboxWarmPool: pre-created Sandbox pool for fast adoption
```

Key capabilities:
- **PodTemplate**: Full Kubernetes PodSpec with init containers, volumes, env vars
- **VolumeClaimTemplates**: Automatic PVC creation and mounting
- **Lifecycle**: `shutdownTime`, `shutdownPolicy`, `OperatingMode` (Running/Suspended)
- **Service**: Optional headless Service creation
- **WarmPool**: Pre-warmed Sandbox pool for sub-second allocation
- **NetworkPolicy**: Automatic default-deny NetworkPolicy per template
- **Env Injection**: SandboxClaim can inject env vars into specific containers

---

## Part 1: Features Implementable with Agent-Sandbox

### 1.1 Direct Equivalence Mapping (Minimal or No Changes)

These features map almost 1:1 between KubeOpenCode and Agent-Sandbox:

#### a) Template Mode (Standalone Task Execution)

| KubeOpenCode | Agent-Sandbox | Mapping |
|--------------|---------------|---------|
| `Task.spec.templateRef` | `SandboxClaim.spec.sandboxTemplateRef` | 1:1 |
| `AgentTemplate` | `SandboxTemplate` | 1:1 |
| Task lifecycle | SandboxClaim lifecycle | Equivalent |

**Implementation**: KubeOpenCode controller translates Task CR to SandboxClaim CR. Agent-Sandbox handles Pod creation, PVC binding, and lifecycle management.

**Additional benefits**:
- SandboxClaim's `shutdownTime` and `TTLSecondsAfterFinished` provide richer lifecycle controls than KubeOpenCode's current Task timeout
- `shutdownPolicy` (Delete/DeleteForeground/Retain) offers flexible cleanup options

#### b) Pod Lifecycle Management

KubeOpenCode's controller currently creates/deletes/monitors Pods directly. Agent-Sandbox's Sandbox controller does the same, but with additional features:
- Automatic Pod recreation (when Pod is deleted)
- Pod metadata drift reconciliation
- Warm pool adoption for fast startup

**Implementation**: Replace direct Pod management with Sandbox/SandboxClaim creation.

#### c) Persistence (PVC)

| KubeOpenCode | Agent-Sandbox | Mapping |
|--------------|---------------|---------|
| `Agent.spec.persistence.sessions` | `Sandbox.spec.volumeClaimTemplates` | 1:1 |
| `Agent.spec.persistence.workspace` | `Sandbox.spec.volumeClaimTemplates` | 1:1 |
| Session PVC (1Gi default) | VolumeClaimTemplate with size | Equivalent |
| Workspace PVC (10Gi default) | VolumeClaimTemplate with size | Equivalent |

**Implementation**: Translate `PersistenceConfig` to `PersistentVolumeClaimTemplate` array.

#### d) Suspend / Resume

| KubeOpenCode | Agent-Sandbox | Mapping |
|--------------|---------------|---------|
| `Agent.spec.suspend` | `Sandbox.spec.operatingMode=Suspended` | 1:1 |
| Deployment replicas=0 | Pod deletion + recreation | Equivalent behavior |

**Implementation**: Map `Agent.spec.suspend` directly to `Sandbox.spec.operatingMode`.

#### e) PodSpec Advanced Configuration

All standard Kubernetes Pod configurations are supported via `PodTemplate.spec`:
- `nodeSelector`, `tolerations`, `affinity` → `PodTemplate.spec` fields
- `runtimeClassName` → `PodTemplate.spec.runtimeClassName`
- `securityContext` (container-level) → `PodTemplate.spec.containers[*].securityContext`
- `podSecurityContext` → `PodTemplate.spec.securityContext`
- `lifecycle` (postStart/preStop) → `PodTemplate.spec.containers[*].lifecycle`
- `resources` → `PodTemplate.spec.containers[*].resources`
- `extraVolumes` / `extraVolumeMounts` → `PodTemplate.spec.volumes` / `containers[*].volumeMounts`
- `imagePullSecrets` → `PodTemplate.spec.imagePullSecrets`

**Implementation**: Direct passthrough from Agent spec to Sandbox PodTemplate.

#### f) Service Exposure (Internal Communication)

Agent-Sandbox creates a headless Service (`ClusterIP: None`) when `sandbox.spec.service=true`.

For KubeOpenCode's use case (cluster-internal communication between Task Pod and Agent):
- Headless Service DNS resolves to Pod IP: `{sandbox-name}.{namespace}.svc.{cluster-domain}`
- This is functionally equivalent to ClusterIP for single-Pod access
- Task Pods can connect via the same URL pattern

**Note**: For external access (Ingress, port-forward), both ClusterIP and headless Service require additional configuration. No functional difference.

### 1.2 Implementable via Adapter Layer

These features are not natively in Agent-Sandbox, but can be implemented by KubeOpenCode controller generating the appropriate PodTemplate:

#### a) Multi-Container Init Chain

KubeOpenCode's ordered init container chain:
```
opencode-init → context-init → git-init-* → plugin-init → worker
```

**Implementation**: KubeOpenCode controller generates all init containers and mounts into `PodTemplate.spec.initContainers` and `PodTemplate.spec.volumes`.

- `opencode-init`: Copies binary to shared `/tools` emptyDir
- `context-init`: Copies ConfigMap content to workspace emptyDir
- `git-init-*`: One per Git context, clones to dedicated emptyDir volumes
- `plugin-init`: npm install to shared `/plugins` emptyDir

All volume sharing contracts (emptyDir mounts, env vars) are expressed in PodTemplate. No Agent-Sandbox changes needed.

#### b) Context Processing System

KubeOpenCode resolves five context types (Text, ConfigMap, Git, Runtime, URL) at reconcile time.

**Implementation**:
1. KubeOpenCode controller resolves contexts (current logic unchanged)
2. Generates a ConfigMap with aggregated content
3. Injects ConfigMap volume mount into PodTemplate
4. `context-init` container (in PodTemplate) copies ConfigMap data to workspace

The context processing logic lives entirely in KubeOpenCode controller. Agent-Sandbox just runs the resulting Pod.

#### c) Credentials Management

KubeOpenCode's `Credential` abstraction supports:
- Entire Secret as env vars or directory mount
- Single key as env var or file
- Custom fileMode

**Implementation**: Controller translates `Credential` array to:
- `PodTemplate.spec.volumes` (Secret volumes)
- `PodTemplate.spec.containers[*].volumeMounts`
- `PodTemplate.spec.containers[*].env` / `envFrom`

This is a straightforward translation layer (~100 lines of code).

#### d) OpenCode Configuration Injection

KubeOpenCode generates `opencode.json` by merging:
- User-provided `Agent.spec.config`
- Auto-discovered `skills.paths`
- Resolved `plugin` array with `file:///plugins/node_modules/...` paths
- Runtime instructions (`OPENCODE_CONFIG_CONTENT`)
- OTel experimental config

**Implementation**:
1. Controller performs all config merging (current logic)
2. Injects merged config as environment variables or writes to ConfigMap
3. PodTemplate references the ConfigMap or env vars
4. `context-init` or entrypoint script writes config to `/tools/opencode.json`

#### e) Skills and Plugins

**Skills**:
- Controller clones skill repositories (Git context)
- Discovers SKILL.md files
- Injects `skills.paths` into OpenCode config
- All done at reconcile time, before Sandbox creation

**Plugins**:
- Controller generates `plugin-init` container in PodTemplate
- Runs `npm install` to shared `/plugins` volume
- Injects plugin paths into OpenCode config

#### f) Git Auto-Sync

**HotReload**:
- Add `git-sync` sidecar container to PodTemplate
- Periodically pulls Git repository
- Updates workspace in-place

**Rollout**:
- Controller monitors remote Git refs (current logic)
- When changes detected, updates Sandbox's PodTemplate
- Agent-Sandbox recreates Pod with new content
- Note: This is Pod recreation, not rolling update (see limitations in Part 2)

#### g) Task State Machine

KubeOpenCode's Task phases (Pending → Queued → Running → Completed/Failed) are maintained by KubeOpenCode controller, not the runtime.

**Implementation**:
- Controller creates SandboxClaim when Task enters Running phase
- Controller watches Sandbox/Pod status to update Task phase
- All queueing logic (maxConcurrentTasks, Quota) is handled before SandboxClaim creation

This is unchanged from current architecture — Task state machine is controller-level, not runtime-level.

#### h) Connection-Aware Standby

KubeOpenCode's standby uses annotation heartbeat to detect active connections.

**Implementation**:
- Controller continues to monitor connection annotations (unchanged)
- When idle timeout expires and no active connections, set `Sandbox.spec.operatingMode=Suspended`
- When new Task arrives on suspended Agent, set `Sandbox.spec.operatingMode=Running`

The standby logic lives in KubeOpenCode controller. Agent-Sandbox provides the primitive (`OperatingMode`).

#### i) Task Timeout

KubeOpenCode supports relative timeout from Task start time.

**Implementation options**:
1. **Controller-managed**: Controller watches running Tasks, deletes SandboxClaim when timeout expires (current logic)
2. **Absolute time**: Set `SandboxClaim.spec.lifecycle.shutdownTime` to `startTime + timeout`

Both approaches work. Controller-managed is more flexible (can be stopped/extended).

#### j) Extra Ports

KubeOpenCode exposes additional ports on Agent Service.

**Implementation**:
- Define ports in `PodTemplate.spec.containers[*].ports`
- Agent-Sandbox's headless Service exposes all container ports automatically
- For ClusterIP behavior, KubeOpenCode controller can create an additional ClusterIP Service

#### k) OTel Integration

KubeOpenCode injects OpenTelemetry environment variables.

**Implementation**: Add OTel env vars to `PodTemplate.spec.containers[*].env`. For server-mode Deployments, use Downward API for Pod name. For Sandbox (single Pod), Pod name is known at creation time.

#### l) Context Hash Rollout Trigger

KubeOpenCode uses ConfigMap content hash to trigger Pod restart when contexts change.

**Implementation**: Add context hash as `PodTemplate.metadata.annotations`. Agent-Sandbox updates Pod metadata when Sandbox spec changes, triggering Pod reconciliation. While not a rolling update, it achieves the same goal (Pod restart with new content).

### 1.3 Agent-Sandbox Capabilities Not in KubeOpenCode

Adopting Agent-Sandbox would bring new capabilities:

#### a) WarmPool for Fast Startup

SandboxWarmPool pre-creates Sandboxes from a template. When a SandboxClaim is created, it can adopt a ready Sandbox instead of creating from scratch.

**Benefit for KubeOpenCode**: Task cold start time reduced from seconds to sub-second (Pod already running, just needs adoption).

#### b) Automatic NetworkPolicy

SandboxTemplate can automatically generate a default-deny NetworkPolicy with customizable ingress/egress rules.

**Benefit for KubeOpenCode**: Enhanced security isolation out of the box, without manual NetworkPolicy configuration.

#### c) Declarative Env Injection

SandboxClaim supports injecting environment variables into specific containers:
```yaml
spec:
  env:
    - name: API_KEY
      value: secret123
      containerName: worker
```

**Benefit for KubeOpenCode**: More granular env var control than current global `extraEnv`.

#### d) HPA for WarmPool

SandboxWarmPool supports `scale` subresource, enabling HorizontalPodAutoscaler integration.

**Benefit for KubeOpenCode**: Auto-scaling warm pool size based on demand.

---

## Part 2: Features Not Implementable with Agent-Sandbox

After thorough analysis, only **three fundamental limitations** exist that cannot be worked around through adapter layers or PodTemplate generation:

### 2.1 Agent Mode: Multi-Task Shared Server Architecture

**KubeOpenCode Design:**
```
Agent (Deployment + ClusterIP Service)
  │
  ├── Task 1: lightweight Pod (~25MB) ──► opencode run --attach http://agent:4096
  ├── Task 2: lightweight Pod (~25MB) ──► opencode run --attach http://agent:4096
  └── Task 3: lightweight Pod (~25MB) ──► opencode run --attach http://agent:4096
```

**Why It Matters:**
- **Resource efficiency**: Task Pods use ~25MB attachImage vs ~1GB devbox image
- **Session persistence**: Agent server maintains SQLite DB across Tasks (conversation history, files)
- **Fast startup**: No image pull for Task Pods (attachImage is tiny)
- **Shared state**: Multiple Tasks share the same workspace and session

**Initial Analysis:**

Sandbox is a **single-Pod abstraction** with no client-server separation:
- One Sandbox = One Pod
- SandboxClaim creates a new Sandbox (Pod) per claim
- No native concept of lightweight clients connecting to a shared server

If we naively map both Agent and Task to Sandbox abstractions:
```
Agent → Sandbox (runs opencode serve)
Task → SandboxClaim (would create a NEW Sandbox, not attach to existing)
```
This fails because each SandboxClaim creates a new Pod.

**Revised Analysis: Valid Workaround**

The key insight is that **Task Pods do not need to be Sandboxes**. They can be regular Pods that connect to the Sandbox Agent via its Service:

```
Agent (Sandbox) ──► Pod + headless Service
  │
  ├── Task 1: regular lightweight Pod ──► opencode run --attach http://sandbox-agent:4096
  ├── Task 2: regular lightweight Pod ──► opencode run --attach http://sandbox-agent:4096
  └── Task 3: regular lightweight Pod ──► opencode run --attach http://sandbox-agent:4096
```

**Why this works:**
1. Agent-Sandbox creates a headless Service (`ClusterIP: None`) for each Sandbox
2. For a single Pod, headless Service DNS resolves to the Pod IP — functionally equivalent to ClusterIP for cluster-internal access
3. Task Pods (created by KubeOpenCode controller as regular Pods) can connect to `http://{sandbox-name}.{namespace}.svc.cluster.local:{port}`
4. The attach protocol (HTTP/WebSocket to opencode server) is unchanged

**Implementation changes required:**
- Replace Agent Deployment with Sandbox CRD
- Task controller creates regular Pods (not SandboxClaims) for agentRef Tasks
- Task controller manages Sandbox lifecycle (resume from `OperatingMode=Suspended` before creating Task Pod)
- Service discovery uses Sandbox's headless Service name instead of Agent's ClusterIP Service name

**Limitations of this workaround:**
While the attach model itself works, the Agent (now a Sandbox) still suffers from:
- **Part 2.2**: Sandbox doesn't auto-recreate Failed Pods (unlike Deployment ReplicaSet)
- **Part 2.3**: No rolling update for Agent config changes

**Impact**: MEDIUM-HIGH — The attach model is implementable, but Agent reliability is reduced compared to Deployment.

### 2.2 Deployment-Grade Reliability: Automatic Pod Recreation

**KubeOpenCode Design:**

Agent uses a Deployment with ReplicaSet:
- Pod crashes → ReplicaSet automatically creates new Pod
- Node failure → Pod rescheduled to new node
- Image update → Rolling update (old Pod stays until new Pod is ready)

**Agent-Sandbox Behavior:**

Sandbox controller manages a single Pod:
- If Pod is deleted → Controller creates new Pod ✓
- If Pod enters Failed state → **Pod is NOT automatically recreated**
  - Sandbox controller's `reconcilePod()` checks if Pod exists
  - If Pod exists (even if Failed), it updates metadata only
  - No logic to delete Failed Pod and create new one
- If node fails → Pod is lost, Sandbox shows Failed condition

**Evidence:**
- `controllers/sandbox_controller.go:623-699` — `reconcilePod()` handles existing Pod by updating metadata, not checking Pod phase
- `controllers/sandbox_controller.go:387-410` — `computeFinishedCondition()` marks Sandbox as Finished when Pod succeeds/fails, but does not trigger recreation

**Impact**: MEDIUM — For long-running Agents (server mode), this reduces reliability. If the OpenCode server crashes, the Sandbox stays in Failed state until manually intervened.

**Mitigation**: Add a controller-side watch that deletes Failed Sandbox Pods to trigger recreation. Or use a liveness probe with `restartPolicy: Always` (but Sandbox PodTemplate uses `RestartPolicy` from template, which may not be Always).

### 2.3 Rolling Update for Zero-Downtime Configuration Changes

**KubeOpenCode Design:**

Deployment supports rolling updates:
1. New Pod created with updated config
2. New Pod passes readiness probe
3. Old Pod terminated
4. Zero downtime during update

**Agent-Sandbox Behavior:**

Sandbox updates PodTemplate by:
1. Patching existing Pod's metadata (labels/annotations)
2. No automatic Pod recreation on spec change

To update Pod spec (e.g., new image, new env vars):
1. Must delete Sandbox or manually delete Pod
2. New Pod created with new spec
3. **Service interruption** between old Pod termination and new Pod readiness

**Impact**: MEDIUM — For server-mode Agents, updating configuration (new skills, plugin versions, context changes) causes service interruption. Users lose active connections.

**Mitigation**: This only matters for server-mode Agents. For ephemeral Tasks (templateRef), Pod recreation is expected behavior.

---

## Summary

### Implementable Features (via direct mapping or adapter layer)

| Feature | Implementation Effort | Notes |
|---------|----------------------|-------|
| Template mode (standalone Tasks) | Low | Direct mapping to SandboxClaim |
| Pod lifecycle management | Low | Agent-Sandbox handles this natively |
| Persistence (PVC) | Low | VolumeClaimTemplates |
| Suspend/Resume | Low | OperatingMode |
| PodSpec advanced config | Low | Direct passthrough |
| Multi-container init chain | Medium | Generate into PodTemplate |
| Context processing | Medium | Controller generates ConfigMap + PodTemplate mounts |
| Credentials | Low | Translate to Secret volumes/env |
| OpenCode config injection | Medium | Controller merges config, injects via env/ConfigMap |
| Skills/Plugins | Medium | Controller clones/installs, injects paths |
| Git auto-sync (HotReload) | Medium | Add git-sync sidecar to PodTemplate |
| Git auto-sync (Rollout) | Medium | Update PodTemplate triggers recreation |
| Task state machine | Low | Controller-level, unchanged |
| Connection-aware standby | Low | Controller sets OperatingMode |
| Task timeout | Low | Controller-managed or shutdownTime |
| Extra ports | Low | PodTemplate ports + optional Service |
| OTel integration | Low | PodTemplate env vars |
| Context hash rollout | Low | PodTemplate annotation |

### Non-Implementable Features (fundamental limitations)

| Feature | Why Not Possible | Impact | Workaround |
|---------|-----------------|--------|------------|
| Agent mode (shared server + lightweight clients) | Sandbox is single-Pod; **Workaround: Task as regular Pod + Sandbox as Agent works** | MEDIUM-HIGH | Use regular Pod for Task, Sandbox only for Agent |
| Automatic Pod recreation on failure | Sandbox controller doesn't recreate Failed Pods | MEDIUM | Controller-side watch to delete Failed Pods |
| Rolling update (zero-downtime config changes) | Sandbox updates don't support rolling replacement | MEDIUM | Manual coordination; only affects long-running Agents |

### New Capabilities Gained

| Feature | Benefit |
|---------|---------|
| WarmPool | Sub-second Task startup |
| Automatic NetworkPolicy | Security isolation out-of-the-box |
| Declarative env injection | Granular per-container env control |
| HPA for WarmPool | Auto-scaling pre-warmed Sandboxes |
| Rich lifecycle controls | shutdownTime, TTL, shutdownPolicy |

---

## Consequences

The consequences vary by adoption option:

### Option 1 (Template-only) Consequences

**Positive:**
- Minimal risk — Agent mode preserved unchanged
- Template Tasks gain WarmPool acceleration without affecting existing workflows
- Gradual adoption possible

**Negative:**
- Two runtime models in codebase (Deployment for Agent, Sandbox for Task)
- Agent does not benefit from Sandbox lifecycle features
- Additional complexity in controller logic

### Option 2 (Remove AgentRef) Consequences

**Positive:**
- Eliminates the primary architectural blocker (Part 2.1)
- Unified Task runtime: every Task is a Sandbox
- All Tasks benefit from WarmPool, NetworkPolicy, lifecycle controls
- Simpler controller: no attach URL management, no Agent/Task coordination

**Negative:**
- **Resource cost**: Every Task pulls full executor image (~1GB vs ~25MB attach image)
- **No session sharing**: Each Task starts fresh; no shared workspace or conversation history
- **Breaking change**: Existing `agentRef` users must migrate to `templateRef`
- **Queue model changes**: `maxConcurrentTasks` moves from Agent-level to global/namespace-level
- Agent (for interactive use) still suffers from Deployment reliability issues (Parts 2.2, 2.3)

**Mitigations available:**
- WarmPool reduces startup time to sub-second
- PersistentVolume can maintain workspace state across Tasks
- Global quota can replace per-Agent concurrency limits

### Option 3 (Full Adoption) Consequences

**Positive:**
- Completely unified runtime model
- All features benefit from Agent-Sandbox capabilities

**Negative:**
- All cons from Option 2, plus:
  - Loss of Deployment reliability for interactive Agents (Parts 2.2, 2.3)
  - Breaking change for Agent's interactive use case
  - No rolling update for Agent config changes

### General (All Options)

**Positive:**
1. **Reduced controller complexity** for Pod lifecycle management
2. **WarmPool acceleration** where Sandboxes are used
3. **Security enhancement** via automatic NetworkPolicy
4. **Richer lifecycle controls** (shutdownTime, TTL, shutdownPolicy)

**Negative:**
1. **Adapter layer maintenance**: Context processing, credentials, config injection must be reimplemented as PodTemplate generators
2. **Headless Service**: May require additional ClusterIP Service for some use cases

---

## Long-term Strategic Benefits of Agent-Sandbox

Beyond the immediate feature mappings, adopting Agent-Sandbox provides strategic advantages that compound over time:

### 1. Alignment with Kubernetes Ecosystem Standards

Agent-Sandbox is a **kubernetes-sigs** project under SIG Apps, developed by the same community that maintains Deployments, StatefulSets, and other core workload APIs.

**Benefits:**
- **API stability**: Follows Kubernetes API conventions (conditions, status, metadata) — no custom abstractions to maintain
- **Tooling compatibility**: Works with standard `kubectl`, Kubernetes dashboards, GitOps (ArgoCD, Flux) without special handling
- **Community trust**: Being part of kubernetes-sigs means governance, security reviews, and long-term maintenance commitment
- **Future-proofing**: As Kubernetes evolves (e.g., sidecar containers, in-place pod resize), Agent-Sandbox will adopt these features natively

### 2. Continuous Feature Evolution

As an active open-source project (2.7K+ stars, GKE integration, Python/Go SDKs), Agent-Sandbox is rapidly evolving:

**Already on the roadmap or in development:**
- **Multi-runtime support**: Native gVisor, Kata Containers integration for stronger isolation
- **Enhanced NetworkPolicy**: More granular egress rules, DNS policy integration
- **Cross-cluster Sandboxes**: Sandbox migration across clusters for disaster recovery
- **Observability integration**: Built-in Prometheus metrics, OpenTelemetry tracing
- **Cost optimization**: Spot instance integration, hibernation for long-idle Sandboxes

**Benefit for KubeOpenCode**: By building on Agent-Sandbox, we automatically inherit these capabilities without implementing them ourselves.

### 3. Reduced Controller Maintenance Burden

KubeOpenCode's current controller manages:
- Pod creation, deletion, status monitoring
- PVC lifecycle (create, bind, resize, delete)
- Service creation and endpoint management
- Manual NetworkPolicy configuration
- Custom standby/suspend logic via Deployment replicas

**With Agent-Sandbox:**
- All Pod/PVC/Service lifecycle is handled by the Sandbox controller
- NetworkPolicy is auto-generated from SandboxTemplate
- Suspend/resume is native via `OperatingMode`
- **Result**: KubeOpenCode controller can focus on higher-level concerns (context processing, Task queueing, OpenCode integration) rather than reinventing Kubernetes workload primitives

### 4. Security and Compliance Out-of-the-Box

Agent-Sandbox provides security features that would require significant custom development in KubeOpenCode:

- **Default-deny NetworkPolicy**: Every SandboxTemplate automatically generates a NetworkPolicy — no manual YAML needed
- **Pod security standards**: Sandbox controller enforces security contexts, non-root containers, read-only root filesystems
- **Runtime isolation**: Future support for gVisor/Kata provides kernel-level isolation between AI workloads
- **Audit logging**: Sandbox lifecycle events (creation, suspension, deletion) are Kubernetes events, integrated with cluster audit systems

**Compliance benefit**: For enterprise users, using a kubernetes-sigs project simplifies security reviews compared to custom controllers.

### 5. Performance and Cost Optimizations

**WarmPool**: Sub-second Task startup from pre-warmed Sandboxes. This is not just a convenience — it enables:
- **Event-driven scaling**: Rapid response to incoming Tasks without cold-start latency
- **Cost savings**: Sandboxes can be suspended (not deleted) during idle periods, preserving state while freeing compute resources
- **HPA integration**: WarmPool auto-scaling means the right number of pre-warmed instances without over-provisioning

**Resource efficiency**: Sandbox's fine-grained lifecycle controls (`shutdownTime`, `shutdownPolicy`, `TTLSecondsAfterFinished`) reduce resource waste from orphaned Pods.

### 6. Ecosystem Interoperability

Agent-Sandbox is designed as a **shared abstraction** for AI agent runtimes:

- **Notebook integration**: Jupyter, VS Code Server can run as Sandboxes, sharing the same warm pool infrastructure
- **Multi-tenant environments**: Sandbox's NetworkPolicy and resource isolation make it ideal for platforms serving multiple users
- **CI/CD integration**: SandboxClaim can be used in GitHub Actions runners, Tekton tasks, or Argo Workflows

**Benefit for KubeOpenCode**: Users can mix KubeOpenCode Tasks with other Sandbox-based tools in the same cluster, sharing warm pools and security policies.

### 7. Simplified Operations and Debugging

**Standard Kubernetes semantics**:
- `kubectl get sandboxes` — see all running workloads
- `kubectl describe sandbox my-agent` — standard events, conditions, status
- `kubectl logs sandbox/my-agent` — standard log access
- Sandbox conditions clearly show: `Ready`, `Suspended`, `Finished`, `Failed`

**vs. current KubeOpenCode debugging**:
- Agent status is spread across Deployment, Pod, Service, PVC resources
- Need to correlate multiple resources to understand state
- Custom conditions require `kubectl get agent` with custom output

### Summary: Why Invest in Agent-Sandbox Now

| Dimension | Current State (Custom) | Future State (Agent-Sandbox) |
|-----------|------------------------|------------------------------|
| **Pod lifecycle** | Custom controller logic | Community-maintained controller |
| **Security** | Manual NetworkPolicy YAML | Auto-generated per-template |
| **Startup time** | Seconds (image pull + init) | Sub-second (WarmPool adoption) |
| **Multi-runtime** | Not supported | gVisor/Kata on roadmap |
| **Cluster portability** | Custom CRDs | Standard kubernetes-sigs CRD |
| **Operations** | Multi-resource correlation | Single `Sandbox` resource |
| **Feature velocity** | KubeOpenCode team implements all | Inherit from SIG Apps community |

The upfront investment in an adapter layer pays dividends as Agent-Sandbox evolves, allowing KubeOpenCode to focus on its core value: **AI agent orchestration and OpenCode integration**, rather than low-level Kubernetes workload management.

---

## Recommendations

### Option 1: Adopt Agent-Sandbox for Template Mode Only (Recommended)

Use Agent-Sandbox **only** for `templateRef` Tasks. Keep the existing Deployment+Service model for Agents.

```
Agent (Server Mode)
  └── Deployment + Service + PVC (unchanged)
        └── Tasks connect via agentRef (unchanged)

templateRef Task
  └── SandboxClaim + SandboxTemplate (new)
        └── Uses Agent-Sandbox for Pod lifecycle
```

**Pros:**
- Preserves Agent mode (attach model) unchanged
- Template Tasks benefit from WarmPool acceleration
- Minimal risk — Agent mode is untouched
- Gradual adoption possible

**Cons:**
- Two runtime models in codebase
- Agent doesn't benefit from Sandbox lifecycle features

### Option 2: Sandbox Agent + Regular Task Pod (Best of Both Worlds)

Use Sandbox for Agent (replacing Deployment), but keep Task as regular lightweight Pods (not Sandboxes).

```
Agent (Sandbox)
  └── Sandbox + headless Service + PVC
        └── Runs opencode serve

Task (agentRef)
  └── Regular lightweight Pod (~25MB)
        └── opencode run --attach http://sandbox-agent:4096

templateRef Task
  └── SandboxClaim + SandboxTemplate
        └── Standalone Sandbox with full devbox
```

**Pros:**
- **Agent mode preserved**: Lightweight Task Pods still attach to Agent, maintaining resource efficiency and session sharing
- **Agent benefits from Sandbox features**: WarmPool, lifecycle controls, automatic NetworkPolicy
- **Unified Task runtime for templateRef**: Every templateRef Task is a Sandbox
- **No breaking change for agentRef users**: Attach model continues to work
- **Simpler than Option 1**: Agent is managed by Sandbox controller, not Deployment

**Cons:**
- Agent (Sandbox) suffers from Part 2.2 (no auto-recreate Failed Pods) and Part 2.3 (no rolling update)
- Controller must manage both Sandbox lifecycle (for Agent) and regular Pod creation (for agentRef Tasks)
- Two runtime abstractions (Sandbox for Agent/templateRef, regular Pod for agentRef Task)

**Mitigations:**
- Add controller-side watch to delete Failed Sandbox Pods for auto-recreate
- For rolling update: document that Agent config changes require brief downtime

### Option 3: Remove AgentRef — Task Always via TemplateRef

Eliminate the `agentRef` execution path entirely. All Tasks must specify `templateRef` (AgentTemplate). Agent CRD remains for interactive use only.

```
Agent (Interactive Mode only)
  └── Deployment + Service + PVC (unchanged)
        └── Users attach via Web terminal or CLI

Task (always standalone)
  └── SandboxClaim + SandboxTemplate
        └── Each Task is an independent Sandbox
```

**Pros:**
- Every Task is a Sandbox — unified runtime
- All Tasks benefit from WarmPool, NetworkPolicy, lifecycle controls
- No attach model complexity

**Cons:**
- **Resource cost**: Every Task pulls full executor image (~1GB vs ~25MB)
- **No session sharing**: Each Task starts fresh
- **Breaking change**: Existing `agentRef` users must migrate
- Agent (interactive) still uses Deployment, not benefiting from Sandbox features

### Option 4: Full Adoption — Everything via Sandbox

Replace both Agent and Task execution with Agent-Sandbox:

```
AgentTemplate ──► SandboxTemplate
       │
       ├── Agent (interactive) ──► Sandbox (long-running)
       │
       └── Task ──► SandboxClaim ──► Sandbox (ephemeral)
```

**Pros:**
- Completely unified runtime
- All workloads benefit from Agent-Sandbox capabilities

**Cons:**
- All cons from Option 3, plus:
  - Loss of Deployment reliability for interactive Agents (Parts 2.2, 2.3)
  - Breaking change for Agent's interactive use case

### Option 5: Status Quo

Continue using Deployment + Service + PVC for all workloads.

**Pros:**
- Zero risk
- Full control
- Agent mode preserved

**Cons:**
- No WarmPool acceleration
- Manual NetworkPolicy management
- Reinvent lifecycle management that Agent-Sandbox provides

---

## References

- [Agent-Sandbox Repository](https://github.com/kubernetes-sigs/agent-sandbox) — `kubernetes-sigs/agent-sandbox`
- KubeOpenCode `internal/controller/pod_builder.go` — Pod construction logic
- KubeOpenCode `internal/controller/server_builder.go` — Server Deployment construction
- KubeOpenCode `internal/controller/agent_controller.go` — Agent lifecycle management
- KubeOpenCode `internal/controller/task_controller.go` — Task state machine and queueing
- Agent-Sandbox `api/v1beta1/sandbox_types.go` — Sandbox CRD specification
- Agent-Sandbox `extensions/api/v1beta1/sandboxclaim_types.go` — SandboxClaim CRD specification
- Agent-Sandbox `extensions/api/v1beta1/sandboxtemplate_types.go` — SandboxTemplate CRD specification
- Agent-Sandbox `controllers/sandbox_controller.go` — Sandbox reconciler (Pod/Service/PVC lifecycle)
- Agent-Sandbox `extensions/controllers/sandboxclaim_controller.go` — SandboxClaim reconciler
- Agent-Sandbox `extensions/controllers/sandboxwarmpool_controller.go` — WarmPool reconciler
