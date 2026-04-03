# Security

This document covers security considerations and best practices for KubeOpenCode.

## RBAC

KubeOpenCode follows the principle of least privilege:

- **Controller**: ClusterRole with minimal permissions for Tasks, Agents, Pods, ConfigMaps, Secrets, and Events
- **Agent ServiceAccount**: Namespace-scoped Role with read/update access to Tasks and read-only access to related resources

### Web UI User Permissions

The Helm chart includes a `kubeopencode-web-user` ClusterRole with all permissions needed to use the web dashboard. Bind it per namespace to grant team access:

| Permission | Resource | Verbs | Used By |
|-----------|----------|-------|---------|
| View tasks and agents | `kubeopencode.io` tasks, agents | get, list, watch | Dashboard, task list |
| Manage tasks | `kubeopencode.io` tasks | create, delete, patch | Task creation, stop, delete |
| View pods | `""` pods | get, list | Task detail (pod status) |
| Stream logs | `""` pods/log | get | Log viewer |
| Web terminal | `""` pods/exec | create | Terminal to agent server pods |

**Example: Grant access to a team in their namespace**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: team-a-kubeopencode-user
  namespace: team-a
subjects:
  - kind: Group
    name: team-a-devs
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: kubeopencode-web-user
  apiGroup: rbac.authorization.k8s.io
```

**Restricted access (no terminal, logs only):**

Create a custom ClusterRole without `pods/exec`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubeopencode-viewer
rules:
  - apiGroups: ["kubeopencode.io"]
    resources: ["tasks", "agents"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get"]
```

> **Note:** The web UI server enforces RBAC by impersonating the authenticated user for all Kubernetes API calls. Users will only see resources and actions they have permission for.

### CLI (`kubeoc`) Permissions

The `kubeoc` CLI communicates directly with the Kubernetes API using your kubeconfig credentials. The following table shows the minimum RBAC permissions required for each command:

| Command | Resource | Verbs | Notes |
|---------|----------|-------|-------|
| `kubeoc get agents` | `kubeopencode.io` agents | get, list | |
| `kubeoc get tasks` | `kubeopencode.io` tasks | get, list | |
| `kubeoc get crontasks` | `kubeopencode.io` crontasks | get, list | |
| `kubeoc get agenttemplates` | `kubeopencode.io` agenttemplates, agents | get, list | Lists agents to count template references |
| `kubeoc agent attach` | `kubeopencode.io` agents | get, **patch** | `patch` needed for connection heartbeat annotation |
| `kubeoc agent attach` | `""` services/proxy | get | Service proxy to kubeopencode-server (in `kubeopencode-system` namespace) |
| `kubeoc agent suspend/resume` | `kubeopencode.io` agents | get, update | |
| `kubeoc task stop` | `kubeopencode.io` tasks | get, update | Adds `kubeopencode.io/stop` annotation |
| `kubeoc task logs` | `kubeopencode.io` tasks, `""` pods, pods/log | get | |
| `kubeoc crontask trigger` | `kubeopencode.io` crontasks | get, patch | Adds `kubeopencode.io/trigger` annotation |
| `kubeoc crontask suspend/resume` | `kubeopencode.io` crontasks | get, update | |

**Full CLI user ClusterRole:**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubeoc-user
rules:
# View all KubeOpenCode resources
- apiGroups: ["kubeopencode.io"]
  resources: ["agents", "agenttemplates", "tasks", "crontasks"]
  verbs: ["get", "list"]
# Manage agents (suspend/resume, connection heartbeat during attach)
- apiGroups: ["kubeopencode.io"]
  resources: ["agents"]
  verbs: ["update", "patch"]
# Manage tasks (stop)
- apiGroups: ["kubeopencode.io"]
  resources: ["tasks"]
  verbs: ["update"]
# Manage crontasks (trigger, suspend/resume)
- apiGroups: ["kubeopencode.io"]
  resources: ["crontasks"]
  verbs: ["update", "patch"]
# View pod status and logs
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get"]
# Service proxy for agent attach (in kubeopencode-system namespace)
- apiGroups: [""]
  resources: ["services/proxy"]
  verbs: ["get"]
```

> **Note:** The `kubeoc-user` ClusterRole must be bound with a **ClusterRoleBinding** (not a namespaced RoleBinding) if the user needs to access the `services/proxy` in the `kubeopencode-system` namespace for `kubeoc agent attach`. Alternatively, create separate RoleBindings for the agent namespace and the server namespace.

> **Note:** The web-user ClusterRole (`kubeopencode-web-user`) included in the Helm chart already covers all CLI permissions. If a user already has the web-user role, no additional role is needed for `kubeoc`.

## Credential Management

- Secrets mounted with restrictive file permissions (default `0600`)
- Supports both environment variable and file-based credential mounting
- Git authentication via SecretRef (HTTPS or SSH)

### Git Authentication for Private Repositories

When a Git context references a private repository, use `secretRef` to provide credentials.
The Secret is referenced by the `git.secretRef.name` field in the context spec.

#### HTTPS Token Authentication (Recommended)

For most Git providers, create a Secret with `username` and `password` keys.
The `password` field should contain a Personal Access Token (PAT), not your actual password.

**GitHub:**

```bash
kubectl create secret generic github-git-credentials \
  --from-literal=username=x-access-token \
  --from-literal=password=ghp_YourGitHubPAT
```

**GitLab:**

```bash
kubectl create secret generic gitlab-git-credentials \
  --from-literal=username=oauth2 \
  --from-literal=password=glpat-YourGitLabPAT
```

**Bitbucket:**

```bash
kubectl create secret generic bitbucket-git-credentials \
  --from-literal=username=x-token-auth \
  --from-literal=password=YourBitbucketAppPassword
```

**Azure DevOps:**

```bash
kubectl create secret generic azdo-git-credentials \
  --from-literal=username=pat \
  --from-literal=password=YourAzureDevOpsPAT
```

Then reference the Secret in your Git context:

```yaml
contexts:
  - name: private-source
    type: Git
    git:
      repository: https://github.com/org/private-repo.git
      ref: main
      secretRef:
        name: github-git-credentials
    mountPath: source
```

#### SSH Key Authentication

For SSH-based authentication, create a Secret with an `ssh-privatekey` key
and optionally an `ssh-known-hosts` key:

```bash
kubectl create secret generic git-ssh-credentials \
  --from-file=ssh-privatekey=$HOME/.ssh/id_rsa \
  --from-file=ssh-known-hosts=$HOME/.ssh/known_hosts
```

Then use an SSH repository URL:

```yaml
contexts:
  - name: private-source
    type: Git
    git:
      repository: git@github.com:org/private-repo.git
      ref: main
      secretRef:
        name: git-ssh-credentials
    mountPath: source
```

> **Security note:** If `ssh-known-hosts` is not provided, SSH host key verification is disabled.
> Always provide `ssh-known-hosts` in production environments to prevent MITM attacks.

#### Provider Username Reference

| Git Provider | Username | Token Type |
|-------------|----------|------------|
| GitHub | `x-access-token` | Personal Access Token (PAT) |
| GitLab | `oauth2` | Personal/Project/Group Access Token |
| Bitbucket | `x-token-auth` | App Password |
| Azure DevOps | (any non-empty string) | Personal Access Token (PAT) |

### Credential Mounting Options

```yaml
# Environment variable
credentials:
  - name: api-key
    secretRef:
      name: my-secrets
      key: api-key
    env: API_KEY

# File mount with restricted permissions
credentials:
  - name: ssh-key
    secretRef:
      name: ssh-keys
      key: id_rsa
    mountPath: /home/agent/.ssh/id_rsa
    fileMode: 0400
```

## TLS and CA Certificate Management

When Tasks need to access private Git servers or internal HTTPS services that use self-signed or private CA certificates, use the Agent's `caBundle` field to provide custom CA certificates.

### Recommended: Custom CA Bundle

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: internal-agent
spec:
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  caBundle:
    configMapRef:
      name: corporate-ca-bundle
      key: ca-bundle.crt
```

The CA certificate is mounted into all containers (init containers and worker container) at `/etc/ssl/certs/custom-ca/tls.crt`. The `git-init` container sets `GIT_SSL_CAINFO` to trust the custom CA, and the `url-fetch` container appends it to the system certificate pool.

This approach is compatible with [cert-manager trust-manager](https://cert-manager.io/docs/trust/trust-manager/), which can automatically distribute CA bundles as ConfigMaps across namespaces.

### Avoid: Disabling TLS Verification

Do not use `InsecureSkipTLSVerify` or `GIT_SSL_NO_VERIFY=true` to work around certificate issues. Disabling TLS verification exposes the agent to man-in-the-middle attacks. Always configure the correct CA bundle instead.

See [Features - Custom CA Certificates](features.md#custom-ca-certificates) for detailed configuration examples.

## Controller Pod Security

The controller runs with hardened security settings:

- `runAsNonRoot: true`
- `allowPrivilegeEscalation: false`
- All Linux capabilities dropped

## Agent Pod Security

### Default Security Context

KubeOpenCode applies a restricted security context by default to all agent containers (init containers and the worker container). When no custom `securityContext` is specified in `podSpec`, the following defaults are applied:

- `allowPrivilegeEscalation: false` - prevents containers from gaining additional privileges
- `capabilities: drop: ["ALL"]` - drops all Linux capabilities
- `seccompProfile: type: RuntimeDefault` - enables the default seccomp profile

These defaults align with the Kubernetes [Restricted Pod Security Standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted) and are suitable for most workloads.

You can override these defaults or add stricter settings using `podSpec.securityContext` (container-level) and `podSpec.podSecurityContext` (pod-level). See [Features - Pod Security](features.md#pod-security) for detailed configuration examples.

### Runtime Isolation

For production deployments, consider additional isolation measures:

- Configuring Pod Security Standards (PSS) at the namespace level
- Using `spec.podSpec.runtimeClassName` for gVisor or Kata Containers isolation
- Applying NetworkPolicies to restrict Agent Pod network access
- Setting resource limits via LimitRange or ResourceQuota

### Example: Enhanced Isolation

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: secure-agent
spec:
  agentImage: quay.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: quay.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  podSpec:
    # Enhanced isolation with gVisor
    runtimeClassName: gvisor
    # Labels for NetworkPolicy targeting
    labels:
      network-policy: agent-restricted
    # Tighter container security
    securityContext:
      runAsNonRoot: true
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
          - ALL
    # Pod-level security
    podSecurityContext:
      runAsUser: 1000
      runAsGroup: 1000
      fsGroup: 1000
```

## Private Registry Authentication

When agent images are hosted in private registries that require authentication, configure `imagePullSecrets` on the Agent. The referenced Secrets must be of type `kubernetes.io/dockerconfigjson` and exist in the same namespace as the Agent.

See [Features - Private Registry Authentication](features.md#private-registry-authentication) for detailed configuration and examples.

## Network Proxy Configuration

Enterprise environments often require outbound traffic to pass through a corporate proxy. KubeOpenCode supports proxy configuration at both the Agent level and the cluster level via `KubeOpenCodeConfig`. Agent-level settings override cluster-level settings.

See [Features - HTTP/HTTPS Proxy Configuration](features.md#httphttps-proxy-configuration) for detailed configuration and examples.

## Best Practices

- **Never commit secrets to Git** - use Kubernetes Secrets, External Secrets Operator, or HashiCorp Vault
- **Apply NetworkPolicies** to limit Agent Pod egress to required endpoints only
- **Enable Kubernetes audit logging** to track Task creation and execution

## Next Steps

- [Getting Started](getting-started.md) - Installation and basic usage
- [Features](features.md) - Context system, concurrency, and more
- [Agent Images](agent-images.md) - Build custom agent images
- [Architecture](architecture.md) - System design and API reference
