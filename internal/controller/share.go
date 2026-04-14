// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const (
	// ShareSecretSuffix is appended to the Agent name to form the share Secret name.
	ShareSecretSuffix = "-share"

	// ShareTokenKey is the key in the share Secret that stores the token.
	ShareTokenKey = "token"

	// ShareTokenLength is the number of random bytes used to generate the token.
	// 32 bytes = 256 bits of entropy, encoded as base64url (~43 characters).
	ShareTokenLength = 32

	// LabelShareToken is the label applied to share Secrets for efficient lookup.
	LabelShareToken = "kubeopencode.io/share-token" //nolint:gosec // Not a credential, it's a label key

	// AnnotationShareAgentName stores the Agent name on the share Secret.
	AnnotationShareAgentName = "kubeopencode.io/agent-name"

	// AnnotationShareAgentNamespace stores the Agent namespace on the share Secret.
	AnnotationShareAgentNamespace = "kubeopencode.io/agent-namespace"

	// AgentConditionShareReady indicates whether the share link is active.
	AgentConditionShareReady = "ShareReady"
)

// ShareSecretName returns the name of the share Secret for a given Agent.
func ShareSecretName(agentName string) string {
	return agentName + ShareSecretSuffix
}

// reconcileShare manages the share token lifecycle.
// When spec.share.enabled is true, it ensures a share Secret exists with a valid token.
// When spec.share is nil or enabled is false, it cleans up the Secret.
// Returns a requeue duration if the share link has an expiry time.
func (r *AgentReconciler) reconcileShare(ctx context.Context, agent *kubeopenv1alpha1.Agent) (time.Duration, error) {
	logger := log.FromContext(ctx)

	shareEnabled := agent.Spec.Share != nil && agent.Spec.Share.Enabled

	if !shareEnabled {
		return 0, r.cleanupShare(ctx, agent)
	}

	// Share is enabled — ensure Secret exists
	secretName := ShareSecretName(agent.Name)
	var existing corev1.Secret
	err := r.Get(ctx, client.ObjectKey{Namespace: agent.Namespace, Name: secretName}, &existing)

	if err != nil {
		if !apierrors.IsNotFound(err) {
			return 0, fmt.Errorf("failed to get share Secret: %w", err)
		}

		// Generate new token and create Secret
		token, genErr := generateShareToken()
		if genErr != nil {
			return 0, fmt.Errorf("failed to generate share token: %w", genErr)
		}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: agent.Namespace,
				Labels: map[string]string{
					LabelShareToken: "true",
				},
				Annotations: map[string]string{
					AnnotationShareAgentName:      agent.Name,
					AnnotationShareAgentNamespace: agent.Namespace,
				},
			},
			Data: map[string][]byte{
				ShareTokenKey: []byte(token),
			},
		}

		// Set owner reference for automatic cleanup
		if err := controllerutil.SetControllerReference(agent, secret, r.Scheme); err != nil {
			return 0, fmt.Errorf("failed to set owner reference on share Secret: %w", err)
		}

		logger.Info("Creating share Secret for Agent", "secret", secretName)
		if err := r.Create(ctx, secret); err != nil {
			return 0, fmt.Errorf("failed to create share Secret: %w", err)
		}
	}

	// Check expiry
	expired := false
	var requeueAfter time.Duration
	if agent.Spec.Share.ExpiresAt != nil {
		remaining := time.Until(agent.Spec.Share.ExpiresAt.Time)
		if remaining <= 0 {
			expired = true
		} else {
			requeueAfter = remaining
		}
	}

	// Update share status
	agent.Status.Share = &kubeopenv1alpha1.ShareStatus{
		SecretName: secretName,
		Active:     !expired && agent.Status.Ready,
	}

	// Set condition
	switch {
	case expired:
		setAgentCondition(agent, AgentConditionShareReady, metav1.ConditionFalse,
			"Expired", fmt.Sprintf("Share link expired at %s", agent.Spec.Share.ExpiresAt.Format(time.RFC3339)))
	case !agent.Status.Ready:
		setAgentCondition(agent, AgentConditionShareReady, metav1.ConditionFalse,
			"AgentNotReady", "Share link is inactive because the Agent is not ready")
	default:
		setAgentCondition(agent, AgentConditionShareReady, metav1.ConditionTrue,
			"Active", "Share link is active")
	}

	return requeueAfter, nil
}

// cleanupShare removes the share Secret and clears status/conditions.
func (r *AgentReconciler) cleanupShare(ctx context.Context, agent *kubeopenv1alpha1.Agent) error {
	logger := log.FromContext(ctx)

	secretName := ShareSecretName(agent.Name)
	var existing corev1.Secret
	if err := r.Get(ctx, client.ObjectKey{Namespace: agent.Namespace, Name: secretName}, &existing); err == nil {
		logger.Info("Deleting share Secret for Agent", "secret", secretName)
		if err := r.Delete(ctx, &existing); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete share Secret: %w", err)
		}
	}

	// Clear share status
	agent.Status.Share = nil

	// Remove share condition if it exists
	meta.RemoveStatusCondition(&agent.Status.Conditions, AgentConditionShareReady)

	return nil
}

// generateShareToken generates a cryptographically random token encoded as base64url.
func generateShareToken() (string, error) {
	b := make([]byte, ShareTokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
