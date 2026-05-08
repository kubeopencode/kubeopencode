// Copyright Contributors to the KubeOpenCode project

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	authv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubetesting "k8s.io/client-go/testing"

	"k8s.io/client-go/kubernetes/fake"
)

// successHandler is a simple handler that writes 200 OK for use in middleware tests.
var successHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// fakeClientsetWithTokenReview creates a fake clientset with a TokenReview reactor
// that returns the specified authentication result.
func fakeClientsetWithTokenReview(authenticated bool, username, uid string, groups []string) *fake.Clientset {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("create", "tokenreviews", func(action kubetesting.Action) (bool, runtime.Object, error) {
		review := action.(kubetesting.CreateAction).GetObject().(*authv1.TokenReview)
		review.Status = authv1.TokenReviewStatus{
			Authenticated: authenticated,
			User: authv1.UserInfo{
				Username: username,
				UID:      uid,
				Groups:   groups,
			},
		}
		return true, review, nil
	})
	return cs
}

// fakeClientsetWithError creates a fake clientset where TokenReview returns an error.
func fakeClientsetWithError(err error) *fake.Clientset {
	cs := fake.NewSimpleClientset()
	cs.PrependReactor("create", "tokenreviews", func(action kubetesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
	return cs
}

func TestAuth_Disabled(t *testing.T) {
	cs := fake.NewSimpleClientset()
	middleware := Auth(cs, AuthConfig{Enabled: false})

	handler := middleware(successHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAuth_NoHeader_AllowAnonymous(t *testing.T) {
	cs := fake.NewSimpleClientset()
	middleware := Auth(cs, AuthConfig{Enabled: true, AllowAnonymous: true})

	handler := middleware(successHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestAuth_NoHeader_RejectAnonymous(t *testing.T) {
	cs := fake.NewSimpleClientset()
	middleware := Auth(cs, AuthConfig{Enabled: true, AllowAnonymous: false})

	handler := middleware(successHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAuth_InvalidHeaderFormat(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "Basic scheme", header: "Basic dXNlcjpwYXNz"},
		{name: "no space", header: "Bearertoken123"},
		{name: "empty bearer", header: "Bearer"},
		{name: "random string", header: "just-a-token"},
	}

	cs := fake.NewSimpleClientset()
	middleware := Auth(cs, AuthConfig{Enabled: true})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := middleware(successHandler)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
			}
		})
	}
}

func TestAuth_TokenReviewError(t *testing.T) {
	cs := fakeClientsetWithError(context.DeadlineExceeded)
	middleware := Auth(cs, AuthConfig{Enabled: true})

	handler := middleware(successHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestAuth_TokenNotAuthenticated(t *testing.T) {
	cs := fakeClientsetWithTokenReview(false, "", "", nil)
	middleware := Auth(cs, AuthConfig{Enabled: true})

	handler := middleware(successHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAuth_TokenAuthenticated(t *testing.T) {
	cs := fakeClientsetWithTokenReview(true, "admin", "uid-123", []string{"system:masters", "dev-team"})
	middleware := Auth(cs, AuthConfig{Enabled: true})

	var capturedUserInfo *UserInfo
	captureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserInfo = GetUserInfo(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(captureHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if capturedUserInfo == nil {
		t.Fatal("expected UserInfo in context, got nil")
	}
	if capturedUserInfo.Username != "admin" {
		t.Errorf("expected username %q, got %q", "admin", capturedUserInfo.Username)
	}
	if capturedUserInfo.UID != "uid-123" {
		t.Errorf("expected UID %q, got %q", "uid-123", capturedUserInfo.UID)
	}
	if len(capturedUserInfo.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(capturedUserInfo.Groups))
	}
	if capturedUserInfo.Groups[0] != "system:masters" {
		t.Errorf("expected group[0] %q, got %q", "system:masters", capturedUserInfo.Groups[0])
	}
	if capturedUserInfo.Groups[1] != "dev-team" {
		t.Errorf("expected group[1] %q, got %q", "dev-team", capturedUserInfo.Groups[1])
	}
}

func TestAuth_BearerCaseInsensitive(t *testing.T) {
	cs := fakeClientsetWithTokenReview(true, "user1", "uid-1", nil)
	middleware := Auth(cs, AuthConfig{Enabled: true})

	handler := middleware(successHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "bearer my-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d (bearer is case-insensitive), got %d", http.StatusOK, rec.Code)
	}
}

func TestGetUserInfo_WithValue(t *testing.T) {
	expected := &UserInfo{Username: "test-user", UID: "test-uid", Groups: []string{"g1"}}
	ctx := context.WithValue(context.Background(), UserInfoKey, expected)

	got := GetUserInfo(ctx)
	if got == nil {
		t.Fatal("expected non-nil UserInfo")
	}
	if got.Username != expected.Username {
		t.Errorf("expected username %q, got %q", expected.Username, got.Username)
	}
}

func TestGetUserInfo_WithoutValue(t *testing.T) {
	got := GetUserInfo(context.Background())
	if got != nil {
		t.Errorf("expected nil UserInfo, got %+v", got)
	}
}

func TestGetUserInfo_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserInfoKey, "not-a-userinfo")

	got := GetUserInfo(ctx)
	if got != nil {
		t.Errorf("expected nil UserInfo for wrong type, got %+v", got)
	}
}
