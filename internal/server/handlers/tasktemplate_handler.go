// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

// TaskTemplateHandler handles TaskTemplate-related HTTP requests
type TaskTemplateHandler struct {
	defaultClient client.Client
}

// NewTaskTemplateHandler creates a new TaskTemplateHandler
func NewTaskTemplateHandler(c client.Client) *TaskTemplateHandler {
	return &TaskTemplateHandler{defaultClient: c}
}

// getClient returns the client from context or falls back to default
func (h *TaskTemplateHandler) getClient(r *http.Request) client.Client {
	if c, ok := r.Context().Value(clientContextKey{}).(client.Client); ok && c != nil {
		return c
	}
	return h.defaultClient
}

// ListAll returns all TaskTemplates across all namespaces with filtering and pagination
func (h *TaskTemplateHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	k8sClient := h.getClient(r)

	filterOpts, err := ParseFilterOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid filter parameters", err.Error())
		return
	}

	var templateList kubeopenv1alpha1.TaskTemplateList
	listOpts := BuildListOptions("", filterOpts) // empty namespace = all namespaces

	if err := k8sClient.List(ctx, &templateList, listOpts...); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list task templates", err.Error())
		return
	}

	// Filter by name (in-memory)
	var filteredItems []kubeopenv1alpha1.TaskTemplate
	for _, tt := range templateList.Items {
		if MatchesNameFilter(tt.Name, filterOpts.Name) {
			filteredItems = append(filteredItems, tt)
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

	response := types.TaskTemplateListResponse{
		Templates: make([]types.TaskTemplateResponse, 0, len(paginatedItems)),
		Total:     totalCount,
		Pagination: &types.Pagination{
			Limit:      filterOpts.Limit,
			Offset:     filterOpts.Offset,
			TotalCount: totalCount,
			HasMore:    hasMore,
		},
	}

	for _, tt := range paginatedItems {
		response.Templates = append(response.Templates, taskTemplateToResponse(&tt))
	}

	writeJSON(w, http.StatusOK, response)
}

// List returns all TaskTemplates in a namespace with filtering and pagination
func (h *TaskTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	ctx := r.Context()
	k8sClient := h.getClient(r)

	filterOpts, err := ParseFilterOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid filter parameters", err.Error())
		return
	}

	var templateList kubeopenv1alpha1.TaskTemplateList
	listOpts := BuildListOptions(namespace, filterOpts)

	if err := k8sClient.List(ctx, &templateList, listOpts...); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list task templates", err.Error())
		return
	}

	// Filter by name (in-memory)
	var filteredItems []kubeopenv1alpha1.TaskTemplate
	for _, tt := range templateList.Items {
		if MatchesNameFilter(tt.Name, filterOpts.Name) {
			filteredItems = append(filteredItems, tt)
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

	response := types.TaskTemplateListResponse{
		Templates: make([]types.TaskTemplateResponse, 0, len(paginatedItems)),
		Total:     totalCount,
		Pagination: &types.Pagination{
			Limit:      filterOpts.Limit,
			Offset:     filterOpts.Offset,
			TotalCount: totalCount,
			HasMore:    hasMore,
		},
	}

	for _, tt := range paginatedItems {
		response.Templates = append(response.Templates, taskTemplateToResponse(&tt))
	}

	writeJSON(w, http.StatusOK, response)
}

// Get returns a specific TaskTemplate
func (h *TaskTemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(r)

	var tt kubeopenv1alpha1.TaskTemplate
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &tt); err != nil {
		writeError(w, http.StatusNotFound, "TaskTemplate not found", err.Error())
		return
	}

	writeResourceOutput(w, r, http.StatusOK, &tt, taskTemplateToResponse(&tt))
}

// Create creates a new TaskTemplate
func (h *TaskTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	ctx := r.Context()
	k8sClient := h.getClient(r)

	var req types.CreateTaskTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required", "")
		return
	}

	tt := &kubeopenv1alpha1.TaskTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: namespace,
		},
		Spec: kubeopenv1alpha1.TaskTemplateSpec{},
	}

	// Set description if provided
	if req.Description != "" {
		tt.Spec.Description = &req.Description
	}

	// Set agent reference if provided
	if req.AgentRef != nil {
		tt.Spec.AgentRef = &kubeopenv1alpha1.AgentReference{
			Name:      req.AgentRef.Name,
			Namespace: req.AgentRef.Namespace,
		}
	}

	// Convert contexts
	for _, c := range req.Contexts {
		item := kubeopenv1alpha1.ContextItem{
			Name:        c.Name,
			Description: c.Description,
			MountPath:   c.MountPath,
		}
		switch c.Type {
		case "Text":
			item.Type = kubeopenv1alpha1.ContextTypeText
			item.Text = c.Text
		case "ConfigMap":
			item.Type = kubeopenv1alpha1.ContextTypeConfigMap
		case "Git":
			item.Type = kubeopenv1alpha1.ContextTypeGit
		case "Runtime":
			item.Type = kubeopenv1alpha1.ContextTypeRuntime
		case "URL":
			item.Type = kubeopenv1alpha1.ContextTypeURL
		default:
			item.Type = kubeopenv1alpha1.ContextTypeText
		}
		tt.Spec.Contexts = append(tt.Spec.Contexts, item)
	}

	if err := k8sClient.Create(ctx, tt); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create task template", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, taskTemplateToResponse(tt))
}

// Delete deletes a TaskTemplate
func (h *TaskTemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(r)

	var tt kubeopenv1alpha1.TaskTemplate
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &tt); err != nil {
		writeError(w, http.StatusNotFound, "TaskTemplate not found", err.Error())
		return
	}

	if err := k8sClient.Delete(ctx, &tt); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete task template", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// taskTemplateToResponse converts a TaskTemplate CRD to an API response
func taskTemplateToResponse(tt *kubeopenv1alpha1.TaskTemplate) types.TaskTemplateResponse {
	resp := types.TaskTemplateResponse{
		Name:          tt.Name,
		Namespace:     tt.Namespace,
		ContextsCount: len(tt.Spec.Contexts),
		CreatedAt:     tt.CreationTimestamp.Time,
		Labels:        tt.Labels,
	}

	if tt.Spec.Description != nil {
		resp.Description = *tt.Spec.Description
	}

	if tt.Spec.AgentRef != nil {
		resp.AgentRef = &types.AgentReference{
			Name:      tt.Spec.AgentRef.Name,
			Namespace: tt.Spec.AgentRef.Namespace,
		}
	}

	// Add context info
	for _, ctx := range tt.Spec.Contexts {
		ctxItem := types.ContextItem{
			Name:        ctx.Name,
			Description: ctx.Description,
			Type:        string(ctx.Type),
			MountPath:   ctx.MountPath,
		}
		resp.Contexts = append(resp.Contexts, ctxItem)
	}

	return resp
}
