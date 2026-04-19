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

func TestAgentTemplateHandler_ListAll(t *testing.T) {
	tests := []struct {
		name       string
		objects    []runtime.Object
		wantTotal  int
		wantStatus int
	}{
		{
			name: "returns templates across namespaces",
			objects: []runtime.Object{
				&kubeopenv1alpha1.AgentTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "tmpl-1", Namespace: "default"},
					Spec: kubeopenv1alpha1.AgentTemplateSpec{
						WorkspaceDir: "/workspace",
					},
				},
				&kubeopenv1alpha1.AgentTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "tmpl-2", Namespace: "production"},
					Spec: kubeopenv1alpha1.AgentTemplateSpec{
						WorkspaceDir: "/workspace",
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
			handler := NewAgentTemplateHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/v1/agenttemplates", nil)
			r.URL = &url.URL{Path: "/api/v1/agenttemplates"}

			handler.ListAll(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			var resp types.AgentTemplateListResponse
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

func TestAgentTemplateHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		tmplName   string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:      "returns template",
			namespace: "default",
			tmplName:  "my-tmpl",
			objects: []runtime.Object{
				&kubeopenv1alpha1.AgentTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "my-tmpl", Namespace: "default"},
					Spec: kubeopenv1alpha1.AgentTemplateSpec{
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
			tmplName:   "missing",
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
			handler := NewAgentTemplateHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", tt.namespace)
			rctx.URLParams.Add("name", tt.tmplName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Get(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp types.AgentTemplateResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Name != tt.tmplName {
					t.Errorf("expected name %q, got %q", tt.tmplName, resp.Name)
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

func TestAgentTemplateHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "creates template",
			body: types.CreateAgentTemplateRequest{
				Name:               "new-tmpl",
				WorkspaceDir:       "/workspace",
				ServiceAccountName: "sa",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "validates name required",
			body: types.CreateAgentTemplateRequest{
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
			handler := NewAgentTemplateHandler(k8sClient)

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
				var resp types.AgentTemplateResponse
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

func TestAgentTemplateHandler_Delete(t *testing.T) {
	tests := []struct {
		name       string
		tmplName   string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:     "deletes template",
			tmplName: "my-tmpl",
			objects: []runtime.Object{
				&kubeopenv1alpha1.AgentTemplate{
					ObjectMeta: metav1.ObjectMeta{Name: "my-tmpl", Namespace: "default"},
					Spec: kubeopenv1alpha1.AgentTemplateSpec{
						WorkspaceDir: "/workspace",
					},
				},
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "handles not found",
			tmplName:   "missing",
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
			handler := NewAgentTemplateHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodDelete, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", "default")
			rctx.URLParams.Add("name", tt.tmplName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Delete(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			// Verify deletion
			if tt.wantStatus == http.StatusNoContent {
				var tmpl kubeopenv1alpha1.AgentTemplate
				err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: tt.tmplName}, &tmpl)
				if err == nil {
					t.Error("expected template to be deleted")
				}
			}
		})
	}
}
