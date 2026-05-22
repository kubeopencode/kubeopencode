# ADR 0040: OpenTelemetry Metrics & Observability Integration

## Status

Proposed

## Date

2026-05-22

## Context

This ADR supersedes [ADR 0031](0031-opentelemetry-observability.md) with updated findings based on direct source-code analysis of OpenCode's current OpenTelemetry support and the latest OTel GenAI Semantic Conventions.

### OpenCode's Current OpenTelemetry Support (Source Analysis)

OpenCode has **two distinct layers** of OTel support:

#### Layer 1: Built-in OTel Exporter (Core Observability)

**Location**: `packages/core/src/effect/observability.ts`

OpenCode has a **native, zero-config OTel exporter** activated by setting `OTEL_EXPORTER_OTLP_ENDPOINT`:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://collector:4318 opencode
```

When this env var is set, OpenCode automatically:
- **Traces**: Creates an `OTLPTraceExporter` sending to `${ENDPOINT}/v1/traces` via HTTP/JSON batch processor
- **Logs**: Creates an `OtlpLogger` sending to `${ENDPOINT}/v1/logs` via Effect's OTLP log serialization
- **Context Manager**: Registers `AsyncLocalStorageContextManager` so that non-Effect code (like AI SDK) correctly propagates span context (parent-child linkage works across the entire call stack)
- **Resource Attributes**: Attaches `service.name=opencode`, `service.version`, `opencode.client`, `opencode.process_role`, `opencode.run_id`, `deployment.environment.name`, plus any user-set `OTEL_RESOURCE_ATTRIBUTES`

**Key insight**: This is NOT the `experimental.openTelemetry` config flag. This is the foundation layer that creates the OTel provider and context manager.

#### Layer 2: AI SDK Telemetry (Per-Request Spans)

**Location**: `packages/opencode/src/session/llm.ts` (lines 202-335), `packages/opencode/src/agent/agent.ts` (lines 392-406)

When `experimental.openTelemetry: true` is set in `opencode.json`, OpenCode:

1. Gets the Effect OTel tracer via `OtelTracer.OtelTracer` service
2. Wraps it in a **Proxy** that injects `session.id` as an attribute on every span:
   ```typescript
   const telemetryTracer = tracer
     ? new Proxy(tracer, {
         get(target, prop, receiver) {
           if (prop !== "startSpan") return Reflect.get(target, prop, receiver)
           return (...args) => {
             const span = target.startSpan(...args)
             span.setAttribute("session.id", input.sessionID)
             return span
           }
         },
       })
     : undefined
   ```
3. Passes it to Vercel AI SDK's `experimental_telemetry` in `streamText()`:
   ```typescript
   experimental_telemetry: {
     isEnabled: cfg.experimental?.openTelemetry,
     functionId: "session.llm",
     tracer: telemetryTracer,
     metadata: { userId: cfg.username ?? "unknown", sessionId: input.sessionID },
   }
   ```

The AI SDK then emits **standard GenAI semantic convention spans** including:
- `gen_ai.request.model` — model name
- `gen_ai.usage.input_tokens` / `gen_ai.usage.output_tokens` — token counts
- `gen_ai.response.finish_reasons` — stop reasons
- `gen_ai.input.messages` / `gen_ai.output.messages` — full conversation (when content recording is enabled)

#### Layer 3: CLI Run Spans

**Location**: `packages/opencode/src/cli/cmd/run/otel.ts`

OpenCode's CLI runtime wraps operations in spans via `withRunSpan(name, attributes, fn)`. This uses the `@opentelemetry/api` tracer directly (not Effect's), creating spans under the `"opencode.run"` tracer. This is used for tool execution, session operations, etc.

**Configuration Summary**:

| Mechanism | Activation | What It Does |
|-----------|-----------|--------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` env var | Set to collector URL | Enables trace + log export, context propagation |
| `OTEL_EXPORTER_OTLP_HEADERS` env var | Optional headers | Auth/API keys for the collector |
| `OTEL_RESOURCE_ATTRIBUTES` env var | Optional attributes | Custom resource attributes on all spans |
| `experimental.openTelemetry: true` in config | In `opencode.json` | Enables AI SDK LLM call spans with session/user metadata |
| `opencode-plugin-otel` | npm plugin | Community plugin for OTLP/gRPC export |

### OTel GenAI Semantic Conventions (Best Practices)

The [OpenTelemetry GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/) define standard attributes for AI workloads:

**Span attributes (every LLM call)**:
- `gen_ai.system` — provider identifier (e.g., `openai`, `anthropic`)
- `gen_ai.request.model` — model requested
- `gen_ai.response.model` — model actually used (may differ)
- `gen_ai.usage.input_tokens` / `gen_ai.usage.output_tokens`
- `gen_ai.response.finish_reasons`
- `gen_ai.operation.name` — e.g., `chat`

**Metrics (derived from spans via Span Metrics Connector)**:
- `gen_ai.client.operation.duration` — histogram of LLM call latencies
- `gen_ai.client.token.usage` — histogram of token consumption by type

**AI Agent spans** (emerging standard):
- Agent execution span with tool call sub-spans
- `ai.agent.id`, `ai.tool.name` attributes

**Kubernetes attributes** (K8s SemConv RC):
- `k8s.pod.name`, `k8s.namespace.name`, `k8s.node.name`
- Auto-enriched by OTel Collector's k8s attributes processor

### KubeOpenCode Current State

**Existing Prometheus metrics** (`internal/controller/metrics.go`):
- `kubeopencode_tasks_total` (gauge, by namespace/phase)
- `kubeopencode_task_duration_seconds` (histogram, by namespace/agent)
- `kubeopencode_agent_capacity` (gauge, remaining capacity)
- `kubeopencode_agent_queue_length` (gauge, queued tasks)

**Existing ADR 0031** proposed a 3-phase approach but was based on the assumption that OpenCode only supports `experimental.openTelemetry` in config. The actual implementation is richer — OpenCode has native OTLP export built in, requiring only `OTEL_EXPORTER_OTLP_ENDPOINT`.

**What's missing**:
1. No way to activate OpenCode's OTel from KubeOpenCode (env vars not injected)
2. No controller-side tracing (Task lifecycle spans)
3. No LLM-specific metrics (tokens, latency, cost)
4. No trace context propagation (controller → Agent Pod)
5. No OTel Collector integration guidance

## Key Insight: OpenCode Already Does the Heavy Lifting

The most important finding from this investigation is that **OpenCode has built-in, production-grade OTel support** that requires zero code changes. It is activated purely by environment variables:

| Env Var | Effect |
|---------|--------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Enables trace + log export via OTLP/HTTP |
| `OTEL_EXPORTER_OTLP_HEADERS` | Auth headers for collector |
| `OTEL_RESOURCE_ATTRIBUTES` | Custom attributes on all spans |
| `experimental.openTelemetry: true` (in config) | Enables AI SDK GenAI semantic convention spans |

This means **Phase 1 is primarily a KubeOpenCode-side change** — inject env vars into Pod specs. OpenCode does the rest automatically.

The AI SDK's `experimental_telemetry` flag produces spans that follow the [OTel GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/), including:
- Per-LLM-call spans with model, token counts, and latency
- `session.id` and `userId` metadata (injected by OpenCode's tracer proxy)
- Tool call spans with tool name and arguments

This is the exact data needed for cost attribution, performance debugging, and enterprise audit trails.

## Decision

Adopt a **4-phase approach** that leverages OpenCode's built-in OTel capabilities progressively, from zero-code-enablement to full distributed tracing.

### Phase 1: Enable OpenCode Built-in OTel (Zero code changes in OpenCode)

**Goal**: Surface LLM-level traces and logs for every Task with minimal KubeOpenCode changes.

**How**: OpenCode already exports OTLP when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. KubeOpenCode just needs to inject this env var into Agent Pods.

**Changes in KubeOpenCode**:

1. **Add `observability` to `KubeOpenCodeConfig`** (cluster-scoped):
   ```yaml
   apiVersion: kubeopencode.io/v1alpha1
   kind: KubeOpenCodeConfig
   metadata:
     name: cluster
   spec:
     observability:
       openTelemetry:
         enabled: true
         # Endpoint for the OTel Collector. Required when enabled.
         # Supports both in-cluster and external URLs.
         endpoint: "http://otel-collector.observability:4318"
         # Optional headers for collector authentication
         headers: {}
         # Inject experimental.openTelemetry into OpenCode config to enable LLM call traces
         enableLLMTraces: true
         # Resource attributes to add to all spans
         resourceAttributes:
           kubeopencode.cluster.name: "production"
   ```

2. **Inject OTel env vars in `pod_builder.go`** — When observability is enabled, the controller injects these environment variables into the executor container:
   ```
   OTEL_EXPORTER_OTLP_ENDPOINT=<from config>
   OTEL_EXPORTER_OTLP_HEADERS=<from config, if set>
   OTEL_RESOURCE_ATTRIBUTES=kubeopencode.task.name=<task>,kubeopencode.task.namespace=<ns>,kubeopencode.agent.name=<agent>,k8s.namespace.name=<ns>,k8s.pod.name=<pod>
   ```

3. **Inject `experimental.openTelemetry: true` into OpenCode config** — When `enableLLMTraces: true`, the controller merges this into the OpenCode config JSON during Pod construction. This activates AI SDK's `experimental_telemetry` which produces GenAI semantic convention spans for every LLM call with token counts, model info, and session identity.

4. **Add OTel Collector sidecar injection annotation** — When configured, add `sidecar.opentelemetry.io/inject: "kubeopencode-collector"` annotation to Agent/Task Pods. This works with the [OpenTelemetry Operator](https://opentelemetry.io/docs/kubernetes/operator/) for automatic sidecar injection. The annotation value is either `"true"` (use default Collector in same namespace) or the name of a specific `OpenTelemetryCollector` CR. KubeOpenCode uses the value configured in `collectorInjection.collectorName`, defaulting to `"true"` if unset.

5. **Update `api/v1alpha1/config_types.go`** — Add `ObservabilitySpec` struct with `OpenTelemetryConfig` nested type.

6. **Update Helm chart RBAC** — Controller needs to read `KubeOpenCodeConfig` for observability config.

**What users get**: Every LLM call in every Task produces OTel spans with GenAI semantic conventions (token usage, model, latency) visible in Jaeger/Tempo/Grafana. Logs flow to the same OTLP endpoint. Zero code changes in OpenCode required.

**Protocol note**: OpenCode uses OTLP/HTTP (port 4318), not gRPC (port 4317). The configured `endpoint` MUST point to an OTLP/HTTP receiver. The controller appends `/v1/traces`, `/v1/logs`, and `/v1/metrics` automatically per OTel spec. If your existing OTel Collector exposes only gRPC, enable the HTTP receiver in its config or front it with the HTTP-to-gRPC bridge.

**Effort**: Low (primarily pod_builder.go env var injection + config API)

### Phase 2: Controller-Side Tracing (End-to-end trace correlation)

**Goal**: Create a single trace spanning the entire Task lifecycle from creation to completion.

**Changes**:

1. **Add Go OTel SDK** — Import `go.opentelemetry.io/otel` and related packages:
   ```
   go.opentelemetry.io/otel
   go.opentelemetry.io/otel/trace
   go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
   go.opentelemetry.io/otel/sdk/resource
   semconv go.opentelemetry.io/otel/semconv/v1.30.0
   ```

2. **Instrument reconcile loops** — Create spans for key operations:
   - `task.reconcile` — root span per reconciliation
   - `task.pod.create` — Pod creation
   - `task.context.resolve` — context init (git clone, configmap, URL)
   - `task.status.update` — status transitions
   - `agent.reconcile` — Agent lifecycle operations
   - `crontask.trigger` — CronTask → Task creation

3. **Propagate trace context to Pod** — Inject `TRACEPARENT` env var into the executor container so OpenCode's spans link to the controller's trace. This creates parent-child span linkage:
   ```
   Controller (task.reconcile) → Init Containers → OpenCode LLM spans → Tool spans → Completion
   ```

4. **Configure OTel SDK in controller** — The controller itself becomes an OTel service:
   ```yaml
   # Controller deployment gets OTel env vars
   OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector.observability:4318"
   OTEL_SERVICE_NAME: "kubeopencode-controller"
   ```

5. **CronTask correlation** — Each Task created by a CronTask gets `kubeopencode.crontask.name` as a span attribute and an OTel Span Link (`go.opentelemetry.io/otel/trace`.Link) to the previous successful Task's root span when available. This enables both attribute-based queries ('show all traces for CronTask X') and graph-based navigation across recurring runs.

**What users get**: A single trace view from Task creation → Pod scheduling → Init containers → LLM call chain → Tool execution → Completion. Full causal linkage.

**Effort**: Medium (Go OTel SDK integration, reconcile instrumentation, trace propagation)

### Phase 3: OTel Metrics for LLM Operations (Complement Prometheus)

**Goal**: Export LLM-specific metrics via OTel for cost dashboards and performance analysis.

This phase has two paths:

#### Path A: Span-to-Metrics via OTel Collector (Zero code, recommended first)

Use the [OTel Collector Span Metrics Connector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/spanmetricsconnector) to derive metrics from GenAI spans produced by Phase 1:

```yaml
# OTel Collector config example
connectors:
  spanmetrics:
    histogram:
      explicit:
        buckets: [100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s, 30s]
    dimensions:
      - name: gen_ai.request.model
      - name: gen_ai.system
      - name: kubeopencode.agent.name
      - name: kubeopencode.task.namespace
    metrics_flush_interval: 15s
```

This produces standard metrics from existing spans:
| Metric | Source | Purpose |
|--------|--------|---------|
| `gen_ai.client.operation.duration` | OTel spans | LLM latency by model/agent |
| `gen_ai.client.token.usage` | OTel spans | Token cost by namespace/agent |
| `calls` (span count) | OTel spans | LLM call volume |

**No KubeOpenCode code changes** — just collector configuration.

#### Path B: Native OTel Metrics in Controller (Additional custom metrics)

Add Go OTel metrics for controller-side operations not covered by span metrics:

```go
// In the controller's init or setup function
meter := otel.GetMeterProvider().Meter("kubeopencode/controller")

llmTokenInput, _ := meter.Int64Counter(
    "kubeopencode.llm.tokens.input",
    metric.WithDescription("Input tokens consumed"),
)

llmTokenOutput, _ := meter.Int64Counter(
    "kubeopencode.llm.tokens.output",
    metric.WithDescription("Output tokens consumed"),
)

taskReconcileDuration, _ := meter.Float64Histogram(
    "kubeopencode.task.reconcile.duration",
    metric.WithDescription("Time spent in task reconciliation"),
)
```

Additionally, expose **existing Prometheus metrics via OTel** by using the OTel Prometheus bridge (`go.opentelemetry.io/otel/exporters/prometheus`), making all existing `kubeopencode_*` metrics available in both Prometheus and OTLP formats.

**What users get**: Grafana dashboards with token costs by namespace/agent/model, LLM latency trends, and tool usage breakdown. Existing Prometheus metrics are also available via OTLP.

**Effort**: Low (Path A is config-only) / Medium (Path B requires Go metrics code)

### Phase 4: UI Observability Dashboard

**Goal**: Surface key observability data in the KubeOpenCode UI without requiring external tools.

**Changes**:

1. **Task detail page metrics panel** — Show token usage, LLM call count, and latency summary from the server API. The server queries the user-configured metrics backend (Prometheus/Mimir/VictoriaMetrics) — KubeOpenCode does NOT query the Collector directly. The backend URL is configured via `KubeOpenCodeConfig.spec.observability.metricsBackend.queryURL`.

2. **Agent metrics page** — Show capacity, queue depth, and LLM cost over time. Use existing Prometheus metrics + new OTel metrics.

3. **Trace link** — For each Task, show a "View Trace" link that opens Jaeger/Tempo/Grafana filtered to that Task's trace ID. This requires storing the root trace ID in `status.traceID` (set in Phase 2).

4. **CronTask trend graphs** — Show performance trends across recurring Task runs.

**Effort**: Medium (UI + server API endpoints)

### What We Explicitly Do NOT Build

- **Custom tracing UI** — Use existing tools (Jaeger, Grafana Tempo, SigNoz)
- **Built-in OTel Collector** — Users bring their own via the OTel Operator
- **Mandatory dependency** — OTel is opt-in; KubeOpenCode works without it
- **Log aggregation** — Out of scope; handled by standard K8s logging stacks
- **Our own OTel SDK for OpenCode** — OpenCode already has native support

### Security & Privacy Considerations

GenAI telemetry can expose sensitive data. KubeOpenCode must apply the following guardrails:

**Prompt and response content**:
- The OTel GenAI semantic conventions define `gen_ai.input.messages` and `gen_ai.output.messages` attributes that capture full prompts and LLM responses. These may contain API keys, PII, customer data, or proprietary code.
- **Default**: KubeOpenCode injects `experimental.openTelemetry: true` but does NOT enable content recording. The AI SDK's `recordInputs`/`recordOutputs` flags stay off unless the user explicitly opts in via:
  ```yaml
  spec:
    observability:
      openTelemetry:
        recordContent: false  # default; set true only in trusted environments
  ```
- When `recordContent: true`, the controller injects `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=true` per OTel GenAI spec.

**Credentials and secret redaction**:
- Resource attributes injected by the controller MUST NOT include any value from `Agent.spec.credentials`, env var references to Secrets, or auth headers.
- The controller only sets identity attributes (Task/Agent/namespace names), never credential material.

**Transport security**:
- The `endpoint` field accepts `https://` URLs for external/SaaS backends. TLS verification is enforced by the underlying OTel SDK (no `insecure` flag exposed in KubeOpenCodeConfig).
- For mutual TLS to in-cluster Collectors, users should rely on service mesh (Istio/Linkerd) or front the Collector with a TLS-terminating proxy.

**Headers and auth tokens**:
- `OTEL_EXPORTER_OTLP_HEADERS` is passed through to OpenCode as-is. To avoid leaking SaaS API keys, the controller resolves header values from K8s Secrets via:
  ```yaml
  spec:
    observability:
      openTelemetry:
        headers:
          x-honeycomb-team:
            valueFrom:
              secretKeyRef:
                name: honeycomb-credentials
                key: api-key
  ```

**Sampling for sensitive workloads**:
- For regulated workloads, configure tail-sampling in the OTel Collector to drop spans containing prompts before they leave the cluster boundary.

## Kubernetes OpenTelemetry Ecosystem Integration

KubeOpenCode runs in Kubernetes, where a mature OTel ecosystem already exists. Rather than reinventing collection, processing, or storage, KubeOpenCode integrates with these established projects. This section describes the integration surface for each major K8s OTel product.

### The Big Picture

```
┌─────────────────────────────────────────────────────────────────┐
│                    KubeOpenCode Components                       │
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐       │
│  │  Controller  │    │  Agent Pod   │    │  Server/UI   │       │
│  │  (Go OTel)   │    │  (OpenCode   │    │  (Go OTel)   │       │
│  │              │    │   OTLP)      │    │              │       │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘       │
│         │                   │                   │               │
│         │ OTLP/HTTP         │ OTLP/HTTP         │ OTLP/HTTP     │
│         ▼                   ▼                   ▼               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
              ┌─────────────────────────────────┐
              │  OpenTelemetry Collector        │
              │  (DaemonSet / Sidecar / Gateway)│
              │  - k8sattributes processor      │
              │  - batch processor              │
              │  - spanmetrics connector        │
              └────────────────┬────────────────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
              ▼                ▼                ▼
       ┌──────────┐     ┌──────────┐    ┌──────────┐
       │ Jaeger / │     │Prometheus│    │   Loki   │
       │  Tempo   │     │/Mimir/VM │    │          │
       │ (traces) │     │ (metrics)│    │  (logs)  │
       └──────────┘     └──────────┘    └──────────┘
                               │
                               ▼
                         ┌──────────┐
                         │ Grafana /│
                         │ SigNoz / │
                         │ Datadog  │
                         └──────────┘
```

KubeOpenCode produces telemetry; the OTel Collector routes it; backends store and visualize it. KubeOpenCode commits to producing **standard, well-attributed OTLP data**, leaving routing/storage/visualization choices entirely to the user.

### 1. OpenTelemetry Operator (Sidecar & Auto-Instrumentation)

The [OpenTelemetry Operator](https://opentelemetry.io/docs/kubernetes/operator/) is the recommended way to deploy and manage Collectors in K8s. KubeOpenCode integrates in two ways:

**A. OpenTelemetryCollector CRD** — Users deploy a Collector via:

```yaml
apiVersion: opentelemetry.io/v1beta1
kind: OpenTelemetryCollector
metadata:
  name: kubeopencode-collector
  namespace: observability
spec:
  mode: sidecar  # or daemonset/deployment/statefulset
  config:
    receivers:
      otlp:
        protocols:
          http:
            endpoint: 0.0.0.0:4318
    processors:
      k8sattributes:
        passthrough: false
        extract:
          metadata:
            - k8s.pod.name
            - k8s.namespace.name
            - k8s.node.name
      batch:
        timeout: 10s
    exporters:
      otlphttp/tempo:
        endpoint: http://tempo.observability:4318
    service:
      pipelines:
        traces: { receivers: [otlp], processors: [k8sattributes, batch], exporters: [otlphttp/tempo] }
```

**B. Sidecar injection annotation** — When `KubeOpenCodeConfig.spec.observability.openTelemetry.collectorInjection: true`, the controller adds the annotation to Agent/Task Pods:

```yaml
metadata:
  annotations:
    sidecar.opentelemetry.io/inject: "kubeopencode-collector"
```

The OTel Operator's mutating webhook then injects the Collector sidecar automatically. **No KubeOpenCode code touches the Operator API directly** — we just add the annotation.

**C. Auto-instrumentation (NOT used)** — The Operator's `Instrumentation` CRD for Java/Python/Node auto-instrumentation is **not applicable** to KubeOpenCode because OpenCode is already instrumented natively.

**Minimum versions**: OTel Operator v0.90.0+ (for v1beta1 OpenTelemetryCollector CRD), OTel Collector Contrib v0.95.0+ (for spanmetrics connector with GenAI attributes), Kubernetes 1.26+ (for stable Downward API features used in DaemonSet mode).

### 2. OpenTelemetry Collector Deployment Patterns

KubeOpenCode supports all three Collector deployment patterns. Users choose based on scale and isolation needs:

| Pattern | When to Use | KubeOpenCode Config |
|---------|------------|--------------------|
| **Sidecar** (per Pod) | Strong isolation, per-tenant Collectors, dev/test | `endpoint: http://localhost:4318` + sidecar annotation |
| **DaemonSet** (per Node) | Most production K8s clusters, balanced perf/cost | `endpoint: http://$(HOST_IP):4318` (uses Downward API) |
| **Gateway** (centralized) | Multi-cluster, large scale, central control | `endpoint: http://otel-gateway.observability:4318` |

The `endpoint` field in `KubeOpenCodeConfig.spec.observability.openTelemetry` is the only required setting; the deployment topology is the user's choice.

For DaemonSet mode, KubeOpenCode injects the Node IP via Downward API:

```yaml
env:
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: "http://$(HOST_IP):4318"
- name: HOST_IP
  valueFrom:
    fieldRef:
      fieldPath: status.hostIP
```

This is handled transparently in `pod_builder.go` when `endpoint` contains the `$(HOST_IP)` template.

### 3. K8s Attributes Processor (Automatic Enrichment)

The [k8sattributes processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/k8sattributesprocessor) auto-enriches all telemetry with K8s metadata (pod name, namespace, node, labels, annotations) by querying the K8s API.

**KubeOpenCode's role**: Set the [Kubernetes Semantic Convention](https://opentelemetry.io/docs/specs/semconv/resource/k8s/) attributes that allow the processor to correlate spans with Pods. We already inject these in Phase 1:

```
OTEL_RESOURCE_ATTRIBUTES=k8s.pod.name=$(POD_NAME),k8s.namespace.name=$(POD_NAMESPACE),...
```

**User's role**: Configure the processor in their Collector to add additional metadata (labels, annotations, node info). No KubeOpenCode change needed.

**Critical**: Pod IP-based discovery works automatically when the processor runs with proper RBAC (`get`, `list`, `watch` on pods/namespaces/nodes). The OTel Operator handles this RBAC when deploying via the `OpenTelemetryCollector` CRD.

### 4. Prometheus Integration (Existing Metrics Bridge)

KubeOpenCode currently exposes Prometheus metrics on `/metrics`. The K8s ecosystem provides multiple paths to integrate these with OTel:

**Path A: Prometheus Receiver in Collector** (recommended, no KubeOpenCode change):

```yaml
receivers:
  prometheus:
    config:
      scrape_configs:
      - job_name: 'kubeopencode-controller'
        kubernetes_sd_configs:
        - role: pod
        relabel_configs:
        - source_labels: [__meta_kubernetes_pod_label_app]
          action: keep
          regex: kubeopencode-controller
```

The Collector scrapes existing `/metrics` endpoints and converts to OTLP. **Zero KubeOpenCode code change.**

**Path B: Target Allocator** (for large clusters):

The [OTel Target Allocator](https://github.com/open-telemetry/opentelemetry-operator/tree/main/cmd/otel-allocator) shards Prometheus targets across Collector replicas. Configured via the `OpenTelemetryCollector` CRD with `spec.targetAllocator.enabled: true`. **No KubeOpenCode change needed**; the existing `PodMonitor`/`ServiceMonitor` resources are discovered automatically.

**Path C: OTel Prometheus Exporter in Controller** (Phase 3 Path B):

The controller registers an OTel `Meter` and exposes metrics in both Prometheus (`/metrics`) and OTLP formats simultaneously. Allows gradual migration without breaking existing dashboards.

### 5. Prometheus Operator Integration (PodMonitor/ServiceMonitor)

KubeOpenCode Pods already carry labels suitable for [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator) discovery:

```yaml
# Helm chart adds these
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: kubeopencode-agents
spec:
  selector:
    matchLabels:
      kubeopencode.io/agent: "true"
  podMetricsEndpoints:
  - port: metrics
    interval: 30s
```

**Phase 1 addition**: Helm chart will ship optional `PodMonitor`/`ServiceMonitor` manifests for the controller, server, and Agent Pods, gated by `metrics.serviceMonitor.enabled: true`. These work with Prometheus Operator, Grafana Mimir, VictoriaMetrics Operator, and any compatible collector.

### 6. Service Mesh Integration (Istio, Linkerd)

If users run KubeOpenCode under a service mesh, mesh-level traces (HTTP/gRPC calls between Pods) are automatically captured by the mesh. **KubeOpenCode's OTel spans must carry the W3C `traceparent` header to merge with mesh traces.**

**Phase 2 requirement**: The controller's outgoing HTTP client (when calling OpenCode API or external services) must inject `traceparent`. The Go OTel SDK does this automatically via `otelhttp.NewTransport()`. OpenCode's HTTP client already supports this via its OTel context manager.

Result: A single trace shows `Istio Envoy → KubeOpenCode Controller → Agent Pod (OpenCode) → LLM API` as one continuous trace.

### 7. Backend-Agnostic Output

KubeOpenCode commits to OTLP-compliant output. The Collector's exporters handle translation to any backend:

| Backend | Collector Exporter | Notes |
|---------|-------------------|-------|
| **Jaeger** | `jaeger` / `otlp` | Native OTLP support in Jaeger v1.35+ |
| **Grafana Tempo** | `otlphttp` | First-class OTLP; recommended for K8s |
| **Grafana Loki** | `loki` | Logs only |
| **Prometheus / Mimir / VictoriaMetrics** | `prometheusremotewrite` / `otlphttp` | Metrics |
| **SigNoz** (open-source) | `otlp` | Native; tested with OpenCode per [SigNoz docs](https://signoz.io/docs/opencode-observability/) |
| **Datadog** | `datadog` | Commercial |
| **New Relic** | `otlp` | Native OTLP |
| **Honeycomb** | `otlp` | Native OTLP |
| **Dash0, Elastic, Splunk, Dynatrace** | `otlp` / vendor-specific | All support OTLP |

**KubeOpenCode does not bundle any vendor-specific exporter.** Users choose by editing their Collector config.

### 8. KubeCon-Native Patterns (CNCF Stack)

The default "open-source CNCF stack" we test against:

```
KubeOpenCode → OTel Collector (DaemonSet via OTel Operator)
            ├─ Tempo (traces, backed by S3)
            ├─ Mimir (metrics, backed by S3)
            └─ Loki (logs, backed by S3)
                    ↓
                Grafana (visualization)
```

This is the reference deployment we will document in `website/docs/observability.md` and validate in E2E tests with the [opentelemetry-demo](https://github.com/open-telemetry/opentelemetry-demo) infrastructure.

### 9. KubeOpenCodeConfig Schema Updates

To support the above, the schema becomes:

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeConfig
metadata:
  name: cluster
spec:
  observability:
    openTelemetry:
      enabled: true

      # Where to send telemetry. Supports:
      # - In-cluster service: http://otel-collector.observability:4318
      # - DaemonSet via Downward API: http://$(HOST_IP):4318
      # - Sidecar: http://localhost:4318
      # - External: https://api.honeycomb.io
      endpoint: "http://otel-collector.observability:4318"

      # Optional headers (e.g., for SaaS backends). Values can be inline or
      # resolved from a Secret via valueFrom.secretKeyRef to avoid leaking
      # API keys in the KubeOpenCodeConfig.
      headers:
        x-honeycomb-team:
          valueFrom:
            secretKeyRef:
              name: honeycomb-credentials
              key: api-key

      # OTel Operator sidecar injection
      collectorInjection:
        enabled: false
        # Name of the OpenTelemetryCollector CR to inject from
        collectorName: "kubeopencode-collector"

      # Inject experimental.openTelemetry into OpenCode config to enable LLM call traces
      enableLLMTraces: true

      # Whether to record full prompt/response content on LLM spans.
      # Default false; set true only in trusted environments. When true,
      # the controller injects OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=true
      # per OTel GenAI spec.
      recordContent: false

      # Additional resource attributes for all spans
      resourceAttributes:
        kubeopencode.cluster.name: "production"

      # Signal-level toggles
      signals:
        traces: true
        metrics: true
        logs: false  # logs default off to reduce volume

    # Metrics backend used by Phase 4 UI dashboards. KubeOpenCode does NOT
    # query the OTel Collector directly; it queries the user-configured
    # metrics backend (Prometheus/Mimir/VictoriaMetrics).
    metricsBackend:
      queryURL: "http://prometheus.observability:9090"
```

This schema is intentionally **declarative and backend-agnostic**. KubeOpenCode never knows about Jaeger, Tempo, or any specific backend — that's the Collector's job.

## Consequences

### Positive

- **Phase 1 is extremely low effort** — OpenCode already implements OTel; we just wire it up
- **Zero-dependency for basic use** — OTel is opt-in via `KubeOpenCodeConfig`
- **Leverages existing ecosystem** — OTel Operator + Collector + Grafana; no custom infrastructure
- **Enterprise value** — Cost visibility, debugging, and audit trails are top enterprise requirements
- **Standards-aligned** — GenAI semantic conventions ensure compatibility with all OTel backends
- **Incremental** — Each phase delivers standalone value; no big-bang rollout
- **No OpenCode fork required** — All integration points are env vars and config injection
- **Backward compatible** — Existing `kubeopencode_*` Prometheus metrics are preserved; OTel metrics are additive. Users can adopt OTel incrementally without breaking existing Prometheus/Grafana dashboards.

### Negative

- **OTel Operator dependency for sidecar injection** — Users need to install it separately (but it's optional)
- **Experimental flag risk** — AI SDK's `experimental_telemetry` is technically experimental, though OTel GenAI semconv are maturing rapidly (GA as of May 2026)
- **Sidecar overhead** — OTel Collector sidecar adds ~50MB memory per Pod (only when using sidecar mode; gateway mode has no per-Pod overhead)
- **Phase 2 adds Go dependencies** — `go.opentelemetry.io/otel` and related packages increase binary size

### Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| AI SDK changes `experimental_telemetry` API | It's converging with stable OTel GenAI semconv; also we control the config injection point |
| Too much trace data for long Tasks | Configure sampling in OTel Collector (head/tail sampling); user controls this |
| OTel Collector adds latency | Collector is async (batch exporter); no impact on LLM call latency |
| Controller binary size increase (Phase 2) | OTel Go SDK is ~2MB; acceptable for controller |
| Span Metrics Connector produces too many series | Configure dimensionality in collector config; user controls this |

## Implementation Priority

| Phase | Effort | Value | Priority |
|-------|--------|-------|----------|
| Phase 1: Enable OpenCode OTel | Low | High | P0 — Immediate |
| Phase 3 Path A: Span-to-Metrics | Low (config-only) | High | P0 — With Phase 1 |
| Phase 2: Controller Tracing | Medium | Medium | P1 — Next iteration |
| Phase 3 Path B: Native OTel Metrics | Medium | Medium | P2 — As needed |
| Phase 4: UI Dashboard | Medium | Low-Medium | P3 — After core metrics work |

## References

- [OpenCode Core Observability](../opencode/packages/core/src/effect/observability.ts) — OTLP trace + log exporter
- [OpenCode LLM Telemetry](../opencode/packages/opencode/src/session/llm.ts) — AI SDK `experimental_telemetry` integration
- [OpenCode CLI OTel](../opencode/packages/opencode/src/cli/cmd/run/otel.ts) — CLI run span wrapper
- [OTel GenAI Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/)
- [OTel AI Agent Observability Blog](https://opentelemetry.io/blog/2025/ai-agent-observability/)
- [OTel GenAI Observability Blog](https://opentelemetry.io/blog/2026/genai-observability/)
- [SigNoz OpenCode Observability Guide](https://signoz.io/docs/opencode-observability/)
- [opencode-plugin-otel](https://github.com/DEVtheOPS/opencode-plugin-otel) — Community OTLP/gRPC plugin
- [OTel Span Metrics Connector](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/connector/spanmetricsconnector)
- [OTel Operator for Kubernetes](https://opentelemetry.io/docs/kubernetes/operator/)
- [Vercel AI SDK Telemetry](https://sdk.vercel.ai/docs/ai-sdk-core/telemetry)
- ADR 0031: Original OpenTelemetry proposal (superseded by this ADR)
