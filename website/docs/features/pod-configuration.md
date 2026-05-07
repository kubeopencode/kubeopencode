# Pod Configuration

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
  agentImage: ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
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
  agentImage: ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
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

## Advanced Pod Settings

Configure advanced Pod settings using `podSpec`:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: advanced-agent
spec:
  profile: "Advanced agent with gVisor isolation and GPU scheduling"
  agentImage: ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
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

## Extra Ports

Expose additional ports on the Agent's Service and Deployment using `extraPorts`. This is useful for [Docker-in-Docker](../use-cases/docker-in-docker.md) scenarios where containers inside the agent need to be accessible from outside — for example, web application UIs, VS Code server, or database ports.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: dev-agent
spec:
  profile: "Development agent with extra port exposure"
  agentImage: ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  port: 4096
  extraPorts:
    - name: webapp
      port: 3000
    - name: vscode
      port: 8080
    - name: postgres
      port: 5432
      protocol: TCP
```

Each extra port is added to both the Deployment's container ports and the Agent's ClusterIP Service. Access them via:

```bash
# Port-forward individual ports
kubectl port-forward svc/dev-agent 3000:3000 8080:8080

# Open in browser
# http://localhost:3000 (webapp)
# http://localhost:8080 (vscode)
```

`extraPorts` can also be defined on `AgentTemplate`. When an Agent references a template, the Agent's `extraPorts` replaces the template's list (same merge strategy as `credentials` and `contexts`).

## Extra Environment Variables

Inject custom environment variables into agent pod containers using `podSpec.extraEnv` (all containers)
or `podSpec.systemContainers` (per-container-type targeting).

### Global Extra Env (All Containers)

`podSpec.extraEnv` injects env vars into **every container** in the pod — all init containers and
the executor container. These are appended after controller-managed env vars, so they can override
controller defaults when necessary.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: corp-agent
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  podSpec:
    extraEnv:
      # Plain value
      - name: CORPORATE_REGISTRY
        value: registry.corp.example.com
      # From a Secret key
      - name: NPM_TOKEN
        valueFrom:
          secretKeyRef:
            name: npm-credentials
            key: token
      # From a ConfigMap key
      - name: ENVIRONMENT
        valueFrom:
          configMapKeyRef:
            name: cluster-config
            key: environment
```

### Per-Container-Type Overrides

`podSpec.systemContainers` provides fine-grained control over specific KubeOpenCode-managed
containers. Use this when you need different env vars or volume mounts on specific container types.

Available container targets:

| Field | Container | Role |
|-------|-----------|------|
| `openCodeInit` | `opencode-init` | Copies OpenCode binary from `agentImage` to `/tools` |
| `contextInit` | `context-init` | Copies ConfigMap content into workspace |
| `gitInit` | `git-init-*` | Clones Git repositories (all git-init containers) |
| `gitSync` | `git-sync-*` | Periodically syncs Git repos (HotReload policy sidecars) |
| `pluginInit` | `plugin-init` | Installs OpenCode plugins via npm |

#### OpenShift SCC Compatibility Fix

On OpenShift with `restricted-v2` SCC, containers run with random UIDs that have no writable home
directory in `/etc/passwd`. This caused `git-init` containers to crash with
`"failed to setup authentication: exit status 255"` when trying to write `~/.gitconfig`.

KubeOpenCode v0.2.0+ automatically sets `HOME=/tmp` and `SHELL=/bin/bash` on `git-init` and
`git-sync` containers (the same fix already applied to the executor container since v0.1.0).

If you are running an older version or a custom system image, you can apply the fix via
`podSpec.systemContainers`:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: AgentTemplate
metadata:
  name: ocp-base
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  podSpec:
    systemContainers:
      gitInit:
        extraEnv:
          - name: HOME
            value: /tmp
      gitSync:
        extraEnv:
          - name: HOME
            value: /tmp
```

#### Mounting Corporate CA Bundles into Git Containers

If your GitLab/GitHub uses a private CA that is not covered by `caBundle` (e.g., a different
CA per container type), you can mount it specifically into git containers:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: corp-git-agent
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  podSpec:
    extraVolumes:
      - name: corp-ca
        configMap:
          name: corp-ca-bundle
          items:
            - key: ca-certificates.crt
              path: tls.crt
    systemContainers:
      gitInit:
        extraEnv:
          - name: HOME
            value: /tmp
          - name: GIT_SSL_CAINFO
            value: /etc/ssl/corp/tls.crt
        extraVolumeMounts:
          - name: corp-ca
            mountPath: /etc/ssl/corp
            readOnly: true
      gitSync:
        extraEnv:
          - name: HOME
            value: /tmp
          - name: GIT_SSL_CAINFO
            value: /etc/ssl/corp/tls.crt
        extraVolumeMounts:
          - name: corp-ca
            mountPath: /etc/ssl/corp
            readOnly: true
    # Also mount on the executor if needed
    extraVolumeMounts:
      - name: corp-ca
        mountPath: /etc/ssl/corp
        readOnly: true
```

#### Private npm Registry for Plugin Init

Inject npm authentication only into the `plugin-init` container without exposing it to the executor:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: plugin-agent
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  plugins:
    - name: "@corp/opencode-plugin@1.0.0"
  podSpec:
    systemContainers:
      pluginInit:
        extraEnv:
          - name: NPM_TOKEN
            valueFrom:
              secretKeyRef:
                name: npm-credentials
                key: token
          - name: NPM_CONFIG_REGISTRY
            value: https://npm.corp.example.com
```

### Merge Strategy with AgentTemplate

`extraEnv` and `systemContainers` live inside `podSpec`, which follows the same merge strategy
as all other `podSpec` fields: **Agent wins entirely if it defines a `podSpec`**. If only the
template defines `podSpec`, those values are inherited.

```yaml
# Template defines base systemContainers
apiVersion: kubeopencode.io/v1alpha1
kind: AgentTemplate
metadata:
  name: ocp-base
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  podSpec:
    systemContainers:
      gitInit:
        extraEnv:
          - name: HOME
            value: /tmp

---
# Agent inherits the template's podSpec (including systemContainers)
# because Agent has no podSpec of its own
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  templateRef:
    name: ocp-base
  # No podSpec here — template's podSpec is inherited in full

---
# Agent overrides podSpec entirely — must repeat systemContainers if needed
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent-custom
spec:
  templateRef:
    name: ocp-base
  podSpec:
    resources:
      limits:
        memory: "8Gi"
    # Must re-declare systemContainers if template's values are still needed
    systemContainers:
      gitInit:
        extraEnv:
          - name: HOME
            value: /tmp
```
