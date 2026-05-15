// Copyright Contributors to the KubeOpenCode project

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=reg
// +kubebuilder:printcolumn:JSONPath=`.status.summary.images`,name="Images",type=integer
// +kubebuilder:printcolumn:JSONPath=`.status.summary.skills`,name="Skills",type=integer
// +kubebuilder:printcolumn:JSONPath=`.status.summary.plugins`,name="Plugins",type=integer
// +kubebuilder:printcolumn:JSONPath=`.status.summary.readyCount`,name="Ready",type=integer
// +kubebuilder:printcolumn:JSONPath=`.status.summary.totalCount`,name="Total",type=integer
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

// Registry is a namespace-scoped catalog of agent assets.
// It indexes executor images, skills, and plugins — providing a central discovery
// mechanism for agent assembly. The Registry does not store or build assets;
// it only references and validates them.
type Registry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired asset catalog
	Spec RegistrySpec `json:"spec"`

	// Status represents the observed state of the asset catalog
	// +optional
	Status RegistryStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RegistryList contains a list of Registry
type RegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Registry `json:"items"`
}

// RegistrySpec defines the desired state of a Registry.
type RegistrySpec struct {
	// CheckInterval defines how often the controller re-validates
	// asset availability (images, skills, plugins).
	// If not specified, the controller defaults to 10 minutes.
	// +optional
	CheckInterval *metav1.Duration `json:"checkInterval,omitempty"`

	// Images defines executor image references available for agent assembly.
	// Each entry references a pre-built container image in an external registry.
	// The Registry does not build images — it only indexes and validates them.
	// +optional
	// +listType=map
	// +listMapKey=name
	Images []RegistryImage `json:"images,omitempty"`

	// Skills defines skill references (reuses existing SkillSource types).
	// +optional
	// +listType=map
	// +listMapKey=name
	Skills []RegistrySkill `json:"skills,omitempty"`

	// Plugins defines plugin references (reuses existing PluginSpec type).
	// +optional
	// +listType=map
	// +listMapKey=name
	Plugins []RegistryPlugin `json:"plugins,omitempty"`

	// NpmRegistryURL overrides the default npm registry URL (https://registry.npmjs.org/)
	// for plugin availability checks. Use this when plugins are hosted on a private
	// npm registry (e.g., Artifactory, Verdaccio, GitHub Packages).
	// Example: "https://npm.company.com/"
	// +optional
	NpmRegistryURL string `json:"npmRegistryURL,omitempty"`
}

// RegistryImage defines a reference to a pre-built container image.
// The image must already exist in an external container registry.
// The Registry controller validates that the image is reachable and records its digest.
type RegistryImage struct {
	// Name is a unique identifier for this image within the Registry.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Image is the full container image reference (e.g., "harbor.company.com/team/go-dev:1.23").
	// Must include registry, repository, and tag or digest.
	// +required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// SecretRef references a Secret containing registry credentials for private images.
	// The Secret must be of type kubernetes.io/dockerconfigjson.
	// NOTE: Phase 1 does not use this field for validation — image checks are
	// format-only. Future phases will use go-containerregistry with these credentials
	// to perform HEAD manifest checks against private registries.
	// +optional
	SecretRef *RegistrySecretReference `json:"secretRef,omitempty"`

	// Metadata provides human-readable information for the UI,
	// helping users choose the right executor image for their use case.
	// +optional
	Metadata ImageMetadata `json:"metadata,omitempty"`
}

// ImageMetadata provides rich descriptive information for executor images.
// This metadata helps users browse and select the right image in the Visual Assembler UI.
type ImageMetadata struct {
	// Description is a human-readable summary of what this image provides.
	// Example: "Go 1.23 development environment with LSP support and debugging tools"
	// +optional
	Description string `json:"description,omitempty"`

	// Category classifies the image for filtering in the UI.
	// Examples: "backend", "frontend", "data-science", "devops", "general"
	// +optional
	Category string `json:"category,omitempty"`

	// Tags are searchable labels for discovery.
	// Examples: ["golang", "backend", "grpc"]
	// +optional
	Tags []string `json:"tags,omitempty"`

	// Tools lists the key tools/binaries available in the image.
	// Displayed in the UI to help users understand what the image provides.
	// Examples: ["go", "gopls", "delve", "golangci-lint"]
	// +optional
	Tools []string `json:"tools,omitempty"`

	// BaseImage indicates the base OS/runtime image (informational).
	// Example: "ubuntu:24.04", "nvidia/cuda:12.4-runtime-ubuntu24.04"
	// +optional
	BaseImage string `json:"baseImage,omitempty"`

	// Maintainer is the contact for this image (informational).
	// Example: "platform-team@company.com"
	// +optional
	Maintainer string `json:"maintainer,omitempty"`
}

// RegistrySecretReference references a Secret for container registry authentication.
// This is a separate type from GitSecretReference intentionally:
//   - GitSecretReference expects username/password or ssh-privatekey (Git credentials)
//   - RegistrySecretReference expects kubernetes.io/dockerconfigjson (Docker credentials)
//
// Keeping them separate allows independent evolution (e.g., adding namespace or
// credential rotation fields) without affecting Git credential semantics.
type RegistrySecretReference struct {
	// Name of the Secret containing registry credentials.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

// RegistrySkill wraps SkillSource with catalog metadata.
// The Git field reuses the existing GitSkillSource type from SkillSource.
type RegistrySkill struct {
	// Name is a unique identifier for this skill within the Registry.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Git specifies the skill source (reuses existing GitSkillSource).
	// +optional
	Git *GitSkillSource `json:"git,omitempty"`

	// Metadata provides human-readable information for the UI.
	// +optional
	Metadata AssetMetadata `json:"metadata,omitempty"`
}

// RegistryPlugin wraps PluginSpec with catalog metadata.
// Note: PluginSpec includes an Options field (runtime configuration).
// In the Registry context, Options is ignored — it exists only because
// we reuse the PluginSpec type as-is. Plugin options are runtime concerns
// and belong in Agent/AgentTemplate, not in the catalog.
type RegistryPlugin struct {
	// Name is a unique identifier for this plugin within the Registry.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	Name string `json:"name"`

	// Plugin specifies the plugin package (reuses existing PluginSpec).
	// Only name and target are meaningful in the Registry; options is ignored.
	Plugin PluginSpec `json:"plugin"`

	// Metadata provides human-readable information for the UI.
	// +optional
	Metadata AssetMetadata `json:"metadata,omitempty"`
}

// AssetMetadata provides human-readable information for UI display (skills and plugins).
type AssetMetadata struct {
	// Description is a human-readable summary.
	// +optional
	Description string `json:"description,omitempty"`

	// Tags are searchable labels for discovery.
	// +optional
	Tags []string `json:"tags,omitempty"`

	// RequiredCredentials lists env vars that must be set (only meaningful for plugins).
	// +optional
	RequiredCredentials []CredentialRequirement `json:"requiredCredentials,omitempty"`
}

// CredentialRequirement describes a credential that must be configured for a plugin.
type CredentialRequirement struct {
	// Env is the environment variable name.
	// +required
	Env string `json:"env"`

	// Description explains what this credential is for.
	// +optional
	Description string `json:"description,omitempty"`
}

// AssetPhase represents the availability status of a registry asset.
// +kubebuilder:validation:Enum=Ready;Unavailable
type AssetPhase string

const (
	// AssetPhaseReady indicates the asset is reachable and available for use.
	AssetPhaseReady AssetPhase = "Ready"
	// AssetPhaseUnavailable indicates the asset could not be validated.
	AssetPhaseUnavailable AssetPhase = "Unavailable"
)

// RegistryStatus tracks the observed state of all assets.
type RegistryStatus struct {
	// ObservedGeneration reflects the generation of the spec that was last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the Registry's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Images tracks the status of each registered image.
	// +optional
	Images []ImageStatus `json:"images,omitempty"`

	// Skills tracks the status of each registered skill.
	// +optional
	Skills []SkillStatus `json:"skills,omitempty"`

	// Plugins tracks the status of each registered plugin.
	// +optional
	Plugins []PluginStatus `json:"plugins,omitempty"`

	// Summary provides aggregate counts for kubectl printcolumn display.
	// +optional
	Summary StatusSummary `json:"summary,omitempty"`
}

// ImageStatus tracks the availability of a registered image.
type ImageStatus struct {
	// Name matches the spec image name.
	Name string `json:"name"`
	// Phase indicates whether the image is reachable.
	Phase AssetPhase `json:"phase"`
	// Image is the full image reference from spec.
	// +optional
	Image string `json:"image,omitempty"`
	// Digest is the resolved image digest from the registry.
	// +optional
	Digest string `json:"digest,omitempty"`
	// LastChecked is the timestamp of the last status check.
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`
	// Message contains error details when Phase is Unavailable.
	// +optional
	Message string `json:"message,omitempty"`
}

// SkillStatus tracks the availability of a registered skill.
type SkillStatus struct {
	// Name matches the spec skill name.
	Name string `json:"name"`
	// Phase indicates whether the Git repository is reachable.
	Phase AssetPhase `json:"phase"`
	// LatestCommit is the HEAD commit SHA from the remote.
	// +optional
	LatestCommit string `json:"latestCommit,omitempty"`
	// LastChecked is the timestamp of the last status check.
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`
	// Message contains error details when Phase is Unavailable.
	// +optional
	Message string `json:"message,omitempty"`
}

// PluginStatus tracks the availability of a registered plugin.
type PluginStatus struct {
	// Name matches the spec plugin name.
	Name string `json:"name"`
	// Phase indicates whether the npm package is reachable.
	Phase AssetPhase `json:"phase"`
	// ResolvedVersion is the resolved semver version from the npm registry.
	// +optional
	ResolvedVersion string `json:"resolvedVersion,omitempty"`
	// LastChecked is the timestamp of the last status check.
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`
	// Message contains error details when Phase is Unavailable.
	// +optional
	Message string `json:"message,omitempty"`
}

// StatusSummary provides aggregate counts for kubectl display.
type StatusSummary struct {
	// Images is the total number of registered images.
	Images int `json:"images"`
	// Skills is the total number of registered skills.
	Skills int `json:"skills"`
	// Plugins is the total number of registered plugins.
	Plugins int `json:"plugins"`
	// ReadyCount is the total number of assets with Ready phase.
	ReadyCount int `json:"readyCount"`
	// TotalCount is the total number of registered assets.
	TotalCount int `json:"totalCount"`
}
