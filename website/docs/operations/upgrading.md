---
sidebar_position: 3
title: Upgrading
description: Upgrade SOP for KubeOpenCode on existing clusters
---

# Upgrade SOP (Standard Operating Procedure)

This document describes how to upgrade KubeOpenCode on an existing cluster.

## Prerequisites

- `helm` CLI installed
- `kubectl` or `oc` CLI with cluster admin access
- Access to the target cluster (`kubeopencode-system` namespace)

## Upgrade Steps

### Step 1: Check Current Version

```bash
helm list -n kubeopencode-system
```

### Step 2: Review Release Notes

Before upgrading, review the release notes for the target version:

```bash
gh release view vNEW_VERSION
```

> **IMPORTANT**: Pay attention to the **Breaking Changes** section. If CRD changes are listed, you **must** manually apply CRDs after the Helm upgrade (see Step 4).

### Step 3: Upgrade via Helm

```bash
helm upgrade kubeopencode oci://quay.io/kubeopencode/helm-charts/kubeopencode \
  --version NEW_VERSION \
  --namespace kubeopencode-system
```

### Step 4: Apply CRD Updates (if applicable)

> **IMPORTANT**: Helm does **not** automatically upgrade CRDs during `helm upgrade`. This is a [known Helm limitation](https://helm.sh/docs/chart_best_practices/custom_resource_definitions/#some-caveats-and-explanations). If the release includes CRD changes (new fields, removed fields, validation updates), you must manually apply them.

**Option A**: Apply from the Helm chart (if you have the repo checked out at the release tag):

```bash
git checkout vNEW_VERSION
kubectl apply -f charts/kubeopencode/crds/
```

**Option B**: Apply from the OCI chart (pull and extract):

```bash
helm pull oci://quay.io/kubeopencode/helm-charts/kubeopencode \
  --version NEW_VERSION --untar
kubectl apply -f kubeopencode/crds/
rm -rf kubeopencode/
```

Verify CRDs are updated:

```bash
kubectl get crd tasks.kubeopencode.io -o jsonpath='{.metadata.resourceVersion}'
kubectl get crd agents.kubeopencode.io -o jsonpath='{.metadata.resourceVersion}'
```

### Step 5: Verify Rollout

```bash
# Check deployments are running
kubectl rollout status deployment/kubeopencode-controller -n kubeopencode-system
kubectl rollout status deployment/kubeopencode-server -n kubeopencode-system

# Verify pods are healthy
kubectl get pods -n kubeopencode-system

# Confirm version
helm list -n kubeopencode-system
```

### Step 6: Validate

Run a quick smoke test to ensure the upgrade was successful:

```bash
# List agents
kubectl get agents -A

# List tasks
kubectl get tasks -A

# Check controller logs for errors
kubectl logs -n kubeopencode-system deployment/kubeopencode-controller --tail=50
```

## Rollback

If something goes wrong, rollback to the previous version:

```bash
helm rollback kubeopencode -n kubeopencode-system
```

> **Note**: Helm rollback does **not** rollback CRD changes. If you need to rollback CRDs, manually apply the CRDs from the previous version.

## Version History with CRD Changes

This section tracks which releases include CRD changes, so operators know when manual CRD updates are required.

| Version | CRD Changes | Description |
|---------|-------------|-------------|
| v0.0.13 | Yes         | Replaced `ServerStatus.readyReplicas` (int32) with `ready` (bool) in Agent CRD |
| v0.0.9  | No (RBAC)   | Added `agents/finalizers` permission to controller ClusterRole (required for Server-mode Agents on OpenShift); fixed UI version display |
| v0.0.4  | Yes         | Removed `AgentReference.Namespace`, `TaskExecutionStatus.PodNamespace`, `AgentSpec.AllowedNamespaces` |
| v0.0.3  | No          | Bug fixes only |
| v0.0.2  | No          | Bug fixes only |
| v0.0.1  | Yes         | Initial release |
