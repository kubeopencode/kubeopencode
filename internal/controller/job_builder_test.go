// Copyright Contributors to the KubeTask project

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kubetaskv1alpha1 "github.com/kubetask/kubetask/api/v1alpha1"
)

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

func TestBuildJob_BasicTask(t *testing.T) {
	task := &kubetaskv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubetask.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-agent:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
	}

	job := buildJob(task, "test-task-job", cfg, nil, nil, nil)

	// Verify job metadata
	if job.Name != "test-task-job" {
		t.Errorf("Job.Name = %q, want %q", job.Name, "test-task-job")
	}
	if job.Namespace != "default" {
		t.Errorf("Job.Namespace = %q, want %q", job.Namespace, "default")
	}

	// Verify labels
	if job.Labels["app"] != "kubetask" {
		t.Errorf("Job.Labels[app] = %q, want %q", job.Labels["app"], "kubetask")
	}
	if job.Labels["kubetask.io/task"] != "test-task" {
		t.Errorf("Job.Labels[kubetask.io/task] = %q, want %q", job.Labels["kubetask.io/task"], "test-task")
	}

	// Verify owner reference
	if len(job.OwnerReferences) != 1 {
		t.Fatalf("len(Job.OwnerReferences) = %d, want 1", len(job.OwnerReferences))
	}
	ownerRef := job.OwnerReferences[0]
	if ownerRef.Name != "test-task" {
		t.Errorf("OwnerReference.Name = %q, want %q", ownerRef.Name, "test-task")
	}
	if ownerRef.Controller == nil || *ownerRef.Controller != true {
		t.Errorf("OwnerReference.Controller = %v, want true", ownerRef.Controller)
	}

	// Verify container
	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("len(Containers) = %d, want 1", len(job.Spec.Template.Spec.Containers))
	}
	container := job.Spec.Template.Spec.Containers[0]
	if container.Name != "agent" {
		t.Errorf("Container.Name = %q, want %q", container.Name, "agent")
	}
	if container.Image != "test-agent:v1.0.0" {
		t.Errorf("Container.Image = %q, want %q", container.Image, "test-agent:v1.0.0")
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
	if job.Spec.Template.Spec.ServiceAccountName != "test-sa" {
		t.Errorf("ServiceAccountName = %q, want %q", job.Spec.Template.Spec.ServiceAccountName, "test-sa")
	}

	// Verify restart policy
	if job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("RestartPolicy = %q, want %q", job.Spec.Template.Spec.RestartPolicy, corev1.RestartPolicyNever)
	}
}

func TestBuildJob_WithCredentials(t *testing.T) {
	task := &kubetaskv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubetask.io/v1alpha1"
	task.Kind = "Task"

	envName := "API_TOKEN"
	mountPath := "/home/agent/.ssh/id_rsa"

	cfg := agentConfig{
		agentImage:         "test-agent:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		credentials: []kubetaskv1alpha1.Credential{
			{
				Name: "api-token",
				SecretRef: kubetaskv1alpha1.SecretReference{
					Name: "my-secret",
					Key:  "token",
				},
				Env: &envName,
			},
			{
				Name: "ssh-key",
				SecretRef: kubetaskv1alpha1.SecretReference{
					Name: "ssh-secret",
					Key:  "private-key",
				},
				MountPath: &mountPath,
			},
		},
	}

	job := buildJob(task, "test-task-job", cfg, nil, nil, nil)

	container := job.Spec.Template.Spec.Containers[0]

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
	for _, vol := range job.Spec.Template.Spec.Volumes {
		if vol.Secret != nil && vol.Secret.SecretName == "ssh-secret" {
			foundVolume = true
		}
	}
	if !foundVolume {
		t.Errorf("Secret volume for ssh-secret not found")
	}
}

func TestBuildJob_WithHumanInTheLoop(t *testing.T) {
	task := &kubetaskv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubetask.io/v1alpha1"
	task.Kind = "Task"

	keepAlive := int32(1800)
	cfg := agentConfig{
		agentImage:         "test-agent:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		command:            []string{"sh", "-c", "echo hello"},
		humanInTheLoop: &kubetaskv1alpha1.HumanInTheLoop{
			Enabled:          true,
			KeepAliveSeconds: &keepAlive,
		},
	}

	job := buildJob(task, "test-task-job", cfg, nil, nil, nil)

	container := job.Spec.Template.Spec.Containers[0]

	// Verify command is wrapped
	if len(container.Command) != 3 {
		t.Fatalf("len(Command) = %d, want 3", len(container.Command))
	}
	if container.Command[0] != "sh" {
		t.Errorf("Command[0] = %q, want %q", container.Command[0], "sh")
	}
	if container.Command[1] != "-c" {
		t.Errorf("Command[1] = %q, want %q", container.Command[1], "-c")
	}

	// Verify wrapped script contains sleep
	script := container.Command[2]
	if !contains(script, "sleep 1800") {
		t.Errorf("Command script should contain 'sleep 1800', got: %s", script)
	}
	if !contains(script, "Human-in-the-loop") {
		t.Errorf("Command script should contain 'Human-in-the-loop', got: %s", script)
	}
	if !contains(script, "sh -c echo hello") {
		t.Errorf("Command script should contain original command 'sh -c echo hello', got: %s", script)
	}

	// Verify keep-alive env var
	var foundKeepAliveEnv bool
	for _, env := range container.Env {
		if env.Name == EnvHumanInTheLoopKeepAlive {
			foundKeepAliveEnv = true
			if env.Value != "1800" {
				t.Errorf("KUBETASK_KEEP_ALIVE_SECONDS = %q, want %q", env.Value, "1800")
			}
		}
	}
	if !foundKeepAliveEnv {
		t.Errorf("KUBETASK_KEEP_ALIVE_SECONDS env not found")
	}
}

func TestBuildJob_WithPodScheduling(t *testing.T) {
	task := &kubetaskv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubetask.io/v1alpha1"
	task.Kind = "Task"

	runtimeClass := "gvisor"
	cfg := agentConfig{
		agentImage:         "test-agent:v1.0.0",
		workspaceDir:       "/workspace",
		serviceAccountName: "test-sa",
		podSpec: &kubetaskv1alpha1.AgentPodSpec{
			Labels: map[string]string{
				"custom-label": "custom-value",
			},
			Scheduling: &kubetaskv1alpha1.PodScheduling{
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

	job := buildJob(task, "test-task-job", cfg, nil, nil, nil)

	podSpec := job.Spec.Template.Spec

	// Verify node selector
	if podSpec.NodeSelector["node-type"] != "gpu" {
		t.Errorf("NodeSelector[node-type] = %q, want %q", podSpec.NodeSelector["node-type"], "gpu")
	}

	// Verify tolerations
	if len(podSpec.Tolerations) != 1 {
		t.Fatalf("len(Tolerations) = %d, want 1", len(podSpec.Tolerations))
	}
	if podSpec.Tolerations[0].Key != "dedicated" {
		t.Errorf("Tolerations[0].Key = %q, want %q", podSpec.Tolerations[0].Key, "dedicated")
	}

	// Verify runtime class
	if podSpec.RuntimeClassName == nil || *podSpec.RuntimeClassName != "gvisor" {
		t.Errorf("RuntimeClassName = %v, want %q", podSpec.RuntimeClassName, "gvisor")
	}

	// Verify custom label on pod template
	podLabels := job.Spec.Template.ObjectMeta.Labels
	if podLabels["custom-label"] != "custom-value" {
		t.Errorf("PodLabels[custom-label] = %q, want %q", podLabels["custom-label"], "custom-value")
	}
	// Verify base labels are still present
	if podLabels["app"] != "kubetask" {
		t.Errorf("PodLabels[app] = %q, want %q", podLabels["app"], "kubetask")
	}
}

func TestBuildJob_WithContextConfigMap(t *testing.T) {
	task := &kubetaskv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubetask.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-agent:v1.0.0",
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

	job := buildJob(task, "test-task-job", cfg, contextConfigMap, fileMounts, nil)

	// Verify context-files volume exists
	var foundContextVolume bool
	for _, vol := range job.Spec.Template.Spec.Volumes {
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

	// Verify volume mount exists
	container := job.Spec.Template.Spec.Containers[0]
	var foundMount bool
	for _, mount := range container.VolumeMounts {
		if mount.MountPath == "/workspace/task.md" {
			foundMount = true
			if mount.SubPath != "workspace-task.md" {
				t.Errorf("VolumeMount.SubPath = %q, want %q", mount.SubPath, "workspace-task.md")
			}
		}
	}
	if !foundMount {
		t.Errorf("Volume mount for /workspace/task.md not found")
	}
}

func TestBuildJob_WithDirMounts(t *testing.T) {
	task := &kubetaskv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-task",
			Namespace: "default",
			UID:       types.UID("test-uid"),
		},
	}
	task.APIVersion = "kubetask.io/v1alpha1"
	task.Kind = "Task"

	cfg := agentConfig{
		agentImage:         "test-agent:v1.0.0",
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

	job := buildJob(task, "test-task-job", cfg, nil, nil, dirMounts)

	// Verify dir-mount volume exists
	var foundDirVolume bool
	for _, vol := range job.Spec.Template.Spec.Volumes {
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

	// Verify volume mount exists
	container := job.Spec.Template.Spec.Containers[0]
	var foundMount bool
	for _, mount := range container.VolumeMounts {
		if mount.MountPath == "/workspace/guides" {
			foundMount = true
		}
	}
	if !foundMount {
		t.Errorf("Volume mount for /workspace/guides not found")
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
