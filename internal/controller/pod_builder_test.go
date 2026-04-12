// Copyright Contributors to the KubeOpenCode project

//go:build !integration

package controller

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

// defaultSystemConfig returns a systemConfig with default values for testing.
func defaultSystemConfig() systemConfig {
	return systemConfig{
		systemImage:           DefaultKubeOpenCodeImage,
		systemImagePullPolicy: corev1.PullIfNotPresent,
	}
}

func TestSanitizeConfigMapKey(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{
			name:     "simple path",
			filePath: "/workspace/task.md",
			want:     "workspace-task.md",
		},
		{
			name:     "nested path",
			filePath: "/workspace/guides/standards.md",
			want:     "workspace-guides-standards.md",
		},
		{
			name:     "deeply nested path",
			filePath: "/home/agent/.config/settings.json",
			want:     "home-agent-.config-settings.json",
		},
		{
			name:     "no leading slash",
			filePath: "workspace/task.md",
			want:     "workspace-task.md",
		},
		{
			name:     "single file",
			filePath: "/task.md",
			want:     "task.md",
		},
		{
			name:     "empty string",
			filePath: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeConfigMapKey(tt.filePath)
			if got != tt.want {
				t.Errorf("sanitizeConfigMapKey(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestInferImagePullPolicy(t *testing.T) {
	tests := []struct {
		image string
		want  corev1.PullPolicy
	}{
		{"ghcr.io/kubeopencode/agent:latest", corev1.PullAlways},
		{"ghcr.io/kubeopencode/agent", corev1.PullAlways},
		{"ghcr.io/kubeopencode/agent:v0.0.18", corev1.PullIfNotPresent},
		{"ghcr.io/kubeopencode/agent:1.3.13", corev1.PullIfNotPresent},
		{"ghcr.io/kubeopencode/agent@sha256:abc123", corev1.PullIfNotPresent},
		{"registry.example.com:5000/image:latest", corev1.PullAlways},
		{"registry.example.com:5000/image", corev1.PullAlways},
		{"registry.example.com:5000/image:v1.0", corev1.PullIfNotPresent},
	}
	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			got := inferImagePullPolicy(tt.image)
			if got != tt.want {
				t.Errorf("inferImagePullPolicy(%q) = %v, want %v", tt.image, got, tt.want)
			}
		})
	}
}

func TestBoolPtr(t *testing.T) {
	trueVal := boolPtr(true)
	if trueVal == nil || *trueVal != true {
		t.Errorf("boolPtr(true) = %v, want *true", trueVal)
	}

	falseVal := boolPtr(false)
	if falseVal == nil || *falseVal != false {
		t.Errorf("boolPtr(false) = %v, want *false", falseVal)
	}
}

func TestGetParentDir(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     string
	}{
		{
			name:     "simple path",
			filePath: "/etc/github-app/script.sh",
			want:     "/etc/github-app",
		},
		{
			name:     "nested path",
			filePath: "/workspace/guides/standards.md",
			want:     "/workspace/guides",
		},
		{
			name:     "root level file",
			filePath: "/task.md",
			want:     "/",
		},
		{
			name:     "single file without path",
			filePath: "task.md",
			want:     "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getParentDir(tt.filePath)
			if got != tt.want {
				t.Errorf("getParentDir(%q) = %q, want %q", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestIsUnderPath(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		basePath string
		want     bool
	}{
		{
			name:     "file under workspace",
			filePath: "/workspace/task.md",
			basePath: "/workspace",
			want:     true,
		},
		{
			name:     "nested file under workspace",
			filePath: "/workspace/guides/readme.md",
			basePath: "/workspace",
			want:     true,
		},
		{
			name:     "file not under workspace",
			filePath: "/etc/github-app/script.sh",
			basePath: "/workspace",
			want:     false,
		},
		{
			name:     "similar prefix but different path",
			filePath: "/workspace-foo/task.md",
			basePath: "/workspace",
			want:     false,
		},
		{
			name:     "exact match",
			filePath: "/workspace",
			basePath: "/workspace",
			want:     true,
		},
		{
			name:     "basePath with trailing slash",
			filePath: "/workspace/task.md",
			basePath: "/workspace/",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUnderPath(tt.filePath, tt.basePath)
			if got != tt.want {
				t.Errorf("isUnderPath(%q, %q) = %v, want %v", tt.filePath, tt.basePath, got, tt.want)
			}
		})
	}
}

func TestSanitizeVolumeName(t *testing.T) {
	tests := []struct {
		name    string
		dirPath string
		want    string
	}{
		{
			name:    "simple path",
			dirPath: "/etc/github-app",
			want:    "ctx-etc-github-app",
		},
		{
			name:    "nested path",
			dirPath: "/home/user/.config",
			want:    "ctx-home-user-.config",
		},
		{
			name:    "uppercase path",
			dirPath: "/ETC/Config",
			want:    "ctx-etc-config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeVolumeName(tt.dirPath)
			if got != tt.want {
				t.Errorf("sanitizeVolumeName(%q) = %q, want %q", tt.dirPath, got, tt.want)
			}
		})
	}
}

func TestBuildPod_BasicTask(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	// Verify pod metadata
	if pod.Name != "test-task-pod" {
		t.Errorf("Pod.Name = %q, want %q", pod.Name, "test-task-pod")
	}
	if pod.Namespace != "default" {
		t.Errorf("Pod.Namespace = %q, want %q", pod.Namespace, "default")
	}

	// Verify labels
	if pod.Labels["app"] != "kubeopencode" {
		t.Errorf("Pod.Labels[app] = %q, want %q", pod.Labels["app"], "kubeopencode")
	}
	if pod.Labels["kubeopencode.io/task"] != "test-task" {
		t.Errorf("Pod.Labels[kubeopencode.io/task] = %q, want %q", pod.Labels["kubeopencode.io/task"], "test-task")
	}

	// Verify owner reference points to the Task
	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("len(Pod.OwnerReferences) = %d, want 1", len(pod.OwnerReferences))
	}
	if pod.OwnerReferences[0].Name != "test-task" {
		t.Errorf("Pod.OwnerReferences[0].Name = %q, want %q", pod.OwnerReferences[0].Name, "test-task")
	}
	if pod.OwnerReferences[0].Kind != "Task" {
		t.Errorf("Pod.OwnerReferences[0].Kind = %q, want %q", pod.OwnerReferences[0].Kind, "Task")
	}

	// Verify init containers (OpenCode init should be first)
	if len(pod.Spec.InitContainers) < 1 {
		t.Fatalf("len(InitContainers) = %d, want at least 1", len(pod.Spec.InitContainers))
	}
	initContainer := pod.Spec.InitContainers[0]
	if initContainer.Name != "opencode-init" {
		t.Errorf("InitContainer.Name = %q, want %q", initContainer.Name, "opencode-init")
	}
	if initContainer.Image != "test-opencode:v1.0.0" {
		t.Errorf("InitContainer.Image = %q, want %q", initContainer.Image, "test-opencode:v1.0.0")
	}

	// Verify init container mounts /tools volume
	var initHasToolsMount bool
	for _, vm := range initContainer.VolumeMounts {
		if vm.Name == "tools" && vm.MountPath == "/tools" {
			initHasToolsMount = true
			break
		}
	}
	if !initHasToolsMount {
		t.Errorf("InitContainer should mount /tools volume")
	}

	// Verify container (uses executorImage)
	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("len(Containers) = %d, want 1", len(pod.Spec.Containers))
	}
	container := pod.Spec.Containers[0]
	if container.Name != "agent" {
		t.Errorf("Container.Name = %q, want %q", container.Name, "agent")
	}
	if container.Image != "test-executor:v1.0.0" {
		t.Errorf("Container.Image = %q, want %q", container.Image, "test-executor:v1.0.0")
	}

	// Verify container mounts /tools volume
	var containerHasToolsMount bool
	for _, vm := range container.VolumeMounts {
		if vm.Name == "tools" && vm.MountPath == "/tools" {
			containerHasToolsMount = true
			break
		}
	}
	if !containerHasToolsMount {
		t.Errorf("Container should mount /tools volume")
	}

	// Verify environment variables
	envMap := make(map[string]string)
	for _, env := range container.Env {
		envMap[env.Name] = env.Value
	}
	if envMap["TASK_NAME"] != "test-task" {
		t.Errorf("Env[TASK_NAME] = %q, want %q", envMap["TASK_NAME"], "test-task")
	}
	if envMap["TASK_NAMESPACE"] != "default" {
		t.Errorf("Env[TASK_NAMESPACE] = %q, want %q", envMap["TASK_NAMESPACE"], "default")
	}
	if envMap["WORKSPACE_DIR"] != "/workspace" {
		t.Errorf("Env[WORKSPACE_DIR] = %q, want %q", envMap["WORKSPACE_DIR"], "/workspace")
	}

	// Verify service account
	if pod.Spec.ServiceAccountName != "test-sa" {
		t.Errorf("ServiceAccountName = %q, want %q", pod.Spec.ServiceAccountName, "test-sa")
	}

	// Verify restart policy
	if pod.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("RestartPolicy = %q, want %q", pod.Spec.RestartPolicy, corev1.RestartPolicyNever)
	}
}

// stringPtr returns a pointer to the given string value
func stringPtr(s string) *string {
	return &s
}

func TestBuildPod_WithCredentials(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	envName := "API_TOKEN"
	mountPath := "/home/agent/.ssh/id_rsa"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		credentials: []kubeopenv1alpha1.Credential{
			{
				Name: "api-token",
				SecretRef: kubeopenv1alpha1.SecretReference{
					Name: "my-secret",
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

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	container := pod.Spec.Containers[0]

	// Verify env credential
	var foundEnvCred bool
	for _, env := range container.Env {
		if env.Name == "API_TOKEN" {
			foundEnvCred = true
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Errorf("API_TOKEN env should have SecretKeyRef")
			} else {
				if env.ValueFrom.SecretKeyRef.Name != "my-secret" {
					t.Errorf("SecretKeyRef.Name = %q, want %q", env.ValueFrom.SecretKeyRef.Name, "my-secret")
				}
				if env.ValueFrom.SecretKeyRef.Key != "token" {
					t.Errorf("SecretKeyRef.Key = %q, want %q", env.ValueFrom.SecretKeyRef.Key, "token")
				}
			}
		}
	}
	if !foundEnvCred {
		t.Errorf("API_TOKEN env not found")
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
	for _, vol := range pod.Spec.Volumes {
		if vol.Secret != nil && vol.Secret.SecretName == "ssh-secret" {
			foundVolume = true
		}
	}
	if !foundVolume {
		t.Errorf("Secret volume for ssh-secret not found")
	}
}

func TestBuildPod_WithEntireSecretCredential(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		credentials: []kubeopenv1alpha1.Credential{
			{
				// No Key specified - mount entire secret as env vars
				Name: "api-keys",
				SecretRef: kubeopenv1alpha1.SecretReference{
					Name: "api-credentials",
					// Key is nil - entire secret should be mounted
				},
			},
		},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	container := pod.Spec.Containers[0]

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

func TestBuildPod_WithMixedCredentials(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	envName := "GITHUB_TOKEN"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		credentials: []kubeopenv1alpha1.Credential{
			{
				// Entire secret mount (no key)
				Name: "all-api-keys",
				SecretRef: kubeopenv1alpha1.SecretReference{
					Name: "api-credentials",
				},
			},
			{
				// Single key mount with env rename
				Name: "github-token",
				SecretRef: kubeopenv1alpha1.SecretReference{
					Name: "github-secret",
					Key:  stringPtr("token"),
				},
				Env: &envName,
			},
		},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	container := pod.Spec.Containers[0]

	// Verify envFrom has 1 entry (entire secret)
	if len(container.EnvFrom) != 1 {
		t.Fatalf("Expected 1 envFrom entry, got %d", len(container.EnvFrom))
	}
	if container.EnvFrom[0].SecretRef.Name != "api-credentials" {
		t.Errorf("EnvFrom.SecretRef.Name = %q, want %q", container.EnvFrom[0].SecretRef.Name, "api-credentials")
	}

	// Verify env has GITHUB_TOKEN from single key mount
	var foundGithubToken bool
	for _, env := range container.Env {
		if env.Name == "GITHUB_TOKEN" {
			foundGithubToken = true
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
	if !foundGithubToken {
		t.Errorf("GITHUB_TOKEN env not found")
	}
}

func TestBuildPod_WithEntireSecretAsDirectory(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	mountPath := "/etc/ssl/certs"
	var fileMode int32 = 0400

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		credentials: []kubeopenv1alpha1.Credential{
			{
				// No Key specified + MountPath = mount entire secret as directory
				Name: "tls-certs",
				SecretRef: kubeopenv1alpha1.SecretReference{
					Name: "tls-certificates",
					// Key is nil - entire secret should be mounted as directory
				},
				MountPath: &mountPath,
				FileMode:  &fileMode,
			},
		},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	container := pod.Spec.Containers[0]

	// Verify envFrom is NOT set (should not be env vars)
	if len(container.EnvFrom) != 0 {
		t.Errorf("Expected 0 envFrom entries, got %d", len(container.EnvFrom))
	}

	// Verify volume is created
	var foundVolume bool
	var volumeName string
	for _, vol := range pod.Spec.Volumes {
		if vol.Secret != nil && vol.Secret.SecretName == "tls-certificates" {
			foundVolume = true
			volumeName = vol.Name

			// Verify DefaultMode is set
			if vol.Secret.DefaultMode == nil {
				t.Errorf("Expected DefaultMode to be set")
			} else if *vol.Secret.DefaultMode != fileMode {
				t.Errorf("DefaultMode = %d, want %d", *vol.Secret.DefaultMode, fileMode)
			}

			// Verify Items is NOT set (mounting entire secret)
			if len(vol.Secret.Items) != 0 {
				t.Errorf("Expected no Items for entire secret mount, got %d", len(vol.Secret.Items))
			}
			break
		}
	}
	if !foundVolume {
		t.Fatalf("Volume for tls-certificates secret not found")
	}

	// Verify volumeMount is created
	var foundVolumeMount bool
	for _, vm := range container.VolumeMounts {
		if vm.Name == volumeName && vm.MountPath == mountPath {
			foundVolumeMount = true

			// Verify SubPath is NOT set (mounting entire directory)
			if vm.SubPath != "" {
				t.Errorf("SubPath should be empty for directory mount, got %q", vm.SubPath)
			}
			break
		}
	}
	if !foundVolumeMount {
		t.Errorf("VolumeMount for %s not found", mountPath)
	}
}

func TestBuildPod_WithPodScheduling(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	runtimeClass := "gvisor"
	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		podSpec: &kubeopenv1alpha1.AgentPodSpec{
			Labels: map[string]string{
				"custom-label": "custom-value",
			},
			Scheduling: &kubeopenv1alpha1.PodScheduling{
				NodeSelector: map[string]string{
					"node-type": "gpu",
				},
				Tolerations: []corev1.Toleration{
					{
						Key:      "dedicated",
						Operator: corev1.TolerationOpEqual,
						Value:    "ai-workload",
						Effect:   corev1.TaintEffectNoSchedule,
					},
				},
			},
			RuntimeClassName: &runtimeClass,
		},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	// Verify node selector
	if pod.Spec.NodeSelector["node-type"] != "gpu" {
		t.Errorf("NodeSelector[node-type] = %q, want %q", pod.Spec.NodeSelector["node-type"], "gpu")
	}

	// Verify tolerations
	if len(pod.Spec.Tolerations) != 1 {
		t.Fatalf("len(Tolerations) = %d, want 1", len(pod.Spec.Tolerations))
	}
	if pod.Spec.Tolerations[0].Key != "dedicated" {
		t.Errorf("Tolerations[0].Key = %q, want %q", pod.Spec.Tolerations[0].Key, "dedicated")
	}

	// Verify runtime class
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "gvisor" {
		t.Errorf("RuntimeClassName = %v, want %q", pod.Spec.RuntimeClassName, "gvisor")
	}

	// Verify custom label on pod
	if pod.Labels["custom-label"] != "custom-value" {
		t.Errorf("Pod.Labels[custom-label] = %q, want %q", pod.Labels["custom-label"], "custom-value")
	}
	// Verify base labels are still present
	if pod.Labels["app"] != "kubeopencode" {
		t.Errorf("Pod.Labels[app] = %q, want %q", pod.Labels["app"], "kubeopencode")
	}
}

func TestBuildPod_WithContextConfigMap(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	contextConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task-context",
			Namespace: "default",
		},
		Data: map[string]string{
			"workspace-task.md": "# Test Task",
		},
	}

	fileMounts := []fileMount{
		{filePath: "/workspace/task.md"},
	}

	pod := buildPod(task, "test-task-pod", cfg, contextConfigMap, fileMounts, nil, nil, defaultSystemConfig(), "")

	// Verify context-files volume exists (for init container to read from)
	var foundContextVolume bool
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == "context-files" && vol.ConfigMap != nil {
			foundContextVolume = true
			if vol.ConfigMap.Name != "test-task-context" {
				t.Errorf("context-files volume ConfigMap.Name = %q, want %q", vol.ConfigMap.Name, "test-task-context")
			}
		}
	}
	if !foundContextVolume {
		t.Errorf("context-files volume not found")
	}

	// Verify workspace emptyDir volume exists
	var foundWorkspaceVolume bool
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == WorkspaceVolumeName && vol.EmptyDir != nil {
			foundWorkspaceVolume = true
		}
	}
	if !foundWorkspaceVolume {
		t.Errorf("workspace emptyDir volume not found")
	}

	// Verify agent container mounts workspace emptyDir
	container := pod.Spec.Containers[0]
	var foundWorkspaceMount bool
	for _, mount := range container.VolumeMounts {
		if mount.MountPath == "/workspace" && mount.Name == WorkspaceVolumeName {
			foundWorkspaceMount = true
		}
	}
	if !foundWorkspaceMount {
		t.Errorf("Agent container should mount workspace emptyDir at /workspace")
	}

	// Verify context-init container exists
	initContainers := pod.Spec.InitContainers
	var foundContextInit bool
	for _, ic := range initContainers {
		if ic.Name == "context-init" {
			foundContextInit = true
			// Verify init container mounts the ConfigMap
			var foundConfigMapMount bool
			var foundInitWorkspaceMount bool
			for _, mount := range ic.VolumeMounts {
				if mount.Name == "context-files" && mount.MountPath == "/configmap-files" {
					foundConfigMapMount = true
				}
				if mount.Name == WorkspaceVolumeName && mount.MountPath == "/workspace" {
					foundInitWorkspaceMount = true
				}
			}
			if !foundConfigMapMount {
				t.Errorf("context-init container should mount context-files ConfigMap at /configmap-files")
			}
			if !foundInitWorkspaceMount {
				t.Errorf("context-init container should mount workspace emptyDir at /workspace")
			}
		}
	}
	if !foundContextInit {
		t.Errorf("context-init init container not found")
	}
}

func TestBuildPod_WithDirMounts(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	dirMounts := []dirMount{
		{
			dirPath:       "/workspace/guides",
			configMapName: "guides-configmap",
			optional:      true,
		},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, dirMounts, nil, defaultSystemConfig(), "")

	// Verify dir-mount volume exists (for init container to read from)
	var foundDirVolume bool
	for _, vol := range pod.Spec.Volumes {
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

	// Verify workspace emptyDir volume exists
	var foundWorkspaceVolume bool
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == WorkspaceVolumeName && vol.EmptyDir != nil {
			foundWorkspaceVolume = true
		}
	}
	if !foundWorkspaceVolume {
		t.Errorf("workspace emptyDir volume not found")
	}

	// Verify agent container mounts workspace emptyDir (not dir-mount directly)
	container := pod.Spec.Containers[0]
	var foundWorkspaceMount bool
	for _, mount := range container.VolumeMounts {
		if mount.MountPath == "/workspace" && mount.Name == WorkspaceVolumeName {
			foundWorkspaceMount = true
		}
	}
	if !foundWorkspaceMount {
		t.Errorf("Agent container should mount workspace emptyDir at /workspace")
	}

	// Verify context-init container exists and mounts the ConfigMap
	initContainers := pod.Spec.InitContainers
	var foundContextInit bool
	for _, ic := range initContainers {
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

func TestBuildPod_WithGitMounts(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       "test-uid",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kubeopencode.io/v1alpha1",
			Kind:       "Task",
		},
	}

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	gitMounts := []gitMount{
		{
			contextName: "my-context",
			repository:  "https://github.com/org/repo.git",
			ref:         "main",
			repoPath:    ".claude/",
			mountPath:   "/workspace/.claude",
			depth:       1,
			secretName:  "",
		},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, gitMounts, defaultSystemConfig(), "")

	// Verify init containers exist (opencode-init first, then git-init-0)
	if len(pod.Spec.InitContainers) != 2 {
		t.Fatalf("Expected 2 init containers (opencode-init + git-init), got %d", len(pod.Spec.InitContainers))
	}

	// First init container should be opencode-init
	openCodeInit := pod.Spec.InitContainers[0]
	if openCodeInit.Name != "opencode-init" {
		t.Errorf("First init container name = %q, want %q", openCodeInit.Name, "opencode-init")
	}

	// Second init container should be git-init-0
	initContainer := pod.Spec.InitContainers[1]
	if initContainer.Name != "git-init-0" {
		t.Errorf("Git init container name = %q, want %q", initContainer.Name, "git-init-0")
	}
	if initContainer.Image != DefaultKubeOpenCodeImage {
		t.Errorf("Git init container image = %q, want %q", initContainer.Image, DefaultKubeOpenCodeImage)
	}

	// Verify environment variables
	envMap := make(map[string]string)
	for _, env := range initContainer.Env {
		envMap[env.Name] = env.Value
	}
	if envMap["GIT_REPO"] != "https://github.com/org/repo.git" {
		t.Errorf("GIT_REPO = %q, want %q", envMap["GIT_REPO"], "https://github.com/org/repo.git")
	}
	if envMap["GIT_REF"] != "main" {
		t.Errorf("GIT_REF = %q, want %q", envMap["GIT_REF"], "main")
	}
	if envMap["GIT_DEPTH"] != "1" {
		t.Errorf("GIT_DEPTH = %q, want %q", envMap["GIT_DEPTH"], "1")
	}

	// Verify emptyDir volume exists
	var foundGitVolume bool
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == "git-context-0" && vol.EmptyDir != nil {
			foundGitVolume = true
		}
	}
	if !foundGitVolume {
		t.Errorf("git-context-0 emptyDir volume not found")
	}

	// Verify volume mount in agent container with correct subPath
	container := pod.Spec.Containers[0]
	var foundMount bool
	for _, mount := range container.VolumeMounts {
		if mount.MountPath == "/workspace/.claude" && mount.Name == "git-context-0" {
			foundMount = true
			expectedSubPath := "repo/.claude/"
			if mount.SubPath != expectedSubPath {
				t.Errorf("Volume mount SubPath = %q, want %q", mount.SubPath, expectedSubPath)
			}
		}
	}
	if !foundMount {
		t.Errorf("Volume mount for /workspace/.claude not found")
	}
}

func TestBuildPod_WithGitMountsAndAuth(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       "test-uid",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kubeopencode.io/v1alpha1",
			Kind:       "Task",
		},
	}

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	gitMounts := []gitMount{
		{
			contextName: "private-repo",
			repository:  "https://github.com/org/private-repo.git",
			ref:         "v1.0.0",
			repoPath:    "",
			mountPath:   "/workspace/git-private-repo",
			depth:       1,
			secretName:  "git-credentials",
		},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, gitMounts, defaultSystemConfig(), "")

	// Verify we have 2 init containers (opencode-init + git-init)
	if len(pod.Spec.InitContainers) != 2 {
		t.Fatalf("Expected 2 init containers, got %d", len(pod.Spec.InitContainers))
	}

	// Verify git-init container (second one) has auth env vars
	gitInitContainer := pod.Spec.InitContainers[1]
	if gitInitContainer.Name != "git-init-0" {
		t.Errorf("Second init container name = %q, want %q", gitInitContainer.Name, "git-init-0")
	}

	var foundUsername, foundPassword, foundSSHKey, foundSSHKnownHosts bool
	for _, env := range gitInitContainer.Env {
		if env.Name == "GIT_USERNAME" && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			if env.ValueFrom.SecretKeyRef.Name == "git-credentials" && env.ValueFrom.SecretKeyRef.Key == "username" {
				foundUsername = true
			}
		}
		if env.Name == "GIT_PASSWORD" && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			if env.ValueFrom.SecretKeyRef.Name == "git-credentials" && env.ValueFrom.SecretKeyRef.Key == "password" {
				foundPassword = true
			}
		}
		if env.Name == "GIT_SSH_KEY" && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			if env.ValueFrom.SecretKeyRef.Name == "git-credentials" && env.ValueFrom.SecretKeyRef.Key == "ssh-privatekey" {
				foundSSHKey = true
			}
		}
		if env.Name == "GIT_SSH_KNOWN_HOSTS" && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
			if env.ValueFrom.SecretKeyRef.Name == "git-credentials" && env.ValueFrom.SecretKeyRef.Key == "ssh-known-hosts" {
				foundSSHKnownHosts = true
			}
		}
	}
	if !foundUsername {
		t.Errorf("GIT_USERNAME env var with secret reference not found")
	}
	if !foundPassword {
		t.Errorf("GIT_PASSWORD env var with secret reference not found")
	}
	if !foundSSHKey {
		t.Errorf("GIT_SSH_KEY env var with secret reference not found")
	}
	if !foundSSHKnownHosts {
		t.Errorf("GIT_SSH_KNOWN_HOSTS env var with secret reference not found")
	}

	// Verify volume mount without subPath (entire repo)
	container := pod.Spec.Containers[0]
	var foundMount bool
	for _, mount := range container.VolumeMounts {
		if mount.MountPath == "/workspace/git-private-repo" && mount.Name == "git-context-0" {
			foundMount = true
			if mount.SubPath != "repo" {
				t.Errorf("Volume mount SubPath = %q, want %q", mount.SubPath, "repo")
			}
		}
	}
	if !foundMount {
		t.Errorf("Volume mount for /workspace/git-private-repo not found")
	}
}

func TestBuildGitInitContainer(t *testing.T) {
	gm := gitMount{
		contextName: "test-context",
		repository:  "https://github.com/test/repo.git",
		ref:         "develop",
		repoPath:    "docs/",
		mountPath:   "/workspace/docs",
		depth:       5,
		secretName:  "",
	}

	container := buildGitInitContainer(gm, "git-vol-0", 0, defaultSystemConfig())

	if container.Name != "git-init-0" {
		t.Errorf("Container name = %q, want %q", container.Name, "git-init-0")
	}

	if container.Image != DefaultKubeOpenCodeImage {
		t.Errorf("Container image = %q, want %q", container.Image, DefaultKubeOpenCodeImage)
	}

	// Check env vars
	envMap := make(map[string]string)
	for _, env := range container.Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}

	if envMap["GIT_REPO"] != "https://github.com/test/repo.git" {
		t.Errorf("GIT_REPO = %q, want %q", envMap["GIT_REPO"], "https://github.com/test/repo.git")
	}
	if envMap["GIT_REF"] != "develop" {
		t.Errorf("GIT_REF = %q, want %q", envMap["GIT_REF"], "develop")
	}
	if envMap["GIT_DEPTH"] != "5" {
		t.Errorf("GIT_DEPTH = %q, want %q", envMap["GIT_DEPTH"], "5")
	}

	// Verify volume mount
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("Expected 1 volume mount, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].Name != "git-vol-0" {
		t.Errorf("Volume mount name = %q, want %q", container.VolumeMounts[0].Name, "git-vol-0")
	}
	if container.VolumeMounts[0].MountPath != "/git" {
		t.Errorf("Volume mount path = %q, want %q", container.VolumeMounts[0].MountPath, "/git")
	}
}

func TestBuildGitInitContainerWithSecret(t *testing.T) {
	gm := gitMount{
		contextName: "private-repo",
		repository:  "git@github.com:org/private-repo.git",
		ref:         "main",
		repoPath:    "",
		mountPath:   "/workspace/private",
		depth:       1,
		secretName:  "my-git-secret",
	}

	container := buildGitInitContainer(gm, "git-vol-0", 0, defaultSystemConfig())

	// Verify all 4 auth env vars are injected (HTTPS + SSH)
	wantEnvVars := map[string]struct {
		secretName string
		secretKey  string
	}{
		"GIT_USERNAME":        {secretName: "my-git-secret", secretKey: "username"},
		"GIT_PASSWORD":        {secretName: "my-git-secret", secretKey: "password"},
		"GIT_SSH_KEY":         {secretName: "my-git-secret", secretKey: "ssh-privatekey"},
		"GIT_SSH_KNOWN_HOSTS": {secretName: "my-git-secret", secretKey: "ssh-known-hosts"},
	}

	for wantName, want := range wantEnvVars {
		found := false
		for _, env := range container.Env {
			if env.Name == wantName && env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
				if env.ValueFrom.SecretKeyRef.Name != want.secretName {
					t.Errorf("%s secret name = %q, want %q", wantName, env.ValueFrom.SecretKeyRef.Name, want.secretName)
				}
				if env.ValueFrom.SecretKeyRef.Key != want.secretKey {
					t.Errorf("%s secret key = %q, want %q", wantName, env.ValueFrom.SecretKeyRef.Key, want.secretKey)
				}
				if env.ValueFrom.SecretKeyRef.Optional == nil || !*env.ValueFrom.SecretKeyRef.Optional {
					t.Errorf("%s should be optional", wantName)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("env var %s with SecretKeyRef not found", wantName)
		}
	}
}

func TestBuildGitInitContainerWithoutSecret(t *testing.T) {
	gm := gitMount{
		contextName: "public-repo",
		repository:  "https://github.com/org/public-repo.git",
		ref:         "main",
		mountPath:   "/workspace/public",
		depth:       1,
		secretName:  "",
	}

	container := buildGitInitContainer(gm, "git-vol-0", 0, defaultSystemConfig())

	// Verify no auth env vars are injected for public repos
	for _, env := range container.Env {
		switch env.Name {
		case "GIT_USERNAME", "GIT_PASSWORD", "GIT_SSH_KEY", "GIT_SSH_KNOWN_HOSTS":
			t.Errorf("unexpected auth env var %s for public repo", env.Name)
		}
	}
}

func TestBuildContextInitContainer(t *testing.T) {
	tests := []struct {
		name         string
		workspaceDir string
		fileMounts   []fileMount
		dirMounts    []dirMount
		wantEnvVars  map[string]string
	}{
		{
			name:         "with file mounts only",
			workspaceDir: "/workspace",
			fileMounts: []fileMount{
				{filePath: "/workspace/task.md"},
				{filePath: "/workspace/guides/readme.md"},
			},
			dirMounts: nil,
			wantEnvVars: map[string]string{
				"WORKSPACE_DIR":  "/workspace",
				"CONFIGMAP_PATH": "/configmap-files",
				"FILE_MAPPINGS":  `[{"key":"workspace-task.md","targetPath":"/workspace/task.md"},{"key":"workspace-guides-readme.md","targetPath":"/workspace/guides/readme.md"}]`,
			},
		},
		{
			name:         "with dir mounts only",
			workspaceDir: "/workspace",
			fileMounts:   nil,
			dirMounts: []dirMount{
				{dirPath: "/workspace/config", configMapName: "config-cm"},
				{dirPath: "/workspace/scripts", configMapName: "scripts-cm"},
			},
			wantEnvVars: map[string]string{
				"WORKSPACE_DIR":  "/workspace",
				"CONFIGMAP_PATH": "/configmap-files",
				"DIR_MAPPINGS":   `[{"sourcePath":"/configmap-dir-0","targetPath":"/workspace/config"},{"sourcePath":"/configmap-dir-1","targetPath":"/workspace/scripts"}]`,
			},
		},
		{
			name:         "with both file and dir mounts",
			workspaceDir: "/workspace",
			fileMounts: []fileMount{
				{filePath: "/workspace/task.md"},
			},
			dirMounts: []dirMount{
				{dirPath: "/workspace/guides", configMapName: "guides-cm"},
			},
			wantEnvVars: map[string]string{
				"WORKSPACE_DIR":  "/workspace",
				"CONFIGMAP_PATH": "/configmap-files",
				"FILE_MAPPINGS":  `[{"key":"workspace-task.md","targetPath":"/workspace/task.md"}]`,
				"DIR_MAPPINGS":   `[{"sourcePath":"/configmap-dir-0","targetPath":"/workspace/guides"}]`,
			},
		},
		{
			name:         "with custom workspace dir",
			workspaceDir: "/home/agent/work",
			fileMounts: []fileMount{
				{filePath: "/home/agent/work/task.md"},
			},
			dirMounts: nil,
			wantEnvVars: map[string]string{
				"WORKSPACE_DIR":  "/home/agent/work",
				"CONFIGMAP_PATH": "/configmap-files",
				"FILE_MAPPINGS":  `[{"key":"home-agent-work-task.md","targetPath":"/home/agent/work/task.md"}]`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := buildContextInitContainer(tt.workspaceDir, tt.fileMounts, tt.dirMounts, defaultSystemConfig())

			// Verify container name
			if container.Name != "context-init" {
				t.Errorf("Container.Name = %q, want %q", container.Name, "context-init")
			}

			// Verify image
			if container.Image != DefaultKubeOpenCodeImage {
				t.Errorf("Container.Image = %q, want %q", container.Image, DefaultKubeOpenCodeImage)
			}

			// Verify command uses /kubeopencode context-init
			if len(container.Command) != 2 {
				t.Fatalf("len(Container.Command) = %d, want 2", len(container.Command))
			}
			if container.Command[0] != "/kubeopencode" {
				t.Errorf("Container.Command[0] = %q, want %q", container.Command[0], "/kubeopencode")
			}
			if container.Command[1] != "context-init" {
				t.Errorf("Container.Command[1] = %q, want %q", container.Command[1], "context-init")
			}

			// Verify environment variables
			envMap := make(map[string]string)
			for _, env := range container.Env {
				envMap[env.Name] = env.Value
			}

			for key, wantValue := range tt.wantEnvVars {
				gotValue, ok := envMap[key]
				if !ok {
					t.Errorf("Missing expected env var: %s", key)
					continue
				}
				if gotValue != wantValue {
					t.Errorf("Env[%s] = %q, want %q", key, gotValue, wantValue)
				}
			}

			// Verify no unexpected env vars for FILE_MAPPINGS/DIR_MAPPINGS
			if len(tt.fileMounts) == 0 {
				if _, ok := envMap["FILE_MAPPINGS"]; ok {
					t.Errorf("FILE_MAPPINGS should not be set when there are no file mounts")
				}
			}
			if len(tt.dirMounts) == 0 {
				if _, ok := envMap["DIR_MAPPINGS"]; ok {
					t.Errorf("DIR_MAPPINGS should not be set when there are no dir mounts")
				}
			}
		})
	}
}

// TestBuildPod_WithExternalFileMounts tests that files mounted outside of /workspace
// are properly handled by creating shared emptyDir volumes.
func TestBuildPod_WithExternalFileMounts(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	contextConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task-context",
			Namespace: "default",
		},
		Data: map[string]string{
			"workspace-task.md":                "# Test Task",
			"etc-github-app-github-app-iat.sh": "#!/bin/bash\necho token",
		},
	}

	// File mounts: one inside /workspace, one outside (in /etc/github-app)
	fileMounts := []fileMount{
		{filePath: "/workspace/task.md"},
		{filePath: "/etc/github-app/github-app-iat.sh"},
	}

	pod := buildPod(task, "test-task-pod", cfg, contextConfigMap, fileMounts, nil, nil, defaultSystemConfig(), "")

	// Verify workspace emptyDir volume exists
	var foundWorkspaceVolume bool
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == WorkspaceVolumeName && vol.EmptyDir != nil {
			foundWorkspaceVolume = true
		}
	}
	if !foundWorkspaceVolume {
		t.Errorf("workspace volume not found")
	}

	// Verify external directory volume exists (for /etc/github-app)
	var foundExternalVolume bool
	var externalVolumeName string
	for _, vol := range pod.Spec.Volumes {
		if strings.HasPrefix(vol.Name, "ctx-") && vol.EmptyDir != nil {
			foundExternalVolume = true
			externalVolumeName = vol.Name
		}
	}
	if !foundExternalVolume {
		t.Errorf("external directory volume (ctx-*) not found for /etc/github-app")
	}

	// Verify context-init container has the external volume mounted
	var contextInitContainer *corev1.Container
	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == "context-init" {
			contextInitContainer = &pod.Spec.InitContainers[i]
			break
		}
	}
	if contextInitContainer == nil {
		t.Fatalf("context-init container not found")
	}

	var contextInitHasExternalMount bool
	for _, vm := range contextInitContainer.VolumeMounts {
		if vm.Name == externalVolumeName && vm.MountPath == "/etc/github-app" {
			contextInitHasExternalMount = true
		}
	}
	if !contextInitHasExternalMount {
		t.Errorf("context-init container does not have external volume mounted at /etc/github-app")
	}

	// Verify agent container has the external volume mounted
	var agentContainer *corev1.Container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == "agent" {
			agentContainer = &pod.Spec.Containers[i]
			break
		}
	}
	if agentContainer == nil {
		t.Fatalf("agent container not found")
	}

	var agentHasExternalMount bool
	for _, vm := range agentContainer.VolumeMounts {
		if vm.Name == externalVolumeName && vm.MountPath == "/etc/github-app" {
			agentHasExternalMount = true
		}
	}
	if !agentHasExternalMount {
		t.Errorf("agent container does not have external volume mounted at /etc/github-app")
	}
}

// TestResolveMountPath tests the Tekton-style path resolution for mountPath.
// Paths starting with "/" are absolute, paths without "/" prefix are relative
// and get prefixed with workspaceDir.
func TestResolveMountPath(t *testing.T) {
	tests := []struct {
		name         string
		mountPath    string
		workspaceDir string
		want         string
	}{
		{
			name:         "empty path returns empty",
			mountPath:    "",
			workspaceDir: "/workspace",
			want:         "",
		},
		{
			name:         "absolute path unchanged",
			mountPath:    "/etc/config/app.conf",
			workspaceDir: "/workspace",
			want:         "/etc/config/app.conf",
		},
		{
			name:         "absolute path with workspace prefix unchanged",
			mountPath:    "/workspace/task.md",
			workspaceDir: "/workspace",
			want:         "/workspace/task.md",
		},
		{
			name:         "relative path gets prefixed",
			mountPath:    "guides/readme.md",
			workspaceDir: "/workspace",
			want:         "/workspace/guides/readme.md",
		},
		{
			name:         "simple filename gets prefixed",
			mountPath:    "task-context.md",
			workspaceDir: "/workspace",
			want:         "/workspace/task-context.md",
		},
		{
			name:         "dot-slash relative path gets prefixed",
			mountPath:    "./guides/readme.md",
			workspaceDir: "/workspace",
			want:         "/workspace/./guides/readme.md",
		},
		{
			name:         "relative path with custom workspaceDir",
			mountPath:    "config/settings.yaml",
			workspaceDir: "/home/agent",
			want:         "/home/agent/config/settings.yaml",
		},
		{
			name:         "deeply nested relative path",
			mountPath:    "a/b/c/d/file.txt",
			workspaceDir: "/workspace",
			want:         "/workspace/a/b/c/d/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveMountPath(tt.mountPath, tt.workspaceDir)
			if got != tt.want {
				t.Errorf("resolveMountPath(%q, %q) = %q, want %q", tt.mountPath, tt.workspaceDir, got, tt.want)
			}
		})
	}
}

func TestBuildPod_WithOpenCodeConfig(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	configJSON := `{"model": "google/gemini-2.5-pro", "small_model": "google/gemini-2.5-flash"}`
	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		config:             &configJSON,
	}

	// Create a ConfigMap with the config content
	// Use sanitizeConfigMapKey to match the key generated by task_controller
	expectedConfigKey := sanitizeConfigMapKey(OpenCodeConfigPath)
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task-context",
			Namespace: "default",
		},
		Data: map[string]string{
			expectedConfigKey: configJSON,
		},
	}

	// Include the config file in fileMounts
	fileMounts := []fileMount{
		{filePath: OpenCodeConfigPath},
	}

	pod := buildPod(task, "test-task-pod", cfg, configMap, fileMounts, nil, nil, defaultSystemConfig(), "")

	// Verify OPENCODE_CONFIG env var is set
	container := pod.Spec.Containers[0]
	var foundOpenCodeConfigEnv bool
	for _, env := range container.Env {
		if env.Name == OpenCodeConfigEnvVar {
			if env.Value != OpenCodeConfigPath {
				t.Errorf("OPENCODE_CONFIG env value = %q, want %q", env.Value, OpenCodeConfigPath)
			}
			foundOpenCodeConfigEnv = true
			break
		}
	}
	if !foundOpenCodeConfigEnv {
		t.Errorf("OPENCODE_CONFIG env var not found in container env")
	}

	// Verify context-init container has /tools volume mount
	var contextInitContainer *corev1.Container
	for i, initC := range pod.Spec.InitContainers {
		if initC.Name == "context-init" {
			contextInitContainer = &pod.Spec.InitContainers[i]
			break
		}
	}
	if contextInitContainer == nil {
		t.Fatalf("context-init container not found")
	}

	var hasToolsMount bool
	for _, vm := range contextInitContainer.VolumeMounts {
		if vm.Name == ToolsVolumeName && vm.MountPath == ToolsMountPath {
			hasToolsMount = true
			break
		}
	}
	if !hasToolsMount {
		t.Errorf("context-init container should mount /tools volume for config file")
	}
}

func TestBuildPod_WithoutOpenCodeConfig(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		config:             nil, // No config provided
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	// Verify OPENCODE_CONFIG env var is NOT set
	container := pod.Spec.Containers[0]
	for _, env := range container.Env {
		if env.Name == OpenCodeConfigEnvVar {
			t.Errorf("OPENCODE_CONFIG env var should not be set when config is nil")
		}
	}
}

func TestBuildPod_WithContextFile(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	// Create context file mount (simulates context without mountPath)
	contextFilePath := cfg.workspaceDir + "/" + ContextFileRelPath
	fileMounts := []fileMount{
		{filePath: contextFilePath},
	}

	contextConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task-context",
			Namespace: "default",
		},
		Data: map[string]string{
			sanitizeConfigMapKey(contextFilePath): "<context>test content</context>",
		},
	}

	pod := buildPod(task, "test-task-pod", cfg, contextConfigMap, fileMounts, nil, nil, defaultSystemConfig(), "")

	// Verify OPENCODE_CONFIG_CONTENT env var is set
	container := pod.Spec.Containers[0]
	foundConfigContentEnv := false
	for _, env := range container.Env {
		if env.Name == OpenCodeConfigContentEnvVar {
			foundConfigContentEnv = true
			expectedValue := `{"instructions":["` + ContextFileRelPath + `"]}`
			if env.Value != expectedValue {
				t.Errorf("OPENCODE_CONFIG_CONTENT env value = %q, want %q", env.Value, expectedValue)
			}
			break
		}
	}
	if !foundConfigContentEnv {
		t.Errorf("OPENCODE_CONFIG_CONTENT env var not found in container env")
	}
}

func TestBuildPod_WithoutContextFile(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	// No context file mount - only task.md
	fileMounts := []fileMount{
		{filePath: "/workspace/task.md"},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, fileMounts, nil, nil, defaultSystemConfig(), "")

	// Verify OPENCODE_CONFIG_CONTENT env var is NOT set
	container := pod.Spec.Containers[0]
	for _, env := range container.Env {
		if env.Name == OpenCodeConfigContentEnvVar {
			t.Errorf("OPENCODE_CONFIG_CONTENT env var should not be set when no context file")
		}
	}
}

func TestBuildPod_AgentRef_WithAttachCommand(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		attachImage:        "test-attach:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	serverURL := "http://test-agent.default.svc.cluster.local:4096"
	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), serverURL)

	// Verify pod was created
	if pod == nil {
		t.Fatal("buildPod returned nil")
	}

	container := pod.Spec.Containers[0]

	// Verify attach image is used for default --attach command (no custom command)
	if container.Image != "test-attach:v1.0.0" {
		t.Errorf("Container.Image = %q, want %q (attach image for agentRef with default command)", container.Image, "test-attach:v1.0.0")
	}

	// Verify command uses --attach flag
	if len(container.Command) != 3 {
		t.Fatalf("len(Container.Command) = %d, want 3", len(container.Command))
	}
	if container.Command[0] != "sh" {
		t.Errorf("Container.Command[0] = %q, want %q", container.Command[0], "sh")
	}
	if container.Command[1] != "-c" {
		t.Errorf("Container.Command[1] = %q, want %q", container.Command[1], "-c")
	}

	// Verify the command includes --attach and serverURL
	if !strings.Contains(container.Command[2], "--attach") {
		t.Errorf("Command should contain --attach flag")
	}
	if !strings.Contains(container.Command[2], serverURL) {
		t.Errorf("Command should contain server URL: %s", serverURL)
	}
	if !strings.Contains(container.Command[2], "/tools/opencode run") {
		t.Errorf("Command should use /tools/opencode run")
	}
	if !strings.Contains(container.Command[2], "$(cat /workspace/task.md)") {
		t.Errorf("Command should read task from /workspace/task.md")
	}
	// Verify the command includes --title with task name prefix
	if !strings.Contains(container.Command[2], "--title") {
		t.Errorf("Command should contain --title flag")
	}
	if !strings.Contains(container.Command[2], "test-task-") {
		t.Errorf("Command --title should contain task name prefix 'test-task-'")
	}
}

func TestBuildPod_AgentRef_WithCustomCommand_KeepsExecutorImage(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		attachImage:        "test-attach:v1.0.0",
		command:            []string{"sh", "-c", "echo hello"},
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	serverURL := "http://test-agent.default.svc.cluster.local:4096"
	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), serverURL)

	if pod == nil {
		t.Fatal("buildPod returned nil")
	}

	container := pod.Spec.Containers[0]

	// When a custom command is provided, keep the executor image since the
	// custom command may need tools not available in the minimal attach image.
	if container.Image != "test-executor:v1.0.0" {
		t.Errorf("Container.Image = %q, want %q (executor image for agentRef with custom command)", container.Image, "test-executor:v1.0.0")
	}

	// Verify custom command is used (not --attach)
	if strings.Contains(strings.Join(container.Command, " "), "--attach") {
		t.Errorf("Custom command should NOT contain --attach flag")
	}
}

func TestBuildPod_TemplateRef_WithoutAttachCommand(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	// Empty serverURL means Pod mode
	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	// Verify pod was created
	if pod == nil {
		t.Fatal("buildPod returned nil")
	}

	container := pod.Spec.Containers[0]

	// Verify command does NOT use --attach flag
	if len(container.Command) != 3 {
		t.Fatalf("len(Container.Command) = %d, want 3", len(container.Command))
	}

	// Verify the command does NOT include --attach
	if strings.Contains(container.Command[2], "--attach") {
		t.Errorf("Command should NOT contain --attach flag in Pod mode")
	}
	if !strings.Contains(container.Command[2], "/tools/opencode run") {
		t.Errorf("Command should use /tools/opencode run")
	}
	// Verify the command includes --title with task name prefix
	if !strings.Contains(container.Command[2], "--title") {
		t.Errorf("Command should contain --title flag")
	}
	if !strings.Contains(container.Command[2], "test-task-") {
		t.Errorf("Command --title should contain task name prefix 'test-task-'")
	}
}

func TestBuildPod_SkipsOPENCODE_PERMISSIONWhenConfigHasPermission(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	configWithPermission := `{"permission": "ask", "model": "gpt-4"}`
	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		config:             &configWithPermission,
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	container := pod.Spec.Containers[0]

	// Verify OPENCODE_PERMISSION env var is NOT set
	for _, env := range container.Env {
		if env.Name == OpenCodePermissionEnvVar {
			t.Errorf("OPENCODE_PERMISSION should not be set when config has permission field")
		}
	}
}

func TestBuildPod_SetsOPENCODE_PERMISSIONWhenConfigHasNoPermission(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	configWithoutPermission := `{"model": "gpt-4"}`
	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		config:             &configWithoutPermission,
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	container := pod.Spec.Containers[0]

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

func TestBuildPod_SessionTitleDefaultsToTaskNameWithSuffix(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	pod := buildPod(task, "my-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")
	container := pod.Spec.Containers[0]

	// Should contain task name prefix with random suffix
	if !strings.Contains(container.Command[2], "'my-task-") {
		t.Errorf("Command --title should start with task name 'my-task-', got: %s", container.Command[2])
	}
}

func TestBuildPod_SessionTitleNotSetForCustomCommand(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		command:            []string{"sh", "-c", "echo hello"},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")
	container := pod.Spec.Containers[0]

	// Custom command should not be modified
	if strings.Contains(container.Command[2], "--title") {
		t.Errorf("Custom command should NOT have --title injected")
	}
	if container.Command[2] != "echo hello" {
		t.Errorf("Custom command should be preserved, got: %s", container.Command[2])
	}
}

func TestBuildCABundleVolumeMountEnv_ConfigMap(t *testing.T) {
	caBundle := &kubeopenv1alpha1.CABundleConfig{
		ConfigMapRef: &kubeopenv1alpha1.CABundleReference{
			Name: "my-ca-bundle",
			Key:  "custom-ca.pem",
		},
	}

	volume, mount, env := buildCABundleVolumeMountEnv(caBundle)

	// Verify volume
	if volume.Name != CABundleVolumeName {
		t.Errorf("Volume.Name = %q, want %q", volume.Name, CABundleVolumeName)
	}
	if volume.ConfigMap == nil {
		t.Fatalf("Volume.ConfigMap should not be nil")
	}
	if volume.ConfigMap.Name != "my-ca-bundle" {
		t.Errorf("Volume.ConfigMap.Name = %q, want %q", volume.ConfigMap.Name, "my-ca-bundle")
	}
	if len(volume.ConfigMap.Items) != 1 {
		t.Fatalf("len(Volume.ConfigMap.Items) = %d, want 1", len(volume.ConfigMap.Items))
	}
	if volume.ConfigMap.Items[0].Key != "custom-ca.pem" {
		t.Errorf("Volume.ConfigMap.Items[0].Key = %q, want %q", volume.ConfigMap.Items[0].Key, "custom-ca.pem")
	}
	if volume.ConfigMap.Items[0].Path != CABundleFileName {
		t.Errorf("Volume.ConfigMap.Items[0].Path = %q, want %q", volume.ConfigMap.Items[0].Path, CABundleFileName)
	}

	// Verify mount
	if mount.Name != CABundleVolumeName {
		t.Errorf("Mount.Name = %q, want %q", mount.Name, CABundleVolumeName)
	}
	if mount.MountPath != CABundleMountPath {
		t.Errorf("Mount.MountPath = %q, want %q", mount.MountPath, CABundleMountPath)
	}
	if !mount.ReadOnly {
		t.Errorf("Mount.ReadOnly = false, want true")
	}

	// Verify env
	if env.Name != CustomCACertEnvVar {
		t.Errorf("Env.Name = %q, want %q", env.Name, CustomCACertEnvVar)
	}
	expectedEnvValue := CABundleMountPath + "/" + CABundleFileName
	if env.Value != expectedEnvValue {
		t.Errorf("Env.Value = %q, want %q", env.Value, expectedEnvValue)
	}
}

func TestBuildCABundleVolumeMountEnv_ConfigMapDefaultKey(t *testing.T) {
	caBundle := &kubeopenv1alpha1.CABundleConfig{
		ConfigMapRef: &kubeopenv1alpha1.CABundleReference{
			Name: "my-ca-bundle",
			// Key is empty - should default to DefaultCABundleConfigMapKey
		},
	}

	volume, _, _ := buildCABundleVolumeMountEnv(caBundle)

	if volume.ConfigMap == nil {
		t.Fatalf("Volume.ConfigMap should not be nil")
	}
	if len(volume.ConfigMap.Items) != 1 {
		t.Fatalf("len(Volume.ConfigMap.Items) = %d, want 1", len(volume.ConfigMap.Items))
	}
	if volume.ConfigMap.Items[0].Key != DefaultCABundleConfigMapKey {
		t.Errorf("Volume.ConfigMap.Items[0].Key = %q, want %q (default)", volume.ConfigMap.Items[0].Key, DefaultCABundleConfigMapKey)
	}
	if volume.ConfigMap.Items[0].Path != CABundleFileName {
		t.Errorf("Volume.ConfigMap.Items[0].Path = %q, want %q", volume.ConfigMap.Items[0].Path, CABundleFileName)
	}
}

func TestBuildCABundleVolumeMountEnv_Secret(t *testing.T) {
	caBundle := &kubeopenv1alpha1.CABundleConfig{
		SecretRef: &kubeopenv1alpha1.CABundleReference{
			Name: "my-ca-secret",
			Key:  "custom-ca.pem",
		},
	}

	volume, mount, env := buildCABundleVolumeMountEnv(caBundle)

	// Verify volume
	if volume.Name != CABundleVolumeName {
		t.Errorf("Volume.Name = %q, want %q", volume.Name, CABundleVolumeName)
	}
	if volume.Secret == nil {
		t.Fatalf("Volume.Secret should not be nil")
	}
	if volume.Secret.SecretName != "my-ca-secret" {
		t.Errorf("Volume.Secret.SecretName = %q, want %q", volume.Secret.SecretName, "my-ca-secret")
	}
	if len(volume.Secret.Items) != 1 {
		t.Fatalf("len(Volume.Secret.Items) = %d, want 1", len(volume.Secret.Items))
	}
	if volume.Secret.Items[0].Key != "custom-ca.pem" {
		t.Errorf("Volume.Secret.Items[0].Key = %q, want %q", volume.Secret.Items[0].Key, "custom-ca.pem")
	}
	if volume.Secret.Items[0].Path != CABundleFileName {
		t.Errorf("Volume.Secret.Items[0].Path = %q, want %q", volume.Secret.Items[0].Path, CABundleFileName)
	}

	// Verify mount
	if mount.Name != CABundleVolumeName {
		t.Errorf("Mount.Name = %q, want %q", mount.Name, CABundleVolumeName)
	}
	if mount.MountPath != CABundleMountPath {
		t.Errorf("Mount.MountPath = %q, want %q", mount.MountPath, CABundleMountPath)
	}
	if !mount.ReadOnly {
		t.Errorf("Mount.ReadOnly = false, want true")
	}

	// Verify env
	if env.Name != CustomCACertEnvVar {
		t.Errorf("Env.Name = %q, want %q", env.Name, CustomCACertEnvVar)
	}
	expectedEnvValue := CABundleMountPath + "/" + CABundleFileName
	if env.Value != expectedEnvValue {
		t.Errorf("Env.Value = %q, want %q", env.Value, expectedEnvValue)
	}
}

func TestBuildCABundleVolumeMountEnv_SecretDefaultKey(t *testing.T) {
	caBundle := &kubeopenv1alpha1.CABundleConfig{
		SecretRef: &kubeopenv1alpha1.CABundleReference{
			Name: "my-ca-secret",
			// Key is empty - should default to DefaultCABundleSecretKey
		},
	}

	volume, _, _ := buildCABundleVolumeMountEnv(caBundle)

	if volume.Secret == nil {
		t.Fatalf("Volume.Secret should not be nil")
	}
	if len(volume.Secret.Items) != 1 {
		t.Fatalf("len(Volume.Secret.Items) = %d, want 1", len(volume.Secret.Items))
	}
	if volume.Secret.Items[0].Key != DefaultCABundleSecretKey {
		t.Errorf("Volume.Secret.Items[0].Key = %q, want %q (default)", volume.Secret.Items[0].Key, DefaultCABundleSecretKey)
	}
	if volume.Secret.Items[0].Path != CABundleFileName {
		t.Errorf("Volume.Secret.Items[0].Path = %q, want %q", volume.Secret.Items[0].Path, CABundleFileName)
	}
}

func TestBuildPod_WithCABundleConfigMap(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		caBundle: &kubeopenv1alpha1.CABundleConfig{
			ConfigMapRef: &kubeopenv1alpha1.CABundleReference{
				Name: "corp-ca-bundle",
				Key:  "ca-bundle.crt",
			},
		},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	// Verify CA bundle volume exists with correct ConfigMap source
	var foundCAVolume bool
	for _, vol := range pod.Spec.Volumes {
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

	// Verify ALL init containers have the CA mount and env
	for _, ic := range pod.Spec.InitContainers {
		var hasCAMount bool
		for _, vm := range ic.VolumeMounts {
			if vm.Name == CABundleVolumeName && vm.MountPath == CABundleMountPath && vm.ReadOnly {
				hasCAMount = true
			}
		}
		if !hasCAMount {
			t.Errorf("Init container %q missing CA bundle volume mount", ic.Name)
		}

		var hasCAEnv bool
		for _, env := range ic.Env {
			if env.Name == CustomCACertEnvVar && env.Value == CABundleMountPath+"/"+CABundleFileName {
				hasCAEnv = true
			}
		}
		if !hasCAEnv {
			t.Errorf("Init container %q missing %s env var", ic.Name, CustomCACertEnvVar)
		}
	}

	// Verify worker container has the CA mount and env
	container := pod.Spec.Containers[0]
	var hasCAMount bool
	for _, vm := range container.VolumeMounts {
		if vm.Name == CABundleVolumeName && vm.MountPath == CABundleMountPath && vm.ReadOnly {
			hasCAMount = true
		}
	}
	if !hasCAMount {
		t.Errorf("Worker container missing CA bundle volume mount")
	}

	var hasCAEnv bool
	for _, env := range container.Env {
		if env.Name == CustomCACertEnvVar && env.Value == CABundleMountPath+"/"+CABundleFileName {
			hasCAEnv = true
		}
	}
	if !hasCAEnv {
		t.Errorf("Worker container missing %s env var", CustomCACertEnvVar)
	}
}

func TestBuildPod_WithCABundleSecret(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		caBundle: &kubeopenv1alpha1.CABundleConfig{
			SecretRef: &kubeopenv1alpha1.CABundleReference{
				Name: "corp-ca-secret",
				Key:  "custom-ca.pem",
			},
		},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	// Verify CA bundle volume exists with correct Secret source
	var foundCAVolume bool
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == CABundleVolumeName {
			foundCAVolume = true
			if vol.Secret == nil {
				t.Fatalf("CA bundle volume should have Secret source")
			}
			if vol.Secret.SecretName != "corp-ca-secret" {
				t.Errorf("CA volume Secret.SecretName = %q, want %q", vol.Secret.SecretName, "corp-ca-secret")
			}
			if len(vol.Secret.Items) != 1 || vol.Secret.Items[0].Key != "custom-ca.pem" {
				t.Errorf("CA volume Secret should project key %q", "custom-ca.pem")
			}
		}
	}
	if !foundCAVolume {
		t.Fatalf("CA bundle volume %q not found", CABundleVolumeName)
	}

	// Verify ALL init containers have the CA mount and env
	for _, ic := range pod.Spec.InitContainers {
		var hasCAMount bool
		for _, vm := range ic.VolumeMounts {
			if vm.Name == CABundleVolumeName && vm.MountPath == CABundleMountPath {
				hasCAMount = true
			}
		}
		if !hasCAMount {
			t.Errorf("Init container %q missing CA bundle volume mount", ic.Name)
		}

		var hasCAEnv bool
		for _, env := range ic.Env {
			if env.Name == CustomCACertEnvVar {
				hasCAEnv = true
			}
		}
		if !hasCAEnv {
			t.Errorf("Init container %q missing %s env var", ic.Name, CustomCACertEnvVar)
		}
	}

	// Verify worker container has the CA mount and env
	container := pod.Spec.Containers[0]
	var hasCAMount bool
	for _, vm := range container.VolumeMounts {
		if vm.Name == CABundleVolumeName && vm.MountPath == CABundleMountPath {
			hasCAMount = true
		}
	}
	if !hasCAMount {
		t.Errorf("Worker container missing CA bundle volume mount")
	}

	var hasCAEnv bool
	for _, env := range container.Env {
		if env.Name == CustomCACertEnvVar {
			hasCAEnv = true
		}
	}
	if !hasCAEnv {
		t.Errorf("Worker container missing %s env var", CustomCACertEnvVar)
	}
}

func TestBuildPod_WithCABundleAndGitMounts(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		caBundle: &kubeopenv1alpha1.CABundleConfig{
			ConfigMapRef: &kubeopenv1alpha1.CABundleReference{
				Name: "corp-ca-bundle",
			},
		},
	}

	gitMounts := []gitMount{
		{
			contextName: "source-code",
			repository:  "https://github.com/org/repo.git",
			ref:         "main",
			mountPath:   "/workspace/source",
			depth:       1,
			secretName:  "git-creds",
		},
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, gitMounts, defaultSystemConfig(), "")

	// Verify we have at least 2 init containers: opencode-init + git-init-0
	if len(pod.Spec.InitContainers) < 2 {
		t.Fatalf("Expected at least 2 init containers, got %d", len(pod.Spec.InitContainers))
	}

	// Verify git-init container has the CA mount and env
	var foundGitInit bool
	for _, ic := range pod.Spec.InitContainers {
		if ic.Name == "git-init-0" {
			foundGitInit = true

			var hasCAMount bool
			for _, vm := range ic.VolumeMounts {
				if vm.Name == CABundleVolumeName && vm.MountPath == CABundleMountPath {
					hasCAMount = true
				}
			}
			if !hasCAMount {
				t.Errorf("git-init-0 container missing CA bundle volume mount")
			}

			var hasCAEnv bool
			for _, env := range ic.Env {
				if env.Name == CustomCACertEnvVar {
					hasCAEnv = true
				}
			}
			if !hasCAEnv {
				t.Errorf("git-init-0 container missing %s env var", CustomCACertEnvVar)
			}
		}
	}
	if !foundGitInit {
		t.Errorf("git-init-0 container not found")
	}

	// Verify ALL init containers have CA mount and env (opencode-init + git-init)
	for _, ic := range pod.Spec.InitContainers {
		var hasCAMount bool
		for _, vm := range ic.VolumeMounts {
			if vm.Name == CABundleVolumeName && vm.MountPath == CABundleMountPath {
				hasCAMount = true
			}
		}
		if !hasCAMount {
			t.Errorf("Init container %q missing CA bundle volume mount", ic.Name)
		}

		var hasCAEnv bool
		for _, env := range ic.Env {
			if env.Name == CustomCACertEnvVar {
				hasCAEnv = true
			}
		}
		if !hasCAEnv {
			t.Errorf("Init container %q missing %s env var", ic.Name, CustomCACertEnvVar)
		}
	}

	// Verify worker container also has CA mount and env
	container := pod.Spec.Containers[0]
	var hasCAMount bool
	for _, vm := range container.VolumeMounts {
		if vm.Name == CABundleVolumeName && vm.MountPath == CABundleMountPath {
			hasCAMount = true
		}
	}
	if !hasCAMount {
		t.Errorf("Worker container missing CA bundle volume mount")
	}

	var hasCAEnv bool
	for _, env := range container.Env {
		if env.Name == CustomCACertEnvVar {
			hasCAEnv = true
		}
	}
	if !hasCAEnv {
		t.Errorf("Worker container missing %s env var", CustomCACertEnvVar)
	}
}

func TestBuildPod_WithoutCABundle(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubeopencode.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-opencode:v1.0.0",
		executorImage:      "test-executor:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		caBundle:           nil, // No CA bundle
	}

	pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

	// Verify no CA bundle volume exists
	for _, vol := range pod.Spec.Volumes {
		if vol.Name == CABundleVolumeName {
			t.Errorf("CA bundle volume should not exist when caBundle is nil")
		}
	}

	// Verify no init containers have CA mount or env
	for _, ic := range pod.Spec.InitContainers {
		for _, vm := range ic.VolumeMounts {
			if vm.Name == CABundleVolumeName {
				t.Errorf("Init container %q should not have CA bundle volume mount when caBundle is nil", ic.Name)
			}
		}
		for _, env := range ic.Env {
			if env.Name == CustomCACertEnvVar {
				t.Errorf("Init container %q should not have %s env var when caBundle is nil", ic.Name, CustomCACertEnvVar)
			}
		}
	}

	// Verify worker container has no CA mount or env
	container := pod.Spec.Containers[0]
	for _, vm := range container.VolumeMounts {
		if vm.Name == CABundleVolumeName {
			t.Errorf("Worker container should not have CA bundle volume mount when caBundle is nil")
		}
	}
	for _, env := range container.Env {
		if env.Name == CustomCACertEnvVar {
			t.Errorf("Worker container should not have %s env var when caBundle is nil", CustomCACertEnvVar)
		}
	}
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "'simple'"},
		{"with space", "'with space'"},
		{"it's", `'it'\''s'`},
		{"a'b'c", `'a'\''b'\''c'`},
		{"", "''"},
	}
	for _, tt := range tests {
		got := shellEscape(tt.input)
		if got != tt.want {
			t.Errorf("shellEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSessionTitle(t *testing.T) {
	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-task",
		},
	}
	title := sessionTitle(task)
	if !strings.HasPrefix(title, "my-task-") {
		t.Errorf("sessionTitle should start with 'my-task-', got %q", title)
	}
	// 8 hex chars after the dash
	suffix := strings.TrimPrefix(title, "my-task-")
	if len(suffix) != 8 {
		t.Errorf("sessionTitle suffix should be 8 hex chars, got %q (len=%d)", suffix, len(suffix))
	}
}

func TestBuildProxyEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		proxy    *kubeopenv1alpha1.ProxyConfig
		wantEnvs map[string]string // expected env var name -> value
		wantNil  bool
	}{
		{
			name:    "nil proxy returns nil",
			proxy:   nil,
			wantNil: true,
		},
		{
			name: "only httpProxy set",
			proxy: &kubeopenv1alpha1.ProxyConfig{
				HttpProxy: "http://proxy:8080",
			},
			wantEnvs: map[string]string{
				"HTTP_PROXY": "http://proxy:8080",
				"http_proxy": "http://proxy:8080",
				"NO_PROXY":   ".svc,.cluster.local",
				"no_proxy":   ".svc,.cluster.local",
			},
		},
		{
			name: "only httpsProxy set",
			proxy: &kubeopenv1alpha1.ProxyConfig{
				HttpsProxy: "http://proxy:8443",
			},
			wantEnvs: map[string]string{
				"HTTPS_PROXY": "http://proxy:8443",
				"https_proxy": "http://proxy:8443",
				"NO_PROXY":    ".svc,.cluster.local",
				"no_proxy":    ".svc,.cluster.local",
			},
		},
		{
			name: "full config with all three fields",
			proxy: &kubeopenv1alpha1.ProxyConfig{
				HttpProxy:  "http://proxy:8080",
				HttpsProxy: "http://proxy:8443",
				NoProxy:    "localhost,127.0.0.1",
			},
			wantEnvs: map[string]string{
				"HTTP_PROXY":  "http://proxy:8080",
				"http_proxy":  "http://proxy:8080",
				"HTTPS_PROXY": "http://proxy:8443",
				"https_proxy": "http://proxy:8443",
				"NO_PROXY":    "localhost,127.0.0.1,.svc,.cluster.local",
				"no_proxy":    "localhost,127.0.0.1,.svc,.cluster.local",
			},
		},
		{
			name: "noProxy auto-appends .svc,.cluster.local when not present",
			proxy: &kubeopenv1alpha1.ProxyConfig{
				HttpProxy: "http://proxy:8080",
				NoProxy:   "10.0.0.0/8,172.16.0.0/12",
			},
			wantEnvs: map[string]string{
				"NO_PROXY": "10.0.0.0/8,172.16.0.0/12,.svc,.cluster.local",
				"no_proxy": "10.0.0.0/8,172.16.0.0/12,.svc,.cluster.local",
			},
		},
		{
			name: "noProxy does not duplicate .svc if already present",
			proxy: &kubeopenv1alpha1.ProxyConfig{
				HttpProxy: "http://proxy:8080",
				NoProxy:   "localhost,.svc,.cluster.local",
			},
			wantEnvs: map[string]string{
				"NO_PROXY": "localhost,.svc,.cluster.local",
				"no_proxy": "localhost,.svc,.cluster.local",
			},
		},
		{
			name: "noProxy has .cluster.local but not .svc appends only .svc",
			proxy: &kubeopenv1alpha1.ProxyConfig{
				HttpProxy: "http://proxy:8080",
				NoProxy:   "localhost,.cluster.local",
			},
			wantEnvs: map[string]string{
				"NO_PROXY": "localhost,.cluster.local,.svc",
				"no_proxy": "localhost,.cluster.local,.svc",
			},
		},
		{
			name: "empty noProxy defaults to .svc,.cluster.local",
			proxy: &kubeopenv1alpha1.ProxyConfig{
				HttpProxy: "http://proxy:8080",
				NoProxy:   "",
			},
			wantEnvs: map[string]string{
				"NO_PROXY": ".svc,.cluster.local",
				"no_proxy": ".svc,.cluster.local",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildProxyEnvVars(tt.proxy)
			if tt.wantNil {
				if got != nil {
					t.Errorf("buildProxyEnvVars() = %v, want nil", got)
				}
				return
			}
			envMap := make(map[string]string)
			for _, env := range got {
				envMap[env.Name] = env.Value
			}
			for wantName, wantValue := range tt.wantEnvs {
				if gotValue, ok := envMap[wantName]; !ok {
					t.Errorf("missing env var %q", wantName)
				} else if gotValue != wantValue {
					t.Errorf("env %q = %q, want %q", wantName, gotValue, wantValue)
				}
			}
		})
	}
}

func TestBuildPodWithProxy(t *testing.T) {
	tests := []struct {
		name      string
		proxy     *kubeopenv1alpha1.ProxyConfig
		wantProxy bool
	}{
		{
			name: "proxy set - all containers have proxy env vars",
			proxy: &kubeopenv1alpha1.ProxyConfig{
				HttpProxy:  "http://proxy:8080",
				HttpsProxy: "http://proxy:8443",
				NoProxy:    "localhost",
			},
			wantProxy: true,
		},
		{
			name:      "proxy nil - no proxy env vars",
			proxy:     nil,
			wantProxy: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-task",
					Namespace: "default",
					UID:       "test-uid",
				},
			}
			task.APIVersion = "kubeopencode.io/v1alpha1"
			task.Kind = "Task"

			cfg := agentConfig{
				agentImage:         "test-opencode:v1.0.0",
				executorImage:      "test-executor:v1.0.0",
				workspaceDir:       "/workspace",
				serviceAccountName: "test-sa",
				proxy:              tt.proxy,
			}

			pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

			hasProxyEnv := func(envs []corev1.EnvVar) bool {
				for _, env := range envs {
					if env.Name == "HTTP_PROXY" || env.Name == "HTTPS_PROXY" {
						return true
					}
				}
				return false
			}

			// Check all init containers
			for _, ic := range pod.Spec.InitContainers {
				if hasProxyEnv(ic.Env) != tt.wantProxy {
					t.Errorf("init container %q: hasProxy = %v, want %v", ic.Name, !tt.wantProxy, tt.wantProxy)
				}
			}

			// Check worker container
			container := pod.Spec.Containers[0]
			if hasProxyEnv(container.Env) != tt.wantProxy {
				t.Errorf("worker container: hasProxy = %v, want %v", !tt.wantProxy, tt.wantProxy)
			}
		})
	}
}

func TestBuildPodWithImagePullSecrets(t *testing.T) {
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
				{Name: "dockerhub-secret"},
			},
			wantCount: 3,
		},
		{
			name:             "empty list - no imagePullSecrets",
			imagePullSecrets: nil,
			wantCount:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-task",
					Namespace: "default",
					UID:       "test-uid",
				},
			}
			task.APIVersion = "kubeopencode.io/v1alpha1"
			task.Kind = "Task"

			cfg := agentConfig{
				agentImage:         "test-opencode:v1.0.0",
				executorImage:      "test-executor:v1.0.0",
				workspaceDir:       "/workspace",
				serviceAccountName: "test-sa",
				imagePullSecrets:   tt.imagePullSecrets,
			}

			pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

			if tt.wantCount == 0 {
				if len(pod.Spec.ImagePullSecrets) != 0 {
					t.Errorf("ImagePullSecrets count = %d, want 0", len(pod.Spec.ImagePullSecrets))
				}
				return
			}

			if len(pod.Spec.ImagePullSecrets) != tt.wantCount {
				t.Fatalf("ImagePullSecrets count = %d, want %d", len(pod.Spec.ImagePullSecrets), tt.wantCount)
			}

			for i, secret := range tt.imagePullSecrets {
				if pod.Spec.ImagePullSecrets[i].Name != secret.Name {
					t.Errorf("ImagePullSecrets[%d].Name = %q, want %q", i, pod.Spec.ImagePullSecrets[i].Name, secret.Name)
				}
			}
		})
	}
}

func TestDefaultSecurityContext(t *testing.T) {
	sc := defaultSecurityContext()

	if sc == nil {
		t.Fatal("defaultSecurityContext() returned nil")
	}

	// AllowPrivilegeEscalation: false
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation != false {
		t.Errorf("AllowPrivilegeEscalation = %v, want false", sc.AllowPrivilegeEscalation)
	}

	// Capabilities: Drop ALL
	if sc.Capabilities == nil {
		t.Fatal("Capabilities should not be nil")
	}
	if len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("Capabilities.Drop = %v, want [ALL]", sc.Capabilities.Drop)
	}

	// SeccompProfile: RuntimeDefault
	if sc.SeccompProfile == nil {
		t.Fatal("SeccompProfile should not be nil")
	}
	if sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("SeccompProfile.Type = %q, want %q", sc.SeccompProfile.Type, corev1.SeccompProfileTypeRuntimeDefault)
	}
}

func TestBuildPodSecurityContext(t *testing.T) {
	t.Run("no podSpec SecurityContext - agent uses default", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-task",
				Namespace: "default",
				UID:       "test-uid",
			},
		}
		task.APIVersion = "kubeopencode.io/v1alpha1"
		task.Kind = "Task"

		cfg := agentConfig{
			agentImage:         "test-opencode:v1.0.0",
			executorImage:      "test-executor:v1.0.0",
			workspaceDir:       "/workspace",
			serviceAccountName: "test-sa",
		}

		pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

		container := pod.Spec.Containers[0]
		if container.SecurityContext == nil {
			t.Fatal("agent container SecurityContext should not be nil")
		}
		if container.SecurityContext.AllowPrivilegeEscalation == nil || *container.SecurityContext.AllowPrivilegeEscalation != false {
			t.Errorf("default SecurityContext AllowPrivilegeEscalation should be false")
		}
		if container.SecurityContext.Capabilities == nil || len(container.SecurityContext.Capabilities.Drop) == 0 {
			t.Errorf("default SecurityContext should drop ALL capabilities")
		}
	})

	t.Run("custom podSpec SecurityContext - agent uses custom", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-task",
				Namespace: "default",
				UID:       "test-uid",
			},
		}
		task.APIVersion = "kubeopencode.io/v1alpha1"
		task.Kind = "Task"

		runAsNonRoot := true
		cfg := agentConfig{
			agentImage:         "test-opencode:v1.0.0",
			executorImage:      "test-executor:v1.0.0",
			workspaceDir:       "/workspace",
			serviceAccountName: "test-sa",
			podSpec: &kubeopenv1alpha1.AgentPodSpec{
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot: &runAsNonRoot,
				},
			},
		}

		pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

		container := pod.Spec.Containers[0]
		if container.SecurityContext == nil {
			t.Fatal("agent container SecurityContext should not be nil")
		}
		if container.SecurityContext.RunAsNonRoot == nil || *container.SecurityContext.RunAsNonRoot != true {
			t.Errorf("custom SecurityContext RunAsNonRoot should be true")
		}
		// Should NOT have the default fields since custom overrides entirely
		if container.SecurityContext.AllowPrivilegeEscalation != nil {
			t.Errorf("custom SecurityContext should not have AllowPrivilegeEscalation from default")
		}
	})

	t.Run("init containers always get default security context", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-task",
				Namespace: "default",
				UID:       "test-uid",
			},
		}
		task.APIVersion = "kubeopencode.io/v1alpha1"
		task.Kind = "Task"

		cfg := agentConfig{
			agentImage:         "test-opencode:v1.0.0",
			executorImage:      "test-executor:v1.0.0",
			workspaceDir:       "/workspace",
			serviceAccountName: "test-sa",
		}

		pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

		for _, ic := range pod.Spec.InitContainers {
			if ic.SecurityContext == nil {
				t.Errorf("init container %q SecurityContext should not be nil", ic.Name)
				continue
			}
			if ic.SecurityContext.AllowPrivilegeEscalation == nil || *ic.SecurityContext.AllowPrivilegeEscalation != false {
				t.Errorf("init container %q AllowPrivilegeEscalation should be false", ic.Name)
			}
		}
	})

	t.Run("lifecycle hook is applied to agent container", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-task",
				Namespace: "default",
				UID:       "test-uid",
			},
		}
		task.APIVersion = "kubeopencode.io/v1alpha1"
		task.Kind = "Task"

		cfg := agentConfig{
			agentImage:         "test-opencode:v1.0.0",
			executorImage:      "test-executor:v1.0.0",
			workspaceDir:       "/workspace",
			serviceAccountName: "test-sa",
			podSpec: &kubeopenv1alpha1.AgentPodSpec{
				Lifecycle: &corev1.Lifecycle{
					PostStart: &corev1.LifecycleHandler{
						Exec: &corev1.ExecAction{
							Command: []string{"/usr/local/bin/start-code-server.sh"},
						},
					},
				},
			},
		}

		pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

		container := pod.Spec.Containers[0]
		if container.Lifecycle == nil {
			t.Fatal("expected Lifecycle to be set on agent container")
		}
		if container.Lifecycle.PostStart == nil || container.Lifecycle.PostStart.Exec == nil {
			t.Fatal("expected PostStart.Exec to be set")
		}
		if container.Lifecycle.PostStart.Exec.Command[0] != "/usr/local/bin/start-code-server.sh" {
			t.Errorf("unexpected PostStart command: %v", container.Lifecycle.PostStart.Exec.Command)
		}
	})

	t.Run("no lifecycle when podSpec has no lifecycle", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-task",
				Namespace: "default",
				UID:       "test-uid",
			},
		}
		task.APIVersion = "kubeopencode.io/v1alpha1"
		task.Kind = "Task"

		cfg := agentConfig{
			agentImage:         "test-opencode:v1.0.0",
			executorImage:      "test-executor:v1.0.0",
			workspaceDir:       "/workspace",
			serviceAccountName: "test-sa",
		}

		pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

		container := pod.Spec.Containers[0]
		if container.Lifecycle != nil {
			t.Errorf("expected Lifecycle to be nil when not configured, got %v", container.Lifecycle)
		}
	})

	t.Run("podSpec PodSecurityContext is set on pod spec", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-task",
				Namespace: "default",
				UID:       "test-uid",
			},
		}
		task.APIVersion = "kubeopencode.io/v1alpha1"
		task.Kind = "Task"

		var runAsUser int64 = 1000
		var fsGroup int64 = 1000
		cfg := agentConfig{
			agentImage:         "test-opencode:v1.0.0",
			executorImage:      "test-executor:v1.0.0",
			workspaceDir:       "/workspace",
			serviceAccountName: "test-sa",
			podSpec: &kubeopenv1alpha1.AgentPodSpec{
				PodSecurityContext: &corev1.PodSecurityContext{
					RunAsUser: &runAsUser,
					FSGroup:   &fsGroup,
				},
			},
		}

		pod := buildPod(task, "test-task-pod", cfg, nil, nil, nil, nil, defaultSystemConfig(), "")

		if pod.Spec.SecurityContext == nil {
			t.Fatal("Pod.Spec.SecurityContext should not be nil")
		}
		if pod.Spec.SecurityContext.RunAsUser == nil || *pod.Spec.SecurityContext.RunAsUser != 1000 {
			t.Errorf("PodSecurityContext.RunAsUser = %v, want 1000", pod.Spec.SecurityContext.RunAsUser)
		}
		if pod.Spec.SecurityContext.FSGroup == nil || *pod.Spec.SecurityContext.FSGroup != 1000 {
			t.Errorf("PodSecurityContext.FSGroup = %v, want 1000", pod.Spec.SecurityContext.FSGroup)
		}
	})
}
