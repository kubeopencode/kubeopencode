# ADR 0017: OpenCode Web UI Integration via Self-Hosted Reverse Proxy

> **Status: Superseded** by [ADR 0018](0018-web-terminal-replaces-web-ui.md). The OpenCode Web UI proxy approach described here has been replaced with a web terminal (xterm.js + Kubernetes pod exec).

## Status

Superseded by [ADR 0018](0018-web-terminal-replaces-web-ui.md) (2026-03-27)

## Context

KubeOpenCode supports **server-mode agents** — persistent OpenCode servers running in Kubernetes
(see ADR 0011). Users interact with these agents through:

1. **CLI**: `kubeopencode agent attach` with kubectl port-forward or proxy mode
2. **Web Terminal**: Browser-based terminal access via xterm.js (ADR 0018)

Neither provides the full OpenCode development experience in the browser. Users must install
the CLI, have kubectl access, and work from the terminal. The goal is to let users interact
with server-mode agents directly from the KubeOpenCode Web UI — zero CLI, zero port-forward.

### OpenCode Web UI Architecture

OpenCode has a SolidJS-based Web UI hosted at `app.opencode.ai`. Key architecture:

- The OpenCode server has a **catch-all route** that proxies all unmatched HTTP requests to
  `https://app.opencode.ai`, serving the Web UI as static assets through the server itself.
- The Web UI uses `@opencode-ai/sdk` with a configurable `baseUrl` for all API calls.
- In production, `baseUrl` defaults to `location.origin` — the Web UI makes API calls to
  the same origin it was loaded from.
- All real-time communication uses **Server-Sent Events (SSE)** via fetch-based streaming
  (not native `EventSource`), going through the SDK.

### Integration Challenge

Serving the OpenCode Web UI through a KubeOpenCode proxy at a subpath
(`/api/v1/namespaces/{ns}/agents/{name}/web/`) breaks two things:

1. **Static assets**: HTML references root-relative paths (`/assets/app.js`) which resolve to
   `{origin}/assets/app.js` instead of through the proxy.
2. **API calls**: The SDK uses `location.origin` as `baseUrl`, so `fetch('{origin}/session/list')`
   bypasses the proxy entirely.

## Decision

### Integration Approach: Transparent Reverse Proxy with URL Rewriting

We add a new route `/{name}/web/*` in the KubeOpenCode server that reverse-proxies to the
agent's OpenCode server, with two modifications to HTML responses:

1. **Asset path rewriting**: Root-relative paths in `src=` and `href=` attributes are rewritten
   to include the proxy base path.
2. **Fetch monkey-patch injection**: A `<script>` is injected before `</head>` that overrides
   `window.fetch` to transparently rewrite API call URLs so they route through the proxy.

**Request flow (self-hosted mode — default for production):**
```
Browser
  → /api/v1/namespaces/{ns}/agents/{name}/web/{path}
  → KubeOpenCode Server (agent_web_handler.go)
    - Static assets (/assets/*): served from embedded filesystem
    - index.html: served with fetch-patch injection
    - SPA routes: serve index.html (client-side routing)
    - API calls (/session/*, /global/*, etc.): proxied to OpenCode Server
  → OpenCode Server (in-cluster, API calls only)
```

**Request flow (fallback mode — no embedded assets):**
```
Browser
  → /api/v1/namespaces/{ns}/agents/{name}/web/{path}
  → KubeOpenCode Server (agent_web_handler.go)
    - ALL requests: proxied with HTML rewriting + fetch patch
  → OpenCode Server (in-cluster)
    - API routes: handled directly
    - Unmatched routes: proxied to app.opencode.ai (static assets)
```

### Self-Hosting Strategy

The OpenCode Web UI is built from source and embedded into the KubeOpenCode binary at
build time, eliminating the runtime dependency on `app.opencode.ai`:

```bash
# Build OpenCode Web UI from source (requires ../opencode/ checkout)
make opencode-app-build

# The built assets are embedded via go:embed in internal/opencode-app/
# Then build the KubeOpenCode binary as usual
make build
```

When embedded assets are available, the handler operates in **self-hosted mode**:
- Static files (JS, CSS, images) are served directly from the embedded filesystem
- Only API calls are proxied to the agent's OpenCode server
- No network requests to `app.opencode.ai` at any point

When embedded assets are NOT available (development or when the build step is skipped),
the handler falls back to **proxy mode** — proxying everything through the OpenCode server,
which proxies static assets from `app.opencode.ai`. This is logged at startup:
```
Self-hosted OpenCode Web UI enabled (embedded assets found)
// or
Self-hosted OpenCode Web UI not available, falling back to proxy mode
```

**Version pinning**: Pin to a specific OpenCode release by checking out the desired tag
before building:
```bash
cd ../opencode && git checkout v1.3.2
make opencode-app-build
```

**Why monkey-patch fetch instead of other approaches:**

| Approach | Problem |
|----------|---------|
| localStorage `defaultServerUrl` | SDK initializes with `location.origin` before localStorage is checked in some code paths. The `servers` array is populated separately. |
| `<base>` tag | Only affects HTML attributes, not JavaScript `fetch()` calls |
| Service Worker | Complex, requires separate registration, HTTPS-only |
| Subdomain-per-agent | Requires DNS wildcard + TLS wildcard cert — not universally available |
| **Fetch monkey-patch + JS rewrite** | **Combined approach: rewrite `localhost:4096` in JS bundle at serve time + monkey-patch fetch for URL prefix rewriting + decompose Request objects to avoid ReadableStream/ALPN issues** |

**Important implementation detail**: The fetch patch must decompose `Request` objects into
`string URL + init dict` (reading the body via `req.text()`) rather than using
`new Request(newUrl, oldReq)`. This avoids two critical browser issues:
1. Non-standard properties (e.g., `timeout` from OpenCode SDK's Bun-specific customFetch) being inherited
2. ReadableStream bodies requiring `duplex: 'half'`, which triggers HTTP/2 ALPN negotiation that fails over kubectl port-forward

### Key Implementation Details

- **New handler**: `internal/server/handlers/agent_web_handler.go`
- **Route**: `/{name}/web/*` alongside existing `/{name}/proxy/*`
- **SSE support**: `FlushInterval: -1`, `context.WithoutCancel()` (same as existing proxy)
- **Auth**: Inherits existing API middleware (token validation + impersonation)
- **UI**: "Open Web UI" button on AgentDetailPage for server-mode agents

## Security Analysis

### Threat 1: Supply Chain — Third-Party Code Execution (MITIGATED)

**Risk**: In fallback (non-self-hosted) mode, the OpenCode Web UI is served from
`app.opencode.ai`, a third-party CDN. This JavaScript would execute in the **same origin**
as the KubeOpenCode dashboard, with access to cookies, localStorage, and API calls.

**Mitigation (implemented):**
- **Self-hosting is the default for production builds.** The OpenCode Web UI is built from
  source (`make opencode-app-build`) and embedded into the KubeOpenCode binary. No runtime
  requests to `app.opencode.ai` are made.
- **CSP hardening**: Both self-hosted and fallback modes set restrictive Content-Security-Policy
  headers: `connect-src 'self'` prevents data exfiltration to external servers,
  `frame-ancestors 'self'` prevents clickjacking.
- **X-Frame-Options: SAMEORIGIN** for older browser support.
- **Version pinning**: Self-hosted assets are built from a specific OpenCode git commit/tag,
  providing full control over what code runs.
- The web route requires the same authentication as all other API routes.

**Residual risk in fallback mode**: If the build step is skipped, the handler falls back to
proxying through the OpenCode server, which proxies `app.opencode.ai`. This mode should only
be used for development. Production Docker images MUST include `make opencode-app-build`.

### Threat 2: RBAC Bypass via Agent Proxy (CRITICAL — Pre-existing)

**Risk**: Once a user accesses the OpenCode Web UI through the proxy, they have full control
of the agent's OpenCode server — including shell execution (`POST /pty`), file read/write,
session management, and AI tool invocation. The question is: can unauthorized users access
agents they shouldn't?

**Current state**: The KubeOpenCode server has an impersonation middleware that creates a
Kubernetes client impersonating the authenticated user. **However**, there is a pre-existing
bug where `clientContextKey` is defined as separate types in `internal/server/server.go`
(package `server`) and `internal/server/handlers/agent_handler.go` (package `handlers`).
Since Go uses type identity for context keys, the handlers never receive the impersonated
client and always fall back to the server's default client, which has unrestricted access.

**Impact**: Any authenticated user (or unauthenticated user when anonymous access is allowed)
can access any server-mode agent in any namespace, regardless of Kubernetes RBAC.

**Note**: This is a pre-existing bug that affects ALL server endpoints (proxy, web, tasks,
agents), not just the new web UI integration. It should be tracked and fixed separately.

**Fix**: Export `clientContextKey` from a shared location (e.g., `handlers` package) and
import it in `server.go`, or pass the key type as a parameter.

### Threat 3: OpenCode Server is Single-User by Design (MEDIUM)

**Risk**: The OpenCode server is designed for single-user local development. It has no
multi-tenancy, no session isolation, and no per-user access control.

**Impact**: When multiple KubeOpenCode users access the same server-mode agent:
- All users share sessions — User A can see/modify User B's sessions
- All users share PTY terminals
- No audit trail of which user performed which action
- `POST /global/dispose` can shut down the entire server

**Mitigation**: This is inherent to OpenCode's design. Server-mode agents should be treated
as shared resources. For user isolation, deploy separate agents per user/team using
Kubernetes namespaces and RBAC (once the impersonation bug is fixed).

### Threat 4: OpenCode Version Drift (MITIGATED)

**Risk**: OpenCode API changes could break our proxy routing or fetch monkey-patch.

**Mitigation (implemented):**
- **Self-hosting with version pinning**: Assets are built from a specific OpenCode commit.
  Updates happen explicitly via `cd ../opencode && git checkout <tag> && make opencode-app-build`.
- **API route list**: `isOpenCodeAPIPath()` maintains a list of known OpenCode API routes.
  When upgrading OpenCode, verify this list matches the new server's routes.
- **Version drift logging**: The handler logs a warning if HTML rewriting patterns don't match
  (e.g., no `</head>` tag), making drift detectable.
- The fetch monkey-patch covers the standard `fetch()` API — only breaks if the SDK moves
  to a non-standard HTTP mechanism.

### Threat 5: SSRF via Agent Status URL (MITIGATED)

**Risk**: The reverse proxy resolves the upstream URL from `agent.Status.ServerStatus.URL`.
Although the controller constructs safe URLs via `ServerURL()` (hardcoded `http://` scheme +
`.svc.cluster.local` suffix), a compromised controller or direct status subresource patch
could inject arbitrary URLs, enabling Server-Side Request Forgery against internal services
(e.g., cloud metadata endpoints at `169.254.169.254`, internal dashboards).

**Mitigation (implemented):**
- Added `validateServerURL()` in `internal/server/handlers/common.go` that validates:
  - Scheme must be `http` (rejects `https`, `ftp`, `file`, etc.)
  - Host must end with `.svc.cluster.local` (rejects external hosts, IPs, localhost)
  - No userinfo allowed (`user:pass@host` rejected)
- Called in both `resolveAgentServerURL()` (used by web and proxy handlers) and
  `getServerURL()` — all proxy paths are covered.
- 13 unit tests in `common_test.go` covering valid URLs, wrong scheme, external hosts,
  cloud metadata endpoint, userinfo injection, etc.

### Threat 6: Proxy Timeout Bypass (MITIGATED)

**Risk**: `proxyToAgent()` uses `context.WithoutCancel()` to detach from chi's 60-second
timeout middleware, which is necessary for SSE streams. However, in fallback mode, ALL
requests (including static HTML/CSS/JS) are proxied through this path. If the upstream
OpenCode server becomes unresponsive, non-SSE proxy requests would hang indefinitely.

**Mitigation (implemented):**
- Non-API paths (static assets) in the proxy now have a 2-minute timeout via
  `context.WithTimeout()`. This prevents indefinite hangs while remaining generous enough
  for large asset transfers.
- API paths (potential SSE streams) remain unbounded — they terminate when the client
  disconnects (write errors in `httputil.ReverseProxy`).
- Added documentation comments explaining the `WithoutCancel` rationale.

### Threat 7: New Tab Window Opener (MITIGATED)

**Risk**: The "Open in new tab" feature uses `window.open()` to open the OpenCode Web UI
in a separate browser tab. In older browsers, the opened tab retains a `window.opener`
reference to the parent page, which could be used for tab-nabbing attacks (redirecting the
parent page to a phishing URL).

**Mitigation (implemented):**
- All `window.open()` calls in `WebUIPanel.tsx` now pass `'noopener,noreferrer'` as the
  third argument, severing the `window.opener` reference.
- Modern browsers (Chrome 88+, Firefox 79+, Safari 12.1+) default to `noopener`, but
  explicit specification ensures coverage for older browsers.

### Security Summary

| Threat | Severity | Status | Owner |
|--------|----------|--------|-------|
| Supply chain (app.opencode.ai) | ~~HIGH~~ MITIGATED | Self-hosting eliminates CDN dependency; CSP hardens both modes | KubeOpenCode |
| RBAC bypass (clientContextKey bug) | ~~CRITICAL~~ FIXED | Exported `ClientContextKey` from handlers package | KubeOpenCode |
| Multi-tenancy (shared sessions) | MEDIUM | By design; use namespace isolation | OpenCode |
| Version drift | ~~LOW~~ MITIGATED | Self-hosting + version pinning + drift logging | KubeOpenCode |
| SSRF via Agent Status URL | ~~HIGH~~ MITIGATED | `validateServerURL()` restricts to `http://*.svc.cluster.local` | KubeOpenCode |
| Proxy timeout bypass | ~~HIGH~~ MITIGATED | 2-min timeout for non-API proxy paths; SSE remains unbounded | KubeOpenCode |
| New tab window opener | ~~LOW~~ MITIGATED | `noopener,noreferrer` on all `window.open()` calls | KubeOpenCode |

## Consequences

### Positive

- Users can interact with server-mode agents directly from the browser — no CLI, no port-forward
- Full OpenCode development experience (sessions, file editing, terminal, permissions) in the dashboard
- Automatic feature parity — OpenCode Web UI updates are reflected immediately
- Same-origin deployment eliminates CORS complexity
- Minimal code footprint (~150 lines of Go, ~15 lines of TSX)

### Negative

- **Supply chain dependency** on `app.opencode.ai` — must trust OpenCode's CDN and release process
- **Fragile integration** — relies on OpenCode's internal URL structure and fetch-based SDK
- **No offline support** — requires connectivity to `app.opencode.ai` (unless self-hosted)
- **Maintenance burden** — must monitor OpenCode releases for breaking changes

### Neutral

- The web UI route (`/{name}/web/*`) coexists with the existing proxy route (`/{name}/proxy/*`).
  The proxy route is used by the CLI (`kubeopencode agent attach`), while the web route serves
  the browser UI. They share the same underlying proxy to the OpenCode server.

## Iframe Embedding (Chrome DevTools-style Panel)

### Design

Instead of opening the OpenCode Web UI in a new browser tab (which exposes internal API
paths and loses dashboard context), we embed it as an iframe within the AgentDetailPage.

The panel has three progressive states, inspired by Chrome DevTools:

| State | Layout | Use Case |
|-------|--------|----------|
| **Collapsed** | CTA card only | Default — no viewport cost |
| **Expanded** | `h-[70vh]` iframe | In-page IDE, still scrollable to agent details |
| **Maximized** | `fixed inset-3 z-50` | Full viewport for deep work |

- Mobile (< 1024px): Falls back to `window.open()` — iframe is unusable on small screens
- ESC key exits maximized mode (capture phase, works even with iframe focus)
- Backdrop blur + scale animation on maximize transition

### Implementation

New component: `ui/src/components/WebUIPanel.tsx`

iframe attributes:
```html
<iframe
  src="/api/v1/namespaces/{ns}/agents/{name}/web/"
  sandbox="allow-same-origin allow-scripts allow-popups allow-forms allow-modals"
  allow="clipboard-read; clipboard-write"
/>
```

## Problems Encountered and Solutions

### Problem 1: Wrong OpenCode Repository URL

**Symptom**: Docker build fails because the OpenCode repo URL doesn't exist.

**Root Cause**: OpenCode has been transferred between GitHub orgs multiple times:
- `nicepkg/opencode` (old, no longer exists)
- `sst/opencode` (301 redirect)
- `anomalyco/opencode` (current as of 2026-03)

**Solution**: Fixed Dockerfile and Makefile to use `https://github.com/anomalyco/opencode.git`.
When OpenCode changes ownership again, update `OPENCODE_APP_VERSION` or repo URL in Dockerfile.

### Problem 2: tree-sitter-bash Native Addon Compilation Fails in Alpine

**Symptom**: `bun install` fails with `gyp ERR! ... error: install script from "tree-sitter-bash" exited with 1`.

**Root Cause**: OpenCode's CLI depends on `tree-sitter-bash` which requires native compilation
(node-gyp, C compiler, Python). Alpine + bun's install scripts fail to build it.

**Solution**: Add `--ignore-scripts` to `bun install`. tree-sitter is a CLI dependency only
and is NOT needed for the Web UI build (`packages/app`):
```dockerfile
bun install --frozen-lockfile --ignore-scripts
```

### Problem 3: bun 1.3.11 Integrity Check Failure

**Symptom**: `bun install` fails with `error: Integrity check failed for tarball: @ibm/plex`.

**Root Cause**: Regression in bun 1.3.11 (the `oven/bun:1-alpine` latest tag at the time).

**Solution**: Pin bun version to a known-good release:
```dockerfile
FROM oven/bun:1.3.6-alpine AS opencode-ui-builder
```

### Problem 4: npm Can't Handle `catalog:` Protocol

**Symptom**: `npm install` fails with `EUNSUPPORTEDPROTOCOL Unsupported URL Type "catalog:": catalog:`.

**Root Cause**: OpenCode uses bun workspace's `catalog:` protocol for dependency resolution,
which is bun-specific and not supported by npm or yarn.

**Solution**: Must use bun for installing OpenCode dependencies. No npm fallback is possible.

### Problem 5: Fallback Mode Doesn't Work in Kind Clusters

**Symptom**: JS module files return `text/html` MIME type errors, "Failed to load module script"
errors in browser console.

**Root Cause**: In fallback mode, the OpenCode server proxies static assets from
`app.opencode.ai`. Pods in Kind clusters have no internet access, so these requests fail.
Additionally, Vite's dynamic `import()` for lazy-loaded JS chunks uses root-relative paths
that bypass the fetch monkey-patch (because `import()` is a language-level construct, not
`window.fetch`).

**Solution**: Use self-hosted mode (`BUILD_OPENCODE_UI=true`) which embeds all static assets
in the binary. For local development, assets can also be downloaded from the CDN:
```bash
# See CONTRIBUTING.md "Local Development Environment" for the full setup
```

### Problem 6: OpenCode Frontend Connects to localhost:4096 Instead of Proxy

**Symptom**: The OpenCode Web UI loads but shows "connecting..." or fails to reach the server.
API calls go to `http://localhost:4096` instead of through the proxy.

**Root Cause**: OpenCode's `entry.tsx` has this logic:
```javascript
const isLocalHost = () =>
  ["localhost","127.0.0.1","0.0.0.0"].includes(location.hostname)

const getCurrentUrl = () => {
  if (isLocalHost()) return `http://localhost:4096`  // ← hardcoded!
  return location.origin
}
```

When the user accesses the KubeOpenCode dashboard at `localhost:2746`, the OpenCode frontend
detects `localhost` and assumes the OpenCode server is at `localhost:4096`. The original fetch
monkey-patch only intercepted same-origin requests, so requests to `localhost:4096` (a
different port = different origin) passed through unmodified.

**Solution (two-part)**:

1. **JS bundle rewrite at serve time**: When serving `.js` files from the embedded filesystem,
   replace the string literal `"http://localhost:4096"` with the JS expression `location.origin`.
   This makes the SDK create same-origin requests from the start, eliminating cross-origin issues.
   See `serveRewrittenJS()` in `agent_web_handler.go`.

2. **Fetch monkey-patch for localhost fallback**: As a safety net, the fetch patch also intercepts
   any remaining requests to `localhost`/`127.0.0.1`/`0.0.0.0` on non-matching ports:
```javascript
function rewriteUrl(url) {
  var u = new URL(url, origin);
  // Same-origin requests (standard case)
  if (u.origin === origin && !u.pathname.startsWith(base)) {
    u.pathname = base + u.pathname;
    return u.toString();
  }
  // Cross-port localhost requests (OpenCode default: localhost:4096)
  var h = u.hostname;
  if ((h === 'localhost' || h === '127.0.0.1' || h === '0.0.0.0') && u.origin !== origin) {
    return origin + base + u.pathname + u.search;
  }
  return url;
}
```

### Problem 7: POST Requests Fail with ERR_ALPN_NEGOTIATION_FAILED

**Symptom**: Sending a message in the OpenCode Web UI fails with:
```
POST http://localhost:2746/.../prompt_async net::ERR_ALPN_NEGOTIATION_FAILED
```
GET requests (config, provider, session/status) all succeed. Only POST requests fail.
curl POST to the same URL works fine.

**Root Cause (three interacting issues)**:

1. **OpenCode SDK's custom fetch wrapper** (`packages/sdk/js/src/v2/client.ts`):
   ```typescript
   const customFetch: any = (req: any) => {
     req.timeout = false  // Bun-specific property, not a standard Web API
     return fetch(req)
   }
   ```
   The SDK mutates Request objects by setting the non-standard `timeout` property before
   passing them to `fetch()`. This is designed for Bun (server-side), not browsers.

2. **Request body becomes ReadableStream**: When our fetch monkey-patch intercepts a Request
   object and extracts its `.body` property, it gets a `ReadableStream` (the Web API wraps
   all bodies as streams internally). Passing a `ReadableStream` body to `fetch()` requires
   `duplex: 'half'` in the init options.

3. **`duplex: 'half'` triggers HTTP/2 negotiation**: Setting `duplex: 'half'` tells the browser
   to use half-duplex streaming, which triggers HTTP/2 ALPN (Application-Layer Protocol
   Negotiation). Since kubectl port-forward only supports HTTP/1.1, the negotiation fails
   with `ERR_ALPN_NEGOTIATION_FAILED`.

**Why curl works**: curl doesn't use the browser's fetch API, doesn't perform ALPN negotiation
on plain HTTP, and uses HTTP/1.1 directly.

**Why GET requests work**: GET requests have no body, so the ReadableStream/duplex issue
doesn't arise.

**Solution**: Decompose the Request object into a clean string URL + init dict, and critically,
**read the body as text** using `req.text()` before passing to fetch. This converts the
ReadableStream back to a plain string, eliminating the need for `duplex: 'half'` and avoiding
HTTP/2 negotiation entirely:

```javascript
window.fetch = function(input, init) {
  if (typeof input === 'string') {
    input = rewriteUrl(input);
    return _fetch.call(this, input, init);
  }
  if (input instanceof Request) {
    var newUrl = rewriteUrl(input.url);
    if (newUrl !== input.url) {
      var req = input;
      var s = req.signal;
      // Read body as text to avoid ReadableStream + duplex + ALPN issues
      return req.text().then(function(body) {
        var o = {method: req.method, headers: req.headers, credentials: req.credentials};
        if (s) o.signal = s;
        if (body) o.body = body;
        return _fetch.call(this, newUrl, o);
      }.bind(this));
    }
  }
  return _fetch.call(this, input, init);
};
```

**Key design decisions**:
- `req.text()` converts ReadableStream → string, so no `duplex` needed
- Only standard RequestInit properties are passed (method, headers, body, credentials, signal)
- Non-standard properties (like `timeout`) injected by the SDK are discarded
- The promise chain is transparent — callers still get a `Promise<Response>` back

**Failed approaches that led to the solution**:

| Attempt | Result |
|---------|--------|
| `new Request(newUrl, input)` — copy Request object | ERR_ALPN_NEGOTIATION_FAILED (polluted Request) |
| Decompose to init dict with `input.body` (ReadableStream) | "duplex member must be specified" error |
| Add `duplex: 'half'` to init | ERR_ALPN_NEGOTIATION_FAILED (triggers HTTP/2) |
| **`req.text()` to read body as string** | **Success — no streaming, no ALPN** |

### Problem 8: CSP Blocks External Fonts

**Symptom**: CSP violation for `r2cdn.perplexity.ai` fonts in browser console.

**Root Cause**: OpenCode Web UI references external fonts from Perplexity's CDN, which
our `font-src 'self' data:` CSP blocks.

**Solution**: Self-hosted mode serves all assets locally (fonts included in the build output),
so no external font requests are needed. This is only an issue in fallback mode.

## Build Configuration

### BUILD_OPENCODE_UI Flag

| Mode | Value | Docker Stage | Behavior |
|------|-------|-------------|----------|
| CI/Production | `true` | Clones OpenCode, runs `bun install && bun run build` | Self-hosted, no CDN dependency |
| Local Dev | `false` (default) | Creates empty `dist/.gitkeep` | Falls back to CDN proxy (or broken in Kind) |

The Makefile defaults to `BUILD_OPENCODE_UI=false` for local dev. CI workflows
(`.github/workflows/*.yaml`) set `BUILD_OPENCODE_UI=true`.

### OpenCode Version Pinning

The OpenCode Web UI version is pinned via `OPENCODE_APP_VERSION` (default: `v1.3.2`).
This is used both in the Dockerfile (git clone tag) and Makefile (local build).

To upgrade:
1. Update `OPENCODE_APP_VERSION` in Dockerfile and Makefile
2. Verify `openCodeAPIRoutes` in `agent_web_handler.go` matches the new API surface
3. Test the fetch monkey-patch still works with the new frontend bundle
4. Rebuild: `make docker-build BUILD_OPENCODE_UI=true`

## Files

| File | Description |
|------|-------------|
| `internal/server/handlers/agent_web_handler.go` | Self-hosted + fallback proxy handler, fetch patch |
| `internal/server/handlers/agent_web_handler_test.go` | Unit tests for HTML rewriting, API path detection |
| `internal/server/handlers/common.go` | Shared helpers: URL resolution, `validateServerURL()` |
| `internal/server/handlers/common_test.go` | Unit tests for server URL validation (SSRF prevention) |
| `internal/opencode-app/embed.go` | Embedded OpenCode Web UI assets |
| `internal/opencode-app/dist/` | Built assets (populated by `make opencode-app-build`) |
| `internal/server/server.go` | Route registration + RBAC fix (ClientContextKey) |
| `internal/server/handlers/agent_handler.go` | Exported ClientContextKey type |
| `ui/src/components/WebUIPanel.tsx` | Chrome DevTools-style iframe panel |
| `ui/src/pages/AgentDetailPage.tsx` | WebUIPanel integration in agent detail view |
| `ui/tailwind.config.js` | `panel-maximize` animation keyframe |
| `Makefile` | `opencode-app-build`, `opencode-app-clean`, `BUILD_OPENCODE_UI` flag |
| `Dockerfile` | Multi-stage build with OpenCode Web UI builder stage |
| `.github/workflows/*.yaml` | CI workflows with `BUILD_OPENCODE_UI=true` |
| `CONTRIBUTING.md` | Local development guide (Kind cluster setup, troubleshooting) |
