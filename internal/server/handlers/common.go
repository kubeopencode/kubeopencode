// Copyright Contributors to the KubeOpenCode project

// Package handlers implements HTTP handlers for the KubeOpenCode server.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes an error response
func writeError(w http.ResponseWriter, status int, err string, message string) {
	writeJSON(w, status, types.ErrorResponse{
		Error:   err,
		Message: message,
		Code:    status,
	})
}

// clientFromContext returns the impersonated client from context or falls back to the default.
func clientFromContext(ctx context.Context, defaultClient client.Client) client.Client {
	if c, ok := ctx.Value(ClientContextKey{}).(client.Client); ok && c != nil {
		return c
	}
	return defaultClient
}

// clientsetFromContext returns the impersonated clientset from context or falls back to the default.
func clientsetFromContext(ctx context.Context, defaultClientset kubernetes.Interface) kubernetes.Interface {
	if cs, ok := ctx.Value(ClientsetContextKey{}).(kubernetes.Interface); ok && cs != nil {
		return cs
	}
	return defaultClientset
}

// resolveAgentServerURL looks up the Agent CR and returns its in-cluster server URL.
func resolveAgentServerURL(ctx context.Context, k8sClient client.Client, namespace, agentName string) (string, error) {
	var agent kubeopenv1alpha1.Agent
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: agentName}, &agent); err != nil {
		return "", fmt.Errorf("agent not found: %w", err)
	}

	if agent.Spec.ServerConfig == nil {
		return "", fmt.Errorf("agent %q is not in Server mode (no serverConfig)", agentName)
	}

	if agent.Status.ServerStatus == nil || agent.Status.ServerStatus.URL == "" {
		return "", fmt.Errorf("agent %q server is not ready (no server URL in status)", agentName)
	}

	serverURL := agent.Status.ServerStatus.URL
	if err := validateServerURL(serverURL); err != nil {
		return "", fmt.Errorf("agent %q has invalid server URL: %w", agentName, err)
	}

	return serverURL, nil
}

// validateServerURL ensures the URL points to an in-cluster Kubernetes service.
// This prevents SSRF if the Agent status is tampered with (e.g., via direct
// status patch or a compromised controller).
func validateServerURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" {
		return fmt.Errorf("URL scheme must be http, got %q", u.Scheme)
	}
	host := u.Hostname()
	if !strings.HasSuffix(host, ".svc.cluster.local") {
		return fmt.Errorf("URL host must be a cluster-local service, got %q", host)
	}
	if u.User != nil {
		return fmt.Errorf("URL must not contain userinfo")
	}
	return nil
}

// normalizeProxyPath extracts and normalizes the wildcard path suffix from a chi route.
func normalizeProxyPath(r *http.Request) string {
	path := chi.URLParam(r, "*")
	if path == "" {
		return "/"
	}
	if path[0] != '/' {
		return "/" + path
	}
	return path
}

// writeResourceOutput writes a Kubernetes resource as JSON or YAML depending on the output query parameter
func writeResourceOutput(w http.ResponseWriter, r *http.Request, statusCode int, obj client.Object, jsonResponse interface{}) {
	output := r.URL.Query().Get("output")
	if output == "yaml" {
		data, err := json.Marshal(obj)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to marshal resource", err.Error())
			return
		}
		yamlData, err := yaml.JSONToYAML(data)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to convert to YAML", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.WriteHeader(statusCode)
		_, _ = w.Write(yamlData)
		return
	}
	writeJSON(w, statusCode, jsonResponse)
}

// credentialsToInfo converts CRD credentials to API response format (without exposing secrets).
func credentialsToInfo(creds []kubeopenv1alpha1.Credential) []types.CredentialInfo {
	if len(creds) == 0 {
		return nil
	}
	result := make([]types.CredentialInfo, 0, len(creds))
	for _, cred := range creds {
		info := types.CredentialInfo{
			Name:      cred.Name,
			SecretRef: cred.SecretRef.Name,
		}
		if cred.MountPath != nil {
			info.MountPath = *cred.MountPath
		}
		if cred.Env != nil {
			info.Env = *cred.Env
		}
		result = append(result, info)
	}
	return result
}

// contextsToItems converts CRD context items to API response format.
func contextsToItems(ctxs []kubeopenv1alpha1.ContextItem) []types.ContextItem {
	if len(ctxs) == 0 {
		return nil
	}
	result := make([]types.ContextItem, 0, len(ctxs))
	for _, ctx := range ctxs {
		result = append(result, types.ContextItem{
			Name:        ctx.Name,
			Description: ctx.Description,
			Type:        string(ctx.Type),
			MountPath:   ctx.MountPath,
		})
	}
	return result
}

// conditionsToResponse converts CRD conditions to API response format.
func conditionsToResponse(conds []metav1.Condition) []types.Condition {
	if len(conds) == 0 {
		return nil
	}
	result := make([]types.Condition, 0, len(conds))
	for _, c := range conds {
		result = append(result, types.Condition{
			Type:    c.Type,
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		})
	}
	return result
}
