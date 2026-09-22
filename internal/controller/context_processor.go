// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const (
	// ContextHashAnnotationKey is the annotation key for the context ConfigMap content hash.
	// When ConfigMap content changes (e.g., skills.paths, config JSON, text contexts),
	// the hash value changes, triggering a Deployment rolling update.
	// This follows the same pattern as Helm's checksum/config annotation.
	ContextHashAnnotationKey = "kubeopencode.io/context-hash"
)

// hashConfigMapData computes a deterministic SHA-256 hash of ConfigMap data.
// Keys are sorted to ensure the hash is stable regardless of map iteration order.
// Returns a 16-character hex string (8 bytes), sufficient for change detection.
func hashConfigMapData(data map[string]string) string {
	if len(data) == 0 {
		return ""
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0}) // null separator between key and value
		h.Write([]byte(data[k]))
		h.Write([]byte{0}) // null separator between entries
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// contextReader is the minimal interface needed for context resolution.
// Both TaskReconciler and AgentReconciler satisfy this via client.Client.
type contextReader interface {
	Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error
}

// resolveContextItemFromReader resolves a ContextItem to its content, directory mount, or git mount.
// This is a standalone function that can be used by both TaskReconciler and AgentReconciler.
func resolveContextItemFromReader(reader contextReader, ctx context.Context, item *kubeopenv1alpha1.ContextItem, namespace, workspaceDir string) (*resolvedContext, *dirMount, *gitMount, error) {
	// Validate: Git context requires mountPath to be specified
	if item.Type == kubeopenv1alpha1.ContextTypeGit && item.MountPath == "" {
		return nil, nil, nil, fmt.Errorf("git context requires mountPath to be specified")
	}

	// Use a generated name for contexts
	name := "context"
	if item.Type == kubeopenv1alpha1.ContextTypeRuntime {
		name = "runtime"
	}

	// Resolve mountPath: relative paths are prefixed with workspaceDir
	resolvedPath := resolveMountPath(item.MountPath, workspaceDir)
	if item.Type == kubeopenv1alpha1.ContextTypeRuntime {
		resolvedPath = "" // Force empty to ensure content is appended to context file
	}

	// Resolve content based on context type
	content, dm, gm, err := resolveContextContentFromReader(reader, ctx, namespace, name, workspaceDir, item, resolvedPath)
	if err != nil {
		return nil, nil, nil, err
	}

	if dm != nil {
		return nil, dm, nil, nil
	}
	if gm != nil {
		return nil, nil, gm, nil
	}

	return &resolvedContext{
		name:      name,
		namespace: namespace,
		ctxType:   string(item.Type),
		content:   content,
		mountPath: resolvedPath,
		fileMode:  item.FileMode,
	}, nil, nil, nil
}

// resolveContextContentFromReader resolves content from a ContextItem.
func resolveContextContentFromReader(reader contextReader, ctx context.Context, namespace, name, workspaceDir string, item *kubeopenv1alpha1.ContextItem, mountPath string) (string, *dirMount, *gitMount, error) {
	switch item.Type {
	case kubeopenv1alpha1.ContextTypeText:
		if item.Text == "" {
			return "", nil, nil, nil
		}
		return item.Text, nil, nil, nil

	case kubeopenv1alpha1.ContextTypeConfigMap:
		if item.ConfigMap == nil {
			return "", nil, nil, nil
		}
		cm := item.ConfigMap

		if cm.Key != "" {
			content, err := getConfigMapKeyFromReader(reader, ctx, namespace, cm.Name, cm.Key, cm.Optional)
			return content, nil, nil, err
		}

		if mountPath != "" {
			optional := false
			if cm.Optional != nil {
				optional = *cm.Optional
			}
			return "", &dirMount{
				dirPath:       mountPath,
				configMapName: cm.Name,
				optional:      optional,
			}, nil, nil
		}

		content, err := getConfigMapAllKeysFromReader(reader, ctx, namespace, cm.Name, cm.Optional)
		return content, nil, nil, err

	case kubeopenv1alpha1.ContextTypeGit:
		if item.Git == nil {
			return "", nil, nil, nil
		}
		git := item.Git

		resolvedMountPath := defaultString(mountPath, workspaceDir+"/git-"+name)

		depth := DefaultGitDepth
		if git.Depth != nil && *git.Depth > 0 {
			depth = *git.Depth
		}

		ref := defaultString(git.Ref, DefaultGitRef)

		secretName := ""
		if git.SecretRef != nil {
			secretName = git.SecretRef.Name
		}

		gm := &gitMount{
			contextName:       name,
			repository:        git.Repository,
			ref:               ref,
			repoPath:          git.Path,
			mountPath:         resolvedMountPath,
			depth:             depth,
			secretName:        secretName,
			recurseSubmodules: git.RecurseSubmodules,
		}

		// Populate sync fields if configured
		if git.Sync != nil && git.Sync.Enabled {
			gm.syncEnabled = true
			gm.syncPolicy = git.Sync.Policy
			if gm.syncPolicy == "" {
				gm.syncPolicy = kubeopenv1alpha1.GitSyncPolicyHotReload
			}
			gm.syncInterval = git.Sync.Interval.Duration
			if gm.syncInterval == 0 {
				gm.syncInterval = 5 * time.Minute
			}
			gm.reloadOnSync = git.Sync.Reload
		}

		return "", nil, gm, nil

	case kubeopenv1alpha1.ContextTypeRuntime:
		return RuntimeSystemPrompt, nil, nil, nil

	default:
		return "", nil, nil, fmt.Errorf("unknown context type: %s", item.Type)
	}
}

// getConfigMapKeyFromReader retrieves a specific key from a ConfigMap.
func getConfigMapKeyFromReader(reader contextReader, ctx context.Context, namespace, name, key string, optional *bool) (string, error) {
	cm := &corev1.ConfigMap{}
	if err := reader.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, cm); err != nil {
		if optional != nil && *optional {
			return "", nil
		}
		return "", err
	}
	if content, ok := cm.Data[key]; ok {
		return content, nil
	}
	if optional != nil && *optional {
		return "", nil
	}
	return "", fmt.Errorf("key %s not found in ConfigMap %s", key, name)
}

// getConfigMapAllKeysFromReader retrieves all keys from a ConfigMap and formats them for aggregation.
func getConfigMapAllKeysFromReader(reader contextReader, ctx context.Context, namespace, name string, optional *bool) (string, error) {
	cm := &corev1.ConfigMap{}
	if err := reader.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, cm); err != nil {
		if optional != nil && *optional {
			return "", nil
		}
		return "", err
	}

	if len(cm.Data) == 0 {
		return "", nil
	}

	keys := make([]string, 0, len(cm.Data))
	for k := range cm.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("<file name=%q>\n%s\n</file>", key, cm.Data[key]))
	}
	return strings.Join(parts, "\n"), nil
}

// processContextItems resolves a slice of ContextItems into resolved contexts, dir mounts, and git mounts.
// This is used by both Task and Agent context processing.
func processContextItems(reader contextReader, ctx context.Context, items []kubeopenv1alpha1.ContextItem, namespace, workspaceDir string) ([]resolvedContext, []dirMount, []gitMount, error) {
	var resolved []resolvedContext
	var dirMounts []dirMount
	var gitMounts []gitMount

	for i, item := range items {
		rc, dm, gm, err := resolveContextItemFromReader(reader, ctx, &item, namespace, workspaceDir)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to resolve context[%d]: %w", i, err)
		}
		switch {
		case dm != nil:
			dirMounts = append(dirMounts, *dm)
		case gm != nil:
			gitMounts = append(gitMounts, *gm)
		case rc != nil:
			resolved = append(resolved, *rc)
		}
	}

	return resolved, dirMounts, gitMounts, nil
}

// buildContextConfigMapData builds ConfigMap data from resolved contexts.
// Returns the ConfigMap data map and file mounts for contexts that have explicit mount paths.
// Contexts without mountPath are aggregated into .kubeopencode/context.md.
func buildContextConfigMapData(resolved []resolvedContext, workspaceDir string) (map[string]string, []fileMount) {
	configMapData := make(map[string]string)
	var fileMounts []fileMount

	var contextParts []string

	for _, rc := range resolved {
		if rc.mountPath != "" {
			configMapKey := sanitizeConfigMapKey(rc.mountPath)
			configMapData[configMapKey] = rc.content
			fileMounts = append(fileMounts, fileMount{filePath: rc.mountPath, fileMode: rc.fileMode})
		} else {
			xmlTag := fmt.Sprintf("<context name=%q namespace=%q type=%q>\n%s\n</context>",
				rc.name, rc.namespace, rc.ctxType, rc.content)
			contextParts = append(contextParts, xmlTag)
		}
	}

	// Create context file if there's any aggregated content
	contextFilePath := workspaceDir + "/" + ContextFileRelPath
	if len(contextParts) > 0 {
		contextContent := strings.Join(contextParts, "\n\n")
		configMapData[sanitizeConfigMapKey(contextFilePath)] = contextContent
		fileMounts = append(fileMounts, fileMount{filePath: contextFilePath})
	}

	return configMapData, fileMounts
}

// ServerContextConfigMapName returns the ConfigMap name for a Server-mode Agent's contexts.
func ServerContextConfigMapName(agentName string) string {
	return agentName + "-server-context"
}

// BuildServerContextConfigMap creates a ConfigMap for a Server-mode Agent's contexts.
// Returns nil if there are no contexts to store.
func BuildServerContextConfigMap(agent *kubeopenv1alpha1.Agent, configMapData map[string]string) *corev1.ConfigMap {
	if len(configMapData) == 0 {
		return nil
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServerContextConfigMapName(agent.Name),
			Namespace: agent.Namespace,
			Labels: map[string]string{
				"app":                         "kubeopencode",
				"app.kubernetes.io/component": "server",
				AgentLabelKey:                 agent.Name,
			},
		},
		Data: configMapData,
	}
}
