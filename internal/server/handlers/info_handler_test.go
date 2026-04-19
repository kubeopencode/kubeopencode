// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

func TestInfoHandler_GetInfo(t *testing.T) {
	scheme := newTestScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	handler := NewInfoHandler(k8sClient)

	// Override Version for deterministic test
	oldVersion := Version
	Version = "v0.0.1-test"
	defer func() { Version = oldVersion }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	r.URL = &url.URL{Path: "/api/v1/info"}

	handler.GetInfo(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp types.ServerInfo
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Version != "v0.0.1-test" {
		t.Errorf("expected version %q, got %q", "v0.0.1-test", resp.Version)
	}
}

func TestInfoHandler_ListNamespaces(t *testing.T) {
	tests := []struct {
		name       string
		objects    []runtime.Object
		wantCount  int
		wantNames  []string
		wantStatus int
	}{
		{
			name: "returns namespaces sorted alphabetically",
			objects: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "delta"}},
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "alpha"}},
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "charlie"}},
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "bravo"}},
			},
			wantCount:  4,
			wantNames:  []string{"alpha", "bravo", "charlie", "delta"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "handles empty namespace list",
			objects:    []runtime.Object{},
			wantCount:  0,
			wantNames:  []string{},
			wantStatus: http.StatusOK,
		},
		{
			name: "single namespace",
			objects: []runtime.Object{
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			},
			wantCount:  1,
			wantNames:  []string{"default"},
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
			handler := NewInfoHandler(k8sClient)

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
			r.URL = &url.URL{Path: "/api/v1/namespaces"}

			handler.ListNamespaces(w, r)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			var resp types.NamespaceList
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if len(resp.Namespaces) != tt.wantCount {
				t.Fatalf("expected %d namespaces, got %d", tt.wantCount, len(resp.Namespaces))
			}

			for i, want := range tt.wantNames {
				if resp.Namespaces[i] != want {
					t.Errorf("namespace[%d] = %q, want %q", i, resp.Namespaces[i], want)
				}
			}
		})
	}
}
