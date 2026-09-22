# ADR 0042: GitHub App Authentication for Git Operations

## Status

Proposed

## Date

2026-09-20

## Context

KubeOpenCode clones Git repositories in three places:

- **Git contexts** — `git-init` init containers (and `git-sync` sidecars when
  `sync.policy: HotReload`) clone repository content for the agent workspace.
- **Skill sources** — the same `git-init` mechanism clones skill repositories
  (e.g. an organization's internal skill catalog) mounted under `/skills`.

Today `setupAuth()` in `cmd/kubeopencode/git_init.go` supports exactly two
authentication methods, selected by which keys exist in the referenced Secret:

| Method | Secret keys |
|---|---|
| HTTPS username/password | `username` + `password` (password may be a PAT) |
| SSH key | `ssh-privatekey` (+ optional `ssh-known-hosts`) |

Neither is a good fit for organization-wide automation:

- **PATs** are long-lived, tied to a human account, and grant broad access. They
  must be rotated manually and are a standing exfiltration risk.
- **SSH deploy keys** are scoped to a single repository. An organization with
  many repositories and a shared skill catalog must provision and rotate one key
  per repository, and each key is separately registered on the GitHub side.
  Deploy keys are also easy to mis-register as (rejected) account keys.

GitHub Apps solve this: a single App installation covers every repository in the
organization, permissions are explicitly scoped (e.g. *Contents: Read*), and the
**installation access token is short-lived** (one hour) and minted on demand from
a private key. This is GitHub's recommended model for automation and is already
the model KubeOpenCode's executor image uses for the worker container
(`agents/devbox/scripts/github-*-app.sh`).

The gap is that those scripts are baked into the *executor* image, while
`git-init` runs from the *system* image (the unified `kubeopencode` binary) and
has no equivalent logic.

## Decision

Add GitHub App authentication to the unified binary's Git subcommands
(`git-init` and `git-sync`), reusing the app-* credentials from the same Secret
already referenced by `secretRef`.

### Credentials

Three optional Secret keys, mapped to environment variables by the controller:

| Secret key | Env var | Description |
|---|---|---|
| `app-id` | `GH_APP_ID` | GitHub App ID |
| `app-installation-id` | `GH_APP_INSTALLATION_ID` | Installation ID for the org/account |
| `app-private-key` | `GH_APP_PRIVATE_KEY` | PEM private key (PKCS#1 or PKCS#8), as content or a file path |

Names are deliberately generic (not `GITHUB_APP_*`) so the same Secret and
variable convention works across organizations and can be reused by other
tooling; they mirror the existing devbox scripts' `GH_APP_*` prefix, which
cannot use the reserved `GITHUB_*` prefix.

An optional `GITHUB_API_URL` (default `https://api.github.com`) targets GitHub
Enterprise Server.

### Behavior

- `setupAuth()` mints an installation access token at clone time and writes it to
  the standard git credential store as `x-access-token:<token>`, then reuses the
  existing HTTPS code path. The token is **not** persisted in the Secret.
- **App credentials take precedence** over `username`/`password` and
  `ssh-privatekey`, so a Secret migrating from a PAT to an App does not need the
  old keys removed first.
- **SSH repository URLs are rewritten to HTTPS** when App auth is active
  (`git@github.com:org/repo.git` → `https://github.com/org/repo.git`), since
  installation tokens are HTTPS credentials. SSH host-key handling is therefore
  irrelevant for App-authenticated clones.
- **`git-sync` re-mints before each sync cycle.** Installation tokens expire
  after one hour, and a sync sidecar can run for far longer; a token minted only
  at startup would fail every sync after the first hour.
- The token is cached in memory and refreshed shortly before expiry, so a single
  `git-init` process does not mint more than one token.
- `git-init` continues to delete the credential file after cloning
  (`cleanupCredentials`), now also in the App-auth case.

### Implementation

- `cmd/kubeopencode/github_app.go` — new file: credential loading, JWT signing
  (RS256 via `crypto/rsa` + `crypto/sha256`, no new dependency), token exchange,
  caching, and URL rewriting.
- `cmd/kubeopencode/git_init.go` / `git_sync.go` — authenticate via the App when
  configured.
- `internal/controller/pod_builder.go` — `buildGitCredentialEnvVars` maps the
  three new Secret keys, all marked optional, so existing Secrets and the
  username/password and SSH paths are unaffected.

No CRD schema change is required: `GitSecretReference` already carries only a
Secret name, and the auth method is selected by which keys exist in that Secret.

## Consequences

### Positive

- One credential for the whole organization; no per-repository deploy keys, no
  long-lived PATs.
- Credentials are short-lived and minted at runtime, reducing the blast radius of
  a leaked Secret (the private key still requires rotation, but tokens cannot be
  replayed after an hour).
- Skill sources can use the same App secret as Git contexts, removing the
  per-repository SSH key workaround.
- No new Go dependency: JWT signing uses the standard library, keeping vendoring
  and the build unchanged.

### Negative

- Incomplete App credentials (e.g. `app-id` set without `app-private-key`) is a
  hard error rather than a silent fallback to anonymous clone. This is
  intentional — a partially-configured Secret is an operator mistake, and an
  unauthenticated clone would obscure it.
- App auth is HTTPS-only. Repositories that must be cloned over SSH with
  host-key verification keep using `ssh-privatekey`; App auth deliberately
  bypasses SSH.
- `git-sync` holds a token in memory for the process lifetime. It is refreshed,
  not revoked on exit; the one-hour expiry bounds exposure.
