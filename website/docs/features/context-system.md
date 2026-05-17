# Flexible Context System

Tasks and Agents use inline **ContextItem** to provide additional context to AI agents.

## Context Types

| Type | Description | Required Fields |
|------|-------------|-----------------|
| **Text** | Inline text content | `text` |
| **ConfigMap** | Content from a Kubernetes ConfigMap | `configMap.name` |
| **Git** | Content from a Git repository | `git.repository`, `git.ref`, `mountPath` |
| **Runtime** | KubeOpenCode platform awareness system prompt | _(none)_ |
| **URL** | Content fetched from a remote HTTP/HTTPS URL | `url.source` |

## Common Fields

All context types support these optional fields:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Identifier for logging, XML tags, and deduplication |
| `description` | string | - | Human-readable documentation (no functional effect) |
| `type` | string | (required) | Context type: `Text`, `ConfigMap`, `Git`, `Runtime`, or `URL` |
| `mountPath` | string | - | Destination path (relative to workspaceDir). Empty = write to `.kubeopencode/context.md` |
| `fileMode` | *int32 | - | File permission mode (e.g., `493` for `0755` to make scripts executable) |

## Content Aggregation

Contexts without `mountPath` are written to `.kubeopencode/context.md` with XML tags. OpenCode loads this via `OPENCODE_CONFIG_CONTENT`, preserving any existing `AGENTS.md` in the repository.

Contexts **with** `mountPath` are written as files at the specified path relative to workspaceDir. Absolute paths are used as-is.

## Examples

### Text Context

Inline text content, written to `.kubeopencode/context.md` by default:

```yaml
contexts:
  - name: coding-standards
    description: "Team coding guidelines"
    type: Text
    text: |
      # Rules for AI Agent
      Always use signed commits.
      Follow Go conventions.
```

### ConfigMap Context

Mount content from a Kubernetes ConfigMap as a file:

```yaml
contexts:
  - name: project-scripts
    type: ConfigMap
    configMap:
      name: my-scripts
      key: run-tests.sh      # Optional: specific key (default: mount all keys)
    mountPath: .scripts      # Relative to workspaceDir → /workspace/.scripts/
    fileMode: 493            # 0755 in decimal — makes scripts executable
```

When `key` is omitted, all keys in the ConfigMap are mounted as individual files under `mountPath`.

### Git Context

Clone a Git repository into the workspace:

```yaml
contexts:
  - name: main-repo
    description: "Main application codebase"
    type: Git
    git:
      repository: https://github.com/org/repo.git
      ref: main
      mountPath: source-code

  - name: private-repo
    type: Git
    git:
      repository: https://github.com/org/private-repo.git
      ref: v2.0.0
      secretRef:
        name: github-git-credentials  # Secret with username + password (PAT)
      mountPath: private-source
      depth: 1                        # Shallow clone (default: 1)
      recurseSubmodules: true         # Clone submodules recursively

  - name: auto-synced-repo
    type: Git
    git:
      repository: https://github.com/org/repo.git
      ref: main
      sync:
        enabled: true
        interval: "5m"
        policy: HotReload    # HotReload (update in-place) or Rollout (rolling restart)
    mountPath: synced-repo
```

See [Git Auto-Sync](git-auto-sync.md) for sync policy details.

#### Git Secret Formats

For private repositories, create a Secret with authentication credentials:

**HTTPS Token Authentication:**
```bash
kubectl create secret generic github-git-credentials \
  --from-literal=username=x-access-token \
  --from-literal=password=ghp_YourGitHubPAT
```

**SSH Key Authentication:**
```bash
kubectl create secret generic git-ssh-credentials \
  --from-file=ssh-privatekey=$HOME/.ssh/id_rsa \
  --from-file=ssh-known-hosts=$HOME/.ssh/known_hosts
```

See [Security - Git Authentication](../security.md#git-authentication-for-private-repositories) for provider-specific username formats.

### Runtime Context

Injects KubeOpenCode platform awareness into the agent's context. This automatically provides information about the Task, Agent, and cluster environment:

```yaml
contexts:
  - name: platform-info
    type: Runtime
```

The Runtime context type generates a system prompt with details about:
- Current Task name and namespace
- Agent name and configuration
- Cluster domain and service URLs
- Workspace directory location

No `mountPath` or other sub-fields are needed for Runtime context.

### URL Context

Fetch content from a remote HTTP/HTTPS URL:

```yaml
contexts:
  - name: openapi-spec
    type: URL
    url:
      source: https://api.example.com/openapi.yaml
    mountPath: specs/openapi.yaml
```

#### URL Context with Authentication and TLS

```yaml
contexts:
  - name: internal-api-docs
    type: URL
    url:
      source: https://internal.example.com/api-docs.json
      headers:
        X-API-Key: "my-api-key"
        Authorization: "Bearer my-token"
      secretRef:
        name: url-credentials    # Secret with basic auth keys
      insecureSkipTLSVerify: false # Skip TLS verification (default: false)
      timeout: 30s                 # Request timeout (default: 30s)
    mountPath: api-docs/
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `url.source` | string | (required) | HTTP/HTTPS URL to fetch |
| `url.headers` | map[string]string | - | Custom HTTP headers |
| `url.secretRef.name` | string | - | Secret for authentication (keys: `username`, `password` for Basic Auth) |
| `url.insecureSkipTLSVerify` | bool | false | Skip TLS certificate verification |
| `url.timeout` | string | `30s` | Request timeout duration |

## Validation Rules

Each context type has specific required fields:

- **Text**: `text` is required
- **ConfigMap**: `configMap` is required
- **Git**: `git` and `mountPath` are required
- **URL**: `url` (with `source`) is required
- **Runtime**: No additional fields required