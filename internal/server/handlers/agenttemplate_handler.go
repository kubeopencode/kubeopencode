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
	"github.com/kubeopencode/kubeopencode/internal/controller"
	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

// AgentTemplateHandler handles agent template-related HTTP requests
type AgentTemplateHandler struct {
	defaultClient client.Client
}

// NewAgentTemplateHandler creates a new AgentTemplateHandler
func NewAgentTemplateHandler(c client.Client) *AgentTemplateHandler {
	return &AgentTemplateHandler{defaultClient: c}
}

func (h *AgentTemplateHandler) getClient(ctx context.Context) client.Client {
	return clientFromContext(ctx, h.defaultClient)
}

// ListAll returns all agent templates across all namespaces with filtering and pagination
func (h *AgentTemplateHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	filterOpts, err := ParseFilterOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid filter parameters", err.Error())
		return
	}

	var tmplList kubeopenv1alpha1.AgentTemplateList
	listOpts := BuildListOptions("", filterOpts)

	if err := k8sClient.List(ctx, &tmplList, listOpts...); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list agent templates", err.Error())
		return
	}

	// Filter by name (in-memory)
	var filteredItems []kubeopenv1alpha1.AgentTemplate
	for _, tmpl := range tmplList.Items {
		if MatchesNameFilter(tmpl.Name, filterOpts.Name) {
			filteredItems = append(filteredItems, tmpl)
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

	// Count referencing agents for each template
	agentCounts := h.countReferencingAgents(ctx, k8sClient, paginatedItems)

	response := types.AgentTemplateListResponse{
		Templates: make([]types.AgentTemplateResponse, 0, len(paginatedItems)),
		Total:     totalCount,
		Pagination: &types.Pagination{
			Limit:      filterOpts.Limit,
			Offset:     filterOpts.Offset,
			TotalCount: totalCount,
			HasMore:    hasMore,
		},
	}

	for i, tmpl := range paginatedItems {
		resp := templateToResponse(&tmpl)
		resp.AgentCount = agentCounts[i]
		response.Templates = append(response.Templates, resp)
	}

	writeJSON(w, http.StatusOK, response)
}

// List returns all agent templates in a namespace with filtering and pagination
func (h *AgentTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	filterOpts, err := ParseFilterOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid filter parameters", err.Error())
		return
	}

	var tmplList kubeopenv1alpha1.AgentTemplateList
	listOpts := BuildListOptions(namespace, filterOpts)

	if err := k8sClient.List(ctx, &tmplList, listOpts...); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list agent templates", err.Error())
		return
	}

	// Filter by name (in-memory)
	var filteredItems []kubeopenv1alpha1.AgentTemplate
	for _, tmpl := range tmplList.Items {
		if MatchesNameFilter(tmpl.Name, filterOpts.Name) {
			filteredItems = append(filteredItems, tmpl)
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

	// Count referencing agents for each template
	agentCounts := h.countReferencingAgents(ctx, k8sClient, paginatedItems)

	response := types.AgentTemplateListResponse{
		Templates: make([]types.AgentTemplateResponse, 0, len(paginatedItems)),
		Total:     totalCount,
		Pagination: &types.Pagination{
			Limit:      filterOpts.Limit,
			Offset:     filterOpts.Offset,
			TotalCount: totalCount,
			HasMore:    hasMore,
		},
	}

	for i, tmpl := range paginatedItems {
		resp := templateToResponse(&tmpl)
		resp.AgentCount = agentCounts[i]
		response.Templates = append(response.Templates, resp)
	}

	writeJSON(w, http.StatusOK, response)
}

// Get returns a specific agent template with referencing agents
func (h *AgentTemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var tmpl kubeopenv1alpha1.AgentTemplate
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &tmpl); err != nil {
		writeError(w, http.StatusNotFound, "AgentTemplate not found", err.Error())
		return
	}

	resp := templateToResponse(&tmpl)

	// Count referencing agents
	var agentList kubeopenv1alpha1.AgentList
	if err := k8sClient.List(ctx, &agentList,
		client.InNamespace(namespace),
		client.MatchingLabels{controller.LabelAgentTemplate: name},
	); err == nil {
		resp.AgentCount = len(agentList.Items)
	}

	writeResourceOutput(w, r, http.StatusOK, &tmpl, resp)
}

// Create creates a new agent template
func (h *AgentTemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var req types.CreateAgentTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required", "")
		return
	}

	tmpl := &kubeopenv1alpha1.AgentTemplate{}
	tmpl.Name = req.Name
	tmpl.Namespace = namespace
	tmpl.Spec.WorkspaceDir = req.WorkspaceDir
	tmpl.Spec.ServiceAccountName = req.ServiceAccountName
	tmpl.Spec.AgentImage = req.AgentImage
	tmpl.Spec.ExecutorImage = req.ExecutorImage

	if err := k8sClient.Create(ctx, tmpl); err != nil {
		if apierrors.IsAlreadyExists(err) {
			writeError(w, http.StatusConflict, "AgentTemplate already exists", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to create agent template", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, templateToResponse(tmpl))
}

// Delete deletes an agent template
func (h *AgentTemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var tmpl kubeopenv1alpha1.AgentTemplate
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &tmpl); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "AgentTemplate not found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get agent template", err.Error())
		return
	}

	if err := k8sClient.Delete(ctx, &tmpl); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete agent template", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Update replaces the AgentTemplate spec from a YAML body.
func (h *AgentTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	var submitted kubeopenv1alpha1.AgentTemplate
	if err := yaml.Unmarshal(body, &submitted); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid YAML", err.Error())
		return
	}

	var existing kubeopenv1alpha1.AgentTemplate
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "AgentTemplate not found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get agent template", err.Error())
		return
	}

	existing.Spec = submitted.Spec
	if err := k8sClient.Update(ctx, &existing); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update agent template", err.Error())
		return
	}

	writeResourceOutput(w, r, http.StatusOK, &existing, templateToResponse(&existing))
}

// templateToResponse converts an AgentTemplate CRD to an API response
func templateToResponse(tmpl *kubeopenv1alpha1.AgentTemplate) types.AgentTemplateResponse {
	return types.AgentTemplateResponse{
		Name:               tmpl.Name,
		Namespace:          tmpl.Namespace,
		AgentImage:         tmpl.Spec.AgentImage,
		ExecutorImage:      tmpl.Spec.ExecutorImage,
		WorkspaceDir:       tmpl.Spec.WorkspaceDir,
		ServiceAccountName: tmpl.Spec.ServiceAccountName,
		ContextsCount:      len(tmpl.Spec.Contexts),
		CredentialsCount:   len(tmpl.Spec.Credentials),
		SkillsCount:        len(tmpl.Spec.Skills),
		CreatedAt:          tmpl.CreationTimestamp.Time,
		Labels:             tmpl.Labels,
		Conditions:         conditionsToResponse(tmpl.Status.Conditions),
		Credentials:        credentialsToInfo(tmpl.Spec.Credentials),
		Contexts:           contextsToItems(tmpl.Spec.Contexts),
		Skills:             skillsToInfo(tmpl.Spec.Skills),
	}
}

// countReferencingAgents counts agents referencing each template using a single
// list call per unique namespace (instead of one call per template).
func (h *AgentTemplateHandler) countReferencingAgents(ctx context.Context, k8sClient client.Client, templates []kubeopenv1alpha1.AgentTemplate) []int {
	counts := make([]int, len(templates))
	if len(templates) == 0 {
		return counts
	}

	// Collect unique namespaces from templates
	namespaces := make(map[string]bool)
	for _, tmpl := range templates {
		namespaces[tmpl.Namespace] = true
	}

	// Single agent list per namespace, build count map
	countMap := make(map[string]map[string]int) // namespace -> templateName -> count
	for ns := range namespaces {
		var agentList kubeopenv1alpha1.AgentList
		if err := k8sClient.List(ctx, &agentList, client.InNamespace(ns)); err != nil {
			continue
		}
		nsMap := make(map[string]int)
		for _, agent := range agentList.Items {
			if tmplName, ok := agent.Labels[controller.LabelAgentTemplate]; ok {
				nsMap[tmplName]++
			}
		}
		countMap[ns] = nsMap
	}

	// Map counts back to template order
	for i, tmpl := range templates {
		if nsMap, ok := countMap[tmpl.Namespace]; ok {
			counts[i] = nsMap[tmpl.Name]
		}
	}
	return counts
}
