// Copyright Contributors to the KubeOpenCode project

//go:build !integration

package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseNpmSpec(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{"pkg", "pkg", ""},
		{"pkg@1.0.0", "pkg", "1.0.0"},
		{"@scope/pkg", "@scope/pkg", ""},
		{"@scope/pkg@1", "@scope/pkg", "1"},
		{"@scope/pkg@^1.0.0", "@scope/pkg", "^1.0.0"},
		{"", "", ""},
		{"pkg@", "pkg", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotName, gotVersion := parseNpmSpec(tt.input)
			if gotName != tt.wantName {
				t.Errorf("parseNpmSpec(%q) name = %q, want %q", tt.input, gotName, tt.wantName)
			}
			if gotVersion != tt.wantVersion {
				t.Errorf("parseNpmSpec(%q) version = %q, want %q", tt.input, gotVersion, tt.wantVersion)
			}
		})
	}
}

func TestIsSemverRange(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"1.0.0", false},
		{"0.8.2", false},
		{"^1.0.0", true},
		{"~1.0.0", true},
		{">=1.0.0", true},
		{"<2.0.0", true},
		{"*", true},
		{"1.x", true},
		{"1.2.x", true},
		{"1.0.0 - 2.0.0", true},
		{">1 <3", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isSemverRange(tt.input)
			if got != tt.want {
				t.Errorf("isSemverRange(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCheckNpmPackage(t *testing.T) {
	// Helper to build a JSON response body for the mock npm registry.
	buildRegistryResponse := func(latest string, versions []string) []byte {
		distTags := map[string]string{}
		if latest != "" {
			distTags["latest"] = latest
		}
		versionsMap := map[string]any{}
		for _, v := range versions {
			versionsMap[v] = struct{}{}
		}
		body := struct {
			DistTags map[string]string `json:"dist-tags"`
			Versions map[string]any    `json:"versions"`
		}{
			DistTags: distTags,
			Versions: versionsMap,
		}
		b, _ := json.Marshal(body)
		return b
	}

	t.Run("200 with valid package returns latest version", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(buildRegistryResponse("1.2.3", []string{"1.2.3"}))
		}))
		defer ts.Close()

		rec := &RegistryReconciler{
			HTTPClient: ts.Client(),
		}

		// Override the URL by using the test server's URL
		// We need to make the reconciler hit the test server, so we wrap the client
		// to redirect requests to the test server.
		rec.HTTPClient = &http.Client{
			Transport: &testTransport{testServerURL: ts.URL},
		}

		version, err := rec.checkNpmPackage(context.Background(), "pkg", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if version != "1.2.3" {
			t.Errorf("expected version %q, got %q", "1.2.3", version)
		}
	})

	t.Run("200 with exact version match", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(buildRegistryResponse("1.2.3", []string{"0.8.2", "1.2.3"}))
		}))
		defer ts.Close()

		rec := &RegistryReconciler{
			HTTPClient: &http.Client{
				Transport: &testTransport{testServerURL: ts.URL},
			},
		}

		version, err := rec.checkNpmPackage(context.Background(), "pkg@0.8.2", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if version != "0.8.2" {
			t.Errorf("expected version %q, got %q", "0.8.2", version)
		}
	})

	t.Run("200 with exact version not found", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(buildRegistryResponse("1.2.3", []string{"1.2.3"}))
		}))
		defer ts.Close()

		rec := &RegistryReconciler{
			HTTPClient: &http.Client{
				Transport: &testTransport{testServerURL: ts.URL},
			},
		}

		_, err := rec.checkNpmPackage(context.Background(), "pkg@9.9.9", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected error containing %q, got %q", "not found", err.Error())
		}
	})

	t.Run("200 with semver range falls back to latest", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(buildRegistryResponse("2.0.0", []string{"1.0.0", "2.0.0"}))
		}))
		defer ts.Close()

		rec := &RegistryReconciler{
			HTTPClient: &http.Client{
				Transport: &testTransport{testServerURL: ts.URL},
			},
		}

		version, err := rec.checkNpmPackage(context.Background(), "pkg@^1.0.0", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if version != "2.0.0" {
			t.Errorf("expected version %q, got %q", "2.0.0", version)
		}
	})

	t.Run("404 package not found", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		rec := &RegistryReconciler{
			HTTPClient: &http.Client{
				Transport: &testTransport{testServerURL: ts.URL},
			},
		}

		_, err := rec.checkNpmPackage(context.Background(), "pkg", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("expected error containing %q, got %q", "not found", err.Error())
		}
	})

	t.Run("500 server error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		rec := &RegistryReconciler{
			HTTPClient: &http.Client{
				Transport: &testTransport{testServerURL: ts.URL},
			},
		}

		_, err := rec.checkNpmPackage(context.Background(), "pkg", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "status 500") {
			t.Errorf("expected error containing %q, got %q", "status 500", err.Error())
		}
	})

	t.Run("custom registry URL is used", func(t *testing.T) {
		var receivedPath string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(buildRegistryResponse("1.0.0", []string{"1.0.0"}))
		}))
		defer ts.Close()

		rec := &RegistryReconciler{
			HTTPClient: ts.Client(),
		}

		// Pass the test server URL as the custom registry URL directly
		version, err := rec.checkNpmPackage(context.Background(), "@scope/pkg@1.0.0", ts.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if version != "1.0.0" {
			t.Errorf("expected version %q, got %q", "1.0.0", version)
		}
		// Verify the scoped package name is in the path.
		// Go's HTTP server decodes %2f back to / when parsing the URL path.
		if receivedPath != "/@scope/pkg" {
			t.Errorf("expected path %q, got %q", "/@scope/pkg", receivedPath)
		}
	})
}

// testTransport redirects all HTTP requests to the test server URL.
// This allows checkNpmPackage (which constructs its own URL to registry.npmjs.org)
// to be intercepted and redirected to the httptest.Server.
type testTransport struct {
	testServerURL string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the scheme+host with the test server URL, keep the path
	newURL := t.testServerURL + req.URL.Path
	newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	newReq.Header = req.Header
	return http.DefaultTransport.RoundTrip(newReq)
}
