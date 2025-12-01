// Copyright Contributors to the KubeTask project

// Package controller implements Kubernetes controllers for KubeTask resources
package controller

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubetaskv1alpha1 "github.com/xuezhaojun/kubetask/api/v1alpha1"
)

// TaskReconciler reconciles a Task object
type TaskReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubetask.io,resources=tasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubetask.io,resources=tasks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubetask.io,resources=tasks/finalizers,verbs=update
// +kubebuilder:rbac:groups=kubetask.io,resources=workspaceconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *TaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get Task CR
	task := &kubetaskv1alpha1.Task{}
	if err := r.Get(ctx, req.NamespacedName, task); err != nil {
		if errors.IsNotFound(err) {
			// Task deleted, nothing to do
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Task")
		return ctrl.Result{}, err
	}

	// If new, initialize status and create Job
	if task.Status.Phase == "" {
		return r.initializeTask(ctx, task)
	}

	// If completed/failed, skip
	if task.Status.Phase == kubetaskv1alpha1.TaskPhaseSucceeded ||
		task.Status.Phase == kubetaskv1alpha1.TaskPhaseFailed {
		return ctrl.Result{}, nil
	}

	// Update task status from Job status
	if err := r.updateTaskStatusFromJob(ctx, task); err != nil {
		log.Error(err, "unable to update task status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// initializeTask initializes a new Task and creates its Job
func (r *TaskReconciler) initializeTask(ctx context.Context, task *kubetaskv1alpha1.Task) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get WorkspaceConfig
	config, err := r.getWorkspaceConfig(ctx, task.Namespace)
	if err != nil {
		log.Error(err, "unable to get WorkspaceConfig, will use built-in default")
		config = nil
	}

	// Get agent template
	agentTemplate, err := r.getAgentTemplate(ctx, task.Namespace, config)
	if err != nil {
		log.Error(err, "unable to get Job template")
		return ctrl.Result{}, err
	}

	// Generate Job name
	jobName := fmt.Sprintf("%s-job", task.Name)

	// Check if Job already exists
	existingJob := &batchv1.Job{}
	jobKey := types.NamespacedName{Name: jobName, Namespace: task.Namespace}
	if err := r.Get(ctx, jobKey, existingJob); err == nil {
		// Job already exists, update status
		task.Status.JobName = jobName
		task.Status.Phase = kubetaskv1alpha1.TaskPhaseRunning
		now := metav1.Now()
		task.Status.StartTime = &now
		return ctrl.Result{}, r.Status().Update(ctx, task)
	}

	// Create Job from template
	job, err := r.buildJobFromTemplate(ctx, task, jobName, agentTemplate)
	if err != nil {
		log.Error(err, "unable to build Job from template", "job", jobName)
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, job); err != nil {
		log.Error(err, "unable to create Job", "job", jobName)
		return ctrl.Result{}, err
	}

	// Update status
	task.Status.JobName = jobName
	task.Status.Phase = kubetaskv1alpha1.TaskPhaseRunning
	now := metav1.Now()
	task.Status.StartTime = &now

	if err := r.Status().Update(ctx, task); err != nil {
		log.Error(err, "unable to update Task status")
		return ctrl.Result{}, err
	}

	log.Info("initialized Task", "job", jobName)
	return ctrl.Result{}, nil
}

// updateTaskStatusFromJob syncs task status from Job status
func (r *TaskReconciler) updateTaskStatusFromJob(ctx context.Context, task *kubetaskv1alpha1.Task) error {
	log := log.FromContext(ctx)

	if task.Status.JobName == "" {
		return nil
	}

	// Get Job status
	job := &batchv1.Job{}
	jobKey := types.NamespacedName{Name: task.Status.JobName, Namespace: task.Namespace}
	if err := r.Get(ctx, jobKey, job); err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "Job not found", "job", task.Status.JobName)
			return nil
		}
		return err
	}

	// Check Job completion
	if job.Status.Succeeded > 0 {
		task.Status.Phase = kubetaskv1alpha1.TaskPhaseSucceeded
		now := metav1.Now()
		task.Status.CompletionTime = &now
		log.Info("task succeeded", "job", task.Status.JobName)
		return r.Status().Update(ctx, task)
	} else if job.Status.Failed > 0 {
		task.Status.Phase = kubetaskv1alpha1.TaskPhaseFailed
		now := metav1.Now()
		task.Status.CompletionTime = &now
		log.Info("task failed", "job", task.Status.JobName)
		return r.Status().Update(ctx, task)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *TaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubetaskv1alpha1.Task{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

// getWorkspaceConfig retrieves the WorkspaceConfig
func (r *TaskReconciler) getWorkspaceConfig(ctx context.Context, namespace string) (*kubetaskv1alpha1.WorkspaceConfig, error) {
	config := &kubetaskv1alpha1.WorkspaceConfig{}
	configKey := types.NamespacedName{
		Name:      "default",
		Namespace: namespace,
	}

	if err := r.Get(ctx, configKey, config); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return config, nil
}

// getAgentTemplate retrieves the agent Job template from ConfigMap
func (r *TaskReconciler) getAgentTemplate(ctx context.Context, namespace string, config *kubetaskv1alpha1.WorkspaceConfig) (string, error) {
	log := log.FromContext(ctx)

	var configMapName string
	var configMapKey string

	if config != nil && config.Spec.AgentTemplateRef != nil {
		configMapName = config.Spec.AgentTemplateRef.Name
		configMapKey = config.Spec.AgentTemplateRef.Key
		if configMapKey == "" {
			configMapKey = "agent-template.yaml"
		}
	} else {
		configMapName = "kubetask-agent"
		configMapKey = "agent-template.yaml"
	}

	cm := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{
		Name:      configMapName,
		Namespace: namespace,
	}

	if err := r.Get(ctx, cmKey, cm); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Job template ConfigMap not found, using built-in default", "configMap", configMapName)
			return r.getBuiltInJobTemplate(), nil
		}
		return "", err
	}

	templateContent, ok := cm.Data[configMapKey]
	if !ok {
		return "", fmt.Errorf("key %s not found in ConfigMap %s", configMapKey, configMapName)
	}

	return templateContent, nil
}

// getBuiltInJobTemplate returns the built-in default Job template
func (r *TaskReconciler) getBuiltInJobTemplate() string {
	return `apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .JobName }}
  namespace: {{ .Namespace }}
  labels:
    app: kubetask
    kubetask.io/task: {{ .TaskName }}
spec:
  template:
    spec:
      serviceAccountName: kubetask-agent
      containers:
      - name: agent
        image: ghcr.io/stolostron/kubetask-agent:latest
        env:
        - name: TASK_NAME
          value: {{ .TaskName }}
        - name: TASK_NAMESPACE
          value: {{ .Namespace }}
      restartPolicy: Never`
}

// buildJobFromTemplate renders the Job template and creates a Job object
func (r *TaskReconciler) buildJobFromTemplate(
	_ context.Context,
	task *kubetaskv1alpha1.Task,
	jobName string,
	templateStr string,
) (*batchv1.Job, error) {
	// Prepare template variables
	templateVars := map[string]interface{}{
		"JobName":   jobName,
		"Namespace": task.Namespace,
		"TaskName":  task.Name,
	}

	// Try to find repository context for template variables (backward compatibility)
	for _, ctx := range task.Spec.Contexts {
		if ctx.Type == kubetaskv1alpha1.ContextTypeRepository && ctx.Repository != nil {
			templateVars["Org"] = ctx.Repository.Org
			templateVars["Repo"] = ctx.Repository.Repo
			templateVars["Branch"] = ctx.Repository.Branch
			break
		}
	}

	// Parse and render template
	tmpl, err := template.New("job").Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Job template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateVars); err != nil {
		return nil, fmt.Errorf("failed to execute Job template: %w", err)
	}

	// Parse YAML to Job object
	job := &batchv1.Job{}
	if err := yaml.Unmarshal(buf.Bytes(), job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Job YAML: %w", err)
	}

	// Set OwnerReference for automatic cleanup
	job.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: task.APIVersion,
			Kind:       task.Kind,
			Name:       task.Name,
			UID:        task.UID,
			Controller: boolPtr(true),
		},
	}

	return job, nil
}
