// Copyright Contributors to the KubeTask project

package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // SHA1 required for legacy GitHub webhook compatibility
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"hash"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubetaskv1alpha1 "github.com/kubetask/kubetask/api/v1alpha1"
)

// validateAuth validates the webhook request against the configured authentication.
func (s *Server) validateAuth(r *http.Request, body []byte, auth *kubetaskv1alpha1.WebhookAuth, namespace string) error {
	if auth.HMAC != nil {
		return s.validateHMAC(r, body, auth.HMAC, namespace)
	}
	if auth.BearerToken != nil {
		return s.validateBearerToken(r, auth.BearerToken, namespace)
	}
	if auth.Header != nil {
		return s.validateHeader(r, auth.Header, namespace)
	}
	return nil
}

// validateHMAC validates HMAC signature.
func (s *Server) validateHMAC(r *http.Request, body []byte, auth *kubetaskv1alpha1.HMACAuth, namespace string) error {
	// Get signature from header
	signature := r.Header.Get(auth.SignatureHeader)
	if signature == "" {
		return fmt.Errorf("missing signature header: %s", auth.SignatureHeader)
	}

	// Get secret from Kubernetes
	secret, err := s.getSecretValue(r.Context(), namespace, auth.SecretRef.Name, auth.SecretRef.Key)
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// Determine algorithm
	algorithm := auth.Algorithm
	if algorithm == "" {
		algorithm = "sha256"
	}

	// Create HMAC
	var h hash.Hash
	switch algorithm {
	case "sha1":
		h = hmac.New(sha1.New, secret)
	case "sha256":
		h = hmac.New(sha256.New, secret)
	case "sha512":
		h = hmac.New(sha512.New, secret)
	default:
		return fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	h.Write(body)
	expectedSignature := hex.EncodeToString(h.Sum(nil))

	// GitHub sends signature in format: sha256=<signature>
	// Strip the algorithm prefix if present
	actualSignature := signature
	if idx := strings.Index(signature, "="); idx != -1 {
		actualSignature = signature[idx+1:]
	}

	// Constant-time comparison
	if subtle.ConstantTimeCompare([]byte(expectedSignature), []byte(actualSignature)) != 1 {
		return fmt.Errorf("signature mismatch")
	}

	return nil
}

// validateBearerToken validates Bearer token from Authorization header.
func (s *Server) validateBearerToken(r *http.Request, auth *kubetaskv1alpha1.BearerTokenAuth, namespace string) error {
	// Get Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("missing Authorization header")
	}

	// Check Bearer prefix
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return fmt.Errorf("invalid Authorization header format, expected Bearer token")
	}

	token := authHeader[len(bearerPrefix):]

	// Get expected token from Secret
	expectedToken, err := s.getSecretValue(r.Context(), namespace, auth.SecretRef.Name, auth.SecretRef.Key)
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// Constant-time comparison
	if subtle.ConstantTimeCompare([]byte(token), expectedToken) != 1 {
		return fmt.Errorf("token mismatch")
	}

	return nil
}

// validateHeader validates a custom header value.
func (s *Server) validateHeader(r *http.Request, auth *kubetaskv1alpha1.HeaderAuth, namespace string) error {
	// Get header value
	headerValue := r.Header.Get(auth.Name)
	if headerValue == "" {
		return fmt.Errorf("missing header: %s", auth.Name)
	}

	// Get expected value from Secret
	expectedValue, err := s.getSecretValue(r.Context(), namespace, auth.SecretRef.Name, auth.SecretRef.Key)
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// Constant-time comparison
	if subtle.ConstantTimeCompare([]byte(headerValue), expectedValue) != 1 {
		return fmt.Errorf("header value mismatch")
	}

	return nil
}

// getSecretValue retrieves a specific key from a Kubernetes Secret.
func (s *Server) getSecretValue(ctx context.Context, namespace, name, key string) ([]byte, error) {
	secret := &corev1.Secret{}
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, secret); err != nil {
		return nil, err
	}

	value, ok := secret.Data[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found in secret %s/%s", key, namespace, name)
	}

	return value, nil
}
