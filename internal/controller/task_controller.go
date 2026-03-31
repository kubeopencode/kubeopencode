// Copyright Contributors to the KubeOpenCode project

// Package controller implements Kubernetes controllers for KubeOpenCode resources
package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const (
	// ContextConfigMapSuffix is the suffix for ConfigMap names created for context
	ContextConfigMapSuffix = "-context"

	// AgentLabelKey is the label key used to identify which Agent a Task uses
	AgentLabelKey = "kubeopencode.io/agent"

	// DefaultQueuedRequeueDelay is the default delay for requeuing queued Tasks
	DefaultQueuedRequeueDelay = 10 * time.Second

	// DefaultQuotaRequeueDelay is the minimum delay for requeuing quota-blocked Tasks
	DefaultQuotaRequeueDelay = 30 * time.Second

	// AnnotationStop is the annotation key for user-initiated task stop
	AnnotationStop = "kubeopencode.io/stop"

	// KubeOpenCodeConfigName is the singleton name for the cluster-scoped KubeOpenCodeConfig.
	// Following OpenShift convention, cluster-wide config resources are named "cluster".
	KubeOpenCodeConfigName = "cluster"

	// RuntimeSystemPrompt is the system prompt injected when Runtime context is enabled.
	// It provides KubeOpenCode platform awareness to the agent.
	RuntimeSystemPrompt = `## KubeOpenCode Runtime Context

You are running as an AI agent inside a Kubernetes Pod, managed by KubeOpenCode.

### Environment Variables
- TASK_NAME: Name of the current Task CR
- TASK_NAMESPACE: Namespace of the current Task CR
- WORKSPACE_DIR: Working directory where task.md and context files are mounted

### Getting More Information
To get full Task specification:
  kubectl get task ${TASK_NAME} -n ${TASK_NAMESPACE} -o yaml

To get Task status:
  kubectl get task ${TASK_NAME} -n ${TASK_NAMESPACE} -o jsonpath='{.status}'

### File Structure
- ${WORKSPACE_DIR}/task.md: Your task instructions (description only)
- ${WORKSPACE_DIR}/.kubeopencode/context.md: KubeOpenCode context (loaded via OpenCode instructions)
- Additional contexts may be mounted as separate files
- Note: Repository's AGENTS.md/CLAUDE.md files are preserved and loaded by OpenCode automatically

### KubeOpenCode Concepts
- Task: Single AI task execution (what you're running now)
- Agent: Configuration for how tasks are executed (image, credentials, etc.)
`
)

// isTaskFinished returns true if the task phase is terminal (Completed or Failed).
func isTaskFinished(phase kubeopenv1alpha1.TaskPhase) bool {
	return phase == kubeopenv1alpha1.TaskPhaseCompleted || phase == kubeopenv1alpha1.TaskPhaseFailed
}

// isTaskStoppedByUser returns true if the task has been marked for stopping via annotation.
func isTaskStoppedByUser(task *kubeopenv1alpha1.Task) bool {
	return task.Annotations != nil && task.Annotations[AnnotationStop] == "true"
}

// TaskReconciler reconciles a Task object
type TaskReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=kubeopencode.io,resources=tasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeopencode.io,resources=tasks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeopencode.io,resources=tasks/finalizers,verbs=update
// +kubebuilder:rbac:groups=kubeopencode.io,resources=agents,verbs=get;list;watch
// +kubebuilder:rbac:groups=kubeopencode.io,resources=agents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeopencode.io,resources=kubeopencodeconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *TaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get Task CR
	task := &kubeopenv1alpha1.Task{}
	if err := r.Get(ctx, req.NamespacedName, task); err != nil {
		if errors.IsNotFound(err) {
			// Task deleted, nothing to do
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Task")
		return ctrl.Result{}, err
	}

	// If new, initialize status and create Pod
	// Also handle incomplete Running state (Running but no Pod created yet)
	// This can happen if context processing failed after Phase was set to Running
	// and the status update to Failed encountered a conflict
	if task.Status.Phase == "" ||
		(task.Status.Phase == kubeopenv1alpha1.TaskPhaseRunning && task.Status.PodName == "") {
		return r.initializeTask(ctx, task)
	}

	// If queued, check if capacity is available
	if task.Status.Phase == kubeopenv1alpha1.TaskPhaseQueued {
		return r.handleQueuedTask(ctx, task)
	}

	// If completed/failed, handle cleanup based on KubeOpenCodeConfig
	if isTaskFinished(task.Status.Phase) {
		return r.handleTaskCleanup(ctx, task)
	}

	// Check for user-initiated stop (only for Running tasks)
	if task.Status.Phase == kubeopenv1alpha1.TaskPhaseRunning {
		if isTaskStoppedByUser(task) {
			return r.handleStop(ctx, task)
		}
	}

	// Update task status from Pod status (both Pod mode and Server mode use Pods now)
	if err := r.updateTaskStatusFromPod(ctx, task); err != nil {
		log.Error(err, "unable to update task status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// initializeTask initializes a new Task and creates its Pod
func (r *TaskReconciler) initializeTask(ctx context.Context, task *kubeopenv1alpha1.Task) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get agent configuration with name (Agent must be in same namespace as Task)
	agentConfig, agentName, err := r.getAgentConfigWithName(ctx, task)
	if err != nil {
		log.Error(err, "unable to get Agent")
		return r.updateTaskFailed(ctx, task, kubeopenv1alpha1.ReasonAgentError, err)
	}

	// Add agent label to Task
	needsUpdate := false
	if task.Labels == nil {
		task.Labels = make(map[string]string)
	}
	if task.Labels[AgentLabelKey] != agentName {
		task.Labels[AgentLabelKey] = agentName
		needsUpdate = true
	}

	if needsUpdate {
		if err := r.Update(ctx, task); err != nil {
			log.Error(err, "unable to update Task")
			return ctrl.Result{}, err
		}
		// Requeue to continue with updated task
		return ctrl.Result{Requeue: true}, nil
	}

	// Check if agent is suspended (Server mode only)
	// Use agentConfig.serverConfig directly — no extra API call needed
	if agentConfig.serverConfig != nil && agentConfig.serverConfig.Suspend {
		log.Info("agent is suspended, queueing task", "agent", agentName)
		r.Recorder.Eventf(task, nil, corev1.EventTypeNormal, "Queued", "Queued", "Agent %q is suspended, task queued", agentName)

		task.Status.ObservedGeneration = task.Generation
		task.Status.Phase = kubeopenv1alpha1.TaskPhaseQueued
		task.Status.AgentRef = &kubeopenv1alpha1.AgentReference{Name: agentName}

		meta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
			Type:    kubeopenv1alpha1.ConditionTypeQueued,
			Status:  metav1.ConditionTrue,
			Reason:  kubeopenv1alpha1.ReasonAgentSuspended,
			Message: fmt.Sprintf("Agent %q is suspended", agentName),
		})

		if err := r.Status().Update(ctx, task); err != nil {
			log.Error(err, "unable to update Task status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: DefaultQueuedRequeueDelay}, nil
	}

	// Check agent capacity if MaxConcurrentTasks is set
	if agentConfig.maxConcurrentTasks != nil && *agentConfig.maxConcurrentTasks > 0 {
		hasCapacity, err := r.checkAgentCapacity(ctx, task.Namespace, agentName, *agentConfig.maxConcurrentTasks)
		if err != nil {
			log.Error(err, "unable to check agent capacity")
			return ctrl.Result{}, err
		}

		if !hasCapacity {
			// Agent is at capacity, queue the task
			log.Info("agent at capacity, queueing task", "agent", agentName, "maxConcurrent", *agentConfig.maxConcurrentTasks)
			r.Recorder.Eventf(task, nil, corev1.EventTypeNormal, "Queued", "Queued", "Agent %q at capacity (max: %d), task queued", agentName, *agentConfig.maxConcurrentTasks)

			task.Status.ObservedGeneration = task.Generation
			task.Status.Phase = kubeopenv1alpha1.TaskPhaseQueued
			task.Status.AgentRef = &kubeopenv1alpha1.AgentReference{
				Name: agentName,
			}

			meta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
				Type:    kubeopenv1alpha1.ConditionTypeQueued,
				Status:  metav1.ConditionTrue,
				Reason:  kubeopenv1alpha1.ReasonAgentAtCapacity,
				Message: fmt.Sprintf("Waiting for agent %q capacity (max: %d)", agentName, *agentConfig.maxConcurrentTasks),
			})

			if err := r.Status().Update(ctx, task); err != nil {
				log.Error(err, "unable to update Task status")
				return ctrl.Result{}, err
			}

			// Requeue with delay
			return ctrl.Result{RequeueAfter: DefaultQueuedRequeueDelay}, nil
		}
	}

	// Check agent quota if configured
	if agentConfig.quota != nil {
		agent, err := r.getAgentForQuota(ctx, agentName, task.Namespace)
		if err != nil {
			log.Error(err, "unable to get Agent for quota check")
			return ctrl.Result{}, err
		}

		hasQuota, requeueDelay, err := r.checkAgentQuota(ctx, agent)
		if err != nil {
			log.Error(err, "unable to check agent quota")
			return ctrl.Result{}, err
		}

		if !hasQuota {
			// Quota exceeded, queue the task
			log.Info("agent quota exceeded, queueing task",
				"agent", agentName,
				"maxTaskStarts", agentConfig.quota.MaxTaskStarts,
				"windowSeconds", agentConfig.quota.WindowSeconds)
			r.Recorder.Eventf(task, nil, corev1.EventTypeNormal, "QuotaExceeded", "Queued", "Agent %q quota exceeded (max: %d per %ds), task queued", agentName, agentConfig.quota.MaxTaskStarts, agentConfig.quota.WindowSeconds)

			task.Status.ObservedGeneration = task.Generation
			task.Status.Phase = kubeopenv1alpha1.TaskPhaseQueued
			task.Status.AgentRef = &kubeopenv1alpha1.AgentReference{
				Name: agentName,
			}

			meta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
				Type:   kubeopenv1alpha1.ConditionTypeQueued,
				Status: metav1.ConditionTrue,
				Reason: kubeopenv1alpha1.ReasonQuotaExceeded,
				Message: fmt.Sprintf("Waiting for agent %q quota (max: %d per %ds)",
					agentName, agentConfig.quota.MaxTaskStarts, agentConfig.quota.WindowSeconds),
			})

			if err := r.Status().Update(ctx, task); err != nil {
				log.Error(err, "unable to update Task status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: requeueDelay}, nil
		}
	}

	// Pre-occupy capacity slot by setting Task status to Running BEFORE creating Pod.
	// This prevents race condition where multiple Tasks pass capacity check simultaneously.
	// If Pod creation fails later, status will be set to Failed (not reverted to empty).
	if task.Status.Phase != kubeopenv1alpha1.TaskPhaseRunning {
		task.Status.ObservedGeneration = task.Generation
		task.Status.Phase = kubeopenv1alpha1.TaskPhaseRunning
		task.Status.AgentRef = &kubeopenv1alpha1.AgentReference{
			Name: agentName,
		}
		now := metav1.Now()
		task.Status.StartTime = &now

		if err := r.Status().Update(ctx, task); err != nil {
			if errors.IsConflict(err) {
				// Optimistic lock conflict - another reconcile is in progress
				// Requeue to let it complete first
				log.V(1).Info("conflict pre-occupying capacity slot, requeuing")
				return ctrl.Result{Requeue: true}, nil
			}
			log.Error(err, "unable to pre-occupy capacity slot")
			return ctrl.Result{}, err
		}

		// Refresh task to get updated version
		if err := r.Get(ctx, types.NamespacedName{Name: task.Name, Namespace: task.Namespace}, task); err != nil {
			log.Error(err, "unable to refresh task after pre-occupying slot")
			return ctrl.Result{}, err
		}

		log.Info("pre-occupied capacity slot", "task", task.Name, "agent", agentName)
	}

	// Determine server URL for Server-mode Agents (empty for Pod mode)
	// In Server mode, Tasks create Pods that use `opencode run --attach` to connect
	// to the persistent OpenCode server instead of running a standalone instance.
	serverURL := ""
	if agentConfig.serverConfig != nil {
		port := GetServerPort(&kubeopenv1alpha1.Agent{Spec: kubeopenv1alpha1.AgentSpec{ServerConfig: agentConfig.serverConfig}})
		serverURL = ServerURL(agentName, task.Namespace, port)
		log.Info("Creating Pod for Server-mode Task", "serverURL", serverURL)
	}

	// Generate Pod name
	podName := fmt.Sprintf("%s-pod", task.Name)

	// Check if Pod already exists
	existingPod := &corev1.Pod{}
	podKey := types.NamespacedName{Name: podName, Namespace: task.Namespace}
	if err := r.Get(ctx, podKey, existingPod); err == nil {
		// Pod already exists, update status with Pod info
		task.Status.PodName = podName
		if updateErr := r.Status().Update(ctx, task); updateErr != nil {
			if errors.IsConflict(updateErr) {
				log.V(1).Info("conflict updating existing Pod status, requeuing")
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	// Process all contexts using priority-based resolution
	// Priority (lowest to highest):
	//   1. Agent.contexts (Agent-level defaults)
	//   2. Task.contexts (Task-specific contexts)
	//   3. Task.description (highest, becomes ${WORKSPACE_DIR}/task.md)
	contextConfigMap, fileMounts, dirMounts, gitMounts, err := r.processAllContexts(ctx, task, agentConfig)
	if err != nil {
		log.Error(err, "unable to process contexts")

		// Refresh task to get latest version before updating status
		if refreshErr := r.Get(ctx, types.NamespacedName{Name: task.Name, Namespace: task.Namespace}, task); refreshErr != nil {
			log.Error(refreshErr, "unable to refresh task for context error status update")
			return ctrl.Result{}, refreshErr
		}

		return r.updateTaskFailed(ctx, task, kubeopenv1alpha1.ReasonContextError, err)
	}

	// Create ConfigMap in Task's namespace (where Pod runs)
	if contextConfigMap != nil {
		if err := r.Create(ctx, contextConfigMap); err != nil {
			if !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create context ConfigMap")

				// Refresh task to get latest version before updating status
				if refreshErr := r.Get(ctx, types.NamespacedName{Name: task.Name, Namespace: task.Namespace}, task); refreshErr != nil {
					log.Error(refreshErr, "unable to refresh task for ConfigMap error status update")
					return ctrl.Result{}, refreshErr
				}

				return r.updateTaskFailed(ctx, task, kubeopenv1alpha1.ReasonConfigMapCreationError, err)
			}
		}
	}

	// Get system configuration (image, pull policies, proxy) from cluster-scoped KubeOpenCodeConfig
	sysCfg := r.getSystemConfig(ctx)

	// Apply cluster-level defaults where Agent doesn't specify its own
	agentConfig.applySystemDefaults(sysCfg)

	// Create Pod with agent configuration and context mounts
	// For Server-mode, serverURL is passed to generate --attach command
	pod := buildPod(task, podName, agentConfig, contextConfigMap, fileMounts, dirMounts, gitMounts, sysCfg, serverURL)

	// Record task start for quota tracking BEFORE creating Pod.
	// This ensures quota is accurate even if Pod creation fails.
	// If Pod creation fails, we rollback the quota record.
	var quotaAgent *kubeopenv1alpha1.Agent
	if agentConfig.quota != nil {
		var err error
		quotaAgent, err = r.getAgentForQuota(ctx, agentName, task.Namespace)
		if err != nil {
			log.Error(err, "unable to get Agent for quota recording")

			// Refresh task to get latest version before updating status
			if refreshErr := r.Get(ctx, types.NamespacedName{Name: task.Name, Namespace: task.Namespace}, task); refreshErr != nil {
				log.Error(refreshErr, "unable to refresh task for quota error status update")
				return ctrl.Result{}, refreshErr
			}

			return r.updateTaskFailed(ctx, task, kubeopenv1alpha1.ReasonAgentError, fmt.Errorf("failed to get Agent for quota: %v", err))
		}

		if err := r.recordTaskStart(ctx, quotaAgent, task); err != nil {
			log.Error(err, "failed to record task start for quota")

			// Refresh task to get latest version before updating status
			if refreshErr := r.Get(ctx, types.NamespacedName{Name: task.Name, Namespace: task.Namespace}, task); refreshErr != nil {
				log.Error(refreshErr, "unable to refresh task for quota record error status update")
				return ctrl.Result{}, refreshErr
			}

			return r.updateTaskFailed(ctx, task, kubeopenv1alpha1.ReasonAgentError, fmt.Errorf("failed to record quota: %v", err))
		}
		log.V(1).Info("recorded task start for quota", "task", task.Name, "agent", agentName)
	}

	if err := r.Create(ctx, pod); err != nil {
		log.Error(err, "unable to create Pod", "pod", podName, "namespace", task.Namespace)
		r.Recorder.Eventf(task, nil, corev1.EventTypeWarning, "PodCreationFailed", "CreatePod", "Failed to create pod: %v", err)

		// Rollback quota record if it was recorded
		if quotaAgent != nil {
			if rollbackErr := r.removeTaskStart(ctx, quotaAgent, task); rollbackErr != nil {
				log.Error(rollbackErr, "failed to rollback quota record after Pod creation failure")
			} else {
				log.V(1).Info("rolled back quota record", "task", task.Name, "agent", agentName)
			}
		}

		// Refresh task to get latest version before updating status
		if refreshErr := r.Get(ctx, types.NamespacedName{Name: task.Name, Namespace: task.Namespace}, task); refreshErr != nil {
			log.Error(refreshErr, "unable to refresh task for Pod creation error status update")
			return ctrl.Result{}, refreshErr
		}

		return r.updateTaskFailed(ctx, task, kubeopenv1alpha1.ReasonPodCreationError, err)
	}

	// Refresh task to get latest version before final status update
	if err := r.Get(ctx, types.NamespacedName{Name: task.Name, Namespace: task.Namespace}, task); err != nil {
		log.Error(err, "unable to refresh task for final status update")
		return ctrl.Result{}, err
	}

	// Update status with Pod info (Task is already Running from pre-occupation)
	task.Status.PodName = podName

	if err := r.Status().Update(ctx, task); err != nil {
		if errors.IsConflict(err) {
			log.V(1).Info("conflict updating final status, requeuing")
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "unable to update Task status")
		return ctrl.Result{}, err
	}

	log.Info("initialized Task", "pod", podName, "image", agentConfig.agentImage)
	r.Recorder.Eventf(task, nil, corev1.EventTypeNormal, "PodCreated", "CreatePod", "Created pod %s", podName)
	return ctrl.Result{}, nil
}

// updateTaskFailed updates the Task status to Failed with a reason and error message.
// This is used for terminal configuration errors where requeuing is not appropriate.
func (r *TaskReconciler) updateTaskFailed(ctx context.Context, task *kubeopenv1alpha1.Task, reason string, err error) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	task.Status.ObservedGeneration = task.Generation
	task.Status.Phase = kubeopenv1alpha1.TaskPhaseFailed
	now := metav1.Now()
	task.Status.CompletionTime = &now
	meta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
		Type:    kubeopenv1alpha1.ConditionTypeReady,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: err.Error(),
	})

	if updateErr := r.Status().Update(ctx, task); updateErr != nil {
		if errors.IsConflict(updateErr) {
			log.V(1).Info("conflict updating failed status, requeuing")
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(updateErr, "unable to update Task status to Failed")
		return ctrl.Result{}, updateErr
	}

	return ctrl.Result{}, nil
}

// updateTaskStatusFromPod syncs task status from Pod status
func (r *TaskReconciler) updateTaskStatusFromPod(ctx context.Context, task *kubeopenv1alpha1.Task) error {
	log := log.FromContext(ctx)

	if task.Status.PodName == "" {
		return nil
	}

	// Get Pod status (Pod is always in same namespace as Task)
	pod := &corev1.Pod{}
	podKey := types.NamespacedName{Name: task.Status.PodName, Namespace: task.Namespace}
	if err := r.Get(ctx, podKey, pod); err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "Pod not found", "pod", task.Status.PodName)
			return nil
		}
		return err
	}

	// Check Pod phase
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		task.Status.ObservedGeneration = task.Generation
		task.Status.Phase = kubeopenv1alpha1.TaskPhaseCompleted
		now := metav1.Now()
		task.Status.CompletionTime = &now
		log.Info("task completed", "pod", task.Status.PodName)
		r.Recorder.Eventf(task, nil, corev1.EventTypeNormal, "Completed", "Completed", "Task completed successfully")
		r.recordTaskDuration(task)
		return r.Status().Update(ctx, task)
	case corev1.PodFailed:
		task.Status.ObservedGeneration = task.Generation
		task.Status.Phase = kubeopenv1alpha1.TaskPhaseFailed
		now := metav1.Now()
		task.Status.CompletionTime = &now
		log.Info("task failed", "pod", task.Status.PodName)
		r.Recorder.Eventf(task, nil, corev1.EventTypeWarning, "Failed", "Failed", "Task failed")
		r.recordTaskDuration(task)
		return r.Status().Update(ctx, task)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
// Pods have OwnerReferences to Tasks (same namespace), so we use Owns for automatic mapping.
func (r *TaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeopenv1alpha1.Task{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

// getAgentConfigWithName retrieves the agent configuration and returns the agent name.
// Agent must be in the same namespace as the Task.
// Returns: agentConfig, agentName, error
func (r *TaskReconciler) getAgentConfigWithName(ctx context.Context, task *kubeopenv1alpha1.Task) (agentConfig, string, error) {
	log := log.FromContext(ctx)

	// AgentRef is required - Task must specify which Agent to use
	if task.Spec.AgentRef == nil {
		return agentConfig{}, "", fmt.Errorf("agentRef is required: Task %q does not specify agentRef", task.Name)
	}

	agentName := task.Spec.AgentRef.Name

	// Get Agent from Task's namespace
	agent := &kubeopenv1alpha1.Agent{}
	agentKey := types.NamespacedName{
		Name:      agentName,
		Namespace: task.Namespace,
	}

	if err := r.Get(ctx, agentKey, agent); err != nil {
		log.Error(err, "unable to get Agent", "agent", agentName, "namespace", task.Namespace)
		return agentConfig{}, "", fmt.Errorf("agent %q not found in namespace %q: %w", agentName, task.Namespace, err)
	}

	// Resolve config, merging with template if referenced
	cfg, err := ResolveAgentConfigFromTemplate(ctx, r.Client, agent)
	if err != nil {
		log.Error(err, "unable to resolve agent config", "agent", agentName)
		return agentConfig{}, "", err
	}

	// ServiceAccountName is required (ResolveAgentConfigFromTemplate validates
	// post-merge, but agents without templates also need this check)
	if cfg.serviceAccountName == "" {
		return agentConfig{}, "", fmt.Errorf("agent %q is missing required field serviceAccountName", agentName)
	}

	return cfg, agentName, nil
}

// getAgentForQuota fetches the Agent object for quota operations.
// This is separate from getAgentConfigWithName to allow quota status updates
// while minimizing changes to existing code paths.
func (r *TaskReconciler) getAgentForQuota(ctx context.Context, agentName, agentNamespace string) (*kubeopenv1alpha1.Agent, error) {
	agent := &kubeopenv1alpha1.Agent{}
	agentKey := types.NamespacedName{
		Name:      agentName,
		Namespace: agentNamespace,
	}

	if err := r.Get(ctx, agentKey, agent); err != nil {
		return nil, err
	}

	return agent, nil
}

// processAllContexts processes all contexts from Agent and Task
// and returns the ConfigMap, file mounts, directory mounts, and git mounts for the Pod.
//
// Content order in task.md (top to bottom):
//  1. Task.description (appears first in task.md)
//  2. Agent.contexts (Agent-level contexts)
//  3. Task.contexts (Task-specific contexts, appears last)
func (r *TaskReconciler) processAllContexts(ctx context.Context, task *kubeopenv1alpha1.Task, cfg agentConfig) (*corev1.ConfigMap, []fileMount, []dirMount, []gitMount, error) {
	var resolved []resolvedContext
	var dirMounts []dirMount
	var gitMounts []gitMount

	// 1. Resolve Agent.contexts (appears after description in task.md)
	agentResolved, agentDirMounts, agentGitMounts, err := processContextItems(r.Client, ctx, cfg.contexts, task.Namespace, cfg.workspaceDir)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to resolve Agent contexts: %w", err)
	}
	resolved = append(resolved, agentResolved...)
	dirMounts = append(dirMounts, agentDirMounts...)
	gitMounts = append(gitMounts, agentGitMounts...)

	// 2. Resolve Task.contexts (appears last in task.md)
	taskResolved, taskDirMounts, taskGitMounts, err := processContextItems(r.Client, ctx, task.Spec.Contexts, task.Namespace, cfg.workspaceDir)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to resolve Task contexts: %w", err)
	}
	resolved = append(resolved, taskResolved...)
	dirMounts = append(dirMounts, taskDirMounts...)
	gitMounts = append(gitMounts, taskGitMounts...)

	// 3. Handle Task.description (highest priority, becomes ${WORKSPACE_DIR}/task.md)
	var taskDescription string
	if task.Spec.Description != nil && *task.Spec.Description != "" {
		taskDescription = *task.Spec.Description
	}

	// Build the final content
	// - Separate contexts with mountPath (independent files)
	// - Contexts without mountPath are written to .kubeopencode/context.md with XML tags
	//   (loaded via OpenCode's instructions config, avoiding conflicts with repo's AGENTS.md)
	// - task.md contains only the description
	configMapData := make(map[string]string)
	var fileMounts []fileMount

	// Build task.md content: description only
	var taskMdParts []string
	if taskDescription != "" {
		taskMdParts = append(taskMdParts, taskDescription)
	}

	// Build context file content: contexts without mountPath
	// Context is written to .kubeopencode/context.md to avoid conflicts with repository's AGENTS.md
	var contextParts []string

	for _, rc := range resolved {
		if rc.mountPath != "" {
			// Context has explicit mountPath - create separate file
			configMapKey := sanitizeConfigMapKey(rc.mountPath)
			configMapData[configMapKey] = rc.content
			fileMounts = append(fileMounts, fileMount{filePath: rc.mountPath, fileMode: rc.fileMode})
		} else {
			// No mountPath - append to .kubeopencode/context.md with XML tags
			// OpenCode loads this via OPENCODE_CONFIG_CONTENT instructions injection
			xmlTag := fmt.Sprintf("<context name=%q namespace=%q type=%q>\n%s\n</context>",
				rc.name, rc.namespace, rc.ctxType, rc.content)
			contextParts = append(contextParts, xmlTag)
		}
	}

	// Create task.md if there's any content (description only)
	// Mount at the configured workspace directory
	taskMdPath := cfg.workspaceDir + "/task.md"
	if len(taskMdParts) > 0 {
		taskMdContent := strings.Join(taskMdParts, "\n\n")
		configMapData["workspace-task.md"] = taskMdContent
		fileMounts = append(fileMounts, fileMount{filePath: taskMdPath})
	}

	// Create context file if there's any context content
	// Written to .kubeopencode/context.md to avoid conflicts with repository's AGENTS.md/CLAUDE.md
	// OpenCode loads this via OPENCODE_CONFIG_CONTENT env var with instructions config
	contextFilePath := cfg.workspaceDir + "/" + ContextFileRelPath
	if len(contextParts) > 0 {
		contextContent := strings.Join(contextParts, "\n\n")
		configMapData[sanitizeConfigMapKey(contextFilePath)] = contextContent
		fileMounts = append(fileMounts, fileMount{filePath: contextFilePath})
	}

	// Add OpenCode config to ConfigMap if provided
	if cfg.config != nil && *cfg.config != "" {
		// Validate JSON syntax
		var jsonCheck interface{}
		if err := json.Unmarshal([]byte(*cfg.config), &jsonCheck); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("invalid JSON in Agent config: %w", err)
		}
		// Use sanitizeConfigMapKey to ensure consistent key naming with fileMount
		configMapKey := sanitizeConfigMapKey(OpenCodeConfigPath)
		configMapData[configMapKey] = *cfg.config
		fileMounts = append(fileMounts, fileMount{filePath: OpenCodeConfigPath})
	}

	// Create ConfigMap if there's any content
	// ConfigMap is created in Agent's namespace (where Pod runs)
	var configMap *corev1.ConfigMap
	if len(configMapData) > 0 {
		configMapName := task.Name + ContextConfigMapSuffix
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: task.Namespace,
				Labels: map[string]string{
					"app":                  "kubeopencode",
					"kubeopencode.io/task": task.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(task, kubeopenv1alpha1.SchemeGroupVersion.WithKind("Task")),
				},
			},
			Data: configMapData,
		}
	}

	// Validate mount path conflicts
	// Multiple contexts mounting to the same path would silently overwrite each other,
	// so we detect and report conflicts explicitly.
	if err := validateMountPathConflicts(fileMounts, dirMounts, gitMounts); err != nil {
		return nil, nil, nil, nil, err
	}

	return configMap, fileMounts, dirMounts, gitMounts, nil
}

// validateMountPathConflicts checks for duplicate mount paths across all mount types.
// Returns an error if any two mounts target the same path.
func validateMountPathConflicts(fileMounts []fileMount, dirMounts []dirMount, gitMounts []gitMount) error {
	mountPaths := make(map[string]string) // path -> source description

	for _, fm := range fileMounts {
		if existing, ok := mountPaths[fm.filePath]; ok {
			return fmt.Errorf("mount path conflict: %q is used by both %s and a file mount", fm.filePath, existing)
		}
		mountPaths[fm.filePath] = "file mount"
	}

	for _, dm := range dirMounts {
		if existing, ok := mountPaths[dm.dirPath]; ok {
			return fmt.Errorf("mount path conflict: %q is used by both %s and directory mount (ConfigMap: %s)", dm.dirPath, existing, dm.configMapName)
		}
		mountPaths[dm.dirPath] = fmt.Sprintf("directory mount (ConfigMap: %s)", dm.configMapName)
	}

	for _, gm := range gitMounts {
		if existing, ok := mountPaths[gm.mountPath]; ok {
			return fmt.Errorf("mount path conflict: %q is used by both %s and git mount (%s)", gm.mountPath, existing, gm.contextName)
		}
		mountPaths[gm.mountPath] = fmt.Sprintf("git mount (%s)", gm.contextName)
	}

	return nil
}

// resolveContextItem resolves a ContextItem to its content, directory mount, or git mount.
func (r *TaskReconciler) resolveContextItem(ctx context.Context, item *kubeopenv1alpha1.ContextItem, defaultNS, workspaceDir string) (*resolvedContext, *dirMount, *gitMount, error) {
	return resolveContextItemFromReader(r.Client, ctx, item, defaultNS, workspaceDir)
}

// resolveMountPath converts relative paths to absolute paths based on workspaceDir.
// Paths starting with "/" are treated as absolute and returned as-is.
// Paths NOT starting with "/" are treated as relative and prefixed with workspaceDir.
// This follows Tekton conventions for workspace path resolution.
func resolveMountPath(mountPath, workspaceDir string) string {
	if mountPath == "" {
		return ""
	}
	if strings.HasPrefix(mountPath, "/") {
		return mountPath
	}
	return workspaceDir + "/" + mountPath
}

// resolveContextContent resolves content from a ContextItem.
func (r *TaskReconciler) resolveContextContent(ctx context.Context, namespace, name, workspaceDir string, item *kubeopenv1alpha1.ContextItem, mountPath string) (string, *dirMount, *gitMount, error) {
	return resolveContextContentFromReader(r.Client, ctx, namespace, name, workspaceDir, item, mountPath)
}

// getConfigMapKey retrieves a specific key from a ConfigMap.
func (r *TaskReconciler) getConfigMapKey(ctx context.Context, namespace, name, key string, optional *bool) (string, error) {
	return getConfigMapKeyFromReader(r.Client, ctx, namespace, name, key, optional)
}

// getConfigMapAllKeys retrieves all keys from a ConfigMap and formats them for aggregation.
func (r *TaskReconciler) getConfigMapAllKeys(ctx context.Context, namespace, name string, optional *bool) (string, error) {
	return getConfigMapAllKeysFromReader(r.Client, ctx, namespace, name, optional)
}

// checkAgentCapacity checks if the agent has capacity for a new task.
// Returns true if capacity is available, false if at limit.
func (r *TaskReconciler) checkAgentCapacity(ctx context.Context, namespace, agentName string, maxConcurrent int32) (bool, error) {
	log := log.FromContext(ctx)

	// List all Tasks for this Agent using label selector
	taskList := &kubeopenv1alpha1.TaskList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{AgentLabelKey: agentName},
	}

	if err := r.List(ctx, taskList, listOpts...); err != nil {
		return false, err
	}

	// Count running and queued tasks
	runningCount := int32(0)
	queuedCount := int32(0)
	for i := range taskList.Items {
		task := &taskList.Items[i]
		switch task.Status.Phase {
		case kubeopenv1alpha1.TaskPhaseRunning:
			runningCount++
		case kubeopenv1alpha1.TaskPhaseQueued:
			queuedCount++
		}
	}

	log.V(1).Info("agent capacity check", "agent", agentName, "running", runningCount, "max", maxConcurrent)

	// Record capacity metric
	AgentCapacity.WithLabelValues(agentName, namespace).Set(float64(maxConcurrent - runningCount))
	AgentQueueLength.WithLabelValues(agentName, namespace).Set(float64(queuedCount))

	return runningCount < maxConcurrent, nil
}

// handleQueuedTask checks if a queued task can now be started
func (r *TaskReconciler) handleQueuedTask(ctx context.Context, task *kubeopenv1alpha1.Task) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Check for user-initiated stop (Queued tasks can also be stopped)
	if isTaskStoppedByUser(task) {
		log.Info("user-initiated stop detected for queued task", "task", task.Name)

		task.Status.ObservedGeneration = task.Generation
		task.Status.Phase = kubeopenv1alpha1.TaskPhaseCompleted
		now := metav1.Now()
		task.Status.CompletionTime = &now
		meta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
			Type:    kubeopenv1alpha1.ConditionTypeStopped,
			Status:  metav1.ConditionTrue,
			Reason:  kubeopenv1alpha1.ReasonUserStopped,
			Message: "Task was stopped while queued",
		})

		if err := r.Status().Update(ctx, task); err != nil {
			log.Error(err, "unable to update stopped task status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Get agent configuration with name
	agentConfig, agentName, err := r.getAgentConfigWithName(ctx, task)
	if err != nil {
		log.Error(err, "unable to get Agent for queued task")
		// Agent might be deleted, fail the task
		task.Status.ObservedGeneration = task.Generation
		task.Status.Phase = kubeopenv1alpha1.TaskPhaseFailed
		now := metav1.Now()
		task.Status.CompletionTime = &now
		meta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
			Type:    kubeopenv1alpha1.ConditionTypeReady,
			Status:  metav1.ConditionFalse,
			Reason:  kubeopenv1alpha1.ReasonAgentError,
			Message: err.Error(),
		})
		if updateErr := r.Status().Update(ctx, task); updateErr != nil {
			log.Error(updateErr, "unable to update Task status")
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	// Check if agent is still suspended
	if agentConfig.serverConfig != nil && agentConfig.serverConfig.Suspend {
		log.V(1).Info("agent still suspended, remaining queued", "agent", agentName)
		return ctrl.Result{RequeueAfter: DefaultQueuedRequeueDelay}, nil
	}

	// Check if agent still has MaxConcurrentTasks set
	hasCapacityLimit := agentConfig.maxConcurrentTasks != nil && *agentConfig.maxConcurrentTasks > 0
	hasQuotaLimit := agentConfig.quota != nil

	// If neither limit is configured, proceed to initialize
	if !hasCapacityLimit && !hasQuotaLimit {
		log.Info("no limits configured, proceeding with task", "agent", agentName)
		task.Status.Phase = ""
		meta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
			Type:    kubeopenv1alpha1.ConditionTypeQueued,
			Status:  metav1.ConditionFalse,
			Reason:  kubeopenv1alpha1.ReasonNoLimits,
			Message: fmt.Sprintf("Agent %q has no capacity or quota limits", agentName),
		})
		if err := r.Status().Update(ctx, task); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Check capacity if limit is set
	if hasCapacityLimit {
		hasCapacity, err := r.checkAgentCapacity(ctx, task.Namespace, agentName, *agentConfig.maxConcurrentTasks)
		if err != nil {
			log.Error(err, "unable to check agent capacity")
			return ctrl.Result{}, err
		}

		if !hasCapacity {
			// Still at capacity, requeue
			log.V(1).Info("agent still at capacity, remaining queued", "agent", agentName)
			return ctrl.Result{RequeueAfter: DefaultQueuedRequeueDelay}, nil
		}
	}

	// Check agent quota if configured
	if agentConfig.quota != nil {
		agent, err := r.getAgentForQuota(ctx, agentName, task.Namespace)
		if err != nil {
			log.Error(err, "unable to get Agent for quota check")
			return ctrl.Result{}, err
		}

		hasQuota, requeueDelay, err := r.checkAgentQuota(ctx, agent)
		if err != nil {
			log.Error(err, "unable to check agent quota")
			return ctrl.Result{}, err
		}

		if !hasQuota {
			// Quota still exceeded, update condition and requeue
			log.V(1).Info("agent quota still exceeded, remaining queued",
				"agent", agentName,
				"maxTaskStarts", agentConfig.quota.MaxTaskStarts,
				"windowSeconds", agentConfig.quota.WindowSeconds)

			// Ensure AgentRef is set (may be missing from older tasks)
			if task.Status.AgentRef == nil {
				task.Status.AgentRef = &kubeopenv1alpha1.AgentReference{
					Name: agentName,
				}
			}

			meta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
				Type:   kubeopenv1alpha1.ConditionTypeQueued,
				Status: metav1.ConditionTrue,
				Reason: kubeopenv1alpha1.ReasonQuotaExceeded,
				Message: fmt.Sprintf("Waiting for agent %q quota (max: %d per %ds)",
					agentName, agentConfig.quota.MaxTaskStarts, agentConfig.quota.WindowSeconds),
			})

			if err := r.Status().Update(ctx, task); err != nil {
				log.Error(err, "unable to update queued task status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: requeueDelay}, nil
		}
	}

	// Capacity available, transition to empty phase to trigger initializeTask
	log.Info("agent capacity available, transitioning to initialize", "agent", agentName)
	task.Status.Phase = ""
	meta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
		Type:    kubeopenv1alpha1.ConditionTypeQueued,
		Status:  metav1.ConditionFalse,
		Reason:  kubeopenv1alpha1.ReasonCapacityAvailable,
		Message: fmt.Sprintf("Agent %q capacity now available", agentName),
	})

	if err := r.Status().Update(ctx, task); err != nil {
		log.Error(err, "unable to update queued task status")
		return ctrl.Result{}, err
	}

	// Requeue immediately to trigger initializeTask
	return ctrl.Result{Requeue: true}, nil
}

// handleStop handles user-initiated task stop via annotation.
// It deletes the Pod which triggers graceful termination via SIGTERM.
// The Pod is deleted but logs may remain accessible for a short period via kubectl logs.
func (r *TaskReconciler) handleStop(ctx context.Context, task *kubeopenv1alpha1.Task) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("user-initiated stop detected", "task", task.Name)

	// Delete the Pod if it exists
	if task.Status.PodName != "" {
		pod := &corev1.Pod{}
		podKey := types.NamespacedName{Name: task.Status.PodName, Namespace: task.Namespace}
		if err := r.Get(ctx, podKey, pod); err == nil {
			// Delete the Pod - Kubernetes will automatically:
			// 1. Send SIGTERM to the Pod
			// 2. Wait for graceful termination period (default 30s)
			// 3. Forcefully kill if still running
			if err := r.Delete(ctx, pod); err != nil {
				log.Error(err, "failed to delete pod")
				return ctrl.Result{}, err
			}
			log.Info("deleted pod for stopped task", "pod", task.Status.PodName)
		}
	}

	// Update Task status to Completed with Stopped condition
	task.Status.Phase = kubeopenv1alpha1.TaskPhaseCompleted
	task.Status.ObservedGeneration = task.Generation
	now := metav1.Now()
	task.Status.CompletionTime = &now

	meta.SetStatusCondition(&task.Status.Conditions, metav1.Condition{
		Type:    kubeopenv1alpha1.ConditionTypeStopped,
		Status:  metav1.ConditionTrue,
		Reason:  kubeopenv1alpha1.ReasonUserStopped,
		Message: "Task stopped by user via kubeopencode.io/stop annotation",
	})

	r.Recorder.Eventf(task, nil, corev1.EventTypeNormal, "Stopped", "Stopped", "Task stopped by user")

	if err := r.Status().Update(ctx, task); err != nil {
		log.Error(err, "failed to update task status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// getSystemConfig retrieves the system configuration from KubeOpenCodeConfig.
func (r *TaskReconciler) getSystemConfig(ctx context.Context) systemConfig {
	return resolveSystemConfig(ctx, r.Client)
}

// resolveSystemConfig retrieves system-level configuration from the cluster-scoped
// KubeOpenCodeConfig singleton. Returns defaults if no config is found.
// Shared by TaskReconciler and AgentReconciler.
func resolveSystemConfig(ctx context.Context, reader client.Reader) systemConfig {
	logger := log.FromContext(ctx)

	cfg := systemConfig{
		systemImage:           DefaultKubeOpenCodeImage,
		systemImagePullPolicy: corev1.PullIfNotPresent,
	}

	config := &kubeopenv1alpha1.KubeOpenCodeConfig{}
	configKey := types.NamespacedName{Name: KubeOpenCodeConfigName}

	if err := reader.Get(ctx, configKey, config); err != nil {
		if !errors.IsNotFound(err) {
			logger.Error(err, "unable to get KubeOpenCodeConfig for system config, using defaults")
		}
		return cfg
	}

	if config.Spec.SystemImage != nil {
		if config.Spec.SystemImage.Image != "" {
			cfg.systemImage = config.Spec.SystemImage.Image
		}
		if config.Spec.SystemImage.ImagePullPolicy != "" {
			cfg.systemImagePullPolicy = config.Spec.SystemImage.ImagePullPolicy
		}
	}

	cfg.proxy = config.Spec.Proxy

	return cfg
}

// handleTaskCleanup handles automatic cleanup of completed/failed Tasks based on KubeOpenCodeConfig.
// It checks both TTL-based and retention-based cleanup policies.
// Returns a Result with RequeueAfter if TTL cleanup is scheduled for the future.
func (r *TaskReconciler) handleTaskCleanup(ctx context.Context, task *kubeopenv1alpha1.Task) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get cleanup configuration from cluster-scoped KubeOpenCodeConfig
	cleanupConfig := r.getCleanupConfig(ctx)
	if cleanupConfig == nil {
		// No cleanup configured, nothing to do
		return ctrl.Result{}, nil
	}

	// Check TTL-based cleanup
	if cleanupConfig.TTLSecondsAfterFinished != nil {
		result, deleted, err := r.checkTTLCleanup(ctx, task, *cleanupConfig.TTLSecondsAfterFinished)
		if err != nil {
			log.Error(err, "failed to check TTL cleanup")
			return ctrl.Result{}, err
		}
		if deleted {
			// Task was deleted, nothing more to do
			return result, nil
		}
		// If TTL is set but not expired, we need to requeue
		// Continue to check retention limit
		if result.RequeueAfter > 0 {
			// Schedule requeue for TTL expiration
			// Also check retention cleanup before returning
			if cleanupConfig.MaxRetainedTasks != nil {
				if err := r.checkRetentionCleanup(ctx, task.Namespace, *cleanupConfig.MaxRetainedTasks); err != nil {
					log.Error(err, "failed to check retention cleanup")
					return ctrl.Result{}, err
				}
			}
			return result, nil
		}
	}

	// Check retention-based cleanup
	if cleanupConfig.MaxRetainedTasks != nil {
		if err := r.checkRetentionCleanup(ctx, task.Namespace, *cleanupConfig.MaxRetainedTasks); err != nil {
			log.Error(err, "failed to check retention cleanup")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// getCleanupConfig retrieves cleanup configuration from cluster-scoped KubeOpenCodeConfig.
// Returns nil if no cleanup is configured.
func (r *TaskReconciler) getCleanupConfig(ctx context.Context) *kubeopenv1alpha1.CleanupConfig {
	log := log.FromContext(ctx)

	// Try to get cluster-scoped KubeOpenCodeConfig
	config := &kubeopenv1alpha1.KubeOpenCodeConfig{}
	configKey := types.NamespacedName{Name: KubeOpenCodeConfigName}

	if err := r.Get(ctx, configKey, config); err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "unable to get KubeOpenCodeConfig for cleanup config")
		}
		// Config not found, no cleanup configured
		return nil
	}

	return config.Spec.Cleanup
}

// checkTTLCleanup checks if the Task should be deleted based on TTL.
// Returns (result, deleted, error) where deleted is true if the Task was deleted.
func (r *TaskReconciler) checkTTLCleanup(ctx context.Context, task *kubeopenv1alpha1.Task, ttlSeconds int32) (ctrl.Result, bool, error) {
	log := log.FromContext(ctx)

	// CompletionTime must be set for TTL calculation
	if task.Status.CompletionTime == nil {
		// This shouldn't happen for completed/failed tasks, but handle gracefully
		return ctrl.Result{}, false, nil
	}

	// Calculate elapsed time since completion
	elapsed := time.Since(task.Status.CompletionTime.Time)
	ttlDuration := time.Duration(ttlSeconds) * time.Second

	if elapsed >= ttlDuration {
		// TTL expired, delete the Task
		log.Info("deleting Task due to TTL expiration",
			"task", task.Name,
			"ttlSeconds", ttlSeconds,
			"elapsedSeconds", int(elapsed.Seconds()))

		if err := r.Delete(ctx, task); err != nil {
			if errors.IsNotFound(err) {
				// Already deleted, that's fine
				return ctrl.Result{}, true, nil
			}
			return ctrl.Result{}, false, err
		}
		return ctrl.Result{}, true, nil
	}

	// TTL not expired yet, schedule requeue for when it expires
	remaining := ttlDuration - elapsed
	log.V(1).Info("scheduling TTL cleanup",
		"task", task.Name,
		"remainingSeconds", int(remaining.Seconds()))

	return ctrl.Result{RequeueAfter: remaining}, false, nil
}

// checkRetentionCleanup checks if any Tasks should be deleted based on retention count.
// Deletes the oldest completed/failed Tasks (by CompletionTime) if count exceeds limit.
func (r *TaskReconciler) checkRetentionCleanup(ctx context.Context, namespace string, maxRetained int32) error {
	log := log.FromContext(ctx)

	// List all Tasks in the namespace
	taskList := &kubeopenv1alpha1.TaskList{}
	if err := r.List(ctx, taskList, client.InNamespace(namespace)); err != nil {
		return err
	}

	// Filter completed/failed Tasks with CompletionTime set
	var completedTasks []kubeopenv1alpha1.Task
	for _, task := range taskList.Items {
		if isTaskFinished(task.Status.Phase) &&
			task.Status.CompletionTime != nil {
			completedTasks = append(completedTasks, task)
		}
	}

	// Check if we're over the limit
	excess := len(completedTasks) - int(maxRetained)
	if excess <= 0 {
		// Under limit, nothing to delete
		return nil
	}

	// Sort by CompletionTime (oldest first)
	sort.Slice(completedTasks, func(i, j int) bool {
		return completedTasks[i].Status.CompletionTime.Before(completedTasks[j].Status.CompletionTime)
	})

	// Delete excess Tasks (oldest first)
	log.Info("deleting Tasks due to retention limit",
		"namespace", namespace,
		"count", len(completedTasks),
		"maxRetained", maxRetained,
		"toDelete", excess)

	for i := 0; i < excess; i++ {
		taskToDelete := &completedTasks[i]
		log.Info("deleting Task due to retention limit",
			"task", taskToDelete.Name,
			"completionTime", taskToDelete.Status.CompletionTime.Time)

		if err := r.Delete(ctx, taskToDelete); err != nil {
			if errors.IsNotFound(err) {
				// Already deleted, continue
				continue
			}
			return err
		}
	}

	return nil
}

// pruneTaskStartHistory removes expired records from TaskStartHistory.
// Records older than the quota window are removed.
func pruneTaskStartHistory(history []kubeopenv1alpha1.TaskStartRecord, windowSeconds int32) []kubeopenv1alpha1.TaskStartRecord {
	windowStart := time.Now().Add(-time.Duration(windowSeconds) * time.Second)
	var pruned []kubeopenv1alpha1.TaskStartRecord
	for _, record := range history {
		if record.StartTime.Time.After(windowStart) { //nolint:staticcheck // Using embedded time.Time's After method
			pruned = append(pruned, record)
		}
	}
	return pruned
}

// calculateQuotaRequeueDelay calculates when the next quota slot becomes available.
// Returns the time until the oldest record in the window expires, with a minimum
// of DefaultQuotaRequeueDelay.
func calculateQuotaRequeueDelay(history []kubeopenv1alpha1.TaskStartRecord, windowSeconds int32) time.Duration {
	if len(history) == 0 {
		return DefaultQuotaRequeueDelay
	}

	// Find the oldest record in the window
	activeRecords := pruneTaskStartHistory(history, windowSeconds)
	if len(activeRecords) == 0 {
		return DefaultQuotaRequeueDelay
	}

	// Sort by StartTime to find the oldest
	sort.Slice(activeRecords, func(i, j int) bool {
		return activeRecords[i].StartTime.Time.Before(activeRecords[j].StartTime.Time) //nolint:staticcheck // Using embedded time.Time's Before method
	})

	// Calculate when the oldest record expires
	oldestRecord := activeRecords[0]
	windowDuration := time.Duration(windowSeconds) * time.Second
	expiresAt := oldestRecord.StartTime.Time.Add(windowDuration) //nolint:staticcheck // Using embedded time.Time's Add method
	delay := time.Until(expiresAt)

	// Ensure minimum delay
	if delay < DefaultQuotaRequeueDelay {
		delay = DefaultQuotaRequeueDelay
	}

	return delay
}

// checkAgentQuota checks if a new Task can start based on the Agent's quota.
// Returns (hasQuota, requeueAfter, error):
//   - hasQuota=true: Task can start immediately
//   - hasQuota=false: Task should be queued, requeueAfter is the suggested delay
func (r *TaskReconciler) checkAgentQuota(ctx context.Context, agent *kubeopenv1alpha1.Agent) (bool, time.Duration, error) {
	log := log.FromContext(ctx)

	if agent.Spec.Quota == nil {
		return true, 0, nil
	}

	quota := agent.Spec.Quota
	activeRecords := pruneTaskStartHistory(agent.Status.TaskStartHistory, quota.WindowSeconds)
	currentCount := int32(len(activeRecords)) //nolint:gosec // len() is always non-negative and bounded by slice capacity

	log.V(1).Info("quota check",
		"agent", agent.Name,
		"activeCount", currentCount,
		"maxTaskStarts", quota.MaxTaskStarts,
		"windowSeconds", quota.WindowSeconds)

	if currentCount >= quota.MaxTaskStarts {
		requeueDelay := calculateQuotaRequeueDelay(agent.Status.TaskStartHistory, quota.WindowSeconds)
		return false, requeueDelay, nil
	}

	return true, 0, nil
}

// recordTaskStart adds a TaskStartRecord to the Agent's status.
// Uses retry logic for optimistic concurrency conflicts in HA mode.
func (r *TaskReconciler) recordTaskStart(ctx context.Context, agent *kubeopenv1alpha1.Agent, task *kubeopenv1alpha1.Task) error {
	log := log.FromContext(ctx)

	if agent.Spec.Quota == nil {
		return nil
	}

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		// Fetch fresh Agent
		freshAgent := &kubeopenv1alpha1.Agent{}
		if err := r.Get(ctx, types.NamespacedName{Name: agent.Name, Namespace: agent.Namespace}, freshAgent); err != nil {
			return err
		}

		// Check if quota is still configured (could be removed between retries)
		if freshAgent.Spec.Quota == nil {
			return nil
		}

		// Prune old records and add new one
		freshAgent.Status.TaskStartHistory = pruneTaskStartHistory(
			freshAgent.Status.TaskStartHistory,
			freshAgent.Spec.Quota.WindowSeconds,
		)
		freshAgent.Status.TaskStartHistory = append(freshAgent.Status.TaskStartHistory, kubeopenv1alpha1.TaskStartRecord{
			TaskName:      task.Name,
			TaskNamespace: task.Namespace,
			StartTime:     metav1.Now(),
		})

		// Update status
		if err := r.Status().Update(ctx, freshAgent); err != nil {
			if errors.IsConflict(err) {
				log.V(1).Info("conflict updating agent status, retrying", "retry", i+1)
				continue
			}
			return err
		}

		log.V(1).Info("recorded task start for quota",
			"agent", agent.Name,
			"task", task.Name,
			"historySize", len(freshAgent.Status.TaskStartHistory))
		return nil
	}

	return fmt.Errorf("failed to record task start after %d retries", maxRetries)
}

// removeTaskStart removes a task start record from Agent status.
// This is used to rollback quota recording when Pod creation fails.
func (r *TaskReconciler) removeTaskStart(ctx context.Context, agent *kubeopenv1alpha1.Agent, task *kubeopenv1alpha1.Task) error {
	log := log.FromContext(ctx)

	if agent.Spec.Quota == nil {
		return nil
	}

	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		// Fetch fresh Agent
		freshAgent := &kubeopenv1alpha1.Agent{}
		if err := r.Get(ctx, types.NamespacedName{Name: agent.Name, Namespace: agent.Namespace}, freshAgent); err != nil {
			return err
		}

		// Filter out the record for this task
		var filtered []kubeopenv1alpha1.TaskStartRecord
		for _, record := range freshAgent.Status.TaskStartHistory {
			if record.TaskName != task.Name || record.TaskNamespace != task.Namespace {
				filtered = append(filtered, record)
			}
		}
		freshAgent.Status.TaskStartHistory = filtered

		// Update status
		if err := r.Status().Update(ctx, freshAgent); err != nil {
			if errors.IsConflict(err) {
				log.V(1).Info("conflict removing task start, retrying", "retry", i+1)
				continue
			}
			return err
		}

		log.V(1).Info("removed task start record",
			"agent", agent.Name,
			"task", task.Name,
			"historySize", len(freshAgent.Status.TaskStartHistory))
		return nil
	}

	return fmt.Errorf("failed to remove task start after %d retries", maxRetries)
}

// recordTaskDuration records the task duration in the Prometheus histogram.
func (r *TaskReconciler) recordTaskDuration(task *kubeopenv1alpha1.Task) {
	if task.Status.StartTime == nil || task.Status.CompletionTime == nil {
		return
	}
	duration := task.Status.CompletionTime.Time.Sub(task.Status.StartTime.Time).Seconds()
	agentName := ""
	if task.Status.AgentRef != nil {
		agentName = task.Status.AgentRef.Name
	}
	TaskDurationSeconds.WithLabelValues(task.Namespace, agentName).Observe(duration)
}
