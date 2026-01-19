// Copyright Contributors to the KubeOpenCode project

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TaskPhase represents the current phase of a task
// +kubebuilder:validation:Enum=Pending;Queued;Running;Completed;Failed
type TaskPhase string

const (
	// TaskPhasePending means the task has not started yet
	TaskPhasePending TaskPhase = "Pending"
	// TaskPhaseQueued means the task is waiting for Agent capacity.
	// This occurs when the Agent has maxConcurrentTasks set and the limit is reached.
	// The task will automatically transition to Running when capacity becomes available.
	TaskPhaseQueued TaskPhase = "Queued"
	// TaskPhaseRunning means the task is currently executing
	TaskPhaseRunning TaskPhase = "Running"
	// TaskPhaseCompleted means the task execution finished (Job exited with code 0).
	// This indicates the agent completed its work, not necessarily that the task "succeeded".
	// The actual outcome should be determined by examining the agent's output.
	TaskPhaseCompleted TaskPhase = "Completed"
	// TaskPhaseFailed means the task had an infrastructure failure
	// (e.g., Job crashed, unable to schedule, missing Agent).
	TaskPhaseFailed TaskPhase = "Failed"
)

const (
	// ConditionTypeReady is the condition type for Task readiness
	ConditionTypeReady = "Ready"
	// ConditionTypeQueued is the condition type for Task queuing
	ConditionTypeQueued = "Queued"
	// ConditionTypeStopped is the condition type for Task stop
	ConditionTypeStopped = "Stopped"

	// ReasonTaskTemplateError is the reason for TaskTemplate errors
	ReasonTaskTemplateError = "TaskTemplateError"
	// ReasonAgentError is the reason for Agent errors
	ReasonAgentError = "AgentError"
	// ReasonAgentAtCapacity is the reason for Agent capacity limit
	ReasonAgentAtCapacity = "AgentAtCapacity"
	// ReasonQuotaExceeded is the reason for Agent quota limit
	ReasonQuotaExceeded = "QuotaExceeded"
	// ReasonContextError is the reason for Context errors
	ReasonContextError = "ContextError"
	// ReasonUserStopped is the reason for user-initiated stop
	ReasonUserStopped = "UserStopped"
	// ReasonNoLimits is the reason for no limits configured
	ReasonNoLimits = "NoLimits"
	// ReasonCapacityAvailable is the reason for capacity availability
	ReasonCapacityAvailable = "CapacityAvailable"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=tk
// +kubebuilder:printcolumn:JSONPath=`.status.phase`,name="Phase",type=string
// +kubebuilder:printcolumn:JSONPath=`.status.podName`,name="Pod",type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

// Task represents a single task execution.
// Task is the primary API for users who want to execute AI-powered tasks.
type Task struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of Task
	Spec TaskSpec `json:"spec"`

	// Status represents the current status of the Task
	// +optional
	Status TaskExecutionStatus `json:"status,omitempty"`
}

// AgentReference specifies which Agent to use for task execution.
// Supports cross-namespace references to enable separation of concerns:
// - Platform teams manage Agents with credentials in dedicated namespaces
// - Dev teams create Tasks in their own namespaces, referencing shared Agents
type AgentReference struct {
	// Name of the Agent.
	// +required
	Name string `json:"name"`

	// Namespace of the Agent.
	// If empty, defaults to the Task's namespace.
	// When specified, the Pod runs in the Agent's namespace (not the Task's namespace),
	// allowing credentials to stay isolated from Task creators.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// TaskSpec defines the Task configuration
type TaskSpec struct {
	// TaskTemplateRef references a TaskTemplate to use as base configuration.
	// The template's settings are merged with this Task's settings.
	//
	// When using a template:
	//   - TaskTemplate.agentRef is used if Task.agentRef is not specified
	//   - TaskTemplate.contexts are prepended to Task.contexts
	//   - TaskTemplate.outputs are merged with Task.outputs (Task takes precedence)
	//   - TaskTemplate.description is used if Task.description is not specified
	//
	// Example:
	//   taskTemplateRef:
	//     name: pr-task-template
	//     namespace: platform-templates
	// +optional
	TaskTemplateRef *TaskTemplateReference `json:"taskTemplateRef,omitempty"`

	// Description is the task instruction/prompt.
	// The controller creates ${WORKSPACE_DIR}/task.md with this content
	// (where WORKSPACE_DIR is configured in Agent.spec.workspaceDir, defaulting to "/workspace").
	// This is the primary way to tell the agent what to do.
	//
	// If taskTemplateRef is specified and description is not set,
	// the template's description is used.
	//
	// Example:
	//   description: "Update all dependencies and create a PR"
	// +optional
	Description *string `json:"description,omitempty"`

	// Contexts provides additional context for the task.
	// Contexts are processed in array order, with later contexts taking precedence.
	//
	// Context priority (lowest to highest):
	//   1. Agent.contexts (Agent-level defaults)
	//   2. TaskTemplate.contexts (Template-level defaults, if taskTemplateRef is set)
	//   3. Task.contexts (Task-specific contexts)
	//   4. Task.description (highest, becomes ${WORKSPACE_DIR}/task.md)
	//
	// Example:
	//   contexts:
	//     - type: Text
	//       text: "Always use conventional commits"
	//     - type: Git
	//       mountPath: src
	//       git:
	//         repository: https://github.com/org/repo
	//         ref: main
	// +optional
	Contexts []ContextItem `json:"contexts,omitempty"`

	// AgentRef references an Agent for this task.
	// Supports cross-namespace references: when Agent is in a different namespace,
	// the Pod runs in the Agent's namespace to keep credentials isolated.
	//
	// If not specified and taskTemplateRef is set, uses the template's agentRef.
	// If neither is specified, uses the "default" Agent in the same namespace.
	// +optional
	AgentRef *AgentReference `json:"agentRef,omitempty"`

	// Outputs defines output parameters to capture from this Task.
	// The controller creates a sidecar to capture these outputs from files.
	//
	// If taskTemplateRef is specified, outputs are merged with the template's outputs.
	// Task outputs take precedence for same-named parameters.
	//
	// Example:
	//   outputs:
	//     parameters:
	//       - name: pr-url
	//         path: ".outputs/pr-url"
	//       - name: summary
	//         path: ".outputs/summary"
	//         default: "No summary provided"
	// +optional
	Outputs *OutputSpec `json:"outputs,omitempty"`
}

// TaskExecutionStatus defines the observed state of Task
type TaskExecutionStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Execution phase
	// +optional
	Phase TaskPhase `json:"phase,omitempty"`

	// Kubernetes Pod name
	// +optional
	PodName string `json:"podName,omitempty"`

	// PodNamespace indicates where the Pod is running.
	// This may differ from Task's namespace when using cross-namespace Agent reference.
	// When Agent is in a different namespace, the Pod runs in the Agent's namespace
	// to keep credentials isolated from Task creators.
	// +optional
	PodNamespace string `json:"podNamespace,omitempty"`

	// Start time
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Completion time
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Outputs contains results captured from task execution.
	// This is populated when the task completes (either success or failure).
	// +optional
	Outputs *TaskOutputs `json:"outputs,omitempty"`

	// Kubernetes standard conditions
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// TaskOutputs contains parameters captured from task execution files.
// The output-collector sidecar reads files specified in OutputSpec and
// writes the captured values to the Pod's termination message.
//
// Size limits (due to Kubernetes termination message 4KB limit):
//   - Total JSON output: max 4KB
//   - For larger outputs, consider using external storage (future feature)
type TaskOutputs struct {
	// Parameters is a key-value map of outputs captured from files.
	// Keys are defined in Task.spec.outputs.
	// Values are read from the corresponding file paths by the sidecar.
	//
	// Example usage:
	//   kubectl get task my-task -o jsonpath='{.status.outputs.parameters.pr-url}'
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// OutputSpec defines output parameters to capture from task execution.
type OutputSpec struct {
	// Parameters defines the output parameters to capture from files.
	// Each parameter specifies a name and file path to read from.
	// +optional
	Parameters []OutputParameterSpec `json:"parameters,omitempty"`
}

// OutputParameterSpec defines a single output parameter to capture from a file.
type OutputParameterSpec struct {
	// Name is the parameter name, used as the key in status.outputs.parameters.
	// Must be unique within the output parameters.
	// +required
	Name string `json:"name"`

	// Path is the file path to read the parameter value from.
	// Relative paths are prefixed with workspaceDir.
	// Example: ".outputs/pr-url" -> ${WORKSPACE_DIR}/.outputs/pr-url
	// +required
	Path string `json:"path"`

	// Default is the default value if the file doesn't exist.
	// If not specified and file doesn't exist, parameter is omitted from output.
	// +optional
	Default *string `json:"default,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskList contains a list of Task
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}

// TaskTemplateReference specifies which TaskTemplate to use.
// Supports cross-namespace references to enable sharing templates across namespaces.
type TaskTemplateReference struct {
	// Name of the TaskTemplate.
	// +required
	Name string `json:"name"`

	// Namespace of the TaskTemplate.
	// If empty, defaults to the Task's namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope="Namespaced",shortName=tt
// +kubebuilder:printcolumn:JSONPath=`.spec.agentRef.name`,name="Agent",type=string
// +kubebuilder:printcolumn:JSONPath=`.metadata.creationTimestamp`,name="Age",type=date

// TaskTemplate defines a reusable template for Task creation.
// TaskTemplates allow users to define common Task configurations (contexts, outputs, agentRef)
// that can be shared across multiple Tasks. Similar to Argo WorkflowTemplate.
type TaskTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the template configuration
	Spec TaskTemplateSpec `json:"spec"`
}

// TaskTemplateSpec defines the template for Task creation.
// It contains all TaskSpec fields that can be shared across multiple Tasks.
type TaskTemplateSpec struct {
	// Description is the default task instruction/prompt.
	// Can be overridden by Task.spec.description.
	// If Task doesn't specify description, this value is used.
	// +optional
	Description *string `json:"description,omitempty"`

	// AgentRef references an Agent for tasks using this template.
	// Can be overridden by Task.spec.agentRef.
	// +optional
	AgentRef *AgentReference `json:"agentRef,omitempty"`

	// Contexts provides default contexts for tasks using this template.
	// These are merged with Task.spec.contexts (Task contexts appended after template contexts).
	//
	// Context priority (lowest to highest):
	//   1. Agent.contexts (Agent-level defaults)
	//   2. TaskTemplate.contexts (Template-level defaults)
	//   3. Task.contexts (Task-specific contexts)
	//   4. Task.description (highest, becomes ${WORKSPACE_DIR}/task.md)
	// +optional
	Contexts []ContextItem `json:"contexts,omitempty"`

	// Outputs defines default output parameters for tasks using this template.
	// Parameters are merged with Task.spec.outputs (Task takes precedence for same-named params).
	// +optional
	Outputs *OutputSpec `json:"outputs,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskTemplateList contains a list of TaskTemplate
type TaskTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TaskTemplate `json:"items"`
}
