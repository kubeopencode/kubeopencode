// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/robfig/cron/v3"

	"github.com/go-chi/chi/v5"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

// CronTaskHandler handles crontask-related HTTP requests
type CronTaskHandler struct {
	defaultClient client.Client
}

// NewCronTaskHandler creates a new CronTaskHandler
func NewCronTaskHandler(c client.Client) *CronTaskHandler {
	return &CronTaskHandler{defaultClient: c}
}

func (h *CronTaskHandler) getClient(r *http.Request) client.Client {
	return clientFromContext(r.Context(), h.defaultClient)
}

// ListAll returns all CronTasks across all namespaces
func (h *CronTaskHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	h.listCronTasks(w, r, "")
}

// List returns CronTasks in a namespace
func (h *CronTaskHandler) List(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	h.listCronTasks(w, r, namespace)
}

func (h *CronTaskHandler) listCronTasks(w http.ResponseWriter, r *http.Request, namespace string) {
	ctx := r.Context()
	k8sClient := h.getClient(r)

	filterOpts, err := ParseFilterOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid filter parameters", err.Error())
		return
	}

	var cronTaskList kubeopenv1alpha1.CronTaskList
	listOpts := BuildListOptions(namespace, filterOpts)

	if err := k8sClient.List(ctx, &cronTaskList, listOpts...); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list CronTasks", err.Error())
		return
	}

	// Filter by name
	var filteredItems []kubeopenv1alpha1.CronTask
	for _, ct := range cronTaskList.Items {
		if !MatchesNameFilter(ct.Name, filterOpts.Name) {
			continue
		}
		filteredItems = append(filteredItems, ct)
	}

	// Sort by CreationTimestamp
	sort.Slice(filteredItems, func(i, j int) bool {
		if filterOpts.SortOrder == "asc" {
			return filteredItems[i].CreationTimestamp.Before(&filteredItems[j].CreationTimestamp)
		}
		return filteredItems[j].CreationTimestamp.Before(&filteredItems[i].CreationTimestamp)
	})

	totalCount := len(filteredItems)
	start := min(filterOpts.Offset, totalCount)
	end := min(start+filterOpts.Limit, totalCount)
	paginatedItems := filteredItems[start:end]

	response := types.CronTaskListResponse{
		CronTasks: make([]types.CronTaskResponse, 0, len(paginatedItems)),
		Total:     totalCount,
		Pagination: &types.Pagination{
			Limit:      filterOpts.Limit,
			Offset:     filterOpts.Offset,
			TotalCount: totalCount,
			HasMore:    end < totalCount,
		},
	}

	for _, ct := range paginatedItems {
		response.CronTasks = append(response.CronTasks, cronTaskToResponse(&ct))
	}

	writeJSON(w, http.StatusOK, response)
}

// Get returns a specific CronTask
func (h *CronTaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	k8sClient := h.getClient(r)

	var cronTask kubeopenv1alpha1.CronTask
	if err := k8sClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, &cronTask); err != nil {
		writeError(w, http.StatusNotFound, "CronTask not found", err.Error())
		return
	}

	writeResourceOutput(w, r, http.StatusOK, &cronTask, cronTaskToResponse(&cronTask))
}

// Create creates a new CronTask
func (h *CronTaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	k8sClient := h.getClient(r)

	var req types.CreateCronTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Schedule == "" {
		writeError(w, http.StatusBadRequest, "Schedule is required", "")
		return
	}

	// Validate cron expression
	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := cronParser.Parse(req.Schedule); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid cron schedule", err.Error())
		return
	}

	// Validate concurrency policy
	if req.ConcurrencyPolicy != "" {
		switch req.ConcurrencyPolicy {
		case "Allow", "Forbid", "Replace":
			// valid
		default:
			writeError(w, http.StatusBadRequest, "Invalid concurrencyPolicy", "must be one of: Allow, Forbid, Replace")
			return
		}
	}

	if req.AgentRef == nil && req.TemplateRef == nil {
		writeError(w, http.StatusBadRequest, "Invalid request", "either agentRef or templateRef must be specified in taskTemplate")
		return
	}
	if req.AgentRef != nil && req.TemplateRef != nil {
		writeError(w, http.StatusBadRequest, "Invalid request", "only one of agentRef or templateRef can be specified")
		return
	}

	cronTask := &kubeopenv1alpha1.CronTask{}
	cronTask.Namespace = namespace

	if req.Name != "" {
		cronTask.Name = req.Name
	} else {
		cronTask.GenerateName = "crontask-"
	}

	cronTask.Spec.Schedule = req.Schedule

	if req.TimeZone != "" {
		cronTask.Spec.TimeZone = &req.TimeZone
	}
	if req.ConcurrencyPolicy != "" {
		cronTask.Spec.ConcurrencyPolicy = kubeopenv1alpha1.ConcurrencyPolicy(req.ConcurrencyPolicy)
	}
	if req.Suspend {
		cronTask.Spec.Suspend = &req.Suspend
	}
	if req.StartingDeadlineSeconds != nil {
		cronTask.Spec.StartingDeadlineSeconds = req.StartingDeadlineSeconds
	}
	if req.MaxRetainedTasks != nil {
		cronTask.Spec.MaxRetainedTasks = req.MaxRetainedTasks
	}

	// Build task template
	taskSpec := kubeopenv1alpha1.TaskSpec{}
	if req.Description != "" {
		taskSpec.Description = &req.Description
	}
	if req.AgentRef != nil {
		taskSpec.AgentRef = &kubeopenv1alpha1.AgentReference{Name: req.AgentRef.Name}
	}
	if req.TemplateRef != nil {
		taskSpec.TemplateRef = &kubeopenv1alpha1.AgentTemplateReference{Name: req.TemplateRef.Name}
	}
	for _, c := range req.Contexts {
		item := kubeopenv1alpha1.ContextItem{
			Name:        c.Name,
			Description: c.Description,
			MountPath:   c.MountPath,
		}
		if c.Type == "Text" {
			item.Type = kubeopenv1alpha1.ContextTypeText
			item.Text = c.Text
		}
		taskSpec.Contexts = append(taskSpec.Contexts, item)
	}
	cronTask.Spec.TaskTemplate = kubeopenv1alpha1.TaskTemplateSpec{
		Spec: taskSpec,
	}

	if err := k8sClient.Create(r.Context(), cronTask); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create CronTask", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, cronTaskToResponse(cronTask))
}

// Update updates an existing CronTask.
// Accepts either JSON (UpdateCronTaskRequest) or YAML (full CronTask spec replacement).
func (h *CronTaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	k8sClient := h.getClient(r)

	var existing kubeopenv1alpha1.CronTask
	if err := k8sClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, &existing); err != nil {
		writeError(w, http.StatusNotFound, "CronTask not found", err.Error())
		return
	}

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "yaml") {
		// YAML mode: full spec replacement (like Agent and AgentTemplate Update)
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB limit
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Failed to read request body", err.Error())
			return
		}

		var submitted kubeopenv1alpha1.CronTask
		if err := yaml.Unmarshal(body, &submitted); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid YAML", err.Error())
			return
		}

		existing.Spec = submitted.Spec
		if err := k8sClient.Update(r.Context(), &existing); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update CronTask", err.Error())
			return
		}

		writeResourceOutput(w, r, http.StatusOK, &existing, cronTaskToResponse(&existing))
		return
	}

	// JSON mode: partial update
	var req types.UpdateCronTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if req.Schedule != "" {
		existing.Spec.Schedule = req.Schedule
	}
	if req.TimeZone != nil {
		existing.Spec.TimeZone = req.TimeZone
	}
	if req.ConcurrencyPolicy != "" {
		existing.Spec.ConcurrencyPolicy = kubeopenv1alpha1.ConcurrencyPolicy(req.ConcurrencyPolicy)
	}
	if req.Suspend != nil {
		existing.Spec.Suspend = req.Suspend
	}
	if req.MaxRetainedTasks != nil {
		existing.Spec.MaxRetainedTasks = req.MaxRetainedTasks
	}
	if req.StartingDeadlineSeconds != nil {
		existing.Spec.StartingDeadlineSeconds = req.StartingDeadlineSeconds
	}
	if req.Description != nil {
		existing.Spec.TaskTemplate.Spec.Description = req.Description
	}

	if err := k8sClient.Update(r.Context(), &existing); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update CronTask", err.Error())
		return
	}

	writeResourceOutput(w, r, http.StatusOK, &existing, cronTaskToResponse(&existing))
}

// Delete deletes a CronTask
func (h *CronTaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	k8sClient := h.getClient(r)

	var cronTask kubeopenv1alpha1.CronTask
	if err := k8sClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, &cronTask); err != nil {
		writeError(w, http.StatusNotFound, "CronTask not found", err.Error())
		return
	}

	if err := k8sClient.Delete(r.Context(), &cronTask); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete CronTask", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Suspend pauses the CronTask scheduling
func (h *CronTaskHandler) Suspend(w http.ResponseWriter, r *http.Request) {
	h.setSuspendState(w, r, true)
}

// Resume resumes the CronTask scheduling
func (h *CronTaskHandler) Resume(w http.ResponseWriter, r *http.Request) {
	h.setSuspendState(w, r, false)
}

func (h *CronTaskHandler) setSuspendState(w http.ResponseWriter, r *http.Request, suspend bool) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	k8sClient := h.getClient(r)

	var cronTask kubeopenv1alpha1.CronTask
	if err := k8sClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, &cronTask); err != nil {
		writeError(w, http.StatusNotFound, "CronTask not found", err.Error())
		return
	}

	cronTask.Spec.Suspend = &suspend
	if err := k8sClient.Update(r.Context(), &cronTask); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update CronTask", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, cronTaskToResponse(&cronTask))
}

// Trigger manually triggers a CronTask to create a Task immediately
func (h *CronTaskHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	k8sClient := h.getClient(r)

	var cronTask kubeopenv1alpha1.CronTask
	if err := k8sClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, &cronTask); err != nil {
		writeError(w, http.StatusNotFound, "CronTask not found", err.Error())
		return
	}

	if cronTask.Annotations == nil {
		cronTask.Annotations = make(map[string]string)
	}
	cronTask.Annotations[kubeopenv1alpha1.CronTaskTriggerAnnotation] = "true"

	if err := k8sClient.Update(r.Context(), &cronTask); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to trigger CronTask", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("CronTask %q triggered", name),
	})
}

// History returns the list of child Tasks created by this CronTask
func (h *CronTaskHandler) History(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	k8sClient := h.getClient(r)

	filterOpts, err := ParseFilterOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid filter parameters", err.Error())
		return
	}

	var taskList kubeopenv1alpha1.TaskList
	if err := k8sClient.List(r.Context(), &taskList,
		client.InNamespace(namespace),
		client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: name},
	); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list child Tasks", err.Error())
		return
	}

	// Sort by creation time descending (newest first)
	items := taskList.Items
	sort.Slice(items, func(i, j int) bool {
		return items[j].CreationTimestamp.Before(&items[i].CreationTimestamp)
	})

	totalCount := len(items)
	start := min(filterOpts.Offset, totalCount)
	end := min(start+filterOpts.Limit, totalCount)
	paginatedItems := items[start:end]

	response := types.TaskListResponse{
		Tasks: make([]types.TaskResponse, 0, len(paginatedItems)),
		Total: totalCount,
		Pagination: &types.Pagination{
			Limit:      filterOpts.Limit,
			Offset:     filterOpts.Offset,
			TotalCount: totalCount,
			HasMore:    end < totalCount,
		},
	}

	for _, task := range paginatedItems {
		response.Tasks = append(response.Tasks, taskToResponse(&task))
	}

	writeJSON(w, http.StatusOK, response)
}

// cronTaskToResponse converts a CronTask CR to API response format
func cronTaskToResponse(ct *kubeopenv1alpha1.CronTask) types.CronTaskResponse {
	resp := types.CronTaskResponse{
		Name:              ct.Name,
		Namespace:         ct.Namespace,
		Schedule:          ct.Spec.Schedule,
		ConcurrencyPolicy: string(ct.Spec.ConcurrencyPolicy),
		Active:            ct.Status.Active,
		TotalExecutions:   ct.Status.TotalExecutions,
		CreatedAt:         ct.CreationTimestamp.Time,
		Labels:            ct.Labels,
		Conditions:        conditionsToResponse(ct.Status.Conditions),
	}

	if ct.Spec.TimeZone != nil {
		resp.TimeZone = *ct.Spec.TimeZone
	}
	if ct.Spec.Suspend != nil {
		resp.Suspend = *ct.Spec.Suspend
	}
	if ct.Spec.MaxRetainedTasks != nil {
		resp.MaxRetainedTasks = *ct.Spec.MaxRetainedTasks
	}
	if ct.Spec.StartingDeadlineSeconds != nil {
		resp.StartingDeadlineSeconds = ct.Spec.StartingDeadlineSeconds
	}

	if ct.Status.LastScheduleTime != nil {
		t := ct.Status.LastScheduleTime.Time
		resp.LastScheduleTime = &t
	}
	if ct.Status.LastSuccessfulTime != nil {
		t := ct.Status.LastSuccessfulTime.Time
		resp.LastSuccessfulTime = &t
	}
	if ct.Status.NextScheduleTime != nil {
		t := ct.Status.NextScheduleTime.Time
		resp.NextScheduleTime = &t
	}

	// Task template info
	resp.TaskTemplate = types.CronTaskTemplateInfo{}
	if ct.Spec.TaskTemplate.Spec.Description != nil {
		resp.TaskTemplate.Description = *ct.Spec.TaskTemplate.Spec.Description
	}
	if ct.Spec.TaskTemplate.Spec.AgentRef != nil {
		resp.TaskTemplate.AgentRef = &types.AgentReference{Name: ct.Spec.TaskTemplate.Spec.AgentRef.Name}
	}
	if ct.Spec.TaskTemplate.Spec.TemplateRef != nil {
		resp.TaskTemplate.TemplateRef = &types.AgentTemplateReference{Name: ct.Spec.TaskTemplate.Spec.TemplateRef.Name}
	}

	return resp
}
