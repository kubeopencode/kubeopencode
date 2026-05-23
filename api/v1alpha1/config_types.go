// Copyright Contributors to the KubeOpenCode project

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope="Cluster",shortName=ktc
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="KubeOpenCodeConfig must be named 'cluster'"

// KubeOpenCodeConfig defines system-level configuration for KubeOpenCode.
// This CRD provides cluster-wide settings for the KubeOpenCode system.
// It is a cluster-scoped singleton resource that must be named "cluster".
// Following the OpenShift convention for cluster-wide configuration resources.
type KubeOpenCodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the KubeOpenCode configuration
	Spec KubeOpenCodeConfigSpec `json:"spec"`
}

// KubeOpenCodeConfigSpec defines the system-level configuration
type KubeOpenCodeConfigSpec struct {
	// ClusterDomain specifies the cluster domain name (e.g., "cluster.local").
	// This is used for constructing in-cluster service URLs.
	// If not specified, "cluster.local" is used as the default.
	// +optional
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=253
	ClusterDomain string `json:"clusterDomain,omitempty"`
	// SystemImage configures the KubeOpenCode system image used for internal components
	// such as git-init and context-init containers.
	// If not specified, uses the built-in default image with IfNotPresent policy.
	// +optional
	SystemImage *SystemImageConfig `json:"systemImage,omitempty"`

	// Cleanup configures automatic cleanup of completed Tasks.
	// When configured, completed/failed Tasks are automatically deleted based on
	// TTL (time-to-live) and/or retention count policies.
	// If not specified, Tasks are not automatically deleted (default behavior).
	// +optional
	Cleanup *CleanupConfig `json:"cleanup,omitempty"`

	// Proxy configures cluster-wide HTTP/HTTPS proxy settings for all generated Pods.
	// Agent-level proxy settings take precedence over cluster-level settings.
	// If not specified, no proxy environment variables are injected.
	// +optional
	Proxy *ProxyConfig `json:"proxy,omitempty"`

	// Observability configures OpenTelemetry telemetry for OpenCode agent Pods.
	// When enabled, the controller injects OTLP environment variables into Pod specs
	// so that OpenCode's built-in OTel support is activated automatically.
	// If not specified, no telemetry is produced.
	// +optional
	Observability *ObservabilitySpec `json:"observability,omitempty"`
}

// CleanupConfig defines cleanup policies for completed/failed Tasks.
// Both TTL and retention-based cleanup can be configured independently or combined.
// When both are configured, TTL is checked first, then retention count.
type CleanupConfig struct {
	// TTLSecondsAfterFinished specifies the TTL for cleaning up finished Tasks.
	// If set, completed/failed Tasks will be deleted after this duration from CompletionTime.
	// If unset or nil, TTL-based cleanup is disabled.
	//
	// Example:
	//   ttlSecondsAfterFinished: 3600  # Delete after 1 hour
	// +optional
	// +kubebuilder:validation:Minimum=0
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`

	// MaxRetainedTasks specifies the maximum number of completed/failed Tasks to retain
	// per namespace. When exceeded, the oldest Tasks (by CompletionTime) are deleted first.
	// If unset or nil, retention-based cleanup is disabled.
	//
	// Note: This is a cluster-wide configuration that applies the same limit to each namespace.
	// TTL cleanup takes precedence - Tasks exceeding TTL are deleted regardless of this limit.
	// This count only applies to Tasks that haven't exceeded TTL yet.
	//
	// Example:
	//   maxRetainedTasks: 100  # Keep at most 100 completed Tasks per namespace
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxRetainedTasks *int32 `json:"maxRetainedTasks,omitempty"`
}

// SystemImageConfig configures the KubeOpenCode system image used for internal components
// such as git-init and context-init containers.
type SystemImageConfig struct {
	// Image specifies the system image to use for internal KubeOpenCode components.
	// If not specified, defaults to the built-in DefaultKubeOpenCodeImage.
	// Example: "ghcr.io/kubeopencode/kubeopencode:v0.2.0"
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy specifies the image pull policy for the system image.
	// Defaults to IfNotPresent if not specified.
	// +optional
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

// ObservabilitySpec configures OpenTelemetry telemetry for KubeOpenCode.
// KubeOpenCode produces OTLP data and sends it to the user-configured endpoint.
// Collector deployment, processing, storage, and visualization are the user's responsibility.
type ObservabilitySpec struct {
	// OpenTelemetry configures OpenTelemetry telemetry integration.
	// When enabled, the controller injects OTLP environment variables into agent Pods
	// so that OpenCode's built-in OTel support is activated automatically.
	// +optional
	OpenTelemetry *OpenTelemetryConfig `json:"openTelemetry,omitempty"`
}

// OpenTelemetryConfig configures OpenTelemetry telemetry for OpenCode agent Pods.
// OpenCode has built-in OTel support activated by setting OTEL_EXPORTER_OTLP_ENDPOINT.
// KubeOpenCode injects this env var and related configuration into Pod specs.
//
// OpenCode's OTel support has three complementary layers:
// - Layer 1 (Infrastructure): TracerProvider + ContextManager + Exporter (activated by endpoint)
// - Layer 2 (LLM traces): AI SDK experimental_telemetry with GenAI semantic conventions (activated by enableLLMTraces)
// - Layer 3 (App spans): Session/turn/lifecycle spans (automatically active with Layer 1)
// +kubebuilder:validation:XValidation:rule="!self.enabled || size(self.endpoint) > 0",message="endpoint is required when enabled is true"
type OpenTelemetryConfig struct {
	// Enabled determines whether OpenTelemetry telemetry is produced.
	// When true, the controller injects OTEL_EXPORTER_OTLP_ENDPOINT and related
	// environment variables into agent Pod specs.
	// +required
	Enabled bool `json:"enabled"`

	// Endpoint is the OTLP/HTTP endpoint for the user's OpenTelemetry Collector.
	// This is the user's existing observability infrastructure —
	// KubeOpenCode does NOT deploy or manage Collectors.
	// Required when enabled is true.
	// Examples:
	//   - Gateway mode:  http://otel-collector.observability:4318
	//   - Sidecar mode:  http://localhost:4318
	//   - External SaaS: https://api.honeycomb.io
	//
	// Note: OpenCode uses OTLP/HTTP (port 4318), not gRPC (port 4317).
	// The configured endpoint MUST point to an OTLP/HTTP receiver.
	// +optional
	// +kubebuilder:validation:Pattern=`^https?://.+$`
	Endpoint string `json:"endpoint,omitempty"`

	// Headers specifies optional headers for collector authentication (e.g., SaaS API keys).
	// Values can be inline or resolved from a Secret via valueFrom.secretKeyRef
	// to avoid leaking API keys in the KubeOpenCodeConfig.
	// +optional
	Headers map[string]OTelHeaderValueSource `json:"headers,omitempty"`

	// EnableLLMTraces injects experimental.openTelemetry into OpenCode config
	// to enable LLM call traces with GenAI semantic conventions.
	// When true, every LLM call produces spans with model, token counts, and latency
	// that nest within the application-level turn spans (Layer 3).
	// Requires endpoint to be set (Layer 1 must be active for Layer 2 to work).
	// +optional
	EnableLLMTraces bool `json:"enableLLMTraces,omitempty"`

	// RecordContent determines whether to record full prompt/response content on LLM spans.
	// Default false; set true only in trusted environments.
	// When true, the controller injects OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=true
	// per OTel GenAI spec. This may expose sensitive data (API keys, PII, proprietary code).
	// +optional
	RecordContent bool `json:"recordContent,omitempty"`

	// ResourceAttributes specifies additional resource attributes to add to all spans.
	// The controller also automatically injects standard attributes:
	// kubeopencode.task.name, kubeopencode.task.namespace, kubeopencode.agent.name,
	// k8s.namespace.name, k8s.pod.name.
	// +optional
	ResourceAttributes map[string]string `json:"resourceAttributes,omitempty"`
}

// OTelHeaderValueSource represents a header value that can be specified inline
// or resolved from a Kubernetes Secret.
// +kubebuilder:validation:XValidation:rule="(has(self.value) && self.value != '') || (has(self.valueFrom) && has(self.valueFrom.secretKeyRef))",message="either a non-empty value or valueFrom.secretKeyRef must be specified"
type OTelHeaderValueSource struct {
	// Value specifies the header value inline.
	// Use for non-sensitive values. For sensitive values (API keys), use valueFrom.secretKeyRef.
	// +optional
	Value string `json:"value,omitempty"`

	// ValueFrom specifies the source for the header value.
	// Use this for sensitive values like SaaS API keys to avoid leaking them in the KubeOpenCodeConfig.
	// +optional
	ValueFrom *OTelHeaderValueSourceFrom `json:"valueFrom,omitempty"`
}

// OTelHeaderValueSourceFrom represents the source for a header value.
type OTelHeaderValueSourceFrom struct {
	// SecretKeyRef references a key in a Secret containing the header value.
	// The Secret must be in the same namespace as the KubeOpenCodeConfig controller
	// (kubeopencode-system by default).
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubeOpenCodeConfigList contains a list of KubeOpenCodeConfig
type KubeOpenCodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeOpenCodeConfig `json:"items"`
}
