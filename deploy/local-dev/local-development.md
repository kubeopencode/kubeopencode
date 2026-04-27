# Local Development Guide

This guide describes how to set up a local development environment for KubeOpenCode using Kind (Kubernetes in Docker).

## Prerequisites

- Docker
- Kind (`brew install kind` on macOS)
- kubectl
- Helm 3.x
- Go 1.25+

## Quick Start

### 1. Create or Use Existing Kind Cluster

Check if you already have a Kind cluster running:

```bash
kind get clusters
```

If you have an existing cluster (e.g., `kind`), you can use it directly. Otherwise, create a new one:

```bash
kind create cluster --name kubeopencode
```

Verify the cluster is running:

```bash
kubectl cluster-info
```

**IMPORTANT:** Local-dev MUST use the `kubeopencode` Kind cluster, NOT `kubeopencode-e2e` (which is reserved for E2E tests). Always verify your current context before running any commands:

```bash
# Verify current context points to the kubeopencode cluster
kubectl config current-context
# Expected: kind-kubeopencode (NOT kind-kubeopencode-e2e)
```

**Note:** The examples below use `--name kubeopencode` for Kind commands. If using an existing cluster with a different name (e.g., `kind`), replace `--name kubeopencode` with your cluster name.

### 2. Build Images

Build all required images:

```bash
# Build the controller image
make docker-build

# Build all agent images (opencode, devbox, attach, etc.)
make agent-build-all
```

Or build individual agent images as needed:

```bash
make agent-build AGENT=opencode    # OpenCode init container (copies /opencode binary)
make agent-build AGENT=devbox      # Executor container (development environment)
make agent-build AGENT=attach      # Attach image (used by agentRef task Pods)
```

> **Note:** The `attach` image is required for Tasks that reference Agents via `agentRef`. `make agent-build-all` is recommended to avoid missing images.

**Note:** The unified kubeopencode image provides both controller and infrastructure utilities:
- `kubeopencode controller`: Kubernetes controller
- `kubeopencode git-init`: Git repository cloning for Git Context
- `kubeopencode save-session`: Workspace persistence for session resume

### 3. Load Images to Kind

Load images into the Kind cluster (required because Kind cannot pull from local Docker):

```bash
# Load controller image
kind load docker-image ghcr.io/kubeopencode/kubeopencode:latest --name kubeopencode

# Tag and load all agent images with :dev tag
for img in opencode devbox attach; do
  docker tag ghcr.io/kubeopencode/kubeopencode-agent-${img}:$(make -s print-version) ghcr.io/kubeopencode/kubeopencode-agent-${img}:dev
  kind load docker-image ghcr.io/kubeopencode/kubeopencode-agent-${img}:dev --name kubeopencode
done
```

> **Important:** All three agent images must be loaded. Missing the `attach` image will cause `agentRef` Tasks to fail with `ErrImagePull`.

> **Important: Avoid `:latest` tag for agent images.** The controller sets `imagePullPolicy: Always` for images with the `:latest` tag or no tag (standard Kubernetes convention). In Kind, this causes `ErrImagePull` because the cluster cannot pull from remote registries. The local-dev agent YAML files use the `:dev` tag to avoid this. If you change agent images, always use a non-latest tag.

### 4. Deploy with Helm

```bash
helm upgrade --install kubeopencode ./charts/kubeopencode \
  --namespace kubeopencode-system \
  --create-namespace \
  --set controller.image.tag=latest \
  --set controller.image.pullPolicy=Never \
  --set agent.image.pullPolicy=Never \
  --set server.enabled=true \
  --set server.image.tag=latest \
  --set server.image.pullPolicy=Never
```

> **Note:** The `server.enabled=true` deploys the UI server. You can omit it if you only need the controller.

> **Important:** The Helm chart defaults to `v<VERSION>` image tags, but `make docker-build` tags images as `<VERSION>` (without `v` prefix) and `latest`. You must explicitly set `controller.image.tag=latest` and `server.image.tag=latest` to match the locally built images. Without this, pods will fail with `ErrImageNeverPull`.

### 5. Verify Deployment

Check the controller is running:

```bash
kubectl get pods -n kubeopencode-system
```

Expected output:

```
NAME                                       READY   STATUS    RESTARTS   AGE
kubeopencode-controller-xxxxxxxxx-xxxxx    1/1     Running   0          30s
kubeopencode-server-xxxxxxxxx-xxxxx        1/1     Running   0          30s
```

> **Note:** If `server.enabled=false`, only the controller pod will be present.

Check CRDs are installed:

```bash
kubectl get crds | grep kubeopencode
```

Expected output:

```
agents.kubeopencode.io               <timestamp>
agenttemplates.kubeopencode.io       <timestamp>
kubeopencodeconfigs.kubeopencode.io   <timestamp>
tasks.kubeopencode.io                <timestamp>
```

Check controller logs:

```bash
kubectl logs -n kubeopencode-system deployment/kubeopencode-controller
```

## UI Server

KubeOpenCode includes a web UI for managing Tasks and viewing Agents. The UI is an "Agent as Application" platform that allows non-technical users to interact with AI agents.

### Access the UI

#### Option 1: Port Forward (Quick Access)

```bash
kubectl port-forward -n kubeopencode-system svc/kubeopencode-server 2746:2746
```

Then open http://localhost:2746 in your browser.

#### Option 2: NodePort (Kind Cluster)

Update Helm values to expose via NodePort:

```bash
helm upgrade kubeopencode ./charts/kubeopencode \
  --namespace kubeopencode-system \
  --set server.enabled=true \
  --set server.service.type=NodePort \
  --set server.service.nodePort=32746
```

Access via: http://localhost:32746

#### Option 3: Ingress

Enable ingress in Helm values:

```bash
helm upgrade kubeopencode ./charts/kubeopencode \
  --namespace kubeopencode-system \
  --set server.enabled=true \
  --set server.ingress.enabled=true \
  --set server.ingress.hosts[0].host=kubeopencode.local \
  --set server.ingress.hosts[0].paths[0].path=/ \
  --set server.ingress.hosts[0].paths[0].pathType=Prefix
```

Add to `/etc/hosts`:
```
127.0.0.1 kubeopencode.local
```

#### Option 4: HTTPRoute (Gateway API)

If your cluster has a [Gateway API](https://gateway-api.sigs.k8s.io/) controller installed:

```bash
helm upgrade kubeopencode ./charts/kubeopencode \
  --namespace kubeopencode-system \
  --set server.enabled=true \
  --set server.route.main.enabled=true \
  --set "server.route.main.parentRefs[0].name=my-gateway" \
  --set "server.route.main.hostnames[0]=kubeopencode.local"
```

> **Note:** SSE streaming may require additional gateway controller configuration
> (e.g., disabling response buffering and increasing timeouts).

### UI Features

| Feature | Description |
|---------|-------------|
| **Task List** | View all Tasks across namespaces with status filtering |
| **Task Detail** | View Task details, logs (real-time streaming) |
| **Task Create** | Create new Tasks with Agent selection (filtered by namespace permissions) |
| **Agent List** | Browse available Agents with namespace filter |
| **Agent Detail** | View Agent configuration, contexts, credentials |
| **AgentTemplate List** | Browse available AgentTemplates |
| **Filtering** | Filter resources by name and Kubernetes label selectors |
| **Pagination** | Server-side pagination for efficient browsing of large resource lists |

#### Resource Filtering

All list pages (Tasks, Agents) support filtering:

- **Name Filter**: Filter resources by name (substring match)
- **Label Selector**: Filter by Kubernetes labels using standard selector syntax (e.g., `app=myapp,env=prod`)

Filters are persisted in the URL as query parameters, making it easy to share filtered views with team members:

```
http://localhost:2746/agents?name=opencode&labels=env%3Dprod
```

#### Pagination

List pages use server-side pagination with 12 items per page. The pagination controls at the bottom of each list show:
- Current page range (e.g., "Showing 1 to 12 of 45")
- Previous/Next navigation buttons

#### Namespace Filtering

The Agent list page includes a namespace selector that allows you to filter Agents by namespace. This helps in multi-tenant environments where different teams have Agents in different namespaces.

#### Agent Availability

When creating a Task, only Agents in the same namespace as the Task are shown. Task and Agent must always be in the same namespace.

### Authentication

The UI uses ServiceAccount token authentication. For local development with port-forward, the server operates in a permissive mode suitable for testing.

For production, enable authentication with RBAC-based filtering:

```bash
helm upgrade kubeopencode ./charts/kubeopencode \
  --namespace kubeopencode-system \
  --set server.enabled=true \
  --set server.authEnabled=true \
  --set server.authAllowAnonymous=false
```

When authentication is enabled:
- Users must provide a Bearer token in the Authorization header
- The token is validated using the Kubernetes TokenReview API
- API requests are executed with user impersonation, respecting Kubernetes RBAC
- Users only see Agents and Tasks they have permission to access

### UI Development

To develop the UI locally with hot-reload:

```bash
# Terminal 1: Run the Go server (API backend)
make run-server

# Terminal 2: Run webpack dev server (frontend with hot-reload)
make ui-dev
```

The webpack dev server runs on http://localhost:3000 and proxies API requests to the Go server on port 2746.

To build the UI for production:

```bash
make ui-build
```

## Iterative Development

### Controller or UI Changes

Use the convenience target to rebuild and reload all images:

```bash
make local-dev-reload
```

This single command rebuilds the Docker image (including UI), loads it into the Kind cluster, and restarts all deployments.

> **Note:** `make docker-build` automatically tags the image as both `:VERSION` and `:latest`, so you never need to manually `docker tag` before loading into Kind.

For faster UI iteration during development, use the dev server instead (no Docker rebuild needed):

```bash
# Terminal 1: Run Go server locally (uses kubeconfig for API access)
make run-server

# Terminal 2: Run webpack dev server with hot-reload
make ui-dev
```

This provides instant feedback at http://localhost:3000 without rebuilding Docker images.

## Local Test Environment

For quick testing, use the pre-configured resources in `deploy/local-dev/`:

### Deploy Test Resources

The local-dev environment uses the free `opencode/big-pickle` model by default — **no API key required**.

```bash
# Deploy all resources (namespace, RBAC, template, agents)
kubectl apply -k deploy/local-dev/

# Verify the AgentTemplate
kubectl get agenttemplate -n test

# Verify the Agents are ready
kubectl get agent -n test

# Verify the agent deployments are created
kubectl get deployment -n test
```

### Resources Created

| Resource | Name | Description |
|----------|------|-------------|
| Namespace | `test` | Isolated namespace for testing |
| ~~Secret~~ | ~~opencode-credentials~~ | Not needed for free model (see secrets.yaml.example for paid models) |
| ServiceAccount | `kubeopencode-agent` | Agent service account |
| Role/RoleBinding | `kubeopencode-agent` | RBAC permissions |
| AgentTemplate | `team-base` | Shared base configuration (images, credentials, workspace) |
| Agent | `team-agent` | Persistent agent with session + workspace persistence |
| Agent | `dev-agent` | Lightweight agent (no persistence, ephemeral storage) |

### Features Demonstrated

The local-dev resources showcase the following features:

| Feature | Resource | Description |
|---------|----------|-------------|
| **AgentTemplate** | `team-base` | Shared config inherited by both Agents via `templateRef` |
| **Persistence** | `team-agent` | Persistent OpenCode server with session (1Gi) + workspace (5Gi) PVCs |
| **Suspend/Resume** | `team-agent` | Can be suspended to save compute (see below) |
| **Concurrency Control** | `team-agent` | Limited to 3 concurrent tasks |
| **Minimal Agent** | `dev-agent` | Agent without persistence (uses ephemeral storage) |
| **Agent Profile** | Both agents | Human-readable description for discovery |

### Test Tasks

#### Agent Task Test (agentRef)

```bash
kubectl apply -n test -f - <<EOF
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: server-test
spec:
  agentRef:
    name: team-agent
  description: "Say hello world"
EOF

# Check status
kubectl get task -n test
kubectl logs -n test server-test-pod -c agent
```

#### Minimal Agent Task Test

```bash
kubectl apply -n test -f - <<EOF
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: pod-test
spec:
  agentRef:
    name: dev-agent
  description: "What is 2+2?"
EOF

# Check status
kubectl get task -n test
kubectl logs -n test pod-test-pod -c agent
```

#### Concurrent Tasks Test

```bash
for i in 1 2 3; do
  kubectl apply -n test -f - <<EOF
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: concurrent-$i
spec:
  agentRef:
    name: team-agent
  description: "Count to $i"
EOF
done

# Watch progress
kubectl get task -n test -w
```

### Testing Persistence

Session and workspace persistence let the team-agent retain state across pod restarts (configured via the `persistence` field).

#### Verify PVCs Are Created

```bash
# After deploying the team-agent, check for PVCs
kubectl get pvc -n test

# Expected output:
# NAME                             STATUS   VOLUME   CAPACITY   AGE
# team-agent-server-sessions     Bound    ...      1Gi        ...
# team-agent-server-workspace    Bound    ...      5Gi        ...
```

#### Test Session Persistence

```bash
# Run a task to create a conversation
kubectl apply -n test -f - <<EOF
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: persist-test-1
spec:
  agentRef:
    name: team-agent
  description: "Remember that the secret code is 42"
EOF

# Wait for completion, then restart the server pod
kubectl rollout restart deployment/team-agent-server -n test
kubectl rollout status deployment/team-agent-server -n test

# The session history should survive the restart
```

#### Test Workspace Persistence

```bash
# Run a task that creates files in the workspace
kubectl apply -n test -f - <<EOF
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: persist-test-2
spec:
  agentRef:
    name: team-agent
  description: "Create a file called hello.txt with the content 'Hello from KubeOpenCode'"
EOF

# Restart the server pod
kubectl rollout restart deployment/team-agent-server -n test

# The workspace files should still be there after restart
```

### Testing Suspend/Resume

Agents can be suspended to save compute resources while retaining all data.

#### Suspend the Agent

```bash
# Edit the agent to set suspend: true
kubectl patch agent team-agent -n test --type=merge -p '
spec:
  suspend: true
'

# Verify the deployment is scaled to 0
kubectl get deployment -n test
# Expected: team-agent-server   0/0

# Check agent status
kubectl get agent team-agent -n test -o jsonpath='{.status.suspended}'
# Expected: true

# PVCs are retained (no data loss)
kubectl get pvc -n test
```

#### Tasks Queue While Suspended

```bash
# Create a task while agent is suspended
kubectl apply -n test -f - <<EOF
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: queued-test
spec:
  agentRef:
    name: team-agent
  description: "This will queue until the agent is resumed"
EOF

# Check task status - should be Queued with reason AgentSuspended
kubectl get task queued-test -n test -o jsonpath='{.status.phase}'
# Expected: Queued
```

#### Resume the Agent

```bash
# Resume the agent
kubectl patch agent team-agent -n test --type=merge -p '
spec:
  suspend: false
'

# Verify the deployment scales back up
kubectl get deployment -n test
# Expected: team-agent-server   1/1

# Queued tasks should automatically start running
kubectl get task -n test -w
```

### Testing AgentTemplate

The `team-base` AgentTemplate provides shared configuration for both agents.

#### View Template Configuration

```bash
# View the template
kubectl get agenttemplate team-base -n test -o yaml

# Check which agents reference this template
kubectl get agent -n test -l kubeopencode.io/agent-template=team-base
```

#### Verify Template Inheritance

```bash
# Both agents should have the label set by the controller
kubectl get agent -n test --show-labels
# Expected labels include: kubeopencode.io/agent-template=team-base
```

### Customization

#### Using Real Secrets (Paid Models)

If using a paid model, create a local secrets file (gitignored):

```bash
cp deploy/local-dev/secrets.yaml.example deploy/local-dev/secrets.yaml
# Edit secrets.yaml with real API key values
kubectl apply -f deploy/local-dev/secrets.yaml -n test
```

#### Using a Paid AI Model

The default `opencode/big-pickle` model is free and requires no API key. To use a paid model:

1. Create a secret with your API key:
   ```bash
   cp deploy/local-dev/secrets.yaml.example deploy/local-dev/secrets.yaml
   vim deploy/local-dev/secrets.yaml  # Add your real API key
   kubectl apply -f deploy/local-dev/secrets.yaml -n test
   ```

2. Edit `agenttemplate.yaml` to change the model and add credentials:
   ```yaml
   config:
     $schema: https://opencode.ai/config.json
     model: anthropic/claude-sonnet-4-20250514
     small_model: anthropic/claude-haiku-4-20250514
   credentials:
     - name: opencode
       secretRef:
         name: opencode-credentials
   ```

3. Uncomment the `credentials` section in agent YAML files.

Agent-level config overrides template config.

#### Adjusting Persistence Sizes

Edit `agent-persistent.yaml` to change PVC sizes:

```yaml
port: 4096
persistence:
  sessions:
    size: "2Gi"              # Increase session storage
    storageClassName: "gp3"  # Use a specific StorageClass
  workspace:
    size: "20Gi"             # Increase workspace storage
```

> **Note:** On Kind clusters, the default StorageClass (`standard`) is used. PVC resizing may not be supported depending on the provisioner.

### Stopping a Running Task

```bash
# Stop a task gracefully (Pod is deleted, Task marked as Stopped)
kubectl annotate task server-test kubeopencode.io/stop=true -n test

# Check status
kubectl get task server-test -n test -o jsonpath='{.status.phase}'
# Expected: Completed (with Stopped condition)
```

> **Note:** Logs are lost when a Task is stopped. Use `kubectl logs` before stopping to capture output.

## Cleanup

### Delete Test Resources

```bash
# Delete all tasks
kubectl delete task --all -n test

# Delete all test resources (PVCs are cleaned up via OwnerReference)
kubectl delete -k deploy/local-dev/
```

### Uninstall KubeOpenCode

```bash
helm uninstall kubeopencode -n kubeopencode-system
kubectl delete namespace kubeopencode-system
```

### Delete Kind Cluster

```bash
kind delete cluster --name kubeopencode
```

## Debugging Tools

### Reading OpenCode Stream JSON Output

When running Tasks with `--format json`, the output is in stream-json format which can be hard to read. We provide a utility script to format the output:

```bash
# Read from kubectl logs
kubectl logs <pod-name> -n kubeopencode-system | ./hack/opencode-stream-reader.sh

# Read from a saved log file
cat task-output.log | ./hack/opencode-stream-reader.sh
```

The script requires `jq` and converts the JSON stream into human-readable output with colors and formatting.

## Troubleshooting

### Image Pull Errors

If you see `ErrImagePull` or `ImagePullBackOff`, ensure:

1. Images are loaded into Kind: `docker exec kubeopencode-control-plane crictl images | grep kubeopencode`
2. `imagePullPolicy` is set to `Never` in Helm values
3. **Agent tasks:** The `attach` image (`kubeopencode-agent-attach`) must be loaded. This image is used by `agentRef` Task Pods to connect to the Agent's OpenCode server. Build and load it with:
   ```bash
   make agent-build AGENT=attach
   kind load docker-image ghcr.io/kubeopencode/kubeopencode-agent-attach:latest --name kubeopencode
   ```

### Controller Not Starting

Check controller logs:

```bash
kubectl logs -n kubeopencode-system deployment/kubeopencode-controller
```

Check events:

```bash
kubectl get events -n kubeopencode-system --sort-by='.lastTimestamp'
```

### CRDs Not Found

Ensure CRDs are installed:

```bash
kubectl get crds | grep kubeopencode
```

If missing, reinstall with Helm or apply manually:

```bash
kubectl apply -f deploy/crds/
```

### PVC Issues (Persistence)

If PVCs are stuck in `Pending`:

```bash
# Check PVC status
kubectl describe pvc -n test

# On Kind, ensure default StorageClass exists
kubectl get storageclass
```

Kind clusters include a `standard` StorageClass by default. If missing, recreate the cluster.
