// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
	"github.com/kubeopencode/kubeopencode/internal/server/types"
)

// TaskHandler handles task-related HTTP requests
type TaskHandler struct {
	defaultClient    client.Client
	defaultClientset kubernetes.Interface
	restConfig       *rest.Config
}

// NewTaskHandler creates a new TaskHandler
func NewTaskHandler(c client.Client, clientset kubernetes.Interface, restConfig *rest.Config) *TaskHandler {
	return &TaskHandler{
		defaultClient:    c,
		defaultClientset: clientset,
		restConfig:       restConfig,
	}
}

// getClient returns the client from context or falls back to default
func (h *TaskHandler) getClient(ctx context.Context) client.Client {
	if c, ok := ctx.Value(clientContextKey{}).(client.Client); ok && c != nil {
		return c
	}
	return h.defaultClient
}

// ListAll returns all tasks across all namespaces with filtering and pagination
func (h *TaskHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	h.listTasks(w, r, "")
}

// List returns all tasks in a namespace with sorting, filtering, and pagination
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	h.listTasks(w, r, namespace)
}

// listTasks is the shared implementation for List and ListAll
func (h *TaskHandler) listTasks(w http.ResponseWriter, r *http.Request, namespace string) {
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	// Parse filter options
	filterOpts, err := ParseFilterOptions(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid filter parameters", err.Error())
		return
	}

	var taskList kubeopenv1alpha1.TaskList
	listOpts := BuildListOptions(namespace, filterOpts)

	if err := k8sClient.List(ctx, &taskList, listOpts...); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list tasks", err.Error())
		return
	}

	// Filter by name and phase (in-memory)
	var filteredItems []kubeopenv1alpha1.Task
	for _, task := range taskList.Items {
		if !MatchesNameFilter(task.Name, filterOpts.Name) {
			continue
		}
		if filterOpts.Phase != "" && !strings.EqualFold(string(task.Status.Phase), filterOpts.Phase) {
			continue
		}
		filteredItems = append(filteredItems, task)
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

	response := types.TaskListResponse{
		Tasks: make([]types.TaskResponse, 0, len(paginatedItems)),
		Total: totalCount,
		Pagination: &types.Pagination{
			Limit:      filterOpts.Limit,
			Offset:     filterOpts.Offset,
			TotalCount: totalCount,
			HasMore:    hasMore,
		},
	}

	for _, task := range paginatedItems {
		response.Tasks = append(response.Tasks, taskToResponse(&task))
	}

	writeJSON(w, http.StatusOK, response)
}

// Get returns a specific task
func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var task kubeopenv1alpha1.Task
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &task); err != nil {
		writeError(w, http.StatusNotFound, "Task not found", err.Error())
		return
	}

	writeResourceOutput(w, r, http.StatusOK, &task, taskToResponse(&task))
}

// Create creates a new task
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var req types.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// Description is required unless a template is specified
	if req.Description == "" && req.TaskTemplateRef == nil {
		writeError(w, http.StatusBadRequest, "Description is required when not using a template", "")
		return
	}

	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: kubeopenv1alpha1.TaskSpec{},
	}

	// Set description if provided
	if req.Description != "" {
		task.Spec.Description = &req.Description
	}

	// Set task template reference if provided
	if req.TaskTemplateRef != nil {
		task.Spec.TaskTemplateRef = &kubeopenv1alpha1.TaskTemplateReference{
			Name:      req.TaskTemplateRef.Name,
			Namespace: req.TaskTemplateRef.Namespace,
		}
	}

	// Set name or generate name
	if req.Name != "" {
		task.ObjectMeta.Name = req.Name
	} else {
		task.ObjectMeta.GenerateName = "task-"
	}

	// Set agent reference if provided
	if req.AgentRef != nil {
		task.Spec.AgentRef = &kubeopenv1alpha1.AgentReference{
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
		}
		task.Spec.Contexts = append(task.Spec.Contexts, item)
	}

	if err := k8sClient.Create(ctx, task); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create task", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, taskToResponse(task))
}

// Delete deletes a task
func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var task kubeopenv1alpha1.Task
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &task); err != nil {
		writeError(w, http.StatusNotFound, "Task not found", err.Error())
		return
	}

	if err := k8sClient.Delete(ctx, &task); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete task", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Stop stops a running task by adding the stop annotation
func (h *TaskHandler) Stop(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	var task kubeopenv1alpha1.Task
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &task); err != nil {
		writeError(w, http.StatusNotFound, "Task not found", err.Error())
		return
	}

	// Check if task is running
	if task.Status.Phase != kubeopenv1alpha1.TaskPhaseRunning {
		writeError(w, http.StatusBadRequest, "Task is not running", fmt.Sprintf("Task phase is %s", task.Status.Phase))
		return
	}

	// Add stop annotation
	if task.Annotations == nil {
		task.Annotations = make(map[string]string)
	}
	task.Annotations["kubeopencode.io/stop"] = "true"

	if err := k8sClient.Update(ctx, &task); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to stop task", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, taskToResponse(&task))
}

// GetLogs streams task logs via Server-Sent Events
func (h *TaskHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	ctx := r.Context()
	k8sClient := h.getClient(ctx)

	// Check if follow mode is requested (default: true for SSE)
	follow := r.URL.Query().Get("follow") != "false"
	// Container name (default: agent)
	container := r.URL.Query().Get("container")
	if container == "" {
		container = "agent"
	}

	var task kubeopenv1alpha1.Task
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &task); err != nil {
		writeError(w, http.StatusNotFound, "Task not found", err.Error())
		return
	}

	if task.Status.PodName == "" {
		writeError(w, http.StatusBadRequest, "Task has no pod", "Pod not yet created")
		return
	}

	// Get pod namespace (might be different from task namespace for cross-namespace agents)
	podNamespace := task.Status.PodNamespace
	if podNamespace == "" {
		podNamespace = namespace
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Streaming not supported", "")
		return
	}

	// Check if pod exists
	var pod corev1.Pod
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: podNamespace, Name: task.Status.PodName}, &pod); err != nil {
		writeSSEEvent(w, flusher, types.LogEvent{Type: "error", Message: fmt.Sprintf("Pod not found: %s", err.Error())})
		return
	}

	// Send initial status
	phase := string(task.Status.Phase)
	podPhase := string(pod.Status.Phase)
	writeSSEEvent(w, flusher, types.LogEvent{Type: "status", Phase: &phase, PodPhase: &podPhase})

	// Stream pod logs using clientset
	h.streamPodLogs(ctx, w, flusher, podNamespace, task.Status.PodName, container, follow, namespace, name)
}

// streamPodLogs streams actual pod logs using clientset
func (h *TaskHandler) streamPodLogs(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, podNamespace, podName, container string, follow bool, taskNamespace, taskName string) {
	// Create pod log options
	logOptions := &corev1.PodLogOptions{
		Container: container,
		Follow:    follow,
	}

	// Get log stream from clientset
	req := h.defaultClientset.CoreV1().Pods(podNamespace).GetLogs(podName, logOptions)
	stream, err := req.Stream(ctx)
	if err != nil {
		// If container not found or not ready, try without specifying container
		logOptions.Container = ""
		req = h.defaultClientset.CoreV1().Pods(podNamespace).GetLogs(podName, logOptions)
		stream, err = req.Stream(ctx)
		if err != nil {
			writeSSEEvent(w, flusher, types.LogEvent{Type: "error", Message: fmt.Sprintf("Failed to get logs: %s", err.Error())})
			return
		}
	}
	defer stream.Close()

	// Read logs line by line and send as SSE events
	reader := bufio.NewReader(stream)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					// Check if task is completed
					k8sClient := h.getClient(ctx)
					var task kubeopenv1alpha1.Task
					phase := "Unknown"
					if getErr := k8sClient.Get(ctx, client.ObjectKey{Namespace: taskNamespace, Name: taskName}, &task); getErr == nil {
						phase = string(task.Status.Phase)
					}
					writeSSEEvent(w, flusher, types.LogEvent{Type: "complete", Phase: &phase})
					return
				}
				writeSSEEvent(w, flusher, types.LogEvent{Type: "error", Message: fmt.Sprintf("Read error: %s", err.Error())})
				return
			}

			// Send log line as SSE event
			logContent := string(line)
			writeSSEEvent(w, flusher, types.LogEvent{Type: "log", Content: &logContent})
		}
	}
}

// writeSSEEvent marshals a LogEvent to JSON and writes it as an SSE data event
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event types.LogEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// taskToResponse converts a Task CRD to an API response
func taskToResponse(task *kubeopenv1alpha1.Task) types.TaskResponse {
	var description string
	if task.Spec.Description != nil {
		description = *task.Spec.Description
	}

	resp := types.TaskResponse{
		Name:         task.Name,
		Namespace:    task.Namespace,
		Phase:        string(task.Status.Phase),
		Description:  description,
		PodName:      task.Status.PodName,
		PodNamespace: task.Status.PodNamespace,
		CreatedAt:    task.CreationTimestamp.Time,
		Labels:       task.Labels,
	}

	if task.Spec.AgentRef != nil {
		resp.AgentRef = &types.AgentReference{
			Name:      task.Spec.AgentRef.Name,
			Namespace: task.Spec.AgentRef.Namespace,
		}
	}

	// Use resolved agent ref from status if available
	if task.Status.AgentRef != nil {
		resp.AgentRef = &types.AgentReference{
			Name:      task.Status.AgentRef.Name,
			Namespace: task.Status.AgentRef.Namespace,
		}
	}

	if task.Status.StartTime != nil {
		t := task.Status.StartTime.Time
		resp.StartTime = &t
	}

	if task.Status.CompletionTime != nil {
		t := task.Status.CompletionTime.Time
		resp.CompletionTime = &t
	}

	// Calculate duration
	if resp.StartTime != nil {
		endTime := time.Now()
		if resp.CompletionTime != nil {
			endTime = *resp.CompletionTime
		}
		resp.Duration = endTime.Sub(*resp.StartTime).Round(time.Second).String()
	}

	// Convert conditions
	for _, c := range task.Status.Conditions {
		resp.Conditions = append(resp.Conditions, types.Condition{
			Type:    c.Type,
			Status:  string(c.Status),
			Reason:  c.Reason,
			Message: c.Message,
		})
	}

	return resp
}
