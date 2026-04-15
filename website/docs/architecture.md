# Architecture & API Design

## System Overview

KubeOpenCode brings Agentic AI capabilities into the Kubernetes ecosystem. By leveraging Kubernetes, it enables AI agents to be deployed as services, run in isolated virtual environments, and integrate with enterprise management and governance frameworks.

### Core Goals

- Use Kubernetes CRDs to define Task and Agent resources
- Use Controller pattern to manage resource lifecycle
- Agents run as persistent Deployments; Tasks execute via lightweight attach Pods or ephemeral template Pods
- Seamless integration with Kubernetes ecosystem

### Key Advantages

- **Native Integration**: Works seamlessly with Helm, Kustomize, ArgoCD and other K8s tools
- **Declarative Management**: Use K8s resource definitions, supports GitOps
- **Infrastructure Reuse**: Logs, monitoring, auth/authz all leverage K8s capabilities
- **Simplified Operations**: Manage with standard K8s tools (kubectl, dashboard)
- **Batch Operations**: Use Helm/Kustomize to create multiple Tasks (Kubernetes-native approach)

### External Integrations

KubeOpenCode focuses on the core Task/Agent abstraction. For advanced features, integrate with external projects:

| Feature | Recommended Integration |
|---------|------------------------|
| Workflow orchestration | [Argo Workflows](https://argoproj.github.io/argo-workflows/) |
| Event-driven triggers | [Argo Events](https://argoproj.github.io/argo-events/) |
| Scheduled execution | [CronTask](features/crontask.md) (built-in) |

See the [kubeopencode/dogfooding](https://github.com/kubeopencode/dogfooding) repository for examples of GitHub webhook integration using Argo Events that creates KubeOpenCode Tasks.

---

## System Architecture

```mermaid
graph TB
    subgraph K8s["Kubernetes Cluster"]
        API["Kubernetes API Server<br/><i>CRDs · RBAC · Events</i>"]

        subgraph CP["Control Plane (kubeopencode-system)"]
            Controller["KubeOpenCode Controller<br/><i>Watch CRs · Reconcile · Manage Pods</i>"]
            Server["KubeOpenCode Server<br/><i>REST API · Embedded React UI</i>"]
        end

        subgraph DP["Data Plane"]
            Agent["Agent Deployment<br/><i>opencode serve (persistent)</i>"]
            AgentSvc["Agent Service<br/><i>ClusterIP</i>"]
            TaskPod1["Task Pod (agentRef)<br/><i>opencode run --attach</i>"]
            TaskPod2["Task Pod (templateRef)<br/><i>opencode run (standalone)</i>"]
        end

        subgraph Storage["Storage"]
            etcd["etcd<br/><i>Task · Agent · CronTask CRs</i>"]
            PVC["PVCs<br/><i>Sessions · Workspace</i>"]
            CM["ConfigMaps / Secrets<br/><i>Context · Credentials</i>"]
        end
    end

    Users["Users<br/><i>kubectl · kubeoc CLI · Web UI</i>"]

    Users --> API
    Server --> API
    API --> Controller
    Controller --> Agent
    Controller --> AgentSvc
    Controller --> TaskPod1
    Controller --> TaskPod2
    TaskPod1 -- "--attach" --> AgentSvc
    AgentSvc --> Agent
    API --> etcd
    Agent --> PVC
    Agent --> CM
```

### Component Roles

| Component | Responsibility |
|-----------|----------------|
| **Controller** | Watches Task/Agent/CronTask CRs, creates Deployments/Pods/Services, manages lifecycle |
| **Server** | REST API + embedded React UI, proxies to Kubernetes API |
| **Agent Deployment** | Persistent `opencode serve` instance, handles multiple Tasks |
| **Task Pod (agentRef)** | Lightweight Pod that connects to Agent server via `--attach` |
| **Task Pod (templateRef)** | Standalone Pod with full environment, runs independently |

### Two-Container Pattern

Agent Deployments use a two-container pattern:

1. **Init Container** (`agentImage`): Copies OpenCode binary to `/tools` shared volume
2. **Worker Container** (`executorImage`): Runs `opencode serve` using `/tools/opencode`

### Image Resolution

| Field | Container | Default |
|-------|-----------|---------|
| `agentImage` | Init Container (OpenCode) | `ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest` |
| `executorImage` | Worker Container (Server) | `ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest` |
| `attachImage` | Task Pod (agentRef) | `ghcr.io/kubeopencode/kubeopencode-agent-attach:latest` |

### Task Execution Flow

```mermaid
flowchart LR
    Created["Task Created"] --> Check{agentRef or<br/>templateRef?}
    Check -- agentRef --> Attach["Create Pod<br/>opencode run --attach URL"]
    Check -- templateRef --> Standalone["Create Pod<br/>opencode run"]
    Attach --> Running["Phase: Running"]
    Standalone --> Running
    Running --> Done["Phase: Completed / Failed"]
```

---

## API Design

### Resource Overview

| Resource | Purpose | Stability |
|----------|---------|-----------|
| **Task** | Single task execution (primary API) | Stable |
| **CronTask** | Scheduled/recurring task execution | Stable |
| **Agent** | Running AI agent instance (Deployment + Service) | Stable |
| **AgentTemplate** | Reusable blueprint for Agents and ephemeral Tasks | Stable |
| **KubeOpenCodeConfig** | Cluster-scoped system-level configuration | Stable |
| **ContextItem** | Inline context for AI agents | Stable |

### Key Design Decisions

#### 1. Task as Primary API

Simple, focused API for single task execution. For batch operations, use Helm/Kustomize to create multiple Tasks.

#### 2. Agent (not KubeOpenCodeConfig)

- **Stable**: Independent of project name — won't change even if project renames
- **Semantic**: "Agent = AI + permissions + tools"

#### 3. No Batch/BatchRun

Kubernetes-native approach — use Helm, Kustomize, or other templating tools to create multiple Tasks. Reduces API complexity and leverages existing Kubernetes tooling.

#### 4. No Retry Mechanism

AI tasks are fundamentally different from traditional functions:

- **Non-deterministic output**: AI agents may produce different results on each run
- **Non-idempotent operations**: Tasks may perform actions (create PRs, modify files) that should not be repeated
- **Compound failures**: Retrying a partially completed task may cause duplicate operations

Pods are created with `RestartPolicy: Never`. If retry is needed, use [Argo Workflows](https://argoproj.github.io/argo-workflows/) or [Tekton Pipelines](https://tekton.dev/).

### Resource Hierarchy

```
Task (single task execution)
├── TaskSpec
│   ├── description: *string                (syntactic sugar for /workspace/task.md)
│   ├── contexts: []ContextItem             (inline context definitions)
│   ├── agentRef: *AgentReference           (Agent reference, same namespace)
│   └── templateRef: *AgentTemplateReference (AgentTemplate reference, alternative to agentRef)
└── TaskExecutionStatus
    ├── phase: TaskPhase
    ├── podName: string
    ├── startTime: Time
    ├── completionTime: Time
    └── conditions: []Condition

Agent (running AI agent instance — always creates Deployment + Service)
└── AgentSpec
    ├── profile: string             (brief human-readable summary)
    ├── agentImage: string           (OpenCode init container image)
    ├── executorImage: string        (Main worker container image)
    ├── attachImage: string          (lightweight image for --attach Pods)
    ├── workspaceDir: string         (default: "/workspace")
    ├── command: []string
    ├── port: int32                  (OpenCode server port, default: 4096)
    ├── persistence: *PersistenceConfig  (session/workspace PVCs)
    ├── suspend: bool                (scale Deployment to 0 replicas)
    ├── standby: *StandbyConfig      (automatic suspend/resume)
    ├── contexts: []ContextItem      (inline context definitions)
    ├── skills: []SkillSource        (external SKILL.md from Git repos)
    ├── plugins: []PluginSpec        (OpenCode plugins to load)
    ├── config: *runtime.RawExtension (inline OpenCode config, YAML object)
    ├── credentials: []Credential
    ├── caBundle: *CABundleConfig    (custom CA certificates for TLS)
    ├── proxy: *ProxyConfig          (HTTP/HTTPS proxy settings)
    ├── imagePullSecrets: []LocalObjectReference  (private registry auth)
    ├── podSpec: *AgentPodSpec
    ├── serviceAccountName: string
    ├── maxConcurrentTasks: *int32   (limit concurrent Tasks)
    ├── quota: *QuotaConfig          (rate limiting for Task starts)
    └── share: *ShareConfig          (shareable terminal link)

AgentTemplate (reusable blueprint for Agents and ephemeral Tasks)
└── AgentTemplateSpec
    ├── (shares most fields with AgentSpec)
    └── (except: profile, port, persistence, suspend, standby, templateRef)

CronTask (scheduled/recurring task execution)
└── CronTaskSpec
    ├── schedule: string             (cron expression)
    ├── timeZone: *string            (IANA timezone, default: UTC)
    ├── concurrencyPolicy: string    (Allow/Forbid/Replace, default: Forbid)
    ├── suspend: *bool
    ├── startingDeadlineSeconds: *int64
    ├── maxRetainedTasks: *int32     (default: 10)
    └── taskTemplate: TaskTemplateSpec

KubeOpenCodeConfig (system configuration, cluster-scoped singleton named "cluster")
└── KubeOpenCodeConfigSpec
    ├── systemImage: *SystemImageConfig
    ├── cleanup: *CleanupConfig
    └── proxy: *ProxyConfig
```

---

## Complete Type Definitions

```go
// Task represents a single task execution
type Task struct {
    Spec   TaskSpec
    Status TaskExecutionStatus
}

type TaskSpec struct {
    Description *string                 // Syntactic sugar for /workspace/task.md
    Contexts    []ContextItem           // Inline context definitions
    AgentRef    *AgentReference         // Agent reference (same namespace)
    TemplateRef *AgentTemplateReference // AgentTemplate reference (alternative to agentRef)
    // Exactly one of AgentRef or TemplateRef must be set
}

// AgentReference references an Agent in the same namespace
type AgentReference struct {
    Name string // Agent name (required)
}

// ContextItem defines inline context content
type ContextItem struct {
    Type      ContextType       // Text, ConfigMap, Git, Runtime, or URL
    MountPath string            // Empty = write to .kubeopencode/context.md (ignored for Runtime)
    FileMode  *int32            // Optional file permission mode (e.g., 0755 for executable)
    Text      string            // Content when Type is Text
    ConfigMap *ConfigMapContext // ConfigMap when Type is ConfigMap
    Git       *GitContext       // Git repo when Type is Git
    Runtime   *RuntimeContext   // Platform awareness when Type is Runtime
    URL       *URLContext       // Remote URL when Type is URL
}

type TaskExecutionStatus struct {
    Phase          TaskPhase
    PodName        string
    StartTime      *metav1.Time
    CompletionTime *metav1.Time
    Conditions     []metav1.Condition
}

type ContextType string
const (
    ContextTypeText      ContextType = "Text"
    ContextTypeConfigMap ContextType = "ConfigMap"
    ContextTypeGit       ContextType = "Git"
    ContextTypeRuntime   ContextType = "Runtime"
    ContextTypeURL       ContextType = "URL"
)

// Agent defines the AI agent configuration
type Agent struct {
    Spec AgentSpec
}

type AgentSpec struct {
    Profile            string
    AgentImage         string
    ExecutorImage      string
    AttachImage        string
    WorkspaceDir       string
    Command            []string
    Port               int32
    Persistence        *PersistenceConfig
    Suspend            bool
    Standby            *StandbyConfig
    Contexts           []ContextItem
    Skills             []SkillSource
    Config             *runtime.RawExtension
    Credentials        []Credential
    CABundle           *CABundleConfig
    Proxy              *ProxyConfig
    ImagePullSecrets   []corev1.LocalObjectReference
    PodSpec            *AgentPodSpec
    ServiceAccountName string
    MaxConcurrentTasks *int32
    Quota              *QuotaConfig
    Share              *ShareConfig
    TemplateRef        *AgentTemplateReference
}

// ShareConfig configures a shareable terminal link for an Agent
type ShareConfig struct {
    Enabled    bool       // Enable/disable the share link
    ExpiresAt  *Time      // Optional expiry time (link invalid after this)
    AllowedIPs []string   // Optional CIDR allowlist (empty = all IPs allowed)
    ReadOnly   bool       // View-only terminal (stdin dropped)
}

// ProxyConfig configures HTTP/HTTPS proxy for all containers
type ProxyConfig struct {
    HttpProxy  string // HTTP proxy URL (sets HTTP_PROXY and http_proxy)
    HttpsProxy string // HTTPS proxy URL (sets HTTPS_PROXY and https_proxy)
    NoProxy    string // Comma-separated bypass list (.svc,.cluster.local always appended)
}

// CABundleConfig configures custom CA certificates for TLS
type CABundleConfig struct {
    ConfigMapRef *CABundleReference // CA bundle from ConfigMap (default key: "ca-bundle.crt")
    SecretRef    *CABundleReference // CA bundle from Secret (default key: "ca.crt")
}

// KubeOpenCodeConfig defines system-level configuration
type KubeOpenCodeConfig struct {
    Spec KubeOpenCodeConfigSpec
}

type KubeOpenCodeConfigSpec struct {
    SystemImage *SystemImageConfig
    Cleanup     *CleanupConfig
    Proxy       *ProxyConfig
}

type CleanupConfig struct {
    TTLSecondsAfterFinished *int32 // TTL for cleaning up finished Tasks (nil = disabled)
    MaxRetainedTasks        *int32 // Max completed Tasks to retain per namespace (nil = unlimited)
}
```

---

## KubeOpenCodeConfig (System Configuration)

KubeOpenCodeConfig provides **cluster-wide** settings for container image configuration and Task cleanup policies.

> **Note**: KubeOpenCodeConfig is a **cluster-scoped singleton** resource. Following OpenShift convention, it must be named `cluster`.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeConfig
metadata:
  name: cluster  # Required singleton name
spec:
  # System image for internal components (git-init, context-init)
  systemImage:
    image: ghcr.io/kubeopencode/kubeopencode:latest
    imagePullPolicy: Always  # Always/Never/IfNotPresent (default: IfNotPresent)

  # Task cleanup (optional)
  cleanup:
    ttlSecondsAfterFinished: 3600  # Delete finished Tasks after 1 hour
    maxRetainedTasks: 100          # Keep at most 100 per namespace

  # Cluster-wide proxy (Agent-level proxy overrides this)
  proxy:
    httpProxy: "http://proxy.corp.example.com:8080"
    httpsProxy: "http://proxy.corp.example.com:8080"
    noProxy: "localhost,127.0.0.1,10.0.0.0/8"
```

| Field | Type | Description |
|-------|------|-------------|
| `systemImage.image` | string | System image for internal components (default: built-in) |
| `systemImage.imagePullPolicy` | string | Pull policy: Always/Never/IfNotPresent (default: IfNotPresent) |
| `cleanup.ttlSecondsAfterFinished` | *int32 | TTL for finished Tasks. nil = disabled |
| `cleanup.maxRetainedTasks` | *int32 | Max completed Tasks per namespace. nil = unlimited |
| `proxy` | *ProxyConfig | Cluster-wide proxy. See [Enterprise](features/enterprise.md#httphttps-proxy-configuration) |

**Task Cleanup behavior:**
- **TTL-based**: Tasks deleted after `ttlSecondsAfterFinished` seconds from completion
- **Retention-based**: Only the most recent `maxRetainedTasks` completed Tasks retained per namespace
- **Combined**: Both can be used together. TTL checked first, then retention count
- **Cascading deletion**: Deleting a Task automatically deletes its associated Pod and ConfigMap
- Cleanup is disabled by default

---

## Web UI & REST API

KubeOpenCode includes a web-based UI for managing Tasks, Agents, and CronTasks.

```mermaid
graph LR
    Users["Users"] --> UI["Embedded React UI<br/><i>TypeScript + React</i>"]
    Users --> CLI["kubeoc CLI"]
    UI --> API["REST API<br/><i>kubeopencode server</i>"]
    CLI --> K8sAPI["Kubernetes API"]
    API --> K8sAPI
    K8sAPI --> CRs["Tasks · Agents<br/>Pods · Namespaces"]
```

**Key Design:**
- Single server binary (`kubeopencode server` subcommand)
- React UI embedded in Go binary via `embed` package
- ServiceAccount token authentication (Kubernetes RBAC)
- No external dependencies (no database)

### REST API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/namespaces/{ns}/tasks` | List Tasks |
| GET | `/api/v1/namespaces/{ns}/tasks/{name}` | Get Task |
| POST | `/api/v1/namespaces/{ns}/tasks` | Create Task |
| DELETE | `/api/v1/namespaces/{ns}/tasks/{name}` | Delete Task |
| POST | `/api/v1/namespaces/{ns}/tasks/{name}/stop` | Stop Task |
| GET | `/api/v1/namespaces/{ns}/tasks/{name}/logs` | Stream logs (SSE) |
| GET | `/api/v1/agents` | List all Agents |
| GET | `/api/v1/namespaces/{ns}/agents` | List Agents in namespace |
| GET | `/api/v1/namespaces/{ns}/agents/{name}` | Get Agent details |
| POST | `/api/v1/namespaces/{ns}/agents/{name}/suspend` | Suspend Agent |
| POST | `/api/v1/namespaces/{ns}/agents/{name}/resume` | Resume Agent |
| GET | `/api/v1/namespaces/{ns}/crontasks` | List CronTasks |
| GET | `/api/v1/namespaces/{ns}/crontasks/{name}` | Get CronTask |
| POST | `/api/v1/namespaces/{ns}/crontasks/{name}/trigger` | Trigger CronTask |
| GET | `/api/v1/info` | Server info |
| GET | `/api/v1/namespaces` | List namespaces |

### Deployment

Enable the UI server in Helm:

```yaml
server:
  enabled: true
  replicas: 1
  service:
    type: ClusterIP
    port: 2746
```

Access via port-forward:

```bash
kubectl port-forward -n kubeopencode-system svc/kubeopencode-server 2746:2746
```

---

## kubectl Usage

```bash
# Task operations
kubectl apply -f task.yaml
kubectl get tasks -n kubeopencode-system
kubectl get task update-service-a -w
kubectl logs $(kubectl get task update-service-a -o jsonpath='{.status.podName}')
kubectl annotate task my-task kubeopencode.io/stop=true

# Agent operations
kubectl get agents -n kubeopencode-system
kubectl apply -f agent.yaml
kubectl get agent my-agent -o yaml

# CronTask operations
kubectl get crontasks -n kubeopencode-system
kubectl annotate crontask daily-scan kubeopencode.io/trigger=true

# Batch operations with Helm
helm template my-tasks ./chart | kubectl apply -f -
```

---

## Feature Reference

For detailed usage and configuration of each feature, see the [Features](features/index.md) section:

- [Live Agents](features/live-agents.md) — Persistent agents, interactive access, agent vs template tasks
- [Context System](features/context-system.md) — Text, ConfigMap, Git, Runtime, URL contexts
- [Agent Configuration](features/agent-configuration.md) — Credentials, OpenCode config
- [Agent Templates](features/agent-templates.md) — Reusable blueprints, merge behavior
- [Skills](features/skills.md) — External SKILL.md from Git repos
- [Plugins](features/plugins.md) — OpenCode plugins for deep agent customization
- [CronTask](features/crontask.md) — Scheduled execution, concurrency policy
- [Concurrency & Quota](features/concurrency-quota.md) — Task limits, rate limiting
- [Persistence & Lifecycle](features/persistence.md) — PVCs, suspend/resume, standby
- [Enterprise](features/enterprise.md) — Proxy, CA certificates, private registry
- [Pod Configuration](features/pod-configuration.md) — Security context, scheduling
