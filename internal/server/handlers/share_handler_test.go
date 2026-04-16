// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"context"
	"net/http"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/controller"
)

// newTestScheme creates a runtime.Scheme with core and kubeopencode types registered.
func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = kubeopenv1alpha1.AddToScheme(s)
	return s
}

// testShareSecret creates a share Secret for testing.
func testShareSecret(name, namespace, agentName, agentNamespace, token string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				controller.LabelShareToken: "true",
			},
			Annotations: map[string]string{
				controller.AnnotationShareAgentName:      agentName,
				controller.AnnotationShareAgentNamespace: agentNamespace,
			},
		},
		Data: map[string][]byte{
			controller.ShareTokenKey: []byte(token),
		},
	}
}

// testAgent creates an Agent for testing with share enabled and ready status.
func testAgent(name, namespace string, share *kubeopenv1alpha1.ShareConfig, ready, suspended bool) *kubeopenv1alpha1.Agent {
	return &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Share:        share,
			WorkspaceDir: "/workspace",
		},
		Status: kubeopenv1alpha1.AgentStatus{
			Ready:     ready,
			Suspended: suspended,
		},
	}
}

func TestResolveShareToken(t *testing.T) {
	const (
		agentName      = "test-agent"
		agentNamespace = "default"
		secretName     = "test-agent-share"
		validToken     = "test-token-abc123"
	)

	pastTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	futureTime := metav1.NewTime(time.Now().Add(1 * time.Hour))

	tests := []struct {
		name    string
		token   string
		objects []runtime.Object
		wantErr string
	}{
		{
			name:  "valid token returns shareContext",
			token: validToken,
			objects: []runtime.Object{
				testShareSecret(secretName, agentNamespace, agentName, agentNamespace, validToken),
				testAgent(agentName, agentNamespace, &kubeopenv1alpha1.ShareConfig{Enabled: true}, true, false),
			},
		},
		{
			name:  "valid token with future expiry",
			token: validToken,
			objects: []runtime.Object{
				testShareSecret(secretName, agentNamespace, agentName, agentNamespace, validToken),
				testAgent(agentName, agentNamespace, &kubeopenv1alpha1.ShareConfig{Enabled: true, ExpiresAt: &futureTime}, true, false),
			},
		},
		{
			name:  "invalid token",
			token: "wrong-token",
			objects: []runtime.Object{
				testShareSecret(secretName, agentNamespace, agentName, agentNamespace, validToken),
				testAgent(agentName, agentNamespace, &kubeopenv1alpha1.ShareConfig{Enabled: true}, true, false),
			},
			wantErr: "invalid share token",
		},
		{
			name:    "no secrets exist",
			token:   validToken,
			objects: []runtime.Object{},
			wantErr: "invalid share token",
		},
		{
			name:  "share disabled",
			token: validToken,
			objects: []runtime.Object{
				testShareSecret(secretName, agentNamespace, agentName, agentNamespace, validToken),
				testAgent(agentName, agentNamespace, &kubeopenv1alpha1.ShareConfig{Enabled: false}, true, false),
			},
			wantErr: "share link is disabled",
		},
		{
			name:  "share nil",
			token: validToken,
			objects: []runtime.Object{
				testShareSecret(secretName, agentNamespace, agentName, agentNamespace, validToken),
				testAgent(agentName, agentNamespace, nil, true, false),
			},
			wantErr: "share link is disabled",
		},
		{
			name:  "share expired",
			token: validToken,
			objects: []runtime.Object{
				testShareSecret(secretName, agentNamespace, agentName, agentNamespace, validToken),
				testAgent(agentName, agentNamespace, &kubeopenv1alpha1.ShareConfig{Enabled: true, ExpiresAt: &pastTime}, true, false),
			},
			wantErr: "has expired",
		},
		{
			name:  "agent not ready",
			token: validToken,
			objects: []runtime.Object{
				testShareSecret(secretName, agentNamespace, agentName, agentNamespace, validToken),
				testAgent(agentName, agentNamespace, &kubeopenv1alpha1.ShareConfig{Enabled: true}, false, false),
			},
			wantErr: "is not ready",
		},
		{
			name:  "agent suspended",
			token: validToken,
			objects: []runtime.Object{
				testShareSecret(secretName, agentNamespace, agentName, agentNamespace, validToken),
				testAgent(agentName, agentNamespace, &kubeopenv1alpha1.ShareConfig{Enabled: true}, true, true),
			},
			wantErr: "is suspended",
		},
		{
			name:  "missing agent annotations",
			token: validToken,
			objects: []runtime.Object{
				// Secret with matching token but no agent annotations
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: agentNamespace,
						Labels: map[string]string{
							controller.LabelShareToken: "true",
						},
						// No annotations
					},
					Data: map[string][]byte{
						controller.ShareTokenKey: []byte(validToken),
					},
				},
			},
			wantErr: "missing agent annotations",
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

			h := &ShareHandler{k8sClient: k8sClient}
			sc, err := h.resolveShareToken(context.Background(), tt.token)

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
			if sc == nil {
				t.Fatal("expected non-nil shareContext")
			}
			if sc.agent == nil {
				t.Fatal("expected non-nil agent in shareContext")
			}
			if sc.agent.Name != agentName {
				t.Errorf("expected agent name %q, got %q", agentName, sc.agent.Name)
			}
		})
	}
}

func TestValidateShareIP(t *testing.T) {
	tests := []struct {
		name       string
		allowedIPs []string
		remoteAddr string
		wantErr    bool
	}{
		{
			name:       "empty allowlist allows all",
			allowedIPs: []string{},
			remoteAddr: "192.168.1.100:12345",
			wantErr:    false,
		},
		{
			name:       "nil allowlist allows all",
			allowedIPs: nil,
			remoteAddr: "10.0.0.1:9999",
			wantErr:    false,
		},
		{
			name:       "IP in CIDR range",
			allowedIPs: []string{"10.0.0.0/8"},
			remoteAddr: "10.1.2.3:12345",
			wantErr:    false,
		},
		{
			name:       "IP not in CIDR range",
			allowedIPs: []string{"10.0.0.0/8"},
			remoteAddr: "192.168.1.1:12345",
			wantErr:    true,
		},
		{
			name:       "exact IP match",
			allowedIPs: []string{"1.2.3.4"},
			remoteAddr: "1.2.3.4:5678",
			wantErr:    false,
		},
		{
			name:       "exact IP no match",
			allowedIPs: []string{"1.2.3.4"},
			remoteAddr: "5.6.7.8:5678",
			wantErr:    true,
		},
		{
			name:       "multiple CIDRs second matches",
			allowedIPs: []string{"10.0.0.0/8", "172.16.0.0/12"},
			remoteAddr: "172.20.1.1:1234",
			wantErr:    false,
		},
		{
			name:       "multiple CIDRs none match",
			allowedIPs: []string{"10.0.0.0/8", "172.16.0.0/12"},
			remoteAddr: "192.168.1.1:1234",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{RemoteAddr: tt.remoteAddr}
			err := validateShareIP(r, tt.allowedIPs)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateShareIP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{
			name:       "standard host:port",
			remoteAddr: "10.0.0.1:12345",
			want:       "10.0.0.1",
		},
		{
			name:       "IPv6 with port",
			remoteAddr: "[::1]:12345",
			want:       "::1",
		},
		{
			name:       "no port fallback",
			remoteAddr: "10.0.0.1",
			want:       "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{RemoteAddr: tt.remoteAddr}
			got := getClientIP(r)
			if got != tt.want {
				t.Errorf("getClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShareValidationInterval(t *testing.T) {
	if shareValidationInterval < 5*time.Second {
		t.Errorf("shareValidationInterval too low (%v), risk of API overload", shareValidationInterval)
	}
	if shareValidationInterval > 60*time.Second {
		t.Errorf("shareValidationInterval too high (%v), disabled share links will linger too long", shareValidationInterval)
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
