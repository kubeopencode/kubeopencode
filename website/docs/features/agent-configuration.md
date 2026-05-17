# Agent Configuration

Agent centralizes execution environment configuration:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: default
spec:
  profile: "Default development agent with org standards and GitHub access"
  agentImage: ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
  attachImage: ghcr.io/kubeopencode/kubeopencode-agent-attach:latest
  workspaceDir: /workspace
  command: ["opencode", "serve"]
  port: 4096
  serviceAccountName: kubeopencode-agent

  # Additional ports (DinD, VS Code, etc.)
  extraPorts:
    - name: docker
      port: 2375
      targetPort: 2375
      protocol: TCP

  # Default contexts for all tasks (inline ContextItems)
  contexts:
    - type: Text
      text: |
        # Organization Standards
        - Use signed commits
        - Follow Go conventions

  # Skills from external Git repos
  skills:
    - name: team-skills
      git:
        repository: https://github.com/my-org/ai-skills.git
        ref: main
        path: skills/

  # OpenCode plugins (installed via npm at pod startup)
  plugins:
    - name: cc-safety-net
    - name: "@nicholasgriffintn/opencode-plugin-otel"
      options:
        endpoint: "http://otel-collector:4318"

  # OpenCode configuration (inline YAML object)
  config:
    $schema: https://opencode.ai/config.json
    model: google/gemini-2.5-pro
    small_model: google/gemini-2.5-flash

  # Credentials (secrets as env vars or file mounts)
  credentials:
    - name: github-token
      secretRef:
        name: github-creds
        key: token
      env: GITHUB_TOKEN

    - name: ssh-key
      secretRef:
        name: ssh-keys
        key: id_rsa
      mountPath: /home/agent/.ssh/id_rsa
      fileMode: 0400

  # Custom CA certificates
  caBundle:
    configMapRef:
      name: corporate-ca-bundle
      key: ca-bundle.crt

  # HTTP/HTTPS proxy
  proxy:
    httpProxy: "http://proxy.corp.example.com:8080"
    httpsProxy: "http://proxy.corp.example.com:8080"
    noProxy: "localhost,127.0.0.1,.svc,.cluster.local"

  # Private registry authentication
  imagePullSecrets:
    - name: my-registry-secret

  # Task concurrency control
  maxConcurrentTasks: 3
  quota:
    maxTaskStarts: 20
    windowSeconds: 3600

  # Shareable terminal link
  share:
    enabled: true
    expiresAt: "2026-12-31T23:59:59Z"
    allowedIPs:
      - "10.0.0.0/8"

  # Pod-level customization
  podSpec:
    labels:
      team: platform
    runtimeClassName: sysbox
    securityContext:
      runAsNonRoot: true
      allowPrivilegeEscalation: false
    extraEnv:
      - name: NODE_OPTIONS
        value: "--max-old-space-size=4096"
    extraVolumes:
      - name: docker-sock
        hostPath:
          path: /var/run/docker.sock
    extraVolumeMounts:
      - name: docker-sock
        mountPath: /var/run/docker.sock
```

## Field Reference

### Core Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `profile` | string | - | Brief human-readable summary (informational only) |
| `agentImage` | string | `ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest` | OpenCode init container image |
| `executorImage` | string | `ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest` | Main worker container image |
| `attachImage` | string | `ghcr.io/kubeopencode/kubeopencode-agent-attach:latest` | Lightweight image for Task Pods (agentRef mode) |
| `workspaceDir` | string | `/workspace` | Agent working directory |
| `command` | []string | `["opencode", "serve"]` | Command to run in the worker container |
| `port` | int32 | 4096 | OpenCode server port |
| `serviceAccountName` | string | - | Kubernetes ServiceAccount for the Agent pod |

### Context and Knowledge

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `contexts` | []ContextItem | - | Inline context definitions (Text, ConfigMap, Git, Runtime, URL). See [Context System](context-system.md) |
| `skills` | []SkillSource | - | External SKILL.md sources from Git repos. See [Skills](skills.md) |

### Configuration and Extensibility

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `plugins` | []PluginSpec | - | OpenCode plugins to install and load. See [Plugins](plugins.md) |
| `config` | *runtime.RawExtension | - | Inline OpenCode configuration (YAML/JSON object). See below |

### Security and Authentication

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `credentials` | []Credential | - | Secrets mounted as env vars or file mounts. See [Security](../security.md) |
| `caBundle` | *CABundleConfig | - | Custom CA certificates for TLS. See [Enterprise](enterprise.md) |
| `proxy` | *ProxyConfig | - | HTTP/HTTPS proxy settings. See [Enterprise](enterprise.md) |
| `imagePullSecrets` | []LocalObjectReference | - | Private registry authentication |

### Concurrency Control

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `maxConcurrentTasks` | *int32 | - | Maximum number of Tasks running simultaneously. See [Concurrency & Quota](concurrency-quota.md) |
| `quota` | *QuotaConfig | - | Rate limiting for Task starts. See [Concurrency & Quota](concurrency-quota.md) |

### Persistence and Lifecycle

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `persistence` | *PersistenceConfig | - | Session/workspace PVCs. See [Persistence](persistence.md) |
| `suspend` | bool | false | Scale Deployment to 0 replicas. See [Persistence](persistence.md) |
| `standby` | *StandbyConfig | - | Automatic suspend/resume lifecycle. See [Persistence](persistence.md) |

### Networking

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `extraPorts` | []ExtraPort | - | Additional Service/Deployment ports (DinD, VS Code, etc.) |
| `share` | *ShareConfig | - | Shareable terminal link. See [Share Link](share-link.md) |

### Pod Customization

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `podSpec` | *AgentPodSpec | - | Pod-level customization (security, scheduling, volumes, etc.). See [Pod Configuration](pod-configuration.md) |
| `templateRef` | *AgentTemplateReference | - | Inherit base config from an AgentTemplate. See [Agent Templates](agent-templates.md) |

## OpenCode Configuration

The `config` field allows you to provide OpenCode configuration as an inline YAML object:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: opencode-agent
spec:
  profile: "OpenCode agent with custom model configuration"
  agentImage: ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  config:
    $schema: https://opencode.ai/config.json
    model: anthropic/claude-sonnet-4-5
    small_model: anthropic/claude-haiku-4-5
```

The configuration is serialized to a config file inside the container and the `OPENCODE_CONFIG` environment variable is set automatically. See [OpenCode configuration schema](https://opencode.ai/config.json) for available options.

## Agent-Only Fields

The following fields can only be set on Agent (not on AgentTemplate):

| Field | Description |
|-------|-------------|
| `profile` | Human-readable agent summary |
| `port` | OpenCode server port |
| `persistence` | Session/workspace PVCs |
| `suspend` | Manual suspend flag |
| `standby` | Auto suspend/resume config |
| `share` | Shareable terminal link config |
| `templateRef` | Reference to AgentTemplate |

See [Agent Templates](agent-templates.md) for merge behavior when using `templateRef`.