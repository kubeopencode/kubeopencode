// Copyright Contributors to the KubeTask project

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubetaskv1alpha1 "github.com/kubetask/kubetask/api/v1alpha1"
	"github.com/kubetask/kubetask/internal/webhook"
)

const (
	// WebhookTriggerLabelKey is the label key used to identify Tasks created by a WebhookTrigger
	WebhookTriggerLabelKey = "kubetask.io/webhook-trigger"
)

// WebhookTriggerReconciler reconciles a WebhookTrigger object
type WebhookTriggerReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	WebhookServer *webhook.Server
}

// +kubebuilder:rbac:groups=kubetask.io,resources=webhooktriggers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubetask.io,resources=webhooktriggers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubetask.io,resources=webhooktriggers/finalizers,verbs=update
// +kubebuilder:rbac:groups=kubetask.io,resources=tasks,verbs=get;list;watch

// Reconcile handles WebhookTrigger events
func (r *WebhookTriggerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the WebhookTrigger
	trigger := &kubetaskv1alpha1.WebhookTrigger{}
	if err := r.Get(ctx, req.NamespacedName, trigger); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Unable to fetch WebhookTrigger")
			return ctrl.Result{}, err
		}
		// WebhookTrigger was deleted, unregister from webhook server
		if r.WebhookServer != nil {
			r.WebhookServer.UnregisterTrigger(req.Namespace, req.Name)
		}
		return ctrl.Result{}, nil
	}

	// Register/update the trigger in the webhook server
	if r.WebhookServer != nil {
		r.WebhookServer.RegisterTrigger(trigger)
	}

	// Update the webhook URL in status
	webhookURL := fmt.Sprintf("/webhooks/%s/%s", trigger.Namespace, trigger.Name)
	if trigger.Status.WebhookURL != webhookURL {
		trigger.Status.WebhookURL = webhookURL
	}

	// Update active tasks list by checking which tasks are still running
	activeTasks, err := r.getActiveTasks(ctx, trigger)
	if err != nil {
		logger.Error(err, "Failed to get active tasks")
		// Don't fail the reconcile, just log the error
	} else {
		trigger.Status.ActiveTasks = activeTasks
	}

	// Set Ready condition
	meta.SetStatusCondition(&trigger.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "WebhookRegistered",
		Message:            fmt.Sprintf("Webhook endpoint registered at %s", webhookURL),
		ObservedGeneration: trigger.Generation,
	})

	trigger.Status.ObservedGeneration = trigger.Generation

	// Update status
	if err := r.Status().Update(ctx, trigger); err != nil {
		logger.Error(err, "Failed to update WebhookTrigger status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// getActiveTasks returns the list of tasks that are still running for this trigger.
func (r *WebhookTriggerReconciler) getActiveTasks(ctx context.Context, trigger *kubetaskv1alpha1.WebhookTrigger) ([]string, error) {
	// List tasks with the webhook trigger label
	taskList := &kubetaskv1alpha1.TaskList{}
	if err := r.List(ctx, taskList,
		client.InNamespace(trigger.Namespace),
		client.MatchingLabels{WebhookTriggerLabelKey: trigger.Name},
	); err != nil {
		return nil, err
	}

	var activeTasks []string
	for _, task := range taskList.Items {
		// Include tasks that are still active (running, pending, queued, or newly created with empty phase)
		// Exclude only completed or failed tasks
		if task.Status.Phase != kubetaskv1alpha1.TaskPhaseCompleted &&
			task.Status.Phase != kubetaskv1alpha1.TaskPhaseFailed {
			activeTasks = append(activeTasks, task.Name)
		}
	}

	return activeTasks, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WebhookTriggerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubetaskv1alpha1.WebhookTrigger{}).
		// Watch Tasks to update ActiveTasks in trigger status
		Owns(&kubetaskv1alpha1.Task{}).
		Complete(r)
}
