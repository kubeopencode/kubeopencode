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
