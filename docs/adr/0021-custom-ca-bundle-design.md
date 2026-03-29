# ADR 0021: Custom CA Bundle Design for Init Containers

## Status

Accepted

## Context

KubeOpenCode's init containers (`git-init`, `url-fetch`) perform HTTPS operations (git clone, URL fetch) but have no mechanism to trust custom Certificate Authorities. Enterprise environments commonly use private CAs for internal Git servers, artifact registries, and API endpoints. Without custom CA support, users must either:

1. Disable TLS verification (`InsecureSkipTLSVerify`) — insecure and not recommended
2. Bake custom CAs into container images — fragile, not portable, breaks image reuse
3. Use the existing `credentials` mechanism to manually mount CAs — works but has no automatic integration with git/curl/Go HTTP

This was reported in [#105](https://github.com/kubeopencode/kubeopencode/issues/105) by a user whose private Git server uses a custom CA from trust-manager.

## Decision

### Single `caBundle` field on AgentSpec

We add a single `caBundle` field (not a list) to `AgentSpec`. The field references a ConfigMap or Secret containing a PEM-encoded CA bundle.

**Why single, not `[]CABundleConfig`?**

- PEM format natively supports multiple certificates in a single file. Users concatenate multiple CAs into one bundle — this is standard practice.
- cert-manager's trust-manager `Bundle` resource already handles multi-source CA aggregation and outputs a single ConfigMap. KubeOpenCode's default key (`ca-bundle.crt`) matches trust-manager's convention.
- A list would add API complexity, require merging logic in the controller, and create ordering/precedence questions — all for a problem already solved by PEM concatenation.

### System CA preservation (concatenation, not replacement)

`git-init` concatenates the custom CA with the system CA bundle before setting `GIT_SSL_CAINFO`. `url-fetch` appends to Go's `x509.SystemCertPool()`.

**Why not just set `GIT_SSL_CAINFO` to the custom CA file directly?**

Git's `GIT_SSL_CAINFO` **replaces** (not supplements) the default CA trust store. If only the custom CA is provided, git operations against public servers (e.g., `github.com`) would fail with certificate verification errors. The same Task might need to clone from both a private internal server and a public GitHub repository.

The concatenation approach (`system CAs + custom CA → combined file`) ensures both public and private HTTPS work simultaneously. This requires detecting the system CA bundle path across Linux distributions (Debian, RHEL, Alpine, OpenSUSE), with a graceful fallback to custom CA only if no system bundle is found.

Go's `x509.SystemCertPool()` + `AppendCertsFromPEM()` provides this behavior natively for `url-fetch`, without needing file concatenation.

### Mount to all containers

The CA volume is mounted into **all** init containers and the worker container, including `opencode-init` (which only copies a binary and doesn't need it). This is simpler than selectively skipping containers, has negligible overhead (read-only projected volume), and is future-proof if any container's behavior changes.

### Agent-level only (not cluster-wide, for now)

The `caBundle` field is on `AgentSpec`, not on `KubeOpenCodeConfig`. This means each Agent must configure its own CA bundle. Cluster-wide CA configuration (via `KubeOpenCodeConfig`) is a natural future extension but was deferred to keep the initial implementation simple. Users who need the same CA on multiple Agents can use Helm values or Kustomize overlays.

## Consequences

### Positive

- Users can clone from private HTTPS Git servers without disabling TLS verification
- Compatible with cert-manager trust-manager out of the box
- Public HTTPS continues working (system CAs preserved)
- No API complexity from list-based configuration

### Negative

- Users with multiple separate CA sources must combine them into a single PEM bundle (or use trust-manager)
- No cluster-wide default — must configure per-Agent (mitigated by Helm/Kustomize)
- `opencode-init` container gets an unnecessary (but harmless) volume mount

### Future Considerations

- Add `caBundle` to `KubeOpenCodeConfig` for cluster-wide default, with Agent-level override
- Consider `caBundle` on `TaskSpec` for per-task CA configuration
