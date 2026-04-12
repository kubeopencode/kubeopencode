// Copyright Contributors to the KubeOpenCode project

//go:build !integration

package controller

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

func TestConfigHasPermission(t *testing.T) {
	tests := []struct {
		name   string
		config *string
		want   bool
	}{
		{
			name:   "nil config",
			config: nil,
			want:   false,
		},
		{
			name:   "empty config",
			config: stringPtr(""),
			want:   false,
		},
		{
			name:   "config with permission field",
			config: stringPtr(`{"permission": "ask", "model": "gpt-4"}`),
			want:   true,
		},
		{
			name:   "config without permission field",
			config: stringPtr(`{"model": "gpt-4", "small_model": "gpt-3.5"}`),
			want:   false,
		},
		{
			name:   "invalid JSON",
			config: stringPtr(`{invalid json`),
			want:   false,
		},
		{
			name:   "permission field is null",
			config: stringPtr(`{"permission": null, "model": "gpt-4"}`),
			want:   true, // field exists even if value is null
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configHasPermission(tt.config)
			if got != tt.want {
				t.Errorf("configHasPermission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildServerDeployment_WithCredentials(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	envName := "GITHUB_TOKEN"
	mountPath := "/home/agent/.ssh/id_rsa"

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
		credentials: []kubeopenv1alpha1.Credential{
			{
				Name: "github-token",
				SecretRef: kubeopenv1alpha1.SecretReference{
					Name: "github-secret",
					Key:  stringPtr("token"),
				},
				Env: &envName,
			},
			{
				Name: "ssh-key",
				SecretRef: kubeopenv1alpha1.SecretReference{
					Name: "ssh-secret",
					Key:  stringPtr("private-key"),
				},
				MountPath: &mountPath,
			},
		},
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Verify env credential
	var foundEnvCred bool
	for _, env := range container.Env {
		if env.Name == "GITHUB_TOKEN" {
			foundEnvCred = true
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Errorf("GITHUB_TOKEN env should have SecretKeyRef")
			} else {
				if env.ValueFrom.SecretKeyRef.Name != "github-secret" {
					t.Errorf("SecretKeyRef.Name = %q, want %q", env.ValueFrom.SecretKeyRef.Name, "github-secret")
				}
				if env.ValueFrom.SecretKeyRef.Key != "token" {
					t.Errorf("SecretKeyRef.Key = %q, want %q", env.ValueFrom.SecretKeyRef.Key, "token")
				}
			}
		}
	}
	if !foundEnvCred {
		t.Errorf("GITHUB_TOKEN env not found")
	}

	// Verify mount credential
	var foundMountCred bool
	for _, mount := range container.VolumeMounts {
		if mount.MountPath == "/home/agent/.ssh/id_rsa" {
			foundMountCred = true
		}
	}
	if !foundMountCred {
		t.Errorf("SSH key mount not found at /home/agent/.ssh/id_rsa")
	}

	// Verify volume exists
	var foundVolume bool
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Secret != nil && vol.Secret.SecretName == "ssh-secret" {
			foundVolume = true
		}
	}
	if !foundVolume {
		t.Errorf("Secret volume for ssh-secret not found")
	}
}

func TestBuildServerDeployment_WithEntireSecretCredential(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
		credentials: []kubeopenv1alpha1.Credential{
			{
				// No Key specified - mount entire secret as env vars
				Name: "api-keys",
				SecretRef: kubeopenv1alpha1.SecretReference{
					Name: "api-credentials",
				},
			},
		},
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Verify envFrom is set with secretRef
	if len(container.EnvFrom) != 1 {
		t.Fatalf("Expected 1 envFrom entry, got %d", len(container.EnvFrom))
	}

	envFrom := container.EnvFrom[0]
	if envFrom.SecretRef == nil {
		t.Errorf("EnvFrom.SecretRef should not be nil")
	} else if envFrom.SecretRef.Name != "api-credentials" {
		t.Errorf("EnvFrom.SecretRef.Name = %q, want %q", envFrom.SecretRef.Name, "api-credentials")
	}
}

func TestBuildServerDeployment_WithHOMEAndSHELLEnvVars(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Verify HOME env var
	var foundHOME bool
	for _, env := range container.Env {
		if env.Name == "HOME" {
			foundHOME = true
			if env.Value != DefaultHomeDir {
				t.Errorf("HOME = %q, want %q", env.Value, DefaultHomeDir)
			}
		}
	}
	if !foundHOME {
		t.Errorf("HOME env var not found")
	}

	// Verify SHELL env var
	var foundSHELL bool
	for _, env := range container.Env {
		if env.Name == "SHELL" {
			foundSHELL = true
			if env.Value != DefaultShell {
				t.Errorf("SHELL = %q, want %q", env.Value, DefaultShell)
			}
		}
	}
	if !foundSHELL {
		t.Errorf("SHELL env var not found")
	}
}

func TestBuildServerDeployment_WithTextContext(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
	}

	contextConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent-server-context",
			Namespace: "default",
		},
		Data: map[string]string{
			"workspace-.kubeopencode-context.md": "<context>test content</context>",
		},
	}

	fileMounts := []fileMount{
		{filePath: "/workspace/.kubeopencode/context.md"},
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), contextConfigMap, fileMounts, nil, nil, nil)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	// Verify context-files volume exists
	var foundContextVolume bool
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == "context-files" && vol.ConfigMap != nil {
			foundContextVolume = true
			if vol.ConfigMap.Name != "test-agent-server-context" {
				t.Errorf("context-files volume ConfigMap.Name = %q, want %q", vol.ConfigMap.Name, "test-agent-server-context")
			}
		}
	}
	if !foundContextVolume {
		t.Errorf("context-files volume not found")
	}

	// Verify context-init container exists
	var foundContextInit bool
	for _, ic := range deployment.Spec.Template.Spec.InitContainers {
		if ic.Name == "context-init" {
			foundContextInit = true
		}
	}
	if !foundContextInit {
		t.Errorf("context-init init container not found")
	}

	// Verify OPENCODE_CONFIG_CONTENT env var is set
	container := deployment.Spec.Template.Spec.Containers[0]
	var foundConfigContentEnv bool
	for _, env := range container.Env {
		if env.Name == OpenCodeConfigContentEnvVar {
			foundConfigContentEnv = true
			expectedValue := `{"instructions":["` + ContextFileRelPath + `"]}`
			if env.Value != expectedValue {
				t.Errorf("OPENCODE_CONFIG_CONTENT = %q, want %q", env.Value, expectedValue)
			}
		}
	}
	if !foundConfigContentEnv {
		t.Errorf("OPENCODE_CONFIG_CONTENT env var not found")
	}
}

func TestBuildServerDeployment_WithConfigMapContext(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
	}

	dirMounts := []dirMount{
		{
			dirPath:       "/workspace/guides",
			configMapName: "guides-configmap",
			optional:      true,
		},
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, dirMounts, nil, nil)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	// Verify dir-mount volume exists
	var foundDirVolume bool
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == "dir-mount-0" && vol.ConfigMap != nil {
			foundDirVolume = true
			if vol.ConfigMap.Name != "guides-configmap" {
				t.Errorf("dir-mount-0 volume ConfigMap.Name = %q, want %q", vol.ConfigMap.Name, "guides-configmap")
			}
			if vol.ConfigMap.Optional == nil || *vol.ConfigMap.Optional != true {
				t.Errorf("dir-mount-0 volume ConfigMap.Optional = %v, want true", vol.ConfigMap.Optional)
			}
		}
	}
	if !foundDirVolume {
		t.Errorf("dir-mount-0 volume not found")
	}

	// Verify context-init container exists and mounts the ConfigMap
	var foundContextInit bool
	for _, ic := range deployment.Spec.Template.Spec.InitContainers {
		if ic.Name == "context-init" {
			foundContextInit = true
			// Verify init container mounts the dir-mount ConfigMap
			var foundDirMount bool
			for _, mount := range ic.VolumeMounts {
				if mount.Name == "dir-mount-0" && mount.MountPath == "/configmap-dir-0" {
					foundDirMount = true
				}
			}
			if !foundDirMount {
				t.Errorf("context-init container should mount dir-mount-0 ConfigMap at /configmap-dir-0")
			}
		}
	}
	if !foundContextInit {
		t.Errorf("context-init init container not found")
	}
}

func TestBuildServerDeployment_WithGitContext(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
	}

	gitMounts := []gitMount{
		{
			contextName: "my-context",
			repository:  "https://github.com/org/repo.git",
			ref:         "main",
			repoPath:    "",
			mountPath:   "/workspace/repo",
			depth:       1,
			secretName:  "",
		},
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, gitMounts, nil)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	// Verify git-context volume exists
	var foundGitVolume bool
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == "git-context-0" && vol.EmptyDir != nil {
			foundGitVolume = true
		}
	}
	if !foundGitVolume {
		t.Errorf("git-context-0 emptyDir volume not found")
	}

	// Verify git-init container exists (should be after opencode-init)
	var foundGitInit bool
	for _, ic := range deployment.Spec.Template.Spec.InitContainers {
		if ic.Name == "git-init-0" {
			foundGitInit = true
			// Verify environment variables
			envMap := make(map[string]string)
			for _, env := range ic.Env {
				envMap[env.Name] = env.Value
			}
			if envMap["GIT_REPO"] != "https://github.com/org/repo.git" {
				t.Errorf("GIT_REPO = %q, want %q", envMap["GIT_REPO"], "https://github.com/org/repo.git")
			}
			if envMap["GIT_REF"] != "main" {
				t.Errorf("GIT_REF = %q, want %q", envMap["GIT_REF"], "main")
			}
		}
	}
	if !foundGitInit {
		t.Errorf("git-init-0 init container not found")
	}

	// Verify GIT_CONFIG_GLOBAL env var is set
	container := deployment.Spec.Template.Spec.Containers[0]
	var foundGitConfigGlobal bool
	for _, env := range container.Env {
		if env.Name == "GIT_CONFIG_GLOBAL" {
			foundGitConfigGlobal = true
			expectedValue := DefaultGitRoot + "/.gitconfig"
			if env.Value != expectedValue {
				t.Errorf("GIT_CONFIG_GLOBAL = %q, want %q", env.Value, expectedValue)
			}
		}
	}
	if !foundGitConfigGlobal {
		t.Errorf("GIT_CONFIG_GLOBAL env var not found")
	}
}

func TestBuildServerDeployment_SkipsOPENCODE_PERMISSIONWhenConfigHasPermission(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	configWithPermission := `{"permission": "ask", "model": "gpt-4"}`
	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
		config:        &configWithPermission,
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Verify OPENCODE_PERMISSION env var is NOT set
	for _, env := range container.Env {
		if env.Name == OpenCodePermissionEnvVar {
			t.Errorf("OPENCODE_PERMISSION should not be set when config has permission field")
		}
	}
}

func TestBuildServerDeployment_SetsOPENCODE_PERMISSIONWhenConfigHasNoPermission(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	configWithoutPermission := `{"model": "gpt-4"}`
	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
		config:        &configWithoutPermission,
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Verify OPENCODE_PERMISSION env var is set
	var foundPermissionEnv bool
	for _, env := range container.Env {
		if env.Name == OpenCodePermissionEnvVar {
			foundPermissionEnv = true
			if env.Value != DefaultOpenCodePermission {
				t.Errorf("OPENCODE_PERMISSION = %q, want %q", env.Value, DefaultOpenCodePermission)
			}
		}
	}
	if !foundPermissionEnv {
		t.Errorf("OPENCODE_PERMISSION env var should be set when config has no permission field")
	}
}

func TestBuildServerDeploymentWithProxy(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
		proxy: &kubeopenv1alpha1.ProxyConfig{
			HttpProxy:  "http://proxy:8080",
			HttpsProxy: "http://proxy:8443",
			NoProxy:    "localhost,127.0.0.1",
		},
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	hasProxyEnv := func(envs []corev1.EnvVar) bool {
		for _, env := range envs {
			if env.Name == "HTTP_PROXY" || env.Name == "HTTPS_PROXY" {
				return true
			}
		}
		return false
	}

	// Verify all init containers have proxy env vars
	for _, ic := range deployment.Spec.Template.Spec.InitContainers {
		if !hasProxyEnv(ic.Env) {
			t.Errorf("init container %q missing proxy env vars", ic.Name)
		}
	}

	// Verify main container has proxy env vars
	container := deployment.Spec.Template.Spec.Containers[0]
	if !hasProxyEnv(container.Env) {
		t.Errorf("server container missing proxy env vars")
	}

	// Verify NO_PROXY includes .svc,.cluster.local
	for _, env := range container.Env {
		if env.Name == "NO_PROXY" {
			if env.Value != "localhost,127.0.0.1,.svc,.cluster.local" {
				t.Errorf("NO_PROXY = %q, want %q", env.Value, "localhost,127.0.0.1,.svc,.cluster.local")
			}
		}
	}
}

func TestBuildServerDeploymentWithImagePullSecrets(t *testing.T) {
	tests := []struct {
		name             string
		imagePullSecrets []corev1.LocalObjectReference
		wantCount        int
	}{
		{
			name: "single imagePullSecret",
			imagePullSecrets: []corev1.LocalObjectReference{
				{Name: "my-registry-secret"},
			},
			wantCount: 1,
		},
		{
			name: "multiple imagePullSecrets",
			imagePullSecrets: []corev1.LocalObjectReference{
				{Name: "harbor-secret"},
				{Name: "gcr-secret"},
			},
			wantCount: 2,
		},
		{
			name:             "empty list - no imagePullSecrets",
			imagePullSecrets: nil,
			wantCount:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "default",
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					Port: 4096,
				},
			}

			cfg := agentConfig{
				executorImage:    "test-executor:v1.0.0",
				agentImage:       "test-agent:v1.0.0",
				workspaceDir:     "/workspace",
				imagePullSecrets: tt.imagePullSecrets,
			}

			deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)

			if deployment == nil {
				t.Fatal("BuildServerDeployment returned nil")
			}

			podSpec := deployment.Spec.Template.Spec

			if tt.wantCount == 0 {
				if len(podSpec.ImagePullSecrets) != 0 {
					t.Errorf("ImagePullSecrets count = %d, want 0", len(podSpec.ImagePullSecrets))
				}
				return
			}

			if len(podSpec.ImagePullSecrets) != tt.wantCount {
				t.Fatalf("ImagePullSecrets count = %d, want %d", len(podSpec.ImagePullSecrets), tt.wantCount)
			}

			for i, secret := range tt.imagePullSecrets {
				if podSpec.ImagePullSecrets[i].Name != secret.Name {
					t.Errorf("ImagePullSecrets[%d].Name = %q, want %q", i, podSpec.ImagePullSecrets[i].Name, secret.Name)
				}
			}
		})
	}
}

func TestBuildServerDeploymentWithSecurityContext(t *testing.T) {
	t.Run("default security context applied", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-agent",
				Namespace: "default",
			},
			Spec: kubeopenv1alpha1.AgentSpec{
				Port: 4096,
			},
		}

		cfg := agentConfig{
			executorImage: "test-executor:v1.0.0",
			agentImage:    "test-agent:v1.0.0",
			workspaceDir:  "/workspace",
		}

		deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)

		if deployment == nil {
			t.Fatal("BuildServerDeployment returned nil")
		}

		container := deployment.Spec.Template.Spec.Containers[0]
		if container.SecurityContext == nil {
			t.Fatal("server container SecurityContext should not be nil")
		}
		if container.SecurityContext.AllowPrivilegeEscalation == nil || *container.SecurityContext.AllowPrivilegeEscalation != false {
			t.Errorf("AllowPrivilegeEscalation should be false")
		}
		if container.SecurityContext.Capabilities == nil || len(container.SecurityContext.Capabilities.Drop) != 1 || container.SecurityContext.Capabilities.Drop[0] != "ALL" {
			t.Errorf("Capabilities.Drop should be [ALL]")
		}

		// Init containers should also have default security context
		for _, ic := range deployment.Spec.Template.Spec.InitContainers {
			if ic.SecurityContext == nil {
				t.Errorf("init container %q SecurityContext should not be nil", ic.Name)
				continue
			}
			if ic.SecurityContext.AllowPrivilegeEscalation == nil || *ic.SecurityContext.AllowPrivilegeEscalation != false {
				t.Errorf("init container %q AllowPrivilegeEscalation should be false", ic.Name)
			}
		}
	})

	t.Run("custom security context on server container", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-agent",
				Namespace: "default",
			},
			Spec: kubeopenv1alpha1.AgentSpec{
				Port: 4096,
			},
		}

		runAsNonRoot := true
		cfg := agentConfig{
			executorImage: "test-executor:v1.0.0",
			agentImage:    "test-agent:v1.0.0",
			workspaceDir:  "/workspace",
			podSpec: &kubeopenv1alpha1.AgentPodSpec{
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot: &runAsNonRoot,
				},
			},
		}

		deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)

		if deployment == nil {
			t.Fatal("BuildServerDeployment returned nil")
		}

		container := deployment.Spec.Template.Spec.Containers[0]
		if container.SecurityContext == nil {
			t.Fatal("server container SecurityContext should not be nil")
		}
		if container.SecurityContext.RunAsNonRoot == nil || *container.SecurityContext.RunAsNonRoot != true {
			t.Errorf("custom SecurityContext RunAsNonRoot should be true")
		}

		// Init containers should still use default security context
		for _, ic := range deployment.Spec.Template.Spec.InitContainers {
			if ic.SecurityContext == nil {
				t.Errorf("init container %q SecurityContext should not be nil", ic.Name)
				continue
			}
			if ic.SecurityContext.AllowPrivilegeEscalation == nil || *ic.SecurityContext.AllowPrivilegeEscalation != false {
				t.Errorf("init container %q should use default AllowPrivilegeEscalation=false", ic.Name)
			}
		}
	})
}

func TestBuildServerDeploymentWithCABundle(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
		caBundle: &kubeopenv1alpha1.CABundleConfig{
			ConfigMapRef: &kubeopenv1alpha1.CABundleReference{
				Name: "corp-ca-bundle",
				Key:  "ca-bundle.crt",
			},
		},
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	// Verify CA bundle volume exists
	var foundCAVolume bool
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == CABundleVolumeName {
			foundCAVolume = true
			if vol.ConfigMap == nil {
				t.Fatalf("CA bundle volume should have ConfigMap source")
			}
			if vol.ConfigMap.Name != "corp-ca-bundle" {
				t.Errorf("CA volume ConfigMap.Name = %q, want %q", vol.ConfigMap.Name, "corp-ca-bundle")
			}
		}
	}
	if !foundCAVolume {
		t.Fatalf("CA bundle volume %q not found", CABundleVolumeName)
	}

	// Verify all init containers have the CA mount and env
	for _, ic := range deployment.Spec.Template.Spec.InitContainers {
		var hasCAMount bool
		for _, vm := range ic.VolumeMounts {
			if vm.Name == CABundleVolumeName && vm.MountPath == CABundleMountPath && vm.ReadOnly {
				hasCAMount = true
			}
		}
		if !hasCAMount {
			t.Errorf("init container %q missing CA bundle volume mount", ic.Name)
		}

		var hasCAEnv bool
		for _, env := range ic.Env {
			if env.Name == CustomCACertEnvVar && env.Value == CABundleMountPath+"/"+CABundleFileName {
				hasCAEnv = true
			}
		}
		if !hasCAEnv {
			t.Errorf("init container %q missing %s env var", ic.Name, CustomCACertEnvVar)
		}
	}

	// Verify server container has the CA mount and env
	container := deployment.Spec.Template.Spec.Containers[0]
	var hasCAMount bool
	for _, vm := range container.VolumeMounts {
		if vm.Name == CABundleVolumeName && vm.MountPath == CABundleMountPath && vm.ReadOnly {
			hasCAMount = true
		}
	}
	if !hasCAMount {
		t.Errorf("server container missing CA bundle volume mount")
	}

	var hasCAEnv bool
	for _, env := range container.Env {
		if env.Name == CustomCACertEnvVar && env.Value == CABundleMountPath+"/"+CABundleFileName {
			hasCAEnv = true
		}
	}
	if !hasCAEnv {
		t.Errorf("server container missing %s env var", CustomCACertEnvVar)
	}
}

func TestBuildServerSessionPVC(t *testing.T) {
	storageClass := "gp3"

	tests := []struct {
		name             string
		persistence      *kubeopenv1alpha1.PersistenceConfig
		wantNil          bool
		wantSize         string
		wantStorageClass *string
	}{
		{
			name:        "no persistence",
			persistence: nil,
			wantNil:     true,
		},
		{
			name:        "persistence with nil sessions",
			persistence: &kubeopenv1alpha1.PersistenceConfig{},
			wantNil:     true,
		},
		{
			name: "sessions with defaults",
			persistence: &kubeopenv1alpha1.PersistenceConfig{
				Sessions: &kubeopenv1alpha1.VolumePersistence{},
			},
			wantNil:  false,
			wantSize: DefaultSessionPVCSize,
		},
		{
			name: "sessions with custom size and storage class",
			persistence: &kubeopenv1alpha1.PersistenceConfig{
				Sessions: &kubeopenv1alpha1.VolumePersistence{
					Size:             "5Gi",
					StorageClassName: &storageClass,
				},
			},
			wantNil:          false,
			wantSize:         "5Gi",
			wantStorageClass: &storageClass,
		},
	}

	// Test invalid size returns error instead of panicking
	t.Run("invalid size returns error", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-agent",
				Namespace: "default",
			},
			Spec: kubeopenv1alpha1.AgentSpec{
				Port: 4096,
				Persistence: &kubeopenv1alpha1.PersistenceConfig{
					Sessions: &kubeopenv1alpha1.VolumePersistence{
						Size: "invalid-size",
					},
				},
			},
		}
		pvc, err := BuildServerSessionPVC(agent)
		if err == nil {
			t.Fatal("expected error for invalid size, got nil")
		}
		if pvc != nil {
			t.Fatal("expected nil PVC on error")
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-agent",
					Namespace: "default",
				},
				Spec: kubeopenv1alpha1.AgentSpec{
					Port:        4096,
					Persistence: tt.persistence,
				},
			}

			pvc, err := BuildServerSessionPVC(agent)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantNil {
				if pvc != nil {
					t.Fatalf("expected nil PVC, got %v", pvc)
				}
				return
			}

			if pvc == nil {
				t.Fatal("expected non-nil PVC")
			}

			// Verify name
			expectedName := "test-agent" + ServerSessionPVCSuffix
			if pvc.Name != expectedName {
				t.Errorf("PVC name = %q, want %q", pvc.Name, expectedName)
			}

			// Verify namespace
			if pvc.Namespace != "default" {
				t.Errorf("PVC namespace = %q, want %q", pvc.Namespace, "default")
			}

			// Verify access mode
			if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
				t.Errorf("PVC access modes = %v, want [ReadWriteOnce]", pvc.Spec.AccessModes)
			}

			// Verify size
			storageReq := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			if storageReq.String() != tt.wantSize {
				t.Errorf("PVC size = %q, want %q", storageReq.String(), tt.wantSize)
			}

			// Verify storage class
			if tt.wantStorageClass != nil {
				if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != *tt.wantStorageClass {
					t.Errorf("PVC storageClassName = %v, want %q", pvc.Spec.StorageClassName, *tt.wantStorageClass)
				}
			} else {
				if pvc.Spec.StorageClassName != nil {
					t.Errorf("PVC storageClassName = %q, want nil", *pvc.Spec.StorageClassName)
				}
			}

			// Verify labels
			if pvc.Labels[AgentLabelKey] != "test-agent" {
				t.Errorf("PVC agent label = %q, want %q", pvc.Labels[AgentLabelKey], "test-agent")
			}
		})
	}
}

func TestBuildServerDeployment_WithSessionPersistence(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
			Persistence: &kubeopenv1alpha1.PersistenceConfig{
				Sessions: &kubeopenv1alpha1.VolumePersistence{
					Size: "2Gi",
				},
			},
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)
	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	// Verify session PVC volume exists
	var foundVolume bool
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == ServerSessionVolumeName {
			foundVolume = true
			if vol.PersistentVolumeClaim == nil {
				t.Error("session volume should be a PVC")
			} else if vol.PersistentVolumeClaim.ClaimName != ServerSessionPVCName("test-agent") {
				t.Errorf("PVC claim name = %q, want %q", vol.PersistentVolumeClaim.ClaimName, ServerSessionPVCName("test-agent"))
			}
		}
	}
	if !foundVolume {
		t.Error("session PVC volume not found")
	}

	// Verify session volume mount exists on server container
	container := deployment.Spec.Template.Spec.Containers[0]
	var foundMount bool
	for _, mount := range container.VolumeMounts {
		if mount.Name == ServerSessionVolumeName {
			foundMount = true
			if mount.MountPath != ServerSessionMountPath {
				t.Errorf("mount path = %q, want %q", mount.MountPath, ServerSessionMountPath)
			}
		}
	}
	if !foundMount {
		t.Error("session volume mount not found on server container")
	}

	// Verify OPENCODE_DB env var
	var foundEnv bool
	for _, env := range container.Env {
		if env.Name == OpenCodeDBEnvVar {
			foundEnv = true
			if env.Value != ServerSessionDBPath {
				t.Errorf("%s = %q, want %q", OpenCodeDBEnvVar, env.Value, ServerSessionDBPath)
			}
		}
	}
	if !foundEnv {
		t.Errorf("%s env var not found", OpenCodeDBEnvVar)
	}
}

func TestBuildServerDeployment_WithoutSessionPersistence(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)
	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	// Verify no session PVC volume
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == ServerSessionVolumeName {
			t.Error("session PVC volume should not be present without persistence config")
		}
	}

	// Verify no OPENCODE_DB env var
	container := deployment.Spec.Template.Spec.Containers[0]
	for _, env := range container.Env {
		if env.Name == OpenCodeDBEnvVar {
			t.Errorf("%s env var should not be present without persistence config", OpenCodeDBEnvVar)
		}
	}
}

func TestBuildServerWorkspacePVC(t *testing.T) {
	storageClass := "gp3"

	tests := []struct {
		name             string
		persistence      *kubeopenv1alpha1.PersistenceConfig
		wantNil          bool
		wantSize         string
		wantStorageClass *string
	}{
		{
			name:        "no persistence",
			persistence: nil,
			wantNil:     true,
		},
		{
			name:        "persistence with nil workspace",
			persistence: &kubeopenv1alpha1.PersistenceConfig{},
			wantNil:     true,
		},
		{
			name: "workspace with defaults",
			persistence: &kubeopenv1alpha1.PersistenceConfig{
				Workspace: &kubeopenv1alpha1.VolumePersistence{},
			},
			wantNil:  false,
			wantSize: DefaultWorkspacePVCSize,
		},
		{
			name: "workspace with custom size and storage class",
			persistence: &kubeopenv1alpha1.PersistenceConfig{
				Workspace: &kubeopenv1alpha1.VolumePersistence{
					Size:             "50Gi",
					StorageClassName: &storageClass,
				},
			},
			wantNil:          false,
			wantSize:         "50Gi",
			wantStorageClass: &storageClass,
		},
	}

	t.Run("invalid size returns error", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{Name: "test-agent", Namespace: "default"},
			Spec: kubeopenv1alpha1.AgentSpec{
				Port: 4096,
				Persistence: &kubeopenv1alpha1.PersistenceConfig{
					Workspace: &kubeopenv1alpha1.VolumePersistence{Size: "invalid"},
				},
			},
		}
		pvc, err := BuildServerWorkspacePVC(agent)
		if err == nil {
			t.Fatal("expected error for invalid size")
		}
		if pvc != nil {
			t.Fatal("expected nil PVC on error")
		}
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{Name: "test-agent", Namespace: "default"},
				Spec: kubeopenv1alpha1.AgentSpec{
					Port:        4096,
					Persistence: tt.persistence,
				},
			}

			pvc, err := BuildServerWorkspacePVC(agent)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if pvc != nil {
					t.Fatalf("expected nil PVC, got %v", pvc)
				}
				return
			}
			if pvc == nil {
				t.Fatal("expected non-nil PVC")
			}

			expectedName := "test-agent" + ServerWorkspacePVCSuffix
			if pvc.Name != expectedName {
				t.Errorf("PVC name = %q, want %q", pvc.Name, expectedName)
			}
			if len(pvc.Spec.AccessModes) != 1 || pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
				t.Errorf("PVC access modes = %v, want [ReadWriteOnce]", pvc.Spec.AccessModes)
			}
			storageReq := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
			if storageReq.String() != tt.wantSize {
				t.Errorf("PVC size = %q, want %q", storageReq.String(), tt.wantSize)
			}
			if tt.wantStorageClass != nil {
				if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != *tt.wantStorageClass {
					t.Errorf("PVC storageClassName = %v, want %q", pvc.Spec.StorageClassName, *tt.wantStorageClass)
				}
			}
		})
	}
}

func TestBuildServerDeployment_WithWorkspacePersistence(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-agent", Namespace: "default"},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
			Persistence: &kubeopenv1alpha1.PersistenceConfig{
				Workspace: &kubeopenv1alpha1.VolumePersistence{Size: "20Gi"},
			},
		},
	}
	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)
	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	// Verify workspace volume is a PVC (not EmptyDir)
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == WorkspaceVolumeName {
			if vol.PersistentVolumeClaim == nil {
				t.Error("workspace volume should be a PVC when persistence is configured")
			} else if vol.PersistentVolumeClaim.ClaimName != ServerWorkspacePVCName("test-agent") {
				t.Errorf("workspace PVC claim = %q, want %q", vol.PersistentVolumeClaim.ClaimName, ServerWorkspacePVCName("test-agent"))
			}
			if vol.EmptyDir != nil {
				t.Error("workspace volume should not be EmptyDir when persistence is configured")
			}
			return
		}
	}
	t.Error("workspace volume not found")
}

func TestBuildServerDeployment_SuspendedAgentStillBuildsDeployment(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-agent", Namespace: "default"},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port:    4096,
			Suspend: true,
		},
	}
	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)
	if deployment == nil {
		t.Fatal("BuildServerDeployment should return non-nil even when suspended")
	}
	// BuildServerDeployment always sets replicas=1; controller overrides for suspend
	if *deployment.Spec.Replicas != 1 {
		t.Errorf("replicas = %d, want 1", *deployment.Spec.Replicas)
	}
}

func TestBuildServerDeployment_WithoutWorkspacePersistence(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Name: "test-agent", Namespace: "default"},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}
	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)
	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	// Verify workspace volume is EmptyDir (not PVC)
	for _, vol := range deployment.Spec.Template.Spec.Volumes {
		if vol.Name == WorkspaceVolumeName {
			if vol.EmptyDir == nil {
				t.Error("workspace volume should be EmptyDir without persistence")
			}
			if vol.PersistentVolumeClaim != nil {
				t.Error("workspace volume should not be PVC without persistence")
			}
			return
		}
	}
	t.Error("workspace volume not found")
}

// TestBuildServerDeployment_WithGitSyncHotReload verifies that a git-sync sidecar
// container is added when a Git context has sync.policy=HotReload.
func TestBuildServerDeployment_WithGitSyncHotReload(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sync-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{Port: 4096},
	}
	cfg := agentConfig{
		executorImage: "test-executor",
		agentImage:    "test-agent",
		workspaceDir:  "/workspace",
	}

	gitMounts := []gitMount{
		{
			contextName:  "team-prompts",
			repository:   "https://github.com/org/prompts.git",
			ref:          "main",
			mountPath:    "/workspace/prompts",
			depth:        1,
			syncEnabled:  true,
			syncPolicy:   kubeopenv1alpha1.GitSyncPolicyHotReload,
			syncInterval: 5 * time.Minute,
		},
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, gitMounts, nil)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	// Should have 2 containers: main server + git-sync sidecar
	containers := deployment.Spec.Template.Spec.Containers
	if len(containers) != 2 {
		t.Fatalf("expected 2 containers, got %d", len(containers))
	}

	// Verify sidecar name and command
	sidecar := containers[1]
	if sidecar.Name != "git-sync-0" {
		t.Errorf("expected sidecar name 'git-sync-0', got %q", sidecar.Name)
	}
	if len(sidecar.Command) < 2 || sidecar.Command[1] != "git-sync" {
		t.Errorf("expected sidecar command to contain 'git-sync', got %v", sidecar.Command)
	}

	// Verify GIT_SYNC_INTERVAL env var
	envMap := make(map[string]string)
	for _, env := range sidecar.Env {
		envMap[env.Name] = env.Value
	}
	if envMap["GIT_SYNC_INTERVAL"] != "300" {
		t.Errorf("expected GIT_SYNC_INTERVAL=300, got %q", envMap["GIT_SYNC_INTERVAL"])
	}
	if envMap["GIT_REPO"] != "https://github.com/org/prompts.git" {
		t.Errorf("expected GIT_REPO to be set, got %q", envMap["GIT_REPO"])
	}

	// Verify sidecar shares the same volume as git-init
	foundVolume := false
	for _, vm := range sidecar.VolumeMounts {
		if vm.Name == "git-context-0" {
			foundVolume = true
		}
	}
	if !foundVolume {
		t.Error("sidecar should mount git-context-0 volume")
	}

	// Verify git-init init container also exists
	foundGitInit := false
	for _, ic := range deployment.Spec.Template.Spec.InitContainers {
		if ic.Name == "git-init-0" {
			foundGitInit = true
		}
	}
	if !foundGitInit {
		t.Error("git-init-0 init container should still exist alongside sidecar")
	}
}

// TestBuildServerDeployment_WithGitSyncRollout verifies that Rollout policy
// does NOT add a sidecar but DOES add pod template annotations.
func TestBuildServerDeployment_WithGitSyncRollout(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rollout-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{Port: 4096},
	}
	cfg := agentConfig{
		executorImage: "test-executor",
		agentImage:    "test-agent",
		workspaceDir:  "/workspace",
	}

	gitMounts := []gitMount{
		{
			contextName:  "agent-config",
			repository:   "https://github.com/org/config.git",
			ref:          "main",
			mountPath:    "/workspace/config",
			depth:        1,
			syncEnabled:  true,
			syncPolicy:   kubeopenv1alpha1.GitSyncPolicyRollout,
			syncInterval: 10 * time.Minute,
		},
	}

	gitHashAnnotations := map[string]string{
		"kubeopencode.io/git-hash-agent-config": "abc123def456",
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, gitMounts, gitHashAnnotations)

	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	// Rollout policy: should only have 1 container (no sidecar)
	containers := deployment.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Fatalf("expected 1 container (no sidecar for Rollout), got %d", len(containers))
	}

	// Should have pod template annotation with git hash
	annotations := deployment.Spec.Template.Annotations
	if annotations == nil {
		t.Fatal("expected pod template annotations, got nil")
	}
	hash, ok := annotations["kubeopencode.io/git-hash-agent-config"]
	if !ok {
		t.Error("expected git hash annotation on pod template")
	}
	if hash != "abc123def456" {
		t.Errorf("expected hash 'abc123def456', got %q", hash)
	}
}

func TestHashConfigMapData(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]string
		wantEmpty bool
	}{
		{
			name:      "nil map returns empty",
			data:      nil,
			wantEmpty: true,
		},
		{
			name:      "empty map returns empty",
			data:      map[string]string{},
			wantEmpty: true,
		},
		{
			name: "single entry produces hash",
			data: map[string]string{
				"config.json": `{"model":"test"}`,
			},
			wantEmpty: false,
		},
		{
			name: "multiple entries produce hash",
			data: map[string]string{
				"config.json": `{"model":"test"}`,
				"context.md":  "some context",
			},
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hashConfigMapData(tt.data)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("hashConfigMapData() = %q, want empty", got)
				}
				return
			}
			if got == "" {
				t.Error("hashConfigMapData() returned empty, want non-empty hash")
			}
			if len(got) != 16 {
				t.Errorf("hashConfigMapData() length = %d, want 16", len(got))
			}
		})
	}

	// Determinism: same data produces same hash
	t.Run("deterministic", func(t *testing.T) {
		data := map[string]string{
			"a": "1",
			"b": "2",
			"c": "3",
		}
		h1 := hashConfigMapData(data)
		h2 := hashConfigMapData(data)
		if h1 != h2 {
			t.Errorf("hashConfigMapData() not deterministic: %q != %q", h1, h2)
		}
	})

	// Different data produces different hash
	t.Run("different data produces different hash", func(t *testing.T) {
		data1 := map[string]string{"config.json": `{"model":"a"}`}
		data2 := map[string]string{"config.json": `{"model":"b"}`}
		h1 := hashConfigMapData(data1)
		h2 := hashConfigMapData(data2)
		if h1 == h2 {
			t.Errorf("hashConfigMapData() produced same hash for different data: %q", h1)
		}
	})

	// Key change produces different hash
	t.Run("different keys produce different hash", func(t *testing.T) {
		data1 := map[string]string{"key1": "value"}
		data2 := map[string]string{"key2": "value"}
		h1 := hashConfigMapData(data1)
		h2 := hashConfigMapData(data2)
		if h1 == h2 {
			t.Errorf("hashConfigMapData() produced same hash for different keys: %q", h1)
		}
	})
}

func TestBuildServerDeployment_ContextHashAnnotation(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ctx-hash",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1",
		agentImage:    "test-agent:v1",
		workspaceDir:  "/workspace",
	}
	sysCfg := systemConfig{}

	// Simulate what the agent controller does: compute hash and inject into annotations
	configMapData := map[string]string{
		"tools-opencode.json": `{"skills":{"paths":["/skills/official-skills/skill-creator","/skills/official-skills/frontend-design"]}}`,
	}
	contextConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ctx-hash-server-context",
			Namespace: "default",
		},
		Data: configMapData,
	}

	annotations := map[string]string{
		ContextHashAnnotationKey: hashConfigMapData(configMapData),
	}

	deployment := BuildServerDeployment(agent, cfg, sysCfg, contextConfigMap, nil, nil, nil, annotations)

	// Verify annotation is present on pod template
	podAnnotations := deployment.Spec.Template.Annotations
	if podAnnotations == nil {
		t.Fatal("expected pod template annotations, got nil")
	}
	hash, ok := podAnnotations[ContextHashAnnotationKey]
	if !ok {
		t.Error("expected context hash annotation on pod template")
	}
	if len(hash) != 16 {
		t.Errorf("expected 16-char hash, got %q (len=%d)", hash, len(hash))
	}

	// Now change the config content and verify hash changes
	configMapData2 := map[string]string{
		"tools-opencode.json": `{"skills":{"paths":["/skills/official-skills/skill-creator","/skills/official-skills/frontend-design","/skills/official-skills/doc-coauthoring"]}}`,
	}
	annotations2 := map[string]string{
		ContextHashAnnotationKey: hashConfigMapData(configMapData2),
	}

	deployment2 := BuildServerDeployment(agent, cfg, sysCfg, contextConfigMap, nil, nil, nil, annotations2)
	hash2 := deployment2.Spec.Template.Annotations[ContextHashAnnotationKey]

	if hash == hash2 {
		t.Error("context hash should differ when ConfigMap content changes")
	}
}

func TestBuildServerDeployment_SkillNamesPerNameMount(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1",
		agentImage:    "test-agent:v1",
		workspaceDir:  "/workspace",
	}

	t.Run("names creates per-name SubPath mounts", func(t *testing.T) {
		gitMounts := []gitMount{
			{
				contextName: "skill-official",
				repository:  "https://github.com/anthropics/skills.git",
				ref:         "main",
				repoPath:    "skills/",
				mountPath:   "/skills/official",
				depth:       1,
				names:       []string{"frontend-design", "webapp-testing"},
			},
		}

		deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, gitMounts, nil)
		container := deployment.Spec.Template.Spec.Containers[0]

		// Should NOT have a mount for the whole /skills/official directory
		for _, vm := range container.VolumeMounts {
			if vm.MountPath == "/skills/official" {
				t.Error("should not mount the entire /skills/official directory when names are specified")
			}
		}

		// Should have per-name mounts
		expectedMounts := map[string]string{
			"/skills/official/frontend-design": "repo/skills/frontend-design",
			"/skills/official/webapp-testing":  "repo/skills/webapp-testing",
		}
		for expectedPath, expectedSubPath := range expectedMounts {
			found := false
			for _, vm := range container.VolumeMounts {
				if vm.MountPath == expectedPath {
					found = true
					if vm.SubPath != expectedSubPath {
						t.Errorf("mount %q SubPath = %q, want %q", expectedPath, vm.SubPath, expectedSubPath)
					}
				}
			}
			if !found {
				t.Errorf("expected volume mount at %q not found", expectedPath)
			}
		}
	})

	t.Run("empty names mounts whole directory", func(t *testing.T) {
		gitMounts := []gitMount{
			{
				contextName: "skill-all",
				repository:  "https://github.com/org/skills.git",
				ref:         "main",
				repoPath:    "skills/",
				mountPath:   "/skills/all",
				depth:       1,
				names:       nil,
			},
		}

		deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, gitMounts, nil)
		container := deployment.Spec.Template.Spec.Containers[0]

		found := false
		for _, vm := range container.VolumeMounts {
			if vm.MountPath == "/skills/all" {
				found = true
				if vm.SubPath != "repo/skills/" {
					t.Errorf("SubPath = %q, want %q", vm.SubPath, "repo/skills/")
				}
			}
		}
		if !found {
			t.Error("expected volume mount at /skills/all not found")
		}
	})

	t.Run("names with empty repoPath", func(t *testing.T) {
		gitMounts := []gitMount{
			{
				contextName: "skill-root",
				repository:  "https://github.com/org/skills.git",
				ref:         "main",
				repoPath:    "",
				mountPath:   "/skills/root",
				depth:       1,
				names:       []string{"some-skill"},
			},
		}

		deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, gitMounts, nil)
		container := deployment.Spec.Template.Spec.Containers[0]

		found := false
		for _, vm := range container.VolumeMounts {
			if vm.MountPath == "/skills/root/some-skill" {
				found = true
				if vm.SubPath != "repo/some-skill" {
					t.Errorf("SubPath = %q, want %q", vm.SubPath, "repo/some-skill")
				}
			}
		}
		if !found {
			t.Error("expected volume mount at /skills/root/some-skill not found")
		}
	})
}

func TestBuildServerService_WithExtraPorts(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dind-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
			ExtraPorts: []kubeopenv1alpha1.ExtraPort{
				{Name: "webapp", Port: 3000},
				{Name: "vscode", Port: 8080, Protocol: corev1.ProtocolTCP},
			},
		},
	}

	service := BuildServerService(agent)
	if service == nil {
		t.Fatal("BuildServerService returned nil")
	}

	// Should have main port + 2 extra ports = 3 total
	if len(service.Spec.Ports) != 3 {
		t.Fatalf("expected 3 service ports, got %d", len(service.Spec.Ports))
	}

	// Main port
	if service.Spec.Ports[0].Name != "http" || service.Spec.Ports[0].Port != 4096 {
		t.Errorf("expected main port http:4096, got %s:%d", service.Spec.Ports[0].Name, service.Spec.Ports[0].Port)
	}
	if service.Spec.Ports[0].TargetPort.IntVal != 4096 {
		t.Errorf("expected main targetPort 4096, got %d", service.Spec.Ports[0].TargetPort.IntVal)
	}

	// Extra port 1
	if service.Spec.Ports[1].Name != "webapp" || service.Spec.Ports[1].Port != 3000 {
		t.Errorf("expected webapp:3000, got %s:%d", service.Spec.Ports[1].Name, service.Spec.Ports[1].Port)
	}
	if service.Spec.Ports[1].TargetPort.IntVal != 3000 {
		t.Errorf("expected webapp targetPort 3000, got %d", service.Spec.Ports[1].TargetPort.IntVal)
	}

	// Extra port 2
	if service.Spec.Ports[2].Name != "vscode" || service.Spec.Ports[2].Port != 8080 {
		t.Errorf("expected vscode:8080, got %s:%d", service.Spec.Ports[2].Name, service.Spec.Ports[2].Port)
	}
}

func TestBuildServerService_WithoutExtraPorts(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	service := BuildServerService(agent)
	if len(service.Spec.Ports) != 1 {
		t.Fatalf("expected 1 service port, got %d", len(service.Spec.Ports))
	}
	if service.Spec.Ports[0].Name != "http" || service.Spec.Ports[0].Port != 4096 {
		t.Errorf("expected main port http:4096, got %s:%d", service.Spec.Ports[0].Name, service.Spec.Ports[0].Port)
	}
}

func TestBuildServerService_ExtraPortDefaultProtocol(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proto-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
			ExtraPorts: []kubeopenv1alpha1.ExtraPort{
				{Name: "no-proto", Port: 9090},                               // no protocol specified
				{Name: "udp-port", Port: 5353, Protocol: corev1.ProtocolUDP}, // explicit UDP
			},
		},
	}

	service := BuildServerService(agent)
	if len(service.Spec.Ports) != 3 {
		t.Fatalf("expected 3 service ports, got %d", len(service.Spec.Ports))
	}

	// Default protocol should be TCP
	if service.Spec.Ports[1].Protocol != corev1.ProtocolTCP {
		t.Errorf("expected default protocol TCP for no-proto, got %s", service.Spec.Ports[1].Protocol)
	}

	// Explicit UDP should be preserved
	if service.Spec.Ports[2].Protocol != corev1.ProtocolUDP {
		t.Errorf("expected UDP protocol for udp-port, got %s", service.Spec.Ports[2].Protocol)
	}
}

func TestBuildServerDeployment_WithExtraPorts(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dind-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
			ExtraPorts: []kubeopenv1alpha1.ExtraPort{
				{Name: "webapp", Port: 3000},
				{Name: "vscode", Port: 8080, Protocol: corev1.ProtocolTCP},
				{Name: "metrics", Port: 9090, Protocol: corev1.ProtocolTCP},
			},
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
		extraPorts:    agent.Spec.ExtraPorts,
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)
	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Should have main port + 3 extra ports = 4 total
	if len(container.Ports) != 4 {
		t.Fatalf("expected 4 container ports, got %d", len(container.Ports))
	}

	// Main port
	if container.Ports[0].Name != "http" || container.Ports[0].ContainerPort != 4096 {
		t.Errorf("expected main port http:4096, got %s:%d", container.Ports[0].Name, container.Ports[0].ContainerPort)
	}

	// Extra ports
	expectedPorts := []struct {
		name     string
		port     int32
		protocol corev1.Protocol
	}{
		{"webapp", 3000, corev1.ProtocolTCP},
		{"vscode", 8080, corev1.ProtocolTCP},
		{"metrics", 9090, corev1.ProtocolTCP},
	}

	for i, expected := range expectedPorts {
		actual := container.Ports[i+1]
		if actual.Name != expected.name {
			t.Errorf("port[%d] name = %q, want %q", i+1, actual.Name, expected.name)
		}
		if actual.ContainerPort != expected.port {
			t.Errorf("port[%d] containerPort = %d, want %d", i+1, actual.ContainerPort, expected.port)
		}
		if actual.Protocol != expected.protocol {
			t.Errorf("port[%d] protocol = %q, want %q", i+1, actual.Protocol, expected.protocol)
		}
	}
}

func TestBuildServerDeployment_WithoutExtraPorts(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)
	container := deployment.Spec.Template.Spec.Containers[0]

	// Should have only the main port
	if len(container.Ports) != 1 {
		t.Fatalf("expected 1 container port, got %d", len(container.Ports))
	}
	if container.Ports[0].Name != "http" || container.Ports[0].ContainerPort != 4096 {
		t.Errorf("expected main port http:4096, got %s:%d", container.Ports[0].Name, container.Ports[0].ContainerPort)
	}
}

func TestBuildServerDeployment_WithLifecycle(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vscode-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
			PodSpec: &kubeopenv1alpha1.AgentPodSpec{
				Lifecycle: &corev1.Lifecycle{
					PostStart: &corev1.LifecycleHandler{
						Exec: &corev1.ExecAction{
							Command: []string{"/usr/local/bin/start-code-server.sh"},
						},
					},
				},
			},
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
		podSpec:       agent.Spec.PodSpec,
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)
	if deployment == nil {
		t.Fatal("BuildServerDeployment returned nil")
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	if container.Lifecycle == nil {
		t.Fatal("expected Lifecycle to be set on server container")
	}
	if container.Lifecycle.PostStart == nil {
		t.Fatal("expected PostStart to be set")
	}
	if container.Lifecycle.PostStart.Exec == nil {
		t.Fatal("expected PostStart.Exec to be set")
	}
	if len(container.Lifecycle.PostStart.Exec.Command) != 1 || container.Lifecycle.PostStart.Exec.Command[0] != "/usr/local/bin/start-code-server.sh" {
		t.Errorf("unexpected PostStart command: %v", container.Lifecycle.PostStart.Exec.Command)
	}
}

func TestBuildServerDeployment_WithoutLifecycle(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simple-agent",
			Namespace: "default",
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Port: 4096,
		},
	}

	cfg := agentConfig{
		executorImage: "test-executor:v1.0.0",
		agentImage:    "test-agent:v1.0.0",
		workspaceDir:  "/workspace",
	}

	deployment := BuildServerDeployment(agent, cfg, defaultSystemConfig(), nil, nil, nil, nil, nil)
	container := deployment.Spec.Template.Spec.Containers[0]

	if container.Lifecycle != nil {
		t.Errorf("expected Lifecycle to be nil when not configured, got %v", container.Lifecycle)
	}
}
