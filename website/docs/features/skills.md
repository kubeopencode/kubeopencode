# Skills

Skills are reusable AI agent capabilities defined as SKILL.md files (Markdown with YAML frontmatter).

Skills are semantically different from Contexts:
- **Contexts** provide knowledge ("what the agent knows")
- **Skills** provide capabilities ("what the agent can do")

## Two Ways to Use Skills

KubeOpenCode supports two complementary approaches for loading skills:

| Approach | How | Best for |
|----------|-----|----------|
| **File-based (workspace)** | Place skills in your project repo's `.opencode/skills/` or `.agents/skills/` directory | Team-owned skills that live alongside your code |
| **CRD `skills[]` field** | Reference external Git repositories in the Agent spec | Shared/public skills from other repositories |

### File-based skills (workspace directory)

If your project repository already contains skills in the standard OpenCode directory structure, they are **automatically discovered** when OpenCode starts — no additional CRD configuration needed. This works identically to running OpenCode locally.

For example, the [kubeopencode-agent](https://github.com/kubeopencode/kubeopencode-agent) repository has skills defined at `.opencode/skills/`:

```
kubeopencode-agent/
└── .opencode/
    └── skills/
        ├── github-respond/SKILL.md
        ├── opencode-update/SKILL.md
        ├── pr-review/SKILL.md
        ├── slack-respond/SKILL.md
        └── tiny-refactor/SKILL.md
```

Mount the repo to your workspace root via a Git context, and all skills are loaded automatically:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  workspaceDir: /workspace
  contexts:
  - name: my-project
    type: Git
    git:
      repository: https://github.com/kubeopencode/kubeopencode-agent.git
      ref: main
    mountPath: .    # mounts to /workspace
```

Since the repo is mounted at the workspace root (`/workspace`), the `.opencode/skills/` directory ends up at `/workspace/.opencode/skills/` — exactly where OpenCode expects it. All standard OpenCode configurations (`.opencode/`, `.agents/`, `AGENTS.md`, etc.) work the same way.

### External skills (CRD `skills[]` field)

For skills maintained in **separate repositories** — such as community skills, organization-wide shared skills, or third-party skill collections — use the `skills[]` field in the Agent spec. The controller clones these repositories and injects them into OpenCode's skill discovery paths.

Both approaches work side by side. You can have project-local skills in your workspace **and** reference external skills via the CRD — they are all available to the agent.

## Basic Usage

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: skilled-agent
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  skills:
  - name: official-skills
    git:
      repository: https://github.com/anthropics/skills.git
      ref: main
      path: skills/
      names:
      - frontend-design
      - webapp-testing
```

## Select Specific Skills

Use the `names` field to include only specific skills from a repository. If omitted, all skills under `path` are included:

```yaml
skills:
- name: team-skills
  git:
    repository: https://github.com/my-org/ai-skills.git
    path: engineering/
    names:
    - code-review
    - testing-strategy
```

## Private Repositories

Use `secretRef` for authentication (same Secret format as Git contexts). The
Secret may contain a GitHub App credential, an HTTPS username/password (a PAT),
or an SSH private key — the method is selected by which keys exist:

```yaml
skills:
- name: internal-skills
  git:
    repository: https://github.com/my-org/private-skills.git
    ref: v2.0.0
    secretRef:
      name: github-pat
```

| Secret keys | Method |
|---|---|
| `app-id`, `app-installation-id`, `app-private-key` | GitHub App — an installation access token is minted at runtime. **Preferred**: short-lived, scoped to the App's permissions, and one App covers every repo in the org. |
| `username`, `password` | HTTPS username/password (token-based auth; `password` may be a PAT). |
| `ssh-privatekey`, optional `ssh-known-hosts` | SSH private key. |

When GitHub App credentials are present they take precedence. App auth is
HTTPS-only, so an SSH repository URL (`git@github.com:org/repo.git`) is rewritten
to HTTPS automatically. This is the recommended way to clone skills from a
private organization repo — see
[Security - GitHub App Authentication](../security.md#github-app-authentication).

```bash
# GitHub App (recommended for organization-wide skill catalogs)
kubectl create secret generic github-app-credentials \
  --from-literal=app-id=123456 \
  --from-literal=app-installation-id=7891011 \
  --from-file=app-private-key=$HOME/.ssh/github-app.pem
```

```yaml
skills:
- name: internal-skills
  git:
    repository: https://github.com/my-org/private-skills.git
    ref: master
    secretRef:
      name: github-app-credentials
```

## How It Works

1. The controller clones skill Git repositories via `git-init` init containers
2. Skills are mounted at `/skills/{source-name}/` in the agent pod
3. The controller auto-injects `skills.paths` into `opencode.json`
4. OpenCode discovers SKILL.md files and makes them available as slash commands

## Keeping Skills Up to Date

OpenCode discovers skills **once, when its server instance starts**, and caches
them. For a long-running Agent that means a change pushed to a skill repository
is invisible until the Pod restarts — even though the cloned files on disk are
updated.

Enable `sync` to close that gap. A `git-sync` sidecar polls the repository and,
when it detects a new commit, updates the files **and asks the server to
re-scan**, so the change takes effect without a restart:

```yaml
skills:
- name: org-skills
  git:
    repository: https://github.com/my-org/standards-skills.git
    path: .claude/skills
    sync:
      enabled: true
      interval: 15m   # default for skills
```

Notes:

- **Only `HotReload` is supported.** `Rollout` is rejected for skills, since it
  would compare remote refs in the controller rather than reloading in place.
- **Reloads wait for an idle agent.** Re-scanning briefly re-initializes the
  server instance, which would abort a turn that is mid-flight. If a session is
  busy the reload is deferred to the next cycle, so a continuously busy agent
  picks up the change at its next idle window. The file update itself still
  happens immediately.
- **`sync` applies to long-running Agents only.** Task pods are ephemeral and
  always start fresh, so they ignore it.
- **Skills selected with `names` are mounted from fixed subpaths.** Edits to
  those skills are picked up, but a newly added skill directory is not mounted
  until the Pod restarts. Omit `names` to mount the whole directory and pick up
  additions without a restart.

Git contexts can do the same with `sync.reload: true` — see
[Git Auto-Sync](git-auto-sync.md).

## Skills in Templates

Skills can be defined in AgentTemplates and inherited by Agents. Agent-level skills replace template-level skills (same merge strategy as contexts):

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: AgentTemplate
metadata:
  name: base-template
spec:
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  skills:
  - name: org-standards
    git:
      repository: https://github.com/my-org/standards-skills.git
```

## Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | (required) | Unique identifier for this skill source |
| `git.repository` | string | (required) | Git URL (https://, http://, or git@) |
| `git.ref` | string | HEAD | Branch, tag, or commit SHA |
| `git.path` | string | (root) | Base directory in repo where skills are located |
| `git.names` | []string | (all) | Specific skill directories to include |
| `git.depth` | int | 1 | Clone depth (1=shallow, 0=full) |
| `git.recurseSubmodules` | bool | false | Clone submodules recursively |
| `git.secretRef.name` | string | - | Secret for Git authentication. Keys: `app-id`/`app-installation-id`/`app-private-key` (GitHub App, preferred), `username`/`password` (HTTPS), or `ssh-privatekey` (SSH) |
| `git.sync.enabled` | bool | false | Poll the repository and reload the server on change (Agent servers only) |
| `git.sync.interval` | duration | 15m | Polling interval for skill sync |
| `git.sync.policy` | string | HotReload | Only `HotReload` is valid for skills |
