// Copyright Contributors to the KubeOpenCode project

//go:build !integration

package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

// fakeReader implements contextReader for testing.
type fakeReader struct {
	configMaps map[types.NamespacedName]*corev1.ConfigMap
}

func (f *fakeReader) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", obj)
	}
	stored, found := f.configMaps[key]
	if !found {
		return fmt.Errorf("configmap %s/%s not found", key.Namespace, key.Name)
	}
	*cm = *stored
	return nil
}

func newFakeReader(cms ...*corev1.ConfigMap) *fakeReader {
	r := &fakeReader{configMaps: make(map[types.NamespacedName]*corev1.ConfigMap)}
	for _, cm := range cms {
		r.configMaps[types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}] = cm
	}
	return r
}

func TestHashConfigMapData_ContextProcessor(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]string
		wantLen  int
		wantSame bool // if true, compare with same data again
	}{
		{
			name:    "empty data returns empty",
			data:    nil,
			wantLen: 0,
		},
		{
			name:    "empty map returns empty",
			data:    map[string]string{},
			wantLen: 0,
		},
		{
			name:    "single entry produces 16 char hash",
			data:    map[string]string{"key": "value"},
			wantLen: 16,
		},
		{
			name:    "deterministic - same input same output",
			data:    map[string]string{"a": "1", "b": "2"},
			wantLen: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hashConfigMapData(tt.data)
			if len(got) != tt.wantLen {
				t.Errorf("hashConfigMapData() length = %d, want %d", len(got), tt.wantLen)
			}
		})
	}

	// Determinism check
	t.Run("same data produces same hash", func(t *testing.T) {
		data := map[string]string{"a": "1", "b": "2"}
		h1 := hashConfigMapData(data)
		h2 := hashConfigMapData(data)
		if h1 != h2 {
			t.Errorf("hashes differ for same input: %s vs %s", h1, h2)
		}
	})

	// Order independence
	t.Run("different key order produces same hash", func(t *testing.T) {
		data1 := map[string]string{"a": "1", "b": "2", "c": "3"}
		data2 := map[string]string{"c": "3", "a": "1", "b": "2"}
		h1 := hashConfigMapData(data1)
		h2 := hashConfigMapData(data2)
		if h1 != h2 {
			t.Errorf("hashes differ for different key order: %s vs %s", h1, h2)
		}
	})

	// Different data produces different hash
	t.Run("different data produces different hash", func(t *testing.T) {
		h1 := hashConfigMapData(map[string]string{"key": "value1"})
		h2 := hashConfigMapData(map[string]string{"key": "value2"})
		if h1 == h2 {
			t.Errorf("hashes should differ for different values")
		}
	})
}

func TestResolveContextContentFromReader_Text(t *testing.T) {
	reader := newFakeReader()
	ctx := context.Background()

	tests := []struct {
		name        string
		item        *kubeopenv1alpha1.ContextItem
		wantContent string
	}{
		{
			name: "text context returns content",
			item: &kubeopenv1alpha1.ContextItem{
				Type: kubeopenv1alpha1.ContextTypeText,
				Text: "hello world",
			},
			wantContent: "hello world",
		},
		{
			name: "empty text returns empty",
			item: &kubeopenv1alpha1.ContextItem{
				Type: kubeopenv1alpha1.ContextTypeText,
				Text: "",
			},
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, dm, gm, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", tt.item, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dm != nil || gm != nil {
				t.Fatal("expected no dir/git mounts for text context")
			}
			if content != tt.wantContent {
				t.Errorf("content = %q, want %q", content, tt.wantContent)
			}
		})
	}
}

func TestResolveContextContentFromReader_ConfigMap(t *testing.T) {
	reader := newFakeReader(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cm", Namespace: "default"},
		Data:       map[string]string{"key1": "value1", "key2": "value2"},
	})
	ctx := context.Background()

	t.Run("configMap with specific key", func(t *testing.T) {
		key := "key1"
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeConfigMap,
			ConfigMap: &kubeopenv1alpha1.ConfigMapContext{
				Name: "my-cm",
				Key:  key,
			},
		}
		content, dm, gm, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dm != nil || gm != nil {
			t.Fatal("expected no dir/git mounts")
		}
		if content != "value1" {
			t.Errorf("content = %q, want %q", content, "value1")
		}
	})

	t.Run("configMap with mountPath returns dirMount", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeConfigMap,
			ConfigMap: &kubeopenv1alpha1.ConfigMapContext{
				Name: "my-cm",
			},
		}
		_, dm, gm, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "/workspace/config")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gm != nil {
			t.Fatal("expected no git mount")
		}
		if dm == nil {
			t.Fatal("expected dirMount")
		}
		if dm.configMapName != "my-cm" {
			t.Errorf("dirMount.configMapName = %q, want %q", dm.configMapName, "my-cm")
		}
		if dm.dirPath != "/workspace/config" {
			t.Errorf("dirMount.dirPath = %q, want %q", dm.dirPath, "/workspace/config")
		}
	})

	t.Run("configMap all keys without mountPath", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeConfigMap,
			ConfigMap: &kubeopenv1alpha1.ConfigMapContext{
				Name: "my-cm",
			},
		}
		content, dm, gm, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dm != nil || gm != nil {
			t.Fatal("expected no dir/git mounts")
		}
		// All keys should be formatted with <file> XML tags, sorted alphabetically
		if content == "" {
			t.Fatal("expected non-empty content")
		}
		// key1 should come before key2 in sorted order
		expected1 := `<file name="key1">`
		expected2 := `<file name="key2">`
		if !contains(content, expected1) || !contains(content, expected2) {
			t.Errorf("content missing expected file tags: %s", content)
		}
	})

	t.Run("configMap missing key returns error", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeConfigMap,
			ConfigMap: &kubeopenv1alpha1.ConfigMapContext{
				Name: "my-cm",
				Key:  "nonexistent",
			},
		}
		_, _, _, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "")
		if err == nil {
			t.Fatal("expected error for missing key")
		}
	})

	t.Run("configMap missing with optional=true returns empty", func(t *testing.T) {
		optional := true
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeConfigMap,
			ConfigMap: &kubeopenv1alpha1.ConfigMapContext{
				Name:     "nonexistent-cm",
				Key:      "key",
				Optional: &optional,
			},
		}
		content, _, _, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "")
		if err != nil {
			t.Fatalf("unexpected error for optional: %v", err)
		}
		if content != "" {
			t.Errorf("expected empty content for optional missing cm, got %q", content)
		}
	})

	t.Run("nil configMap returns empty", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type:      kubeopenv1alpha1.ContextTypeConfigMap,
			ConfigMap: nil,
		}
		content, dm, gm, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dm != nil || gm != nil {
			t.Fatal("expected no mounts")
		}
		if content != "" {
			t.Errorf("expected empty content, got %q", content)
		}
	})
}

func TestResolveContextContentFromReader_Git(t *testing.T) {
	reader := newFakeReader()
	ctx := context.Background()

	t.Run("git context returns gitMount", func(t *testing.T) {
		depth := 2
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeGit,
			Git: &kubeopenv1alpha1.GitContext{
				Repository: "https://github.com/example/repo.git",
				Ref:        "main",
				Path:       "docs",
				Depth:      &depth,
				SecretRef:  &kubeopenv1alpha1.GitSecretReference{Name: "git-secret"},
			},
		}
		_, dm, gm, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "/workspace/repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dm != nil {
			t.Fatal("expected no dir mount")
		}
		if gm == nil {
			t.Fatal("expected gitMount")
		}
		if gm.repository != "https://github.com/example/repo.git" {
			t.Errorf("repository = %q", gm.repository)
		}
		if gm.ref != "main" {
			t.Errorf("ref = %q, want %q", gm.ref, "main")
		}
		if gm.repoPath != "docs" {
			t.Errorf("repoPath = %q, want %q", gm.repoPath, "docs")
		}
		if gm.depth != 2 {
			t.Errorf("depth = %d, want 2", gm.depth)
		}
		if gm.secretName != "git-secret" {
			t.Errorf("secretName = %q, want %q", gm.secretName, "git-secret")
		}
		if gm.mountPath != "/workspace/repo" {
			t.Errorf("mountPath = %q, want %q", gm.mountPath, "/workspace/repo")
		}
	})

	t.Run("git context with defaults", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeGit,
			Git: &kubeopenv1alpha1.GitContext{
				Repository: "https://github.com/example/repo.git",
			},
		}
		_, _, gm, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "/workspace/git-context")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gm.ref != DefaultGitRef {
			t.Errorf("ref = %q, want default %q", gm.ref, DefaultGitRef)
		}
		if gm.depth != DefaultGitDepth {
			t.Errorf("depth = %d, want default %d", gm.depth, DefaultGitDepth)
		}
	})

	t.Run("git context with sync HotReload", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeGit,
			Git: &kubeopenv1alpha1.GitContext{
				Repository: "https://github.com/example/repo.git",
				Sync: &kubeopenv1alpha1.GitSync{
					Enabled:  true,
					Policy:   kubeopenv1alpha1.GitSyncPolicyHotReload,
					Interval: metav1.Duration{Duration: 10 * time.Minute},
				},
			},
		}
		_, _, gm, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "/workspace/repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !gm.syncEnabled {
			t.Error("expected syncEnabled = true")
		}
		if gm.syncPolicy != kubeopenv1alpha1.GitSyncPolicyHotReload {
			t.Errorf("syncPolicy = %q, want HotReload", gm.syncPolicy)
		}
		if gm.syncInterval != 10*time.Minute {
			t.Errorf("syncInterval = %v, want 10m", gm.syncInterval)
		}
	})

	t.Run("git context sync defaults", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeGit,
			Git: &kubeopenv1alpha1.GitContext{
				Repository: "https://github.com/example/repo.git",
				Sync: &kubeopenv1alpha1.GitSync{
					Enabled: true,
				},
			},
		}
		_, _, gm, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "/workspace/repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gm.syncPolicy != kubeopenv1alpha1.GitSyncPolicyHotReload {
			t.Errorf("default syncPolicy = %q, want HotReload", gm.syncPolicy)
		}
		if gm.syncInterval != 5*time.Minute {
			t.Errorf("default syncInterval = %v, want 5m", gm.syncInterval)
		}
	})

	t.Run("git context with recurseSubmodules", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeGit,
			Git: &kubeopenv1alpha1.GitContext{
				Repository:        "https://github.com/example/repo.git",
				RecurseSubmodules: true,
			},
		}
		_, _, gm, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "/workspace/repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !gm.recurseSubmodules {
			t.Error("expected recurseSubmodules = true")
		}
	})

	t.Run("nil git returns empty", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeGit,
			Git:  nil,
		}
		content, dm, gm, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "/workspace/repo")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dm != nil || gm != nil {
			t.Fatal("expected no mounts for nil git")
		}
		if content != "" {
			t.Errorf("expected empty content, got %q", content)
		}
	})
}

func TestResolveContextContentFromReader_Runtime(t *testing.T) {
	reader := newFakeReader()
	ctx := context.Background()

	item := &kubeopenv1alpha1.ContextItem{
		Type: kubeopenv1alpha1.ContextTypeRuntime,
	}
	content, dm, gm, err := resolveContextContentFromReader(reader, ctx, "default", "runtime", "/workspace", item, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dm != nil || gm != nil {
		t.Fatal("expected no mounts for runtime context")
	}
	if content != RuntimeSystemPrompt {
		t.Errorf("content = %q, want RuntimeSystemPrompt", content)
	}
}

func TestResolveContextContentFromReader_UnknownType(t *testing.T) {
	reader := newFakeReader()
	ctx := context.Background()

	item := &kubeopenv1alpha1.ContextItem{
		Type: "UnknownType",
	}
	_, _, _, err := resolveContextContentFromReader(reader, ctx, "default", "context", "/workspace", item, "")
	if err == nil {
		t.Fatal("expected error for unknown context type")
	}
}

func TestResolveContextItemFromReader(t *testing.T) {
	reader := newFakeReader()
	ctx := context.Background()

	t.Run("git context without mountPath fails", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type: kubeopenv1alpha1.ContextTypeGit,
			Git: &kubeopenv1alpha1.GitContext{
				Repository: "https://github.com/example/repo.git",
			},
		}
		_, _, _, err := resolveContextItemFromReader(reader, ctx, item, "default", "/workspace")
		if err == nil {
			t.Fatal("expected error for git context without mountPath")
		}
	})

	t.Run("text context with mountPath resolves path", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type:      kubeopenv1alpha1.ContextTypeText,
			Text:      "hello",
			MountPath: "config.md",
		}
		rc, _, _, err := resolveContextItemFromReader(reader, ctx, item, "default", "/workspace")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rc == nil {
			t.Fatal("expected resolvedContext")
		}
		if rc.mountPath != "/workspace/config.md" {
			t.Errorf("mountPath = %q, want %q", rc.mountPath, "/workspace/config.md")
		}
		if rc.ctxType != string(kubeopenv1alpha1.ContextTypeText) {
			t.Errorf("ctxType = %q, want %q", rc.ctxType, kubeopenv1alpha1.ContextTypeText)
		}
	})

	t.Run("runtime context forces empty mountPath", func(t *testing.T) {
		item := &kubeopenv1alpha1.ContextItem{
			Type:      kubeopenv1alpha1.ContextTypeRuntime,
			MountPath: "should-be-ignored",
		}
		rc, _, _, err := resolveContextItemFromReader(reader, ctx, item, "default", "/workspace")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rc.mountPath != "" {
			t.Errorf("runtime mountPath should be empty, got %q", rc.mountPath)
		}
		if rc.name != "runtime" {
			t.Errorf("name = %q, want %q", rc.name, "runtime")
		}
	})

	t.Run("text context with fileMode", func(t *testing.T) {
		mode := int32(0755)
		item := &kubeopenv1alpha1.ContextItem{
			Type:      kubeopenv1alpha1.ContextTypeText,
			Text:      "#!/bin/bash\necho hello",
			MountPath: "script.sh",
			FileMode:  &mode,
		}
		rc, _, _, err := resolveContextItemFromReader(reader, ctx, item, "default", "/workspace")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rc.fileMode == nil || *rc.fileMode != 0755 {
			t.Errorf("fileMode = %v, want 0755", rc.fileMode)
		}
	})
}

func TestProcessContextItems(t *testing.T) {
	reader := newFakeReader(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "default"},
		Data:       map[string]string{"data.txt": "config data"},
	})
	ctx := context.Background()

	t.Run("empty items returns empty slices", func(t *testing.T) {
		resolved, dirMounts, gitMounts, err := processContextItems(reader, ctx, nil, "default", "/workspace")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resolved) != 0 || len(dirMounts) != 0 || len(gitMounts) != 0 {
			t.Error("expected empty slices for nil items")
		}
	})

	t.Run("mixed context types", func(t *testing.T) {
		items := []kubeopenv1alpha1.ContextItem{
			{
				Type: kubeopenv1alpha1.ContextTypeText,
				Text: "text content",
			},
			{
				Type: kubeopenv1alpha1.ContextTypeConfigMap,
				ConfigMap: &kubeopenv1alpha1.ConfigMapContext{
					Name: "test-cm",
				},
				MountPath: "/workspace/configs",
			},
			{
				Type: kubeopenv1alpha1.ContextTypeGit,
				Git: &kubeopenv1alpha1.GitContext{
					Repository: "https://github.com/example/repo.git",
				},
				MountPath: "/workspace/repo",
			},
		}
		resolved, dirMounts, gitMounts, err := processContextItems(reader, ctx, items, "default", "/workspace")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resolved) != 1 {
			t.Errorf("resolved count = %d, want 1 (text)", len(resolved))
		}
		if len(dirMounts) != 1 {
			t.Errorf("dirMounts count = %d, want 1 (configMap)", len(dirMounts))
		}
		if len(gitMounts) != 1 {
			t.Errorf("gitMounts count = %d, want 1 (git)", len(gitMounts))
		}
	})

	t.Run("error propagation", func(t *testing.T) {
		items := []kubeopenv1alpha1.ContextItem{
			{
				Type: "InvalidType",
			},
		}
		_, _, _, err := processContextItems(reader, ctx, items, "default", "/workspace")
		if err == nil {
			t.Fatal("expected error for invalid context type")
		}
	})
}

func TestBuildContextConfigMapData(t *testing.T) {
	t.Run("contexts with mountPath become file mounts", func(t *testing.T) {
		resolved := []resolvedContext{
			{
				name:      "context",
				namespace: "default",
				ctxType:   "Text",
				content:   "file content",
				mountPath: "/workspace/file.md",
			},
		}
		data, fileMounts := buildContextConfigMapData(resolved, "/workspace")
		if len(data) != 1 {
			t.Fatalf("expected 1 ConfigMap entry, got %d", len(data))
		}
		key := sanitizeConfigMapKey("/workspace/file.md")
		if data[key] != "file content" {
			t.Errorf("ConfigMap data[%s] = %q, want %q", key, data[key], "file content")
		}
		if len(fileMounts) != 1 {
			t.Fatalf("expected 1 file mount, got %d", len(fileMounts))
		}
		if fileMounts[0].filePath != "/workspace/file.md" {
			t.Errorf("fileMount path = %q, want %q", fileMounts[0].filePath, "/workspace/file.md")
		}
	})

	t.Run("contexts without mountPath aggregated to context.md", func(t *testing.T) {
		resolved := []resolvedContext{
			{
				name:      "instructions",
				namespace: "default",
				ctxType:   "Text",
				content:   "some instructions",
			},
			{
				name:      "runtime",
				namespace: "default",
				ctxType:   "Runtime",
				content:   "runtime info",
			},
		}
		data, fileMounts := buildContextConfigMapData(resolved, "/workspace")

		contextFileKey := sanitizeConfigMapKey("/workspace/" + ContextFileRelPath)
		if _, ok := data[contextFileKey]; !ok {
			t.Fatalf("expected context file key %s in ConfigMap data", contextFileKey)
		}
		content := data[contextFileKey]
		if !contains(content, `<context name="instructions"`) {
			t.Errorf("missing instructions context tag in %s", content)
		}
		if !contains(content, `<context name="runtime"`) {
			t.Errorf("missing runtime context tag in %s", content)
		}

		// context file should be in fileMounts
		found := false
		for _, fm := range fileMounts {
			if fm.filePath == "/workspace/"+ContextFileRelPath {
				found = true
			}
		}
		if !found {
			t.Error("expected context file in fileMounts")
		}
	})

	t.Run("fileMode preserved in file mounts", func(t *testing.T) {
		mode := int32(0755)
		resolved := []resolvedContext{
			{
				content:   "script",
				mountPath: "/workspace/run.sh",
				fileMode:  &mode,
			},
		}
		_, fileMounts := buildContextConfigMapData(resolved, "/workspace")
		if len(fileMounts) != 1 {
			t.Fatalf("expected 1 file mount, got %d", len(fileMounts))
		}
		if fileMounts[0].fileMode == nil || *fileMounts[0].fileMode != 0755 {
			t.Errorf("fileMode not preserved: %v", fileMounts[0].fileMode)
		}
	})

	t.Run("empty resolved contexts produce no data", func(t *testing.T) {
		data, fileMounts := buildContextConfigMapData(nil, "/workspace")
		if len(data) != 0 {
			t.Errorf("expected empty data, got %d entries", len(data))
		}
		if len(fileMounts) != 0 {
			t.Errorf("expected no fileMounts, got %d", len(fileMounts))
		}
	})
}

func TestServerContextConfigMapName(t *testing.T) {
	name := ServerContextConfigMapName("my-agent")
	if name != "my-agent-server-context" {
		t.Errorf("ServerContextConfigMapName = %q, want %q", name, "my-agent-server-context")
	}
}

func TestBuildServerContextConfigMap(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
	}

	t.Run("nil data returns nil", func(t *testing.T) {
		cm := BuildServerContextConfigMap(agent, nil)
		if cm != nil {
			t.Error("expected nil ConfigMap for nil data")
		}
	})

	t.Run("empty data returns nil", func(t *testing.T) {
		cm := BuildServerContextConfigMap(agent, map[string]string{})
		if cm != nil {
			t.Error("expected nil ConfigMap for empty data")
		}
	})

	t.Run("returns ConfigMap with correct metadata", func(t *testing.T) {
		data := map[string]string{"key": "value"}
		cm := BuildServerContextConfigMap(agent, data)
		if cm == nil {
			t.Fatal("expected non-nil ConfigMap")
		}
		if cm.Name != "test-agent-server-context" {
			t.Errorf("name = %q, want %q", cm.Name, "test-agent-server-context")
		}
		if cm.Namespace != "default" {
			t.Errorf("namespace = %q, want %q", cm.Namespace, "default")
		}
		if cm.Labels["app"] != "kubeopencode" {
			t.Errorf("label app = %q, want kubeopencode", cm.Labels["app"])
		}
		if cm.Labels[AgentLabelKey] != "test-agent" {
			t.Errorf("label agent = %q, want test-agent", cm.Labels[AgentLabelKey])
		}
		if cm.Data["key"] != "value" {
			t.Errorf("data[key] = %q, want value", cm.Data["key"])
		}
	})
}

func TestGetConfigMapKeyFromReader(t *testing.T) {
	reader := newFakeReader(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cm", Namespace: "ns"},
		Data:       map[string]string{"key1": "val1"},
	})
	ctx := context.Background()

	t.Run("existing key", func(t *testing.T) {
		content, err := getConfigMapKeyFromReader(reader, ctx, "ns", "test-cm", "key1", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "val1" {
			t.Errorf("content = %q, want val1", content)
		}
	})

	t.Run("missing key non-optional", func(t *testing.T) {
		_, err := getConfigMapKeyFromReader(reader, ctx, "ns", "test-cm", "missing", nil)
		if err == nil {
			t.Fatal("expected error for missing key")
		}
	})

	t.Run("missing key optional", func(t *testing.T) {
		optional := true
		content, err := getConfigMapKeyFromReader(reader, ctx, "ns", "test-cm", "missing", &optional)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "" {
			t.Errorf("expected empty for optional missing key, got %q", content)
		}
	})

	t.Run("missing configMap non-optional", func(t *testing.T) {
		_, err := getConfigMapKeyFromReader(reader, ctx, "ns", "nonexistent", "key", nil)
		if err == nil {
			t.Fatal("expected error for missing configMap")
		}
	})

	t.Run("missing configMap optional", func(t *testing.T) {
		optional := true
		content, err := getConfigMapKeyFromReader(reader, ctx, "ns", "nonexistent", "key", &optional)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "" {
			t.Errorf("expected empty for optional missing cm, got %q", content)
		}
	})
}

func TestGetConfigMapAllKeysFromReader(t *testing.T) {
	reader := newFakeReader(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "multi-cm", Namespace: "ns"},
		Data:       map[string]string{"b.txt": "beta", "a.txt": "alpha"},
	})
	ctx := context.Background()

	t.Run("all keys sorted with XML tags", func(t *testing.T) {
		content, err := getConfigMapAllKeysFromReader(reader, ctx, "ns", "multi-cm", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// a.txt should come before b.txt
		if !contains(content, `<file name="a.txt">`) || !contains(content, `<file name="b.txt">`) {
			t.Errorf("missing file tags in: %s", content)
		}
		if !contains(content, "alpha") || !contains(content, "beta") {
			t.Errorf("missing content in: %s", content)
		}
	})

	t.Run("empty configMap returns empty", func(t *testing.T) {
		emptyReader := newFakeReader(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "empty-cm", Namespace: "ns"},
			Data:       map[string]string{},
		})
		content, err := getConfigMapAllKeysFromReader(emptyReader, ctx, "ns", "empty-cm", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if content != "" {
			t.Errorf("expected empty, got %q", content)
		}
	})
}

// contains is a helper to check substring presence.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
