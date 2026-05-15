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

func TestRegistryHandler_ListAll(t *testing.T) {
	tests := []struct {
		name       string
		objects    []runtime.Object
		wantTotal  int
		wantStatus int
	}{
		{
			name: "returns registries across namespaces",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Registry{
					ObjectMeta: metav1.ObjectMeta{Name: "reg-1", Namespace: "default"},
					Spec:       kubeopenv1alpha1.RegistrySpec{},
				},
				&kubeopenv1alpha1.Registry{
					ObjectMeta: metav1.ObjectMeta{Name: "reg-2", Namespace: "production"},
					Spec:       kubeopenv1alpha1.RegistrySpec{},
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
			handler := NewRegistryHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/v1/registries", nil)
			r.URL = &url.URL{Path: "/api/v1/registries"}

			handler.ListAll(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			var resp types.RegistryListResponse
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

func TestRegistryHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		regName    string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:      "returns registry",
			namespace: "default",
			regName:   "my-reg",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Registry{
					ObjectMeta: metav1.ObjectMeta{Name: "my-reg", Namespace: "default"},
					Spec: kubeopenv1alpha1.RegistrySpec{
						Images: []kubeopenv1alpha1.RegistryImage{
							{Name: "go-dev", Image: "harbor.company.com/team/go-dev:1.23"},
						},
						Skills: []kubeopenv1alpha1.RegistrySkill{
							{
								Name: "computer-use",
								Git:  &kubeopenv1alpha1.GitSkillSource{Repository: "https://github.com/anthropics/skills.git"},
							},
						},
						Plugins: []kubeopenv1alpha1.RegistryPlugin{
							{
								Name:   "safety-net",
								Plugin: kubeopenv1alpha1.PluginSpec{Name: "cc-safety-net"},
							},
						},
					},
					Status: kubeopenv1alpha1.RegistryStatus{
						Summary: kubeopenv1alpha1.StatusSummary{
							Images:     1,
							Skills:     1,
							Plugins:    1,
							ReadyCount: 3,
							TotalCount: 3,
						},
						Images: []kubeopenv1alpha1.ImageStatus{
							{Name: "go-dev", Phase: kubeopenv1alpha1.AssetPhaseReady, Image: "harbor.company.com/team/go-dev:1.23"},
						},
						Skills: []kubeopenv1alpha1.SkillStatus{
							{Name: "computer-use", Phase: kubeopenv1alpha1.AssetPhaseReady},
						},
						Plugins: []kubeopenv1alpha1.PluginStatus{
							{Name: "safety-net", Phase: kubeopenv1alpha1.AssetPhaseReady, ResolvedVersion: "0.8.2"},
						},
					},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "handles not found",
			namespace:  "default",
			regName:    "missing",
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
			handler := NewRegistryHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", tt.namespace)
			rctx.URLParams.Add("name", tt.regName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Get(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp types.RegistryResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Name != tt.regName {
					t.Errorf("expected name %q, got %q", tt.regName, resp.Name)
				}
				if resp.Namespace != tt.namespace {
					t.Errorf("expected namespace %q, got %q", tt.namespace, resp.Namespace)
				}
				if resp.Summary.Images != 1 {
					t.Errorf("expected summary.images %d, got %d", 1, resp.Summary.Images)
				}
				if resp.Summary.Skills != 1 {
					t.Errorf("expected summary.skills %d, got %d", 1, resp.Summary.Skills)
				}
				if resp.Summary.Plugins != 1 {
					t.Errorf("expected summary.plugins %d, got %d", 1, resp.Summary.Plugins)
				}
				if resp.Summary.ReadyCount != 3 {
					t.Errorf("expected summary.readyCount %d, got %d", 3, resp.Summary.ReadyCount)
				}
				if resp.Summary.TotalCount != 3 {
					t.Errorf("expected summary.totalCount %d, got %d", 3, resp.Summary.TotalCount)
				}
				if len(resp.Images) != 1 {
					t.Errorf("expected 1 image, got %d", len(resp.Images))
				}
				if len(resp.Skills) != 1 {
					t.Errorf("expected 1 skill, got %d", len(resp.Skills))
				}
				if len(resp.Plugins) != 1 {
					t.Errorf("expected 1 plugin, got %d", len(resp.Plugins))
				}
				if len(resp.Images) > 0 && resp.Images[0].Phase != "Ready" {
					t.Errorf("expected image phase %q, got %q", "Ready", resp.Images[0].Phase)
				}
			}
		})
	}
}

func TestRegistryHandler_Create(t *testing.T) {
	tests := []struct {
		name       string
		body       interface{}
		wantStatus int
	}{
		{
			name: "creates registry",
			body: map[string]interface{}{
				"metadata": map[string]string{"name": "test-reg"},
				"spec":     map[string]interface{}{},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "validates name required",
			body:       map[string]string{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				Build()
			handler := NewRegistryHandler(k8sClient)

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
				var resp types.RegistryResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Name != "test-reg" {
					t.Errorf("expected name %q, got %q", "test-reg", resp.Name)
				}
				if resp.Namespace != "default" {
					t.Errorf("expected namespace %q, got %q", "default", resp.Namespace)
				}
			}
		})
	}
}

func TestRegistryHandler_Delete(t *testing.T) {
	tests := []struct {
		name       string
		regName    string
		objects    []runtime.Object
		wantStatus int
	}{
		{
			name:    "deletes registry",
			regName: "my-reg",
			objects: []runtime.Object{
				&kubeopenv1alpha1.Registry{
					ObjectMeta: metav1.ObjectMeta{Name: "my-reg", Namespace: "default"},
					Spec:       kubeopenv1alpha1.RegistrySpec{},
				},
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "handles not found",
			regName:    "missing",
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
			handler := NewRegistryHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodDelete, "/", nil)
			r.URL = &url.URL{Path: "/"}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("namespace", "default")
			rctx.URLParams.Add("name", tt.regName)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

			handler.Delete(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			// Verify deletion
			if tt.wantStatus == http.StatusNoContent {
				var reg kubeopenv1alpha1.Registry
				err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: tt.regName}, &reg)
				if err == nil {
					t.Error("expected registry to be deleted")
				}
			}
		})
	}
}
