---
title: Running a newer opencode (custom agent image)
sidebar_position: 60
---

# Running a newer opencode version (custom agent image)

The opencode binary is **baked into the agent image** (`agentImage`,
`ghcr.io/kubeopencode/kubeopencode-agent-opencode`). The version is set at build
time by the `OPENCODE_VERSION` build arg in `agents/Makefile` — there is **no runtime
override**. The published `:latest` therefore ships whatever version was current when
it was built, which can lag behind upstream opencode.

To run a specific / newer opencode, build the agent image yourself with a newer
`OPENCODE_VERSION`, push it to a registry you control, and point your `Agent` at it.

## 1. Check what you're running

```bash
POD=$(kubectl -n kubeopencode-system get pod -l kubeopencode.io/agent=<agent> -o name | head -1)
kubectl -n kubeopencode-system logs $POD -c opencode-init | grep -i version
# e.g. [opencode-init] OpenCode binary installed successfully (version 1.17.11).
```

Compare with the latest opencode release (https://opencode.ai/changelog).

## 2. Build a custom agent image

Requires **git + Docker (with buildx)** and a registry you can push to (your own
GHCR namespace works well). Build on a machine with Docker — a k3s node running only
containerd cannot build.

```bash
git clone https://github.com/kubeopencode/kubeopencode
cd kubeopencode/agents

docker login ghcr.io                 # GitHub PAT with write:packages

make AGENT=opencode \
     OPENCODE_VERSION=1.18.31 \      # the opencode version you want
     IMG_ORG=<your-namespace> \      # e.g. your GitHub username
     VERSION=v0.1.9-oc1.18.31 \      # your own tag
     buildx                          # builds AND pushes (multi-arch)
```

Result: `ghcr.io/<your-namespace>/kubeopencode-agent-opencode:v0.1.9-oc1.18.31`.

- `buildx` builds and pushes in one step (`--push` is in the recipe), so
  `docker login` must succeed first.
- For single-arch instead: `make ... build` then `make ... push`.
- **Version note:** opencode's release-asset naming can change across major versions.
  If the build fails at the download step for a new major (e.g. `2.x`), pin the newest
  minor of the series you're on (e.g. `1.18.x`) or adjust the `FILENAME` pattern in
  `agents/opencode/Dockerfile` to match the release assets.
- Only `agentImage` carries the opencode binary — `executorImage` and `attachImage`
  do not need rebuilding to change the opencode version.

## 3. Make the image pullable by the cluster

### Option A — public package (simplest)

The first push creates the package **private**. Make it public once:

GitHub → your **Packages** → `kubeopencode-agent-opencode` → **Package settings** →
**Change visibility** → **Public**.

No cluster changes needed. Verify:

```bash
# on a node:
sudo k3s crictl pull ghcr.io/<your-namespace>/kubeopencode-agent-opencode:v0.1.9-oc1.18.31
```

### Option B — keep it private with an imagePullSecret

```bash
# GitHub PAT with read:packages
kubectl -n kubeopencode-system create secret docker-registry ghcr-pull \
  --docker-server=ghcr.io \
  --docker-username=<your-username> \
  --docker-password='<PAT>' \
  --docker-email=<your-email>
```

Reference it on the Agent (targeted), or attach it to the agent ServiceAccount
(covers all agent/task pods in the namespace):

```bash
# service-account variant:
kubectl -n kubeopencode-system patch serviceaccount kubeopencode-agent \
  -p '{"imagePullSecrets":[{"name":"ghcr-pull"}]}'
```

## 4. Point the Agent at your image

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: <agent>
  namespace: kubeopencode-system
spec:
  agentImage: ghcr.io/<your-namespace>/kubeopencode-agent-opencode:v0.1.9-oc1.18.31
  # imagePullSecrets:            # only for Option B
  #   - name: ghcr-pull
  # ...your existing spec (profile, config, credentials)...
```

```bash
kubectl apply -f agent.yaml
kubectl -n kubeopencode-system rollout status deploy/<agent>-server
```

## 5. Verify

```bash
POD=$(kubectl -n kubeopencode-system get pod -l kubeopencode.io/agent=<agent> -o name | head -1)
kubectl -n kubeopencode-system logs $POD -c opencode-init | grep -i version
# should now report the version you built
```

## Notes

- **Pin a tag or digest — avoid `:latest`.** `:latest` with the default
  `IfNotPresent` pull policy means a node silently keeps whatever it cached, so updates
  don't take effect until the image is removed. A pinned tag/digest makes updates
  deliberate and rollbacks trivial (set the previous tag back).
- **`helm upgrade` does not change the opencode version** — it updates the
  controller/server. opencode version = `agentImage` only. Keep the two concerns
  separate.
- To force a re-pull of a `:latest` you must remove the cached image while it is not in
  use: `kubectl scale deploy/<agent>-server --replicas=0`, then
  `crictl --timeout 3m rmi <image>`, then scale back to 1.
