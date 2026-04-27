# KubeOpenCode Helm Chart

This Helm chart deploys KubeOpenCode, a Kubernetes-native system for executing AI-powered tasks.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.8+
- AI provider API key (e.g., Anthropic, Google AI, OpenAI) for OpenCode (optional — a free model is available by default)

## Installing the Chart

### Quick Start

```bash
# Create namespace
kubectl create namespace kubeopencode-system

# Install from OCI registry
helm install kubeopencode oci://ghcr.io/kubeopencode/helm-charts/kubeopencode \
  --namespace kubeopencode-system

# Or install from local chart (for development)
helm install kubeopencode ./charts/kubeopencode \
  --namespace kubeopencode-system
```

### Production Installation

```bash
# Create a values file with your configuration
cat > my-values.yaml <<EOF
controller:
  image:
    repository: ghcr.io/kubeopencode/kubeopencode
    tag: v0.0.26

  resources:
    limits:
      cpu: 1000m
      memory: 1Gi
    requests:
      cpu: 200m
      memory: 256Mi

server:
  enabled: true
  ingress:
    enabled: true
    className: nginx
    hosts:
      - host: kubeopencode.example.com
        paths:
          - path: /
            pathType: Prefix
EOF

# Install the chart
helm install kubeopencode oci://ghcr.io/kubeopencode/helm-charts/kubeopencode \
  --namespace kubeopencode-system \
  --values my-values.yaml
```

#### Using Gateway API (HTTPRoute)

If your cluster uses the [Gateway API](https://gateway-api.sigs.k8s.io/) instead of Ingress:

```yaml
server:
  enabled: true
  route:
    main:
      enabled: true
      parentRefs:
        - name: my-gateway
          namespace: gateway-system
      hostnames:
        - kubeopencode.example.com
```

> **Note:** The agent proxy uses SSE (Server-Sent Events) for streaming. Depending on your
> gateway controller, you may need to disable response buffering and increase timeouts.
> For example, with Envoy Gateway, add a [BackendTrafficPolicy](https://gateway.envoyproxy.io/docs/api/extension_types/#backendtrafficpolicy)
> to configure timeouts.

## Configuration

The following table lists the configurable parameters of the KubeOpenCode chart and their default values.

### Controller Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.image.repository` | Controller image repository | `ghcr.io/kubeopencode/kubeopencode` |
| `controller.image.tag` | Controller image tag | `""` (uses chart appVersion) |
| `controller.image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `controller.replicas` | Number of controller replicas | `1` |
| `controller.resources.limits.cpu` | CPU limit | `500m` |
| `controller.resources.limits.memory` | Memory limit | `512Mi` |
| `controller.resources.requests.cpu` | CPU request | `100m` |
| `controller.resources.requests.memory` | Memory request | `128Mi` |

### Server Configuration (UI + API)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `server.enabled` | Enable the UI server | `true` |
| `server.service.type` | Service type | `ClusterIP` |
| `server.service.port` | Service port | `2746` |
| `server.auth.enabled` | Enable token-based authentication | `true` |
| `server.auth.allowAnonymous` | Allow unauthenticated requests | `false` |
| `server.ingress.enabled` | Enable Ingress | `false` |
| `server.ingress.className` | Ingress class name | `""` |
| `server.route.main.enabled` | Enable Gateway API HTTPRoute | `false` |
| `server.route.main.parentRefs` | Gateway references for HTTPRoute | `[]` |
| `server.route.main.hostnames` | Hostnames for HTTPRoute matching | `[]` |
| `server.route.main.httpsRedirect` | Enable HTTPS redirect (HTTP 301) | `false` |
| `server.portForward.enabled` | Create port-forward RBAC | `false` |

### Agent Configuration

Agent images are configured in Agent CRDs, not in this Helm chart. The two-container pattern uses:
- `agentImage`: OpenCode init container (default: `ghcr.io/kubeopencode/kubeopencode-agent-opencode`)
- `executorImage`: Worker container (default: `ghcr.io/kubeopencode/kubeopencode-agent-devbox`)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `agent.image.pullPolicy` | Agent image pull policy | `IfNotPresent` |

### System Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `kubeopencodeConfig.create` | Create default KubeOpenCodeConfig | `true` |
| `kubeopencodeConfig.systemImage.image` | System image for init containers | `""` (uses controller image) |
| `kubeopencodeConfig.systemImage.imagePullPolicy` | System image pull policy | `IfNotPresent` |

### Cleanup Configuration

Task cleanup is configured via the `KubeOpenCodeConfig` resource (not Helm values):

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeConfig
metadata:
  name: cluster
spec:
  cleanup:
    ttlSecondsAfterFinished: 604800  # 7 days
    maxRetainedTasks: 100            # Per namespace
```

## Usage Examples

### Creating an Agent

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: default
  namespace: kubeopencode-system
spec:
  agentImage: ghcr.io/kubeopencode/kubeopencode-agent-opencode:latest
  executorImage: ghcr.io/kubeopencode/kubeopencode-agent-devbox:latest
  workspaceDir: /workspace
  serviceAccountName: kubeopencode-agent
  credentials:
    - name: anthropic-key
      secretRef:
        name: ai-credentials
        key: ANTHROPIC_API_KEY
      env: ANTHROPIC_API_KEY
```

### Creating a Task

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: update-deps
  namespace: kubeopencode-system
spec:
  agentRef:
    name: default
  description: |
    Update go.mod to Go 1.25 and run go mod tidy.
    Ensure all tests pass after the upgrade.
  contexts:
    # Git repository to work on
    - type: Git
      name: source-code
      git:
        repository: https://github.com/example/my-project.git
        ref: main
      mountPath: /workspace/my-project

    # Additional context from ConfigMap
    - type: ConfigMap
      name: workflow-guide
      configMap:
        name: workflow-guides
        key: pr-workflow.md
```

### Batch Operations with Helm

For running the same task across multiple targets, use Helm templating:

```yaml
# values.yaml
tasks:
  - name: update-service-a
    repo: service-a
  - name: update-service-b
    repo: service-b

# templates/tasks.yaml
{{- range .Values.tasks }}
---
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: {{ .name }}
spec:
  agentRef:
    name: default
  description: "Update dependencies for {{ .repo }}"
  contexts:
    - type: Git
      name: source
      git:
        repository: "https://github.com/example/{{ .repo }}.git"
        ref: main
      mountPath: /workspace/{{ .repo }}
{{- end }}
```

```bash
# Generate and apply multiple tasks
helm template my-tasks ./chart | kubectl apply -f -
```

### Monitoring Progress

```bash
# Watch Task status
kubectl get tasks -n kubeopencode-system -w

# View detailed status with events
kubectl describe task update-deps -n kubeopencode-system

# View task logs via CLI
kubeoc task logs update-deps -n kubeopencode-system -f

# View task logs via kubectl
kubectl logs $(kubectl get task update-deps -n kubeopencode-system -o jsonpath='{.status.podName}') -c agent -n kubeopencode-system

# Access the UI (port 2746)
kubectl port-forward -n kubeopencode-system svc/kubeopencode-server 2746:2746
# Then open http://localhost:2746
```

## Uninstalling the Chart

```bash
helm uninstall kubeopencode --namespace kubeopencode-system
```

To also delete the namespace:

```bash
kubectl delete namespace kubeopencode-system
```

## Security Considerations

1. **Secrets Management**: Never commit secrets to Git. Use:
   - Kubernetes Secrets
   - External Secrets Operator
   - Sealed Secrets
   - HashiCorp Vault

2. **RBAC**: The chart creates minimal RBAC permissions:
   - Controller: Manages CRDs, Deployments, Services, Pods, ConfigMaps, Secrets
   - Server: Read access to CRDs and Pods for UI/API

3. **Network Policies**: Consider adding NetworkPolicies to restrict traffic

4. **Pod Security**: Controller runs with non-root user and dropped capabilities

## Troubleshooting

### Controller not starting

```bash
# Check controller logs
kubectl logs -n kubeopencode-system deployment/kubeopencode-controller

# Check RBAC permissions
kubectl auth can-i create tasks --as=system:serviceaccount:kubeopencode-system:kubeopencode-controller -n kubeopencode-system
```

### Tasks failing

```bash
# List failed Tasks
kubectl get tasks -n kubeopencode-system --field-selector status.phase=Failed

# Check Task conditions for error details
kubectl get task <task-name> -n kubeopencode-system -o jsonpath='{.status.conditions}' | jq .

# View Task pod logs
kubeoc task logs <task-name> -n kubeopencode-system

# Describe Task for events
kubectl describe task <task-name> -n kubeopencode-system
```

### Agent not ready

```bash
# Check Agent status
kubeoc get agents -n kubeopencode-system

# Check Deployment
kubectl get deployment -n kubeopencode-system -l kubeopencode.io/agent

# View Deployment events
kubectl describe deployment <agent-deployment> -n kubeopencode-system
```

## Contributing

See the main project [README](../../README.md) for contribution guidelines.

## License

Copyright Contributors to the KubeOpenCode project. Licensed under the Apache License 2.0.
