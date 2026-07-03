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

func TestMergeAgentWithTemplate_ConfigRef(t *testing.T) {
	tests := []struct {
		name     string
		agent    *kubeopenv1alpha1.Agent
		template *kubeopenv1alpha1.AgentTemplate
		check    func(t *testing.T, cfg agentConfig)
	}{
		{
			name: "agent configRef overrides template configRef",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ConfigRef: &kubeopenv1alpha1.OpenCodeConfigSource{
						ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "agent-config"},
					},
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ConfigRef: &kubeopenv1alpha1.OpenCodeConfigSource{
						ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "tmpl-config"},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.configRef == nil || cfg.configRef.ConfigMapRef == nil || cfg.configRef.ConfigMapRef.Name != "agent-config" {
					t.Errorf("expected agent configRef to win, got %v", cfg.configRef)
				}
			},
		},
		{
			name: "nil agent configRef inherits template configRef",
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
					ConfigRef: &kubeopenv1alpha1.OpenCodeConfigSource{
						ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "tmpl-config", Key: "config.json"},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.configRef == nil || cfg.configRef.ConfigMapRef == nil || cfg.configRef.ConfigMapRef.Name != "tmpl-config" {
					t.Errorf("expected template configRef inherited, got %v", cfg.configRef)
				}
				if cfg.configRef.ConfigMapRef.Key != "config.json" {
					t.Errorf("expected template configRef key inherited, got %s", cfg.configRef.ConfigMapRef.Key)
				}
			},
		},
		{
			name: "agent config (inline) with template configRef — both set after merge",
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
					ConfigRef: &kubeopenv1alpha1.OpenCodeConfigSource{
						ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "tmpl-config"},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.config == nil || string(cfg.config.Raw) != `{"model":"claude"}` {
					t.Errorf("expected agent inline config, got %v", cfg.config)
				}
				if cfg.configRef == nil || cfg.configRef.ConfigMapRef == nil || cfg.configRef.ConfigMapRef.Name != "tmpl-config" {
					t.Errorf("expected template configRef inherited, got %v", cfg.configRef)
				}
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
		ref       *kubeopenv1alpha1.OpenCodeConfigMapReference
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
			ref:       &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "my-config"},
			namespace: "default",
			wantRaw:   `{"model":"gpt-4"}`,
		},
		{
			name: "custom key",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-config", Namespace: "default"},
				Data:       map[string]string{"config.json": `{"model":"claude"}`},
			},
			ref:       &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "my-config", Key: "config.json"},
			namespace: "default",
			wantRaw:   `{"model":"claude"}`,
		},
		{
			name:      "configmap not found",
			configMap: nil,
			ref:       &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "missing"},
			namespace: "default",
			wantErr:   true,
		},
		{
			name: "key not found in configmap",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-config", Namespace: "default"},
				Data:       map[string]string{"other.json": `{}`},
			},
			ref:       &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "my-config"},
			namespace: "default",
			wantErr:   true,
		},
		{
			name: "invalid JSON content",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-config", Namespace: "default"},
				Data:       map[string]string{"opencode.json": "not json{"},
			},
			ref:       &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "my-config"},
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
			objs := []client.Object{}
			if tt.configMap != nil {
				objs = append(objs, tt.configMap)
			}
			reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

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

func TestResolveSecretRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Data:       map[string][]byte{"opencode.json": []byte(`{"model":"gpt-4"}`)},
	}

	tests := []struct {
		name      string
		objects   []client.Object
		ref       *kubeopenv1alpha1.OpenCodeConfigSecretReference
		namespace string
		wantErr   bool
		wantRaw   string
	}{
		{
			name:    "default key opencode.json",
			objects: []client.Object{secret},
			ref:     &kubeopenv1alpha1.OpenCodeConfigSecretReference{Name: "my-secret"},
			wantRaw: `{"model":"gpt-4"}`,
		},
		{
			name: "custom key",
			objects: []client.Object{&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
				Data:       map[string][]byte{"config.json": []byte(`{"model":"claude"}`)},
			}},
			ref:       &kubeopenv1alpha1.OpenCodeConfigSecretReference{Name: "my-secret", Key: "config.json"},
			namespace: "default",
			wantRaw:   `{"model":"claude"}`,
		},
		{
			name:    "secret not found",
			objects: nil,
			ref:     &kubeopenv1alpha1.OpenCodeConfigSecretReference{Name: "missing"},
			wantErr: true,
		},
		{
			name: "key not found in secret",
			objects: []client.Object{&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
				Data:       map[string][]byte{"other.json": []byte(`{}`)},
			}},
			ref:       &kubeopenv1alpha1.OpenCodeConfigSecretReference{Name: "my-secret"},
			namespace: "default",
			wantErr:   true,
		},
		{
			name: "invalid JSON content",
			objects: []client.Object{&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
				Data:       map[string][]byte{"opencode.json": []byte("not json{")},
			}},
			ref:       &kubeopenv1alpha1.OpenCodeConfigSecretReference{Name: "my-secret"},
			namespace: "default",
			wantErr:   true,
		},
		{
			name:    "nil ref returns nil without error",
			objects: nil,
			ref:     nil,
			wantRaw: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := tt.namespace
			if ns == "" {
				ns = "default"
			}
			reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()

			result, err := resolveSecretRef(context.Background(), reader, ns, tt.ref)
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

func TestResolveAgentConfigRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "my-config", Namespace: "default"},
		Data:       map[string]string{"opencode.json": `{"model":"gpt-4"}`},
	}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm).Build()

	t.Run("resolves configRef.configMapRef into config", func(t *testing.T) {
		cfg := agentConfig{
			configRef: &kubeopenv1alpha1.OpenCodeConfigSource{
				ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "my-config"},
			},
		}
		if err := resolveAgentConfigRef(context.Background(), reader, "default", &cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.config == nil || string(cfg.config.Raw) != `{"model":"gpt-4"}` {
			t.Errorf("expected config resolved from ConfigMap, got %v", cfg.config)
		}
	})

	t.Run("error when both config and configRef are set", func(t *testing.T) {
		cfg := agentConfig{
			config: rawExtPtr(`{"model":"claude"}`),
			configRef: &kubeopenv1alpha1.OpenCodeConfigSource{
				ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "my-config"},
			},
		}
		err := resolveAgentConfigRef(context.Background(), reader, "default", &cfg)
		if err == nil {
			t.Fatal("expected error for mutually exclusive fields, got nil")
		}
		if !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("expected mutually exclusive error, got: %v", err)
		}
	})

	t.Run("no-op when configRef is nil", func(t *testing.T) {
		cfg := agentConfig{}
		if err := resolveAgentConfigRef(context.Background(), reader, "default", &cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.config != nil {
			t.Errorf("expected nil config, got %v", cfg.config)
		}
	})

	t.Run("error when configmap not found", func(t *testing.T) {
		cfg := agentConfig{
			configRef: &kubeopenv1alpha1.OpenCodeConfigSource{
				ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "missing"},
			},
		}
		err := resolveAgentConfigRef(context.Background(), reader, "default", &cfg)
		if err == nil {
			t.Fatal("expected error for missing configmap, got nil")
		}
		if !strings.Contains(err.Error(), "missing") {
			t.Errorf("expected error to mention configmap name, got: %v", err)
		}
	})
}

func TestValidateConfigMutualExclusion(t *testing.T) {
	t.Run("both config and configRef set returns error", func(t *testing.T) {
		cfg := agentConfig{
			config: rawExtPtr(`{"model":"claude"}`),
			configRef: &kubeopenv1alpha1.OpenCodeConfigSource{
				ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "my-config"},
			},
		}
		err := validateConfigMutualExclusion(&cfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("expected mutually exclusive error, got: %v", err)
		}
	})

	t.Run("only config set is valid", func(t *testing.T) {
		cfg := agentConfig{
			config: rawExtPtr(`{"model":"claude"}`),
		}
		if err := validateConfigMutualExclusion(&cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("only configRef set is valid", func(t *testing.T) {
		cfg := agentConfig{
			configRef: &kubeopenv1alpha1.OpenCodeConfigSource{
				ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "my-config"},
			},
		}
		if err := validateConfigMutualExclusion(&cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("neither set is valid", func(t *testing.T) {
		cfg := agentConfig{}
		if err := validateConfigMutualExclusion(&cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestResolveAgentConfigFromTemplate_ConfigRef(t *testing.T) {
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
			ConfigRef: &kubeopenv1alpha1.OpenCodeConfigSource{
				ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "opencode-config"},
			},
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
		},
	}

	cfg, err := ResolveAgentConfigFromTemplate(context.Background(), reader, agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.config == nil || string(cfg.config.Raw) != `{"model":"big-pickle"}` {
		t.Errorf("expected config resolved from template configRef, got %v", cfg.config)
	}
}

func TestResolveAgentConfigFromTemplate_SecretRef(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = kubeopenv1alpha1.AddToScheme(scheme)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "opencode-secret", Namespace: "default"},
		Data:       map[string][]byte{"opencode.json": []byte(`{"model":"big-pickle"}`)},
	}

	tmpl := &kubeopenv1alpha1.AgentTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "my-template", Namespace: "default"},
		Spec: kubeopenv1alpha1.AgentTemplateSpec{
			WorkspaceDir:       "/workspace",
			ServiceAccountName: "sa",
			ConfigRef: &kubeopenv1alpha1.OpenCodeConfigSource{
				SecretRef: &kubeopenv1alpha1.OpenCodeConfigSecretReference{Name: "opencode-secret"},
			},
		},
	}

	reader := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret, tmpl).
		Build()

	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
		Spec: kubeopenv1alpha1.AgentSpec{
			TemplateRef: &kubeopenv1alpha1.AgentTemplateReference{Name: "my-template"},
		},
	}

	cfg, err := ResolveAgentConfigFromTemplate(context.Background(), reader, agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.config == nil || string(cfg.config.Raw) != `{"model":"big-pickle"}` {
		t.Errorf("expected config resolved from template configRef.secretRef, got %v", cfg.config)
	}
}

func TestResolveAgentConfigFromTemplate_ConfigAndConfigRefExclusive(t *testing.T) {
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
			ConfigRef: &kubeopenv1alpha1.OpenCodeConfigSource{
				ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "opencode-config"},
			},
		},
	}

	reader := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cm, tmpl).
		Build()

	t.Run("agent with inline config and template with configRef returns error", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
			Spec: kubeopenv1alpha1.AgentSpec{
				TemplateRef:        &kubeopenv1alpha1.AgentTemplateReference{Name: "my-template"},
				WorkspaceDir:       "/workspace",
				ServiceAccountName: "sa",
				Config:             rawExtPtr(`{"model":"claude"}`),
			},
		}
		_, err := ResolveAgentConfigFromTemplate(context.Background(), reader, agent)
		if err == nil {
			t.Fatal("expected error for mutually exclusive config and configRef, got nil")
		}
		if !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("expected mutually exclusive error, got: %v", err)
		}
	})

	t.Run("agent without template: both config and configRef returns error", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
			Spec: kubeopenv1alpha1.AgentSpec{
				WorkspaceDir:       "/workspace",
				ServiceAccountName: "sa",
				Config:             rawExtPtr(`{"model":"claude"}`),
				ConfigRef: &kubeopenv1alpha1.OpenCodeConfigSource{
					ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "opencode-config"},
				},
			},
		}
		_, err := ResolveAgentConfigFromTemplate(context.Background(), reader, agent)
		if err == nil {
			t.Fatal("expected error for mutually exclusive config and configRef, got nil")
		}
		if !strings.Contains(err.Error(), "mutually exclusive") {
			t.Errorf("expected mutually exclusive error, got: %v", err)
		}
	})
}

func TestResolveAgentConfig_ConfigRef(t *testing.T) {
	t.Run("configRef populated from agent spec", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			Spec: kubeopenv1alpha1.AgentSpec{
				WorkspaceDir:       "/workspace",
				ServiceAccountName: "sa",
				ConfigRef: &kubeopenv1alpha1.OpenCodeConfigSource{
					ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "my-config", Key: "custom.json"},
				},
			},
		}
		cfg := ResolveAgentConfig(agent)
		if cfg.configRef == nil || cfg.configRef.ConfigMapRef == nil || cfg.configRef.ConfigMapRef.Name != "my-config" {
			t.Errorf("expected configRef from agent spec, got %v", cfg.configRef)
		}
		if cfg.configRef.ConfigMapRef.Key != "custom.json" {
			t.Errorf("expected key from agent spec, got %s", cfg.configRef.ConfigMapRef.Key)
		}
	})

	t.Run("configRef nil when not set", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			Spec: kubeopenv1alpha1.AgentSpec{
				WorkspaceDir:       "/workspace",
				ServiceAccountName: "sa",
			},
		}
		cfg := ResolveAgentConfig(agent)
		if cfg.configRef != nil {
			t.Errorf("expected nil configRef, got %v", cfg.configRef)
		}
	})
}

func TestResolveTemplateToConfig_ConfigRef(t *testing.T) {
	tmpl := &kubeopenv1alpha1.AgentTemplate{
		Spec: kubeopenv1alpha1.AgentTemplateSpec{
			WorkspaceDir:       "/workspace",
			ServiceAccountName: "sa",
			ConfigRef: &kubeopenv1alpha1.OpenCodeConfigSource{
				ConfigMapRef: &kubeopenv1alpha1.OpenCodeConfigMapReference{Name: "tmpl-config"},
			},
		},
	}
	cfg := ResolveTemplateToConfig(tmpl)
	if cfg.configRef == nil || cfg.configRef.ConfigMapRef == nil || cfg.configRef.ConfigMapRef.Name != "tmpl-config" {
		t.Errorf("expected configRef from template spec, got %v", cfg.configRef)
	}
}
