// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"context"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/controller"
	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

// ClientContextKey is the context key for the impersonated Kubernetes client.
// It is used by the impersonation middleware in the server package to store
// the per-request client, and by all handlers to retrieve it.
type ClientContextKey struct{}

// ClientsetContextKey is the context key for the impersonated Kubernetes clientset.
// Used by handlers that need clientset operations (e.g., pod logs) with user RBAC.
type ClientsetContextKey struct{}

// AgentHandler handles agent-related HTTP requests
type AgentHandler struct {
	defaultClient client.Client
}

// NewAgentHandler creates a new AgentHandler
func NewAgentHandler(c client.Client) *AgentHandler {
	return &AgentHandler{defaultClient: c}
}

func (h *AgentHandler) getClient(ctx context.Context) client.Client {
	return clientFromContext(ctx, h.defaultClient)
}

// ListAll returns all agents across all namespaces with filtering and pagination
func (h *AgentHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	filterOpts, err := ParseFilterOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid filter parameters", err.Error())
		return
	}

	var agentList kubeopenv1alpha1.AgentList
	listOpts := BuildListOptions("", filterOpts) // empty namespace = all namespaces

	if err := k8sClient.List(ctx, &agentList, listOpts...); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list agents", err.Error())
		return
	}

	// Filter by name (in-memory)
	var filteredItems []kubeopenv1alpha1.Agent
	for _, agent := range agentList.Items {
		if MatchesNameFilter(agent.Name, filterOpts.Name) {
			filteredItems = append(filteredItems, agent)
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

	response := types.AgentListResponse{
		Agents: make([]types.AgentResponse, 0, len(paginatedItems)),
		Total:  totalCount,
		Pagination: &types.Pagination{
			Limit:      filterOpts.Limit,
			Offset:     filterOpts.Offset,
			TotalCount: totalCount,
			HasMore:    hasMore,
		},
	}

	for _, agent := range paginatedItems {
		response.Agents = append(response.Agents, agentToResponse(&agent))
	}

	writeJSON(w, http.StatusOK, response)
}

// List returns all agents in a namespace with filtering and pagination
func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	filterOpts, err := ParseFilterOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid filter parameters", err.Error())
		return
	}

	var agentList kubeopenv1alpha1.AgentList
	listOpts := BuildListOptions(namespace, filterOpts)

	if err := k8sClient.List(ctx, &agentList, listOpts...); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list agents", err.Error())
		return
	}

	// Filter by name (in-memory)
	var filteredItems []kubeopenv1alpha1.Agent
	for _, agent := range agentList.Items {
		if MatchesNameFilter(agent.Name, filterOpts.Name) {
			filteredItems = append(filteredItems, agent)
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

	response := types.AgentListResponse{
		Agents: make([]types.AgentResponse, 0, len(paginatedItems)),
		Total:  totalCount,
		Pagination: &types.Pagination{
			Limit:      filterOpts.Limit,
			Offset:     filterOpts.Offset,
			TotalCount: totalCount,
			HasMore:    hasMore,
		},
	}

	for _, agent := range paginatedItems {
		response.Agents = append(response.Agents, agentToResponse(&agent))
	}

	writeJSON(w, http.StatusOK, response)
}

// Get returns a specific agent
func (h *AgentHandler) Get(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var agent kubeopenv1alpha1.Agent
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &agent); err != nil {
		writeError(w, http.StatusNotFound, "Agent not found", err.Error())
		return
	}

	writeResourceOutput(w, r, http.StatusOK, &agent, agentToResponse(&agent))
}

// agentToResponse converts an Agent CRD to an API response
func agentToResponse(agent *kubeopenv1alpha1.Agent) types.AgentResponse {
	mode := "Pod"
	if agent.Spec.ServerConfig != nil {
		mode = "Server"
	}

	resp := types.AgentResponse{
		Name:             agent.Name,
		Namespace:        agent.Namespace,
		Profile:          agent.Spec.Profile,
		ExecutorImage:    agent.Spec.ExecutorImage,
		AgentImage:       agent.Spec.AgentImage,
		WorkspaceDir:     agent.Spec.WorkspaceDir,
		ContextsCount:    len(agent.Spec.Contexts),
		CredentialsCount: len(agent.Spec.Credentials),
		CreatedAt:        agent.CreationTimestamp.Time,
		Labels:           agent.Labels,
		Mode:             mode,
	}

	if agent.Spec.TemplateRef != nil {
		resp.TemplateRef = &types.AgentReference{Name: agent.Spec.TemplateRef.Name}
	}

	if agent.Spec.MaxConcurrentTasks != nil {
		resp.MaxConcurrentTasks = agent.Spec.MaxConcurrentTasks
	}

	if agent.Spec.Quota != nil {
		resp.Quota = &types.QuotaInfo{
			MaxTaskStarts: agent.Spec.Quota.MaxTaskStarts,
			WindowSeconds: agent.Spec.Quota.WindowSeconds,
		}
	}

	resp.Conditions = conditionsToResponse(agent.Status.Conditions)

	// Add server status if in Server mode
	if agent.Status.ServerStatus != nil {
		resp.ServerStatus = &types.ServerStatusInfo{
			DeploymentName: agent.Status.ServerStatus.DeploymentName,
			ServiceName:    agent.Status.ServerStatus.ServiceName,
			URL:            agent.Status.ServerStatus.URL,
			Ready:          agent.Status.ServerStatus.Ready,
			Port:           controller.GetServerPort(agent),
			Suspended:      agent.Status.ServerStatus.Suspended,
		}
	}

	resp.Credentials = credentialsToInfo(agent.Spec.Credentials)
	resp.Contexts = contextsToItems(agent.Spec.Contexts)

	return resp
}

// Suspend scales the server deployment to 0 replicas.
func (h *AgentHandler) Suspend(w http.ResponseWriter, r *http.Request) {
	h.setSuspendState(w, r, true)
}

// Resume scales the server deployment back to 1 replica.
func (h *AgentHandler) Resume(w http.ResponseWriter, r *http.Request) {
	h.setSuspendState(w, r, false)
}

func (h *AgentHandler) setSuspendState(w http.ResponseWriter, r *http.Request, suspend bool) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var agent kubeopenv1alpha1.Agent
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &agent); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "Agent not found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get Agent", err.Error())
		return
	}

	if agent.Spec.ServerConfig == nil {
		writeError(w, http.StatusBadRequest, "Invalid operation", "Suspend is only supported for Server-mode agents")
		return
	}

	agent.Spec.ServerConfig.Suspend = suspend
	if err := k8sClient.Update(ctx, &agent); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update Agent", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, agentToResponse(&agent))
}
