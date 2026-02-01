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

// KubeOpenCodeConfig defines system-level configuration for KubeOpenCode.
// This CRD provides cluster-wide settings for the KubeOpenCode system.
// It is a cluster-scoped resource (not namespaced) and typically a single
// instance named "default" is used for the entire cluster.
type KubeOpenCodeConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the KubeOpenCode configuration
	Spec KubeOpenCodeConfigSpec `json:"spec"`
}

// KubeOpenCodeConfigSpec defines the system-level configuration
type KubeOpenCodeConfigSpec struct {
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
	// Example: "quay.io/kubeopencode/kubeopencode:v0.2.0"
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy specifies the image pull policy for the system image.
	// Defaults to IfNotPresent if not specified.
	// +optional
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubeOpenCodeConfigList contains a list of KubeOpenCodeConfig
type KubeOpenCodeConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeOpenCodeConfig `json:"items"`
}
