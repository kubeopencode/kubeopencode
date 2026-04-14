// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"testing"
)

func TestGenerateShareToken(t *testing.T) {
	token1, err := generateShareToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token1 == "" {
		t.Fatal("expected non-empty token")
	}
	// base64url of 32 bytes = 43 characters (no padding with RawURLEncoding)
	if len(token1) != 43 {
		t.Errorf("expected token length 43, got %d", len(token1))
	}

	// Ensure uniqueness
	token2, err := generateShareToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token1 == token2 {
		t.Error("expected unique tokens, got duplicates")
	}
}

func TestShareSecretName(t *testing.T) {
	tests := []struct {
		agentName string
		expected  string
	}{
		{"my-agent", "my-agent-share"},
		{"dev", "dev-share"},
	}
	for _, tt := range tests {
		got := ShareSecretName(tt.agentName)
		if got != tt.expected {
			t.Errorf("ShareSecretName(%q) = %q, want %q", tt.agentName, got, tt.expected)
		}
	}
}
