# ADR 0033: Agent Share Link — Token-based Terminal Sharing

## Status

Accepted

## Date

2026-04-14

## Context

KubeOpenCode's web terminal currently requires Kubernetes RBAC credentials (kubeconfig or Bearer token) for access. All API routes sit behind authentication middleware, and the terminal handler uses impersonated user credentials for pod exec.

This creates a barrier for "Agent Consumers" — users who need to interact with an Agent but don't have (or shouldn't have) Kubernetes cluster access. Common scenarios:

1. **Developer → QA handoff**: A developer sets up an AI agent and wants to share terminal access with a QA engineer via Slack
2. **Demo/presentation**: Sharing a live terminal session with stakeholders who don't have cluster credentials
3. **External collaboration**: Granting temporary terminal access to contractors or partners

### Requirements

- Generate a unique, shareable URL for an Agent's web terminal
- The shared terminal UI must be standalone (no admin sidebar/navigation)
- No Kubernetes credentials required for the consumer
- Declarative configuration (GitOps-friendly)
- Security: cryptographically strong tokens, optional expiry, IP allowlisting

## Decision

### API Design

Add `ShareConfig` to `AgentSpec` and `ShareStatus` to `AgentStatus`:

```go
type ShareConfig struct {
    Enabled    bool        `json:"enabled"`
    ExpiresAt  *metav1.Time `json:"expiresAt,omitempty"`
    AllowedIPs []string    `json:"allowedIPs,omitempty"`
}

type ShareStatus struct {
    SecretName string `json:"secretName,omitempty"`
    URL        string `json:"url,omitempty"`
    Active     bool   `json:"active,omitempty"`
}
```

### Token Generation (Controller)

When `spec.share.enabled` is set to `true`, the AgentReconciler:

1. Generates a 32-byte cryptographically random token (base64url-encoded, ~43 characters)
2. Stores it in a Secret named `{agent-name}-share` with:
   - Label `kubeopencode.io/share-token: "true"` for efficient lookup
   - Annotation `kubeopencode.io/agent-name` and `kubeopencode.io/agent-namespace`
   - OwnerReference to the Agent (automatic cleanup on deletion)
3. Sets `status.share.secretName`, `status.share.active = true`
4. If `expiresAt` is set, schedules requeue and marks `active = false` when expired

When `spec.share` is nil or `enabled` is false:
1. Deletes the share Secret (if exists)
2. Clears `status.share`

### Server Routes

New routes **outside** the `/api/v1` auth middleware:

| Route | Method | Purpose |
|---|---|---|
| `/s/{token}` | GET | Serve standalone terminal HTML page |
| `/s/{token}/info` | GET | Return agent info (name, namespace) |
| `/s/{token}/terminal` | GET (WebSocket) | WebSocket terminal connection |

### Token Validation Flow

1. Extract token from URL path
2. List Secrets with label `kubeopencode.io/share-token=true` (cached via server's default client)
3. Find matching token in Secret data
4. Resolve Agent from Secret annotations
5. Validate: Agent exists, ready, share enabled, not expired
6. Validate IP allowlist against `X-Forwarded-For` or remote address
7. Record audit event on Agent object

### Terminal Access Model

The share terminal handler differs from the existing terminal handler:

| Aspect | Existing Terminal | Share Terminal |
|---|---|---|
| Auth | K8s Bearer Token + RBAC | Share Token (URL path) |
| K8s Client | Impersonated user credentials | Server's own ServiceAccount |
| Origin Check | Same-origin only | Cross-origin allowed (token is credential) |
| Read-only | Not supported | Not supported |
| Audit | None | K8s Events on Agent |

### Standalone UI

A new React page at route `/s/:token`:
- Full-screen terminal (no admin Layout/sidebar)
- Minimal header with agent name and connection status
- Connects WebSocket to `/s/{token}/terminal`

### Security

| Measure | Detail |
|---|---|
| Token entropy | 256 bits (32 bytes crypto/rand), brute-force infeasible |
| Token storage | K8s Secret (encrypted at rest in etcd) |
| Token rotation | Disable + re-enable share to regenerate |
| Rate limiting | Server-level throttle on `/s/` prefix |
| IP allowlist | Server validates against `X-Forwarded-For` / RemoteAddr |
| SSRF prevention | Reuses existing `validateServerURL` |
| Scope | Token only grants terminal access, no API operations |

### CLI Support

```bash
kubeoc agent share <name> [--expires-in 24h] [--allowed-ips 10.0.0.0/8]
kubeoc agent share <name> --show
kubeoc agent unshare <name>
```

These commands patch the Agent spec declaratively.

### RBAC Updates

- **Server ClusterRole**: Add Secrets `get`/`list`/`watch` (for token lookup), Events `create`/`patch` (for audit)
- **Controller ClusterRole**: Already has Secrets full access (no change needed)

## Consequences

### Positive

- Simple sharing model: one URL, no additional credentials
- Declarative (GitOps-friendly) — share config lives in Agent spec
- Automatic cleanup via OwnerReference
- Audit trail via K8s Events

### Negative

- Server's ServiceAccount gains terminal access to all Agent pods (but it already has pods/exec)
- Token in URL can leak via browser history, server logs, Slack preview — mitigated by expiry and IP allowlist
- One token per Agent (no multi-user tracking); future enhancement could add named tokens

### Does Not Cover

- Multi-tenant token management (multiple share links per Agent)
- Password-based authentication (decided against in design phase for simplicity)
- Persistent session identity for shared users (all share users appear as the same ServiceAccount)

### Decision: No `externalURL` Configuration (for now)

We considered adding a `server.externalURL` field to `KubeOpenCodeConfig` so the controller could generate a full share URL (e.g., `https://kubeopencode.example.com/s/{token}`) in `status.share.url`.

**Decision: Not implementing at this stage.** Rationale:

1. **UI auto-detection covers the primary use case.** The Agent detail page constructs the share URL using `window.location.origin`. When a user accesses KubeOpenCode through an Ingress (production) or NodePort (internal), the copied URL is already correct and shareable.

2. **CLI is acceptable without it.** The CLI outputs `Path: /s/{token}` with a note to construct the full URL manually. CLI users typically have cluster access and know their server address.

3. **externalURL is mainly for server-initiated links.** Industry precedent (Argo Workflows `server.baseUrl`, Grafana `root_url`, GitLab `external_url`) shows that this configuration is primarily needed when the server proactively pushes links — in notifications, webhooks, or emails. In our case, the user manually copies the link from the UI, making auto-detection sufficient.

4. **port-forward URLs are inherently local.** When using `kubectl port-forward`, the generated URL (`http://localhost:2746/s/...`) only works on the local machine. An `externalURL` config would fix this, but port-forward is a development workflow — production deployments use Ingress where auto-detection works correctly.

**When to revisit:** If we add notification/webhook features (e.g., auto-posting share links to Slack when enabled), `externalURL` becomes necessary. The implementation is minimal: add the field to `KubeOpenCodeConfig`, read it in the controller during share reconciliation, and set `status.share.url`.

## Supersedes

None (new feature)
