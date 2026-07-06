// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

// rawExtPtr creates a *runtime.RawExtension from a JSON string for testing.
func rawExtPtr(s string) *runtime.RawExtension {
	return &runtime.RawExtension{Raw: []byte(s)}
}

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
					Config:             rawExtPtr(`{"model":"gpt-4"}`),
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
				if cfg.config == nil || string(cfg.config.Raw) != `{"model":"gpt-4"}` {
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
					Config:             rawExtPtr(`{"model":"claude"}`),
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					AgentImage:         "tmpl-agent:v1",
					ExecutorImage:      "tmpl-executor:v1",
					WorkspaceDir:       "/tmpl-workspace",
					ServiceAccountName: "tmpl-sa",
					Config:             rawExtPtr(`{"model":"gpt-4"}`),
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
				if cfg.config == nil || string(cfg.config.Raw) != `{"model":"claude"}` {
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
			name: "maxConcurrentTasks and quota inherited from template",
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
					MaxConcurrentTasks: int32Ptr(3),
					Quota: &kubeopenv1alpha1.QuotaConfig{
						MaxTaskStarts: 20,
						WindowSeconds: 3600,
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.maxConcurrentTasks == nil || *cfg.maxConcurrentTasks != 3 {
					t.Errorf("expected maxConcurrentTasks=3 from template, got %v", cfg.maxConcurrentTasks)
				}
				if cfg.quota == nil || cfg.quota.MaxTaskStarts != 20 {
					t.Errorf("expected quota.MaxTaskStarts=20 from template, got %v", cfg.quota)
				}
			},
		},
		{
			name: "agent maxConcurrentTasks and quota override template",
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
					MaxConcurrentTasks: int32Ptr(3),
					Quota: &kubeopenv1alpha1.QuotaConfig{
						MaxTaskStarts: 20,
						WindowSeconds: 7200,
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.maxConcurrentTasks == nil || *cfg.maxConcurrentTasks != 5 {
					t.Errorf("expected maxConcurrentTasks=5 from agent override, got %v", cfg.maxConcurrentTasks)
				}
				if cfg.quota == nil || cfg.quota.MaxTaskStarts != 10 {
					t.Errorf("expected quota.MaxTaskStarts=10 from agent override, got %v", cfg.quota)
				}
			},
		},
		{
			name: "port, persistence, and suspend are agent-only (not inherited from template)",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					Port:               9090,
					Persistence: &kubeopenv1alpha1.PersistenceConfig{
						Workspace: &kubeopenv1alpha1.VolumePersistence{Size: "10Gi"},
					},
					Suspend: true,
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.port != 9090 {
					t.Errorf("expected agent port 9090, got %d", cfg.port)
				}
				if cfg.persistence == nil || cfg.persistence.Workspace == nil || cfg.persistence.Workspace.Size != "10Gi" {
					t.Errorf("expected agent persistence with Workspace.Size=10Gi, got %v", cfg.persistence)
				}
				if !cfg.suspend {
					t.Errorf("expected agent suspend=true, got false")
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
		{
			name: "skills replaced by agent",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					Skills: []kubeopenv1alpha1.SkillSource{
						{Name: "agent-skills", Git: &kubeopenv1alpha1.GitSkillSource{Repository: "https://github.com/org/agent-skills.git"}},
					},
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					Skills: []kubeopenv1alpha1.SkillSource{
						{Name: "tmpl-skills-1", Git: &kubeopenv1alpha1.GitSkillSource{Repository: "https://github.com/org/tmpl1.git"}},
						{Name: "tmpl-skills-2", Git: &kubeopenv1alpha1.GitSkillSource{Repository: "https://github.com/org/tmpl2.git"}},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if len(cfg.skills) != 1 || cfg.skills[0].Name != "agent-skills" {
					t.Errorf("expected agent skills to replace template, got %v", cfg.skills)
				}
			},
		},
		{
			name: "nil agent skills inherits template skills",
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
					Skills: []kubeopenv1alpha1.SkillSource{
						{Name: "tmpl-skills", Git: &kubeopenv1alpha1.GitSkillSource{Repository: "https://github.com/org/tmpl.git"}},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if len(cfg.skills) != 1 || cfg.skills[0].Name != "tmpl-skills" {
					t.Errorf("expected template skills inherited, got %v", cfg.skills)
				}
			},
		},
		{
			name: "extraPorts replaced by agent",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ExtraPorts: []kubeopenv1alpha1.ExtraPort{
						{Name: "webapp", Port: 3000},
					},
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					ExtraPorts: []kubeopenv1alpha1.ExtraPort{
						{Name: "tmpl-port-1", Port: 8080},
						{Name: "tmpl-port-2", Port: 9090},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if len(cfg.extraPorts) != 1 || cfg.extraPorts[0].Name != "webapp" {
					t.Errorf("expected agent extraPorts to replace template, got %v", cfg.extraPorts)
				}
			},
		},
		{
			name: "agent podSpec with lifecycle wins over template",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					PodSpec: &kubeopenv1alpha1.AgentPodSpec{
						Lifecycle: &corev1.Lifecycle{
							PostStart: &corev1.LifecycleHandler{
								Exec: &corev1.ExecAction{
									Command: []string{"/start-agent.sh"},
								},
							},
						},
					},
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					PodSpec: &kubeopenv1alpha1.AgentPodSpec{
						Lifecycle: &corev1.Lifecycle{
							PostStart: &corev1.LifecycleHandler{
								Exec: &corev1.ExecAction{
									Command: []string{"/start-template.sh"},
								},
							},
						},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.podSpec == nil || cfg.podSpec.Lifecycle == nil || cfg.podSpec.Lifecycle.PostStart == nil {
					t.Fatal("expected agent podSpec lifecycle to be set")
				}
				cmd := cfg.podSpec.Lifecycle.PostStart.Exec.Command[0]
				if cmd != "/start-agent.sh" {
					t.Errorf("expected agent lifecycle to win, got command %s", cmd)
				}
			},
		},
		{
			name: "nil agent podSpec inherits template lifecycle",
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
					PodSpec: &kubeopenv1alpha1.AgentPodSpec{
						Lifecycle: &corev1.Lifecycle{
							PostStart: &corev1.LifecycleHandler{
								Exec: &corev1.ExecAction{
									Command: []string{"/start-template.sh"},
								},
							},
						},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.podSpec == nil || cfg.podSpec.Lifecycle == nil || cfg.podSpec.Lifecycle.PostStart == nil {
					t.Fatal("expected template podSpec lifecycle to be inherited")
				}
				cmd := cfg.podSpec.Lifecycle.PostStart.Exec.Command[0]
				if cmd != "/start-template.sh" {
					t.Errorf("expected template lifecycle inherited, got command %s", cmd)
				}
			},
		},
		{
			name: "nil agent extraPorts inherits template extraPorts",
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
					ExtraPorts: []kubeopenv1alpha1.ExtraPort{
						{Name: "tmpl-port", Port: 8080},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if len(cfg.extraPorts) != 1 || cfg.extraPorts[0].Name != "tmpl-port" {
					t.Errorf("expected template extraPorts inherited, got %v", cfg.extraPorts)
				}
			},
		},
		{
			name: "plugins replaced by agent",
			agent: &kubeopenv1alpha1.Agent{
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					Plugins: []kubeopenv1alpha1.PluginSpec{
						{Name: "slack-plugin"},
					},
				},
			},
			template: &kubeopenv1alpha1.AgentTemplate{
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					WorkspaceDir:       "/workspace",
					ServiceAccountName: "sa",
					Plugins: []kubeopenv1alpha1.PluginSpec{
						{Name: "template-plugin-a"},
						{Name: "template-plugin-b"},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if len(cfg.plugins) != 1 || cfg.plugins[0].Name != "slack-plugin" {
					t.Errorf("expected agent plugins to replace template, got %v", cfg.plugins)
				}
			},
		},
		{
			name: "nil agent plugins inherits template plugins",
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
					Plugins: []kubeopenv1alpha1.PluginSpec{
						{Name: "tmpl-plugin"},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if len(cfg.plugins) != 1 || cfg.plugins[0].Name != "tmpl-plugin" {
					t.Errorf("expected template plugins inherited, got %v", cfg.plugins)
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

func TestResolveTemplateToConfig(t *testing.T) {
	tests := []struct {
		name     string
		template *kubeopenv1alpha1.AgentTemplate
		check    func(t *testing.T, cfg agentConfig)
	}{
		{
			name: "all fields populated from template",
			template: &kubeopenv1alpha1.AgentTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "full-template", Namespace: "default"},
				Spec: kubeopenv1alpha1.AgentTemplateSpec{
					AgentImage:         "custom-agent:v1",
					ExecutorImage:      "custom-executor:v1",
					AttachImage:        "custom-attach:v1",
					WorkspaceDir:       "/tmpl-workspace",
					ServiceAccountName: "tmpl-sa",
					Command:            []string{"sh", "-c", "run"},
					Config:             rawExtPtr(`{"model":"gpt-4"}`),
					MaxConcurrentTasks: int32Ptr(5),
					Quota: &kubeopenv1alpha1.QuotaConfig{
						MaxTaskStarts: 100,
						WindowSeconds: 3600,
					},
					Contexts: []kubeopenv1alpha1.ContextItem{
						{Name: "ctx1", Type: kubeopenv1alpha1.ContextTypeText, Text: "hello"},
					},
					Credentials: []kubeopenv1alpha1.Credential{
						{Name: "cred1", SecretRef: kubeopenv1alpha1.SecretReference{Name: "secret1"}},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "pull-secret"},
					},
				},
			},
			check: func(t *testing.T, cfg agentConfig) {
				if cfg.agentImage != "custom-agent:v1" {
					t.Errorf("expected agentImage=custom-agent:v1, got %s", cfg.agentImage)
				}
				if cfg.executorImage != "custom-executor:v1" {
					t.Errorf("expected executorImage=custom-executor:v1, got %s", cfg.executorImage)
				}
				if cfg.attachImage != "custom-attach:v1" {
					t.Errorf("expected attachImage=custom-attach:v1, got %s", cfg.attachImage)
				}
				if cfg.workspaceDir != "/tmpl-workspace" {
					t.Errorf("expected workspaceDir=/tmpl-workspace, got %s", cfg.workspaceDir)
				}
				if cfg.serviceAccountName != "tmpl-sa" {
					t.Errorf("expected serviceAccountName=tmpl-sa, got %s", cfg.serviceAccountName)
				}
				if len(cfg.command) != 3 || cfg.command[0] != "sh" {
					t.Errorf("expected command [sh -c run], got %v", cfg.command)
				}
				if cfg.config == nil || string(cfg.config.Raw) != `{"model":"gpt-4"}` {
					t.Errorf("expected config from template, got %v", cfg.config)
				}
				// maxConcurrentTasks and quota are intentionally NOT populated
				// for templateRef tasks (no persistent Agent to enforce limits)
				if cfg.maxConcurrentTasks != nil {
					t.Errorf("expected maxConcurrentTasks=nil for template config, got %v", cfg.maxConcurrentTasks)
				}
				if cfg.quota != nil {
					t.Errorf("expected quota=nil for template config, got %v", cfg.quota)
				}
				if len(cfg.contexts) != 1 || cfg.contexts[0].Name != "ctx1" {
					t.Errorf("expected contexts from template, got %v", cfg.contexts)
				}
				if len(cfg.credentials) != 1 || cfg.credentials[0].Name != "cred1" {
					t.Errorf("expected credentials from template, got %v", cfg.credentials)
				}
				if len(cfg.imagePullSecrets) != 1 || cfg.imagePullSecrets[0].Name != "pull-secret" {
					t.Errorf("expected imagePullSecrets from template, got %v", cfg.imagePullSecrets)
				}
				// port, persistence, suspend are not set from template
				if cfg.port != 0 {
					t.Errorf("expected port=0 (not applicable for template), got %d", cfg.port)
				}
				if cfg.persistence != nil {
					t.Errorf("expected persistence=nil (not applicable for template), got %v", cfg.persistence)
				}
				if cfg.suspend {
					t.Errorf("expected suspend=false (not applicable for template), got true")
				}
			},
		},
		{
			name: "empty template uses image defaults",
			template: &kubeopenv1alpha1.AgentTemplate{
				ObjectMeta: metav1.ObjectMeta{Name: "empty-template", Namespace: "default"},
				Spec:       kubeopenv1alpha1.AgentTemplateSpec{},
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
				if cfg.workspaceDir != "" {
					t.Errorf("expected empty workspaceDir, got %s", cfg.workspaceDir)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ResolveTemplateToConfig(tt.template)
			tt.check(t, cfg)
		})
	}
}

// --- Tests for extraEnv / systemContainers merge behaviour ---

// TestMergeAgentWithTemplate_ExtraEnv_DeepMerged verifies that podSpec.extraEnv
// from the template and the Agent are combined (not replaced). Agent wins on
// name collision. See issue #282.
func TestMergeAgentWithTemplate_ExtraEnv_DeepMerged(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		Spec: kubeopenv1alpha1.AgentSpec{
			WorkspaceDir:       "/workspace",
			ServiceAccountName: "agent-sa",
			PodSpec: &kubeopenv1alpha1.AgentPodSpec{
				ExtraEnv: []corev1.EnvVar{
					{Name: "FROM_AGENT", Value: "yes"},
					{Name: "SHARED", Value: "agent-wins"},
				},
			},
		},
	}
	tmpl := &kubeopenv1alpha1.AgentTemplate{
		Spec: kubeopenv1alpha1.AgentTemplateSpec{
			WorkspaceDir:       "/tmpl-workspace",
			ServiceAccountName: "tmpl-sa",
			PodSpec: &kubeopenv1alpha1.AgentPodSpec{
				ExtraEnv: []corev1.EnvVar{
					{Name: "FROM_TEMPLATE", Value: "yes"},
					{Name: "SHARED", Value: "template-value"},
				},
			},
		},
	}

	cfg := MergeAgentWithTemplate(agent, tmpl)

	envMap := make(map[string]string)
	for _, e := range cfg.extraEnv {
		envMap[e.Name] = e.Value
	}
	if envMap["FROM_AGENT"] != "yes" {
		t.Errorf("expected FROM_AGENT=yes in merged extraEnv, got %v", envMap["FROM_AGENT"])
	}
	if envMap["FROM_TEMPLATE"] != "yes" {
		t.Errorf("expected FROM_TEMPLATE=yes (deep-merged, not replaced) in merged extraEnv, got %v", envMap["FROM_TEMPLATE"])
	}
	// Collision: Agent wins
	if envMap["SHARED"] != "agent-wins" {
		t.Errorf("expected SHARED=agent-wins (Agent wins on collision), got %v", envMap["SHARED"])
	}
}

func TestMergeAgentWithTemplate_ExtraEnv_TemplateInheritedWhenAgentHasNoPodSpec(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		Spec: kubeopenv1alpha1.AgentSpec{
			WorkspaceDir:       "/workspace",
			ServiceAccountName: "agent-sa",
			// No PodSpec — should inherit from template
		},
	}
	tmpl := &kubeopenv1alpha1.AgentTemplate{
		Spec: kubeopenv1alpha1.AgentTemplateSpec{
			WorkspaceDir:       "/tmpl-workspace",
			ServiceAccountName: "tmpl-sa",
			PodSpec: &kubeopenv1alpha1.AgentPodSpec{
				ExtraEnv: []corev1.EnvVar{
					{Name: "FROM_TEMPLATE", Value: "inherited"},
				},
				SystemContainers: &kubeopenv1alpha1.SystemContainerOverrides{
					GitInit: &kubeopenv1alpha1.InitContainerOverrides{
						ExtraEnv: []corev1.EnvVar{{Name: "HOME", Value: "/tmp"}},
					},
				},
			},
		},
	}

	cfg := MergeAgentWithTemplate(agent, tmpl)

	found := false
	for _, e := range cfg.extraEnv {
		if e.Name == "FROM_TEMPLATE" && e.Value == "inherited" {
			found = true
		}
	}
	if !found {
		t.Error("expected FROM_TEMPLATE to be inherited from template when Agent has no podSpec")
	}

	if cfg.systemContainers == nil {
		t.Fatal("expected systemContainers to be inherited from template")
	}
	if cfg.systemContainers.GitInit == nil {
		t.Fatal("expected systemContainers.GitInit to be inherited from template")
	}
	foundHome := false
	for _, e := range cfg.systemContainers.GitInit.ExtraEnv {
		if e.Name == "HOME" && e.Value == "/tmp" {
			foundHome = true
		}
	}
	if !foundHome {
		t.Error("expected HOME=/tmp in systemContainers.GitInit.ExtraEnv from template")
	}
}

// TestMergeAgentWithTemplate_SystemContainers_DeepMerged verifies that
// podSpec.systemContainers per-container-type overrides are deep-merged between
// template and Agent (ExtraEnv combined, Agent wins on name collision).
// See issue #282.
func TestMergeAgentWithTemplate_SystemContainers_DeepMerged(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		Spec: kubeopenv1alpha1.AgentSpec{
			WorkspaceDir:       "/workspace",
			ServiceAccountName: "agent-sa",
			PodSpec: &kubeopenv1alpha1.AgentPodSpec{
				SystemContainers: &kubeopenv1alpha1.SystemContainerOverrides{
					GitInit: &kubeopenv1alpha1.InitContainerOverrides{
						ExtraEnv: []corev1.EnvVar{
							{Name: "FROM_AGENT_SC", Value: "agent"},
							{Name: "SHARED_SC", Value: "agent-wins"},
						},
					},
				},
			},
		},
	}
	tmpl := &kubeopenv1alpha1.AgentTemplate{
		Spec: kubeopenv1alpha1.AgentTemplateSpec{
			WorkspaceDir:       "/tmpl-workspace",
			ServiceAccountName: "tmpl-sa",
			PodSpec: &kubeopenv1alpha1.AgentPodSpec{
				SystemContainers: &kubeopenv1alpha1.SystemContainerOverrides{
					GitInit: &kubeopenv1alpha1.InitContainerOverrides{
						ExtraEnv: []corev1.EnvVar{
							{Name: "FROM_TMPL_SC", Value: "template"},
							{Name: "SHARED_SC", Value: "template-value"},
						},
					},
				},
			},
		},
	}

	cfg := MergeAgentWithTemplate(agent, tmpl)

	if cfg.systemContainers == nil || cfg.systemContainers.GitInit == nil {
		t.Fatal("expected systemContainers.GitInit to be set")
	}
	envMap := make(map[string]string)
	for _, e := range cfg.systemContainers.GitInit.ExtraEnv {
		envMap[e.Name] = e.Value
	}
	if envMap["FROM_AGENT_SC"] != "agent" {
		t.Errorf("expected FROM_AGENT_SC=agent, got %v", envMap["FROM_AGENT_SC"])
	}
	if envMap["FROM_TMPL_SC"] != "template" {
		t.Errorf("expected FROM_TMPL_SC=template (deep-merged, not replaced), got %v", envMap["FROM_TMPL_SC"])
	}
	if envMap["SHARED_SC"] != "agent-wins" {
		t.Errorf("expected SHARED_SC=agent-wins (Agent wins on collision), got %v", envMap["SHARED_SC"])
	}
}

// --- Direct tests for mergePodSpec covering each field's merge strategy (#282) ---

func TestMergePodSpec_BothNil(t *testing.T) {
	if got := mergePodSpec(nil, nil); got != nil {
		t.Errorf("mergePodSpec(nil, nil) = %v, want nil", got)
	}
}

func TestMergePodSpec_AgentNilReturnsTemplate(t *testing.T) {
	tmpl := &kubeopenv1alpha1.AgentPodSpec{Labels: map[string]string{"a": "b"}}
	got := mergePodSpec(nil, tmpl)
	if got != tmpl {
		t.Errorf("mergePodSpec(nil, tmpl) should return template as-is, got %v", got)
	}
}

func TestMergePodSpec_TemplateNilReturnsAgent(t *testing.T) {
	agent := &kubeopenv1alpha1.AgentPodSpec{Labels: map[string]string{"a": "b"}}
	got := mergePodSpec(agent, nil)
	if got != agent {
		t.Errorf("mergePodSpec(agent, nil) should return agent as-is, got %v", got)
	}
}

func TestMergePodSpec_LabelsAndAnnotationsMerged(t *testing.T) {
	agent := &kubeopenv1alpha1.AgentPodSpec{
		Labels:      map[string]string{"agent-only": "a", "shared": "agent"},
		Annotations: map[string]string{"agent-only-ann": "a", "shared-ann": "agent"},
	}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		Labels:      map[string]string{"tmpl-only": "t", "shared": "template"},
		Annotations: map[string]string{"tmpl-only-ann": "t", "shared-ann": "template"},
	}
	got := mergePodSpec(agent, tmpl)

	if got.Labels["agent-only"] != "a" {
		t.Errorf("agent-only label lost: %v", got.Labels)
	}
	if got.Labels["tmpl-only"] != "t" {
		t.Errorf("tmpl-only label lost: %v", got.Labels)
	}
	if got.Labels["shared"] != "agent" {
		t.Errorf("shared label should be agent (Agent wins), got %v", got.Labels["shared"])
	}
	if got.Annotations["agent-only-ann"] != "a" {
		t.Errorf("agent-only annotation lost: %v", got.Annotations)
	}
	if got.Annotations["tmpl-only-ann"] != "t" {
		t.Errorf("tmpl-only annotation lost: %v", got.Annotations)
	}
	if got.Annotations["shared-ann"] != "agent" {
		t.Errorf("shared annotation should be agent (Agent wins), got %v", got.Annotations["shared-ann"])
	}
}

func TestMergePodSpec_ExtraVolumesDedupByName(t *testing.T) {
	agent := &kubeopenv1alpha1.AgentPodSpec{
		ExtraVolumes: []corev1.Volume{
			{Name: "agent-vol"},
			{Name: "shared-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}},
		},
	}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		ExtraVolumes: []corev1.Volume{
			{Name: "tmpl-vol"},
			{Name: "shared-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumDefault}}},
		},
	}
	got := mergePodSpec(agent, tmpl)

	names := volumeNames(got.ExtraVolumes)
	if !sliceContains(names, "agent-vol") || !sliceContains(names, "tmpl-vol") {
		t.Errorf("expected both agent-vol and tmpl-vol, got %v", names)
	}
	// Exactly one shared-vol, and it must be the Agent's (Memory medium)
	count := 0
	for _, v := range got.ExtraVolumes {
		if v.Name == "shared-vol" {
			count++
			if v.EmptyDir == nil || v.EmptyDir.Medium != corev1.StorageMediumMemory {
				t.Errorf("shared-vol should be Agent's (Memory), got %v", v.EmptyDir)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 shared-vol after dedup, got %d", count)
	}
}

func TestMergePodSpec_ExtraVolumeMountsDedupByName(t *testing.T) {
	agent := &kubeopenv1alpha1.AgentPodSpec{
		ExtraVolumeMounts: []corev1.VolumeMount{
			{Name: "agent-mount", MountPath: "/agent"},
			{Name: "shared-mount", MountPath: "/agent-path"},
		},
	}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		ExtraVolumeMounts: []corev1.VolumeMount{
			{Name: "tmpl-mount", MountPath: "/tmpl"},
			{Name: "shared-mount", MountPath: "/tmpl-path"},
		},
	}
	got := mergePodSpec(agent, tmpl)

	names := volumeMountNames(got.ExtraVolumeMounts)
	if !sliceContains(names, "agent-mount") || !sliceContains(names, "tmpl-mount") {
		t.Errorf("expected both agent-mount and tmpl-mount, got %v", names)
	}
	count := 0
	for _, vm := range got.ExtraVolumeMounts {
		if vm.Name == "shared-mount" {
			count++
			if vm.MountPath != "/agent-path" {
				t.Errorf("shared-mount should be Agent's path, got %s", vm.MountPath)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 shared-mount after dedup, got %d", count)
	}
}

func TestMergePodSpec_SchedulingMerged(t *testing.T) {
	agentAffinity := &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{}}
	tmplAffinity := &corev1.Affinity{PodAffinity: &corev1.PodAffinity{}}
	agent := &kubeopenv1alpha1.AgentPodSpec{
		Scheduling: &kubeopenv1alpha1.PodScheduling{
			NodeSelector: map[string]string{"agent-only": "a", "shared": "agent"},
			Tolerations:  []corev1.Toleration{{Key: "agent-taint", Effect: corev1.TaintEffectNoSchedule}},
			Affinity:     agentAffinity,
		},
	}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		Scheduling: &kubeopenv1alpha1.PodScheduling{
			NodeSelector: map[string]string{"tmpl-only": "t", "shared": "template"},
			Tolerations:  []corev1.Toleration{{Key: "tmpl-taint", Effect: corev1.TaintEffectNoSchedule}},
			Affinity:     tmplAffinity,
		},
	}
	got := mergePodSpec(agent, tmpl)

	if got.Scheduling == nil {
		t.Fatal("expected merged scheduling")
	}
	if got.Scheduling.NodeSelector["agent-only"] != "a" || got.Scheduling.NodeSelector["tmpl-only"] != "t" {
		t.Errorf("NodeSelector not merged: %v", got.Scheduling.NodeSelector)
	}
	if got.Scheduling.NodeSelector["shared"] != "agent" {
		t.Errorf("NodeSelector shared should be agent (Agent wins): %v", got.Scheduling.NodeSelector["shared"])
	}
	if len(got.Scheduling.Tolerations) != 2 {
		t.Errorf("expected 2 tolerations (appended), got %d: %v", len(got.Scheduling.Tolerations), got.Scheduling.Tolerations)
	}
	if got.Scheduling.Affinity != agentAffinity {
		t.Errorf("Affinity should be Agent's (firstNonNilPtr), got %v", got.Scheduling.Affinity)
	}
}

func TestMergePodSpec_SchedulingTemplateAffinityWhenAgentNil(t *testing.T) {
	tmplAffinity := &corev1.Affinity{PodAffinity: &corev1.PodAffinity{}}
	agent := &kubeopenv1alpha1.AgentPodSpec{
		Scheduling: &kubeopenv1alpha1.PodScheduling{NodeSelector: map[string]string{"a": "b"}},
	}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		Scheduling: &kubeopenv1alpha1.PodScheduling{Affinity: tmplAffinity},
	}
	got := mergePodSpec(agent, tmpl)
	if got.Scheduling.Affinity != tmplAffinity {
		t.Errorf("Affinity should be template's when Agent's is nil, got %v", got.Scheduling.Affinity)
	}
}

func TestMergePodSpec_ScalarPointersAgentWins(t *testing.T) {
	agentRuntime := "gvisor"
	agentRes := corev1.ResourceRequirements{Limits: corev1.ResourceList{}}
	agentSC := &corev1.SecurityContext{Privileged: boolPtr(true)}
	agentPodSC := &corev1.PodSecurityContext{RunAsUser: int64Ptr(1000)}
	agentLifecycle := &corev1.Lifecycle{}
	agent := &kubeopenv1alpha1.AgentPodSpec{
		RuntimeClassName:   &agentRuntime,
		Resources:          &agentRes,
		SecurityContext:    agentSC,
		PodSecurityContext: agentPodSC,
		Lifecycle:          agentLifecycle,
	}
	tmplRuntime := "kata"
	tmplRes := corev1.ResourceRequirements{}
	tmplSC := &corev1.SecurityContext{Privileged: boolPtr(false)}
	tmplPodSC := &corev1.PodSecurityContext{RunAsUser: int64Ptr(0)}
	tmplLifecycle := &corev1.Lifecycle{}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		RuntimeClassName:   &tmplRuntime,
		Resources:          &tmplRes,
		SecurityContext:    tmplSC,
		PodSecurityContext: tmplPodSC,
		Lifecycle:          tmplLifecycle,
	}
	got := mergePodSpec(agent, tmpl)

	if got.RuntimeClassName == nil || *got.RuntimeClassName != "gvisor" {
		t.Errorf("RuntimeClassName should be Agent's gvisor, got %v", got.RuntimeClassName)
	}
	if got.SecurityContext != agentSC {
		t.Errorf("SecurityContext should be Agent's, got %v", got.SecurityContext)
	}
	if got.PodSecurityContext != agentPodSC {
		t.Errorf("PodSecurityContext should be Agent's, got %v", got.PodSecurityContext)
	}
	if got.Lifecycle != agentLifecycle {
		t.Errorf("Lifecycle should be Agent's, got %v", got.Lifecycle)
	}
	if got.Resources == nil || got.Resources != &agentRes {
		t.Errorf("Resources should be Agent's, got %v", got.Resources)
	}
}

func TestMergePodSpec_ScalarPointersInheritTemplateWhenAgentNil(t *testing.T) {
	tmplRuntime := "kata"
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		RuntimeClassName: &tmplRuntime,
	}
	agent := &kubeopenv1alpha1.AgentPodSpec{
		// RuntimeClassName nil
		Labels: map[string]string{"a": "b"},
	}
	got := mergePodSpec(agent, tmpl)
	if got.RuntimeClassName == nil || *got.RuntimeClassName != "kata" {
		t.Errorf("RuntimeClassName should be template's kata, got %v", got.RuntimeClassName)
	}
}

// --- More merge edge-case tests (nil branches, all systemContainer types) ---

func TestMergePodSpec_SchedulingAgentNilInheritsTemplate(t *testing.T) {
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		Scheduling: &kubeopenv1alpha1.PodScheduling{NodeSelector: map[string]string{"t": "v"}},
	}
	agent := &kubeopenv1alpha1.AgentPodSpec{Labels: map[string]string{"a": "b"}}
	got := mergePodSpec(agent, tmpl)
	if got.Scheduling == nil || got.Scheduling.NodeSelector["t"] != "v" {
		t.Errorf("expected template Scheduling inherited when Agent's is nil, got %v", got.Scheduling)
	}
}

func TestMergePodSpec_SchedulingTemplateNilUsesAgent(t *testing.T) {
	agent := &kubeopenv1alpha1.AgentPodSpec{
		Scheduling: &kubeopenv1alpha1.PodScheduling{NodeSelector: map[string]string{"a": "v"}},
	}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{Labels: map[string]string{"t": "b"}}
	got := mergePodSpec(agent, tmpl)
	if got.Scheduling == nil || got.Scheduling.NodeSelector["a"] != "v" {
		t.Errorf("expected agent Scheduling used when template's is nil, got %v", got.Scheduling)
	}
}

func TestMergePodSpec_TolerationsOneSideEmpty(t *testing.T) {
	// Agent Tolerations empty -> inherit template's
	agent := &kubeopenv1alpha1.AgentPodSpec{
		Scheduling: &kubeopenv1alpha1.PodScheduling{Tolerations: nil},
	}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		Scheduling: &kubeopenv1alpha1.PodScheduling{
			Tolerations: []corev1.Toleration{{Key: "k1", Effect: corev1.TaintEffectNoSchedule}},
		},
	}
	got := mergePodSpec(agent, tmpl)
	if len(got.Scheduling.Tolerations) != 1 || got.Scheduling.Tolerations[0].Key != "k1" {
		t.Errorf("expected template tolerations inherited, got %v", got.Scheduling.Tolerations)
	}

	// Template Tolerations empty -> use agent's
	agent2 := &kubeopenv1alpha1.AgentPodSpec{
		Scheduling: &kubeopenv1alpha1.PodScheduling{
			Tolerations: []corev1.Toleration{{Key: "k2", Effect: corev1.TaintEffectNoSchedule}},
		},
	}
	tmpl2 := &kubeopenv1alpha1.AgentPodSpec{
		Scheduling: &kubeopenv1alpha1.PodScheduling{Tolerations: nil},
	}
	got2 := mergePodSpec(agent2, tmpl2)
	if len(got2.Scheduling.Tolerations) != 1 || got2.Scheduling.Tolerations[0].Key != "k2" {
		t.Errorf("expected agent tolerations used, got %v", got2.Scheduling.Tolerations)
	}
}

func TestMergePodSpec_SystemContainersAllTypesAndVolumeMountsMerged(t *testing.T) {
	mkSC := func(prefix string) *kubeopenv1alpha1.SystemContainerOverrides {
		return &kubeopenv1alpha1.SystemContainerOverrides{
			OpenCodeInit: &kubeopenv1alpha1.InitContainerOverrides{
				ExtraEnv:          []corev1.EnvVar{{Name: prefix + "-oci-env"}},
				ExtraVolumeMounts: []corev1.VolumeMount{{Name: prefix + "-oci-vm", MountPath: "/" + prefix + "-oci"}},
			},
			ContextInit: &kubeopenv1alpha1.InitContainerOverrides{
				ExtraEnv:          []corev1.EnvVar{{Name: prefix + "-ci-env"}},
				ExtraVolumeMounts: []corev1.VolumeMount{{Name: prefix + "-ci-vm", MountPath: "/" + prefix + "-ci"}},
			},
			GitInit: &kubeopenv1alpha1.InitContainerOverrides{
				ExtraEnv:          []corev1.EnvVar{{Name: prefix + "-gi-env"}},
				ExtraVolumeMounts: []corev1.VolumeMount{{Name: prefix + "-gi-vm", MountPath: "/" + prefix + "-gi"}},
			},
			GitSync: &kubeopenv1alpha1.InitContainerOverrides{
				ExtraEnv:          []corev1.EnvVar{{Name: prefix + "-gs-env"}},
				ExtraVolumeMounts: []corev1.VolumeMount{{Name: prefix + "-gs-vm", MountPath: "/" + prefix + "-gs"}},
			},
			PluginInit: &kubeopenv1alpha1.InitContainerOverrides{
				ExtraEnv:          []corev1.EnvVar{{Name: prefix + "-pi-env"}},
				ExtraVolumeMounts: []corev1.VolumeMount{{Name: prefix + "-pi-vm", MountPath: "/" + prefix + "-pi"}},
			},
		}
	}
	agent := &kubeopenv1alpha1.AgentPodSpec{SystemContainers: mkSC("agent")}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{SystemContainers: mkSC("tmpl")}
	got := mergePodSpec(agent, tmpl)

	if got.SystemContainers == nil {
		t.Fatal("expected merged systemContainers")
	}
	checks := []struct {
		name string
		abbr string
		sc   *kubeopenv1alpha1.InitContainerOverrides
	}{
		{"OpenCodeInit", "oci", got.SystemContainers.OpenCodeInit},
		{"ContextInit", "ci", got.SystemContainers.ContextInit},
		{"GitInit", "gi", got.SystemContainers.GitInit},
		{"GitSync", "gs", got.SystemContainers.GitSync},
		{"PluginInit", "pi", got.SystemContainers.PluginInit},
	}
	for _, c := range checks {
		if c.sc == nil {
			t.Errorf("%s: expected non-nil override", c.name)
			continue
		}
		envNames := envVarNames(c.sc.ExtraEnv)
		if !sliceContains(envNames, "agent-"+c.abbr+"-env") {
			t.Errorf("%s: agent env missing in %v", c.name, envNames)
		}
		if !sliceContains(envNames, "tmpl-"+c.abbr+"-env") {
			t.Errorf("%s: template env missing (deep-merged) in %v", c.name, envNames)
		}
		vmNames := volumeMountNames(c.sc.ExtraVolumeMounts)
		if !sliceContains(vmNames, "agent-"+c.abbr+"-vm") {
			t.Errorf("%s: agent volume mount missing in %v", c.name, vmNames)
		}
		if !sliceContains(vmNames, "tmpl-"+c.abbr+"-vm") {
			t.Errorf("%s: template volume mount missing (deep-merged) in %v", c.name, vmNames)
		}
	}
}

func TestMergePodSpec_SystemContainersOneSideNil(t *testing.T) {
	tmplSC := &kubeopenv1alpha1.SystemContainerOverrides{
		GitInit: &kubeopenv1alpha1.InitContainerOverrides{ExtraEnv: []corev1.EnvVar{{Name: "FROM_TMPL"}}},
	}
	// Agent SystemContainers nil -> inherit template's
	agent := &kubeopenv1alpha1.AgentPodSpec{Labels: map[string]string{"a": "b"}}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{SystemContainers: tmplSC}
	got := mergePodSpec(agent, tmpl)
	if got.SystemContainers != tmplSC {
		t.Errorf("expected template SystemContainers inherited when Agent's is nil, got %v", got.SystemContainers)
	}

	// Agent SystemContainers set, template nil -> use agent's
	agentSC := &kubeopenv1alpha1.SystemContainerOverrides{
		GitInit: &kubeopenv1alpha1.InitContainerOverrides{ExtraEnv: []corev1.EnvVar{{Name: "FROM_AGENT"}}},
	}
	agent2 := &kubeopenv1alpha1.AgentPodSpec{SystemContainers: agentSC}
	tmpl2 := &kubeopenv1alpha1.AgentPodSpec{Labels: map[string]string{"t": "b"}}
	got2 := mergePodSpec(agent2, tmpl2)
	if got2.SystemContainers != agentSC {
		t.Errorf("expected agent SystemContainers used when template's is nil, got %v", got2.SystemContainers)
	}
}

func TestMergePodSpec_InitContainerOverrideOneSideNil(t *testing.T) {
	// Agent defines GitInit only; template defines ContextInit only.
	// Merged result should have both, each from the side that set it.
	agent := &kubeopenv1alpha1.AgentPodSpec{
		SystemContainers: &kubeopenv1alpha1.SystemContainerOverrides{
			GitInit: &kubeopenv1alpha1.InitContainerOverrides{ExtraEnv: []corev1.EnvVar{{Name: "GIT_FROM_AGENT"}}},
		},
	}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		SystemContainers: &kubeopenv1alpha1.SystemContainerOverrides{
			ContextInit: &kubeopenv1alpha1.InitContainerOverrides{ExtraEnv: []corev1.EnvVar{{Name: "CTX_FROM_TMPL"}}},
		},
	}
	got := mergePodSpec(agent, tmpl)

	if got.SystemContainers == nil {
		t.Fatal("expected merged systemContainers")
	}
	if got.SystemContainers.GitInit == nil || len(envVarNames(got.SystemContainers.GitInit.ExtraEnv)) == 0 {
		t.Errorf("expected GitInit from agent preserved, got %v", got.SystemContainers.GitInit)
	}
	if got.SystemContainers.ContextInit == nil || len(envVarNames(got.SystemContainers.ContextInit.ExtraEnv)) == 0 {
		t.Errorf("expected ContextInit from template inherited, got %v", got.SystemContainers.ContextInit)
	}
	if got.SystemContainers.GitSync != nil || got.SystemContainers.PluginInit != nil || got.SystemContainers.OpenCodeInit != nil {
		t.Errorf("unset container types should stay nil, got %v", got.SystemContainers)
	}
}

func TestMergePodSpec_ScalarPointersInheritAllFromTemplate(t *testing.T) {
	tmplRuntime := "kata"
	tmplRes := corev1.ResourceRequirements{Limits: corev1.ResourceList{}}
	tmplSC := &corev1.SecurityContext{Privileged: boolPtr(false)}
	tmplPodSC := &corev1.PodSecurityContext{RunAsUser: int64Ptr(1000)}
	tmplLifecycle := &corev1.Lifecycle{}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		RuntimeClassName:   &tmplRuntime,
		Resources:          &tmplRes,
		SecurityContext:    tmplSC,
		PodSecurityContext: tmplPodSC,
		Lifecycle:          tmplLifecycle,
	}
	// Agent has podSpec but all scalar pointers nil
	agent := &kubeopenv1alpha1.AgentPodSpec{Labels: map[string]string{"a": "b"}}
	got := mergePodSpec(agent, tmpl)

	if got.RuntimeClassName == nil || *got.RuntimeClassName != "kata" {
		t.Errorf("RuntimeClassName should inherit template, got %v", got.RuntimeClassName)
	}
	if got.Resources != &tmplRes {
		t.Errorf("Resources should inherit template, got %v", got.Resources)
	}
	if got.SecurityContext != tmplSC {
		t.Errorf("SecurityContext should inherit template, got %v", got.SecurityContext)
	}
	if got.PodSecurityContext != tmplPodSC {
		t.Errorf("PodSecurityContext should inherit template, got %v", got.PodSecurityContext)
	}
	if got.Lifecycle != tmplLifecycle {
		t.Errorf("Lifecycle should inherit template, got %v", got.Lifecycle)
	}
}

func TestMergePodSpec_ExtraVolumesAgentOnlyWhenTemplateHasNone(t *testing.T) {
	// Template podSpec is non-nil (has Labels) but no ExtraVolumes; Agent has
	// ExtraVolumes. Covers mergeNamedSlice's len(b)==0 -> return a passthrough.
	agent := &kubeopenv1alpha1.AgentPodSpec{
		Labels:       map[string]string{"a": "b"},
		ExtraVolumes: []corev1.Volume{{Name: "agent-vol", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
	}
	tmpl := &kubeopenv1alpha1.AgentPodSpec{Labels: map[string]string{"t": "b"}}
	got := mergePodSpec(agent, tmpl)
	names := volumeNames(got.ExtraVolumes)
	if len(names) != 1 || names[0] != "agent-vol" {
		t.Errorf("expected only agent-vol, got %v", names)
	}
}

func TestMergeNamedSlice_EmptyNameAppended(t *testing.T) {
	// Items with an empty Name are always appended (no dedup possible). This
	// exercises the defensive empty-name branch.
	a := []corev1.EnvVar{{Name: "", Value: "a-empty"}, {Name: "X", Value: "a-x"}}
	b := []corev1.EnvVar{{Name: "", Value: "b-empty"}, {Name: "Y", Value: "b-y"}}
	got := mergeEnvVars(a, b)
	if len(got) != 4 {
		t.Fatalf("expected 4 env vars (no dedup for empty names), got %d: %v", len(got), got)
	}
	emptyCount := 0
	for _, e := range got {
		if e.Name == "" {
			emptyCount++
		}
	}
	if emptyCount != 2 {
		t.Errorf("expected both empty-name entries preserved, got %d", emptyCount)
	}
}

func TestMergeNamedSlice_WithinTemplateDuplicateKeepsFirst(t *testing.T) {
	// Two template volumes share a name; the first must survive (agentWins=false
	// collision branch). Agent has no collision.
	tmpl := &kubeopenv1alpha1.AgentPodSpec{
		ExtraVolumes: []corev1.Volume{
			{Name: "dup", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumDefault}}},
			{Name: "dup", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}},
		},
	}
	agent := &kubeopenv1alpha1.AgentPodSpec{
		ExtraVolumes: []corev1.Volume{{Name: "agent-vol"}},
	}
	got := mergePodSpec(agent, tmpl)

	count := 0
	for _, v := range got.ExtraVolumes {
		if v.Name == "dup" {
			count++
			if v.EmptyDir == nil || v.EmptyDir.Medium != corev1.StorageMediumDefault {
				t.Errorf("first template 'dup' (Default medium) should be kept, got %v", v.EmptyDir)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'dup' after dedup, got %d", count)
	}
}

// --- helpers ---

func volumeNames(vols []corev1.Volume) []string {
	out := make([]string, 0, len(vols))
	for _, v := range vols {
		out = append(out, v.Name)
	}
	return out
}

func envVarNames(envs []corev1.EnvVar) []string {
	out := make([]string, 0, len(envs))
	for _, e := range envs {
		out = append(out, e.Name)
	}
	return out
}

func volumeMountNames(vms []corev1.VolumeMount) []string {
	out := make([]string, 0, len(vms))
	for _, vm := range vms {
		out = append(out, vm.Name)
	}
	return out
}

func sliceContains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func int64Ptr(i int64) *int64 {
	return &i
}
