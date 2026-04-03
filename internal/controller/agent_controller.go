// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const (
	// AgentConditionServerReady indicates whether the OpenCode server is ready.
	AgentConditionServerReady = "ServerReady"

	// AgentConditionServerHealthy indicates whether the Agent is responding to health checks.
	// Based on Deployment readiness rather than HTTP health checks.
	AgentConditionServerHealthy = "ServerHealthy"

	// AgentConditionSuspended indicates whether the Agent is intentionally suspended.
	AgentConditionSuspended = "Suspended"

	// AgentConditionGitSyncPending indicates a Git sync rollout is waiting
	// for active Tasks to complete before triggering a Deployment update.
	AgentConditionGitSyncPending = "GitSyncPending"

	// AgentConditionStandbyConfigWarning indicates a warning about standby configuration.
	AgentConditionStandbyConfigWarning = "StandbyConfigWarning"

	// AnnotationLastConnectionActive tracks the last time an active user connection
	// (web terminal or CLI attach) was observed. Value is an RFC3339 timestamp.
	// Used by reconcileStandby to prevent auto-suspend while users are connected.
	AnnotationLastConnectionActive = "kubeopencode.io/last-connection-active"

	// ConnectionHeartbeatInterval is how often clients update the connection annotation.
	ConnectionHeartbeatInterval = 60 * time.Second

	// ConnectionHeartbeatStaleness is how long after the last heartbeat update
	// the connection is considered stale (no longer active).
	ConnectionHeartbeatStaleness = 2 * time.Minute

	// DefaultServerReconcileInterval is how often to reconcile Agents.
	DefaultServerReconcileInterval = 30 * time.Second
)

// GitLsRemoteFunc is the function signature for checking remote Git refs.
type GitLsRemoteFunc func(ctx context.Context, repo, ref, secretName string) (string, error)

// CountActiveTasksFunc is the function signature for counting active tasks.
type CountActiveTasksFunc func(ctx context.Context, agentName, namespace string) (int, error)

// AgentReconciler reconciles Agent resources.
// It manages the Deployment and Service for each Agent.
type AgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// GitLsRemoteFn checks remote Git refs. Defaults to gitLsRemote.
	GitLsRemoteFn GitLsRemoteFunc
	// CountActiveTasksFn counts active tasks. Defaults to r.countActiveTasks.
	CountActiveTasksFn CountActiveTasksFunc
}

// +kubebuilder:rbac:groups=kubeopencode.io,resources=agents,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=kubeopencode.io,resources=agents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeopencode.io,resources=tasks,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;delete

// Reconcile handles Agent reconciliation.
// It ensures the Deployment and Service exist and are up-to-date.
func (r *AgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Agent
	var agent kubeopenv1alpha1.Agent
	if err := r.Get(ctx, req.NamespacedName, &agent); err != nil {
		if apierrors.IsNotFound(err) {
			// Agent was deleted, nothing to do (Deployment/Service will be garbage collected)
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Agent")
		return ctrl.Result{}, err
	}

	// Manage agent-template label
	if err := r.reconcileTemplateLabel(ctx, &agent); err != nil {
		logger.Error(err, "Failed to reconcile template label")
		return ctrl.Result{}, err
	}

	// Resolve agent configuration (merge with template if referenced).
	agentCfg, err := r.resolveAgentConfig(ctx, &agent)
	if err != nil {
		logger.Error(err, "Failed to resolve agent config")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling Agent", "agent", agent.Name)
	sysCfg := r.getSystemConfig(ctx)

	// Apply cluster-level defaults where Agent doesn't specify its own
	agentCfg.applySystemDefaults(sysCfg)

	// Process Agent contexts (Text, ConfigMap, Git, Runtime)
	contextConfigMap, fileMounts, dirMounts, gitMounts, err := r.processAgentContexts(ctx, &agent, agentCfg)
	if err != nil {
		logger.Error(err, "Failed to process Agent contexts")
		return ctrl.Result{}, err
	}

	// Reconcile context ConfigMap if there are any contexts to store
	if err := r.reconcileContextConfigMap(ctx, &agent, contextConfigMap); err != nil {
		logger.Error(err, "Failed to reconcile context ConfigMap")
		return ctrl.Result{}, err
	}

	// Handle standby lifecycle (auto-suspend/auto-resume).
	// If spec.suspend was patched, return early — the patch triggers a new reconcile.
	patched, err := r.reconcileStandby(ctx, &agent)
	if err != nil {
		logger.Error(err, "Failed to reconcile standby")
		return ctrl.Result{}, err
	}
	if patched {
		return ctrl.Result{}, nil
	}

	// Reconcile persistence PVCs if configured
	if err := r.reconcilePVC(ctx, &agent, BuildServerSessionPVC, "session"); err != nil {
		logger.Error(err, "Failed to reconcile session PVC")
		return ctrl.Result{}, err
	}
	if err := r.reconcilePVC(ctx, &agent, BuildServerWorkspacePVC, "workspace"); err != nil {
		logger.Error(err, "Failed to reconcile workspace PVC")
		return ctrl.Result{}, err
	}

	// Reconcile Git sync (detect remote changes, handle rollout with Task protection)
	gitHashAnnotations, syncRequeueAfter, err := r.reconcileGitSync(ctx, &agent, gitMounts)
	if err != nil {
		logger.Error(err, "Failed to reconcile git sync")
		return ctrl.Result{}, err
	}

	// Reconcile the Deployment (with context support)
	if err := r.reconcileDeployment(ctx, &agent, agentCfg, sysCfg, contextConfigMap, fileMounts, dirMounts, gitMounts, gitHashAnnotations); err != nil {
		logger.Error(err, "Failed to reconcile Deployment")
		return ctrl.Result{}, err
	}

	// Reconcile the Service
	if err := r.reconcileService(ctx, &agent); err != nil {
		logger.Error(err, "Failed to reconcile Service")
		return ctrl.Result{}, err
	}

	// Update Agent status
	if err := r.updateAgentStatus(ctx, &agent); err != nil {
		logger.Error(err, "Failed to update Agent status")
		return ctrl.Result{}, err
	}

	// Calculate optimal requeue interval.
	// Use the shortest of: default interval, standby idle timeout, git sync interval.
	requeueAfter := DefaultServerReconcileInterval
	if agent.Spec.Standby != nil && !agent.Spec.Suspend && agent.Status.IdleSince != nil {
		remaining := time.Until(agent.Status.IdleSince.Time.Add(agent.Spec.Standby.IdleTimeout.Duration))
		if remaining > 0 && remaining < requeueAfter {
			requeueAfter = remaining
		}
	}
	if syncRequeueAfter > 0 && syncRequeueAfter < requeueAfter {
		requeueAfter = syncRequeueAfter
	}
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// reconcileDeployment ensures the Deployment exists and is up-to-date.
func (r *AgentReconciler) reconcileDeployment(ctx context.Context, agent *kubeopenv1alpha1.Agent, agentCfg agentConfig, sysCfg systemConfig, contextConfigMap *corev1.ConfigMap, fileMounts []fileMount, dirMounts []dirMount, gitMounts []gitMount, gitHashAnnotations map[string]string) error {
	logger := log.FromContext(ctx)

	desired := BuildServerDeployment(agent, agentCfg, sysCfg, contextConfigMap, fileMounts, dirMounts, gitMounts, gitHashAnnotations)

	// Scale to 0 replicas when suspended
	if agent.Spec.Suspend {
		replicas := int32(0)
		desired.Spec.Replicas = &replicas
	}

	// Set owner reference for garbage collection
	if err := controllerutil.SetControllerReference(agent, desired, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Check if Deployment exists
	var existing appsv1.Deployment
	err := r.Get(ctx, client.ObjectKey{Namespace: desired.Namespace, Name: desired.Name}, &existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create the Deployment
			logger.Info("Creating Deployment for Agent", "deployment", desired.Name)
			if err := r.Create(ctx, desired); err != nil {
				return fmt.Errorf("failed to create Deployment: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get Deployment: %w", err)
	}

	// Update the Deployment if needed
	// For now, we do a simple update of the spec
	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	if err := r.Update(ctx, &existing); err != nil {
		return fmt.Errorf("failed to update Deployment: %w", err)
	}

	return nil
}

// reconcileService ensures the Service exists and is up-to-date.
func (r *AgentReconciler) reconcileService(ctx context.Context, agent *kubeopenv1alpha1.Agent) error {
	logger := log.FromContext(ctx)

	desired := BuildServerService(agent)

	// Set owner reference for garbage collection
	if err := controllerutil.SetControllerReference(agent, desired, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Check if Service exists
	var existing corev1.Service
	err := r.Get(ctx, client.ObjectKey{Namespace: desired.Namespace, Name: desired.Name}, &existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Create the Service
			logger.Info("Creating Service for Agent", "service", desired.Name)
			if err := r.Create(ctx, desired); err != nil {
				return fmt.Errorf("failed to create Service: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get Service: %w", err)
	}

	// Update the Service if needed
	// Preserve ClusterIP as it's immutable
	desired.Spec.ClusterIP = existing.Spec.ClusterIP
	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	if err := r.Update(ctx, &existing); err != nil {
		return fmt.Errorf("failed to update Service: %w", err)
	}

	return nil
}

// updateAgentStatus updates the Agent's status with deployment information.
// Health is determined by Deployment readiness (liveness/readiness probes on the Deployment
// already check the server's /session/status endpoint).
func (r *AgentReconciler) updateAgentStatus(ctx context.Context, agent *kubeopenv1alpha1.Agent) error {
	deploymentName := ServerDeploymentName(agent.Name)
	agent.Status.DeploymentName = deploymentName
	agent.Status.ServiceName = ServerServiceName(agent.Name)
	agent.Status.URL = ServerURL(agent.Name, agent.Namespace, GetServerPort(agent))

	// status.Suspended mirrors spec.suspend
	if agent.Spec.Suspend {
		agent.Status.Suspended = true
		agent.Status.Ready = false

		// Determine suspend reason
		reason := "UserRequested"
		message := "Agent is suspended"
		if agent.Spec.Standby != nil {
			reason = "Standby"
			message = fmt.Sprintf("Agent is suspended (standby configured with %s idle timeout)", agent.Spec.Standby.IdleTimeout.Duration)
		}
		setAgentCondition(agent, AgentConditionSuspended, metav1.ConditionTrue, reason, message)
		setAgentCondition(agent, AgentConditionServerReady, metav1.ConditionFalse, "Suspended", message)
	} else {
		agent.Status.Suspended = false
		setAgentCondition(agent, AgentConditionSuspended, metav1.ConditionFalse, "Active", "Agent is active")

		var deployment appsv1.Deployment
		err := r.Get(ctx, client.ObjectKey{Namespace: agent.Namespace, Name: deploymentName}, &deployment)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to get Deployment: %w", err)
			}
			agent.Status.Ready = false
			setAgentCondition(agent, AgentConditionServerHealthy, metav1.ConditionFalse, "DeploymentNotFound", "Agent deployment not found")
		} else {
			agent.Status.Ready = deployment.Status.ReadyReplicas > 0

			if agent.Status.Ready {
				setAgentCondition(agent, AgentConditionServerHealthy, metav1.ConditionTrue, "DeploymentHealthy", "Agent deployment is ready")
			} else {
				setAgentCondition(agent, AgentConditionServerHealthy, metav1.ConditionFalse, "DeploymentNotReady", "Agent deployment is not ready")
			}
		}

		// Set ServerReady condition
		if agent.Status.Ready {
			setAgentCondition(agent, AgentConditionServerReady, metav1.ConditionTrue, "DeploymentReady", "Agent deployment is ready")
		} else {
			setAgentCondition(agent, AgentConditionServerReady, metav1.ConditionFalse, "DeploymentNotReady", "Agent deployment is not ready")
		}
	}

	// Update observed generation
	agent.Status.ObservedGeneration = agent.Generation

	// Update the status
	if err := r.Status().Update(ctx, agent); err != nil {
		return fmt.Errorf("failed to update Agent status: %w", err)
	}

	return nil
}

// processAgentContexts resolves Agent-level contexts into a ConfigMap, file mounts, dir mounts, and git mounts.
// This is similar to TaskReconciler.processAllContexts but only handles Agent.contexts (no Task description).
func (r *AgentReconciler) processAgentContexts(ctx context.Context, agent *kubeopenv1alpha1.Agent, cfg agentConfig) (*corev1.ConfigMap, []fileMount, []dirMount, []gitMount, error) {
	if len(cfg.contexts) == 0 && len(cfg.skills) == 0 && (cfg.config == nil || *cfg.config == "") {
		return nil, nil, nil, nil, nil
	}

	var resolved []resolvedContext
	var dirMounts []dirMount
	var gitMounts []gitMount
	configMapData := make(map[string]string)
	var fileMounts []fileMount

	// Resolve all context items
	if len(cfg.contexts) > 0 {
		ctxResolved, ctxDirMounts, ctxGitMounts, err := processContextItems(r.Client, ctx, cfg.contexts, agent.Namespace, cfg.workspaceDir)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to resolve Agent contexts: %w", err)
		}
		resolved = append(resolved, ctxResolved...)
		dirMounts = append(dirMounts, ctxDirMounts...)
		gitMounts = append(gitMounts, ctxGitMounts...)
	}

	// Build ConfigMap data from resolved contexts
	if len(resolved) > 0 {
		ctxData, ctxFileMounts := buildContextConfigMapData(resolved, cfg.workspaceDir)
		for k, v := range ctxData {
			configMapData[k] = v
		}
		fileMounts = append(fileMounts, ctxFileMounts...)
	}

	// Process skills and inject config (skills.paths + user config → opencode.json)
	skillGitMounts, fileMounts, err := processSkillsAndInjectConfig(cfg.skills, cfg.config, configMapData, fileMounts)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	gitMounts = append(gitMounts, skillGitMounts...)

	// Validate mount path conflicts
	if err := validateMountPathConflicts(fileMounts, dirMounts, gitMounts); err != nil {
		return nil, nil, nil, nil, err
	}

	// Build ConfigMap
	var contextConfigMap *corev1.ConfigMap
	if len(configMapData) > 0 {
		contextConfigMap = BuildServerContextConfigMap(agent, configMapData)
	}

	return contextConfigMap, fileMounts, dirMounts, gitMounts, nil
}

// reconcileContextConfigMap ensures the context ConfigMap exists and is up-to-date.
func (r *AgentReconciler) reconcileContextConfigMap(ctx context.Context, agent *kubeopenv1alpha1.Agent, desired *corev1.ConfigMap) error {
	logger := log.FromContext(ctx)
	configMapName := ServerContextConfigMapName(agent.Name)

	if desired == nil {
		// No contexts — clean up existing ConfigMap if present
		var existing corev1.ConfigMap
		if err := r.Get(ctx, client.ObjectKey{Namespace: agent.Namespace, Name: configMapName}, &existing); err == nil {
			logger.Info("Cleaning up stale context ConfigMap", "configmap", configMapName)
			if err := r.Delete(ctx, &existing); err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete context ConfigMap: %w", err)
			}
		}
		return nil
	}

	// Set owner reference for garbage collection
	if err := controllerutil.SetControllerReference(agent, desired, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on context ConfigMap: %w", err)
	}

	// Check if ConfigMap exists
	var existing corev1.ConfigMap
	err := r.Get(ctx, client.ObjectKey{Namespace: desired.Namespace, Name: desired.Name}, &existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating context ConfigMap for Agent", "configmap", desired.Name)
			if err := r.Create(ctx, desired); err != nil {
				return fmt.Errorf("failed to create context ConfigMap: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get context ConfigMap: %w", err)
	}

	// Update ConfigMap data
	existing.Data = desired.Data
	existing.Labels = desired.Labels
	if err := r.Update(ctx, &existing); err != nil {
		return fmt.Errorf("failed to update context ConfigMap: %w", err)
	}

	return nil
}

// reconcilePVC ensures a PVC exists when the build function returns a desired PVC.
// PVCs are immutable after creation, so we only create — never update.
func (r *AgentReconciler) reconcilePVC(ctx context.Context, agent *kubeopenv1alpha1.Agent, buildFn func(*kubeopenv1alpha1.Agent) (*corev1.PersistentVolumeClaim, error), label string) error {
	logger := log.FromContext(ctx)

	desired, err := buildFn(agent)
	if err != nil {
		return fmt.Errorf("failed to build %s PVC: %w", label, err)
	}
	if desired == nil {
		// Persistence not configured for this volume type.
		// Stale PVCs are cleaned up by cleanupServerResources (on mode switch)
		// and by OwnerReference GC (on Agent deletion).
		return nil
	}

	// Set owner reference for garbage collection
	if err := controllerutil.SetControllerReference(agent, desired, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference on %s PVC: %w", label, err)
	}

	// Check if PVC already exists
	var existing corev1.PersistentVolumeClaim
	err = r.Get(ctx, client.ObjectKey{Namespace: desired.Namespace, Name: desired.Name}, &existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating PVC for Agent", "pvc", desired.Name, "type", label)
			if err := r.Create(ctx, desired); err != nil {
				return fmt.Errorf("failed to create %s PVC: %w", label, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get %s PVC: %w", label, err)
	}

	// PVC already exists — no update needed (PVC spec is immutable)
	return nil
}

// countActiveTasks counts Tasks targeting this Agent that are in Running, Queued, or Pending phase.
func (r *AgentReconciler) countActiveTasks(ctx context.Context, agentName, namespace string) (int, error) {
	taskList := &kubeopenv1alpha1.TaskList{}
	if err := r.List(ctx, taskList,
		client.InNamespace(namespace),
		client.MatchingLabels{AgentLabelKey: agentName},
	); err != nil {
		return 0, fmt.Errorf("failed to list tasks for agent %q: %w", agentName, err)
	}

	count := 0
	for i := range taskList.Items {
		phase := taskList.Items[i].Status.Phase
		if phase == kubeopenv1alpha1.TaskPhaseRunning ||
			phase == kubeopenv1alpha1.TaskPhaseQueued ||
			phase == kubeopenv1alpha1.TaskPhasePending ||
			phase == "" {
			count++
		}
	}
	return count, nil
}

// hasActiveConnection checks whether the Agent has a recent connection heartbeat annotation.
// Returns true if the annotation exists and is within the given staleness threshold.
func hasActiveConnection(agent *kubeopenv1alpha1.Agent, staleness time.Duration) bool {
	if agent.Annotations == nil {
		return false
	}
	val, ok := agent.Annotations[AnnotationLastConnectionActive]
	if !ok || val == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return false
	}
	return time.Since(t) < staleness
}

// reconcileHeartbeatStaleness returns the effective staleness threshold for the Agent
// and reconciles the StandbyConfigWarning condition as a side effect.
// If idleTimeout < ConnectionHeartbeatStaleness, it degrades to idleTimeout/2 and sets
// a warning condition. Enforces a minimum floor of ConnectionHeartbeatInterval to ensure
// the heartbeat has a chance to update before staleness expires.
func reconcileHeartbeatStaleness(agent *kubeopenv1alpha1.Agent) time.Duration {
	if agent.Spec.Standby == nil {
		return ConnectionHeartbeatStaleness
	}

	staleness := ConnectionHeartbeatStaleness
	if agent.Spec.Standby.IdleTimeout.Duration < staleness {
		staleness = agent.Spec.Standby.IdleTimeout.Duration / 2
		// Enforce minimum floor: staleness must be at least the heartbeat interval,
		// otherwise the heartbeat will always be stale by the time the controller checks.
		if staleness < ConnectionHeartbeatInterval {
			staleness = ConnectionHeartbeatInterval
		}
		setAgentCondition(agent, AgentConditionStandbyConfigWarning, metav1.ConditionTrue,
			"IdleTimeoutTooShort",
			fmt.Sprintf("idleTimeout (%s) is less than connection heartbeat staleness (%s); "+
				"connection-aware detection may be degraded",
				agent.Spec.Standby.IdleTimeout.Duration, ConnectionHeartbeatStaleness))
	} else {
		// Clear warning if previously set
		meta.RemoveStatusCondition(&agent.Status.Conditions, AgentConditionStandbyConfigWarning)
	}
	return staleness
}

// reconcileStandby manages the standby lifecycle (auto-suspend/auto-resume).
// When standby is configured, the controller patches spec.suspend:
//   - Sets spec.suspend=true when idle timeout expires (no active Tasks and no active connections)
//   - Sets spec.suspend=false when a new Task arrives on a suspended Agent
//
// Returns true if spec.suspend was patched (caller should return early since
// the patch triggers a new reconcile via the watch).
func (r *AgentReconciler) reconcileStandby(ctx context.Context, agent *kubeopenv1alpha1.Agent) (bool, error) {
	logger := log.FromContext(ctx)

	if agent.Spec.Standby == nil {
		// Standby not configured — clear idle tracking if leftover
		agent.Status.IdleSince = nil
		return false, nil
	}

	// Compute effective staleness threshold (may degrade if idleTimeout is too short)
	staleness := reconcileHeartbeatStaleness(agent)

	// Detect resume transition: spec says active but status still shows suspended.
	// Reset idle timer so the agent gets a full idle timeout period after resume.
	if !agent.Spec.Suspend && agent.Status.Suspended && agent.Status.IdleSince != nil {
		logger.Info("Agent just resumed, resetting idle timer", "agent", agent.Name)
		agent.Status.IdleSince = nil
	}

	activeTasks, err := r.CountActiveTasksFn(ctx, agent.Name, agent.Namespace)
	if err != nil {
		return false, err
	}

	activeConnection := hasActiveConnection(agent, staleness)

	if activeTasks > 0 {
		// Tasks are active — clear idle timer
		if agent.Status.IdleSince != nil {
			logger.Info("Tasks active, clearing idle timer", "agent", agent.Name, "activeTasks", activeTasks)
			agent.Status.IdleSince = nil
		}

		// Auto-resume: if agent is suspended and tasks are waiting, unsuspend
		if agent.Spec.Suspend {
			logger.Info("Standby auto-resume: active tasks detected, unsuspending agent", "agent", agent.Name, "activeTasks", activeTasks)
			patch := client.MergeFrom(agent.DeepCopy())
			agent.Spec.Suspend = false
			if err := r.Patch(ctx, agent, patch); err != nil {
				return false, fmt.Errorf("failed to patch spec.suspend=false for auto-resume: %w", err)
			}
			return true, nil
		}
	} else if activeConnection {
		// No active tasks but active connection — defer idle timer
		if agent.Status.IdleSince != nil {
			logger.Info("Active connection detected, clearing idle timer", "agent", agent.Name)
			agent.Status.IdleSince = nil
		}
	} else {
		// No active tasks and no active connection
		if agent.Status.IdleSince == nil {
			now := metav1.Now()
			agent.Status.IdleSince = &now
			logger.Info("No active tasks or connections, starting idle timer", "agent", agent.Name)
		} else if !agent.Spec.Suspend && time.Since(agent.Status.IdleSince.Time) >= agent.Spec.Standby.IdleTimeout.Duration {
			// Idle timeout expired — auto-suspend
			logger.Info("Standby auto-suspend: idle timeout expired", "agent", agent.Name, "idleTimeout", agent.Spec.Standby.IdleTimeout.Duration)
			patch := client.MergeFrom(agent.DeepCopy())
			agent.Spec.Suspend = true
			if err := r.Patch(ctx, agent, patch); err != nil {
				return false, fmt.Errorf("failed to patch spec.suspend=true for auto-suspend: %w", err)
			}
			return true, nil
		}
	}

	return false, nil
}

// findAgentForTask returns a reconcile request for the Agent referenced by a Task.
// This enables the agent controller to react immediately when Tasks are created or updated,
// supporting fast auto-resume via standby.
func (r *AgentReconciler) findAgentForTask(ctx context.Context, obj client.Object) []reconcile.Request {
	task, ok := obj.(*kubeopenv1alpha1.Task)
	if !ok {
		return nil
	}

	agentName := task.Labels[AgentLabelKey]
	if agentName == "" {
		return nil
	}

	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{
			Name:      agentName,
			Namespace: task.Namespace,
		},
	}}
}

// setAgentCondition sets a condition on the Agent.
func setAgentCondition(agent *kubeopenv1alpha1.Agent, conditionType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&agent.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: agent.Generation,
		Reason:             reason,
		Message:            message,
	})
}

// getSystemConfig retrieves the system configuration from KubeOpenCodeConfig.
func (r *AgentReconciler) getSystemConfig(ctx context.Context) systemConfig {
	return resolveSystemConfig(ctx, r.Client)
}

// LabelAgentTemplate is the label key used to track which AgentTemplate an Agent references.
const LabelAgentTemplate = "kubeopencode.io/agent-template"

// resolveAgentConfig resolves the Agent configuration, merging with template if referenced.
func (r *AgentReconciler) resolveAgentConfig(ctx context.Context, agent *kubeopenv1alpha1.Agent) (agentConfig, error) {
	return ResolveAgentConfigFromTemplate(ctx, r.Client, agent)
}

// reconcileTemplateLabel ensures the agent-template label is consistent with the templateRef.
// Uses Patch instead of Update to avoid unnecessary reconciliation loops.
func (r *AgentReconciler) reconcileTemplateLabel(ctx context.Context, agent *kubeopenv1alpha1.Agent) error {
	if agent.Labels == nil {
		agent.Labels = make(map[string]string)
	}

	var desiredValue string
	if agent.Spec.TemplateRef != nil {
		desiredValue = agent.Spec.TemplateRef.Name
	}

	currentValue := agent.Labels[LabelAgentTemplate]
	if desiredValue == currentValue {
		return nil
	}

	patch := client.MergeFrom(agent.DeepCopy())

	if desiredValue == "" {
		delete(agent.Labels, LabelAgentTemplate)
	} else {
		agent.Labels[LabelAgentTemplate] = desiredValue
	}

	if err := r.Patch(ctx, agent, patch); err != nil {
		return fmt.Errorf("failed to patch template label: %w", err)
	}
	return nil
}

// findAgentsForTemplate returns reconcile requests for all Agents referencing
// the given AgentTemplate. Used to re-reconcile Agents when a template changes.
func (r *AgentReconciler) findAgentsForTemplate(ctx context.Context, obj client.Object) []reconcile.Request {
	logger := log.FromContext(ctx)
	tmpl, ok := obj.(*kubeopenv1alpha1.AgentTemplate)
	if !ok {
		return nil
	}

	var agentList kubeopenv1alpha1.AgentList
	if err := r.List(ctx, &agentList,
		client.InNamespace(tmpl.Namespace),
		client.MatchingLabels{LabelAgentTemplate: tmpl.Name},
	); err != nil {
		logger.Error(err, "Failed to list Agents for template", "template", tmpl.Name)
		return nil
	}

	requests := make([]reconcile.Request, len(agentList.Items))
	for i, agent := range agentList.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      agent.Name,
				Namespace: agent.Namespace,
			},
		}
	}
	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.CountActiveTasksFn == nil {
		r.CountActiveTasksFn = r.countActiveTasks
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeopenv1alpha1.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Watches(&kubeopenv1alpha1.AgentTemplate{}, handler.EnqueueRequestsFromMapFunc(r.findAgentsForTemplate)).
		Watches(&kubeopenv1alpha1.Task{}, handler.EnqueueRequestsFromMapFunc(r.findAgentForTask)).
		Complete(r)
}
