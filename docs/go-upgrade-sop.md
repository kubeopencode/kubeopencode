# Go Version Upgrade SOP

Standard Operating Procedure for upgrading the Go version used by KubeOpenCode.

## Prerequisites

- Verify the target Go version is **stable** (not RC/beta):
  ```bash
  curl -s https://go.dev/dl/?mode=json | python3 -c "import json,sys; data=json.load(sys.stdin); [print(r['version'], 'stable' if r.get('stable') else 'unstable') for r in data[:5]]"
  ```
- Ensure the target version is installed locally (or `GOTOOLCHAIN=auto` is set so it downloads automatically).

## Files to Update

### 1. `go.mod` (project root)

Update the `go` directive:
```
go 1.XX.0
```

### 2. CI Workflow Files (`.github/workflows/`)

Update `GO_VERSION` env var in **all** workflow files:

| File | Variable |
|------|----------|
| `.github/workflows/pr.yaml` | `GO_VERSION: "1.XX"` |
| `.github/workflows/push.yaml` | `GO_VERSION: "1.XX"` |
| `.github/workflows/release.yaml` | `GO_VERSION: "1.XX"` |
| `.github/workflows/manual-build.yaml` | `GO_VERSION: "1.XX"` |
| `.github/workflows/weekly-full-build.yaml` | `GO_VERSION: "1.XX"` |

### 3. `Dockerfile` (project root)

Update the builder base image:
```dockerfile
FROM golang:1.XX-alpine AS builder
```

> **Note**: Agent Dockerfiles in `agents/` typically use runtime images (not Go builder), so they usually don't need changes.

## Post-Update Steps

Run these commands **in order**:

```bash
# 1. Update dependencies and vendor
go mod tidy

# 2. Build
make build

# 3. Run unit tests
make test

# 4. Run integration tests
make integration-test

# 5. Verify generated code is up to date
make verify

# 6. Lint
make lint
```

## Common Issues

### `setup-envtest` version incompatibility

The `Makefile` installs `setup-envtest@latest`. If the latest version requires a newer Go than what CI uses, the integration test job will fail with:
```
requires go >= 1.XX.0 (running go 1.YY.Z; GOTOOLCHAIN=local)
```

**Fix**: Upgrade Go (this SOP) or pin `setup-envtest` in the Makefile to a compatible version.

### `GOTOOLCHAIN=local` in CI

CI workflows set `GOTOOLCHAIN=local` to prevent automatic toolchain downloads. This means CI strictly uses the Go version installed by the `setup-go` action. If `go.mod` specifies a newer version than CI installs, builds will fail.

**Fix**: Ensure `GO_VERSION` in workflows matches or exceeds the version in `go.mod`.

## Checklist

- [ ] `go.mod` — `go` directive updated
- [ ] `.github/workflows/pr.yaml` — `GO_VERSION` updated
- [ ] `.github/workflows/push.yaml` — `GO_VERSION` updated
- [ ] `.github/workflows/release.yaml` — `GO_VERSION` updated
- [ ] `.github/workflows/manual-build.yaml` — `GO_VERSION` updated
- [ ] `.github/workflows/weekly-full-build.yaml` — `GO_VERSION` updated
- [ ] `Dockerfile` — builder image updated (if needed)
- [ ] `go mod tidy` — ran successfully
- [ ] `make build` — passed
- [ ] `make test` — passed
- [ ] `make integration-test` — passed
- [ ] `make verify` — passed

## History

| Date | From | To | Notes |
|------|------|----|-------|
| 2026-04-01 | Go 1.25 | Go 1.26 | `setup-envtest@latest` started requiring Go 1.26; Dockerfile already had `golang:1.26-alpine` |
