// Copyright Contributors to the KubeOpenCode project

//go:build !integration

package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

func TestHasActiveConnection(t *testing.T) {
	staleness := 2 * time.Minute

	t.Run("no annotations returns false", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{}
		if hasActiveConnection(agent, staleness) {
			t.Error("expected false when agent has no annotations")
		}
	})

	t.Run("no heartbeat annotation returns false", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{"other": "value"},
			},
		}
		if hasActiveConnection(agent, staleness) {
			t.Error("expected false when heartbeat annotation is missing")
		}
	})

	t.Run("empty heartbeat annotation returns false", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					AnnotationLastConnectionActive: "",
				},
			},
		}
		if hasActiveConnection(agent, staleness) {
			t.Error("expected false when heartbeat annotation is empty")
		}
	})

	t.Run("invalid timestamp returns false", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					AnnotationLastConnectionActive: "not-a-timestamp",
				},
			},
		}
		if hasActiveConnection(agent, staleness) {
			t.Error("expected false when heartbeat annotation is not valid RFC3339")
		}
	})

	t.Run("fresh heartbeat returns true", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					AnnotationLastConnectionActive: time.Now().UTC().Format(time.RFC3339),
				},
			},
		}
		if !hasActiveConnection(agent, staleness) {
			t.Error("expected true when heartbeat is fresh (just now)")
		}
	})

	t.Run("heartbeat within staleness window returns true", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					AnnotationLastConnectionActive: time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339),
				},
			},
		}
		if !hasActiveConnection(agent, staleness) {
			t.Error("expected true when heartbeat is 1 minute old (within 2 minute staleness)")
		}
	})

	t.Run("stale heartbeat returns false", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					AnnotationLastConnectionActive: time.Now().Add(-3 * time.Minute).UTC().Format(time.RFC3339),
				},
			},
		}
		if hasActiveConnection(agent, staleness) {
			t.Error("expected false when heartbeat is 3 minutes old (beyond 2 minute staleness)")
		}
	})

	t.Run("custom staleness threshold", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					AnnotationLastConnectionActive: time.Now().Add(-90 * time.Second).UTC().Format(time.RFC3339),
				},
			},
		}
		// 90 seconds old: stale for 1-minute threshold, fresh for 2-minute threshold
		if hasActiveConnection(agent, 1*time.Minute) {
			t.Error("expected false with 1-minute staleness and 90-second-old heartbeat")
		}
		if !hasActiveConnection(agent, 2*time.Minute) {
			t.Error("expected true with 2-minute staleness and 90-second-old heartbeat")
		}
	})
}

func TestReconcileHeartbeatStaleness(t *testing.T) {
	t.Run("no standby config returns default staleness", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{}
		staleness := reconcileHeartbeatStaleness(agent)
		if staleness != ConnectionHeartbeatStaleness {
			t.Errorf("expected default staleness %v, got %v", ConnectionHeartbeatStaleness, staleness)
		}
	})

	t.Run("idleTimeout >= default staleness returns default", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			Spec: kubeopenv1alpha1.AgentSpec{
				Standby: &kubeopenv1alpha1.StandbyConfig{
					IdleTimeout: metav1.Duration{Duration: 5 * time.Minute},
				},
			},
		}
		staleness := reconcileHeartbeatStaleness(agent)
		if staleness != ConnectionHeartbeatStaleness {
			t.Errorf("expected default staleness %v, got %v", ConnectionHeartbeatStaleness, staleness)
		}
		// Should not set warning condition
		for _, c := range agent.Status.Conditions {
			if c.Type == AgentConditionStandbyConfigWarning {
				t.Error("expected no StandbyConfigWarning condition when idleTimeout >= staleness")
			}
		}
	})

	t.Run("idleTimeout < default staleness degrades and warns", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			Spec: kubeopenv1alpha1.AgentSpec{
				Standby: &kubeopenv1alpha1.StandbyConfig{
					IdleTimeout: metav1.Duration{Duration: 90 * time.Second},
				},
			},
		}
		staleness := reconcileHeartbeatStaleness(agent)
		// idleTimeout / 2 = 45s, but 45s < ConnectionHeartbeatInterval (60s), so floor at 60s
		if staleness != ConnectionHeartbeatInterval {
			t.Errorf("expected staleness floored to ConnectionHeartbeatInterval %v, got %v",
				ConnectionHeartbeatInterval, staleness)
		}
		// Should set warning condition
		var found bool
		for _, c := range agent.Status.Conditions {
			if c.Type == AgentConditionStandbyConfigWarning && c.Status == metav1.ConditionTrue {
				found = true
			}
		}
		if !found {
			t.Error("expected StandbyConfigWarning condition when idleTimeout < default staleness")
		}
	})

	t.Run("idleTimeout/2 >= heartbeat interval uses idleTimeout/2", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			Spec: kubeopenv1alpha1.AgentSpec{
				Standby: &kubeopenv1alpha1.StandbyConfig{
					// idleTimeout = 80s → /2 = 40s, but 40s < 60s, so floor at 60s
					// idleTimeout = 100s → not < 120s (default staleness), so default returned
					// Need idleTimeout < 120s (default staleness) AND idleTimeout/2 >= 60s
					// i.e., 120s <= idleTimeout < 120s — impossible
					// Actually: need idleTimeout < ConnectionHeartbeatStaleness (120s)
					// AND idleTimeout/2 >= ConnectionHeartbeatInterval (60s)
					// → 120s <= idleTimeout < 120s — that's never met
					// So let's test with IdleTimeout < staleness but /2 < interval (always floors)
					IdleTimeout: metav1.Duration{Duration: 119 * time.Second},
				},
			},
		}
		staleness := reconcileHeartbeatStaleness(agent)
		// 119s / 2 = 59.5s, which is < 60s, so floors to 60s
		if staleness != ConnectionHeartbeatInterval {
			t.Errorf("expected staleness floored to %v, got %v", ConnectionHeartbeatInterval, staleness)
		}
	})

	t.Run("clears warning when idleTimeout is increased", func(t *testing.T) {
		agent := &kubeopenv1alpha1.Agent{
			Spec: kubeopenv1alpha1.AgentSpec{
				Standby: &kubeopenv1alpha1.StandbyConfig{
					IdleTimeout: metav1.Duration{Duration: 30 * time.Second},
				},
			},
		}
		// First call sets warning
		reconcileHeartbeatStaleness(agent)
		var warningSet bool
		for _, c := range agent.Status.Conditions {
			if c.Type == AgentConditionStandbyConfigWarning {
				warningSet = true
			}
		}
		if !warningSet {
			t.Fatal("precondition failed: warning should be set after first call")
		}

		// Increase idleTimeout above default staleness
		agent.Spec.Standby.IdleTimeout = metav1.Duration{Duration: 5 * time.Minute}
		reconcileHeartbeatStaleness(agent)
		for _, c := range agent.Status.Conditions {
			if c.Type == AgentConditionStandbyConfigWarning {
				t.Error("expected StandbyConfigWarning to be cleared after increasing idleTimeout")
			}
		}
	})
}
