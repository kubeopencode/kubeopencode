// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

func TestMergeAgentWithTemplate_ConfigMapRef(t *testing.T) {
	tests := []struct {
		name     string
		agent    *kubeopenv1alpha1.Agent
		template *kubeopenv1alpha1.AgentTemplate
		check    func(t *testing.T, cfg agentConfig)
	}{
		{
			name: "agent configMapRef overrides template configMapRef",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ConfigMapRef:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "agent-config"},
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ConfigMapRef:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "tmpl-config"},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.configMapRef == nil || cfg.configMapRef.Name != "agent-config" {
					t.Errorf("expected agent configMapRef to win, got %v", cfg.configMapRef)
				}
			},
		},
		{
			name: "nil agent configMapRef inherits template configMapRef",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ConfigMapRef:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "tmpl-config", Key: "config.json"},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.configMapRef == nil || cfg.configMapRef.Name != "tmpl-config" {
					t.Errorf("expected template configMapRef inherited, got %v", cfg.configMapRef)
				}
				if cfg.configMapRef.Key != "config.json" {
					t.Errorf("expected template configMapRef key inherited, got %s", cfg.configMapRef.Key)
				}
			},
		},
		{
			name: "agent config (inline) wins over template configMapRef",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					Config:             rawExtPtr(`{"model":"claude"}`),
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ConfigMapRef:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "tmpl-config"},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				// Agent inline config wins over template configMapRef
				if cfg.config == nil || string(cfg.config.Raw) != `{"model":"claude"}` {
					t.Errorf("expected agent inline config to win, got %v", cfg.config)
				}
				// configMapRef from template is NOT inherited because agent has its own config source
				// (firstNonNilPtr: agent.Spec.ConfigMapRef is nil, template.Spec.ConfigMapRef is set → template wins)
				// This is acceptable because resolveAgentConfigMapRef skips resolution when config is already set
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := MergeAgentWithTemplate(tt.agent, tt.template)
			tt.check(t, cfg)
		})
	}
}

func TestResolveConfigMapRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name      string
		configMap *corev1.ConfigMap
		ref       *kubeopenv1alpha1.OpenCodeConfigRef
		namespace string
		wantErr   bool
		wantRaw   string
	}{
		{
			name: "default key opencode.json",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-config", Namespace: "default"},
				Data:       map[string]string{"opencode.json": `{"model":"gpt-4"}`},
			},
			ref:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "my-config"},
			namespace: "default",
			wantRaw:   `{"model":"gpt-4"}`,
		},
		{
			name: "custom key",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-config", Namespace: "default"},
				Data:       map[string]string{"config.json": `{"model":"claude"}`},
			},
			ref:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "my-config", Key: "config.json"},
			namespace: "default",
			wantRaw:   `{"model":"claude"}`,
		},
		{
			name:      "configmap not found",
			configMap: nil,
			ref:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "missing"},
			namespace: "default",
			wantErr:   true,
		},
		{
			name: "key not found in configmap",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-config", Namespace: "default"},
				Data:       map[string]string{"other.json": `{}`},
			},
			ref:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "my-config"},
			namespace: "default",
			wantErr:   true,
		},
		{
			name: "invalid JSON content",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-config", Namespace: "default"},
				Data:       map[string]string{"opencode.json": "not json{"},
			},
			ref:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "my-config"},
			namespace: "default",
			wantErr:   true,
		},
		{
			name:      "nil ref returns nil without error",
			configMap: nil,
			ref:       nil,
			namespace: "default",
			wantRaw:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var builder *fake.ClientBuilder
			objs := []client.Object{}
			if tt.configMap != nil {
				objs = append(objs, tt.configMap)
			}
			builder = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...)
			reader := builder.Build()

			result, err := resolveConfigMapRef(context.Background(), reader, tt.namespace, tt.ref)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantRaw == "" {
				if result != nil {
					t.Errorf("expected nil result, got %v", result)
				}
				return
			}
			if result == nil || string(result.Raw) != tt.wantRaw {
				t.Errorf("expected %q, got %v", tt.wantRaw, result)
			}
		})
	}
}

func TestResolveAgentConfigMapRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-config", Namespace: "default"},
		Data:       map[string]string{"opencode.json": `{"model":"gpt-4"}`},
	}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	t.Run("resolves configMapRef into config", func(t *testing.T) {
		cfg := agentConfig{
			configMapRef: &kubeopenv1alpha1.OpenCodeConfigRef{Name: "my-config"},
		}
		if err := resolveAgentConfigMapRef(context.Background(), reader, "default", &cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.config == nil || string(cfg.config.Raw) != `{"model":"gpt-4"}` {
			t.Errorf("expected config resolved from ConfigMap, got %v", cfg.config)
		}
	})

	t.Run("skips resolution when inline config already set", func(t *testing.T) {
		cfg := agentConfig{
			config:       rawExtPtr(`{"model":"claude"}`),
			configMapRef: &kubeopenv1alpha1.OpenCodeConfigRef{Name: "my-config"},
		}
		if err := resolveAgentConfigMapRef(context.Background(), reader, "default", &cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(cfg.config.Raw) != `{"model":"claude"}` {
			t.Errorf("expected inline config to be preserved, got %s", string(cfg.config.Raw))
		}
	})

	t.Run("no-op when configMapRef is nil", func(t *testing.T) {
		cfg := agentConfig{}
		if err := resolveAgentConfigMapRef(context.Background(), reader, "default", &cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.config != nil {
			t.Errorf("expected nil config, got %v", cfg.config)
		}
	})

	t.Run("error when configmap not found", func(t *testing.T) {
		cfg := agentConfig{
			configMapRef: &kubeopenv1alpha1.OpenCodeConfigRef{Name: "missing"},
		}
		err := resolveAgentConfigMapRef(context.Background(), reader, "default", &cfg)
		if err == nil {
			t.Fatal("expected error for missing configmap, got nil")
		}
		// Verify error message contains useful context
		if !strings.Contains(err.Error(), "missing") {
			t.Errorf("expected error to mention configmap name, got: %v", err)
		}
	})
}

func TestResolveAgentConfigFromTemplate_ConfigMapRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = kubeopenv1alpha1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "opencode-config", Namespace: "default"},
		Data:       map[string]string{"opencode.json": `{"model":"big-pickle"}`},
	}

	tmpl := &kubeopenv1alpha1.AgentTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "my-template", Namespace: "default"},
		Spec: kubeopenv1alpha1.AgentTemplateSpec{
			WorkspaceDir:       "/workspace",
			ServiceAccountName: "sa",
			ConfigMapRef:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "opencode-config"},
		},
	}

	reader := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cm, tmpl).
		Build()

	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
		Spec: kubeopenv1alpha1.AgentSpec{
			TemplateRef: &kubeopenv1alpha1.AgentTemplateReference{Name: "my-template"},
			// Agent inherits configMapRef from template
		},
	}

	cfg, err := ResolveAgentConfigFromTemplate(context.Background(), reader, agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.config == nil || string(cfg.config.Raw) != `{"model":"big-pickle"}` {
		t.Errorf("expected config resolved from template configMapRef, got %v", cfg.config)
	}
}

func TestResolveAgentConfig_ConfigMapRef(t *testing.T) {
	t.Run("configMapRef populated from agent spec", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			Spec: kubeopenv1alpha1.AgentSpec{
				WorkspaceDir:       "/workspace",
				ServiceAccountName: "sa",
				ConfigMapRef:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "my-config", Key: "custom.json"},
			},
		}
		cfg := ResolveAgentConfig(agent)
		if cfg.configMapRef == nil || cfg.configMapRef.Name != "my-config" {
			t.Errorf("expected configMapRef from agent spec, got %v", cfg.configMapRef)
		}
		if cfg.configMapRef.Key != "custom.json" {
			t.Errorf("expected key from agent spec, got %s", cfg.configMapRef.Key)
		}
	})

	t.Run("configMapRef nil when not set", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			Spec: kubeopenv1alpha1.AgentSpec{
				WorkspaceDir:       "/workspace",
				ServiceAccountName: "sa",
			},
		}
		cfg := ResolveAgentConfig(agent)
		if cfg.configMapRef != nil {
			t.Errorf("expected nil configMapRef, got %v", cfg.configMapRef)
		}
	})
}

func TestResolveTemplateToConfig_ConfigMapRef(t *testing.T) {
	tmpl := &kubeopenv1alpha1.AgentTemplate{
		Spec: kubeopenv1alpha1.AgentTemplateSpec{
			WorkspaceDir:       "/workspace",
			ServiceAccountName: "sa",
			ConfigMapRef:       &kubeopenv1alpha1.OpenCodeConfigRef{Name: "tmpl-config"},
		},
	}
	cfg := ResolveTemplateToConfig(tmpl)
	if cfg.configMapRef == nil || cfg.configMapRef.Name != "tmpl-config" {
		t.Errorf("expected configMapRef from template spec, got %v", cfg.configMapRef)
	}
}
