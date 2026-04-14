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
    ReadOnly   bool        `json:"readOnly,omitempty"`
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
| `/s/{token}/info` | GET | Return agent info (name, namespace, readOnly) |
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
| Read-only | Not supported | Supported (drop stdin when readOnly=true) |
| Audit | None | K8s Events on Agent |

### Standalone UI

A new React page at route `/s/:token`:
- Full-screen terminal (no admin Layout/sidebar)
- Minimal header with agent name and connection status
- Read-only badge when applicable
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
kubeoc agent share <name> [--expires-in 24h] [--allowed-ips 10.0.0.0/8] [--read-only]
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
- Read-only mode for safe observation
- Audit trail via K8s Events

### Negative

- Server's ServiceAccount gains terminal access to all Agent pods (but it already has pods/exec)
- Token in URL can leak via browser history, server logs, Slack preview — mitigated by expiry and IP allowlist
- One token per Agent (no multi-user tracking); future enhancement could add named tokens

### Does Not Cover

- Multi-tenant token management (multiple share links per Agent)
- Password-based authentication (decided against in design phase for simplicity)
- Persistent session identity for shared users (all share users appear as the same ServiceAccount)

## Supersedes

None (new feature)
