# Development Guidelines for KubeOpenCode

This document provides guidelines for AI assistants working on the KubeOpenCode project.

## Project Overview

> **Disclaimer**: This project uses [OpenCode](https://opencode.ai) as its primary AI coding tool. KubeOpenCode is not built by or affiliated with the OpenCode team.

KubeOpenCode brings Agentic AI capabilities into the Kubernetes ecosystem. By leveraging Kubernetes, it enables AI agents to be deployed as services, run in isolated virtual environments, and integrate with enterprise management and governance frameworks.

> **IMPORTANT FOR AI ASSISTANTS**: The OpenCode project source code is located at `../opencode/` (sibling directory to kubeopencode). Always prioritize searching the local OpenCode codebase first before using web search.

**Key Technologies:** Go 1.25, Kubernetes CRDs, Controller Runtime (kubebuilder), Helm

**Architecture Philosophy:**
- No external dependencies (no PostgreSQL, Redis) — Kubernetes-native (etcd for state, Pods for execution)
- Simple API: Task (WHAT to do) + Agent (HOW to execute)
- Use Helm/Kustomize for batch operations (multiple Tasks)
- Event-driven triggers delegated to [Argo Events](https://argoproj.github.io/argo-events/)

**Unified Binary:** Single container image (`ghcr.io/kubeopencode/kubeopencode`) with subcommands: `controller`, `git-init`, `context-init`, `url-fetch`. Image constant: `internal/controller/pod_builder.go` → `DefaultKubeOpenCodeImage`.

## Core Concepts

### Resource Hierarchy

1. **Task** - Single task execution (the primary API)
2. **CronTask** - Scheduled/recurring task execution (creates Tasks on cron schedule)
3. **Agent** - AI agent configuration (HOW to execute)
4. **AgentTemplate** - Reusable base configuration for Agents (optional)
5. **KubeOpenCodeConfig** - Cluster-scoped system-level configuration (optional, singleton named `cluster`)

### Important Design Decisions

- **Agent** (not KubeOpenCodeConfig) - Stable, project-independent naming
- **Agent = running instance** - Always creates Deployment + Service (no Pod mode vs Server mode)
- **agentRef or templateRef** - Task must reference exactly one: an Agent (persistent) or AgentTemplate (ephemeral)
- **No Batch/BatchRun** - Use Helm/Kustomize (Kubernetes-native approach)
- **CronTask** - Scheduled task execution (CronTask:Task = CronJob:Job). Creates Tasks on cron schedule with concurrency policy, maxRetainedTasks (blocks creation, does NOT delete), and manual trigger support
- **Two-container pattern**: Init container (`agentImage`) copies OpenCode binary to `/tools`, Worker container (`executorImage`) runs the server

### Context System

Tasks and Agents use inline **ContextItem** for additional context. Types: Text, ConfigMap, Git, Runtime, URL.

Key behaviors:
- Empty `mountPath` → content written to `${WORKSPACE_DIR}/.kubeopencode/context.md` with XML tags
- Relative paths prefixed with `workspaceDir`; absolute paths used as-is

> See `website/docs/features.md` for detailed context examples and field reference.

### Agent Configuration (Summary)

Key Agent spec fields: `templateRef`, `profile`, `agentImage`, `executorImage`, `attachImage`, `command` (optional, has default), `workspaceDir` (required), `port` (default: 4096), `extraPorts` (additional Service/Deployment ports for DinD, VS Code, etc.), `persistence`, `suspend`, `standby` (automatic suspend/resume lifecycle), `contexts`, `skills` (external SKILL.md sources from Git repos, auto-injects `skills.paths` into OpenCode config), `plugins` (OpenCode plugins installed via npm; controller creates a plugin-init container that runs `npm install` into a shared `/plugins` volume, loads via `file:///plugins/node_modules/<package>` path into OpenCode config's `plugin` array — executor container does not need npm), `config` (inline YAML/JSON object → `/tools/opencode.json`), `credentials`, `caBundle`, `proxy`, `imagePullSecrets`, `podSpec`, `serviceAccountName`, `maxConcurrentTasks`, `quota`, `share` (shareable terminal link).

> See `website/docs/features.md` for detailed YAML examples of Agent configuration, proxy, credentials, concurrency, quota, persistence, and Git auto-sync.

**Docker in Docker (DinD)**: The devbox executor image includes Docker CLI. Full DinD (build + run containers) is supported via `podSpec.runtimeClassName` (Sysbox recommended) or `podSpec.securityContext` (privileged/rootless). No code changes needed — existing `AgentPodSpec` fields are sufficient. See `website/docs/use-cases/docker-in-docker.md`.

**Git Auto-Sync**: Git contexts support `sync` field with `enabled`, `interval`, and `policy` (HotReload or Rollout). HotReload uses a `kubeopencode git-sync` sidecar for in-place updates. Rollout triggers Deployment rolling update with Task protection (waits for active Tasks, 1h safety timeout). See ADR 0027.

### AgentTemplate

Reusable blueprint serving two roles: (1) base configuration for Agents via `spec.templateRef.name`, (2) blueprint for ephemeral Tasks via `Task.spec.templateRef`. Merge strategy: Agent wins for scalars; Agent **replaces** template for lists. Agent-only fields: `profile`, `port`, `persistence`, `suspend`, `standby`, `templateRef`.

> See `website/docs/architecture.md` for AgentTemplate spec fields and merge details.

### Agent Lifecycle

Agent always creates a Deployment + Service running `opencode serve`. Supports persistence (sessions/workspace PVCs), manual suspend/resume (`suspend`), and standby mode (`standby` auto-suspends after idle, auto-resumes when new task arrives). Standby is connection-aware: active web terminal or CLI attach sessions prevent auto-suspend via annotation heartbeat (`kubeopencode.io/last-connection-active`). See ADR 0028.

> See `website/docs/features.md` for Agent setup, persistence, suspend/resume, and comparison table.

### Agent Share Link

Agents support shareable terminal links for users without Kubernetes credentials. When `spec.share.enabled: true`, the controller generates a cryptographic token (32-byte, base64url) stored in a Secret `{agent-name}-share`. The server exposes a standalone terminal page at `/s/{token}` outside the auth middleware. Options: `expiresAt` (expiry time), `allowedIPs` (CIDR allowlist), `readOnly` (view-only terminal). CLI: `kubeoc agent share/unshare`. See ADR 0033.

### Task Stop

Stop running Tasks via annotation: `kubectl annotate task <name> kubeopencode.io/stop=true`. Prefer this over `kubectl delete task`. Logs are lost when stopped.

### Task Cleanup

Automatic cleanup via `KubeOpenCodeConfig` (cluster-scoped singleton named `cluster`): `ttlSecondsAfterFinished` and/or `maxRetainedTasks` per namespace.

### CronTask (Scheduled Execution)

CronTask creates Tasks on a cron schedule. Key fields: `schedule` (cron expression), `timeZone`, `concurrencyPolicy` (Allow/Forbid/Replace, default Forbid), `maxRetainedTasks` (blocks creation when reached, default 10), `suspend`, `taskTemplate`.

- Manual trigger: annotation `kubeopencode.io/trigger=true` or API `POST /trigger`
- CronTask does NOT delete Tasks — cleanup is handled by global KubeOpenCodeConfig
- Generated Tasks are named `{crontask-name}-{unix-timestamp}` with label `kubeopencode.io/crontask={name}`

> See `docs/adr/0025-crontask.md` for design decisions.

## Code Standards

### File Headers

All Go files must include:
```go
// Copyright Contributors to the KubeOpenCode project
```

### Naming Conventions

- **API Resources**: Semantic names (`Agent`, not `KubeOpenCodeConfig`)
- **Go Code**: Standard Go conventions (PascalCase exported, camelCase unexported)
- **CRD Group**: `kubeopencode.io`, API Version: `v1alpha1`

### Code Comments

- Write all comments in English
- Use godoc format for exported types/functions

## Development Workflow

### Building and Testing

```bash
make build          # Build controller
make test           # Run unit tests
make lint           # Run linter
make update         # Regenerate CRDs and deepcopy
make verify         # Verify generated code is up to date
make run            # Run controller locally
make fmt            # Format code
```

### E2E Testing

> **CRITICAL**: E2E commands are for **local Kind clusters only**. For remote clusters, use `docker-buildx` + `kubectl rollout restart`.

> **CRITICAL**: NEVER run `make e2e-test` alone. Always execute all three steps:

```bash
make e2e-teardown   # Step 1: Clean up existing Kind cluster
make e2e-setup      # Step 2: Setup complete e2e environment
make e2e-test       # Step 3: Run e2e tests
```

For iterative e2e testing (after full flow ran once in this session):
```bash
make e2e-reload     # Rebuild + reload controller image + run e2e-test
```

> **CRITICAL**: `e2e-reload` is ONLY for e2e testing (hardcoded Kind cluster `kubeopencode-e2e`). For local-dev, use:
> ```bash
> make local-dev-reload
> ```
> This rebuilds the image (with both `:VERSION` and `:latest` tags), loads into Kind, and restarts all deployments. Never manually run `docker-build` + `kind load` for local-dev — use `local-dev-reload` to avoid tag mismatches.
>
> **Known issue: Kind image cache stale after reload.** `make local-dev-reload` does `kind load docker-image` + `kubectl rollout restart`, but Kind nodes cache images by digest. If the `:latest` tag digest on the node matches, `kind load` skips the actual load. The restarted Pod then pulls the stale cached image. **Workaround:**
> ```bash
> # 1. Tag with a unique name to force Kind to re-tag on the node
> docker tag ghcr.io/kubeopencode/kubeopencode:latest ghcr.io/kubeopencode/kubeopencode:dev-$(date +%s)
> kind load docker-image ghcr.io/kubeopencode/kubeopencode:dev-$(date +%s) --name kubeopencode
> # 2. Patch the deployment to use the new tag
> kubectl set image deployment/kubeopencode-controller controller=ghcr.io/kubeopencode/kubeopencode:dev-$(date +%s) -n kubeopencode-system
> # 3. Verify with: kubectl exec -n kubeopencode-system deployment/kubeopencode-controller -- /kubeopencode version
> ```
> Also remember to `kubectl apply -f deploy/crds/` if CRD schemas changed (new fields won't be accepted until CRDs are updated).

### Docker, Registry, and Deployment

```bash
make docker-build   # Build image (local)
make docker-push    # Push image
make docker-buildx  # Multi-arch build+push (recommended for remote clusters)
```

> **CRITICAL**: Always deploy to `kubeopencode-system` namespace.

### UI Server

The UI server listens on port **2746** (not 8080). To access:

```bash
kubectl port-forward -n kubeopencode-system svc/kubeopencode-server 2746:2746
# Then open http://localhost:2746
```

Helm flag `--set server.enabled=true` deploys the UI server. Port is configured in `charts/kubeopencode/values.yaml` (`server.port: 2746`).

### Agent Images

Two-container pattern: `opencode` (init container) + `devbox` (executor). Located in `agents/`.

```bash
make agent-build AGENT=opencode    # Local (Kind)
make agent-buildx AGENT=opencode   # Remote (multi-arch)
```

> See `docs/agent-images.md` for image resolution, customization, and backward compatibility.

## Key Files and Directories

```
api/v1alpha1/             # CRD type definitions (types.go, agenttemplate_types.go)
cmd/kubeopencode/         # Unified binary (controller, git-init, context-init, url-fetch)
cmd/kubeoc/               # CLI client (kubeoc get agents, kubeoc agent attach, etc.)
internal/controller/      # Reconcilers (task, agent, agenttemplate, pod_builder, context_resolver, template_merge)
deploy/crds/              # Generated CRD YAMLs
deploy/local-dev/         # Local development environment
charts/kubeopencode/      # Helm chart
agents/                   # Agent images (opencode/, devbox/)
e2e/                      # E2E tests
docs/                     # Documentation
```

## Making Changes

### New Feature Checklist

> **CRITICAL**: When planning a new feature, go through EVERY item below and mark "needed/not needed" with reason BEFORE writing any code. This prevents missing critical steps mid-development.

**1. API Layer**
- [ ] `api/v1alpha1/` — New/modified types with kubebuilder markers
- [ ] `api/v1alpha1/register.go` — Register new resource type
- [ ] Run `make update` — Regenerate deepcopy + CRDs (`deploy/crds/` and `charts/kubeopencode/crds/`)

**2. Controller Layer**
- [ ] `internal/controller/` — New/modified reconciler
- [ ] `cmd/kubeopencode/controller.go` — Register new controller
- [ ] `internal/controller/pod_builder.go` — If Pod construction changes
- [ ] `internal/controller/template_merge.go` — If template merge logic changes

**3. Server / API Handler Layer**
- [ ] `internal/server/server.go` — Register new routes
- [ ] `internal/server/handlers/` — New handler file
- [ ] `internal/server/types/types.go` — New request/response types

**4. CLI Layer (`kubeoc`)**
- [ ] `cmd/kubeoc/` — New/modified subcommands (e.g., `kubeoc get <resource>`, `kubeoc <resource> <action>`)
- [ ] `cmd/kubeoc/main.go` — Register new subcommand if adding a top-level command

**5. UI Layer**
- [ ] `ui/src/pages/` — New page components (List / Detail / Create)
- [ ] `ui/src/App.tsx` — Register routes
- [ ] `ui/src/components/Layout.tsx` — Sidebar navigation entry
- [ ] `ui/src/api/client.ts` — API client methods

**6. UI Mock Data**
- [ ] `ui/src/mocks/data.ts` — Mock data for new resource
- [ ] `ui/src/mocks/handlers.ts` — MSW handlers for new API endpoints

**7. Tests**
- [ ] Unit tests — Controller logic (`make test`)
- [ ] Integration tests — `internal/controller/*_test.go` (`//go:build integration`), update `suite_test.go` scheme if needed
- [ ] E2E tests — `e2e/*_test.go`, update `suite_test.go` if needed
- [ ] UI tests — `ui/src/**/__tests__/`

**8. Helm Chart (especially RBAC!)**
- [ ] `charts/kubeopencode/templates/rbac/controller-clusterrole.yaml` — Controller permissions
- [ ] `charts/kubeopencode/templates/rbac/web-user-clusterrole.yaml` — UI user permissions
- [ ] `charts/kubeopencode/templates/server/clusterrole.yaml` — **Server permissions (most commonly missed!)**
- [ ] `charts/kubeopencode/values.yaml` — New config values if needed

**9. Documentation**
- [ ] `website/docs/architecture.md` — Architecture, resource relationships
- [ ] `website/docs/features.md` — Feature details, YAML examples
- [ ] `docs/adr/` — ADR for design decisions (recommended for new features)
- [ ] `README.md` — User-facing feature summary
- [ ] `AGENTS.md` — Update AI assistant guidelines
- [ ] `deploy/local-dev/` — Example YAML for local testing

**10. Website**
- [ ] `website/docs/` — Sync with `docs/` (architecture.md, features.md, etc.)

**11. Build Verification**
- [ ] `make update && make build && make test && make lint && make verify`
- [ ] `go.mod` / `go.sum` / `vendor/` — If new dependencies added

### API Changes (Add/Update/Delete Fields)

1. Update `api/v1alpha1/types.go` with kubebuilder markers
2. Run `make update` then `make verify`
3. **Update documentation** in `website/docs/architecture.md`
4. **Update integration tests** in `internal/controller/*_test.go`
5. **Update E2E tests** in `e2e/`

> **IMPORTANT**: API changes are incomplete without documentation, integration tests, AND E2E tests.

### Modifying Controllers

1. Update logic in `internal/controller/`
2. Ensure proper error handling, logging, and status conditions
3. Test with `make run` or `make e2e-setup`

### Adding or Modifying Agents

1. Add/update agent files in `agents/<agent-name>/`
2. **Update GitHub workflow** `.github/workflows/push.yaml` (path filter + build job)
3. Update `agents/README.md` if adding a new agent
4. Test with `make agent-build AGENT=<name>`

> **IMPORTANT**: Agent changes are incomplete without updating the CI workflow.

### Updating CRDs

```bash
make update-crds    # Generates to deploy/crds/ and charts/kubeopencode/templates/crds/
```

## Testing Guidelines

Three-tier strategy: unit (`make test`), integration (`make integration-test`, uses envtest), E2E (`make e2e-test`, uses Kind).

- Integration tests use `//go:build integration` tag (standard Kubernetes ecosystem pattern)
- Integration test files live alongside source code in `internal/controller/`
- E2E tests in `e2e/` directory

## Documentation

| File | When to Update |
|------|----------------|
| `README.md` | User-facing changes, new features |
| `website/docs/getting-started.md` | Installation, examples |
| `website/docs/features.md` | Context, concurrency, quota, pod config, server mode |
| `docs/agent-images.md` | Agent image changes |
| `website/docs/security.md` | RBAC, credentials |
| `website/docs/architecture.md` | System design, API reference |
| `docs/troubleshooting.md` | Common issues |
| `charts/kubeopencode/README.md` | Helm values |
| `AGENTS.md` | API changes (also update inline godoc) |

> **IMPORTANT**: Always update ALL relevant documentation when making changes. ADRs go in `docs/adr/`.

## Git Workflow

- **Conventional commits**: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`
- **Always sign commits**: `git commit -s -m "feat: description"`
- **PRs**: Check for upstream repos first; create PRs against upstream, not forks
- **Releasing**: Follow `docs/releasing.md` (Release SOP) step-by-step. Never skip steps or improvise the release process.

## Troubleshooting

1. **CRDs not updating**: `make update-crds`
2. **Deepcopy errors**: `make update`
3. **Lint failures**: `make lint` locally first
4. **E2E failures**: Check Kind cluster storage class

> See `docs/troubleshooting.md` for debugging commands and detailed solutions.

## Best Practices

1. Keep reconcile loops idempotent
2. Use owner references for garbage collection
3. Never log sensitive data (tokens, credentials)
4. Use `kubectl annotate task <name> kubeopencode.io/stop=true` to stop Tasks (not `kubectl delete`)

## Project Status

- **Version**: v0.0.25
- **API Stability**: v1alpha1 (subject to change)
- **License**: Apache License 2.0

---

**Last Updated**: 2026-04-14
