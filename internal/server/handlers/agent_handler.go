// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

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
	resp := types.AgentResponse{
		Name:             agent.Name,
		Namespace:        agent.Namespace,
		Profile:          agent.Spec.Profile,
		ExecutorImage:    agent.Spec.ExecutorImage,
		AgentImage:       agent.Spec.AgentImage,
		WorkspaceDir:       agent.Spec.WorkspaceDir,
		ServiceAccountName: agent.Spec.ServiceAccountName,
		ContextsCount:      len(agent.Spec.Contexts),
		CredentialsCount: len(agent.Spec.Credentials),
		CreatedAt:        agent.CreationTimestamp.Time,
		Labels:           agent.Labels,
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

	if agent.Spec.Standby != nil {
		resp.Standby = &types.StandbyInfo{
			IdleTimeout: agent.Spec.Standby.IdleTimeout.Duration.String(),
		}
	}

	resp.Conditions = conditionsToResponse(agent.Status.Conditions)

	// Always populate server status (Agent is always a running instance)
	serverStatus := &types.ServerStatusInfo{
		DeploymentName: agent.Status.DeploymentName,
		ServiceName:    agent.Status.ServiceName,
		URL:            agent.Status.URL,
		Ready:          agent.Status.Ready,
		Port:           controller.GetServerPort(agent),
		Suspended:      agent.Status.Suspended,
	}
	if agent.Status.IdleSince != nil {
		t := agent.Status.IdleSince.Time
		serverStatus.IdleSince = &t
	}
	resp.ServerStatus = serverStatus

	resp.Credentials = credentialsToInfo(agent.Spec.Credentials)
	resp.Contexts = contextsToItems(agent.Spec.Contexts)

	return resp
}

// Create creates a new agent
func (h *AgentHandler) Create(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var req types.CreateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Name is required", "")
		return
	}
	// workspaceDir and serviceAccountName are required only when no template is referenced,
	// since they can be inherited from the template.
	if req.TemplateRef == nil {
		if req.WorkspaceDir == "" {
			writeError(w, http.StatusBadRequest, "WorkspaceDir is required when no template is specified", "")
			return
		}
		if req.ServiceAccountName == "" {
			writeError(w, http.StatusBadRequest, "ServiceAccountName is required when no template is specified", "")
			return
		}
	}

	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: namespace,
		},
		Spec: kubeopenv1alpha1.AgentSpec{
			Profile:            req.Profile,
			WorkspaceDir:       req.WorkspaceDir,
			ServiceAccountName: req.ServiceAccountName,
			AgentImage:         req.AgentImage,
			ExecutorImage:      req.ExecutorImage,
			MaxConcurrentTasks: req.MaxConcurrentTasks,
		},
	}

	if req.TemplateRef != nil {
		agent.Spec.TemplateRef = &kubeopenv1alpha1.AgentTemplateReference{
			Name: req.TemplateRef.Name,
		}
	}

	if req.Standby != nil {
		d, err := time.ParseDuration(req.Standby.IdleTimeout)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid standby.idleTimeout format", fmt.Sprintf("expected Go duration (e.g. 30m, 1h): %v", err))
			return
		}
		agent.Spec.Standby = &kubeopenv1alpha1.StandbyConfig{
			IdleTimeout: metav1.Duration{Duration: d},
		}
	}

	if req.Persistence != nil {
		agent.Spec.Persistence = &kubeopenv1alpha1.PersistenceConfig{}
		if req.Persistence.Sessions != nil {
			agent.Spec.Persistence.Sessions = &kubeopenv1alpha1.VolumePersistence{
				Size: req.Persistence.Sessions.Size,
			}
			if req.Persistence.Sessions.StorageClassName != "" {
				sc := req.Persistence.Sessions.StorageClassName
				agent.Spec.Persistence.Sessions.StorageClassName = &sc
			}
		}
		if req.Persistence.Workspace != nil {
			agent.Spec.Persistence.Workspace = &kubeopenv1alpha1.VolumePersistence{
				Size: req.Persistence.Workspace.Size,
			}
			if req.Persistence.Workspace.StorageClassName != "" {
				sc := req.Persistence.Workspace.StorageClassName
				agent.Spec.Persistence.Workspace.StorageClassName = &sc
			}
		}
	}

	if req.Port != nil {
		agent.Spec.Port = *req.Port
	}

	if req.Proxy != nil {
		agent.Spec.Proxy = &kubeopenv1alpha1.ProxyConfig{
			HttpProxy:  req.Proxy.HttpProxy,
			HttpsProxy: req.Proxy.HttpsProxy,
			NoProxy:    req.Proxy.NoProxy,
		}
	}

	if err := k8sClient.Create(ctx, agent); err != nil {
		if apierrors.IsAlreadyExists(err) {
			writeError(w, http.StatusConflict, "Agent already exists", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to create agent", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, agentToResponse(agent))
}

// Delete deletes an agent
func (h *AgentHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
		writeError(w, http.StatusInternalServerError, "Failed to get agent", err.Error())
		return
	}

	if err := k8sClient.Delete(ctx, &agent); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete agent", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Update replaces the Agent spec from a YAML body.
func (h *AgentHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	var submitted kubeopenv1alpha1.Agent
	if err := yaml.Unmarshal(body, &submitted); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid YAML", err.Error())
		return
	}

	var existing kubeopenv1alpha1.Agent
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "Agent not found", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get agent", err.Error())
		return
	}

	existing.Spec = submitted.Spec
	if err := k8sClient.Update(ctx, &existing); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update agent", err.Error())
		return
	}

	writeResourceOutput(w, r, http.StatusOK, &existing, agentToResponse(&existing))
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

	// Reject suspend if there are active tasks (Running, Queued, Pending)
	if suspend {
		var taskList kubeopenv1alpha1.TaskList
		if err := k8sClient.List(ctx, &taskList,
			client.InNamespace(namespace),
			client.MatchingLabels{controller.AgentLabelKey: name},
		); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to list tasks", err.Error())
			return
		}
		for i := range taskList.Items {
			phase := taskList.Items[i].Status.Phase
			if phase == kubeopenv1alpha1.TaskPhaseRunning ||
				phase == kubeopenv1alpha1.TaskPhasePending ||
				phase == "" {
				writeError(w, http.StatusConflict, "Cannot suspend agent", "Agent has active tasks")
				return
			}
		}
	}

	agent.Spec.Suspend = suspend
	if err := k8sClient.Update(ctx, &agent); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update Agent", err.Error())
		return
	}

	resp := agentToResponse(&agent)
	// Optimistically reflect the spec change in the response since the
	// controller has not reconciled the status yet.
	if resp.ServerStatus != nil {
		resp.ServerStatus.Suspended = suspend
		if suspend {
			resp.ServerStatus.Ready = false
		}
	}
	writeJSON(w, http.StatusOK, resp)
}
