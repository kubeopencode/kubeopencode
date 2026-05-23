# OpenTelemetry Observability

KubeOpenCode integrates with OpenTelemetry to provide observability for AI agent workloads — LLM call traces, token usage, latency, and application-level spans flow automatically to your existing observability infrastructure.

OpenCode has **built-in OTel support** that is activated by injecting standard OTLP environment variables into agent Pods. KubeOpenCode handles the injection; OpenCode handles the telemetry production. No code changes or custom instrumentation required.

## How It Works

OpenCode's OTel support has three complementary layers:

| Layer | What It Does | Activation |
|-------|-------------|------------|
| **Layer 1: Infrastructure** | TracerProvider + ContextManager + OTLP Exporter | `OTEL_EXPORTER_OTLP_ENDPOINT` env var |
| **Layer 2: LLM Traces** | Per-LLM-call spans with model, tokens, latency | `experimental.openTelemetry: true` in OpenCode config + Layer 1 |
| **Layer 3: App Spans** | Session/turn/lifecycle spans | Automatically active with Layer 1 |

Layer 1 is the prerequisite — it creates the TracerProvider and wires exporters. Layer 3 is automatically active with Layer 1. Layer 2 is an additional opt-in that adds fine-grained LLM call detail.

When you enable observability in KubeOpenCodeConfig, the controller:

1. Injects `OTEL_EXPORTER_OTLP_ENDPOINT` into agent Pod specs (activates Layers 1 + 3)
2. Optionally injects `experimental.openTelemetry: true` into the OpenCode config (activates Layer 2)
3. Injects Kubernetes resource attributes for trace correlation
4. Handles header authentication via inline values or Secret references

## Configuration

Observability is configured in the `KubeOpenCodeConfig` cluster-scoped singleton (must be named `cluster`):

```yaml
apiVersion: kubeopencode.io/v1alpha1
kind: KubeOpenCodeConfig
metadata:
  name: cluster
spec:
  observability:
    openTelemetry:
      enabled: true
      # OTLP/HTTP endpoint for your Collector. Required when enabled.
      # KubeOpenCode does NOT deploy or manage Collectors.
      endpoint: "http://otel-collector.observability:4318"
      # Optional headers for collector authentication
      headers:
        x-honeycomb-team:
          valueFrom:
            secretKeyRef:
              name: honeycomb-credentials
              key: api-key
      # Enable LLM call traces (model, tokens, latency per call)
      enableLLMTraces: true
      # Record full prompt/response content on LLM spans
      recordContent: false
      # Additional resource attributes on all spans
      resourceAttributes:
        kubeopencode.cluster.name: "production"
```

When using Helm, configure it in `values.yaml`:

```yaml
kubeopencodeConfig:
  observability:
    openTelemetry:
      enabled: true
      endpoint: "http://otel-collector.observability:4318"
      enableLLMTraces: true
```

### Field Reference

| Field | Type | Description |
|-------|------|-------------|
| `openTelemetry.enabled` | bool | Enable OTel telemetry injection. Required. |
| `openTelemetry.endpoint` | string | OTLP/HTTP endpoint for your Collector. Required when `enabled: true`. Must be an HTTP or HTTPS URL. |
| `openTelemetry.headers` | map | Optional headers for collector authentication. Values can be inline or from Secrets. |
| `openTelemetry.enableLLMTraces` | bool | Inject `experimental.openTelemetry` into OpenCode config to enable per-LLM-call spans (Layer 2). Default: `false`. |
| `openTelemetry.recordContent` | bool | Record full prompt/response content on LLM spans. Default: `false`. Only enable in trusted environments — may expose sensitive data. |
| `openTelemetry.resourceAttributes` | map | Additional resource attributes for all spans. The controller also injects standard attributes automatically. |

### Protocol Note

OpenCode uses **OTLP/HTTP** (port 4318), not gRPC (port 4317). The configured `endpoint` must point to an OTLP/HTTP receiver. If your Collector exposes only gRPC, enable the HTTP receiver in its configuration or front it with an HTTP-to-gRPC bridge.

## What You Get

### Layer 1 + 3 (Basic: `enabled: true`)

Setting `enabled: true` with an `endpoint` activates the OTel infrastructure and application-level spans:

- **Session/turn/lifecycle spans** — `RunInteractive.session`, `RunInteractive.turn`, `RunLifecycle.boot/close`, etc.
- **Log export** — OpenCode logs flow to the same OTLP endpoint
- **Trace context** — AsyncLocalStorageContextManager ensures correct parent-child span linkage

This gives you application-level visibility: when sessions start and end, how many turns occur, and runtime lifecycle events.

### Layer 2 (LLM Detail: `enableLLMTraces: true`)

Setting `enableLLMTraces: true` additionally activates AI SDK telemetry for every LLM call. Observed span attributes:

| Span | Key Attributes |
|------|---------------|
| `ai.streamText` | `ai.model.id`, `ai.model.provider`, `ai.usage.inputTokens/outputTokens/totalTokens`, `ai.usage.cachedInputTokens`, `ai.usage.reasoningTokens`, `ai.response.finishReason` |
| `ai.streamText.doStream` | Sub-span for the actual HTTP streaming execution |
| `ai.toolCall` | `ai.toolCall.name`, `ai.toolCall.args`, `ai.toolCall.result` |

These spans nest within Layer 3's `RunInteractive.turn` span, producing a unified trace tree:

```
Config.get / Agent.state / ...           ← Layer 3 (app-level spans)
  └─ LLM.run                             ← Layer 3 (OpenCode LLM orchestration)
       └─ ai.streamText                  ← Layer 2 (AI SDK streaming span)
            ├─ ai.model.id=big-pickle
            ├─ ai.usage.inputTokens=8247
            ├─ ai.usage.outputTokens=117
            └─ ai.toolCall               ← Layer 2 (AI SDK tool call)
                 ├─ ai.toolCall.name=read
                 └─ ai.toolCall.args={"filePath":"/workspace/task.md"}
```

This gives you fine-grained LLM observability: which model was called, how many tokens were consumed, how long each call took, and what tools were invoked.

> **Note on attribute namespace**: The Vercel AI SDK currently emits `ai.*` attributes rather than the OpenTelemetry `gen_ai.*` semantic conventions. The data is functionally equivalent for cost attribution, model tracking, and behavior analysis. When the AI SDK adds `gen_ai.*` support in a future version, those attributes will automatically appear without any KubeOpenCode changes.

## Headers & Authentication

For Collectors that require authentication (SaaS backends like Honeycomb, Datadog, New Relic), configure headers with either inline values or Secret references:

### Inline Value (Non-Sensitive)

```yaml
headers:
  x-custom-header:
    value: "my-static-value"
```

### Secret Reference (Sensitive — Recommended for API Keys)

```yaml
headers:
  x-honeycomb-team:
    valueFrom:
      secretKeyRef:
        name: honeycomb-credentials
        key: api-key
```

Create the Secret in the controller namespace:

```bash
kubectl create secret generic honeycomb-credentials \
  --from-literal=api-key=your-api-key \
  -n kubeopencode-system
```

When using `secretKeyRef`, the controller injects a helper environment variable that kubelet resolves from the Secret at container startup. The header value is never stored in the KubeOpenCodeConfig resource.

## Resource Attributes

The controller automatically injects these resource attributes on all spans:

| Attribute | Source |
|-----------|--------|
| `kubeopencode.task.name` | Task resource name (Task Pods only) |
| `kubeopencode.task.namespace` | Task/Agent namespace |
| `kubeopencode.agent.name` | Agent or AgentTemplate name |
| `k8s.namespace.name` | Namespace |
| `k8s.pod.name` | Pod name (Downward API for Deployments) |

You can add custom attributes via `resourceAttributes`:

```yaml
resourceAttributes:
  kubeopencode.cluster.name: "production"
  environment: "staging"
```

## Record Content

By default, LLM spans do not include full prompt and response content. To enable content recording:

```yaml
openTelemetry:
  recordContent: true
```

This injects `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=true` per the OTel GenAI specification. **Use with caution** — recorded content may contain API keys, PII, proprietary code, or other sensitive data. Only enable in trusted environments with appropriate data handling policies.

## Responsibility Boundary

KubeOpenCode produces standardized OTLP data and sends it to the user-configured `endpoint`. Everything beyond that is the user's responsibility:

| What KubeOpenCode Does | What the User Does |
|------------------------|-------------------|
| Produces OTLP-compliant traces and logs | Deploys and manages OTel Collectors |
| Injects env vars and config into agent Pods | Configures Collector processors, exporters, and backends |
| Sets K8s semantic convention resource attributes | Chooses backend (Jaeger, Tempo, Grafana, Datadog, etc.) |
| Provides the `endpoint` configuration field | Decides Collector topology (DaemonSet, Gateway, Sidecar) |

KubeOpenCode does **not** deploy Collectors, prescribe topology, or bundle vendor-specific exporters.

## Recommended Collector Configuration

The following examples help integrate KubeOpenCode with your existing OTel Collector.

### Basic Gateway Collector

```yaml
receivers:
  otlp:
    protocols:
      http:
        endpoint: 0.0.0.0:4318

exporters:
  otlphttp:
    endpoint: "http://jaeger-collector.observability:4318"

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlphttp]
```

### k8sattributes Processor

Recommended for auto-enriching spans with Kubernetes metadata:

```yaml
processors:
  k8sattributes:
    auth_type: "serviceAccount"
    passthrough: false
    filter:
      node_from_env_var: KUBE_NODE_NAME
    extract:
      metadata:
        - k8s.pod.name
        - k8s.namespace.name
        - k8s.node.name
    pod_association:
      - sources:
          - from: resource_attribute
            name: k8s.pod.name

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [k8sattributes]
      exporters: [otlphttp]
```

### spanmetrics Connector (with ai.* to gen_ai.* Mapping)

The OTel Collector's `spanmetrics` connector derives metrics from spans, but it looks for `gen_ai.*` attributes. Since the AI SDK emits `ai.*` attributes, use the `transform` processor to rename them:

```yaml
processors:
  transform/ai-to-genai:
    error_mode: ignore
    trace_statements:
      - context: span
        statements:
          - set(attributes["gen_ai.system"], attributes["ai.model.provider"]) where attributes["ai.model.provider"] != nil
          - set(attributes["gen_ai.request.model"], attributes["ai.model.id"]) where attributes["ai.model.id"] != nil
          - set(attributes["gen_ai.usage.input_tokens"], attributes["ai.usage.inputTokens"]) where attributes["ai.usage.inputTokens"] != nil
          - set(attributes["gen_ai.usage.output_tokens"], attributes["ai.usage.outputTokens"]) where attributes["ai.usage.outputTokens"] != nil

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

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [transform/ai-to-genai]
      exporters: [spanmetrics, otlphttp]
    metrics:
      receivers: [spanmetrics]
      exporters: [prometheus]
```

## Local Development with Jaeger

For local development and testing, deploy an OTel Collector + Jaeger to your Kind cluster:

```bash
# Load images into Kind
make load-otel-images

# Deploy OTel Collector + Jaeger
kubectl apply -f deploy/local-dev/otel-observability.yaml

# Enable observability in KubeOpenCodeConfig
kubectl patch kubeopencodeconfig cluster --type=merge -p \
  '{"spec":{"observability":{"openTelemetry":{"enabled":true,"endpoint":"http://otel-collector.observability:4318","enableLLMTraces":true}}}}'

# Port-forward Jaeger UI
kubectl port-forward -n observability svc/jaeger-query 16686:16686
```

Open http://localhost:16686 to query traces. Select the `opencode` service to see LLM call spans.

## Backend Compatibility

KubeOpenCode produces OTLP-compliant data. Any OTLP-capable backend can receive it through your Collector:

| Backend | Notes |
|---------|-------|
| **Jaeger** | Native OTLP support in v1.35+ |
| **Grafana Tempo** | First-class OTLP; recommended for Kubernetes |
| **SigNoz** | Native OTLP support |
| **Honeycomb** | OTLP via `x-honeycomb-team` header |
| **Datadog** | OTLP ingest endpoint available |
| **Prometheus / Mimir** | Via Collector's `prometheusremotewrite` exporter |
| **Elastic, Splunk, New Relic, Dynatrace** | All support OTLP via vendor or standard exporters |
