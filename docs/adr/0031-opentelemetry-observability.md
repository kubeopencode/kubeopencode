# ADR 0031: OpenTelemetry Observability for Tasks and Agents

## Status

Superseded by [ADR 0040](0040-opentelemetry-metrics-observability.md)

## Date

2026-04-07

## Context

KubeOpenCode runs AI agents as Kubernetes workloads. Understanding what happens inside a Task — how many tokens were consumed, which tools were called, why the task was slow or failed — is critical for enterprise adoption. Users currently have limited visibility into Task execution.

### Current State

**KubeOpenCode side:**
- Prometheus metrics exist (`internal/controller/metrics.go`): `kubeopencode_tasks_total`, `kubeopencode_task_duration_seconds`, `kubeopencode_agent_capacity`, `kubeopencode_agent_queue_length`
- No distributed tracing or OpenTelemetry integration
- Agent Pod labels support PodMonitor/ServiceMonitor discovery

**OpenCode side:**
- OpenCode already supports OpenTelemetry via AI SDK's `experimental_telemetry` flag
- Configuration: `experimental.openTelemetry: true` in `opencode.json`
- Emits spans for every LLM call with metadata (userId, sessionId)
- Depends on `@opentelemetry/api@1.9.0` through the `ai` package
- Requires an external OTel Collector to receive spans (no built-in exporter)

**Kubernetes OTel ecosystem:**
- [OpenTelemetry Operator](https://opentelemetry.io/docs/kubernetes/operator/) supports auto-injection of collector sidecars via pod annotations
- OTel Collector can export to Jaeger, Tempo, OTLP endpoints, etc.
- K8s attributes processor automatically enriches spans with pod/namespace/node metadata

### Problem

1. Users cannot see what happened inside a Task (token usage, tool calls, LLM latency)
2. Failed Tasks are hard to debug — users only see "Failed" status with no breakdown
3. Cost attribution per namespace/team/agent is not possible
4. CronTask patterns (recurring tasks) have no performance trend visibility

## Decision

Integrate OpenTelemetry in phases, leveraging the existing OpenCode OTel support and the Kubernetes OTel Operator ecosystem.

### Phase 1: Enable OpenCode OTel Spans (Low effort, high value)

**Goal**: Surface LLM-level observability (token usage, latency, model info) for every Task.

**Changes:**

1. **Auto-inject OTel config into OpenCode** — When observability is enabled, the controller injects `experimental.openTelemetry: true` into the OpenCode config (`/tools/opencode.json`) during Pod construction in `pod_builder.go`.

2. **Propagate Task identity via environment variables** — Set `OTEL_RESOURCE_ATTRIBUTES` on the worker container:
   ```
   OTEL_RESOURCE_ATTRIBUTES=kubeopencode.task.name=<task>,kubeopencode.task.namespace=<ns>,kubeopencode.agent.name=<agent>
   ```
   This enriches all spans with Task/Agent identity without modifying OpenCode.

3. **OTel Collector sidecar injection** — Users deploy the [OpenTelemetry Operator](https://opentelemetry.io/docs/kubernetes/operator/) and create an `OpenTelemetryCollector` resource. KubeOpenCode adds the annotation `sidecar.opentelemetry.io/inject: "true"` to Task/Agent Pods when observability is enabled, triggering automatic collector sidecar injection by the OTel Operator.

4. **Configuration** — Add an `observability` field to `KubeOpenCodeConfig` (cluster-scoped):
   ```yaml
   apiVersion: kubeopencode.io/v1alpha1
   kind: KubeOpenCodeConfig
   metadata:
     name: cluster
   spec:
     observability:
       openTelemetry:
         enabled: true
         # Optional: override the OTel Collector sidecar injection annotation
         collectorAnnotation: "sidecar.opentelemetry.io/inject"
         collectorAnnotationValue: "true"
   ```

**What users get**: Every LLM call in every Task produces OTel spans visible in Jaeger/Tempo/Grafana, with Task/Agent/namespace identity attached.

### Phase 2: Controller-Side Tracing (Medium effort)

**Goal**: End-to-end trace from Task creation to completion.

**Changes:**

1. **Add OTel tracing to controller reconcile loops** — Use the Go OTel SDK (`go.opentelemetry.io/otel`) to create spans for key operations:
   - Task reconcile: `task.reconcile` (root span)
   - Pod creation: `task.pod.create`
   - Context resolution: `task.context.resolve` (git clone, configmap fetch, URL fetch)
   - Status updates: `task.status.update`

2. **Inject trace context into Pod** — Pass the `traceparent` header as an environment variable (`TRACEPARENT`) so OpenCode spans are linked to the controller's trace. This creates a single trace spanning controller → init containers → OpenCode LLM calls.

3. **CronTask trace correlation** — CronTask-triggered Tasks carry a `kubeopencode.crontask.name` attribute, enabling queries like "show me all traces for CronTask X over the past week".

**What users get**: A single trace view showing the full lifecycle: reconcile → git-init → context-init → LLM call 1 → tool call → LLM call 2 → completion.

### Phase 3: OTel Metrics (Complement Prometheus)

**Goal**: Export LLM-specific metrics via OTel for cost and performance dashboards.

This phase bridges the gap between existing Prometheus metrics (infrastructure-level) and LLM-specific metrics:

| Metric | Source | Purpose |
|--------|--------|---------|
| `kubeopencode.llm.tokens.input` | OTel spans | Cost attribution |
| `kubeopencode.llm.tokens.output` | OTel spans | Cost attribution |
| `kubeopencode.llm.duration` | OTel spans | Latency tracking |
| `kubeopencode.llm.calls` | OTel spans | Usage patterns |
| `kubeopencode.tool.calls` | OTel spans | Tool usage analysis |

These can be derived from spans using the OTel Collector's [Span Metrics Connector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/spanmetricsconnector), requiring no code changes — just collector configuration.

**What users get**: Grafana dashboards showing token costs by namespace/agent/user, LLM latency trends, and tool usage breakdown.

### What We Explicitly Do NOT Build

- **Custom tracing UI** — Use existing tools (Jaeger, Grafana Tempo)
- **Built-in OTel Collector** — Users bring their own via the OTel Operator
- **Mandatory dependency** — OTel is opt-in; KubeOpenCode works without it
- **Log collection** — Out of scope; handled by standard K8s logging stacks

## Consequences

### Positive

- **Zero-dependency for basic use** — OTel is purely opt-in via `KubeOpenCodeConfig`
- **Leverages existing ecosystem** — OTel Operator + Collector + Grafana; no custom infrastructure
- **Enterprise value** — Cost visibility, debugging, and audit trails are top enterprise requirements
- **Incremental** — Each phase delivers standalone value; no big-bang rollout
- **OpenCode already supports it** — Phase 1 mostly wires up existing capabilities

### Negative

- **OTel Operator dependency for full value** — Users need to install the OTel Operator separately
- **Experimental flag risk** — OpenCode's `experimental_telemetry` could change or be removed upstream
- **Sidecar overhead** — OTel Collector sidecar adds ~50MB memory per Pod (configurable)
- **Phase 2 adds Go dependencies** — `go.opentelemetry.io/otel` and related packages

### Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| OpenCode removes `experimental_telemetry` | It's moving toward stable in AI SDK; if removed, we adapt the config injection |
| OTel Collector sidecar adds latency | Collector is async (batch exporter); no impact on LLM call latency |
| Too much trace data for long Tasks | Configure sampling in OTel Collector (head or tail sampling) |

## References

- [OpenTelemetry Operator for Kubernetes](https://opentelemetry.io/docs/kubernetes/operator/)
- [AI SDK Telemetry](https://sdk.vercel.ai/docs/ai-sdk-core/telemetry)
- [OTel Collector Span Metrics Connector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/spanmetricsconnector)
- ADR 0013: Defer Token Usage Tracking to Post-v0.1 (related — OTel spans provide token data)
