// Copyright Contributors to the KubeOpenCode project

//go:build !integration

package controller

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

func TestCronScheduleWithTZ(t *testing.T) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	t.Run("timezone-aware Next", func(t *testing.T) {
		// Parse "0 9 * * *" (9:00 AM daily)
		baseSched, err := parser.Parse("0 9 * * *")
		if err != nil {
			t.Fatalf("failed to parse schedule: %v", err)
		}

		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			t.Fatalf("failed to load timezone: %v", err)
		}

		tzSched := &cronScheduleWithTZ{Schedule: baseSched, loc: loc}

		// Use a known UTC time: 2024-01-15 14:00:00 UTC (= 9:00 AM EST)
		now := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC)

		next := tzSched.Next(now)
		// Next should be 9:00 AM EST on Jan 16, which is 14:00 UTC Jan 16
		expectedUTC := time.Date(2024, 1, 16, 14, 0, 0, 0, time.UTC)
		if !next.Equal(expectedUTC) {
			t.Errorf("Next() = %v, want %v (9 AM EST on Jan 16 = 14:00 UTC)", next, expectedUTC)
		}
	})

	t.Run("UTC schedule (no timezone wrapper)", func(t *testing.T) {
		sched, err := parser.Parse("0 9 * * *")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}

		now := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
		next := sched.Next(now)
		expected := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
		if !next.Equal(expected) {
			t.Errorf("Next() = %v, want %v", next, expected)
		}
	})
}

func TestGetMostRecentScheduleTime(t *testing.T) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	r := &CronTaskReconciler{}

	t.Run("no schedule due yet", func(t *testing.T) {
		sched, _ := parser.Parse("0 0 1 1 *") // Jan 1 midnight (far future)
		cronTask := &kubeopenv1alpha1.CronTask{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.NewTime(time.Now()),
			},
		}
		now := time.Now()
		scheduled, missed := r.getMostRecentScheduleTime(cronTask, sched, now)
		if scheduled != nil {
			t.Errorf("expected nil scheduled time, got %v", scheduled)
		}
		if missed != 0 {
			t.Errorf("expected 0 missed, got %d", missed)
		}
	})

	t.Run("one missed schedule", func(t *testing.T) {
		sched, _ := parser.Parse("* * * * *") // every minute
		past := time.Now().Add(-2 * time.Minute)
		cronTask := &kubeopenv1alpha1.CronTask{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.NewTime(past),
			},
		}
		now := time.Now()
		scheduled, missed := r.getMostRecentScheduleTime(cronTask, sched, now)
		if scheduled == nil {
			t.Fatal("expected non-nil scheduled time")
		}
		if missed < 1 {
			t.Errorf("expected at least 1 missed, got %d", missed)
		}
	})

	t.Run("uses lastScheduleTime when set", func(t *testing.T) {
		sched, _ := parser.Parse("* * * * *") // every minute
		creation := time.Now().Add(-10 * time.Minute)
		lastSchedule := time.Now().Add(-1 * time.Minute)
		cronTask := &kubeopenv1alpha1.CronTask{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.NewTime(creation),
			},
			Status: kubeopenv1alpha1.CronTaskStatus{
				LastScheduleTime: &metav1.Time{Time: lastSchedule},
			},
		}
		now := time.Now()
		_, missed := r.getMostRecentScheduleTime(cronTask, sched, now)
		// With lastSchedule only ~1 minute ago, should have ~1 missed
		if missed > 3 {
			t.Errorf("expected <= 3 missed (from lastScheduleTime), got %d", missed)
		}
	})

	t.Run("too many missed schedules caps at 101", func(t *testing.T) {
		sched, _ := parser.Parse("* * * * *") // every minute
		longAgo := time.Now().Add(-200 * time.Minute)
		cronTask := &kubeopenv1alpha1.CronTask{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.NewTime(longAgo),
			},
		}
		now := time.Now()
		_, missed := r.getMostRecentScheduleTime(cronTask, sched, now)
		if missed < 101 {
			t.Errorf("expected > 100 missed (safety break), got %d", missed)
		}
	})
}

func TestIsAtRetainedLimit(t *testing.T) {
	r := &CronTaskReconciler{}

	t.Run("nil maxRetainedTasks never limits", func(t *testing.T) {
		cronTask := &kubeopenv1alpha1.CronTask{}
		tasks := make([]kubeopenv1alpha1.Task, 100)
		if r.isAtRetainedLimit(cronTask, tasks) {
			t.Error("expected no limit when maxRetainedTasks is nil")
		}
	})

	t.Run("under limit", func(t *testing.T) {
		max := int32(10)
		cronTask := &kubeopenv1alpha1.CronTask{
			Spec: kubeopenv1alpha1.CronTaskSpec{MaxRetainedTasks: &max},
		}
		tasks := make([]kubeopenv1alpha1.Task, 5)
		if r.isAtRetainedLimit(cronTask, tasks) {
			t.Error("expected not at limit with 5/10 tasks")
		}
	})

	t.Run("at limit", func(t *testing.T) {
		max := int32(10)
		cronTask := &kubeopenv1alpha1.CronTask{
			Spec: kubeopenv1alpha1.CronTaskSpec{MaxRetainedTasks: &max},
		}
		tasks := make([]kubeopenv1alpha1.Task, 10)
		if !r.isAtRetainedLimit(cronTask, tasks) {
			t.Error("expected at limit with 10/10 tasks")
		}
	})

	t.Run("over limit", func(t *testing.T) {
		max := int32(10)
		cronTask := &kubeopenv1alpha1.CronTask{
			Spec: kubeopenv1alpha1.CronTaskSpec{MaxRetainedTasks: &max},
		}
		tasks := make([]kubeopenv1alpha1.Task, 15)
		if !r.isAtRetainedLimit(cronTask, tasks) {
			t.Error("expected at limit with 15/10 tasks")
		}
	})
}
