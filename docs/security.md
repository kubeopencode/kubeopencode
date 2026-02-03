# Security

This document covers security considerations and best practices for KubeOpenCode.

## RBAC

KubeOpenCode follows the principle of least privilege:

- **Controller**: ClusterRole with minimal permissions for Tasks, Agents, Pods, ConfigMaps, Secrets, and Events
- **Agent ServiceAccount**: Namespace-scoped Role with read/update access to Tasks and read-only access to related resources
- **Cross-Namespace Isolation**: When Tasks reference Agents in different namespaces, Pods run in the Agent's namespace, keeping credentials isolated

## Credential Management

- Secrets mounted with restrictive file permissions (default `0600`)
- Supports both environment variable and file-based credential mounting
- Git authentication via SecretRef (HTTPS or SSH)
- Cross-namespace Agent references keep credentials in the Agent's namespace

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
- **Use AllowedNamespaces** on Agents to restrict which namespaces can create Tasks against them
- **Apply NetworkPolicies** to limit Agent Pod egress to required endpoints only
- **Enable Kubernetes audit logging** to track Task creation and execution

### Cross-Namespace Security

When using cross-namespace Task/Agent separation:

1. Platform team manages Agents and credentials in a dedicated namespace
2. Development teams create Tasks in their own namespaces
3. Pods run in the Agent's namespace, keeping credentials isolated
4. Use `allowedNamespaces` to control which namespaces can use each Agent

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: production-agent
  namespace: platform-agents
spec:
  # Only allow production namespaces
  allowedNamespaces:
    - "prod-*"
    - "staging"
  # ... rest of config
```

## Next Steps

- [Getting Started](getting-started.md) - Installation and basic usage
- [Features](features.md) - Context system, concurrency, and more
- [Agent Images](agent-images.md) - Build custom agent images
- [Architecture](architecture.md) - System design and API reference
