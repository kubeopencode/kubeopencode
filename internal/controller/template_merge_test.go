// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

func TestMergeAgentWithTemplate(t *testing.T) {
	tests := []struct {
		name     string
		agent    *kubeopenv1alpha1.Agent
		template *kubeopenv1alpha1.AgentTemplate
		check    func(t *testing.T, cfg agentConfig)
	}{
		{
			name: "agent inherits all template values when agent fields are empty",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "default-sa",
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					AgentImage:         "custom-agent:v1",
					ExecutorImage:      "custom-executor:v1",
					AttachImage:        "custom-attach:v1",
					WorkspaceDir:       "/tmpl-workspace",
					ServiceAccountName: "tmpl-sa",
					Command:            []string{"sh", "-c", "echo hello"},
					Config:             strPtr(`{"model":"gpt-4"}`),
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				// Agent scalar fields win (workspaceDir, serviceAccountName are required on Agent)
				if cfg.workspaceDir != "/workspace" {
					t.Errorf("expected workspaceDir=/workspace, got %s", cfg.workspaceDir)
				}
				if cfg.serviceAccountName != "default-sa" {
					t.Errorf("expected serviceAccountName=default-sa, got %s", cfg.serviceAccountName)
				}
				// Images: agent is empty, so template + defaults
				if cfg.agentImage != "custom-agent:v1" {
					t.Errorf("expected agentImage=custom-agent:v1, got %s", cfg.agentImage)
				}
				if cfg.executorImage != "custom-executor:v1" {
					t.Errorf("expected executorImage=custom-executor:v1, got %s", cfg.executorImage)
				}
				if cfg.attachImage != "custom-attach:v1" {
					t.Errorf("expected attachImage=custom-attach:v1, got %s", cfg.attachImage)
				}
				// Command inherited from template
				if len(cfg.command) != 3 || cfg.command[0] != "sh" {
					t.Errorf("expected command from template, got %v", cfg.command)
				}
				// Config inherited from template
				if cfg.config == nil || *cfg.config != `{"model":"gpt-4"}` {
					t.Errorf("expected config from template, got %v", cfg.config)
				}
			},
		},
		{
			name: "agent overrides template scalar fields",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					AgentImage:         "my-agent:v2",
					ExecutorImage:      "my-executor:v2",
					WorkspaceDir:       "/my-workspace",
					ServiceAccountName: "my-sa",
					Config:             strPtr(`{"model":"claude"}`),
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					AgentImage:         "tmpl-agent:v1",
					ExecutorImage:      "tmpl-executor:v1",
					WorkspaceDir:       "/tmpl-workspace",
					ServiceAccountName: "tmpl-sa",
					Config:             strPtr(`{"model":"gpt-4"}`),
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.agentImage != "my-agent:v2" {
					t.Errorf("expected agent image override, got %s", cfg.agentImage)
				}
				if cfg.executorImage != "my-executor:v2" {
					t.Errorf("expected executor image override, got %s", cfg.executorImage)
				}
				if cfg.workspaceDir != "/my-workspace" {
					t.Errorf("expected workspaceDir override, got %s", cfg.workspaceDir)
				}
				if cfg.config == nil || *cfg.config != `{"model":"claude"}` {
					t.Errorf("expected config override, got %v", cfg.config)
				}
			},
		},
		{
			name: "agent list fields replace template (contexts)",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					Contexts: []kubeopenv1alpha1.ContextItem{
						{Name: "agent-ctx", Type: kubeopenv1alpha1.ContextTypeText, Text: "hello"},
					},
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					Contexts: []kubeopenv1alpha1.ContextItem{
						{Name: "tmpl-ctx-1", Type: kubeopenv1alpha1.ContextTypeText, Text: "tmpl1"},
						{Name: "tmpl-ctx-2", Type: kubeopenv1alpha1.ContextTypeText, Text: "tmpl2"},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				// Agent contexts replace template (not append)
				if len(cfg.contexts) != 1 {
					t.Fatalf("expected 1 context (agent replaces template), got %d", len(cfg.contexts))
				}
				if cfg.contexts[0].Name != "agent-ctx" {
					t.Errorf("expected agent-ctx, got %s", cfg.contexts[0].Name)
				}
			},
		},
		{
			name: "nil agent list inherits template list",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					// Contexts is nil
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					Contexts: []kubeopenv1alpha1.ContextItem{
						{Name: "tmpl-ctx", Type: kubeopenv1alpha1.ContextTypeText, Text: "tmpl"},
					},
					Credentials: []kubeopenv1alpha1.Credential{
						{Name: "tmpl-cred", SecretRef: kubeopenv1alpha1.SecretReference{Name: "secret"}},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if len(cfg.contexts) != 1 || cfg.contexts[0].Name != "tmpl-ctx" {
					t.Errorf("expected template contexts inherited, got %v", cfg.contexts)
				}
				if len(cfg.credentials) != 1 || cfg.credentials[0].Name != "tmpl-cred" {
					t.Errorf("expected template credentials inherited, got %v", cfg.credentials)
				}
			},
		},
		{
			name: "image defaults applied when both agent and template are empty",
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
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.agentImage != DefaultAgentImage {
					t.Errorf("expected default agent image %s, got %s", DefaultAgentImage, cfg.agentImage)
				}
				if cfg.executorImage != DefaultExecutorImage {
					t.Errorf("expected default executor image %s, got %s", DefaultExecutorImage, cfg.executorImage)
				}
				if cfg.attachImage != DefaultAttachImage {
					t.Errorf("expected default attach image %s, got %s", DefaultAttachImage, cfg.attachImage)
				}
			},
		},
		{
			name: "agent-only fields (maxConcurrentTasks, quota) come from agent",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					MaxConcurrentTasks: int32Ptr(5),
					Quota: &kubeopenv1alpha1.QuotaConfig{
						MaxTaskStarts: 10,
						WindowSeconds: 3600,
					},
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.maxConcurrentTasks == nil || *cfg.maxConcurrentTasks != 5 {
					t.Errorf("expected maxConcurrentTasks=5, got %v", cfg.maxConcurrentTasks)
				}
				if cfg.quota == nil || cfg.quota.MaxTaskStarts != 10 {
					t.Errorf("expected quota.MaxTaskStarts=10, got %v", cfg.quota)
				}
			},
		},
		{
			name: "serverConfig inherited from template",
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
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 8080,
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.serverConfig == nil {
					t.Fatal("expected serverConfig from template")
				}
				if cfg.serverConfig.Port != 8080 {
					t.Errorf("expected port 8080, got %d", cfg.serverConfig.Port)
				}
			},
		},
		{
			name: "agent serverConfig overrides template",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 9090,
					},
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ServerConfig: &kubeopenv1alpha1.ServerConfig{
						Port: 8080,
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.serverConfig == nil || cfg.serverConfig.Port != 9090 {
					t.Errorf("expected agent serverConfig override with port 9090, got %v", cfg.serverConfig)
				}
			},
		},
		{
			name: "imagePullSecrets replaced by agent",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "agent-secret"},
					},
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "tmpl-secret-1"},
						{Name: "tmpl-secret-2"},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if len(cfg.imagePullSecrets) != 1 || cfg.imagePullSecrets[0].Name != "agent-secret" {
					t.Errorf("expected agent imagePullSecrets to replace template, got %v", cfg.imagePullSecrets)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set ObjectMeta for valid objects
			if tt.agent.Name == "" {
				tt.agent.Name = "test-agent"
				tt.agent.Namespace = "default"
			}
			if tt.template.Name == "" {
				tt.template.Name = "test-template"
				tt.template.Namespace = "default"
			}

			cfg := MergeAgentWithTemplate(tt.agent, tt.template)
			tt.check(t, cfg)
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func int32Ptr(i int32) *int32 {
	return &i
}

// Verify that MergeAgentWithTemplate with empty template behaves like ResolveAgentConfig
func TestMergeWithEmptyTemplateMatchesResolveConfig(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: kubeopenv1alpha1.AgentSpec{
			AgentImage:         "my-agent:v1",
			ExecutorImage:      "my-executor:v1",
			WorkspaceDir:       "/workspace",
			ServiceAccountName: "my-sa",
			MaxConcurrentTasks: int32Ptr(3),
		},
	}
	emptyTemplate := &kubeopenv1alpha1.AgentTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: "default"},
		Spec: kubeopenv1alpha1.AgentTemplateSpec{
			WorkspaceDir:       "/tmpl-workspace",
			ServiceAccountName: "tmpl-sa",
		},
	}

	merged := MergeAgentWithTemplate(agent, emptyTemplate)
	direct := ResolveAgentConfig(agent)

	// Agent fields should be the same (agent always wins for non-zero values)
	if merged.agentImage != direct.agentImage {
		t.Errorf("agentImage mismatch: merged=%s direct=%s", merged.agentImage, direct.agentImage)
	}
	if merged.executorImage != direct.executorImage {
		t.Errorf("executorImage mismatch: merged=%s direct=%s", merged.executorImage, direct.executorImage)
	}
	if merged.workspaceDir != direct.workspaceDir {
		t.Errorf("workspaceDir mismatch: merged=%s direct=%s", merged.workspaceDir, direct.workspaceDir)
	}
	if merged.serviceAccountName != direct.serviceAccountName {
		t.Errorf("serviceAccountName mismatch: merged=%s direct=%s", merged.serviceAccountName, direct.serviceAccountName)
	}
}
