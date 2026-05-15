// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

// RegistryHandler handles registry-related HTTP requests
type RegistryHandler struct {
	defaultClient client.Client
}

// NewRegistryHandler creates a new RegistryHandler
func NewRegistryHandler(c client.Client) *RegistryHandler {
	return &RegistryHandler{defaultClient: c}
}

func (h *RegistryHandler) getClient(ctx context.Context) client.Client {
	return clientFromContext(ctx, h.defaultClient)
}

// ListAll returns all registries across all namespaces with filtering and pagination
func (h *RegistryHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	filterOpts, err := ParseFilterOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid filter parameters", err.Error())
		return
	}

	var registryList kubeopenv1alpha1.RegistryList
	listOpts := BuildListOptions("", filterOpts)

	if err := k8sClient.List(ctx, &registryList, listOpts...); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list registries", err.Error())
		return
	}

	// Filter by name (in-memory)
	var filteredItems []kubeopenv1alpha1.Registry
	for _, reg := range registryList.Items {
		if MatchesNameFilter(reg.Name, filterOpts.Name) {
			filteredItems = append(filteredItems, reg)
		}
	}

	// Sort by CreationTimestamp
	sort.Slice(filteredItems, func(i, j int) bool {
		if filterOpts.SortOrder == "asc" {
			return filteredItems[i].CreationTimestamp.Before(&filteredItems[j].CreationTimestamp)
		}
		return filteredItems[j].CreationTimestamp.Before(&filteredItems[i].CreationTimestamp)
	})

	totalCount := len(filteredItems)

	// Apply pagination bounds
	start := min(filterOpts.Offset, totalCount)
	end := min(start+filterOpts.Limit, totalCount)

	paginatedItems := filteredItems[start:end]
	hasMore := end < totalCount

	response := types.RegistryListResponse{
		Registries: make([]types.RegistryResponse, 0, len(paginatedItems)),
		Total:      totalCount,
		Pagination: &types.Pagination{
			Limit:      filterOpts.Limit,
			Offset:     filterOpts.Offset,
			TotalCount: totalCount,
			HasMore:    hasMore,
		},
	}

	for _, reg := range paginatedItems {
		response.Registries = append(response.Registries, registryToResponse(&reg))
	}

	writeJSON(w, http.StatusOK, response)
}

// List returns all registries in a namespace with filtering and pagination
func (h *RegistryHandler) List(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	filterOpts, err := ParseFilterOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid filter parameters", err.Error())
		return
	}

	var registryList kubeopenv1alpha1.RegistryList
	listOpts := BuildListOptions(namespace, filterOpts)

	if err := k8sClient.List(ctx, &registryList, listOpts...); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list registries", err.Error())
		return
	}

	// Filter by name (in-memory)
	var filteredItems []kubeopenv1alpha1.Registry
	for _, reg := range registryList.Items {
		if MatchesNameFilter(reg.Name, filterOpts.Name) {
			filteredItems = append(filteredItems, reg)
		}
	}

	// Sort by CreationTimestamp
	sort.Slice(filteredItems, func(i, j int) bool {
		if filterOpts.SortOrder == "asc" {
			return filteredItems[i].CreationTimestamp.Before(&filteredItems[j].CreationTimestamp)
		}
		return filteredItems[j].CreationTimestamp.Before(&filteredItems[i].CreationTimestamp)
	})

	totalCount := len(filteredItems)

	// Apply pagination bounds
	start := min(filterOpts.Offset, totalCount)
	end := min(start+filterOpts.Limit, totalCount)

	paginatedItems := filteredItems[start:end]
	hasMore := end < totalCount

	response := types.RegistryListResponse{
		Registries: make([]types.RegistryResponse, 0, len(paginatedItems)),
		Total:      totalCount,
		Pagination: &types.Pagination{
			Limit:      filterOpts.Limit,
			Offset:     filterOpts.Offset,
			TotalCount: totalCount,
			HasMore:    hasMore,
		},
	}

	for _, reg := range paginatedItems {
		response.Registries = append(response.Registries, registryToResponse(&reg))
	}

	writeJSON(w, http.StatusOK, response)
}

// Get returns a specific registry
func (h *RegistryHandler) Get(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var reg kubeopenv1alpha1.Registry
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &reg); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "Registry not found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get registry", err.Error())
		return
	}

	resp := registryToResponse(&reg)
	writeResourceOutput(w, r, http.StatusOK, &reg, resp)
}

// Create creates a new registry
func (h *RegistryHandler) Create(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB limit
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body", err.Error())
		return
	}

	// Try YAML first (YAML is a superset of JSON)
	var reg kubeopenv1alpha1.Registry
	if err := yaml.Unmarshal(body, &reg); err != nil {
		// Fallback: try as a simple JSON create request
		var req types.CreateRegistryRequest
		if jsonErr := json.Unmarshal(body, &req); jsonErr != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
			return
		}
		reg.Name = req.Name
	}

	reg.Namespace = namespace
	if reg.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required", "")
		return
	}

	if err := k8sClient.Create(ctx, &reg); err != nil {
		if apierrors.IsAlreadyExists(err) {
			writeError(w, http.StatusConflict, "Registry already exists", err.Error())
			return
		}
		if apierrors.IsInvalid(err) {
			writeError(w, http.StatusUnprocessableEntity, "Invalid Registry", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to create registry", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, registryToResponse(&reg))
}

// Update replaces the Registry spec from a YAML body
func (h *RegistryHandler) Update(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB limit
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body", err.Error())
		return
	}

	var submitted kubeopenv1alpha1.Registry
	if err := yaml.Unmarshal(body, &submitted); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid YAML", err.Error())
		return
	}

	var existing kubeopenv1alpha1.Registry
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "Registry not found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get registry", err.Error())
		return
	}

	existing.Spec = submitted.Spec
	if err := k8sClient.Update(ctx, &existing); err != nil {
		if apierrors.IsInvalid(err) {
			writeError(w, http.StatusUnprocessableEntity, "Invalid Registry", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to update registry", err.Error())
		return
	}

	writeResourceOutput(w, r, http.StatusOK, &existing, registryToResponse(&existing))
}

// Delete deletes a registry
func (h *RegistryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var reg kubeopenv1alpha1.Registry
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &reg); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "Registry not found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get registry", err.Error())
		return
	}

	if err := k8sClient.Delete(ctx, &reg); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete registry", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// registryToResponse converts a Registry CRD to an API response
func registryToResponse(reg *kubeopenv1alpha1.Registry) types.RegistryResponse {
	resp := types.RegistryResponse{
		Name:      reg.Name,
		Namespace: reg.Namespace,
		Summary: types.RegistrySummaryInfo{
			Images:     reg.Status.Summary.Images,
			Skills:     reg.Status.Summary.Skills,
			Plugins:    reg.Status.Summary.Plugins,
			ReadyCount: reg.Status.Summary.ReadyCount,
			TotalCount: reg.Status.Summary.TotalCount,
		},
		CreatedAt:  reg.CreationTimestamp.Time,
		Labels:     reg.Labels,
		Conditions: conditionsToResponse(reg.Status.Conditions),
	}

	// Map image statuses merged with spec metadata
	imageStatusMap := make(map[string]*kubeopenv1alpha1.ImageStatus, len(reg.Status.Images))
	for i := range reg.Status.Images {
		imageStatusMap[reg.Status.Images[i].Name] = &reg.Status.Images[i]
	}
	for _, img := range reg.Spec.Images {
		info := types.RegistryImageInfo{
			Name:  img.Name,
			Image: img.Image,
			Phase: string(kubeopenv1alpha1.AssetPhaseUnavailable), // default until status populated
		}
		if status, ok := imageStatusMap[img.Name]; ok {
			info.Phase = string(status.Phase)
			info.Digest = status.Digest
			if status.LastChecked != nil {
				t := status.LastChecked.Time
				info.LastChecked = &t
			}
			info.Message = status.Message
		}
		if img.Metadata.Description != "" || img.Metadata.Category != "" || len(img.Metadata.Tags) > 0 {
			info.Metadata = &types.ImageMetadataInfo{
				Description: img.Metadata.Description,
				Category:    img.Metadata.Category,
				Tags:        img.Metadata.Tags,
				Tools:       img.Metadata.Tools,
				BaseImage:   img.Metadata.BaseImage,
				Maintainer:  img.Metadata.Maintainer,
			}
		}
		resp.Images = append(resp.Images, info)
	}

	// Map skill statuses merged with spec metadata
	skillStatusMap := make(map[string]*kubeopenv1alpha1.SkillStatus, len(reg.Status.Skills))
	for i := range reg.Status.Skills {
		skillStatusMap[reg.Status.Skills[i].Name] = &reg.Status.Skills[i]
	}
	for _, skill := range reg.Spec.Skills {
		info := types.RegistrySkillInfo{
			Name:        skill.Name,
			Phase:       string(kubeopenv1alpha1.AssetPhaseUnavailable),
			Description: skill.Metadata.Description,
			Tags:        skill.Metadata.Tags,
		}
		if skill.Git != nil {
			info.Repository = skill.Git.Repository
			info.Ref = skill.Git.Ref
			info.Path = skill.Git.Path
			info.Names = skill.Git.Names
		}
		if status, ok := skillStatusMap[skill.Name]; ok {
			info.Phase = string(status.Phase)
			info.LatestCommit = status.LatestCommit
			if status.LastChecked != nil {
				t := status.LastChecked.Time
				info.LastChecked = &t
			}
			info.Message = status.Message
		}
		resp.Skills = append(resp.Skills, info)
	}

	// Map plugin statuses merged with spec metadata
	pluginStatusMap := make(map[string]*kubeopenv1alpha1.PluginStatus, len(reg.Status.Plugins))
	for i := range reg.Status.Plugins {
		pluginStatusMap[reg.Status.Plugins[i].Name] = &reg.Status.Plugins[i]
	}
	for _, plugin := range reg.Spec.Plugins {
		info := types.RegistryPluginInfo{
			Name:        plugin.Name,
			Package:     plugin.Plugin.Name,
			Target:      string(plugin.Plugin.Target),
			Phase:       string(kubeopenv1alpha1.AssetPhaseUnavailable),
			Description: plugin.Metadata.Description,
			Tags:        plugin.Metadata.Tags,
		}
		if status, ok := pluginStatusMap[plugin.Name]; ok {
			info.Phase = string(status.Phase)
			info.ResolvedVersion = status.ResolvedVersion
			if status.LastChecked != nil {
				t := status.LastChecked.Time
				info.LastChecked = &t
			}
			info.Message = status.Message
		}
		resp.Plugins = append(resp.Plugins, info)
	}

	return resp
}
