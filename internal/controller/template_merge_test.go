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
					Config:             strPtr(`{"model":"gpt-4"}`),
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
				if cfg.config == nil || *cfg.config != `{"model":"gpt-4"}` {
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
