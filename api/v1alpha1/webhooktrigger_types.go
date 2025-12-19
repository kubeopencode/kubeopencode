// Copyright Contributors to the KubeTask project

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConcurrencyPolicy describes how the webhook trigger will handle concurrent tasks.
// +kubebuilder:validation:Enum=Allow;Forbid;Replace
type ConcurrencyPolicy string

const (
	// ConcurrencyPolicyAllow allows multiple tasks to run concurrently.
	ConcurrencyPolicyAllow ConcurrencyPolicy = "Allow"

	// ConcurrencyPolicyForbid ignores new webhooks if there's already a running task.
	ConcurrencyPolicyForbid ConcurrencyPolicy = "Forbid"

	// ConcurrencyPolicyReplace stops the running task and creates a new one.
	ConcurrencyPolicyReplace ConcurrencyPolicy = "Replace"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=wht
// +kubebuilder:printcolumn:JSONPath=`.status.totalTriggered`,name="Triggered",type=integer
// +kubebuilder:printcolumn:JSONPath=`.status.lastTriggeredTime`,name="Last Trigger",type=date
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

// WebhookTrigger represents a webhook-to-Task mapping rule.
// When the webhook endpoint receives a request that matches the configured filters,
// it creates a Task based on the taskTemplate.
//
// Each WebhookTrigger has a unique endpoint at:
//
//	/webhooks/<namespace>/<trigger-name>
//
// WebhookTrigger is platform-agnostic and supports any webhook source
// (GitHub, GitLab, custom systems, etc.) through configurable authentication
// and filtering.
type WebhookTrigger struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of WebhookTrigger
	Spec WebhookTriggerSpec `json:"spec"`

	// Status represents the current status of the WebhookTrigger
	// +optional
	Status WebhookTriggerStatus `json:"status,omitempty"`
}

// WebhookTriggerSpec defines the WebhookTrigger configuration
type WebhookTriggerSpec struct {
	// Auth defines authentication for webhook validation.
	// If not specified, webhooks are accepted without authentication.
	// +optional
	Auth *WebhookAuth `json:"auth,omitempty"`

	// Filter is a CEL expression that must evaluate to true for the webhook to trigger.
	// If not specified, all webhooks are accepted.
	//
	// The webhook payload is available as the variable "body".
	// HTTP headers are available as the variable "headers" (map[string]string, lowercase keys).
	//
	// CEL provides powerful filtering capabilities:
	//   - Field access: body.action, body.repository.full_name
	//   - Comparisons: body.action == "opened", body.pull_request.additions < 500
	//   - List operations: body.action in ["opened", "synchronize"]
	//   - String functions: body.title.startsWith("[WIP]"), body.branch.matches("^feature/.*")
	//   - Existence checks: has(body.pull_request) && body.pull_request.draft == false
	//   - List predicates: body.labels.exists(l, l.name == "needs-review")
	//
	// Examples:
	//   # Simple equality
	//   filter: 'body.action == "opened"'
	//
	//   # Multiple conditions
	//   filter: 'body.action in ["opened", "synchronize"] && body.repository.full_name == "myorg/myrepo"'
	//
	//   # Complex logic
	//   filter: |
	//     !body.pull_request.title.startsWith("[WIP]") &&
	//     body.pull_request.additions + body.pull_request.deletions < 500 &&
	//     body.pull_request.labels.exists(l, l.name == "needs-review")
	//
	// +optional
	Filter string `json:"filter,omitempty"`

	// ConcurrencyPolicy specifies how to treat concurrent tasks triggered by this webhook.
	// - Allow: Create a new task regardless of existing running tasks (default)
	// - Forbid: Skip this webhook if there's already a running task
	// - Replace: Stop the running task and create a new one
	// +kubebuilder:validation:Enum=Allow;Forbid;Replace
	// +kubebuilder:default=Allow
	// +optional
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrencyPolicy,omitempty"`

	// TaskTemplate defines the Task to create when a webhook matches.
	// The template supports Go template syntax with webhook payload data.
	// +required
	TaskTemplate WebhookTaskTemplate `json:"taskTemplate"`
}

// WebhookAuth defines authentication configuration for webhook validation.
// Exactly one authentication method should be specified.
type WebhookAuth struct {
	// HMAC configures HMAC signature validation.
	// Common for GitHub (X-Hub-Signature-256) and similar platforms.
	// +optional
	HMAC *HMACAuth `json:"hmac,omitempty"`

	// BearerToken configures Bearer token validation from Authorization header.
	// +optional
	BearerToken *BearerTokenAuth `json:"bearerToken,omitempty"`

	// Header configures simple header value matching.
	// Common for GitLab (X-Gitlab-Token) and custom webhooks.
	// +optional
	Header *HeaderAuth `json:"header,omitempty"`
}

// HMACAuth configures HMAC signature validation.
// The signature is computed over the request body and compared
// with the value in the specified header.
type HMACAuth struct {
	// SecretRef references a Secret containing the HMAC secret key.
	// +required
	SecretRef SecretKeyReference `json:"secretRef"`

	// SignatureHeader is the HTTP header name containing the signature.
	// Example: "X-Hub-Signature-256" for GitHub, "X-Signature" for custom systems.
	// +required
	SignatureHeader string `json:"signatureHeader"`

	// Algorithm specifies the HMAC algorithm to use.
	// +kubebuilder:validation:Enum=sha1;sha256;sha512
	// +kubebuilder:default=sha256
	// +optional
	Algorithm string `json:"algorithm,omitempty"`
}

// BearerTokenAuth configures Bearer token validation.
// Expects the Authorization header in format: "Bearer <token>"
type BearerTokenAuth struct {
	// SecretRef references a Secret containing the expected token.
	// +required
	SecretRef SecretKeyReference `json:"secretRef"`
}

// HeaderAuth configures simple header value matching.
// The specified header's value must exactly match the secret value.
type HeaderAuth struct {
	// Name is the HTTP header name to check.
	// Example: "X-Gitlab-Token", "X-Custom-Auth"
	// +required
	Name string `json:"name"`

	// SecretRef references a Secret containing the expected header value.
	// +required
	SecretRef SecretKeyReference `json:"secretRef"`
}

// SecretKeyReference references a specific key within a Secret.
type SecretKeyReference struct {
	// Name of the Secret.
	// +required
	Name string `json:"name"`

	// Key of the Secret to select.
	// +required
	Key string `json:"key"`
}

// WebhookTaskTemplate defines the Task to create when a webhook matches.
// The description field supports Go template syntax with the webhook payload
// available as the template data.
type WebhookTaskTemplate struct {
	// AgentRef references an Agent for task execution.
	// If not specified, uses the "default" Agent in the same namespace.
	// +optional
	AgentRef string `json:"agentRef,omitempty"`

	// Description is the task instruction/prompt.
	// Supports Go template syntax with webhook payload data.
	//
	// Available template data:
	//   - The entire webhook JSON payload is available as the root object
	//   - Example: {{ .pull_request.number }}, {{ .repository.full_name }}
	//
	// Example:
	//   description: |
	//     Review Pull Request #{{ .pull_request.number }}
	//     Repository: {{ .repository.full_name }}
	//     Author: {{ .pull_request.user.login }}
	// +required
	Description string `json:"description"`

	// Contexts provides additional context for the task.
	// Each context can be a reference to a Context CRD (via Ref) or inline definition.
	// +optional
	Contexts []ContextSource `json:"contexts,omitempty"`
}

// WebhookTriggerStatus defines the observed state of WebhookTrigger
type WebhookTriggerStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastTriggeredTime is when the webhook was last triggered successfully.
	// +optional
	LastTriggeredTime *metav1.Time `json:"lastTriggeredTime,omitempty"`

	// TotalTriggered is the total number of times this trigger created a Task.
	// +optional
	TotalTriggered int64 `json:"totalTriggered,omitempty"`

	// ActiveTasks lists the names of currently running Tasks created by this trigger.
	// Used for concurrency policy enforcement.
	// +optional
	ActiveTasks []string `json:"activeTasks,omitempty"`

	// WebhookURL is the full URL path for this webhook endpoint.
	// Format: /webhooks/<namespace>/<trigger-name>
	// +optional
	WebhookURL string `json:"webhookURL,omitempty"`

	// Conditions represent the latest available observations of the WebhookTrigger's state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WebhookTriggerList contains a list of WebhookTrigger
type WebhookTriggerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebhookTrigger `json:"items"`
}
