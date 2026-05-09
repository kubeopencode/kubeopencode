// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"context"
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

func TestCheckSameOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{
			name:   "no origin header (non-browser client)",
			origin: "",
			host:   "example.com",
			want:   true,
		},
		{
			name:   "same origin",
			origin: "http://example.com",
			host:   "example.com",
			want:   true,
		},
		{
			name:   "same origin with port",
			origin: "http://localhost:2746",
			host:   "localhost:2746",
			want:   true,
		},
		{
			name:   "different origin",
			origin: "http://evil.com",
			host:   "example.com",
			want:   false,
		},
		{
			name:   "different port is different origin",
			origin: "http://localhost:3000",
			host:   "localhost:2746",
			want:   false,
		},
		{
			name:   "invalid origin URL",
			origin: "://invalid",
			host:   "example.com",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{
				Header: make(http.Header),
				Host:   tt.host,
			}
			if tt.origin != "" {
				r.Header.Set("Origin", tt.origin)
			}

			got := checkSameOrigin(r)
			if got != tt.want {
				t.Errorf("checkSameOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTransientExecError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "exit code 137 (SIGKILL)",
			err:  &testError{msg: "command terminated with exit code 137"},
			want: true,
		},
		{
			name: "exit code 1 (not transient)",
			err:  &testError{msg: "command terminated with exit code 1"},
			want: false,
		},
		{
			name: "connection refused (not transient)",
			err:  &testError{msg: "connection refused"},
			want: false,
		},
		{
			name: "exit code 137 in wrapped error",
			err:  &testError{msg: "exec failed: exit code 137: something went wrong"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientExecError(tt.err)
			if got != tt.want {
				t.Errorf("isTransientExecError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestResolveAgentServerPod(t *testing.T) {
	tests := []struct {
		name          string
		agentName     string
		namespace     string
		objects       []runtime.Object
		wantErr       string
		wantPod       string
		wantContainer string
	}{
		{
			name:      "agent not found",
			agentName: "missing",
			namespace: "default",
			objects:   []runtime.Object{},
			wantErr:   "agent not found",
		},
		{
			name:      "agent is suspended",
			agentName: "my-agent",
			namespace: "default",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
					Spec:       kubeopenv1alpha1.AgentSpec{WorkspaceDir: "/workspace"},
					Status: kubeopenv1alpha1.AgentStatus{
						Suspended: true,
					},
				},
			},
			wantErr: "is suspended",
		},
		{
			name:      "agent not ready",
			agentName: "my-agent",
			namespace: "default",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
					Spec:       kubeopenv1alpha1.AgentSpec{WorkspaceDir: "/workspace"},
					Status: kubeopenv1alpha1.AgentStatus{
						Ready: false,
					},
				},
			},
			wantErr: "is not ready",
		},
		{
			name:      "no ready pods",
			agentName: "my-agent",
			namespace: "default",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
					Spec:       kubeopenv1alpha1.AgentSpec{WorkspaceDir: "/workspace"},
					Status: kubeopenv1alpha1.AgentStatus{
						Ready: true,
					},
				},
			},
			wantErr: "no ready server pod found",
		},
		{
			name:      "finds ready pod",
			agentName: "my-agent",
			namespace: "default",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
					Spec:       kubeopenv1alpha1.AgentSpec{WorkspaceDir: "/workspace"},
					Status: kubeopenv1alpha1.AgentStatus{
						Ready: true,
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-agent-server-abc123",
						Namespace: "default",
						Labels: map[string]string{
							"app.kubernetes.io/name":      "kubeopencode-server",
							"app.kubernetes.io/instance":  "my-agent",
							"app.kubernetes.io/component": "server",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			wantPod:       "my-agent-server-abc123",
			wantContainer: "opencode-server",
		},
		{
			name:      "skips non-running pod",
			agentName: "my-agent",
			namespace: "default",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
					Spec:       kubeopenv1alpha1.AgentSpec{WorkspaceDir: "/workspace"},
					Status: kubeopenv1alpha1.AgentStatus{
						Ready: true,
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-agent-server-pending",
						Namespace: "default",
						Labels: map[string]string{
							"app.kubernetes.io/name":      "kubeopencode-server",
							"app.kubernetes.io/instance":  "my-agent",
							"app.kubernetes.io/component": "server",
						},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodPending,
					},
				},
			},
			wantErr: "no ready server pod found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tt.objects...).
				WithStatusSubresource(&kubeopenv1alpha1.Agent{}).
				Build()

			podName, containerName, _, err := resolveAgentServerPod(context.Background(), k8sClient, tt.namespace, tt.agentName)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !containsSubstring(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if podName != tt.wantPod {
				t.Errorf("podName = %q, want %q", podName, tt.wantPod)
			}
			if containerName != tt.wantContainer {
				t.Errorf("containerName = %q, want %q", containerName, tt.wantContainer)
			}
		})
	}
}

func TestTerminalSizeQueue(t *testing.T) {
	q := &terminalSizeQueue{ch: make(chan *remotecommand.TerminalSize, 1)}

	// Send a size
	q.ch <- &remotecommand.TerminalSize{Width: 120, Height: 40}

	// Receive it
	size := q.Next()
	if size == nil {
		t.Fatal("expected non-nil size")
	}
	if size.Width != 120 || size.Height != 40 {
		t.Errorf("size = %dx%d, want 120x40", size.Width, size.Height)
	}

	// Close and verify nil return
	close(q.ch)
	size = q.Next()
	if size != nil {
		t.Errorf("expected nil after close, got %v", size)
	}
}
