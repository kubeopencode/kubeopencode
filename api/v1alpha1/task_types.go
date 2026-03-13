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
	// ReasonPodCreationError is the reason for Pod creation failures
	ReasonPodCreationError = "PodCreationError"
	// ReasonConfigMapCreationError is the reason for ConfigMap creation failures
	ReasonConfigMapCreationError = "ConfigMapCreationError"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=tk
// +kubebuilder:printcolumn:JSONPath=`.status.phase`,name="Phase",type=string
// +kubebuilder:printcolumn:JSONPath=`.status.agentRef.name`,name="Agent",type=string
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
// The Agent must be in the same namespace as the Task.
type AgentReference struct {
	// Name of the Agent.
	// +required
	Name string `json:"name"`
}

// TaskSpec defines the Task configuration
type TaskSpec struct {
	// Description is the task instruction/prompt.
	// The controller creates ${WORKSPACE_DIR}/task.md with this content
	// (where WORKSPACE_DIR is configured in Agent.spec.workspaceDir, defaulting to "/workspace").
	// This is the primary way to tell the agent what to do.
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
	//   2. Task.contexts (Task-specific contexts)
	//   3. Task.description (highest, becomes ${WORKSPACE_DIR}/task.md)
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

	// AgentRef references an Agent in the same namespace for this task.
	//
	// +required
	AgentRef *AgentReference `json:"agentRef,omitempty"`
}

// TaskExecutionStatus defines the observed state of Task
type TaskExecutionStatus struct {
	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Execution phase
	// +optional
	Phase TaskPhase `json:"phase,omitempty"`

	// AgentRef is the resolved Agent reference used for this task.
	// +optional
	AgentRef *AgentReference `json:"agentRef,omitempty"`

	// Kubernetes Pod name
	// +optional
	PodName string `json:"podName,omitempty"`

	// Start time
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Completion time
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Kubernetes standard conditions
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TaskList contains a list of Task
type TaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Task `json:"items"`
}
