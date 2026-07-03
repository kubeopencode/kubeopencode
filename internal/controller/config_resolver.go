// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

// DefaultOpenCodeConfigMapKey is the default ConfigMap key for OpenCode config JSON.
const DefaultOpenCodeConfigMapKey = "opencode.json"

// DefaultOpenCodeSecretKey is the default Secret key for OpenCode config JSON.
const DefaultOpenCodeSecretKey = "opencode.json"

// resolveConfigMapRef reads a ConfigMap referenced by OpenCodeConfigMapReference and
// returns the config content as a RawExtension. If ref is nil, returns nil
// without error (caller should handle nil as "no config").
// The ConfigMap must exist in the given namespace and contain the specified key
// (or the default key if Key is empty). The value must be valid JSON.
func resolveConfigMapRef(ctx context.Context, reader client.Reader, namespace string, ref *kubeopenv1alpha1.OpenCodeConfigMapReference) (*runtime.RawExtension, error) {
	if ref == nil {
		return nil, nil
	}

	cm := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}
	if err := reader.Get(ctx, cmKey, cm); err != nil {
		return nil, fmt.Errorf("configmap %q not found in namespace %q: %w", ref.Name, namespace, err)
	}

	key := ref.Key
	if key == "" {
		key = DefaultOpenCodeConfigMapKey
	}

	raw, ok := cm.Data[key]
	if !ok {
		return nil, fmt.Errorf("configmap %q does not contain key %q", ref.Name, key)
	}

	// Validate that the content is valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("configmap %q key %q contains invalid JSON: %w", ref.Name, key, err)
	}

	return &runtime.RawExtension{Raw: []byte(raw)}, nil
}

// resolveSecretRef reads a Secret referenced by OpenCodeConfigSecretReference and
// returns the config content as a RawExtension. If ref is nil, returns nil
// without error. The Secret must exist in the given namespace and contain the
// specified key (or the default key if Key is empty). The value must be valid JSON.
func resolveSecretRef(ctx context.Context, reader client.Reader, namespace string, ref *kubeopenv1alpha1.OpenCodeConfigSecretReference) (*runtime.RawExtension, error) {
	if ref == nil {
		return nil, nil
	}

	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}
	if err := reader.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("secret %q not found in namespace %q: %w", ref.Name, namespace, err)
	}

	key := ref.Key
	if key == "" {
		key = DefaultOpenCodeSecretKey
	}

	rawBytes, ok := secret.Data[key]
	if !ok {
		return nil, fmt.Errorf("secret %q does not contain key %q", ref.Name, key)
	}

	// Validate that the content is valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(rawBytes, &parsed); err != nil {
		return nil, fmt.Errorf("secret %q key %q contains invalid JSON: %w", ref.Name, key, err)
	}

	return &runtime.RawExtension{Raw: rawBytes}, nil
}

// validateConfigMutualExclusion checks that config and configRef are not
// both set on the same agentConfig. Since the config field is Schemaless
// (x-kubernetes-preserve-unknown-fields), CEL XValidation cannot reference it,
// so this check is done at reconcile time instead.
func validateConfigMutualExclusion(cfg *agentConfig) error {
	if cfg.configRef != nil && !configIsEmpty(cfg.config) {
		return fmt.Errorf("config and configRef are mutually exclusive: both are set")
	}
	return nil
}

// resolveAgentConfigRef resolves the configRef on an agentConfig into
// the config field. If configRef is set and config is empty, the referenced
// ConfigMap or Secret is read and config is populated. If both are set, an
// error is returned (enforced at reconcile time since CEL XValidation cannot
// reference schemaless fields). If configRef is nil, no action is taken.
func resolveAgentConfigRef(ctx context.Context, reader client.Reader, namespace string, cfg *agentConfig) error {
	if cfg.configRef == nil {
		return nil
	}
	// Mutual exclusivity check (enforced here since config is Schemaless
	// and cannot be referenced by CEL XValidation rules).
	if !configIsEmpty(cfg.config) {
		return fmt.Errorf("config and configRef are mutually exclusive: both are set")
	}

	resolved, err := resolveConfigSource(ctx, reader, namespace, cfg.configRef)
	if err != nil {
		return fmt.Errorf("failed to resolve configRef: %w", err)
	}
	cfg.config = resolved
	return nil
}

// resolveConfigSource reads the ConfigMap or Secret referenced by OpenCodeConfigSource
// and returns the config content as a RawExtension.
func resolveConfigSource(ctx context.Context, reader client.Reader, namespace string, src *kubeopenv1alpha1.OpenCodeConfigSource) (*runtime.RawExtension, error) {
	if src.ConfigMapRef != nil {
		return resolveConfigMapRef(ctx, reader, namespace, src.ConfigMapRef)
	}
	if src.SecretRef != nil {
		return resolveSecretRef(ctx, reader, namespace, src.SecretRef)
	}
	// Should not happen due to XValidation, but defensive
	return nil, fmt.Errorf("configRef must specify either configMapRef or secretRef")
}
