// Copyright Contributors to the KubeTask project

// Package webhook provides an HTTP server for receiving webhooks and creating Tasks.
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubetaskv1alpha1 "github.com/kubetask/kubetask/api/v1alpha1"
)

// Server is an HTTP server that handles webhook requests and creates Tasks.
type Server struct {
	client client.Client
	log    logr.Logger

	// triggers maps webhook paths to their configurations
	// key format: "<namespace>/<name>"
	triggers map[string]*kubetaskv1alpha1.WebhookTrigger
	mu       sync.RWMutex

	// celFilter evaluates CEL expressions for webhook filtering
	celFilter *CELFilter

	// httpServer is the underlying HTTP server
	httpServer *http.Server
}

// NewServer creates a new webhook server.
func NewServer(c client.Client, log logr.Logger) *Server {
	return &Server{
		client:    c,
		log:       log.WithName("webhook-server"),
		triggers:  make(map[string]*kubetaskv1alpha1.WebhookTrigger),
		celFilter: NewCELFilter(),
	}
}

// RegisterTrigger registers or updates a WebhookTrigger.
func (s *Server) RegisterTrigger(trigger *kubetaskv1alpha1.WebhookTrigger) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s/%s", trigger.Namespace, trigger.Name)
	s.triggers[key] = trigger.DeepCopy()
	s.log.Info("Registered webhook trigger", "key", key)
}

// UnregisterTrigger removes a WebhookTrigger.
func (s *Server) UnregisterTrigger(namespace, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	delete(s.triggers, key)
	s.log.Info("Unregistered webhook trigger", "key", key)
}

// GetTrigger retrieves a WebhookTrigger by namespace and name.
func (s *Server) GetTrigger(namespace, name string) (*kubetaskv1alpha1.WebhookTrigger, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	trigger, ok := s.triggers[key]
	if !ok {
		return nil, false
	}
	return trigger.DeepCopy(), true
}

// Start starts the HTTP server on the specified port.
func (s *Server) Start(ctx context.Context, port int) error {
	mux := http.NewServeMux()

	// Main webhook handler
	mux.HandleFunc("/webhooks/", s.handleWebhook)

	// Health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Ready check endpoint
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	s.log.Info("Starting webhook server", "port", port)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		s.log.Info("Shutting down webhook server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// handleWebhook processes incoming webhook requests.
// Expected URL format: /webhooks/<namespace>/<trigger-name>
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse path: /webhooks/<namespace>/<name>
	// Path should be at least "/webhooks/ns/name" = 3 parts after split
	path := r.URL.Path
	if len(path) < len("/webhooks/") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Remove "/webhooks/" prefix and split
	remaining := path[len("/webhooks/"):]
	var namespace, name string
	for i := 0; i < len(remaining); i++ {
		if remaining[i] == '/' {
			namespace = remaining[:i]
			name = remaining[i+1:]
			break
		}
	}

	if namespace == "" || name == "" {
		http.Error(w, "Invalid webhook path, expected /webhooks/<namespace>/<name>", http.StatusBadRequest)
		return
	}

	log := s.log.WithValues("namespace", namespace, "name", name)

	// Get trigger configuration
	trigger, ok := s.GetTrigger(namespace, name)
	if !ok {
		log.Info("Webhook trigger not found")
		http.Error(w, "Webhook trigger not found", http.StatusNotFound)
		return
	}

	// Read request body
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		log.Error(err, "Failed to read request body")
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Validate authentication
	if trigger.Spec.Auth != nil {
		if err := s.validateAuth(r, body, trigger.Spec.Auth, namespace); err != nil {
			log.Info("Authentication failed", "error", err.Error())
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			return
		}
	}

	// Parse payload
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Error(err, "Failed to parse JSON payload")
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Apply CEL filter
	match, err := s.celFilter.Evaluate(trigger.Spec.Filter, payload, r.Header)
	if err != nil {
		log.Error(err, "Failed to evaluate CEL filter")
		http.Error(w, "Filter evaluation error", http.StatusBadRequest)
		return
	}
	if !match {
		log.Info("Webhook did not match filter")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "filtered"}`))
		return
	}

	// Handle concurrency policy
	ctx := r.Context()
	if err := s.handleConcurrency(ctx, trigger, namespace); err != nil {
		if err == errConcurrencySkipped {
			log.Info("Webhook skipped due to concurrency policy")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "skipped", "reason": "concurrency_policy"}`))
			return
		}
		log.Error(err, "Failed to handle concurrency")
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Create Task
	task, err := s.createTask(ctx, trigger, payload, namespace)
	if err != nil {
		log.Error(err, "Failed to create Task")
		http.Error(w, "Failed to create Task", http.StatusInternalServerError)
		return
	}

	log.Info("Created Task from webhook", "task", task.Name)

	// Update trigger status
	if err := s.updateTriggerStatus(ctx, trigger, task.Name); err != nil {
		log.Error(err, "Failed to update trigger status")
		// Don't fail the request, task was created successfully
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "created",
		"task":      task.Name,
		"namespace": task.Namespace,
	})
}

var errConcurrencySkipped = fmt.Errorf("skipped due to concurrency policy")

// handleConcurrency applies the concurrency policy.
func (s *Server) handleConcurrency(ctx context.Context, trigger *kubetaskv1alpha1.WebhookTrigger, namespace string) error {
	policy := trigger.Spec.ConcurrencyPolicy
	if policy == "" {
		policy = kubetaskv1alpha1.ConcurrencyPolicyAllow
	}

	if policy == kubetaskv1alpha1.ConcurrencyPolicyAllow {
		return nil
	}

	// Query active tasks directly from cluster for accurate state
	activeTasks, err := s.getActiveTasks(ctx, namespace, trigger.Name)
	if err != nil {
		return fmt.Errorf("failed to get active tasks: %w", err)
	}

	switch policy {
	case kubetaskv1alpha1.ConcurrencyPolicyForbid:
		// Check if there are active tasks
		if len(activeTasks) > 0 {
			return errConcurrencySkipped
		}
		return nil

	case kubetaskv1alpha1.ConcurrencyPolicyReplace:
		// Stop all active tasks
		for _, taskName := range activeTasks {
			if err := s.stopTask(ctx, namespace, taskName); err != nil {
				s.log.Error(err, "Failed to stop task", "task", taskName)
				// Continue trying to stop other tasks
			}
		}
		return nil
	}

	return nil
}

// getActiveTasks returns the list of active tasks for a trigger by querying directly.
func (s *Server) getActiveTasks(ctx context.Context, namespace, triggerName string) ([]string, error) {
	taskList := &kubetaskv1alpha1.TaskList{}
	if err := s.client.List(ctx, taskList,
		client.InNamespace(namespace),
		client.MatchingLabels{"kubetask.io/webhook-trigger": triggerName},
	); err != nil {
		return nil, err
	}

	var activeTasks []string
	for _, task := range taskList.Items {
		// Include tasks that are still active (not completed or failed)
		if task.Status.Phase != kubetaskv1alpha1.TaskPhaseCompleted &&
			task.Status.Phase != kubetaskv1alpha1.TaskPhaseFailed {
			activeTasks = append(activeTasks, task.Name)
		}
	}

	return activeTasks, nil
}

// stopTask stops a running task by adding the stop annotation.
func (s *Server) stopTask(ctx context.Context, namespace, name string) error {
	task := &kubetaskv1alpha1.Task{}
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, task); err != nil {
		return err
	}

	// Check if task is already completed/failed
	if task.Status.Phase == kubetaskv1alpha1.TaskPhaseCompleted ||
		task.Status.Phase == kubetaskv1alpha1.TaskPhaseFailed {
		return nil
	}

	// Add stop annotation
	if task.Annotations == nil {
		task.Annotations = make(map[string]string)
	}
	task.Annotations["kubetask.io/stop"] = "true"

	return s.client.Update(ctx, task)
}

// createTask creates a new Task from the webhook trigger template.
func (s *Server) createTask(ctx context.Context, trigger *kubetaskv1alpha1.WebhookTrigger, payload map[string]interface{}, namespace string) (*kubetaskv1alpha1.Task, error) {
	// Render description template
	description, err := RenderTemplate(trigger.Spec.TaskTemplate.Description, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to render description template: %w", err)
	}

	// Create Task
	task := &kubetaskv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", trigger.Name),
			Namespace:    namespace,
			Labels: map[string]string{
				"kubetask.io/webhook-trigger": trigger.Name,
			},
		},
		Spec: kubetaskv1alpha1.TaskSpec{
			Description: &description,
			AgentRef:    trigger.Spec.TaskTemplate.AgentRef,
			Contexts:    trigger.Spec.TaskTemplate.Contexts,
		},
	}

	if err := s.client.Create(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

// updateTriggerStatus updates the WebhookTrigger status after creating a task.
func (s *Server) updateTriggerStatus(ctx context.Context, trigger *kubetaskv1alpha1.WebhookTrigger, taskName string) error {
	// Get fresh trigger
	currentTrigger := &kubetaskv1alpha1.WebhookTrigger{}
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: trigger.Namespace, Name: trigger.Name}, currentTrigger); err != nil {
		return err
	}

	// Update status
	now := metav1.Now()
	currentTrigger.Status.LastTriggeredTime = &now
	currentTrigger.Status.TotalTriggered++
	currentTrigger.Status.ActiveTasks = append(currentTrigger.Status.ActiveTasks, taskName)

	return s.client.Status().Update(ctx, currentTrigger)
}
