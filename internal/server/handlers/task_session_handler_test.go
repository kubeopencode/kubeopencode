// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

func TestResolveTaskSessionURL(t *testing.T) {
	const clusterDomain = "cluster.local"

	tests := []struct {
		name          string
		namespace     string
		taskName      string
		objects       []runtime.Object
		wantErr       string
		wantSessionID string
	}{
		{
			name:      "task not found",
			namespace: "default",
			taskName:  "missing",
			objects:   []runtime.Object{},
			wantErr:   "task not found",
		},
		{
			name:      "task has no session info",
			namespace: "default",
			taskName:  "my-task",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "my-task", Namespace: "default"},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
					Status: kubeopenv1alpha1.TaskExecutionStatus{
						Phase: kubeopenv1alpha1.TaskPhaseRunning,
						// No Session field
					},
				},
			},
			wantErr: "has no session information",
		},
		{
			name:      "task has no agent reference",
			namespace: "default",
			taskName:  "my-task",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "my-task", Namespace: "default"},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
					Status: kubeopenv1alpha1.TaskExecutionStatus{
						Phase: kubeopenv1alpha1.TaskPhaseCompleted,
						Session: &kubeopenv1alpha1.SessionInfo{
							ID: "session-123",
						},
						// No AgentRef in status
					},
				},
			},
			wantErr: "has no agent reference",
		},
		{
			name:      "agent not found",
			namespace: "default",
			taskName:  "my-task",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "my-task", Namespace: "default"},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
					Status: kubeopenv1alpha1.TaskExecutionStatus{
						Phase: kubeopenv1alpha1.TaskPhaseCompleted,
						Session: &kubeopenv1alpha1.SessionInfo{
							ID: "session-123",
						},
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
				},
			},
			wantErr: "agent not found",
		},
		{
			name:      "agent has no URL",
			namespace: "default",
			taskName:  "my-task",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "my-task", Namespace: "default"},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
					Status: kubeopenv1alpha1.TaskExecutionStatus{
						Phase: kubeopenv1alpha1.TaskPhaseCompleted,
						Session: &kubeopenv1alpha1.SessionInfo{
							ID: "session-123",
						},
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
				},
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
					Spec:       kubeopenv1alpha1.AgentSpec{WorkspaceDir: "/workspace"},
					Status: kubeopenv1alpha1.AgentStatus{
						Ready: true,
						URL:   "", // No URL
					},
				},
			},
			wantErr: "has no server URL",
		},
		{
			name:      "successful resolution",
			namespace: "default",
			taskName:  "my-task",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "my-task", Namespace: "default"},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
					Status: kubeopenv1alpha1.TaskExecutionStatus{
						Phase: kubeopenv1alpha1.TaskPhaseCompleted,
						Session: &kubeopenv1alpha1.SessionInfo{
							ID: "session-456",
						},
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
				},
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
					Spec:       kubeopenv1alpha1.AgentSpec{WorkspaceDir: "/workspace"},
					Status: kubeopenv1alpha1.AgentStatus{
						Ready: true,
						URL:   "http://my-agent-server.default.svc.cluster.local:4096",
					},
				},
			},
			wantSessionID: "session-456",
		},
		{
			name:      "works with suspended agent (read-only access)",
			namespace: "default",
			taskName:  "my-task",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "my-task", Namespace: "default"},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
					Status: kubeopenv1alpha1.TaskExecutionStatus{
						Phase: kubeopenv1alpha1.TaskPhaseCompleted,
						Session: &kubeopenv1alpha1.SessionInfo{
							ID: "session-789",
						},
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
				},
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
					Spec:       kubeopenv1alpha1.AgentSpec{WorkspaceDir: "/workspace", Suspend: true},
					Status: kubeopenv1alpha1.AgentStatus{
						Ready:     false,
						Suspended: true,
						URL:       "http://my-agent-server.default.svc.cluster.local:4096",
					},
				},
			},
			wantSessionID: "session-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tt.objects...).
				WithStatusSubresource(&kubeopenv1alpha1.Task{}, &kubeopenv1alpha1.Agent{}).
				Build()

			handler := NewTaskSessionHandler(k8sClient, clusterDomain)
			_, sessionID, err := handler.resolveTaskSessionURL(context.Background(), tt.namespace, tt.taskName)

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
			if sessionID != tt.wantSessionID {
				t.Errorf("sessionID = %q, want %q", sessionID, tt.wantSessionID)
			}
		})
	}
}
