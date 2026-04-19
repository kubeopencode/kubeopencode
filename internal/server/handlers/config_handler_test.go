// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

func TestConfigHandler_Get(t *testing.T) {
	tests := []struct {
		name       string
		objects    []runtime.Object
		wantStatus int
		wantName   string
	}{
		{
			name: "returns config response",
			objects: []runtime.Object{
				&kubeopenv1alpha1.KubeOpenCodeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: kubeopenv1alpha1.KubeOpenCodeConfigSpec{
						Cleanup: &kubeopenv1alpha1.CleanupConfig{
							TTLSecondsAfterFinished: int32Ptr(3600),
						},
					},
				},
			},
			wantStatus: http.StatusOK,
			wantName:   "cluster",
		},
		{
			name:       "handles not found",
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
			handler := NewConfigHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
			r.URL = &url.URL{Path: "/api/v1/config"}

			handler.Get(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp types.ConfigResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Name != tt.wantName {
					t.Errorf("expected name %q, got %q", tt.wantName, resp.Name)
				}
				if resp.Cleanup == nil {
					t.Fatal("expected non-nil Cleanup")
				}
				if resp.Cleanup.TTLSecondsAfterFinished == nil || *resp.Cleanup.TTLSecondsAfterFinished != 3600 {
					t.Errorf("expected TTLSecondsAfterFinished=3600, got %v", resp.Cleanup.TTLSecondsAfterFinished)
				}
			}
		})
	}
}

func TestConfigHandler_Update(t *testing.T) {
	tests := []struct {
		name       string
		objects    []runtime.Object
		body       string
		wantStatus int
	}{
		{
			name: "updates config spec",
			objects: []runtime.Object{
				&kubeopenv1alpha1.KubeOpenCodeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: kubeopenv1alpha1.KubeOpenCodeConfigSpec{},
				},
			},
			body:       `{"spec":{"cleanup":{"ttlSecondsAfterFinished":7200}}}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "handles not found",
			objects:    []runtime.Object{},
			body:       `{"spec":{"cleanup":{"ttlSecondsAfterFinished":7200}}}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name: "handles invalid YAML",
			objects: []runtime.Object{
				&kubeopenv1alpha1.KubeOpenCodeConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: kubeopenv1alpha1.KubeOpenCodeConfigSpec{},
				},
			},
			body:       `{not valid yaml or json`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tt.objects...).
				Build()
			handler := NewConfigHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPut, "/api/v1/config", bytes.NewBufferString(tt.body))
			r.URL = &url.URL{Path: "/api/v1/config"}

			handler.Update(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp types.ConfigResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Name != "cluster" {
					t.Errorf("expected name %q, got %q", "cluster", resp.Name)
				}
			}
		})
	}
}

// int32Ptr returns a pointer to an int32 value.
func int32Ptr(v int32) *int32 {
	return &v
}
