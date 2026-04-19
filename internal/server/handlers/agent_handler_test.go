// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

func TestAgentHandler_ListAll(t *testing.T) {
	tests := []struct {
		name       string
		objects    []runtime.Object
		wantTotal  int
		wantStatus int
	}{
		{
			name: "returns agents across namespaces",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "agent-1", Namespace: "default"},
					Spec:       kubeopenv1alpha1.AgentSpec{WorkspaceDir: "/workspace"},
				},
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "agent-2", Namespace: "production"},
					Spec:       kubeopenv1alpha1.AgentSpec{WorkspaceDir: "/workspace"},
				},
			},
			wantTotal:  2,
			wantStatus: http.StatusOK,
		},
		{
			name:       "returns empty list",
			objects:    []runtime.Object{},
			wantTotal:  0,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tt.objects...).
				Build()
			handler := NewAgentHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
			r.URL = &url.URL{Path: "/api/v1/agents"}

			handler.ListAll(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			var resp types.AgentListResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Total != tt.wantTotal {
				t.Errorf("expected total %d, got %d", tt.wantTotal, resp.Total)
			}

			if resp.Pagination == nil {
				t.Fatal("expected non-nil pagination")
			}
		})
	}
}

func TestAgentHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		agentName  string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:      "returns agent",
			namespace: "default",
			agentName: "my-agent",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
					Spec: kubeopenv1alpha1.AgentSpec{
						WorkspaceDir:       "/workspace",
						ServiceAccountName: "sa",
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "handles not found",
			namespace:  "default",
			agentName:  "missing",
			objects:    []runtime.Object{},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tt.objects...).
				Build()
			handler := NewAgentHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", tt.namespace)
			rctx.URLParams.Add("name", tt.agentName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Get(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp types.AgentResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Name != tt.agentName {
					t.Errorf("expected name %q, got %q", tt.agentName, resp.Name)
				}
				if resp.Namespace != tt.namespace {
					t.Errorf("expected namespace %q, got %q", tt.namespace, resp.Namespace)
				}
				if resp.WorkspaceDir != "/workspace" {
					t.Errorf("expected workspaceDir %q, got %q", "/workspace", resp.WorkspaceDir)
				}
			}
		})
	}
}

func TestAgentHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "creates agent",
			body: types.CreateAgentRequest{
				Name:               "new-agent",
				WorkspaceDir:       "/workspace",
				ServiceAccountName: "sa",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "creates agent with templateRef (no workspaceDir/sa required)",
			body: types.CreateAgentRequest{
				Name:        "tmpl-agent",
				TemplateRef: &types.AgentReference{Name: "my-template"},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "validates name required",
			body: types.CreateAgentRequest{
				WorkspaceDir:       "/workspace",
				ServiceAccountName: "sa",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "validates workspaceDir required without template",
			body: types.CreateAgentRequest{
				Name:               "no-ws",
				ServiceAccountName: "sa",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "validates serviceAccountName required without template",
			body: types.CreateAgentRequest{
				Name:         "no-sa",
				WorkspaceDir: "/workspace",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()
			handler := NewAgentHandler(k8sClient)

			bodyBytes, _ := json.Marshal(tt.body)
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", "default")
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Create(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusCreated {
				var resp types.AgentResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Namespace != "default" {
					t.Errorf("expected namespace %q, got %q", "default", resp.Namespace)
				}
			}
		})
	}
}

func TestAgentHandler_Suspend(t *testing.T) {
	tests := []struct {
		name       string
		agentName  string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:      "suspends agent",
			agentName: "my-agent",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
					Spec: kubeopenv1alpha1.AgentSpec{
						WorkspaceDir: "/workspace",
						Suspend:      false,
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "handles not found",
			agentName:  "missing",
			objects:    []runtime.Object{},
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tt.objects...).
				Build()
			handler := NewAgentHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", "default")
			rctx.URLParams.Add("name", tt.agentName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Suspend(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				// Verify the agent spec was updated
				var agent kubeopenv1alpha1.Agent
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: tt.agentName}, &agent); err != nil {
					t.Fatalf("failed to get agent: %v", err)
				}
				if !agent.Spec.Suspend {
					t.Error("expected agent.Spec.Suspend to be true")
				}

				// Verify response reflects suspended state
				var resp types.AgentResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.ServerStatus == nil {
					t.Fatal("expected non-nil ServerStatus")
				}
				if !resp.ServerStatus.Suspended {
					t.Error("expected response ServerStatus.Suspended to be true")
				}
			}
		})
	}
}

func TestAgentHandler_Resume(t *testing.T) {
	tests := []struct {
		name       string
		agentName  string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:      "resumes agent",
			agentName: "my-agent",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "default"},
					Spec: kubeopenv1alpha1.AgentSpec{
						WorkspaceDir: "/workspace",
						Suspend:      true,
					},
					Status: kubeopenv1alpha1.AgentStatus{
						Suspended: true,
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "handles not found",
			agentName:  "missing",
			objects:    []runtime.Object{},
			wantStatus: http.StatusNotFound,
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
			handler := NewAgentHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", "default")
			rctx.URLParams.Add("name", tt.agentName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Resume(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				// Verify the agent spec was updated
				var agent kubeopenv1alpha1.Agent
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: tt.agentName}, &agent); err != nil {
					t.Fatalf("failed to get agent: %v", err)
				}
				if agent.Spec.Suspend {
					t.Error("expected agent.Spec.Suspend to be false")
				}

				// Verify response reflects resumed state
				var resp types.AgentResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.ServerStatus == nil {
					t.Fatal("expected non-nil ServerStatus")
				}
				if resp.ServerStatus.Suspended {
					t.Error("expected response ServerStatus.Suspended to be false")
				}
			}
		})
	}
}

func TestAgentHandler_Suspend_RejectsWithActiveTasks(t *testing.T) {
	scheme := newTestScheme()
	agentName := "busy-agent"

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(
			&kubeopenv1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{Name: agentName, Namespace: "default"},
				Spec: kubeopenv1alpha1.AgentSpec{
					WorkspaceDir: "/workspace",
				},
			},
			&kubeopenv1alpha1.Task{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "running-task",
					Namespace: "default",
					Labels:    map[string]string{"kubeopencode.io/agent": agentName},
				},
				Spec: kubeopenv1alpha1.TaskSpec{
					AgentRef: &kubeopenv1alpha1.AgentReference{Name: agentName},
				},
				Status: kubeopenv1alpha1.TaskExecutionStatus{
					Phase: kubeopenv1alpha1.TaskPhaseRunning,
				},
			},
		).
		WithStatusSubresource(&kubeopenv1alpha1.Task{}).
		Build()

	handler := NewAgentHandler(k8sClient)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.URL = &url.URL{Path: "/"}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", "default")
	rctx.URLParams.Add("name", agentName)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	handler.Suspend(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, w.Code, w.Body.String())
	}
}
