# KubeOpenCode Features

This document covers the key features of KubeOpenCode.

## Flexible Context System

Tasks and Agents use inline **ContextItem** to provide additional context to AI agents.

### Context Types

- **Text**: Inline text content
- **ConfigMap**: Content from ConfigMap
- **Git**: Content from Git repository
- **Runtime**: KubeOpenCode platform awareness system prompt
- **URL**: Content fetched from remote HTTP/HTTPS URL

### Example

```yaml
contexts:
  - type: Text
    text: |
      # Rules for AI Agent
      Always use signed commits...
  - type: ConfigMap
    configMap:
      name: my-scripts
    mountPath: .scripts
    fileMode: 493  # 0755 in decimal
  - type: Git
    git:
      repository: https://github.com/org/repo.git
      ref: main
    mountPath: source-code
  - name: private-repo
    type: Git
    git:
      repository: https://github.com/org/private-repo.git
      ref: main
      secretRef:
        name: github-git-credentials  # Secret with username + password (PAT)
    mountPath: private-source
  - type: URL
    url:
      source: https://api.example.com/openapi.yaml
    mountPath: specs/openapi.yaml
```

### Content Aggregation

Contexts without `mountPath` are written to `.kubeopencode/context.md` with XML tags. OpenCode loads this via `OPENCODE_CONFIG_CONTENT`, preserving any existing `AGENTS.md` in the repository.

## Agent Configuration

Agent centralizes execution environment configuration:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: default
spec:
  profile: "Default development agent with org standards and GitHub access"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent

  # Default contexts for all tasks (inline ContextItems)
  contexts:
    - type: Text
      text: |
        # Organization Standards
        - Use signed commits
        - Follow Go conventions

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
```

## Custom CA Certificates

When accessing private Git servers or internal HTTPS services that use self-signed or private CA certificates, configure `caBundle` on the Agent to mount custom CA certificates into all containers.

### ConfigMap Example (trust-manager Compatible)

If you use [cert-manager trust-manager](https://cert-manager.io/docs/trust/trust-manager/), it can automatically populate a ConfigMap with your organization's CA bundle. KubeOpenCode's default key (`ca-bundle.crt`) matches trust-manager's convention.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: internal-agent
spec:
  profile: "Agent with custom CA for internal services"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  caBundle:
    configMapRef:
      name: custom-ca-bundle       # ConfigMap containing the CA certificate
      key: ca-bundle.crt           # Optional, defaults to "ca-bundle.crt"
```

### Secret Example

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: internal-agent
spec:
  profile: "Agent with custom CA from Secret"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  caBundle:
    secretRef:
      name: custom-ca-secret       # Secret containing the CA certificate
      key: ca.crt                  # Optional, defaults to "ca.crt"
```

### How It Works

- The CA certificate is mounted at `/etc/ssl/certs/custom-ca/tls.crt` in **all** containers (init containers and the worker container)
- The `CUSTOM_CA_CERT_PATH` environment variable is set in all containers
- **git-init**: Concatenates the custom CA with system CAs and sets `GIT_SSL_CAINFO` so `git clone` trusts the private server
- **url-fetch**: Appends the custom CA to Go's x509 system certificate pool for HTTPS URL fetching

This is the recommended approach for private HTTPS servers. Avoid disabling TLS verification (`InsecureSkipTLSVerify`) in favor of proper CA bundle configuration.

### Multiple CA Certificates

The `caBundle` field accepts a single ConfigMap or Secret reference, but PEM format supports multiple certificates in one file. To trust multiple CAs, concatenate them into a single bundle:

```bash
# Combine multiple CA certificates into one PEM bundle
cat internal-ca.crt partner-ca.crt > combined-ca-bundle.crt
kubectl create configmap custom-ca-bundle --from-file=ca-bundle.crt=combined-ca-bundle.crt
```

If you use [cert-manager trust-manager](https://cert-manager.io/docs/trust/trust-manager/), it handles multi-source aggregation automatically:

```yaml
apiVersion: trust.cert-manager.io/v1alpha1
kind: Bundle
metadata:
  name: custom-ca-bundle
spec:
  sources:
    - useDefaultCAs: true          # Include public CAs
    - secret:
        name: internal-ca
        key: ca.crt
    - configMap:
        name: partner-ca
        key: ca-bundle.crt
  target:
    configMap:
      key: ca-bundle.crt           # Matches KubeOpenCode's default key
```

> **Note**: `git-init` automatically concatenates the custom CA bundle with the container's system CAs, so public HTTPS (e.g., github.com) continues working even when `caBundle` is configured. You do not need to include public CAs in your bundle unless you want to explicitly control the full trust chain.

## HTTP/HTTPS Proxy Configuration

Enterprise networks often require all outbound traffic to pass through a corporate proxy server. KubeOpenCode supports proxy configuration at both the Agent level and the cluster level via `KubeOpenCodeConfig`.

### Agent-Level Proxy

Configure proxy settings directly on an Agent. These settings are injected as environment variables into all init containers and the worker container.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: enterprise-agent
spec:
  profile: "Agent with corporate proxy configuration"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  proxy:
    httpProxy: "http://proxy.corp.example.com:8080"
    httpsProxy: "http://proxy.corp.example.com:8080"
    noProxy: "localhost,127.0.0.1,10.0.0.0/8,.corp.example.com"
```

### Cluster-Level Proxy

For organizations where all agents should use the same proxy, configure it once in `KubeOpenCodeConfig`:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeConfig
metadata:
  name: cluster
spec:
  proxy:
    httpProxy: "http://proxy.corp.example.com:8080"
    httpsProxy: "http://proxy.corp.example.com:8080"
    noProxy: "localhost,127.0.0.1,10.0.0.0/8,.corp.example.com"
```

### How It Works

- Both uppercase and lowercase environment variables are set: `HTTP_PROXY`/`http_proxy`, `HTTPS_PROXY`/`https_proxy`, `NO_PROXY`/`no_proxy`
- The `.svc` and `.cluster.local` suffixes are always appended automatically to `noProxy` to prevent proxying in-cluster traffic
- **Agent-level proxy overrides cluster-level proxy**: If an Agent has `proxy` configured, it takes precedence over the `KubeOpenCodeConfig` proxy settings
- Proxy environment variables are injected into all containers (init containers and the worker container)

## Private Registry Authentication

When using container images from private registries (e.g., Harbor, AWS ECR, GCR), configure `imagePullSecrets` on the Agent to provide registry authentication credentials.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: private-registry-agent
spec:
  profile: "Agent using images from private registries"
  agentImage: registry.corp.example.com/kubeopencode/agent-opencode:latest
  executorImage: registry.corp.example.com/kubeopencode/agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  imagePullSecrets:
    - name: harbor-registry-secret
    - name: gcr-secret
```

### Prerequisites

1. The referenced Secrets must exist in the **same namespace** as the Agent
2. Secrets must be of type `kubernetes.io/dockerconfigjson`

Create registry credentials:

```bash
kubectl create secret docker-registry harbor-registry-secret \
  --docker-server=registry.corp.example.com \
  --docker-username=myuser \
  --docker-password=mypassword \
  -n kubeopencode-system
```

The `imagePullSecrets` are added to the Pod spec of all generated Pods, enabling Kubernetes to authenticate when pulling `agentImage`, `executorImage`, or `attachImage` from private registries.

## Pod Security

KubeOpenCode applies a restricted security context by default to all agent containers, following the Kubernetes Pod Security Standards (Restricted profile).

### Default Security Context

When no `securityContext` is specified in `podSpec`, the controller applies these defaults to all containers (init containers and the worker container):

- `allowPrivilegeEscalation: false`
- `capabilities: drop: ["ALL"]`
- `seccompProfile: type: RuntimeDefault`

These defaults align with the Kubernetes Restricted Pod Security Standard and are suitable for most workloads.

### Custom Container Security Context

Override the default security context for tighter or workload-specific settings using `podSpec.securityContext`:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: hardened-agent
spec:
  profile: "Security-hardened agent with strict container settings"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  podSpec:
    securityContext:
      runAsNonRoot: true
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
          - ALL
      seccompProfile:
        type: RuntimeDefault
```

> **Note**: When using `readOnlyRootFilesystem: true`, ensure the agent image supports it. You may need to use `emptyDir` volumes for writable paths (e.g., `/tmp`, `/home/agent`).

### Pod-Level Security Context

Use `podSpec.podSecurityContext` to configure security attributes that apply to the entire Pod (all containers):

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: uid-agent
spec:
  profile: "Agent running as specific user and group"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  podSpec:
    podSecurityContext:
      runAsUser: 1000
      runAsGroup: 1000
      fsGroup: 1000
```

`podSecurityContext` is useful for:
- Enforcing a specific UID/GID for all containers
- Setting `fsGroup` for shared volume permissions
- Meeting namespace-level Pod Security Admission requirements

## OpenCode Configuration

The `config` field allows you to provide OpenCode configuration as an inline JSON string:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: opencode-agent
spec:
  profile: "OpenCode agent with custom model configuration"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  config: |
    {
      "$schema": "https://opencode.ai/config.json",
      "model": "google/gemini-2.5-pro",
      "small_model": "google/gemini-2.5-flash"
    }
```

The configuration is written to `/tools/opencode.json` and the `OPENCODE_CONFIG` environment variable is set automatically. See [OpenCode configuration schema](https://opencode.ai/config.json) for available options.

## Multi-AI Support

Use different Agents with different executorImages for various use cases:

```yaml
# Standard OpenCode agent with devbox
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: opencode-devbox
spec:
  profile: "Standard OpenCode agent with devbox environment"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
---
# Task using specific agent
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: task-with-opencode
spec:
  agentRef:
    name: opencode-devbox
  description: "Update dependencies and create a PR"
```

## Task Stop

Stop a running task using the stop annotation:

```bash
kubectl annotate task my-task kubeopencode.io/stop=true
```

When this annotation is detected:
- The controller deletes the Pod (with graceful termination period)
- Task status is set to `Completed` with a `Stopped` condition
- The `Stopped` condition has reason `UserStopped`

**Note:** Logs are lost when a Task is stopped. For log persistence, use an external log aggregation system.

## Concurrency Control

Limit concurrent tasks per Agent when using rate-limited AI services:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: rate-limited-agent
spec:
  profile: "Rate-limited agent for API-quota-constrained backends"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  maxConcurrentTasks: 3  # Only 3 Tasks can run at once
```

When the limit is reached:
- New Tasks enter `Queued` phase instead of `Running`
- Queued Tasks automatically transition to `Running` when capacity becomes available
- Tasks are processed in approximate FIFO order

## Quota (Rate Limiting)

In addition to `maxConcurrentTasks` (which limits simultaneous running Tasks), you can configure `quota` to limit the rate at which Tasks can start using a sliding time window:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: rate-limited-agent
spec:
  profile: "Rate-limited agent with sliding window quota"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  quota:
    maxTaskStarts: 10     # Maximum 10 task starts
    windowSeconds: 3600   # Per hour (sliding window)
```

### Quota vs MaxConcurrentTasks

| Feature | `maxConcurrentTasks` | `quota` |
|---------|----------------------|---------|
| What it limits | Simultaneous running Tasks | Rate of new Task starts |
| Time component | No (instant check) | Yes (sliding window) |
| Queued Reason | `AgentAtCapacity` | `QuotaExceeded` |
| Use case | Limit resource usage | API rate limiting |

Both can be used together for comprehensive control. When quota is exceeded, new Tasks enter `Queued` phase with reason `QuotaExceeded`.

## Pod Configuration

Configure advanced Pod settings using `podSpec`:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: advanced-agent
spec:
  profile: "Advanced agent with gVisor isolation and GPU scheduling"
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  podSpec:
    # Labels for NetworkPolicy, monitoring, etc.
    labels:
      network-policy: agent-restricted
    # Enhanced isolation with gVisor or Kata
    runtimeClassName: gvisor
    # Scheduling configuration
    scheduling:
      nodeSelector:
        node-type: ai-workload
      tolerations:
        - key: "dedicated"
          operator: "Equal"
          value: "ai-workload"
          effect: "NoSchedule"
```

## Interactive Agent Sessions

For Server-mode Agents, you can interact with running agents using these tools:

- **Web Terminal**: Access the agent's OpenCode TUI directly from the KubeOpenCode dashboard
- **`kubeoc agent attach`**: Connect to the agent from the CLI
- **`opencode attach`**: Direct OpenCode TUI connection via port-forward

See [Getting Started](getting-started.md) for detailed usage instructions.

## Next Steps

- [Getting Started](getting-started.md) - Installation and basic usage
- [Agent Images](agent-images.md) - Build custom agent images
- [Security](security.md) - RBAC, credential management, and best practices
- [Architecture](architecture.md) - System design and API reference
