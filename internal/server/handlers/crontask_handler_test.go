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

func TestCronTaskHandler_ListAll(t *testing.T) {
	tests := []struct {
		name       string
		objects    []runtime.Object
		wantTotal  int
		wantStatus int
	}{
		{
			name: "returns crontasks across namespaces",
			objects: []runtime.Object{
				&kubeopenv1alpha1.CronTask{
					ObjectMeta: metav1.ObjectMeta{Name: "ct-1", Namespace: "default"},
					Spec: kubeopenv1alpha1.CronTaskSpec{
						Schedule: "*/5 * * * *",
						TaskTemplate: kubeopenv1alpha1.TaskTemplateSpec{
							Spec: kubeopenv1alpha1.TaskSpec{
								AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"},
							},
						},
					},
				},
				&kubeopenv1alpha1.CronTask{
					ObjectMeta: metav1.ObjectMeta{Name: "ct-2", Namespace: "production"},
					Spec: kubeopenv1alpha1.CronTaskSpec{
						Schedule: "0 * * * *",
						TaskTemplate: kubeopenv1alpha1.TaskTemplateSpec{
							Spec: kubeopenv1alpha1.TaskSpec{
								AgentRef: &kubeopenv1alpha1.AgentReference{Name: "b"},
							},
						},
					},
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
			handler := NewCronTaskHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/v1/crontasks", nil)
			r.URL = &url.URL{Path: "/api/v1/crontasks"}

			handler.ListAll(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			var resp types.CronTaskListResponse
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

func TestCronTaskHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		ctName     string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:      "returns crontask",
			namespace: "default",
			ctName:    "my-ct",
			objects: []runtime.Object{
				&kubeopenv1alpha1.CronTask{
					ObjectMeta: metav1.ObjectMeta{Name: "my-ct", Namespace: "default"},
					Spec: kubeopenv1alpha1.CronTaskSpec{
						Schedule: "*/10 * * * *",
						TaskTemplate: kubeopenv1alpha1.TaskTemplateSpec{
							Spec: kubeopenv1alpha1.TaskSpec{
								AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"},
							},
						},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "handles not found",
			namespace:  "default",
			ctName:     "missing",
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
			handler := NewCronTaskHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", tt.namespace)
			rctx.URLParams.Add("name", tt.ctName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Get(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp types.CronTaskResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Name != tt.ctName {
					t.Errorf("expected name %q, got %q", tt.ctName, resp.Name)
				}
				if resp.Schedule != "*/10 * * * *" {
					t.Errorf("expected schedule %q, got %q", "*/10 * * * *", resp.Schedule)
				}
			}
		})
	}
}

func TestCronTaskHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "creates crontask with agentRef",
			body: types.CreateCronTaskRequest{
				Name:        "new-ct",
				Schedule:    "*/5 * * * *",
				Description: "run every 5 minutes",
				AgentRef:    &types.AgentReference{Name: "my-agent"},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "validates schedule required",
			body: types.CreateCronTaskRequest{
				Name:     "no-schedule",
				AgentRef: &types.AgentReference{Name: "a"},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "validates agentRef or templateRef required",
			body: types.CreateCronTaskRequest{
				Name:     "no-ref",
				Schedule: "*/5 * * * *",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "validates mutual exclusivity of refs",
			body: types.CreateCronTaskRequest{
				Name:        "both-refs",
				Schedule:    "*/5 * * * *",
				AgentRef:    &types.AgentReference{Name: "a"},
				TemplateRef: &types.AgentTemplateReference{Name: "t"},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "validates invalid cron expression",
			body: types.CreateCronTaskRequest{
				Name:     "bad-cron",
				Schedule: "not a cron",
				AgentRef: &types.AgentReference{Name: "a"},
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
			handler := NewCronTaskHandler(k8sClient)

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
				var resp types.CronTaskResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Namespace != "default" {
					t.Errorf("expected namespace %q, got %q", "default", resp.Namespace)
				}
				if resp.Schedule != "*/5 * * * *" {
					t.Errorf("expected schedule %q, got %q", "*/5 * * * *", resp.Schedule)
				}
			}
		})
	}
}

func TestCronTaskHandler_Suspend(t *testing.T) {
	tests := []struct {
		name       string
		ctName     string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:   "suspends crontask",
			ctName: "my-ct",
			objects: []runtime.Object{
				&kubeopenv1alpha1.CronTask{
					ObjectMeta: metav1.ObjectMeta{Name: "my-ct", Namespace: "default"},
					Spec: kubeopenv1alpha1.CronTaskSpec{
						Schedule: "*/5 * * * *",
						TaskTemplate: kubeopenv1alpha1.TaskTemplateSpec{
							Spec: kubeopenv1alpha1.TaskSpec{
								AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"},
							},
						},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "handles not found",
			ctName:     "missing",
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
			handler := NewCronTaskHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", "default")
			rctx.URLParams.Add("name", tt.ctName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Suspend(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				// Verify the crontask spec was updated
				var ct kubeopenv1alpha1.CronTask
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: tt.ctName}, &ct); err != nil {
					t.Fatalf("failed to get crontask: %v", err)
				}
				if ct.Spec.Suspend == nil || !*ct.Spec.Suspend {
					t.Error("expected crontask.Spec.Suspend to be true")
				}

				var resp types.CronTaskResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if !resp.Suspend {
					t.Error("expected response Suspend to be true")
				}
			}
		})
	}
}

func TestCronTaskHandler_Resume(t *testing.T) {
	suspended := true
	scheme := newTestScheme()
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(
			&kubeopenv1alpha1.CronTask{
				ObjectMeta: metav1.ObjectMeta{Name: "my-ct", Namespace: "default"},
				Spec: kubeopenv1alpha1.CronTaskSpec{
					Schedule: "*/5 * * * *",
					Suspend:  &suspended,
					TaskTemplate: kubeopenv1alpha1.TaskTemplateSpec{
						Spec: kubeopenv1alpha1.TaskSpec{
							AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"},
						},
					},
				},
			},
		).
		Build()
	handler := NewCronTaskHandler(k8sClient)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.URL = &url.URL{Path: "/"}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("namespace", "default")
	rctx.URLParams.Add("name", "my-ct")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	handler.Resume(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	// Verify the crontask spec was updated
	var ct kubeopenv1alpha1.CronTask
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "my-ct"}, &ct); err != nil {
		t.Fatalf("failed to get crontask: %v", err)
	}
	if ct.Spec.Suspend == nil || *ct.Spec.Suspend {
		t.Error("expected crontask.Spec.Suspend to be false")
	}

	var resp types.CronTaskResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Suspend {
		t.Error("expected response Suspend to be false")
	}
}

func TestCronTaskHandler_Trigger(t *testing.T) {
	tests := []struct {
		name       string
		ctName     string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:   "triggers crontask",
			ctName: "my-ct",
			objects: []runtime.Object{
				&kubeopenv1alpha1.CronTask{
					ObjectMeta: metav1.ObjectMeta{Name: "my-ct", Namespace: "default"},
					Spec: kubeopenv1alpha1.CronTaskSpec{
						Schedule: "*/5 * * * *",
						TaskTemplate: kubeopenv1alpha1.TaskTemplateSpec{
							Spec: kubeopenv1alpha1.TaskSpec{
								AgentRef: &kubeopenv1alpha1.AgentReference{Name: "a"},
							},
						},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "handles not found",
			ctName:     "missing",
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
			handler := NewCronTaskHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", "default")
			rctx.URLParams.Add("name", tt.ctName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Trigger(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				// Verify trigger annotation was set
				var ct kubeopenv1alpha1.CronTask
				if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: tt.ctName}, &ct); err != nil {
					t.Fatalf("failed to get crontask: %v", err)
				}
				if ct.Annotations[kubeopenv1alpha1.CronTaskTriggerAnnotation] != "true" {
					t.Errorf("expected trigger annotation to be set, got annotations: %v", ct.Annotations)
				}
			}
		})
	}
}
