# ADR 0020: Enterprise Readiness Roadmap

## Status

Proposed

## Context

KubeOpenCode is designed as a Kubernetes-native AI agent platform, but its current feature set primarily addresses the "happy path" — a single team, trusted network, no compliance requirements. As adoption grows, enterprise environments expose fundamental gaps that block production deployment.

[Issue #105](https://github.com/kubeopencode/kubeopencode/issues/105) is a concrete example: a user cannot clone a Git repository from an internal server with a custom CA certificate. This single issue represents a broader class of enterprise infrastructure challenges — firewalls, proxies, private registries, multi-tenant isolation, audit requirements — that KubeOpenCode does not yet address.

This ADR catalogs the enterprise readiness gaps, proposes solutions organized into workstreams, and establishes priorities to guide the roadmap from v0.1 to v1.0.

### Current State Assessment

| Area | Current State | Enterprise Expectation |
|------|--------------|----------------------|
| Network | Assumes direct internet access | Custom CAs, HTTP proxies, air-gapped environments |
| Observability | `kubectl logs` + Task status conditions | Structured logs, Prometheus metrics, OpenTelemetry traces |
| Multi-tenancy | Single cluster-scoped KubeOpenCodeConfig | Per-namespace config, ResourceQuota integration, tenant isolation |
| Security | Basic ServiceAccount, no pod security enforcement | Pod Security Standards, non-root, read-only rootfs, secret rotation |
| Reliability | No timeout, no retry, no priority | Configurable timeouts, retry policies, priority queuing |
| Compliance | No audit trail, no cost tracking | Full audit log, token usage tracking, data sovereignty |
| Operations | Manual Helm install | HA controller, CRD versioning, upgrade path |
| AI Governance | No guardrails | Token budgets, prompt audit, output validation |

## Decision

We will address enterprise readiness through **seven workstreams**, each independently deliverable, prioritized by deployment-blocking severity.

---

### Workstream 1: Enterprise Network Infrastructure (P0)

**Trigger:** Issue #105 — custom CA certificates in init containers.

Enterprise networks are characterized by: self-signed CA certificates, mandatory HTTP/HTTPS proxies, private container registries, and network segmentation. KubeOpenCode must work within these constraints without requiring users to build custom images.

#### 1.1 Custom CA Certificates

Add a `caBundle` field to Agent and KubeOpenCodeConfig that mounts CA certificates into all containers (init containers and worker containers).

**Agent-level (per-agent override):**

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  caBundle:
    configMapRef:
      name: corporate-ca-bundle    # ConfigMap containing CA certificates
      key: ca-bundle.crt           # Optional: specific key (default: ca-bundle.crt)
```

**Cluster-level (global default via KubeOpenCodeConfig):**

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeConfig
metadata:
  name: cluster
spec:
  caBundle:
    configMapRef:
      name: corporate-ca-bundle
      namespace: kubeopencode-system  # Cluster-scoped, so namespace is required
```

**Implementation:**
- Mount the ConfigMap as a volume at `/etc/ssl/certs/kubeopencode/`
- Set `SSL_CERT_DIR` (or `SSL_CERT_FILE`) and `GIT_SSL_CAINFO` env vars in all containers
- Agent-level `caBundle` overrides cluster-level if both are set
- Works with trust-manager (cert-manager) which populates ConfigMaps with CA bundles

**Affected containers:**
- `git-init` (Git clone — this is what #105 hits)
- `url-fetch` (URL context fetching)
- `context-init` (ConfigMap/Text context — not network-dependent, but include for consistency)
- Worker container (AI agent may access internal APIs)

#### 1.2 HTTP/HTTPS Proxy Configuration

Add `proxy` configuration to KubeOpenCodeConfig and Agent.

**Cluster-level:**

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeConfig
metadata:
  name: cluster
spec:
  proxy:
    httpProxy: "http://proxy.corp.example.com:8080"
    httpsProxy: "http://proxy.corp.example.com:8080"
    noProxy: "localhost,127.0.0.1,10.0.0.0/8,.svc,.cluster.local,.corp.example.com"
```

**Implementation:**
- Set `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` (and lowercase variants) in all containers
- Agent-level `proxy` overrides cluster-level
- `.svc` and `.cluster.local` are always appended to `noProxy` to prevent proxying in-cluster traffic

#### 1.3 Private Registry Authentication

Add `imagePullSecrets` to KubeOpenCodeConfig and Agent so that agent images can be pulled from private registries without modifying the default ServiceAccount.

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  imagePullSecrets:
    - name: harbor-registry-secret
```

**Implementation:**
- Propagate `imagePullSecrets` to generated Pod specs
- Cluster-level defaults apply when Agent-level is not set
- Controller does NOT need pull secrets — only the Pods it creates do

---

### Workstream 2: Observability (P1)

Without observability, operators cannot answer: "How many Tasks ran today? How many failed? Why? How long did they take? How much did they cost?"

#### 2.1 Prometheus Metrics

Expose controller metrics via `/metrics` endpoint (controller-runtime already provides the framework).

**Task metrics:**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `kubeopencode_tasks_total` | Counter | `namespace`, `agent`, `phase` | Total Tasks by final phase |
| `kubeopencode_task_duration_seconds` | Histogram | `namespace`, `agent`, `phase` | Task execution duration |
| `kubeopencode_tasks_active` | Gauge | `namespace`, `agent`, `phase` | Currently active Tasks by phase |
| `kubeopencode_task_queue_depth` | Gauge | `namespace`, `agent` | Tasks in `Queued` phase |

**Agent metrics:**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `kubeopencode_agent_concurrent_tasks` | Gauge | `namespace`, `agent` | Current concurrent running Tasks |
| `kubeopencode_agent_concurrent_limit` | Gauge | `namespace`, `agent` | maxConcurrentTasks limit |
| `kubeopencode_agent_quota_remaining` | Gauge | `namespace`, `agent` | Remaining quota in current window |

**Controller health metrics:**

| Metric | Type | Description |
|--------|------|-------------|
| `kubeopencode_reconcile_errors_total` | Counter | Reconciliation errors |
| `kubeopencode_reconcile_duration_seconds` | Histogram | Reconciliation latency |

**Implementation:**
- Use `controller-runtime/pkg/metrics` to register custom metrics
- Update metrics in reconcile loops (minimal overhead)
- Add Grafana dashboard JSON to `deploy/monitoring/` (optional)
- Add ServiceMonitor CR for Prometheus Operator users

#### 2.2 Structured Logging

Migrate from unstructured log messages to structured JSON logging.

**Current:** `log.Info("Pod created", "task", task.Name, "pod", pod.Name)`

This already uses structured key-value pairs via `logr`. Ensure:
- All log lines include `task`, `namespace`, `agent` fields consistently
- Error logs include `error` field with wrapped context
- Add correlation ID (`task.UID`) to all logs for a given Task lifecycle
- Controller startup logs version, build info, feature gates

#### 2.3 OpenTelemetry Traces (Future)

Add tracing spans for the Task lifecycle:

```
Task Created → Context Resolved → Pod Created → Pod Running → Pod Completed → Status Updated
```

This is lower priority than metrics — defer to post-v0.2 unless a specific enterprise customer requires it.

#### 2.4 Task Result Persistence

Currently, Task output is only available via `kubectl logs` and is lost when the Pod is deleted or the Task is cleaned up. Add a mechanism to persist Task results.

**Option A: Status field (small results)**

```go
type TaskExecutionStatus struct {
    // ... existing fields ...
    Result    string `json:"result,omitempty"`    // Short result summary (max 10KB)
    OutputRef string `json:"outputRef,omitempty"` // Reference to full output (ConfigMap, PVC, S3)
}
```

**Option B: ConfigMap-backed output (medium results)**

Controller creates a ConfigMap with the Task's output after completion, referenced in `status.outputRef`. Limited to 1MB (etcd value size limit).

**Option C: External storage (large results, future)**

Support PVC or S3-compatible storage for large outputs. This requires significant complexity and should be deferred.

**Recommendation:** Start with Option A (status field with size limit). It covers the most common case (did the task succeed? what was the summary?) with minimal complexity.

---

### Workstream 3: Security Hardening (P0)

Enterprise security teams will audit KubeOpenCode's Pod specs before approving deployment. Current defaults are too permissive.

#### 3.1 Pod Security Standards

Enforce `restricted` Pod Security Standards by default in generated Pods.

```go
// Default security context for all generated containers
SecurityContext: &corev1.SecurityContext{
    RunAsNonRoot:             ptr.To(true),
    ReadOnlyRootFilesystem:   ptr.To(true),
    AllowPrivilegeEscalation: ptr.To(false),
    Capabilities: &corev1.Capabilities{
        Drop: []corev1.Capability{"ALL"},
    },
    SeccompProfile: &corev1.SeccompProfile{
        Type: corev1.SeccompProfileTypeRuntimeDefault,
    },
}
```

**Challenges:**
- Agent containers need writable directories for `/workspace`, `/tmp`, `/home`
- Solution: Add `emptyDir` volumes for writable paths while keeping rootfs read-only
- Init containers (`git-init`, `context-init`) also need writable temp directories

**Agent-level override:**

```yaml
spec:
  podSpec:
    securityContext:
      runAsUser: 1000
      runAsGroup: 1000
      fsGroup: 1000
```

#### 3.2 NetworkPolicy Templates

Provide optional NetworkPolicy templates in Helm chart to restrict Agent Pod network access.

```yaml
# Deny all egress by default, allow only AI API endpoints
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: kubeopencode-agent-egress
spec:
  podSelector:
    matchLabels:
      kubeopencode.io/component: task-pod
  policyTypes: ["Egress"]
  egress:
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
      ports:
        - port: 443
          protocol: TCP
```

This should be opt-in via Helm values, not enforced by default — network requirements vary widely.

#### 3.3 Sensitive Data Protection

- Never log Secret values in controller logs (already a best practice, but audit for compliance)
- Add `--redact-secrets` controller flag that redacts env var values in debug logs
- Consider adding output scanning for common secret patterns (API keys, tokens) in Task results

---

### Workstream 4: Multi-Tenancy (P1)

When multiple teams share a KubeOpenCode installation, they need isolation, quotas, and independent configuration.

#### 4.1 Namespace-Scoped Configuration

Current `KubeOpenCodeConfig` is cluster-scoped. Add a namespace-scoped resource for per-team settings.

**Option A: Namespace-scoped KubeOpenCodeConfig**

Introduce a new resource `KubeOpenCodeNamespaceConfig` (namespace-scoped) that overrides cluster-level settings:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeNamespaceConfig
metadata:
  name: config
  namespace: team-alpha
spec:
  cleanup:
    ttlSecondsAfterFinished: 1800  # Team wants faster cleanup
  proxy:
    httpsProxy: "http://team-alpha-proxy:8080"  # Team-specific proxy
  defaults:
    taskTimeout: 30m  # Team-specific default timeout
```

**Resolution order:** Agent field > KubeOpenCodeNamespaceConfig > KubeOpenCodeConfig > built-in defaults.

**Option B: Annotations on Namespace**

Use annotations on the Namespace resource itself. Simpler but less structured.

**Recommendation:** Option A — a dedicated CRD is more discoverable, validatable, and self-documenting.

#### 4.2 Resource Quotas and Limits

Integrate with Kubernetes ResourceQuota naturally — since KubeOpenCode creates Pods, existing quota enforcement applies. Document this clearly rather than building custom quota logic.

Additionally, add KubeOpenCode-specific quotas to `KubeOpenCodeNamespaceConfig`:

```yaml
spec:
  quotas:
    maxActiveTasks: 50          # Max concurrent Tasks in this namespace
    maxTasksPerHour: 200        # Rate limit for Task creation
```

These are enforced by a validating webhook, not by Pod-level ResourceQuota.

#### 4.3 RBAC Guidance

Document recommended ClusterRole/Role bindings for common personas:

| Persona | Resources | Verbs |
|---------|-----------|-------|
| Task Creator | tasks | create, get, list, watch, delete |
| Agent Admin | agents, kubeopencodenamespaceco nfigs | create, get, list, watch, update, delete |
| Platform Admin | kubeopencode configs, agents (all ns) | * |
| Viewer | tasks, agents | get, list, watch |

Provide these as Helm templates with opt-in via `rbac.createDefaultRoles: true`.

---

### Workstream 5: Reliability and Lifecycle (P0-P1)

#### 5.1 Task Timeout (P0)

Without timeouts, a stuck AI agent consumes resources indefinitely. This is the highest-priority reliability gap.

**Implementation:**

Add `timeout` field to Task spec and Agent spec (as default):

```yaml
# Task-level timeout
apiVersion: kubeopencode.io/v1alpha1
kind: Task
metadata:
  name: fix-bug
spec:
  agentRef: my-agent
  instruction: "Fix the null pointer exception"
  timeout: 30m  # Task-level override

---
# Agent-level default timeout
apiVersion: kubeopencode.io/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  timeout: 1h  # Default for all Tasks using this Agent
```

**Resolution order:** Task `timeout` > Agent `timeout` > KubeOpenCodeConfig default > no timeout (backward compatible).

**Behavior when timeout is reached:**
- Controller annotates the Task with `kubeopencode.io/stop=true` (reuses existing stop mechanism)
- Task transitions to `Completed` with condition `Stopped`, reason `Timeout`
- This is consistent with the existing stop flow — no new Pod deletion logic needed

**Implementation in controller:**

```go
// In reconcile loop, check if running Task has exceeded timeout
if task.Status.Phase == TaskPhaseRunning && task.Status.StartTime != nil {
    timeout := resolveTimeout(task, agent, config)
    if timeout > 0 && time.Since(task.Status.StartTime.Time) > timeout {
        // Trigger stop via annotation
        task.Annotations["kubeopencode.io/stop"] = "true"
    }
}
```

#### 5.2 Task Retry Policy (P1)

Allow automatic retry of failed Tasks with configurable backoff.

```yaml
spec:
  retryPolicy:
    maxRetries: 3
    backoff: exponential  # linear | exponential | fixed
    initialDelay: 30s
    maxDelay: 10m
```

**Implementation:**
- Track retry count in `status.retryCount`
- On failure, if `retryCount < maxRetries`, re-create the Pod after delay
- Use controller-runtime's `RequeueAfter` for backoff timing
- Each retry creates a new Pod (not reuse) — consistent with existing architecture

#### 5.3 Task Priority (P2)

When `maxConcurrentTasks` causes queuing, higher-priority Tasks should be scheduled first.

```yaml
spec:
  priority: 100  # Higher number = higher priority (default: 0)
```

**Implementation:**
- When dequeuing Tasks, sort by `priority` descending, then by creation timestamp (FIFO within same priority)
- Map to Kubernetes PriorityClass for Pod scheduling priority (separate concern but aligned)

#### 5.4 Controller High Availability (P1)

Controller-runtime already supports leader election. Ensure it's enabled and tested:

```yaml
# Helm values
controller:
  replicas: 2
  leaderElection:
    enabled: true
```

Verify that:
- Only the leader reconciles Tasks/Agents
- Failover completes within ~15 seconds (default lease duration)
- No duplicate Pod creation during failover

---

### Workstream 6: Compliance and Governance (P2)

#### 6.1 Audit Logging

Emit Kubernetes Events for all significant Task lifecycle transitions:

| Event | Reason | Message |
|-------|--------|---------|
| Task created | `TaskCreated` | `Task fix-bug created by user:admin, agent: my-agent` |
| Pod created | `PodCreated` | `Pod fix-bug-abc123 created for Task fix-bug` |
| Task completed | `TaskCompleted` | `Task fix-bug completed in 5m23s` |
| Task failed | `TaskFailed` | `Task fix-bug failed: exit code 1` |
| Task stopped | `TaskStopped` | `Task fix-bug stopped by user annotation` |
| Task timed out | `TaskTimeout` | `Task fix-bug timed out after 1h` |
| Task queued | `TaskQueued` | `Task fix-bug queued: maxConcurrentTasks (3) reached` |
| Quota exceeded | `QuotaExceeded` | `Task fix-bug queued: quota 10/hour exceeded` |

These events are queryable via `kubectl get events` and can be forwarded to SIEM systems (Splunk, Elastic) via standard Kubernetes event exporters.

For deeper audit (who created what Task with what instruction), integrate with Kubernetes audit logging (API server level) — this requires no KubeOpenCode changes, just cluster configuration guidance.

#### 6.2 Token Usage Tracking

Deferred per ADR 0013. When implemented, token usage enables:
- Per-Task cost visibility
- Per-namespace cost aggregation (chargeback)
- Token budget enforcement (reject Tasks that would exceed budget)

#### 6.3 Data Sovereignty

For regulated industries, ensure AI data does not leave specific regions:

- **Pod scheduling constraints:** Document how to use `nodeSelector`, `affinity`, and `tolerations` in `podSpec` to constrain Task execution to specific nodes/zones
- **AI endpoint control:** The Agent's `config` field already controls which AI model/endpoint is used — document how to point to region-specific endpoints
- No KubeOpenCode-specific features needed — leverage Kubernetes primitives

---

### Workstream 7: AI-Specific Governance (P2-P3)

These features differentiate an AI agent platform from a generic task runner.

#### 7.1 Token Budget (P2)

Add per-Agent or per-namespace token limits (requires ADR 0013 token tracking first):

```yaml
# In Agent spec
spec:
  tokenBudget:
    maxTokensPerTask: 1000000     # 1M tokens max per Task
    maxTokensPerDay: 10000000     # 10M tokens max per day across all Tasks
```

When the budget is exceeded:
- Per-task: AI agent receives a signal to wrap up (requires OpenCode support)
- Per-day: New Tasks are rejected with a clear error message

#### 7.2 Output Validation (P3)

Optional post-Task validation hooks:

```yaml
spec:
  validation:
    - name: security-scan
      image: aquasec/trivy:latest
      command: ["trivy", "fs", "--exit-code", "1", "/workspace"]
```

This runs a validation container after the AI agent completes, before marking the Task as `Completed`. If validation fails, the Task is marked `Failed` with reason `ValidationFailed`.

**Note:** This is a significant feature that may not be needed for v1.0. Evaluate based on enterprise customer feedback.

#### 7.3 Prompt Audit (P2)

For regulated industries (finance, healthcare), log all Task instructions and AI outputs:

- Task `instruction` is already stored in the Task CR (queryable via API server audit log)
- AI output persistence requires Workstream 2.4 (Task Result Persistence)
- Combined with Kubernetes audit logging, this provides a complete prompt-to-output audit trail

No new KubeOpenCode features needed — document how to configure Kubernetes audit logging to capture Task CR create/update events with request bodies.

---

## Priority Summary

| Priority | Workstream | Items | Rationale |
|----------|-----------|-------|-----------|
| **P0** | Network Infrastructure | CA certs, proxy, registry auth | Deployment blocker in enterprise networks |
| **P0** | Task Timeout | Timeout field + enforcement | Resource leak prevention |
| **P0** | Security Hardening | Pod Security Standards, non-root | Security team gate |
| **P1** | Observability | Prometheus metrics, structured logs | Cannot operate without metrics |
| **P1** | Multi-Tenancy | Namespace config, RBAC guidance | Multi-team deployment |
| **P1** | Reliability | Retry policy, HA controller | Production reliability |
| **P2** | Compliance | Audit events, prompt audit | Regulated industry requirement |
| **P2** | AI Governance | Token budget | Cost control |
| **P3** | AI Governance | Output validation, guardrails | Long-term differentiator |

## Consequences

### Positive

1. **Removes deployment blockers**: P0 items (CA certs, timeout, pod security) are the minimum for enterprise adoption
2. **Incremental delivery**: Each workstream is independently deliverable — no "big bang" release required
3. **Leverages Kubernetes primitives**: Many enterprise features (quotas, scheduling, audit logging) are solved by documenting Kubernetes integration rather than building custom logic
4. **Backward compatible**: All new fields are optional with sensible defaults; existing deployments continue to work unchanged

### Negative

1. **API surface growth**: New fields in Agent, Task, KubeOpenCodeConfig, and a new KubeOpenCodeNamespaceConfig CRD increase API complexity
2. **Configuration hierarchy complexity**: Four-level resolution (Task > Agent > NamespaceConfig > ClusterConfig > default) can be confusing — requires clear documentation and `kubectl describe` output
3. **Maintenance burden**: More features mean more code to maintain, test, and document
4. **Scope creep risk**: Enterprise features can expand indefinitely — strict prioritization and "document, don't build" for Kubernetes-native solutions is essential

### What We Explicitly Defer

| Feature | Reason | Revisit When |
|---------|--------|-------------|
| Token tracking | Upstream OpenCode dependency (ADR 0013) | OpenCode adds `stats --format json` |
| OpenTelemetry tracing | Low demand relative to metrics | Customer request or post-v0.2 |
| Cross-cluster management | Significant complexity, low initial demand | Multi-cluster customer |
| Custom admission webhooks | Can use external policy engines (OPA/Kyverno) | V1.0 |
| AI output scanning/guardrails | Requires deep AI integration | Post-v1.0 |

## References

- [Issue #105: Custom CA cert in init-containers](https://github.com/kubeopencode/kubeopencode/issues/105)
- ADR 0013: Defer Token Usage Tracking (`docs/adr/0013-defer-token-usage-tracking.md`)
- ADR 0016: Human-in-the-Loop Design (`docs/adr/0016-human-in-the-loop-design.md`)
- [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [Kubernetes Audit Logging](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/)
- [cert-manager trust-manager](https://cert-manager.io/docs/trust/trust-manager/)
- [Prometheus Operator ServiceMonitor](https://prometheus-operator.dev/docs/user-guides/getting-started/)
