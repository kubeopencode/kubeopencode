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

func TestTaskHandler_List(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		query      string
		objects    []runtime.Object
		wantTotal  int
		wantStatus int
	}{
		{
			name:      "returns tasks with pagination",
			namespace: "default",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "task-1",
						Namespace:         "default",
						CreationTimestamp: metav1.Now(),
					},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
				},
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "task-2",
						Namespace:         "default",
						CreationTimestamp: metav1.Now(),
					},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
					},
				},
			},
			wantTotal:  2,
			wantStatus: http.StatusOK,
		},
		{
			name:      "filter by name",
			namespace: "default",
			query:     "name=task-1",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "task-1", Namespace: "default"},
					Spec:       kubeopenv1alpha1.TaskSpec{AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"}},
				},
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "task-2", Namespace: "default"},
					Spec:       kubeopenv1alpha1.TaskSpec{AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"}},
				},
			},
			wantTotal:  1,
			wantStatus: http.StatusOK,
		},
		{
			name:      "filter by phase",
			namespace: "default",
			query:     "phase=Running",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "task-running", Namespace: "default"},
					Spec:       kubeopenv1alpha1.TaskSpec{AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"}},
					Status:     kubeopenv1alpha1.TaskExecutionStatus{Phase: kubeopenv1alpha1.TaskPhaseRunning},
				},
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "task-completed", Namespace: "default"},
					Spec:       kubeopenv1alpha1.TaskSpec{AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"}},
					Status:     kubeopenv1alpha1.TaskExecutionStatus{Phase: kubeopenv1alpha1.TaskPhaseCompleted},
				},
			},
			wantTotal:  1,
			wantStatus: http.StatusOK,
		},
		{
			name:       "empty list",
			namespace:  "default",
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
			handler := NewTaskHandler(k8sClient, nil, nil)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/"+tt.namespace+"/tasks", nil)
			r.URL = &url.URL{Path: "/api/v1/namespaces/" + tt.namespace + "/tasks", RawQuery: tt.query}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", tt.namespace)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.List(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			var resp types.TaskListResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Total != tt.wantTotal {
				t.Errorf("expected total %d, got %d", tt.wantTotal, resp.Total)
			}
		})
	}
}

func TestTaskHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		taskName   string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:      "returns task",
			namespace: "default",
			taskName:  "my-task",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "my-task", Namespace: "default"},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef:    &kubeopenv1alpha1.AgentReference{Name: "my-agent"},
						Description: strPtr("do something"),
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "handles not found",
			namespace:  "default",
			taskName:   "missing",
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
			handler := NewTaskHandler(k8sClient, nil, nil)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", tt.namespace)
			rctx.URLParams.Add("name", tt.taskName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Get(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp types.TaskResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Name != tt.taskName {
					t.Errorf("expected name %q, got %q", tt.taskName, resp.Name)
				}
				if resp.Description != "do something" {
					t.Errorf("expected description %q, got %q", "do something", resp.Description)
				}
			}
		})
	}
}

func TestTaskHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
		wantErr    string
	}{
		{
			name: "creates task with agentRef",
			body: types.CreateTaskRequest{
				Name:        "new-task",
				Description: "test task",
				AgentRef:    &types.AgentReference{Name: "my-agent"},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "creates task with templateRef",
			body: types.CreateTaskRequest{
				Name:        "tmpl-task",
				Description: "template task",
				TemplateRef: &types.AgentTemplateReference{Name: "my-tmpl"},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "validates description required",
			body: types.CreateTaskRequest{
				Name:     "no-desc",
				AgentRef: &types.AgentReference{Name: "my-agent"},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "validates mutual exclusivity",
			body: types.CreateTaskRequest{
				Name:        "both-refs",
				Description: "has both",
				AgentRef:    &types.AgentReference{Name: "a"},
				TemplateRef: &types.AgentTemplateReference{Name: "t"},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "validates at least one ref required",
			body: types.CreateTaskRequest{
				Name:        "no-refs",
				Description: "no refs",
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
			handler := NewTaskHandler(k8sClient, nil, nil)

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
				var resp types.TaskResponse
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

func TestTaskHandler_Delete(t *testing.T) {
	tests := []struct {
		name       string
		taskName   string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:     "deletes task",
			taskName: "my-task",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "my-task", Namespace: "default"},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"},
					},
				},
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "handles not found",
			taskName:   "missing",
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
			handler := NewTaskHandler(k8sClient, nil, nil)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodDelete, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", "default")
			rctx.URLParams.Add("name", tt.taskName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Delete(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			// Verify deletion
			if tt.wantStatus == http.StatusNoContent {
				var task kubeopenv1alpha1.Task
				err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: tt.taskName}, &task)
				if err == nil {
					t.Error("expected task to be deleted")
				}
			}
		})
	}
}

func TestTaskHandler_Stop(t *testing.T) {
	tests := []struct {
		name       string
		taskName   string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:     "stops running task",
			taskName: "running-task",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "running-task", Namespace: "default"},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"},
					},
					Status: kubeopenv1alpha1.TaskExecutionStatus{
						Phase: kubeopenv1alpha1.TaskPhaseRunning,
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "rejects non-running task",
			taskName: "completed-task",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Task{
					ObjectMeta: metav1.ObjectMeta{Name: "completed-task", Namespace: "default"},
					Spec: kubeopenv1alpha1.TaskSpec{
						AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"},
					},
					Status: kubeopenv1alpha1.TaskExecutionStatus{
						Phase: kubeopenv1alpha1.TaskPhaseCompleted,
					},
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "handles not found",
			taskName:   "missing",
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
				WithStatusSubresource(&kubeopenv1alpha1.Task{}).
				Build()
			handler := NewTaskHandler(k8sClient, nil, nil)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", "default")
			rctx.URLParams.Add("name", tt.taskName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Stop(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			// Verify annotation was added
			if tt.wantStatus == http.StatusOK {
				var task kubeopenv1alpha1.Task
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: tt.taskName}, &task); err != nil {
					t.Fatalf("failed to get task: %v", err)
				}
				if task.Annotations["kubeopencode.io/stop"] != "true" {
					t.Errorf("expected stop annotation to be set, got annotations: %v", task.Annotations)
				}
			}
		})
	}
}

// strPtr returns a pointer to a string value.
func strPtr(s string) *string {
	return &s
}
