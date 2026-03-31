// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

// AgentTemplateReconciler reconciles AgentTemplate resources.
// It is a lightweight controller that updates ObservedGeneration and conditions.
// Agent re-reconciliation on template changes is handled by AgentReconciler's
// Watches + findAgentsForTemplate.
type AgentTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=kubeopencode.io,resources=agenttemplates,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=kubeopencode.io,resources=agenttemplates/status,verbs=get;update;patch

// Reconcile handles AgentTemplate reconciliation.
func (r *AgentTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var tmpl kubeopenv1alpha1.AgentTemplate
	if err := r.Get(ctx, req.NamespacedName, &tmpl); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get AgentTemplate")
		return ctrl.Result{}, err
	}

	// Update observed generation
	if tmpl.Status.ObservedGeneration != tmpl.Generation {
		tmpl.Status.ObservedGeneration = tmpl.Generation

		// Set Ready condition using standard meta.SetStatusCondition
		meta.SetStatusCondition(&tmpl.Status.Conditions, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			ObservedGeneration: tmpl.Generation,
			Reason:             "Valid",
			Message:            "AgentTemplate is valid and ready for use",
		})

		if err := r.Status().Update(ctx, &tmpl); err != nil {
			logger.Error(err, "Failed to update AgentTemplate status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeopenv1alpha1.AgentTemplate{}).
		Complete(r)
}
