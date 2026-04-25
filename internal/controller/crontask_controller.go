// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

const (
	// MaxMissedSchedules is the maximum number of missed cron schedules to track
	// before resetting. Prevents infinite loops during clock skew or long suspends.
	MaxMissedSchedules = 100
)

// CronTaskReconciler reconciles a CronTask object
type CronTaskReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder events.EventRecorder
}

// +kubebuilder:rbac:groups=kubeopencode.io,resources=crontasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubeopencode.io,resources=crontasks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubeopencode.io,resources=crontasks/finalizers,verbs=update
// +kubebuilder:rbac:groups=kubeopencode.io,resources=tasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *CronTaskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get CronTask
	cronTask := &kubeopenv1alpha1.CronTask{}
	if err := r.Get(ctx, req.NamespacedName, cronTask); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Parse the cron schedule
	sched, err := r.parseSchedule(cronTask)
	if err != nil {
		log.Error(err, "invalid cron schedule", "schedule", cronTask.Spec.Schedule)
		r.setCondition(cronTask, "Ready", metav1.ConditionFalse, "InvalidSchedule", fmt.Sprintf("Invalid cron schedule: %v", err))
		if statusErr := r.Status().Update(ctx, cronTask); statusErr != nil {
			log.Error(statusErr, "failed to update status")
		}
		return ctrl.Result{}, nil // Don't requeue, schedule is invalid
	}

	// List all child Tasks owned by this CronTask
	childTasks, err := r.listChildTasks(ctx, cronTask)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Categorize child Tasks
	var activeTasks, finishedTasks []*kubeopenv1alpha1.Task
	for i := range childTasks {
		t := &childTasks[i]
		if isTaskFinished(t.Status.Phase) {
			finishedTasks = append(finishedTasks, t)
		} else {
			activeTasks = append(activeTasks, t)
		}
	}

	// Update lastSuccessfulTime from finished tasks
	r.updateLastSuccessfulTime(cronTask, finishedTasks)

	// Calculate and set next schedule time
	now := time.Now()
	nextScheduleTime := sched.Next(now)
	cronTask.Status.NextScheduleTime = &metav1.Time{Time: nextScheduleTime}

	// Update active count and refs
	cronTask.Status.Active = int32(len(activeTasks)) //nolint:gosec // activeTasks count is bounded by maxRetainedTasks (default 10)
	cronTask.Status.ActiveRefs = make([]corev1.ObjectReference, 0, len(activeTasks))
	for _, t := range activeTasks {
		cronTask.Status.ActiveRefs = append(cronTask.Status.ActiveRefs, corev1.ObjectReference{
			Kind:      "Task",
			Name:      t.Name,
			Namespace: t.Namespace,
			UID:       t.UID,
		})
	}

	// Check if suspended
	if cronTask.Spec.Suspend != nil && *cronTask.Spec.Suspend {
		log.V(1).Info("CronTask is suspended, skipping scheduling")
		r.setCondition(cronTask, "Ready", metav1.ConditionFalse, "Suspended", "CronTask is suspended")
		cronTask.Status.NextScheduleTime = nil
		if statusErr := r.Status().Update(ctx, cronTask); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{}, nil
	}

	// Handle manual trigger annotation
	if cronTask.Annotations != nil && cronTask.Annotations[kubeopenv1alpha1.CronTaskTriggerAnnotation] == "true" {
		log.Info("manual trigger detected")

		// Remove trigger annotation first using patch to avoid race with status updates
		patch := client.MergeFrom(cronTask.DeepCopy())
		delete(cronTask.Annotations, kubeopenv1alpha1.CronTaskTriggerAnnotation)
		if err := r.Patch(ctx, cronTask, patch); err != nil {
			return ctrl.Result{}, err
		}

		// Check maxRetainedTasks before creating
		if r.isAtRetainedLimit(cronTask, childTasks) {
			r.Recorder.Eventf(cronTask, nil, corev1.EventTypeWarning, "MaxRetainedTasksReached", "Trigger",
				"Cannot trigger: %d child Tasks exist (max: %d). Configure global cleanup or delete old Tasks.",
				len(childTasks), *cronTask.Spec.MaxRetainedTasks)
		} else {
			if created, err := r.createTask(ctx, cronTask, now); err != nil {
				return ctrl.Result{}, err
			} else if created {
				cronTask.Status.LastScheduleTime = &metav1.Time{Time: now}
				cronTask.Status.TotalExecutions++
				r.Recorder.Eventf(cronTask, nil, corev1.EventTypeNormal, "Triggered", "CreateTask", "Manually triggered Task creation")
			}
		}

		if statusErr := r.Status().Update(ctx, cronTask); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{}, nil // Will be re-reconciled due to annotation removal
	}

	// Determine missed schedules
	scheduledTime, missedCount := r.getMostRecentScheduleTime(cronTask, sched, now)

	if scheduledTime == nil {
		// No schedule due yet
		r.setCondition(cronTask, "Ready", metav1.ConditionTrue, "Scheduled", "Waiting for next schedule")
		if statusErr := r.Status().Update(ctx, cronTask); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{RequeueAfter: time.Until(nextScheduleTime)}, nil
	}

	// Check starting deadline
	if cronTask.Spec.StartingDeadlineSeconds != nil {
		deadline := scheduledTime.Add(time.Duration(*cronTask.Spec.StartingDeadlineSeconds) * time.Second)
		if now.After(deadline) {
			log.Info("missed starting deadline", "scheduledTime", scheduledTime, "deadline", deadline)
			r.Recorder.Eventf(cronTask, nil, corev1.EventTypeWarning, "MissedDeadline", "Schedule",
				"Missed starting deadline for schedule at %s (deadline: %ds)", scheduledTime.Format(time.RFC3339), *cronTask.Spec.StartingDeadlineSeconds)
			r.setCondition(cronTask, "Ready", metav1.ConditionTrue, "MissedDeadline", "Missed starting deadline, waiting for next schedule")
			if statusErr := r.Status().Update(ctx, cronTask); statusErr != nil {
				return ctrl.Result{}, statusErr
			}
			return ctrl.Result{RequeueAfter: time.Until(nextScheduleTime)}, nil
		}
	}

	if missedCount > MaxMissedSchedules {
		// Too many missed schedules — likely clock skew or long suspend. Reset.
		log.Info("too many missed schedules, resetting", "missed", missedCount)
		r.Recorder.Eventf(cronTask, nil, corev1.EventTypeWarning, "TooManyMissed", "Schedule",
			"Missed %d schedules, resetting to next future schedule", missedCount)
		r.setCondition(cronTask, "Ready", metav1.ConditionTrue, "Scheduled", "Reset after too many missed schedules")
		if statusErr := r.Status().Update(ctx, cronTask); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		return ctrl.Result{RequeueAfter: time.Until(nextScheduleTime)}, nil
	}

	// Check maxRetainedTasks
	if r.isAtRetainedLimit(cronTask, childTasks) {
		log.Info("maxRetainedTasks reached, skipping Task creation",
			"current", len(childTasks), "max", *cronTask.Spec.MaxRetainedTasks)
		r.Recorder.Eventf(cronTask, nil, corev1.EventTypeWarning, "MaxRetainedTasksReached", "Schedule",
			"Cannot create Task: %d child Tasks exist (max: %d). Configure global cleanup or delete old Tasks.",
			len(childTasks), *cronTask.Spec.MaxRetainedTasks)
		r.setCondition(cronTask, "Ready", metav1.ConditionFalse, "MaxRetainedTasksReached",
			fmt.Sprintf("Cannot create Tasks: %d/%d retained Tasks", len(childTasks), *cronTask.Spec.MaxRetainedTasks))
		if statusErr := r.Status().Update(ctx, cronTask); statusErr != nil {
			return ctrl.Result{}, statusErr
		}
		// Requeue periodically to check if cleanup has freed space
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check ConcurrencyPolicy
	if len(activeTasks) > 0 {
		switch cronTask.Spec.ConcurrencyPolicy {
		case kubeopenv1alpha1.ForbidConcurrent:
			log.V(1).Info("concurrency policy Forbid: active Task exists, skipping")
			r.Recorder.Eventf(cronTask, nil, corev1.EventTypeNormal, "TaskStillActive", "Schedule",
				"Skipping scheduled Task creation: %d active Task(s) (concurrencyPolicy: Forbid)", len(activeTasks))
			r.setCondition(cronTask, "Ready", metav1.ConditionTrue, "ActiveTaskExists", "Skipped due to concurrencyPolicy Forbid")
			if statusErr := r.Status().Update(ctx, cronTask); statusErr != nil {
				return ctrl.Result{}, statusErr
			}
			return ctrl.Result{RequeueAfter: time.Until(nextScheduleTime)}, nil

		case kubeopenv1alpha1.ReplaceConcurrent:
			log.Info("concurrency policy Replace: stopping active Tasks")
			for _, activeTask := range activeTasks {
				if err := r.stopTask(ctx, activeTask); err != nil {
					log.Error(err, "failed to stop active Task", "task", activeTask.Name)
					return ctrl.Result{}, err
				}
			}
			r.Recorder.Eventf(cronTask, nil, corev1.EventTypeNormal, "ReplacedTasks", "Schedule",
				"Stopped %d active Task(s) for replacement (concurrencyPolicy: Replace)", len(activeTasks))

		case kubeopenv1alpha1.AllowConcurrent:
			// Allow: proceed to create new Task
		}
	}

	// Create the scheduled Task
	created, err := r.createTask(ctx, cronTask, *scheduledTime)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update status only if a new Task was actually created
	cronTask.Status.LastScheduleTime = &metav1.Time{Time: *scheduledTime}
	if created {
		cronTask.Status.TotalExecutions++
	}
	r.setCondition(cronTask, "Ready", metav1.ConditionTrue, "Scheduled", "Task created successfully")

	if statusErr := r.Status().Update(ctx, cronTask); statusErr != nil {
		return ctrl.Result{}, statusErr
	}

	return ctrl.Result{RequeueAfter: time.Until(nextScheduleTime)}, nil
}

// parseSchedule parses the cron schedule with optional timezone.
func (r *CronTaskReconciler) parseSchedule(cronTask *kubeopenv1alpha1.CronTask) (cron.Schedule, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	if cronTask.Spec.TimeZone != nil && *cronTask.Spec.TimeZone != "" {
		loc, err := time.LoadLocation(*cronTask.Spec.TimeZone)
		if err != nil {
			return nil, fmt.Errorf("invalid timezone %q: %w", *cronTask.Spec.TimeZone, err)
		}
		sched, err := parser.Parse(cronTask.Spec.Schedule)
		if err != nil {
			return nil, err
		}
		return &cronScheduleWithTZ{Schedule: sched, loc: loc}, nil
	}

	return parser.Parse(cronTask.Spec.Schedule)
}

// cronScheduleWithTZ wraps a cron.Schedule to compute next times in a specific timezone.
type cronScheduleWithTZ struct {
	cron.Schedule
	loc *time.Location
}

func (s *cronScheduleWithTZ) Next(t time.Time) time.Time {
	return s.Schedule.Next(t.In(s.loc))
}

// listChildTasks returns all Tasks owned by this CronTask.
func (r *CronTaskReconciler) listChildTasks(ctx context.Context, cronTask *kubeopenv1alpha1.CronTask) ([]kubeopenv1alpha1.Task, error) {
	taskList := &kubeopenv1alpha1.TaskList{}
	if err := r.List(ctx, taskList,
		client.InNamespace(cronTask.Namespace),
		client.MatchingLabels{kubeopenv1alpha1.CronTaskLabelKey: cronTask.Name},
	); err != nil {
		return nil, fmt.Errorf("failed to list child Tasks: %w", err)
	}
	return taskList.Items, nil
}

// isAtRetainedLimit checks if the number of child Tasks has reached maxRetainedTasks.
func (r *CronTaskReconciler) isAtRetainedLimit(cronTask *kubeopenv1alpha1.CronTask, childTasks []kubeopenv1alpha1.Task) bool {
	if cronTask.Spec.MaxRetainedTasks == nil {
		return false
	}
	return int32(len(childTasks)) >= *cronTask.Spec.MaxRetainedTasks //nolint:gosec // childTasks count is bounded by maxRetainedTasks
}

// getMostRecentScheduleTime returns the most recent unmet schedule time and how many were missed.
// Returns nil if no schedule is due yet.
func (r *CronTaskReconciler) getMostRecentScheduleTime(
	cronTask *kubeopenv1alpha1.CronTask, sched cron.Schedule, now time.Time,
) (*time.Time, int64) {
	// Use the later of: creation time or last schedule time
	earliestTime := cronTask.CreationTimestamp.Time
	if cronTask.Status.LastScheduleTime != nil {
		earliestTime = cronTask.Status.LastScheduleTime.Time
	}

	var missedCount int64
	var mostRecent *time.Time

	t := sched.Next(earliestTime)
	for !t.After(now) {
		tCopy := t
		mostRecent = &tCopy
		missedCount++
		t = sched.Next(t)

		// Safety: prevent infinite loop
		if missedCount > MaxMissedSchedules {
			break
		}
	}

	return mostRecent, missedCount
}

// createTask creates a new Task from the CronTask's taskTemplate.
// Returns true if a new Task was created, false if it already existed.
func (r *CronTaskReconciler) createTask(ctx context.Context, cronTask *kubeopenv1alpha1.CronTask, scheduledTime time.Time) (bool, error) {
	log := log.FromContext(ctx)

	taskName := fmt.Sprintf("%s-%d", cronTask.Name, scheduledTime.Unix())

	// Build labels: merge template labels with CronTask tracking label
	labels := make(map[string]string)
	for k, v := range cronTask.Spec.TaskTemplate.Metadata.Labels {
		labels[k] = v
	}
	labels[kubeopenv1alpha1.CronTaskLabelKey] = cronTask.Name

	// Build annotations from template
	annotations := make(map[string]string)
	for k, v := range cronTask.Spec.TaskTemplate.Metadata.Annotations {
		annotations[k] = v
	}

	task := &kubeopenv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:        taskName,
			Namespace:   cronTask.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cronTask, kubeopenv1alpha1.GroupVersion.WithKind("CronTask")),
			},
		},
		Spec: *cronTask.Spec.TaskTemplate.Spec.DeepCopy(),
	}

	if err := r.Create(ctx, task); err != nil {
		if errors.IsAlreadyExists(err) {
			log.V(1).Info("Task already exists, skipping creation", "task", taskName)
			return false, nil
		}
		r.Recorder.Eventf(cronTask, nil, corev1.EventTypeWarning, "TaskCreationFailed", "CreateTask",
			"Failed to create Task %s: %v", taskName, err)
		return false, fmt.Errorf("failed to create Task %s: %w", taskName, err)
	}

	log.Info("created scheduled Task", "task", taskName)
	r.Recorder.Eventf(cronTask, nil, corev1.EventTypeNormal, "TaskCreated", "CreateTask",
		"Created Task %s", taskName)
	return true, nil
}

// stopTask stops a Task by adding the stop annotation.
func (r *CronTaskReconciler) stopTask(ctx context.Context, task *kubeopenv1alpha1.Task) error {
	if task.Annotations == nil {
		task.Annotations = make(map[string]string)
	}
	task.Annotations[AnnotationStop] = "true"
	return r.Update(ctx, task)
}

// updateLastSuccessfulTime updates lastSuccessfulTime from completed child Tasks.
func (r *CronTaskReconciler) updateLastSuccessfulTime(cronTask *kubeopenv1alpha1.CronTask, finishedTasks []*kubeopenv1alpha1.Task) {
	// Sort by completion time descending
	sort.Slice(finishedTasks, func(i, j int) bool {
		ti := finishedTasks[i].Status.CompletionTime
		tj := finishedTasks[j].Status.CompletionTime
		if ti == nil {
			return false
		}
		if tj == nil {
			return true
		}
		return ti.After(tj.Time)
	})

	for _, t := range finishedTasks {
		if t.Status.Phase == kubeopenv1alpha1.TaskPhaseCompleted && t.Status.CompletionTime != nil {
			cronTask.Status.LastSuccessfulTime = t.Status.CompletionTime
			break
		}
	}
}

// setCondition sets a condition on the CronTask status.
func (r *CronTaskReconciler) setCondition(cronTask *kubeopenv1alpha1.CronTask, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&cronTask.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		ObservedGeneration: cronTask.Generation,
		Reason:             reason,
		Message:            message,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *CronTaskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubeopenv1alpha1.CronTask{}).
		Owns(&kubeopenv1alpha1.Task{}).
		Complete(r)
}
