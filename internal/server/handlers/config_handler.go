// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"context"
	"io"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

// ConfigHandler handles config-related HTTP requests
type ConfigHandler struct {
	defaultClient client.Client
}

// NewConfigHandler creates a new ConfigHandler
func NewConfigHandler(c client.Client) *ConfigHandler {
	return &ConfigHandler{defaultClient: c}
}

func (h *ConfigHandler) getClient(ctx context.Context) client.Client {
	return clientFromContext(ctx, h.defaultClient)
}

// Get returns the KubeOpenCodeConfig singleton
func (h *ConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var config kubeopenv1alpha1.KubeOpenCodeConfig
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, &config); err != nil {
		writeError(w, http.StatusNotFound, "Config not found", err.Error())
		return
	}

	resp := configToResponse(&config)
	writeResourceOutput(w, r, http.StatusOK, &config, resp)
}

// Update replaces the KubeOpenCodeConfig spec from a YAML body.
func (h *ConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB limit
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body", err.Error())
		return
	}

	var submitted kubeopenv1alpha1.KubeOpenCodeConfig
	if err := yaml.Unmarshal(body, &submitted); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid YAML", err.Error())
		return
	}

	var existing kubeopenv1alpha1.KubeOpenCodeConfig
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: "cluster"}, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "Config not found", "Create a KubeOpenCodeConfig named 'cluster' first")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get config", err.Error())
		return
	}

	existing.Spec = submitted.Spec
	if err := k8sClient.Update(ctx, &existing); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update config", err.Error())
		return
	}

	resp := configToResponse(&existing)
	writeResourceOutput(w, r, http.StatusOK, &existing, resp)
}

func configToResponse(config *kubeopenv1alpha1.KubeOpenCodeConfig) *types.ConfigResponse {
	resp := &types.ConfigResponse{
		Name:      config.Name,
		CreatedAt: config.CreationTimestamp.Time,
		Labels:    config.Labels,
	}

	if config.Spec.SystemImage != nil {
		resp.SystemImage = &types.SystemImageConfig{
			Image:           config.Spec.SystemImage.Image,
			ImagePullPolicy: string(config.Spec.SystemImage.ImagePullPolicy),
		}
	}

	if config.Spec.Cleanup != nil {
		resp.Cleanup = &types.CleanupConfig{
			TTLSecondsAfterFinished: config.Spec.Cleanup.TTLSecondsAfterFinished,
			MaxRetainedTasks:        config.Spec.Cleanup.MaxRetainedTasks,
		}
	}

	if config.Spec.Proxy != nil {
		resp.Proxy = &types.ProxyConfigInfo{
			HttpProxy:  config.Spec.Proxy.HttpProxy,
			HttpsProxy: config.Spec.Proxy.HttpsProxy,
			NoProxy:    config.Spec.Proxy.NoProxy,
		}
	}

	return resp
}
