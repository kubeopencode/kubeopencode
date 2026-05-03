# Troubleshooting Guide

This guide helps diagnose and resolve common issues with KubeOpenCode.

## Controller Issues

### Controller Not Starting

Check controller logs:

```bash
kubectl logs -n kubeopencode-system deployment/kubeopencode-controller
```

Check events:

```bash
kubectl get events -n kubeopencode-system --sort-by='.lastTimestamp'
```

Common causes:
- Missing RBAC permissions
- CRDs not installed
- Resource limits too low

### RBAC Errors

Verify controller permissions:

```bash
kubectl auth can-i create pods \
  --as=system:serviceaccount:kubeopencode-system:kubeopencode-controller \
  -n kubeopencode-system
```

If denied, check the ClusterRole and ClusterRoleBinding are properly installed.

### Agent Fails with "cannot set blockOwnerDeletion"

If controller logs show:

```
configmaps "xxx-server-context" is forbidden: cannot set blockOwnerDeletion
if an ownerReference refers to a resource you can't set finalizers on
```

This means the controller ClusterRole is missing `agents/finalizers` permission. This is required for creating ConfigMaps and Deployments with `blockOwnerDeletion` OwnerReferences pointing to Agent resources.

**Fix**: Ensure the controller ClusterRole includes `agents/finalizers`:

```yaml
- apiGroups:
  - kubeopencode.io
  resources:
  - tasks/finalizers
  - agents/finalizers    # Required for Agents
  verbs:
  - update
```

Or patch it directly:

```bash
kubectl patch clusterrole kubeopencode-controller --type='json' \
  -p='[{"op": "replace", "path": "/rules/2/resources", "value": ["tasks/finalizers", "agents/finalizers"]}]'
```

> **Note**: This issue is more likely to surface on OpenShift clusters, which enforce `blockOwnerDeletion` RBAC checks more strictly than Kind or vanilla Kubernetes clusters.

### CRDs Not Found

Ensure CRDs are installed:

```bash
kubectl get crds | grep kubeopencode
```

Expected output:

```
agents.kubeopencode.io                <timestamp>
kubeopencodeconfigs.kubeopencode.io   <timestamp>
tasks.kubeopencode.io                 <timestamp>
```

If missing, reinstall with Helm or apply manually:

```bash
kubectl apply -f deploy/crds/
```

## Task and Pod Issues

### Task Stuck in Pending

Check Task status:

```bash
kubectl describe task <task-name> -n <namespace>
```

Common causes:
- Agent not found (check `agentRef`)
- Agent does not exist in the same namespace as the Task

### Task Stuck in Queued

Check if Agent has concurrency limits:

```bash
kubectl get agent <agent-name> -o yaml | grep -A5 maxConcurrentTasks
kubectl get agent <agent-name> -o yaml | grep -A5 quota
```

If `maxConcurrentTasks` is set, wait for running Tasks to complete or increase the limit.

If `quota` is configured, check the sliding window hasn't exceeded `maxTaskStarts`.

### Pod Failures

List task pods:

```bash
kubectl get pods -n <namespace> -l kubeopencode.io/task=<task-name>
```

Check pod logs:

```bash
kubectl logs <pod-name> -n <namespace>
```

Check pod events:

```bash
kubectl describe pod <pod-name> -n <namespace>
```

### ImagePullBackOff

Check if the image exists and is accessible:

```bash
kubectl describe pod <pod-name> -n <namespace> | grep -A10 Events
```

Common causes:
- Image doesn't exist
- Private registry requires imagePullSecrets
- In Kind cluster, images need to be loaded with `kind load docker-image`

### Init Container Failures

Check init container logs:

```bash
# List containers
kubectl get pod <pod-name> -o jsonpath='{.spec.initContainers[*].name}'

# Check specific init container logs
kubectl logs <pod-name> -c git-init
kubectl logs <pod-name> -c context-init
kubectl logs <pod-name> -c url-fetch
```

## Context Resolution Issues

### Git Context Failures

Check the git-init container logs:

```bash
kubectl logs <pod-name> -c git-init
```

Common causes:
- Repository URL is incorrect
- Authentication failed (check secretRef)
- Branch/ref doesn't exist
- Network connectivity issues

### ConfigMap Context Not Found

Verify the ConfigMap exists in the correct namespace:

```bash
kubectl get configmap <configmap-name> -n <namespace>
```

Note: ConfigMaps are resolved from the same namespace as the Task and Agent.

### URL Context Failures

Check the url-fetch container logs:

```bash
kubectl logs <pod-name> -c url-fetch
```

Common causes:
- URL is unreachable
- SSL/TLS certificate issues
- Network policies blocking egress

## Output Issues

### Outputs Not Captured

Check if output files exist in the workspace:

```bash
kubectl exec <pod-name> -- ls -la /workspace/.outputs/
```

Verify output parameter paths match what the agent writes:

```yaml
outputs:
  parameters:
    - name: pr-url
      path: ".outputs/pr-url"  # Relative to workspaceDir
```

### Output Truncated

Kubernetes has a 4KB limit for termination messages. For larger outputs:
- Use external storage (S3, GCS)
- Write to a ConfigMap or Secret
- Use a log aggregation system

## Performance Issues

### Controller High Memory Usage

Check controller resource usage:

```bash
kubectl top pod -n kubeopencode-system
```

Consider increasing memory limits in Helm values:

```yaml
controller:
  resources:
    limits:
      memory: 1Gi
```

### Slow Task Creation

Check controller logs for reconciliation times:

```bash
kubectl logs -n kubeopencode-system deployment/kubeopencode-controller | grep "Reconcile"
```

Common causes:
- Too many Tasks with `maxConcurrentTasks` limits
- Slow etcd performance
- Network latency to API server

## Debugging Commands

### Get All KubeOpenCode Resources

```bash
kubectl get tasks,agents,kubeopencodeconfigs --all-namespaces
```

### Watch Task Status Changes

```bash
kubectl get tasks -w
```

### Check Controller Logs with Debug Level

If running locally:

```bash
go run ./cmd/kubeopencode controller --zap-log-level=debug
```

In cluster, update the deployment to add the flag.

### Verify Webhook/Event Triggers (Argo Events)

```bash
# Check EventSource
kubectl get eventsource -n <namespace>

# Check Sensor
kubectl get sensor -n <namespace>

# Check Sensor logs
kubectl logs -n <namespace> -l sensor-name=<sensor-name>
```

## Debugging Tools

### Reading OpenCode Stream JSON Output

When running Tasks with `--format json`, the output is in stream-json format which can be hard to read. Use the provided utility script to format the output:

```bash
# Read from kubectl logs
kubectl logs <pod-name> -n <namespace> | ./hack/opencode-stream-reader.sh

# Read from a saved log file
cat task-output.log | ./hack/opencode-stream-reader.sh
```

The script requires `jq` and converts the JSON stream into human-readable output with colors and formatting.

## Common Error Messages

### "Agent not found"

The referenced Agent doesn't exist. Check:
- Agent name is correct
- Agent exists in the same namespace as the Task

### "context resolution failed"

A context couldn't be resolved. Check:
- ConfigMap/Secret exists
- Git repository is accessible
- URL is reachable
- `optional: true` if the context should be skipped on failure

### "quota exceeded"

The Agent's rate limit has been reached. Wait for the sliding window to pass or adjust quota settings.
