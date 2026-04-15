# ADR 0026: MCP Server Support in Agent API

## Status

Proposed

## Date

2026-04-02

## Context

[Model Context Protocol (MCP)](https://modelcontextprotocol.io/) is an open standard by Anthropic for connecting AI applications to external tools, data sources, and workflows. It has been widely adopted by AI clients (Claude Desktop, Claude Code, VS Code Copilot, Cursor, ChatGPT, etc.) and has thousands of available servers for services like GitHub, Slack, PostgreSQL, Sentry, and more.

OpenCode already has native, production-ready MCP support. MCP servers are configured in the `mcp` section of `opencode.json`:

```json
{
  "mcp": {
    "github": {
      "type": "remote",
      "url": "https://mcp.github.com/sse",
      "headers": { "Authorization": "Bearer ghp_xxx" }
    },
    "filesystem": {
      "type": "local",
      "command": ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/workspace"],
      "environment": { "API_KEY": "value" }
    }
  }
}
```

Currently, KubeOpenCode users must manually embed MCP configuration inside the `spec.config` JSON field. This approach has several problems:

1. **Not secure** — Credentials (API keys, tokens) must be written as plaintext in the JSON string. There is no way to leverage Kubernetes Secrets.
2. **Not Kubernetes-native** — No schema validation, no kubectl visibility, no support for AgentTemplate inheritance.
3. **Poor user experience** — Users must understand OpenCode's internal config schema and manually construct JSON.

MCP is a critical capability for AI agents — it determines what external tools and data sources the agent can access. It deserves first-class API support in KubeOpenCode, just like `contexts`, `credentials`, and `skills`.

## Decision

### New API Type: MCPServer

Introduce `MCPServer` as an inline type in `AgentSpec` and `AgentTemplateSpec`. MCP servers are declared as a list rather than requiring a separate CRD, because their lifecycle is tightly bound to the Agent.

#### Type Definitions (`api/v1alpha1/mcp_types.go`)

```go
// MCPServerType defines the transport type for an MCP server.
// +kubebuilder:validation:Enum=local;remote
type MCPServerType string

const (
    MCPServerTypeLocal  MCPServerType = "local"
    MCPServerTypeRemote MCPServerType = "remote"
)

// MCPServer defines an MCP (Model Context Protocol) server configuration.
// MCP servers provide additional tools, resources, and prompts to the AI agent
// via the standardized MCP protocol.
type MCPServer struct {
    // Name is a unique identifier for this MCP server.
    // Used as the key in OpenCode's mcpServers configuration.
    // Must be a valid DNS subdomain name (lowercase alphanumeric and hyphens).
    // +required
    // +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
    // +kubebuilder:validation:MaxLength=63
    Name string `json:"name"`

    // Type is the transport type for connecting to the MCP server.
    //   - "local": Starts the server as a subprocess using stdio transport.
    //   - "remote": Connects to a running server via HTTP (Streamable HTTP or SSE).
    // +required
    // +kubebuilder:validation:Enum=local;remote
    Type MCPServerType `json:"type"`

    // URL of the remote MCP server endpoint.
    // Required when type is "remote". Ignored when type is "local".
    //
    // For in-cluster MCP services:
    //   url: "http://mcp-github.tools.svc.cluster.local:8080/mcp"
    // For external MCP services:
    //   url: "https://mcp.example.com/sse"
    // +optional
    URL string `json:"url,omitempty"`

    // Headers to send with HTTP requests to a remote MCP server.
    // For sensitive values (tokens, API keys), use credentialRef instead.
    // Ignored when type is "local".
    // +optional
    Headers map[string]string `json:"headers,omitempty"`

    // Command and arguments to run the local MCP server process.
    // The server communicates via stdio (stdin/stdout JSON-RPC).
    // Required when type is "local". Ignored when type is "remote".
    //
    // Example:
    //   command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/workspace"]
    // +optional
    Command []string `json:"command,omitempty"`

    // Environment variables for the local MCP server process.
    // For sensitive values, use credentialRef instead.
    // Ignored when type is "remote".
    // +optional
    Environment map[string]string `json:"environment,omitempty"`

    // CredentialRef references a Secret containing sensitive configuration.
    // The Secret must exist in the same namespace as the Agent.
    //
    // How the Secret keys are used depends on the server type:
    //
    // For type "remote":
    //   Secret keys are injected as Pod environment variables.
    //   At container startup, an entrypoint script reads these environment
    //   variables and injects them as HTTP headers into the MCP config.
    //   Naming convention: MCP_{SERVER_NAME}_{KEY} (uppercase, hyphens → underscores).
    //   Common usage: Secret with key "Authorization" → env MCP_GITHUB_AUTHORIZATION.
    //
    // For type "local":
    //   Secret keys are injected directly as Pod environment variables
    //   (using their original key names). The local MCP server subprocess
    //   inherits all container environment variables automatically.
    //   Common usage: Secret with key "DATABASE_URL" → env DATABASE_URL.
    //
    // +optional
    CredentialRef *corev1.LocalObjectReference `json:"credentialRef,omitempty"`

    // Enabled controls whether this MCP server is active.
    // Defaults to true if not specified.
    // +optional
    Enabled *bool `json:"enabled,omitempty"`

    // Timeout in milliseconds for MCP server requests.
    // Defaults to 60000 (60 seconds) if not specified.
    // Kubernetes environments typically have higher network latency
    // than local development, so the default is higher than OpenCode's
    // default of 5000ms.
    // +optional
    // +kubebuilder:validation:Minimum=1000
    // +kubebuilder:validation:Maximum=300000
    Timeout *int32 `json:"timeout,omitempty"`
}
```

#### AgentSpec Change

Add `MCPServers` field after `Config`:

```go
// MCPServers defines MCP (Model Context Protocol) servers available to this Agent.
// MCP servers extend the Agent's capabilities by providing additional tools,
// resources, and prompts via the standardized MCP protocol.
//
// These are merged into the OpenCode configuration at runtime. If spec.config
// also contains an "mcp" section, mcpServers takes precedence (overwrites).
// +optional
MCPServers []MCPServer `json:"mcpServers,omitempty"`
```

The same field is added to `AgentTemplateSpec`.

### User-Facing YAML Examples

#### In-Cluster MCP Server (Kubernetes Service)

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: dev-agent
spec:
  workspaceDir: /workspace
  serviceAccountName: dev-agent-sa
  mcpServers:
    - name: github
      type: remote
      url: "http://mcp-github.tools.svc.cluster.local:8080/mcp"
      credentialRef:
        name: github-mcp-token  # Secret with key "Authorization"
```

#### External SaaS MCP Server

```yaml
  mcpServers:
    - name: sentry
      type: remote
      url: "https://mcp.sentry.dev/sse"
      credentialRef:
        name: sentry-mcp-token
      timeout: 30000
```

#### Local stdio MCP Server

```yaml
  mcpServers:
    - name: filesystem
      type: local
      command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/workspace"]
      timeout: 10000
    - name: postgres
      type: local
      command: ["npx", "-y", "@modelcontextprotocol/server-postgres"]
      credentialRef:
        name: postgres-mcp-env  # Secret with key "DATABASE_URL"
```

#### AgentTemplate with Shared MCP Config

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: AgentTemplate
metadata:
  name: team-base
spec:
  workspaceDir: /workspace
  serviceAccountName: team-sa
  mcpServers:
    - name: github
      type: remote
      url: "http://mcp-github.tools.svc.cluster.local:8080/mcp"
      credentialRef:
        name: shared-github-token
    - name: sentry
      type: remote
      url: "https://mcp.sentry.dev/sse"
      credentialRef:
        name: shared-sentry-token
---
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  templateRef:
    name: team-base
  # Inherits mcpServers from team-base template
  # Can override by setting mcpServers (replaces entire list)
```

### Config Merge Strategy

The controller generates the final OpenCode config by merging `spec.mcpServers` into `spec.config`:

1. Parse `spec.config` JSON (if provided).
2. Convert `spec.mcpServers` to OpenCode's `mcp` config format.
3. Set the `mcp` key in the config JSON. **`mcpServers` always overwrites** any `mcp` section in `spec.config`.
4. Write the merged config to `/tools/opencode.json`.

This "overwrite" strategy is simple and avoids confusing merge semantics. If users need fine-grained control, they should use `mcpServers` exclusively and remove `mcp` from `spec.config`.

### Credential Security: Environment Variable Injection

Sensitive credentials from `credentialRef` must NOT appear in ConfigMap data. The controller uses environment variable injection:

#### Remote MCP Server Credentials

1. Controller reads the referenced Secret keys.
2. Each key-value pair becomes a Pod environment variable with naming convention: `MCP_{SERVER_NAME}_{KEY}` (uppercase, hyphens replaced with underscores).
3. The config JSON written to `/tools/opencode.json` does NOT contain credential values.
4. The container entrypoint script reads the environment variables and dynamically injects them as headers into the config before starting OpenCode.

```bash
# Generated entrypoint (conceptual):
sh -c '
  sed -i "s|__MCP_GITHUB_AUTHORIZATION__|$MCP_GITHUB_AUTHORIZATION|g" /tools/opencode.json
  /tools/opencode serve --port 4096 --hostname 0.0.0.0
'
```

**Alternative (simpler):** For remote MCP servers, if OpenCode's MCP client supports reading headers from environment variables natively, we can skip the entrypoint injection entirely and just set the env vars. This needs to be verified against OpenCode's implementation.

#### Local MCP Server Credentials

Local MCP servers are simpler — they run as subprocesses and automatically inherit all parent process environment variables:

1. Controller reads the referenced Secret keys.
2. Each key-value pair becomes a Pod environment variable using the **original key name** (e.g., `DATABASE_URL`, `API_KEY`).
3. No config JSON modification needed — the subprocess inherits the env vars.

This aligns with the standard MCP pattern where stdio servers receive credentials via environment variables.

### Template Merge

`mcpServers` follows the same list merge strategy as `contexts`, `credentials`, and `skills`:

- If Agent's `mcpServers` is non-nil (even if empty `[]`): **use Agent's list** (completely replaces template).
- If Agent's `mcpServers` is nil: **inherit from template**.

This is consistent with the existing API contract and avoids complex per-item merge logic.

### Validation Rules

The controller validates `mcpServers` during reconciliation:

1. **Name uniqueness**: No two servers in the same Agent can have the same name.
2. **Type-specific required fields**:
   - `type: remote` requires `url` to be non-empty.
   - `type: local` requires `command` to be non-empty.
3. **Type-specific field warnings**:
   - `type: remote` with `command` or `environment` set: warn (fields are ignored).
   - `type: local` with `url` or `headers` set: warn (fields are ignored).
4. **credentialRef**: Referenced Secret must exist in the same namespace.

## Key Design Choices

### 1. Inline list vs. separate MCPServer CRD

**Decision: Inline list.**

A separate CRD (`MCPServer`) would enable cross-Agent sharing and independent lifecycle management. However:
- MCP server config is lightweight (a few fields), not worth a full CRD.
- AgentTemplate already provides the sharing mechanism.
- MCP server lifecycle is tightly coupled to the Agent — no need for independent management.
- A separate CRD adds operational complexity (RBAC, watchers, garbage collection).

This can be revisited in a future API version if cross-namespace sharing becomes a real need.

### 2. No OAuth support (v1alpha1)

**Decision: No OAuth flow.**

OpenCode supports OAuth for remote MCP servers (browser redirect, PKCE, dynamic client registration). However, Kubernetes is a headless environment — there is no browser for OAuth redirects. In enterprise Kubernetes:
- Pre-registered service accounts and API tokens are the norm.
- OAuth is typically handled at the infrastructure level (service mesh, API gateway).
- Tokens are stored in Kubernetes Secrets and rotated by external secret operators.

Users who need OAuth-authenticated MCP servers should obtain tokens externally and store them in Secrets. The `credentialRef` mechanism handles this cleanly.

### 3. Timeout defaults to 60 seconds

**Decision: 60s default (vs. OpenCode's 5s default).**

Kubernetes environments have higher network latency than local development:
- Cross-namespace service calls add DNS resolution time.
- External MCP servers go through proxies, load balancers, and firewalls.
- Pod startup and scaling can delay first responses.

60 seconds is a safer default that avoids false timeouts while still catching genuinely unresponsive servers.

### 4. mcpServers overwrites config.mcp

**Decision: Full overwrite, not merge.**

If both `spec.mcpServers` and `spec.config.mcp` are set, `mcpServers` wins entirely. This avoids:
- Ambiguous merge semantics (what if the same server name appears in both?).
- Confusing precedence rules.
- Users maintaining MCP config in two places.

The recommendation is to use `mcpServers` for all MCP configuration and keep `spec.config` for non-MCP settings.

### 5. credentialRef uses environment variable injection

**Decision: Env vars, not ConfigMap/inline embedding.**

Alternatives considered:
- **Embedding in config JSON**: Simple but exposes credentials in ConfigMap data (security concern).
- **Secret volume + entrypoint merge**: Most secure but complex (requires jq or custom merge logic in entrypoint).
- **Environment variable injection**: Good balance — credentials stay in Secrets/env vars (not in ConfigMap), implementation is straightforward. Local MCP servers inherit env vars naturally. Remote servers need entrypoint script to inject headers.

## Implementation Scope

### Files to Create/Modify

**API Layer:**
- `api/v1alpha1/mcp_types.go` — New file: MCPServer type definitions
- `api/v1alpha1/agent_types.go` — Add MCPServers field to AgentSpec
- `api/v1alpha1/agenttemplate_types.go` — Add MCPServers field to AgentTemplateSpec
- Run `make update` to regenerate deepcopy + CRDs

**Controller Layer:**
- `internal/controller/pod_builder.go` — Add mcpServers to agentConfig struct, add buildMCPConfig() and mergeConfigWithMCP() functions
- `internal/controller/server_builder.go` — MCP config merge for Agent Deployment
- `internal/controller/template_merge.go` — Add mcpServers merge logic
- `internal/controller/task_controller.go` — MCP-aware config handling

**Tests:**
- `internal/controller/pod_builder_test.go` — MCP config generation/merge tests
- `internal/controller/template_merge_test.go` — mcpServers merge tests
- `internal/controller/server_builder_test.go` — Agent Deployment MCP tests

**Documentation:**
- `docs/features.md` — MCP configuration section
- `docs/architecture.md` — API field reference update
- `AGENTS.md` — Update Agent Configuration Summary

**Helm Chart:**
- No RBAC changes needed (mcpServers is a field on existing Agent/AgentTemplate CRDs)

## Consequences

### Positive

- **Kubernetes-native MCP**: Schema validation, kubectl visibility, template inheritance
- **Secure credential handling**: Secrets-based, no plaintext in config JSON
- **Consistent with existing API patterns**: Same merge strategy as contexts, credentials, skills
- **Extensible**: Can add OAuth, sidecar mode, or MCPServer CRD in future versions

### Negative

- **Remote credentialRef adds entrypoint complexity**: Need shell script to inject env vars as headers at startup
- **No cross-namespace sharing**: MCP server configs can only be shared via AgentTemplate within the same namespace
- **No dynamic MCP server discovery**: Servers must be statically declared (no auto-discovery from K8s service annotations)

### Risks

- OpenCode's MCP config format may change — the controller must track OpenCode releases
- Environment variable naming collisions (MCP_* prefix mitigates this)
- Local MCP servers in containers may have limited tool availability (e.g., npx requires Node.js in the executor image)

## Future Considerations

1. **MCPServer CRD**: If cross-namespace or cross-Agent sharing becomes a real need
2. **Sidecar MCP servers**: Run MCP server as a sidecar container (can be done today via `podSpec` + `type: remote` with localhost URL)
3. **MCP server auto-discovery**: Kubernetes Service annotations to auto-register MCP servers
4. **OAuth support**: When OpenCode adds headless OAuth flows or device authorization grant
5. **MCP server health monitoring**: Expose MCP server connection status in Agent status
