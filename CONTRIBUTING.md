# Contributing to KubeOpenCode

We welcome contributions! This document provides guidelines for contributing to KubeOpenCode.

## Getting Started

Before contributing, please:

1. Review the [Architecture Documentation](website/docs/architecture.md)
2. Set up your [Local Development Environment](#local-development-environment)
3. Read through [AGENTS.md](AGENTS.md) for detailed development guidelines

## Commit Standards

Always use signed commits with the `-s` flag (Developer Certificate of Origin):

```bash
git commit -s -m "feat: add new feature"
```

### Commit Message Format

Follow the [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>: <description>

[optional body]

Signed-off-by: Your Name <your.email@example.com>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

## Pull Requests

### Before Submitting

1. Check for upstream repositories first
2. Create PRs against upstream, not forks
3. Ensure your branch is up to date with main
4. Run all checks locally:

```bash
make lint    # Run linter
make test    # Run unit tests
make verify  # Verify generated code is up to date
```

### PR Guidelines

- Use descriptive titles and comprehensive descriptions
- Reference related issues (e.g., "Fixes #123")
- Keep PRs focused - one feature or fix per PR
- Update documentation if your changes affect user-facing behavior
- Add tests for new functionality

### PR Description Template

```markdown
## Summary
Brief description of the changes

## Related Issues
Fixes #<issue-number>

## Test Plan
- [ ] Unit tests pass
- [ ] Integration tests pass (if applicable)
- [ ] E2E tests pass (if applicable)
- [ ] Manual testing performed
```

## Code Standards

### Go Code

- Follow standard Go conventions
- Use `gofmt` and `golint`
- Write comments in English
- Document exported types and functions
- Use meaningful variable and function names

### Testing

- Write tests for new features
- Maintain test coverage
- Use table-driven tests where appropriate

```bash
# Run unit tests
make test

# Run integration tests (uses envtest)
make integration-test

# Run E2E tests (uses Kind cluster)
make e2e-teardown && make e2e-setup && make e2e-test
```

### API Changes

When modifying CRD definitions:

1. Update `api/v1alpha1/types.go`
2. Run `make update` to regenerate CRDs and deepcopy
3. Run `make verify` to ensure everything is correct
4. Update documentation in `website/docs/architecture.md`
5. Update integration tests in `internal/controller/*_test.go`
6. Update E2E tests in `e2e/`

## Local Development Environment

This section describes how to set up a local development environment using Kind (Kubernetes in Docker).

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Kind](https://kind.sigs.k8s.io/) (`brew install kind` on macOS)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/) 3.8+
- [Go](https://go.dev/) 1.25+

### One-Command Setup (Recommended)

The fastest way to get a local environment running:

```bash
make local-dev-setup
```

This single command performs all of the steps below automatically:
1. Creates a Kind cluster named `kubeopencode`
2. Builds the controller image and all agent images
3. Tags and loads all images into Kind with the `:dev` tag
4. Installs via Helm with correct image tags and pull policies
5. Deploys test resources (`deploy/local-dev/`) via Kustomize

### AI-Assisted Setup

Alternatively, tell your AI agent:

> Set up a local development environment for KubeOpenCode using `make local-dev-setup`.

The agent will handle everything automatically. See `AGENTS.md` for guidance on how agents discover local-dev instructions.

### Manual Setup

If you prefer to run each step individually:

#### 1. Create a Kind Cluster

Check if you already have a Kind cluster running:

```bash
kind get clusters
```

If you have an existing cluster, you can use it. Otherwise, create one:

```bash
kind create cluster --name kubeopencode
```

**IMPORTANT:** Local-dev MUST use the `kubeopencode` Kind cluster, NOT `kubeopencode-e2e` (which is reserved for E2E tests). Always verify your current context:

```bash
kubectl config current-context
# Expected: kind-kubeopencode (NOT kind-kubeopencode-e2e)
```

#### 2. Build Images

```bash
# Build the controller image
make docker-build

# Build all agent images (opencode, devbox, attach)
make agent-build-all
```

Or build individual agent images:

```bash
make agent-build AGENT=opencode    # OpenCode init container
make agent-build AGENT=devbox      # Executor container (development environment)
make agent-build AGENT=attach      # Attach image (required for agentRef Tasks)
```

> **Note:** The `attach` image is required for Tasks that reference Agents via `agentRef`. `make agent-build-all` is recommended to avoid missing images.

#### 3. Load Images to Kind

```bash
# Load controller image (use :dev tag, not :latest, to avoid PullAlways in Kind)
kind load docker-image ghcr.io/kubeopencode/kubeopencode:dev --name kubeopencode

# Tag and load all agent images with :dev tag
for img in opencode devbox attach; do
  docker tag ghcr.io/kubeopencode/kubeopencode-agent-${img}:$(make -s print-version) ghcr.io/kubeopencode/kubeopencode-agent-${img}:dev
  kind load docker-image ghcr.io/kubeopencode/kubeopencode-agent-${img}:dev --name kubeopencode
done
```

> **Important: Avoid `:latest` tag in Kind clusters.** The controller sets `imagePullPolicy: Always` for images with the `:latest` tag (standard Kubernetes convention). In Kind, this causes `ErrImagePull` because the cluster cannot pull from remote registries. All local-dev images use the `:dev` tag to avoid this.

#### 4. Deploy with Helm

```bash
helm upgrade --install kubeopencode ./charts/kubeopencode \
  --namespace kubeopencode-system \
  --create-namespace \
  --set controller.image.tag=dev \
  --set controller.image.pullPolicy=Never \
  --set agent.image.pullPolicy=Never \
  --set server.enabled=true \
  --set server.image.tag=dev \
  --set server.image.pullPolicy=Never \
  --set kubeopencodeConfig.systemImage.image=ghcr.io/kubeopencode/kubeopencode:dev \
  --set kubeopencodeConfig.systemImage.imagePullPolicy=Never
```

> **Important:** All images in Kind must use the `:dev` tag with `imagePullPolicy=Never`. The `systemImage` override ensures init containers (git-init, context-init) also use the local `:dev` image instead of the default `:latest`.

#### 5. Deploy Test Resources

```bash
kubectl apply -k deploy/local-dev/
```

#### 6. Verify Deployment

```bash
# Controller and UI server
kubectl get pods -n kubeopencode-system

# Agents in test namespace
kubectl get agent -n test
```

### Access the Web UI

```bash
kubectl port-forward -n kubeopencode-system svc/kubeopencode-server 2746:2746
```

Open http://localhost:2746. See `website/docs/getting-started.md` for UI features overview.

### Test Resources

The `deploy/local-dev/` directory provides pre-configured resources for testing:

| Resource | Name | Description |
|----------|------|-------------|
| Namespace | `test` | Isolated namespace for testing |
| AgentTemplate | `team-base` | Shared base configuration (images, model, workspace) |
| Agent | `team-agent` | Persistent agent with session + workspace storage, standby, concurrency control |
| Agent | `dev-agent` | Lightweight agent with ephemeral storage |

These demonstrate key features: template inheritance, persistence, suspend/resume, and concurrency control. The default setup uses the free `opencode/big-pickle` model — **no API key required**.

#### Using a Paid Model

To use a paid AI model:

```bash
cp deploy/local-dev/secrets.yaml.example deploy/local-dev/secrets.yaml
# Edit secrets.yaml with your real API key
kubectl apply -f deploy/local-dev/secrets.yaml -n test
```

Then update the AgentTemplate to reference the model and credentials. See `deploy/local-dev/secrets.yaml.example` for instructions.

### Iterative Development

After the initial setup, use this command to rebuild and reload changes:

```bash
make local-dev-reload
```

This rebuilds the Docker image (including UI), loads it into the Kind cluster, and restarts all deployments.

For faster UI iteration, use the dev server (no Docker rebuild needed):

```bash
# Terminal 1: Run Go server locally
make run-server

# Terminal 2: Run webpack dev server with hot-reload
make ui-dev
```

### Teardown

```bash
make local-dev-teardown
```

This removes test resources, uninstalls Helm, and deletes the Kind cluster.

### Troubleshooting

#### Image Pull Errors

If you see `ErrImagePull` or `ImagePullBackOff`:

1. Verify images are loaded: `docker exec kubeopencode-control-plane crictl images | grep kubeopencode`
2. Check `imagePullPolicy` is `Never` in Helm values
3. Ensure the `attach` image is loaded (required for `agentRef` Tasks)

#### Controller Not Starting

```bash
kubectl logs -n kubeopencode-system deployment/kubeopencode-controller
kubectl get events -n kubeopencode-system --sort-by='.lastTimestamp'
```

#### CRDs Not Found

```bash
kubectl get crds | grep kubeopencode
# If missing:
kubectl apply -f deploy/crds/
```

#### PVC Issues

```bash
kubectl describe pvc -n test
kubectl get storageclass
```

Kind clusters include a `standard` StorageClass by default.

## Development Workflow

### Building

```bash
make build        # Build the controller
make docker-build # Build Docker image
```

### Running Locally

```bash
make run  # Run controller locally (requires kubeconfig)
```

## Reporting Issues

When reporting issues:

1. Search existing issues to avoid duplicates
2. Use a clear, descriptive title
3. Include:
   - Steps to reproduce
   - Expected behavior
   - Actual behavior
   - Environment details (Kubernetes version, OS, etc.)
   - Relevant logs or error messages

## Getting Help

- Review existing documentation in `docs/`
- Check [Troubleshooting Guide](docs/troubleshooting.md)
- Open a [GitHub Discussion](https://github.com/kubeopencode/kubeopencode/discussions)
- Review existing issues and PRs

## License

By contributing to KubeOpenCode, you agree that your contributions will be licensed under the Apache License 2.0.
