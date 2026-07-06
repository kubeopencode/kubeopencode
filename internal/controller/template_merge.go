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
	var cfg agentConfig
	if agent.Spec.TemplateRef == nil {
		cfg = ResolveAgentConfig(agent)
	} else {
		tmpl := &kubeopenv1alpha1.AgentTemplate{}
		tmplKey := types.NamespacedName{
			Name:      agent.Spec.TemplateRef.Name,
			Namespace: agent.Namespace,
		}
		if err := reader.Get(ctx, tmplKey, tmpl); err != nil {
			return agentConfig{}, fmt.Errorf("agent template %q not found in namespace %q: %w",
				agent.Spec.TemplateRef.Name, agent.Namespace, err)
		}

		cfg = MergeAgentWithTemplate(agent, tmpl)
		if cfg.workspaceDir == "" {
			return agentConfig{}, fmt.Errorf("agent %q has empty workspaceDir after template merge", agent.Name)
		}
		if cfg.serviceAccountName == "" {
			return agentConfig{}, fmt.Errorf("agent %q has empty serviceAccountName after template merge", agent.Name)
		}
	}

	// Resolve configRef into inline config if set (configRef → config).
	// This happens after merge so that template-provided configRef is also resolved.
	if err := resolveAgentConfigRef(ctx, reader, agent.Namespace, &cfg); err != nil {
		return agentConfig{}, err
	}

	return cfg, nil
}

// MergeAgentWithTemplate merges an Agent's spec with its referenced AgentTemplate.
// Agent-level fields take precedence over template values:
//   - Scalar/pointer fields: Agent wins if non-zero/non-nil
//   - List fields (contexts, credentials, imagePullSecrets): Agent replaces template if non-nil
//   - podSpec: deep-merged field by field (see mergePodSpec). List/map fields are
//     combined (template values first, Agent values appended and winning on name/key
//     conflict); scalar pointer fields use Agent if non-nil, else template. This lets
//     a template define common volumes/env/labels while an Agent extends them with
//     instance-specific values. See issue #282.
//
// The returned agentConfig has image defaults applied (same as ResolveAgentConfig).
func MergeAgentWithTemplate(agent *kubeopenv1alpha1.Agent, tmpl *kubeopenv1alpha1.AgentTemplate) agentConfig {
	// podSpec is deep-merged so a template can contribute common volumes/env/labels
	// and the Agent can extend (not replace) them. When only one side defines a
	// podSpec, it is used as-is. See issue #282.
	mergedPodSpec := mergePodSpec(agent.Spec.PodSpec, tmpl.Spec.PodSpec)

	merged := agentConfig{
		agentImage:    defaultString(agent.Spec.AgentImage, defaultString(tmpl.Spec.AgentImage, DefaultAgentImage)),
		executorImage: defaultString(agent.Spec.ExecutorImage, defaultString(tmpl.Spec.ExecutorImage, DefaultExecutorImage)),
		attachImage:   defaultString(agent.Spec.AttachImage, defaultString(tmpl.Spec.AttachImage, DefaultAttachImage)),

		// Agent wins if non-empty; otherwise inherited from template
		workspaceDir:       defaultString(agent.Spec.WorkspaceDir, tmpl.Spec.WorkspaceDir),
		serviceAccountName: defaultString(agent.Spec.ServiceAccountName, tmpl.Spec.ServiceAccountName),

		maxConcurrentTasks: firstNonNilPtr(agent.Spec.MaxConcurrentTasks, tmpl.Spec.MaxConcurrentTasks),
		quota:              firstNonNilPtr(agent.Spec.Quota, tmpl.Spec.Quota),

		command:          firstNonEmptyStringSlice(agent.Spec.Command, tmpl.Spec.Command),
		contexts:         firstNonNilSlice(agent.Spec.Contexts, tmpl.Spec.Contexts),
		skills:           firstNonNilSlice(agent.Spec.Skills, tmpl.Spec.Skills),
		plugins:          firstNonNilSlice(agent.Spec.Plugins, tmpl.Spec.Plugins),
		config:           firstNonNilPtr(agent.Spec.Config, tmpl.Spec.Config),
		configRef:        firstNonNilPtr(agent.Spec.ConfigRef, tmpl.Spec.ConfigRef),
		credentials:      firstNonNilSlice(agent.Spec.Credentials, tmpl.Spec.Credentials),
		podSpec:          mergedPodSpec,
		caBundle:         firstNonNilPtr(agent.Spec.CABundle, tmpl.Spec.CABundle),
		proxy:            firstNonNilPtr(agent.Spec.Proxy, tmpl.Spec.Proxy),
		imagePullSecrets: firstNonNilSlice(agent.Spec.ImagePullSecrets, tmpl.Spec.ImagePullSecrets),
		extraPorts:       firstNonNilSlice(agent.Spec.ExtraPorts, tmpl.Spec.ExtraPorts),
		port:             agent.Spec.Port,
		persistence:      agent.Spec.Persistence,
		suspend:          agent.Spec.Suspend,
		serverReady:      agent.Status.Ready,
	}

	// Populate extraEnv and systemContainers from the merged podSpec.
	if mergedPodSpec != nil {
		merged.extraEnv = mergedPodSpec.ExtraEnv
		merged.systemContainers = mergedPodSpec.SystemContainers
	}

	return merged
}

// Merge helpers: return agent value if non-nil/non-empty, else template value.

func firstNonEmptyStringSlice(a, b []string) []string {
	if len(a) > 0 {
		return a
	}
	return b
}

func firstNonNilSlice[T any](a, b []T) []T {
	if a != nil {
		return a
	}
	return b
}

func firstNonNilPtr[T any](a, b *T) *T {
	if a != nil {
		return a
	}
	return b
}

// mergePodSpec deep-merges an Agent's podSpec with its template's podSpec.
//
// Merge strategy:
//   - Labels, Annotations: maps are combined; Agent wins on key conflict.
//   - Scheduling: NodeSelector maps are combined (Agent wins); Tolerations are
//     appended (template first, then Agent); Affinity uses Agent if non-nil
//     else template (deep-merging affinity is complex and rarely needed).
//   - ExtraVolumes, ExtraVolumeMounts, ExtraEnv: slices are combined, deduped by
//     Name with Agent winning on collision. This avoids generating pods that
//     Kubernetes would reject for duplicate volume/env/mount names.
//   - SystemContainers: each per-container-type override is deep-merged (ExtraEnv
//     and ExtraVolumeMounts combined with Agent winning on Name collision).
//   - Scalar pointer fields (RuntimeClassName, Resources, SecurityContext,
//     PodSecurityContext, Lifecycle): Agent wins if non-nil, else template.
//
// When only one side is non-nil, it is returned as-is (no allocation). When both
// are nil, nil is returned. See issue #282.
func mergePodSpec(agentPS, tmplPS *kubeopenv1alpha1.AgentPodSpec) *kubeopenv1alpha1.AgentPodSpec {
	if agentPS == nil {
		return tmplPS
	}
	if tmplPS == nil {
		return agentPS
	}
	return &kubeopenv1alpha1.AgentPodSpec{
		Labels:             mergeStringMap(agentPS.Labels, tmplPS.Labels),
		Annotations:        mergeStringMap(agentPS.Annotations, tmplPS.Annotations),
		Scheduling:         mergeScheduling(agentPS.Scheduling, tmplPS.Scheduling),
		RuntimeClassName:   firstNonNilPtr(agentPS.RuntimeClassName, tmplPS.RuntimeClassName),
		Resources:          firstNonNilPtr(agentPS.Resources, tmplPS.Resources),
		SecurityContext:    firstNonNilPtr(agentPS.SecurityContext, tmplPS.SecurityContext),
		PodSecurityContext: firstNonNilPtr(agentPS.PodSecurityContext, tmplPS.PodSecurityContext),
		Lifecycle:          firstNonNilPtr(agentPS.Lifecycle, tmplPS.Lifecycle),
		ExtraVolumes:       mergeVolumes(agentPS.ExtraVolumes, tmplPS.ExtraVolumes),
		ExtraVolumeMounts:  mergeVolumeMounts(agentPS.ExtraVolumeMounts, tmplPS.ExtraVolumeMounts),
		ExtraEnv:           mergeEnvVars(agentPS.ExtraEnv, tmplPS.ExtraEnv),
		SystemContainers:   mergeSystemContainers(agentPS.SystemContainers, tmplPS.SystemContainers),
	}
}

// mergeStringMap combines two string maps. Agent (a) wins on key conflict.
// Returns nil if both are empty.
func mergeStringMap(a, b map[string]string) map[string]string {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	out := make(map[string]string, len(a)+len(b))
	for k, v := range b {
		out[k] = v
	}
	for k, v := range a {
		out[k] = v
	}
	return out
}

// mergeNamedSlice combines two slices of named Kubernetes objects. Items from b
// (template) come first, then items from a (agent). On Name collision, the agent
// (a) value replaces the template (b) value. Items with an empty Name are always
// appended. Returns nil if both are empty.
func mergeNamedSlice[T any](a, b []T, nameOf func(T) string) []T {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	out := make([]T, 0, len(a)+len(b))
	idxByName := make(map[string]int, len(a)+len(b))
	add := func(item T, agentWins bool) {
		n := nameOf(item)
		if n == "" {
			out = append(out, item)
			return
		}
		if idx, ok := idxByName[n]; ok && agentWins {
			out[idx] = item // agent wins over earlier (template) entry
		} else if !ok {
			out = append(out, item)
			idxByName[n] = len(out) - 1
		}
		// If collision and !agentWins (a duplicate within template), keep the first.
	}
	for _, item := range b {
		add(item, false)
	}
	for _, item := range a {
		add(item, true)
	}
	return out
}

func mergeVolumes(a, b []corev1.Volume) []corev1.Volume {
	return mergeNamedSlice(a, b, func(v corev1.Volume) string { return v.Name })
}

func mergeVolumeMounts(a, b []corev1.VolumeMount) []corev1.VolumeMount {
	return mergeNamedSlice(a, b, func(vm corev1.VolumeMount) string { return vm.Name })
}

func mergeEnvVars(a, b []corev1.EnvVar) []corev1.EnvVar {
	return mergeNamedSlice(a, b, func(e corev1.EnvVar) string { return e.Name })
}

// mergeScheduling deep-merges two PodScheduling pointers. Agent (a) wins for
// scalar pointer fields; NodeSelector maps are combined (Agent wins); Tolerations
// are appended (template first, then Agent).
func mergeScheduling(a, b *kubeopenv1alpha1.PodScheduling) *kubeopenv1alpha1.PodScheduling {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return &kubeopenv1alpha1.PodScheduling{
		NodeSelector: mergeStringMap(a.NodeSelector, b.NodeSelector),
		Tolerations:  mergeTolerations(a.Tolerations, b.Tolerations),
		Affinity:     firstNonNilPtr(a.Affinity, b.Affinity),
	}
}

// mergeTolerations appends agent tolerations after template tolerations.
// Duplicate tolerations are harmless to Kubernetes, so no dedup is performed.
func mergeTolerations(a, b []corev1.Toleration) []corev1.Toleration {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	out := make([]corev1.Toleration, 0, len(a)+len(b))
	out = append(out, b...)
	out = append(out, a...)
	return out
}

// mergeSystemContainers deep-merges per-container-type overrides. Each
// container-type override is merged independently; Agent (a) wins for nil-or-not.
func mergeSystemContainers(a, b *kubeopenv1alpha1.SystemContainerOverrides) *kubeopenv1alpha1.SystemContainerOverrides {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return &kubeopenv1alpha1.SystemContainerOverrides{
		OpenCodeInit: mergeInitContainerOverrides(a.OpenCodeInit, b.OpenCodeInit),
		ContextInit:  mergeInitContainerOverrides(a.ContextInit, b.ContextInit),
		GitInit:      mergeInitContainerOverrides(a.GitInit, b.GitInit),
		GitSync:      mergeInitContainerOverrides(a.GitSync, b.GitSync),
		PluginInit:   mergeInitContainerOverrides(a.PluginInit, b.PluginInit),
	}
}

// mergeInitContainerOverrides combines ExtraEnv and ExtraVolumeMounts from both
// overrides, deduped by Name with Agent (a) winning on collision.
func mergeInitContainerOverrides(a, b *kubeopenv1alpha1.InitContainerOverrides) *kubeopenv1alpha1.InitContainerOverrides {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return &kubeopenv1alpha1.InitContainerOverrides{
		ExtraEnv:          mergeEnvVars(a.ExtraEnv, b.ExtraEnv),
		ExtraVolumeMounts: mergeVolumeMounts(a.ExtraVolumeMounts, b.ExtraVolumeMounts),
	}
}
