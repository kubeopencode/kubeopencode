// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

// ResolveAgentConfigFromTemplate fetches the referenced AgentTemplate (if any)
// and returns the merged agentConfig. This is the shared entry point used by
// both AgentReconciler and TaskReconciler.
func ResolveAgentConfigFromTemplate(ctx context.Context, reader client.Reader, agent *kubeopenv1alpha1.Agent) (agentConfig, error) {
	if agent.Spec.TemplateRef == nil {
		return ResolveAgentConfig(agent), nil
	}

	tmpl := &kubeopenv1alpha1.AgentTemplate{}
	tmplKey := types.NamespacedName{
		Name:      agent.Spec.TemplateRef.Name,
		Namespace: agent.Namespace,
	}
	if err := reader.Get(ctx, tmplKey, tmpl); err != nil {
		return agentConfig{}, fmt.Errorf("agent template %q not found in namespace %q: %w",
			agent.Spec.TemplateRef.Name, agent.Namespace, err)
	}

	merged := MergeAgentWithTemplate(agent, tmpl)
	if merged.workspaceDir == "" {
		return agentConfig{}, fmt.Errorf("agent %q has empty workspaceDir after template merge", agent.Name)
	}
	if merged.serviceAccountName == "" {
		return agentConfig{}, fmt.Errorf("agent %q has empty serviceAccountName after template merge", agent.Name)
	}
	return merged, nil
}

// MergeAgentWithTemplate merges an Agent's spec with its referenced AgentTemplate.
// Agent-level fields take precedence over template values:
//   - Scalar/pointer fields: Agent wins if non-zero/non-nil
//   - List fields (contexts, credentials, imagePullSecrets): Agent replaces template if non-nil
//
// The returned agentConfig has image defaults applied (same as ResolveAgentConfig).
func MergeAgentWithTemplate(agent *kubeopenv1alpha1.Agent, tmpl *kubeopenv1alpha1.AgentTemplate) agentConfig {
	merged := agentConfig{
		agentImage:    defaultString(agent.Spec.AgentImage, defaultString(tmpl.Spec.AgentImage, DefaultAgentImage)),
		executorImage: defaultString(agent.Spec.ExecutorImage, defaultString(tmpl.Spec.ExecutorImage, DefaultExecutorImage)),
		attachImage:   defaultString(agent.Spec.AttachImage, defaultString(tmpl.Spec.AttachImage, DefaultAttachImage)),

		// Agent wins if non-empty; otherwise inherited from template
		workspaceDir:       defaultString(agent.Spec.WorkspaceDir, tmpl.Spec.WorkspaceDir),
		serviceAccountName: defaultString(agent.Spec.ServiceAccountName, tmpl.Spec.ServiceAccountName),

		maxConcurrentTasks: firstNonNilInt32(agent.Spec.MaxConcurrentTasks, tmpl.Spec.MaxConcurrentTasks),
		quota:              firstNonNilQuota(agent.Spec.Quota, tmpl.Spec.Quota),

		command:          firstNonNilSlice(agent.Spec.Command, tmpl.Spec.Command),
		contexts:         firstNonNilContexts(agent.Spec.Contexts, tmpl.Spec.Contexts),
		config:           firstNonNilPtr(agent.Spec.Config, tmpl.Spec.Config),
		credentials:      firstNonNilCreds(agent.Spec.Credentials, tmpl.Spec.Credentials),
		podSpec:          firstNonNilPodSpec(agent.Spec.PodSpec, tmpl.Spec.PodSpec),
		caBundle:         firstNonNilCABundle(agent.Spec.CABundle, tmpl.Spec.CABundle),
		proxy:            firstNonNilProxy(agent.Spec.Proxy, tmpl.Spec.Proxy),
		imagePullSecrets: firstNonNilIPS(agent.Spec.ImagePullSecrets, tmpl.Spec.ImagePullSecrets),
		port:             agent.Spec.Port,
		persistence:      agent.Spec.Persistence,
		suspend:          agent.Spec.Suspend,
		serverReady:      agent.Status.Ready,
	}

	return merged
}

// Merge helpers: return agent value if non-nil/non-empty, else template value.

func firstNonNilSlice(a, b []string) []string {
	if len(a) > 0 {
		return a
	}
	return b
}

func firstNonNilContexts(a, b []kubeopenv1alpha1.ContextItem) []kubeopenv1alpha1.ContextItem {
	if a != nil {
		return a
	}
	return b
}

func firstNonNilPtr(a, b *string) *string {
	if a != nil {
		return a
	}
	return b
}

func firstNonNilCreds(a, b []kubeopenv1alpha1.Credential) []kubeopenv1alpha1.Credential {
	if a != nil {
		return a
	}
	return b
}

func firstNonNilPodSpec(a, b *kubeopenv1alpha1.AgentPodSpec) *kubeopenv1alpha1.AgentPodSpec {
	if a != nil {
		return a
	}
	return b
}

func firstNonNilCABundle(a, b *kubeopenv1alpha1.CABundleConfig) *kubeopenv1alpha1.CABundleConfig {
	if a != nil {
		return a
	}
	return b
}

func firstNonNilProxy(a, b *kubeopenv1alpha1.ProxyConfig) *kubeopenv1alpha1.ProxyConfig {
	if a != nil {
		return a
	}
	return b
}

func firstNonNilIPS(a, b []corev1.LocalObjectReference) []corev1.LocalObjectReference {
	if a != nil {
		return a
	}
	return b
}

func firstNonNilInt32(a, b *int32) *int32 {
	if a != nil {
		return a
	}
	return b
}

func firstNonNilQuota(a, b *kubeopenv1alpha1.QuotaConfig) *kubeopenv1alpha1.QuotaConfig {
	if a != nil {
		return a
	}
	return b
}
