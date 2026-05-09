// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNormalizeProxyPath(t *testing.T) {
	tests := []struct {
		name     string
		wildcard string
		want     string
	}{
		{
			name:     "empty wildcard returns root",
			wildcard: "",
			want:     "/",
		},
		{
			name:     "path without leading slash gets one added",
			wildcard: "session/123",
			want:     "/session/123",
		},
		{
			name:     "path with leading slash is unchanged",
			wildcard: "/session/123",
			want:     "/session/123",
		},
		{
			name:     "root path",
			wildcard: "/",
			want:     "/",
		},
		{
			name:     "nested path",
			wildcard: "api/v1/sessions/abc/messages",
			want:     "/api/v1/sessions/abc/messages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newProxyTestRequest(tt.wildcard)
			got := normalizeProxyPath(r)
			if got != tt.want {
				t.Errorf("normalizeProxyPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

// newProxyTestRequest creates a request with a chi route context containing the wildcard parameter.
func newProxyTestRequest(wildcard string) *http.Request {
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("*", wildcard)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestResolveAgentServerURL(t *testing.T) {
	const clusterDomain = "cluster.local"

	tests := []struct {
		name    string
		agent   *testAgentSetup
		wantErr string
		wantURL string
	}{
		{
			name: "valid agent with URL",
			agent: &testAgentSetup{
				name:      "my-agent",
				namespace: "default",
				url:       "http://my-agent-server.default.svc.cluster.local:4096",
				suspended: false,
			},
			wantURL: "http://my-agent-server.default.svc.cluster.local:4096",
		},
		{
			name: "suspended agent",
			agent: &testAgentSetup{
				name:      "my-agent",
				namespace: "default",
				url:       "http://my-agent-server.default.svc.cluster.local:4096",
				suspended: true,
			},
			wantErr: "is suspended",
		},
		{
			name: "agent with no URL",
			agent: &testAgentSetup{
				name:      "my-agent",
				namespace: "default",
				url:       "",
				suspended: false,
			},
			wantErr: "is not ready",
		},
		{
			name: "agent with external URL (SSRF protection)",
			agent: &testAgentSetup{
				name:      "my-agent",
				namespace: "default",
				url:       "http://evil.com:4096",
				suspended: false,
			},
			wantErr: "invalid server URL",
		},
		{
			name:    "agent not found",
			agent:   nil,
			wantErr: "agent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme()
			builder := fake.NewClientBuilder().WithScheme(scheme)

			agentName := "missing"
			namespace := "default"

			if tt.agent != nil {
				agentName = tt.agent.name
				namespace = tt.agent.namespace
				agent := testAgent(
					tt.agent.name, tt.agent.namespace,
					nil, !tt.agent.suspended, tt.agent.suspended,
				)
				agent.Status.URL = tt.agent.url
				builder = builder.WithRuntimeObjects(agent).
					WithStatusSubresource(agent)
			}

			k8sClient := builder.Build()

			url, err := resolveAgentServerURL(context.Background(), k8sClient, namespace, agentName, clusterDomain)

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
			if url != tt.wantURL {
				t.Errorf("url = %q, want %q", url, tt.wantURL)
			}
		})
	}
}

type testAgentSetup struct {
	name      string
	namespace string
	url       string
	suspended bool
}
