# Sharing Agent Terminals

This guide walks through common patterns for sharing AI agent terminal access with team members, stakeholders, and external collaborators.

## Overview

KubeOpenCode's share link feature generates a unique URL that provides direct terminal access to an Agent — no Kubernetes credentials, no admin UI, no login. The consumer opens the URL in a browser and gets a full-screen terminal.

This is useful when:

- A platform team manages the cluster but product teams need agent access
- A developer wants QA to verify agent behavior interactively
- A manager or stakeholder needs to see a live demo
- External contractors need temporary terminal access

## Scenario 1: Developer → QA Handoff

The developer sets up an agent with the project codebase, enables sharing, and posts the link in the team's Slack channel.

### Agent Configuration

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: qa-review-agent
  namespace: qa
spec:
  profile: "QA review agent for feature-xyz"
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  contexts:
    - name: codebase
      type: Git
      git:
        repository: https://github.com/your-org/your-repo.git
        ref: feature/xyz
      mountPath: code
  credentials:
    - name: api-key
      secretRef:
        name: ai-credentials
        key: api-key
      env: OPENCODE_API_KEY
  share:
    enabled: true
    expiresAt: "2026-04-20T00:00:00Z"   # Expire after the sprint
```

### Sharing via CLI

```bash
kubeoc agent share qa-review-agent -n qa --expires-in 168h
# Output:
# Share link enabled for agent qa/qa-review-agent
# Token:  abc123...
# Path:   /s/abc123...
```

Post in Slack: *"QA review agent for feature-xyz is ready: https://kubeopencode.example.com/s/abc123..."*

### After QA is done

```bash
kubeoc agent unshare qa-review-agent -n qa
```

## Scenario 2: External Access with IP Restriction

Grant access to external contractors while restricting to their office IP range.

```yaml
spec:
  share:
    enabled: true
    allowedIPs:
      - "203.0.113.0/24"    # Contractor office IP range
      - "198.51.100.0/24"   # VPN exit IPs
    expiresAt: "2026-05-01T00:00:00Z"
```

Access from any IP outside these ranges returns a `403 Forbidden` error.

## Scenario 3: GitOps-Managed Shared Agent

For teams using GitOps (Argo CD, Flux), the share configuration is declarative and version-controlled:

```yaml
# team-agent.yaml (in your GitOps repo)
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: team-agent
  namespace: platform
spec:
  profile: "Platform team shared agent"
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  share:
    enabled: true
    # No expiry — always available
    # No IP restriction — internal network only (enforced by Ingress/NetworkPolicy)
```

The share token is generated automatically and stored in a Secret. Team members retrieve it with:

```bash
kubeoc agent share team-agent -n platform --show
```

## Tips

### Token Security

- The URL itself is the credential — treat it like a password
- Use `expiresAt` to limit the window of access
- Use `allowedIPs` when sharing with external parties
- Rotate tokens by disabling and re-enabling: `kubeoc agent unshare && kubeoc agent share`

### Networking

- The share URL must be reachable by the consumer — ensure your Ingress, HTTPRoute, or LoadBalancer exposes the KubeOpenCode server
- The server port is `2746` by default
- For local testing: `kubectl port-forward -n kubeopencode-system svc/kubeopencode-server 2746:2746`

### Monitoring

Share link access is logged as Kubernetes Events on the Agent object:

```bash
kubectl get events --field-selector involvedObject.name=team-agent -n platform
```

## Related

- [Agent Share Link (Feature Reference)](../features/share-link.md) — Field reference and status details
- [Live Agents](../features/live-agents.md) — Agent deployment and lifecycle
- [Persistence & Lifecycle](../features/persistence.md) — Suspend, resume, and standby
