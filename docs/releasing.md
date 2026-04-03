# Release SOP (Standard Operating Procedure)

This document describes how to release a new version of KubeOpenCode. It is designed to be followed step-by-step by either a human or an AI agent.

## Overview

A KubeOpenCode release produces the following artifacts:

| Artifact | Registry | Example |
|----------|----------|---------|
| Controller image | `quay.io/kubeopencode/kubeopencode` | `:v0.1.0` |
| Agent OpenCode image | `quay.io/kubeopencode/kubeopencode-agent-opencode` | `:v0.1.0` |
| Agent Devbox image | `quay.io/kubeopencode/kubeopencode-agent-devbox` | `:v0.1.0` |
| Agent Attach image | `quay.io/kubeopencode/kubeopencode-agent-attach` | `:v0.1.0` |
| Helm chart | `oci://quay.io/kubeopencode/helm-charts/kubeopencode` | `0.1.0` |
| GitHub Release | github.com/kubeopencode/kubeopencode/releases | `v0.1.0` |

All container images are built for `linux/amd64` and `linux/arm64`.

## Versioning Convention

- **Git tag**: `v{MAJOR}.{MINOR}.{PATCH}` (e.g., `v0.1.0`)
- **Image tag**: Same as git tag (e.g., `v0.1.0`), following the Kubernetes ecosystem convention
- **Helm chart version**: `{MAJOR}.{MINOR}.{PATCH}` without `v` prefix (e.g., `0.1.0`), following Helm convention
- **Helm chart appVersion**: Same as git tag with `v` prefix (e.g., `v0.1.0`), used to resolve default image tags

## Release Steps

### Step 1: Determine the New Version

Decide the new version number based on [Semantic Versioning](https://semver.org/):
- **MAJOR**: Incompatible API changes (CRD breaking changes)
- **MINOR**: New features, backward-compatible
- **PATCH**: Bug fixes, backward-compatible

For the rest of this document, `NEW_VERSION` refers to the version **without** the `v` prefix (e.g., `0.1.0`), and `NEW_TAG` refers to the version **with** the `v` prefix (e.g., `v0.1.0`).

### Step 2: Update Version References

Update the following files on a release branch:

```bash
git checkout main
git pull origin main
git checkout -b release/vNEW_VERSION
```

#### 2.1 `Makefile` (line 8)

```makefile
VERSION ?= NEW_VERSION
```

#### 2.2 `agents/Makefile` (line 21)

```makefile
VERSION ?= NEW_VERSION
```

#### 2.3 `charts/kubeopencode/Chart.yaml`

```yaml
version: NEW_VERSION
appVersion: "vNEW_VERSION"
```

Note: `version` has no `v` prefix, `appVersion` has the `v` prefix.

#### 2.4 `CLAUDE.md` (Project Status section)

```markdown
- **Version**: vNEW_VERSION
```

### Step 3: Verify Locally

Run the following commands to ensure everything is correct:

```bash
# Verify generated code is up to date
make verify

# Run all test levels
make test
make integration-test

# Run lint and fix ALL issues before proceeding
make lint

# Verify version command works
go run -ldflags "-X main.Version=NEW_VERSION" ./cmd/kubeopencode version

# Verify Helm chart renders correct image tags
helm template kubeopencode charts/kubeopencode | grep 'image:'
# Expected: quay.io/kubeopencode/kubeopencode:vNEW_VERSION
```

> **IMPORTANT**: `make lint` must report **0 issues** before proceeding to Step 4. The lint version is auto-detected from Go version (see `ci/lint/run-lint.sh`), so upgrading Go may surface new lint findings. Fix all issues on the release branch before creating the PR.

### Step 4: Commit, Create PR, and Merge

```bash
git add Makefile agents/Makefile charts/kubeopencode/Chart.yaml CLAUDE.md
git commit -s -m "chore: prepare vNEW_VERSION release

- Update VERSION to NEW_VERSION in Makefile and agents/Makefile
- Update Chart.yaml version to NEW_VERSION and appVersion to vNEW_VERSION
- Update CLAUDE.md project status version"

git push origin release/vNEW_VERSION
```

Create a PR targeting `main`:

```bash
gh pr create --title "chore: release vNEW_VERSION" --body "..."
```

Wait for all CI checks to pass, then merge:

```bash
# Poll until checks pass (or fail)
gh pr checks <PR_NUMBER> --watch

# If all checks pass, merge immediately
gh pr merge <PR_NUMBER> --merge --delete-branch
```

> **IMPORTANT (for AI assistants)**: This step is fully automated. Do NOT stop to ask the user for merge approval. If CI checks pass, merge the PR and proceed to Step 5. Only escalate to the user if CI checks fail.

### Step 5: Tag and Push

After the PR is merged:

```bash
git checkout main
git pull origin main
git tag -a vNEW_VERSION -m "Release vNEW_VERSION"
git push origin vNEW_VERSION
```

This triggers the `.github/workflows/release.yaml` workflow, which:
1. Verifies `Chart.yaml` appVersion matches the tag
2. Runs all tests (unit, integration, e2e)
3. Builds and pushes all container images (multi-arch)
4. Packages and pushes the Helm chart to OCI registry
5. Creates a GitHub Release with auto-generated release notes

### Step 6: Monitor the Release Workflow

Wait for the Release workflow to complete:

```bash
# Get the workflow run triggered by the tag
gh run list --workflow=release.yaml --limit=1
gh run watch <RUN_ID>
```

> **IMPORTANT (for AI assistants)**: Monitor the workflow automatically. If all jobs pass, proceed to Step 7. Only escalate to the user if any job fails.

Expected jobs:

- `verify-versions`
- `unit-test`, `integration-test`, `e2e-test`
- `build-kubeopencode`
- `build-agent-opencode`
- `build-agent-devbox`
- `build-agent-attach`
- `push-helm-chart`
- `github-release`

### Step 7: Verify Published Artifacts

```bash
# Verify container images
docker pull quay.io/kubeopencode/kubeopencode:vNEW_VERSION
docker run --rm quay.io/kubeopencode/kubeopencode:vNEW_VERSION version
# Expected output: kubeopencode version NEW_VERSION

# Verify Helm chart
helm pull oci://quay.io/kubeopencode/helm-charts/kubeopencode --version NEW_VERSION

# Test Helm install (dry-run)
helm install kubeopencode oci://quay.io/kubeopencode/helm-charts/kubeopencode \
  --version NEW_VERSION \
  --namespace kubeopencode-system \
  --create-namespace \
  --dry-run
```

### Step 8: Update GitHub Release Notes

Update the GitHub Release notes using the `gh` CLI. The release notes **must** follow this standard format:

```bash
gh release edit vNEW_VERSION --notes "$(cat <<'EOF'
## Highlights

<1-2 sentence summary of the most important changes in this release.>

## Breaking Changes

- **<Change title>** (#PR): <Description of what changed and migration guidance.>

## New Features

- **<Feature title>** (#PR): <Brief description.>

## Bug Fixes

- **<Fix title>** (#PR): <Brief description.>

## Refactoring

- <Description> (#PR)

## Dependencies

- <Description> (#PR)

## Installation

\```bash
# Helm install
helm install kubeopencode oci://quay.io/kubeopencode/helm-charts/kubeopencode \
  --version NEW_VERSION \
  --namespace kubeopencode-system \
  --create-namespace

# Or upgrade
helm upgrade kubeopencode oci://quay.io/kubeopencode/helm-charts/kubeopencode \
  --version NEW_VERSION \
  --namespace kubeopencode-system
\```

## All Changes

<Keep the auto-generated changelog from GitHub Actions as-is.>

**Full Changelog**: https://github.com/kubeopencode/kubeopencode/compare/vPREV_VERSION...vNEW_VERSION
EOF
)"
```

**Format guidelines:**
- Omit any section that has no entries (e.g., skip "Bug Fixes" if there are none)
- "Breaking Changes" section is required whenever there are incompatible API changes
- "All Changes" section preserves the auto-generated PR list from GitHub Actions
- "Installation" section always includes both fresh install and upgrade commands

## Troubleshooting

### Release workflow fails at `verify-versions`

The `Chart.yaml` `appVersion` does not match the git tag. Ensure `appVersion` is set to the tag value (e.g., `v0.1.0`). You need to fix it, merge, delete the tag, and re-tag.

```bash
git tag -d vNEW_VERSION
git push origin :refs/tags/vNEW_VERSION
# Fix Chart.yaml, commit, merge, then re-tag
```

### Image build fails

Check the workflow logs. Common causes:
- Quay.io credentials expired (`QUAY_ROBOT_ACCOUNT` / `QUAY_TOKEN` secrets)
- Docker buildx platform issues
- `build-agent-attach` depends on `build-agent-opencode` — if opencode fails, attach also fails

### Helm chart push fails

Ensure the Helm OCI registry login is working:
```bash
echo "$QUAY_TOKEN" | helm registry login quay.io -u $QUAY_ROBOT_ACCOUNT --password-stdin
```

## Key Files

| File | Purpose |
|------|---------|
| `Makefile` | `VERSION ?=` default, build targets with ldflags |
| `agents/Makefile` | Agent image `VERSION ?=` default |
| `charts/kubeopencode/Chart.yaml` | Helm chart `version` and `appVersion` |
| `cmd/kubeopencode/main.go` | `Version`, `GitCommit`, `BuildDate` variables (set via ldflags) |
| `Dockerfile` | `ARG VERSION` passed to Go ldflags at build time |
| `.github/workflows/release.yaml` | Release workflow triggered by `v*` tags |
| `CLAUDE.md` | Project status version |

## CRD Change Reminder

> **IMPORTANT FOR AI ASSISTANTS**: When a release includes **any** CRD changes (added/removed/modified fields in `api/v1alpha1/`), you **must**:
> 1. Mention this prominently in the release notes under "Breaking Changes" or "New Features"
> 2. Remind the user that `helm upgrade` does **not** update CRDs automatically
> 3. Point the user to the [Upgrade SOP](upgrading.md) for manual CRD update instructions
> 4. Update the "Version History with CRD Changes" table in `docs/upgrading.md`

## Notes

- The `DefaultKubeOpenCodeImage` in `internal/controller/pod_builder.go` stays as `:latest` intentionally. Users who want pinned versions configure it via `KubeOpenCodeConfig.spec.systemImage`.
- Agent image defaults (`DefaultAgentImage`, `DefaultExecutorImage`) also stay as `:latest`. Production users set explicit images in Agent CRDs.
- No "development version" reset is needed after a release. The Go code defaults to `Version = "dev"` for untagged builds.
- The `:latest` tag is also updated on every release, keeping it in sync with the most recent version.
