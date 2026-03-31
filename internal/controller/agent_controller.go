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

	// AgentConditionServerHealthy indicates whether the server is responding to health checks.
	// In the Pod-based approach, this is based on Deployment readiness rather than HTTP health checks.
	AgentConditionServerHealthy = "ServerHealthy"

	// AgentConditionSuspended indicates whether the Agent server is intentionally suspended.
	AgentConditionSuspended = "Suspended"

	// DefaultServerReconcileInterval is how often to reconcile Server-mode Agents.
	DefaultServerReconcileInterval = 30 * time.Second
)

// AgentReconciler reconciles Agent resources.
// For Server-mode Agents, it manages the Deployment and Service.
type AgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubeopencode.io,resources=agents,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=kubeopencode.io,resources=agents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;delete

// Reconcile handles Agent reconciliation.
// For Server-mode Agents, it ensures the Deployment and Service exist and are up-to-date.
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
	// Must happen before IsServerMode check because template may provide serverConfig.
	agentCfg, err := r.resolveAgentConfig(ctx, &agent)
	if err != nil {
		logger.Error(err, "Failed to resolve agent config")
		return ctrl.Result{}, err
	}

	// Only handle Server-mode Agents (check merged config, not just agent spec)
	if agentCfg.serverConfig == nil {
		if err := r.cleanupServerResources(ctx, &agent); err != nil {
			logger.Error(err, "Failed to cleanup server resources")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	logger.Info("Reconciling Server-mode Agent", "agent", agent.Name)
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

	// Reconcile persistence PVCs if configured
	if err := r.reconcilePVC(ctx, &agent, BuildServerSessionPVC, "session"); err != nil {
		logger.Error(err, "Failed to reconcile session PVC")
		return ctrl.Result{}, err
	}
	if err := r.reconcilePVC(ctx, &agent, BuildServerWorkspacePVC, "workspace"); err != nil {
		logger.Error(err, "Failed to reconcile workspace PVC")
		return ctrl.Result{}, err
	}

	// Reconcile the Deployment (with context support)
	if err := r.reconcileDeployment(ctx, &agent, agentCfg, sysCfg, contextConfigMap, fileMounts, dirMounts, gitMounts); err != nil {
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

	// Requeue periodically to check server health
	return ctrl.Result{RequeueAfter: DefaultServerReconcileInterval}, nil
}

// reconcileDeployment ensures the Deployment exists and is up-to-date.
func (r *AgentReconciler) reconcileDeployment(ctx context.Context, agent *kubeopenv1alpha1.Agent, agentCfg agentConfig, sysCfg systemConfig, contextConfigMap *corev1.ConfigMap, fileMounts []fileMount, dirMounts []dirMount, gitMounts []gitMount) error {
	logger := log.FromContext(ctx)

	desired := BuildServerDeployment(agent, agentCfg, sysCfg, contextConfigMap, fileMounts, dirMounts, gitMounts)
	if desired == nil {
		return nil
	}

	// Scale to 0 replicas when suspended
	if agent.Spec.ServerConfig.Suspend {
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
			logger.Info("Creating Deployment for Server-mode Agent", "deployment", desired.Name)
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
	if desired == nil {
		return nil
	}

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
			logger.Info("Creating Service for Server-mode Agent", "service", desired.Name)
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

// updateAgentStatus updates the Agent's status with server information.
// Health is determined by Deployment readiness (liveness/readiness probes on the Deployment
// already check the server's /session/status endpoint).
func (r *AgentReconciler) updateAgentStatus(ctx context.Context, agent *kubeopenv1alpha1.Agent) error {
	deploymentName := ServerDeploymentName(agent.Name)
	if agent.Status.ServerStatus == nil {
		agent.Status.ServerStatus = &kubeopenv1alpha1.ServerStatus{}
	}
	agent.Status.ServerStatus.DeploymentName = deploymentName
	agent.Status.ServerStatus.ServiceName = ServerServiceName(agent.Name)
	agent.Status.ServerStatus.URL = ServerURL(agent.Name, agent.Namespace, GetServerPort(agent))

	// Handle suspended state
	if agent.Spec.ServerConfig.Suspend {
		agent.Status.ServerStatus.Suspended = true
		agent.Status.ServerStatus.Ready = false
		setAgentCondition(agent, AgentConditionSuspended, metav1.ConditionTrue, "UserRequested", "Agent is suspended")
		setAgentCondition(agent, AgentConditionServerReady, metav1.ConditionFalse, "Suspended", "Server is suspended")
	} else {
		agent.Status.ServerStatus.Suspended = false
		setAgentCondition(agent, AgentConditionSuspended, metav1.ConditionFalse, "Active", "Agent is active")

		var deployment appsv1.Deployment
		err := r.Get(ctx, client.ObjectKey{Namespace: agent.Namespace, Name: deploymentName}, &deployment)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to get Deployment: %w", err)
			}
			agent.Status.ServerStatus.Ready = false
		} else {
			agent.Status.ServerStatus.Ready = deployment.Status.ReadyReplicas > 0

			if agent.Status.ServerStatus.Ready {
				setAgentCondition(agent, AgentConditionServerHealthy, metav1.ConditionTrue, "DeploymentHealthy", "Server deployment is ready")
			}
		}

		// Set ServerReady condition
		if agent.Status.ServerStatus.Ready {
			setAgentCondition(agent, AgentConditionServerReady, metav1.ConditionTrue, "DeploymentReady", "Server deployment is ready")
		} else {
			setAgentCondition(agent, AgentConditionServerReady, metav1.ConditionFalse, "DeploymentNotReady", "Server deployment is not ready")
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
	if len(cfg.contexts) == 0 {
		return nil, nil, nil, nil, nil
	}

	// Resolve all context items
	resolved, dirMounts, gitMounts, err := processContextItems(r.Client, ctx, cfg.contexts, agent.Namespace, cfg.workspaceDir)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to resolve Agent contexts: %w", err)
	}

	// Build ConfigMap data from resolved contexts
	configMapData, fileMounts := buildContextConfigMapData(resolved, cfg.workspaceDir)

	// Add OpenCode config to ConfigMap if provided (same as Pod mode)
	if cfg.config != nil && *cfg.config != "" {
		configMapKey := sanitizeConfigMapKey(OpenCodeConfigPath)
		configMapData[configMapKey] = *cfg.config
		fileMounts = append(fileMounts, fileMount{filePath: OpenCodeConfigPath})
	}

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
			logger.Info("Creating context ConfigMap for Server-mode Agent", "configmap", desired.Name)
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
			logger.Info("Creating PVC for Server-mode Agent", "pvc", desired.Name, "type", label)
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

// cleanupServerResources removes Deployment and Service if they exist.
// This is called when an Agent is changed from Server-mode to Pod-mode.
func (r *AgentReconciler) cleanupServerResources(ctx context.Context, agent *kubeopenv1alpha1.Agent) error {
	logger := log.FromContext(ctx)

	// Delete Deployment if exists
	deploymentName := ServerDeploymentName(agent.Name)
	var deployment appsv1.Deployment
	if err := r.Get(ctx, client.ObjectKey{Namespace: agent.Namespace, Name: deploymentName}, &deployment); err == nil {
		logger.Info("Cleaning up stale Deployment", "deployment", deploymentName)
		if err := r.Delete(ctx, &deployment); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete Deployment: %w", err)
		}
	}

	// Delete Service if exists
	serviceName := ServerServiceName(agent.Name)
	var service corev1.Service
	if err := r.Get(ctx, client.ObjectKey{Namespace: agent.Namespace, Name: serviceName}, &service); err == nil {
		logger.Info("Cleaning up stale Service", "service", serviceName)
		if err := r.Delete(ctx, &service); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete Service: %w", err)
		}
	}

	// Delete context ConfigMap if exists
	contextCMName := ServerContextConfigMapName(agent.Name)
	var contextCM corev1.ConfigMap
	if err := r.Get(ctx, client.ObjectKey{Namespace: agent.Namespace, Name: contextCMName}, &contextCM); err == nil {
		logger.Info("Cleaning up stale context ConfigMap", "configmap", contextCMName)
		if err := r.Delete(ctx, &contextCM); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete context ConfigMap: %w", err)
		}
	}

	// Delete session PVC if exists
	sessionPVCName := ServerSessionPVCName(agent.Name)
	var sessionPVC corev1.PersistentVolumeClaim
	if err := r.Get(ctx, client.ObjectKey{Namespace: agent.Namespace, Name: sessionPVCName}, &sessionPVC); err == nil {
		logger.Info("Cleaning up stale session PVC", "pvc", sessionPVCName)
		if err := r.Delete(ctx, &sessionPVC); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete session PVC: %w", err)
		}
	}

	// Delete workspace PVC if exists
	workspacePVCName := ServerWorkspacePVCName(agent.Name)
	var workspacePVC corev1.PersistentVolumeClaim
	if err := r.Get(ctx, client.ObjectKey{Namespace: agent.Namespace, Name: workspacePVCName}, &workspacePVC); err == nil {
		logger.Info("Cleaning up stale workspace PVC", "pvc", workspacePVCName)
		if err := r.Delete(ctx, &workspacePVC); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete workspace PVC: %w", err)
		}
	}

	// Clear server status if present
	if agent.Status.ServerStatus != nil {
		agent.Status.ServerStatus = nil
		if err := r.Status().Update(ctx, agent); err != nil {
			return fmt.Errorf("failed to clear server status: %w", err)
		}
	}

	return nil
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
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeopenv1alpha1.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Watches(&kubeopenv1alpha1.AgentTemplate{}, handler.EnqueueRequestsFromMapFunc(r.findAgentsForTemplate)).
		Complete(r)
}
