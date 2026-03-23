# Security

This document covers security considerations and best practices for KubeOpenCode.

## RBAC

KubeOpenCode follows the principle of least privilege:

- **Controller**: ClusterRole with minimal permissions for Tasks, Agents, Pods, ConfigMaps, Secrets, and Events
- **Agent ServiceAccount**: Namespace-scoped Role with read/update access to Tasks and read-only access to related resources

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

## Controller Pod Security

The controller runs with hardened security settings:

- `runAsNonRoot: true`
- `allowPrivilegeEscalation: false`
- All Linux capabilities dropped

## Agent Pod Security

Agent Pods rely on cluster-level security policies. For production deployments, consider:

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
```

## Best Practices

- **Never commit secrets to Git** - use Kubernetes Secrets, External Secrets Operator, or HashiCorp Vault
- **Apply NetworkPolicies** to limit Agent Pod egress to required endpoints only
- **Enable Kubernetes audit logging** to track Task creation and execution

## Next Steps

- [Getting Started](getting-started.md) - Installation and basic usage
- [Features](features.md) - Context system, concurrency, and more
- [Agent Images](agent-images.md) - Build custom agent images
- [Architecture](architecture.md) - System design and API reference
