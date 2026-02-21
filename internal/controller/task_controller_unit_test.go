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

func TestRetryBackoffDelay(t *testing.T) {
	tests := []struct {
		name    string
		attempt int32
		profile kubeopenv1alpha1.RetryProfile
		wantMin time.Duration
		wantMax time.Duration
	}{
		{"linear attempt 1", 1, kubeopenv1alpha1.RetryProfileLinear, 29 * time.Second, 31 * time.Second},
		{"linear attempt 2", 2, kubeopenv1alpha1.RetryProfileLinear, 29 * time.Second, 31 * time.Second},
		{"exponential attempt 1", 1, kubeopenv1alpha1.RetryProfileExponential, 4 * time.Second, 6 * time.Second},
		{"exponential attempt 2", 2, kubeopenv1alpha1.RetryProfileExponential, 9 * time.Second, 11 * time.Second},
		{"exponential attempt 3", 3, kubeopenv1alpha1.RetryProfileExponential, 19 * time.Second, 21 * time.Second},
		{"default profile", 1, "", 29 * time.Second, 31 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retryBackoffDelay(tt.attempt, tt.profile)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("retryBackoffDelay(%d, %q) = %v, want between %v and %v", tt.attempt, tt.profile, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestTerminalReasonFromPod(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		want kubeopenv1alpha1.TerminalReasonCode
	}{
		{
			name: "empty container statuses",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{}},
			},
			want: kubeopenv1alpha1.TerminalReasonUnknown,
		},
		{
			name: "OOMKilled",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								Reason: "OOMKilled", ExitCode: 137,
							},
						},
					}},
				},
			},
			want: kubeopenv1alpha1.TerminalReasonInfrastructureError,
		},
		{
			name: "Error exit non-zero",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								Reason: "Error", ExitCode: 1, Message: "agent failed",
							},
						},
					}},
				},
			},
			want: kubeopenv1alpha1.TerminalReasonAgentExitNonZero,
		},
		{
			name: "unknown reason",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{{
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								Reason: "SomeReason", ExitCode: 1,
							},
						},
					}},
				},
			},
			want: kubeopenv1alpha1.TerminalReasonUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pod.ObjectMeta = metav1.ObjectMeta{Name: "test-pod"}
			got := terminalReasonFromPod(tt.pod)
			if got == nil {
				t.Fatal("terminalReasonFromPod returned nil")
			}
			if got.Code != tt.want {
				t.Errorf("terminalReasonFromPod() Code = %v, want %v", got.Code, tt.want)
			}
		})
	}
}
