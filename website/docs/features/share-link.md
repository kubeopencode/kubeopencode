# Agent Share Link

Share your Agent's web terminal with anyone — no Kubernetes credentials required.

## Why Share Links?

When an Agent developer sets up a coding assistant, they often need to share access with people who don't have (or shouldn't have) direct Kubernetes cluster access:

| Scenario | Description |
|----------|-------------|
| **Developer → QA** | Share a terminal so QA can interact with the agent directly |
| **Demo / Presentation** | Give stakeholders a live terminal URL for a demo |
| **External Collaboration** | Grant temporary access to contractors or partners |
| **Slack / ChatOps** | Post a terminal link in Slack for quick access |

Share links provide a standalone terminal page at `/s/{token}` — no admin UI, no login, just the terminal.

## Quick Start

Enable sharing on any Agent:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: team-agent
spec:
  # ... other fields ...
  share:
    enabled: true
```

Or use the CLI:

```bash
kubeoc agent share team-agent -n default
```

Output:

```
Share link enabled for agent default/team-agent

Agent:  default/team-agent
Active: true
Token:  <generated-token>
Path:   /s/<generated-token>
```

The consumer opens `https://<your-server>/s/<generated-token>` in a browser and gets a full-screen terminal.

## Configuration

### Full Example

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: team-agent
spec:
  share:
    enabled: true
    expiresAt: "2026-05-01T00:00:00Z"   # Link expires at this time
    allowedIPs:                           # Only these IPs can access
      - "10.0.0.0/8"
      - "192.168.1.0/24"
    readOnly: true                        # View-only terminal (no input)
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable/disable the share link |
| `expiresAt` | string | — | RFC3339 timestamp; link becomes invalid after this time |
| `allowedIPs` | []string | — | CIDR ranges allowed to access; empty = all IPs |
| `readOnly` | bool | `false` | When true, shared terminal is view-only (stdin dropped) |

## CLI Usage

```bash
# Enable share link
kubeoc agent share <agent-name> -n <namespace>

# Enable with options
kubeoc agent share <agent-name> --expires-in 24h --read-only
kubeoc agent share <agent-name> --allowed-ips 10.0.0.0/8,192.168.1.0/24

# Show existing share link info
kubeoc agent share <agent-name> --show

# Disable share link
kubeoc agent unshare <agent-name> -n <namespace>
```

## How It Works

1. When `spec.share.enabled` is set to `true`, the **controller** generates a 256-bit cryptographically random token
2. The token is stored in a Kubernetes Secret named `{agent-name}-share` with an OwnerReference to the Agent
3. The **server** exposes three routes outside the auth middleware:
   - `GET /s/{token}` — Standalone terminal HTML page
   - `GET /s/{token}/info` — Agent info (name, namespace, readOnly)
   - `GET /s/{token}/terminal` — WebSocket terminal connection
4. The terminal connection uses the **server's own ServiceAccount** for pod exec (no user credentials needed)
5. When share is disabled, the Secret is deleted and the token becomes invalid immediately

## Token Rotation

To rotate a share token, disable and re-enable sharing:

```bash
kubeoc agent unshare team-agent
kubeoc agent share team-agent
```

A new token is generated each time sharing is enabled.

## Status

The Agent status tracks share link state:

```yaml
status:
  share:
    secretName: team-agent-share
    active: true
  conditions:
    - type: ShareReady
      status: "True"
      reason: Active
      message: "Share link is active"
```

The `ShareReady` condition reflects the current state:

| Reason | Status | Description |
|--------|--------|-------------|
| `Active` | True | Share link is active and accessible |
| `Expired` | False | Share link has passed its `expiresAt` time |
| `AgentNotReady` | False | Agent is not ready (e.g., deployment unhealthy) |

## Security

| Measure | Detail |
|---------|--------|
| **Token strength** | 256 bits (32 bytes `crypto/rand`), brute-force infeasible |
| **Token storage** | Kubernetes Secret (encrypted at rest in etcd) |
| **Token scope** | Only grants terminal access — no API operations |
| **IP restriction** | Optional CIDR allowlist via `allowedIPs` |
| **Expiry** | Optional time-based expiration via `expiresAt` |
| **Read-only** | Optional view-only mode — stdin is dropped server-side |
| **Rate limiting** | Server throttles concurrent share requests |
| **Cleanup** | Secret has OwnerReference — deleted when Agent is deleted |
| **Audit** | Terminal access logged via Kubernetes Events on the Agent |

> **Important**: The token in the URL is the sole credential. Anyone with the URL has terminal access. Share it through secure channels, and use `expiresAt` and `allowedIPs` for additional protection.
