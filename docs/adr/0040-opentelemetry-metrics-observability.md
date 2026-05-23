# ADR 0040: OpenTelemetry Metrics & Observability Integration

## Status

Proposed

## Date

2026-05-22

## Context

This ADR supersedes [ADR 0031](0031-opentelemetry-observability.md) with updated findings based on direct source-code analysis of OpenCode's current OpenTelemetry support and the latest OTel GenAI Semantic Conventions.

### OpenCode's Current OpenTelemetry Support (Source Analysis)

OpenCode has **three layers** of OTel support. These layers are **complementary, not redundant** — each has a distinct responsibility, and they form a dependency chain where Layer 1 is the prerequisite infrastructure for Layers 2 and 3.

#### Layer 1: Built-in OTel Infrastructure (Provider + Exporter)

**Location**: `packages/core/src/effect/observability.ts`

OpenCode has a **native OTel infrastructure layer** activated by setting `OTEL_EXPORTER_OTLP_ENDPOINT`:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://collector:4318 opencode
```

When this env var is set, OpenCode automatically:
- **Traces**: Creates a `NodeTracerProvider` with `OTLPTraceExporter` sending to `${ENDPOINT}/v1/traces` via HTTP/JSON batch processor
- **Logs**: Creates an `OtlpLogger` sending to `${ENDPOINT}/v1/logs` via Effect's OTLP log serialization
- **Context Manager**: Registers `AsyncLocalStorageContextManager` as the global context manager on `@opentelemetry/api`, so that non-Effect code (like the AI SDK) correctly propagates span context — parent-child linkage works across the entire call stack
- **Resource Attributes**: Attaches `service.name=opencode`, `service.version`, `opencode.client`, `opencode.process_role`, `opencode.run_id`, `deployment.environment.name`, plus any user-set `OTEL_RESOURCE_ATTRIBUTES`

**Critical distinction**: Layer 1 does **not produce any business spans itself**. Its role is purely infrastructure — it creates the TracerProvider, registers the context manager, and wires exporters. Layers 2 and 3 are the business-span producers that rely on this infrastructure. Without Layer 1, Layers 2 and 3 have no provider to obtain tracers from and no exporter to send spans through.

This is NOT the `experimental.openTelemetry` config flag. This is the foundation layer that creates the OTel provider and context manager.

#### Layer 2: AI SDK Telemetry (LLM Call Spans — GenAI Semantic Conventions)

**Location**: `packages/opencode/src/session/llm.ts` (lines 202-335), `packages/opencode/src/agent/agent.ts` (lines 392-406)

**Dependency**: Requires Layer 1 to be active (`OTEL_EXPORTER_OTLP_ENDPOINT` must be set). Layer 2 obtains its tracer from Layer 1's TracerProvider via the `OtelTracer.OtelTracer` Effect service, and relies on Layer 1's `AsyncLocalStorageContextManager` for correct parent-child span linkage with Layer 3 spans.

When `experimental.openTelemetry: true` is set in `opencode.json`, OpenCode:

1. Gets the Effect OTel tracer from Layer 1's TracerProvider via `OtelTracer.OtelTracer` service
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

The AI SDK then emits spans with **partial GenAI semantic convention support** — it produces both its own `ai.*` private attributes and a subset of `gen_ai.*` standard attributes, including:
- `gen_ai.system` — provider identifier
- `gen_ai.request.model` — model name
- `gen_ai.usage.input_tokens` / `gen_ai.usage.output_tokens` — token counts
- `gen_ai.response.finish_reasons` — stop reasons
- `gen_ai.response.model` — model actually used
- `ai.model.id`, `ai.model.provider` — AI SDK private attributes
- `ai.usage.inputTokens`, `ai.usage.outputTokens` — AI SDK private token attributes
- `ai.response.finishReason`, `ai.response.text` — AI SDK private response attributes

**Note**: The AI SDK does NOT fully conform to OTel GenAI Semantic Conventions. Span names use AI SDK's own convention (`ai.generateText`, `ai.streamText`, `ai.toolCall`) rather than the spec's `gen_ai.client.chat`. Some spec attributes (e.g., `gen_ai.operation.name`, `gen_ai.system_instructions`) are not emitted. Content recording (`gen_ai.input.messages`, `gen_ai.output.messages`) requires explicit opt-in. See [GitHub issue #2590](https://github.com/vercel/ai/issues/2590) for full compatibility tracking.

#### Layer 3: CLI Run Spans (Application Operation Spans)

**Location**: `packages/opencode/src/cli/cmd/run/otel.ts`

**Dependency**: Requires Layer 1 to be active. Layer 3 explicitly initializes Layer 1 via `ManagedRuntime.make(Observability.layer)` and obtains its tracer from the global `@opentelemetry/api` provider that Layer 1 registers (`trace.getTracer("opencode.run")`). If `Observability.enabled` is false (i.e., Layer 1 is not active), Layer 3 silently degrades to a no-op span.

OpenCode's CLI runtime wraps operations in spans via `withRunSpan(name, attributes, fn)`. This uses the `@opentelemetry/api` tracer directly (not Effect's), creating spans under the `"opencode.run"` tracer. Span names include:
- `RunInteractive.session` — overall session lifecycle
- `RunInteractive.turn` — a single user-AI interaction turn
- `RunInteractive.localMode` / `RunInteractive.attachMode` — session mode
- `RunLifecycle.boot` / `RunLifecycle.close` — runtime startup/shutdown
- `RunScrollbackStream.complete`, `RunFooter.flush` — UI rendering operations

These spans provide **application-level** visibility (session flow, turn boundaries, tool execution) at a coarser granularity than Layer 2's per-LLM-call spans.

#### Layer Dependency & Span Hierarchy

The three layers form a strict dependency chain:

```
Layer 1 (Infrastructure)  ← activated by OTEL_EXPORTER_OTLP_ENDPOINT
  ├── Layer 2 (LLM spans) ← additionally requires experimental.openTelemetry: true
  └── Layer 3 (App spans)  ← automatically active when Layer 1 is enabled
```

**Key relationships**:

1. **Layer 1 is the prerequisite** — It creates the TracerProvider, registers the global context manager, and wires the OTLP exporters. Without it, Layers 2 and 3 have no tracers and no export path.

2. **Layer 3 is automatically active with Layer 1** — Setting `OTEL_EXPORTER_OTLP_ENDPOINT` enables both Layer 1 and Layer 3. Layer 3's `withRunSpan()` checks `Observability.enabled` (which is `!!OTEL_EXPORTER_OTLP_ENDPOINT`) and degrades gracefully to a no-op when Layer 1 is off.

3. **Layer 2 is an additional opt-in** — It requires both `OTEL_EXPORTER_OTLP_ENDPOINT` (Layer 1 infrastructure) AND `experimental.openTelemetry: true` (config flag). The OpenCode team placed this under `experimental` because it depends on the Vercel AI SDK's `experimental_telemetry` API, which was not yet stable at the time of implementation. Setting `experimental.openTelemetry: true` without `OTEL_EXPORTER_OTLP_ENDPOINT` has no effect — there is no TracerProvider to create tracers from.

4. **Layers 2 and 3 produce a unified trace tree** — Because Layer 1 registers `AsyncLocalStorageContextManager` as the global context manager, spans from Layer 3 (e.g., `RunInteractive.turn`) correctly become parents of Layer 2 spans (e.g., AI SDK's `chat ..streamText`). The resulting trace in Jaeger/Tempo appears as:

```
RunInteractive.session                    ← Layer 3 (app-level)
  └─ RunInteractive.turn                  ← Layer 3 (app-level)
       └─ chat ..streamText               ← Layer 2 (LLM-level, AI SDK)
            ├─ gen_ai.request.model       ← GenAI semantic convention attribute
            ├─ gen_ai.usage.input_tokens  ← GenAI semantic convention attribute
            └─ gen_ai.usage.output_tokens ← GenAI semantic convention attribute
```

5. **The layers are complementary, not redundant** — Layer 1 produces no business spans (infrastructure only). Layer 3 provides coarse-grained application flow (session → turn → lifecycle). Layer 2 provides fine-grained LLM call detail (model, tokens, latency). Together they give full-stack observability; individually each gives only a partial view.

**Configuration Summary**:

| Mechanism | Activation | What It Does | Layer Dependency |
|-----------|-----------|--------------|------------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` env var | Set to collector URL | Activates Layer 1 (TracerProvider + Exporter + ContextManager) and Layer 3 (app-level spans) | Standalone — this is the foundation |
| `OTEL_EXPORTER_OTLP_HEADERS` env var | Optional headers | Auth/API keys for the collector | Requires `OTEL_EXPORTER_OTLP_ENDPOINT` |
| `OTEL_RESOURCE_ATTRIBUTES` env var | Optional attributes | Custom resource attributes on all spans | Requires `OTEL_EXPORTER_OTLP_ENDPOINT` |
| `experimental.openTelemetry: true` in config | In `opencode.json` | Activates Layer 2 (AI SDK LLM call spans with GenAI semantic conventions) | Requires `OTEL_EXPORTER_OTLP_ENDPOINT` (Layer 1) — without a TracerProvider, this flag has no effect |
| `opencode-plugin-otel` | npm plugin | Community plugin for OTLP/gRPC export | Independent alternative |

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
3. No documentation for recommended Collector configuration examples

## Key Insight: OpenCode Already Does the Heavy Lifting

The most important finding from this investigation is that **OpenCode has built-in, production-grade OTel support** that requires zero code changes. It is activated through a two-tier configuration model that reflects the layer dependency chain:

| Configuration | Activates | Depends On | What You Get |
|---------------|-----------|------------|--------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` env var | Layer 1 (infrastructure) + Layer 3 (app-level spans) | Standalone | Session/turn/lifecycle spans, log export. No LLM detail. |
| `experimental.openTelemetry: true` (in `opencode.json`) | Layer 2 (LLM call spans) | **Requires** `OTEL_EXPORTER_OTLP_ENDPOINT` | GenAI semantic convention spans: model, tokens, latency per LLM call. Adds detail **on top of** Layer 3's turn-level structure. |

**The two configurations are not interchangeable alternatives** — they are a layered activation where Layer 2 is additive to Layer 1+3. Setting `experimental.openTelemetry: true` without `OTEL_EXPORTER_OTLP_ENDPOINT` has no effect because there is no TracerProvider for the AI SDK to create tracers from.

This means **Phase 1 is primarily a KubeOpenCode-side change** — inject the `OTEL_EXPORTER_OTLP_ENDPOINT` env var into Pod specs (activating Layers 1+3), and conditionally inject `experimental.openTelemetry: true` into the OpenCode config (activating Layer 2). OpenCode does the rest automatically.

The AI SDK's `experimental_telemetry` flag produces spans with **partial** GenAI Semantic Conventions support — both `ai.*` private attributes and a subset of `gen_ai.*` standard attributes are emitted (see Layer 2 description for details). This includes:
- Per-LLM-call spans with model, token counts, and latency
- `session.id` and `userId` metadata (injected by OpenCode's tracer proxy)
- Tool call spans with tool name and arguments

These Layer 2 spans nest within Layer 3's `RunInteractive.turn` span, producing a unified trace tree. This is the exact data needed for cost attribution, performance debugging, and enterprise audit trails.

## Decision

Adopt a **2-phase approach** that leverages OpenCode's built-in OTel capabilities progressively. Phase 1 exposes OpenCode's existing OTel data; Phase 2 adds KubeOpenCode controller-side observability after Phase 1 is validated. KubeOpenCode's responsibility boundary is clear: produce standardized OTLP data and expose an `endpoint` config; Collector deployment, processing, storage, and visualization are the user's responsibility.

### Phase 1: Enable OpenCode Built-in OTel (Zero code changes in OpenCode)

**Goal**: Surface LLM-level traces and logs for every Task with minimal KubeOpenCode changes.

**How**: OpenCode already exports OTLP when `OTEL_EXPORTER_OTLP_ENDPOINT` is set (activating Layers 1+3). KubeOpenCode just needs to inject this env var into Agent Pods, and conditionally inject `experimental.openTelemetry: true` to additionally activate Layer 2 (LLM call detail).

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
         # OTLP/HTTP endpoint for the user's Collector. Required when enabled.
         # This is the user's existing observability infrastructure —
         # KubeOpenCode does NOT deploy or manage Collectors.
         # Examples:
         #   Gateway mode:  http://otel-collector.observability:4318
         #   Sidecar mode:  http://localhost:4318
         #   External SaaS: https://api.honeycomb.io
         endpoint: "http://otel-collector.observability:4318"
         # Optional headers for collector authentication (e.g., SaaS API keys).
         # Values can be inline or resolved from a Secret via valueFrom.secretKeyRef.
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

4. **Update `api/v1alpha1/config_types.go`** — Add `ObservabilitySpec` struct with `OpenTelemetryConfig` nested type.

5. **Update Helm chart RBAC** — Controller needs to read `KubeOpenCodeConfig` for observability config.

**What users get**: Every LLM call in every Task produces OTel spans with GenAI semantic conventions (token usage, model, latency) visible in Jaeger/Tempo/Grafana. Logs flow to the same OTLP endpoint. Zero code changes in OpenCode required.

**Protocol note**: OpenCode uses OTLP/HTTP (port 4318), not gRPC (port 4317). The configured `endpoint` MUST point to an OTLP/HTTP receiver. The controller appends `/v1/traces`, `/v1/logs`, and `/v1/metrics` automatically per OTel spec. If your existing OTel Collector exposes only gRPC, enable the HTTP receiver in its config or front it with the HTTP-to-gRPC bridge.

**Effort**: Low (primarily pod_builder.go env var injection + config API)

### Phase 2: Controller-Side Observability (Tracing + Metrics)

**Prerequisite**: Phase 1 must be validated in a real deployment before starting Phase 2. Specifically:
- OpenCode's OTel output is confirmed complete and correctly attributed in Agent Pods
- Span structure (names, attributes, parent-child linkage) is stable
- Data volume is acceptable and Collector is processing it without issues
- The `endpoint` config works for the user's Collector topology

Starting Phase 2 before Phase 1 is validated risks building parent spans on top of unstable child spans.

**Goal**: Add KubeOpenCode controller-side telemetry to create end-to-end trace correlation and expose controller-level metrics via OTel.

**Changes**:

1. **Add Go OTel SDK** — Import `go.opentelemetry.io/otel` and related packages:
   ```
   go.opentelemetry.io/otel
   go.opentelemetry.io/otel/trace
   go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
   go.opentelemetry.io/otel/sdk/resource
   go.opentelemetry.io/otel/metric
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

6. **Add OTel Metrics in controller** — Register OTel `Meter` for controller-side operations not covered by span-derived metrics:
   ```go
   meter := otel.GetMeterProvider().Meter("kubeopencode/controller")

   taskReconcileDuration, _ := meter.Float64Histogram(
       "kubeopencode.task.reconcile.duration",
       metric.WithDescription("Time spent in task reconciliation"),
   )
   ```
   Additionally, expose **existing Prometheus metrics via OTel** by using the OTel Prometheus bridge (`go.opentelemetry.io/otel/exporters/prometheus`), making all existing `kubeopencode_*` metrics available in both Prometheus and OTLP formats. This allows gradual migration without breaking existing dashboards.

**What users get**: A single trace view from Task creation → Pod scheduling → Init containers → LLM call chain → Tool execution → Completion. Controller-level metrics available in both Prometheus and OTLP formats.

**Effort**: Medium (Go OTel SDK integration, reconcile instrumentation, trace propagation, metrics registration)

### What We Explicitly Do NOT Build

- **Custom observability UI** — Users already have Grafana, Jaeger, SigNoz, Datadog, or other dashboards. KubeOpenCode produces OTLP data that flows into these tools natively; building our own UI is redundant and would compete with the user's existing investment in their observability platform.
- **Built-in OTel Collector** — The Collector is the user's observability infrastructure. KubeOpenCode only produces data and exposes an `endpoint` config.
- **Mandatory dependency** — OTel is opt-in; KubeOpenCode works without it.
- **Log aggregation** — Out of scope; handled by standard K8s logging stacks.
- **Our own OTel SDK for OpenCode** — OpenCode already has native support.
- **Metrics backend query proxy** — KubeOpenCode does NOT query Prometheus/Mimir on behalf of a UI. Users view metrics directly in their own dashboards.

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

## KubeOpenCode's Observability Responsibility Boundary

A core design principle of this ADR is that **KubeOpenCode produces standardized telemetry; the user's observability infrastructure collects, processes, and stores it.** The OTel Collector belongs to the user's infrastructure, not to KubeOpenCode.

This follows the established pattern in the Kubernetes ecosystem (cert-manager, Argo CD, Flux, etc.): the product exposes an OTLP endpoint configuration, produces well-attributed data, and documents what it emits. The user decides how to deploy, configure, and route their Collectors.

### What KubeOpenCode Does

- **Produces** OTLP-compliant traces, metrics, and logs via standard OTel SDKs
- **Exposes** a single `endpoint` config in `KubeOpenCodeConfig` — the user fills in their Collector address
- **Injects** K8s semantic convention resource attributes (`k8s.pod.name`, `k8s.namespace.name`) so that the user's Collector `k8sattributes` processor can correlate spans with Pods
- **Ships** optional `PodMonitor`/`ServiceMonitor` manifests in the Helm chart (gated by `metrics.serviceMonitor.enabled: true`) for existing Prometheus Operator integrations
- **Documents** what telemetry is produced, which semantic conventions are used, and recommended Collector configuration examples

### What KubeOpenCode Does NOT Do

- **Does NOT deploy or manage Collectors** — No Collector CRs, no Collector containers, no Collector configs in the Helm chart
- **Does NOT prescribe Collector deployment topology** — DaemonSet, Gateway, or Sidecar is the user's architecture decision; KubeOpenCode only needs an `endpoint` URL
- **Does NOT inject sidecar annotations** — If users want OTel Operator sidecar injection, they add `sidecar.opentelemetry.io/inject` annotations themselves (via `podSpec` on Agent, or via namespace-level defaults)
- **Does NOT handle Collector configuration** — Sampling rates, processors, exporters, and backend routing are the user's operational concern
- **Does NOT bundle vendor-specific exporters** — The Collector's exporter config is the user's choice

### Why This Boundary Matters

1. **No conflict with existing infrastructure** — Users likely already have Collectors (DaemonSet, Grafana Alloy, Datadog Agent, cloud-vendor variants). KubeOpenCode must not introduce a competing Collector.
2. **No forced architecture decisions** — Collector topology (sidecar vs. DaemonSet vs. gateway) is a cluster-level decision that depends on scale, isolation needs, and team structure. KubeOpenCode must not prescribe one.
3. **Clear maintenance responsibility** — Collector config (backends, sampling, label enrichment) is environment-specific and evolves independently of KubeOpenCode releases.

### Data Flow

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
              │  User's OTel Collector          │
              │  (DaemonSet / Sidecar / Gateway) │
              │  User's config, user's backends  │
              └─────────────────────────────────┘
```

KubeOpenCode commits to producing **standard, well-attributed OTLP data**. Everything to the right of the `│` boundary is the user's responsibility.

### Protocol Note

OpenCode uses OTLP/HTTP (port 4318), not gRPC (port 4317). The configured `endpoint` MUST point to an OTLP/HTTP receiver. If the user's Collector exposes only gRPC, they can enable the HTTP receiver or front it with the HTTP-to-gRPC bridge — this is a Collector-side configuration, not a KubeOpenCode concern.

### Recommended Collector Configuration (Documentation Scope)

The following are **not ADR decisions** — they are examples that will live in `website/docs/observability.md` to help users integrate KubeOpenCode with their existing Collectors:

- **k8sattributes processor** — Recommended to auto-enrich spans with K8s metadata. KubeOpenCode already sets `k8s.pod.name` and `k8s.namespace.name` resource attributes for correlation.
- **spanmetrics connector** — Recommended to derive `gen_ai.client.operation.duration` and `gen_ai.client.token.usage` metrics from GenAI spans. This is a Collector-side configuration that requires zero KubeOpenCode code changes — once Phase 1 produces GenAI spans, the user's Collector can automatically derive metrics from them. Example:
  ```yaml
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
- **Prometheus receiver** — Can scrape existing `/metrics` endpoints to bridge KubeOpenCode's Prometheus metrics into OTLP.
- **OTel Operator sidecar injection** — Users who want per-Pod Collectors can add `sidecar.opentelemetry.io/inject` annotations via `Agent.spec.podSpec` and set `endpoint: http://localhost:4318`.
- **Service mesh integration** — When running under Istio/Linkerd, KubeOpenCode's OTel spans carry `traceparent` (Phase 2) so mesh traces merge with application traces.

### Backend Compatibility

KubeOpenCode produces OTLP-compliant data. Any OTLP-capable backend can receive it through the user's Collector:

| Backend | Notes |
|---------|-------|
| **Jaeger** | Native OTLP support in Jaeger v1.35+ |
| **Grafana Tempo** | First-class OTLP; recommended for K8s |
| **Prometheus / Mimir / VictoriaMetrics** | Via Collector's `prometheusremotewrite` exporter |
| **SigNoz** | Native OTLP; [tested with OpenCode](https://signoz.io/docs/opencode-observability/) |
| **Datadog, New Relic, Honeycomb, Elastic, Splunk, Dynatrace** | All support OTLP via vendor or standard exporters |

### KubeOpenCodeConfig Schema

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

      # Where to send telemetry. The user's Collector address.
      # KubeOpenCode does NOT deploy or manage Collectors.
      # Examples:
      # - In-cluster: http://otel-collector.observability:4318
      # - Sidecar:    http://localhost:4318
      # - External:   https://api.honeycomb.io
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
```

This schema is intentionally **declarative and backend-agnostic**. KubeOpenCode produces OTLP data and sends it to the user-configured `endpoint`. Everything beyond that — Collector deployment, processing, storage, visualization — is the user's responsibility.

## Consequences

### Positive

- **Phase 1 is extremely low effort** — OpenCode already implements OTel; we just wire it up
- **Zero-dependency for basic use** — OTel is opt-in via `KubeOpenCodeConfig`
- **Leverages existing ecosystem** — Users bring their own Collectors and dashboards; no custom infrastructure required
- **Enterprise value** — Cost visibility, debugging, and audit trails are top enterprise requirements
- **Standards-aligned** — GenAI semantic conventions ensure compatibility with all OTel backends
- **Incremental** — Each phase delivers standalone value; no big-bang rollout
- **No OpenCode fork required** — All integration points are env vars and config injection
- **Backward compatible** — Existing `kubeopencode_*` Prometheus metrics are preserved; OTel metrics are additive. Users can adopt OTel incrementally without breaking existing Prometheus/Grafana dashboards.
- **Clear responsibility boundary** — KubeOpenCode produces data, users' infrastructure collects and visualizes it. No redundant UI or Collector.

### Negative

- **OTel Operator dependency for sidecar injection** — If users want sidecar Collectors, they need the OTel Operator (but this is purely a user-side choice, not a KubeOpenCode requirement)
- **Experimental flag risk** — AI SDK's `experimental_telemetry` is technically experimental, though OTel GenAI semconv are maturing rapidly (GA as of May 2026)
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
| Phase 2: Controller Observability | Medium | Medium | P1 — After Phase 1 validated |

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
