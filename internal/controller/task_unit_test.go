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

func TestIsTaskFinished(t *testing.T) {
	tests := []struct {
		name     string
		phase    kubeopenv1alpha1.TaskPhase
		expected bool
	}{
		{"empty phase", "", false},
		{"pending", kubeopenv1alpha1.TaskPhasePending, false},
		{"running", kubeopenv1alpha1.TaskPhaseRunning, false},
		{"queued", kubeopenv1alpha1.TaskPhaseQueued, false},
		{"completed", kubeopenv1alpha1.TaskPhaseCompleted, true},
		{"failed", kubeopenv1alpha1.TaskPhaseFailed, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTaskFinished(tt.phase); got != tt.expected {
				t.Errorf("isTaskFinished(%q) = %v, want %v", tt.phase, got, tt.expected)
			}
		})
	}
}

func TestIsTaskStoppedByUser(t *testing.T) {
	t.Run("nil annotations", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{}
		if isTaskStoppedByUser(task) {
			t.Error("expected false when annotations are nil")
		}
	})

	t.Run("no stop annotation", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"other": "value"},
			},
		}
		if isTaskStoppedByUser(task) {
			t.Error("expected false when stop annotation is missing")
		}
	})

	t.Run("stop annotation set to true", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{AnnotationStop: "true"},
			},
		}
		if !isTaskStoppedByUser(task) {
			t.Error("expected true when stop annotation is 'true'")
		}
	})

	t.Run("stop annotation set to false", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{AnnotationStop: "false"},
			},
		}
		if isTaskStoppedByUser(task) {
			t.Error("expected false when stop annotation is 'false'")
		}
	})

	t.Run("stop annotation empty", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{AnnotationStop: ""},
			},
		}
		if isTaskStoppedByUser(task) {
			t.Error("expected false when stop annotation is empty")
		}
	})
}

func TestIsTaskTimedOut(t *testing.T) {
	t.Run("no timeout configured", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			Status: kubeopenv1alpha1.TaskExecutionStatus{
				StartTime: &metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
			},
		}
		if isTaskTimedOut(task) {
			t.Error("expected false when no timeout is configured")
		}
	})

	t.Run("no start time", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			Spec: kubeopenv1alpha1.TaskSpec{
				Timeout: &metav1.Duration{Duration: 30 * time.Minute},
			},
		}
		if isTaskTimedOut(task) {
			t.Error("expected false when start time is not set (task still in queue)")
		}
	})

	t.Run("both nil", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{}
		if isTaskTimedOut(task) {
			t.Error("expected false when both timeout and start time are nil")
		}
	})

	t.Run("within timeout window", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			Spec: kubeopenv1alpha1.TaskSpec{
				Timeout: &metav1.Duration{Duration: 30 * time.Minute},
			},
			Status: kubeopenv1alpha1.TaskExecutionStatus{
				StartTime: &metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
			},
		}
		if isTaskTimedOut(task) {
			t.Error("expected false when elapsed (10m) < timeout (30m)")
		}
	})

	t.Run("exactly at timeout boundary", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			Spec: kubeopenv1alpha1.TaskSpec{
				Timeout: &metav1.Duration{Duration: 30 * time.Minute},
			},
			Status: kubeopenv1alpha1.TaskExecutionStatus{
				// Start time is exactly 30 minutes ago; elapsed >= timeout → timed out
				StartTime: &metav1.Time{Time: time.Now().Add(-30 * time.Minute)},
			},
		}
		if !isTaskTimedOut(task) {
			t.Error("expected true when elapsed exactly equals timeout")
		}
	})

	t.Run("exceeded timeout", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			Spec: kubeopenv1alpha1.TaskSpec{
				Timeout: &metav1.Duration{Duration: 30 * time.Minute},
			},
			Status: kubeopenv1alpha1.TaskExecutionStatus{
				StartTime: &metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
			},
		}
		if !isTaskTimedOut(task) {
			t.Error("expected true when elapsed (1h) > timeout (30m)")
		}
	})

	t.Run("queue time excluded", func(t *testing.T) {
		// Task was created 2 hours ago but started 10 minutes ago.
		// Timeout is 30 minutes. Should NOT be timed out because
		// timeout is measured from startTime, not creation time.
		task := &kubeopenv1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
			},
			Spec: kubeopenv1alpha1.TaskSpec{
				Timeout: &metav1.Duration{Duration: 30 * time.Minute},
			},
			Status: kubeopenv1alpha1.TaskExecutionStatus{
				StartTime: &metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
			},
		}
		if isTaskTimedOut(task) {
			t.Error("expected false: timeout should be measured from startTime (10m ago), not creation time (2h ago)")
		}
	})

	t.Run("very short timeout", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			Spec: kubeopenv1alpha1.TaskSpec{
				Timeout: &metav1.Duration{Duration: 1 * time.Second},
			},
			Status: kubeopenv1alpha1.TaskExecutionStatus{
				StartTime: &metav1.Time{Time: time.Now().Add(-2 * time.Second)},
			},
		}
		if !isTaskTimedOut(task) {
			t.Error("expected true for 1-second timeout with 2-second elapsed")
		}
	})

	t.Run("zero timeout immediately expires", func(t *testing.T) {
		task := &kubeopenv1alpha1.Task{
			Spec: kubeopenv1alpha1.TaskSpec{
				Timeout: &metav1.Duration{Duration: 0},
			},
			Status: kubeopenv1alpha1.TaskExecutionStatus{
				StartTime: &metav1.Time{Time: time.Now()},
			},
		}
		if !isTaskTimedOut(task) {
			t.Error("expected true when timeout is zero (immediately expires)")
		}
	})
}

func TestGetPodFailureDetail(t *testing.T) {

	t.Run("empty when all containers succeeded", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "agent",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 0,
								Reason:   "Completed",
							},
						},
					},
				},
			},
		}
		detail := getPodFailureDetail(pod)
		if detail != "" {
			t.Errorf("expected empty detail, got %q", detail)
		}
	})

	t.Run("returns detail for main container failure", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "agent",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 1,
								Reason:   "Error",
								Message:  "Model not found",
							},
						},
					},
				},
			},
		}
		detail := getPodFailureDetail(pod)
		if detail == "" {
			t.Error("expected non-empty detail")
		}
		if detail != "container agent: exit code 1 (Error: Model not found)" {
			t.Errorf("unexpected detail: %q", detail)
		}
	})

	t.Run("returns detail for init container failure", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "git-init-0",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 128,
								Reason:   "Error",
							},
						},
					},
				},
			},
		}
		detail := getPodFailureDetail(pod)
		if detail == "" {
			t.Error("expected non-empty detail")
		}
		if detail != "container git-init-0: exit code 128 (Error)" {
			t.Errorf("unexpected detail: %q", detail)
		}
	})

	t.Run("returns OOMKilled detail", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "agent",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 137,
								Reason:   "OOMKilled",
							},
						},
					},
				},
			},
		}
		detail := getPodFailureDetail(pod)
		if detail != "container agent: OOMKilled" {
			t.Errorf("unexpected detail: %q", detail)
		}
	})

	t.Run("prioritizes init container failure over main container", func(t *testing.T) {
		pod := &corev1.Pod{
			Status: corev1.PodStatus{
				InitContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "git-init-0",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 1,
								Reason:   "Error",
							},
						},
					},
				},
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "agent",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode: 1,
								Reason:   "Error",
							},
						},
					},
				},
			},
		}
		detail := getPodFailureDetail(pod)
		if detail != "container git-init-0: exit code 1 (Error)" {
			t.Errorf("unexpected detail: %q", detail)
		}
	})
}
