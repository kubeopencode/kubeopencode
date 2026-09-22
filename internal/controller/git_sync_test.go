// Copyright Contributors to the KubeOpenCode project

//go:build !integration

package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeopenv1alpha1 "github.com/kubeopencode/kubeopencode/api/v1alpha1"
)

// --- Helper functions for tests ---

func newTestReconciler(lsRemote GitLsRemoteFunc, activeTaskCount int) *AgentReconciler {
	return &AgentReconciler{
		GitLsRemoteFn: lsRemote,
		CountActiveTasksFn: func(_ context.Context, _, _ string) (int, error) {
			return activeTaskCount, nil
		},
	}
}

func newAgent(name string, syncStatuses []kubeopenv1alpha1.GitSyncStatus, conditions []metav1.Condition) *kubeopenv1alpha1.Agent {
	return &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  "default",
			Generation: 1,
		},
		Status: kubeopenv1alpha1.AgentStatus{
			GitSyncStatuses: syncStatuses,
			Conditions:      conditions,
		},
	}
}

func rolloutMount(name, repo string, interval time.Duration) gitMount {
	return gitMount{
		contextName:  name,
		repository:   repo,
		ref:          "main",
		mountPath:    "/workspace/" + name,
		depth:        1,
		syncEnabled:  true,
		syncPolicy:   kubeopenv1alpha1.GitSyncPolicyRollout,
		syncInterval: interval,
	}
}

func hotReloadMount(name, repo string) gitMount {
	return gitMount{
		contextName:  name,
		repository:   repo,
		ref:          "main",
		mountPath:    "/workspace/" + name,
		depth:        1,
		syncEnabled:  true,
		syncPolicy:   kubeopenv1alpha1.GitSyncPolicyHotReload,
		syncInterval: 5 * time.Minute,
	}
}

func noSyncMount(name, repo string) gitMount {
	return gitMount{
		contextName: name,
		repository:  repo,
		ref:         "main",
		mountPath:   "/workspace/" + name,
		depth:       1,
	}
}

func mockLsRemote(hashes map[string]string) GitLsRemoteFunc {
	return func(_ context.Context, repo, _, _ string) (string, error) {
		h, ok := hashes[repo]
		if !ok {
			return "", fmt.Errorf("unknown repo: %s", repo)
		}
		return h, nil
	}
}

// --- Helper function tests ---

func TestUpdateSyncStatus(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{}
	now := metav1.Now()

	updateSyncStatus(agent, "test-context", "abc123", &now)
	if len(agent.Status.GitSyncStatuses) != 1 {
		t.Fatalf("expected 1 status entry, got %d", len(agent.Status.GitSyncStatuses))
	}
	if agent.Status.GitSyncStatuses[0].CommitHash != "abc123" {
		t.Errorf("expected hash 'abc123', got %q", agent.Status.GitSyncStatuses[0].CommitHash)
	}

	updateSyncStatus(agent, "test-context", "def456", &now)
	if len(agent.Status.GitSyncStatuses) != 1 {
		t.Fatalf("expected 1 status entry after update, got %d", len(agent.Status.GitSyncStatuses))
	}
	if agent.Status.GitSyncStatuses[0].CommitHash != "def456" {
		t.Errorf("expected hash 'def456', got %q", agent.Status.GitSyncStatuses[0].CommitHash)
	}

	updateSyncStatus(agent, "other-context", "789abc", &now)
	if len(agent.Status.GitSyncStatuses) != 2 {
		t.Fatalf("expected 2 status entries, got %d", len(agent.Status.GitSyncStatuses))
	}
}

func TestGetStatusCommitHash(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		Status: kubeopenv1alpha1.AgentStatus{
			GitSyncStatuses: []kubeopenv1alpha1.GitSyncStatus{
				{Name: "ctx-a", CommitHash: "hash-a"},
				{Name: "ctx-b", CommitHash: "hash-b"},
			},
		},
	}

	if h := getStatusCommitHash(agent, "ctx-a"); h != "hash-a" {
		t.Errorf("expected 'hash-a', got %q", h)
	}
	if h := getStatusCommitHash(agent, "nonexistent"); h != "" {
		t.Errorf("expected empty string, got %q", h)
	}
}

func TestSetAndClearGitSyncPending(t *testing.T) {
	agent := &kubeopenv1alpha1.Agent{
		ObjectMeta: metav1.ObjectMeta{Generation: 1},
	}

	setGitSyncPending(agent, 3, []string{"ctx-a", "ctx-b"})
	cond := meta.FindStatusCondition(agent.Status.Conditions, AgentConditionGitSyncPending)
	if cond == nil {
		t.Fatal("expected GitSyncPending condition to be set")
	}
	if cond.Status != metav1.ConditionTrue {
		t.Errorf("expected ConditionTrue, got %v", cond.Status)
	}

	clearGitSyncPending(agent)
	cond = meta.FindStatusCondition(agent.Status.Conditions, AgentConditionGitSyncPending)
	if cond == nil {
		t.Fatal("expected GitSyncPending condition to still exist (as False)")
	}
	if cond.Status != metav1.ConditionFalse {
		t.Errorf("expected ConditionFalse, got %v", cond.Status)
	}
}

func TestBuildGitSyncSidecar(t *testing.T) {
	gm := gitMount{
		contextName:  "my-context",
		repository:   "https://github.com/org/repo.git",
		ref:          "main",
		syncEnabled:  true,
		syncPolicy:   kubeopenv1alpha1.GitSyncPolicyHotReload,
		syncInterval: 10 * time.Minute,
		secretName:   "git-creds",
	}
	sysCfg := systemConfig{systemImage: "ghcr.io/kubeopencode/kubeopencode:latest"}

	sidecar := buildGitSyncSidecar(gm, "git-context-0", 0, sysCfg, "")

	if sidecar.Name != "git-sync-0" {
		t.Errorf("expected name 'git-sync-0', got %q", sidecar.Name)
	}

	envMap := make(map[string]string)
	for _, env := range sidecar.Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}
	if envMap["GIT_SYNC_INTERVAL"] != "600" {
		t.Errorf("expected GIT_SYNC_INTERVAL=600, got %q", envMap["GIT_SYNC_INTERVAL"])
	}

	hasUsername := false
	for _, env := range sidecar.Env {
		if env.Name == "GIT_USERNAME" && env.ValueFrom != nil {
			hasUsername = true
		}
	}
	if !hasUsername {
		t.Error("expected GIT_USERNAME env var from secret")
	}
	if sidecar.SecurityContext == nil {
		t.Error("expected SecurityContext on sidecar")
	}
}

func TestBuildGitSyncSidecar_Reload(t *testing.T) {
	reloadMount := gitMount{
		contextName:  "skill-org-skills",
		repository:   "https://github.com/org/skills.git",
		ref:          "main",
		mountPath:    "/skills/org-skills",
		syncEnabled:  true,
		syncPolicy:   kubeopenv1alpha1.GitSyncPolicyHotReload,
		syncInterval: 15 * time.Minute,
		reloadOnSync: true,
	}
	plainMount := reloadMount
	plainMount.reloadOnSync = false

	sysCfg := defaultSystemConfig()
	const serverURL = "http://127.0.0.1:4096"

	hasReloadEnv := func(envs []corev1.EnvVar) bool {
		for _, e := range envs {
			if e.Name == EnvOpenCodeReloadURL {
				return true
			}
		}
		return false
	}

	t.Run("reload mount gets the server URL", func(t *testing.T) {
		sidecar := buildGitSyncSidecar(reloadMount, "git-context-0", 0, sysCfg, serverURL)
		if !hasEnvVar(sidecar.Env, EnvOpenCodeReloadURL, serverURL) {
			t.Errorf("expected %s=%s on the sidecar", EnvOpenCodeReloadURL, serverURL)
		}
	})

	t.Run("non-reload mount gets no server URL", func(t *testing.T) {
		sidecar := buildGitSyncSidecar(plainMount, "git-context-0", 0, sysCfg, serverURL)
		if hasReloadEnv(sidecar.Env) {
			t.Errorf("did not expect %s on a mount without reload", EnvOpenCodeReloadURL)
		}
	})

	t.Run("no server URL means no reload env", func(t *testing.T) {
		// Ephemeral Task pods have no long-lived server to reload.
		sidecar := buildGitSyncSidecar(reloadMount, "git-context-0", 0, sysCfg, "")
		if hasReloadEnv(sidecar.Env) {
			t.Errorf("did not expect %s when no server URL is provided", EnvOpenCodeReloadURL)
		}
	})

	t.Run("reload does not disturb existing env", func(t *testing.T) {
		sidecar := buildGitSyncSidecar(reloadMount, "git-context-0", 0, sysCfg, serverURL)
		if !hasEnvVar(sidecar.Env, "GIT_SYNC_INTERVAL", "900") {
			t.Error("expected the 15m interval to be passed through")
		}
		if !hasEnvVar(sidecar.Env, "GIT_REPO", reloadMount.repository) {
			t.Error("expected GIT_REPO to be preserved")
		}
	})
}

func TestTruncateHash(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"abcdef1234567890", "abcdef123456"},
		{"short", "short"},
		{"", ""},
		{"exactly12ch", "exactly12ch"},
	}
	for _, tt := range tests {
		if got := truncateHash(tt.in); got != tt.want {
			t.Errorf("truncateHash(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// --- reconcileGitSync tests ---
// All tests use mock functions — no network, no K8s API, fully deterministic.

func TestReconcileGitSync_NoSyncContexts(t *testing.T) {
	r := newTestReconciler(nil, 0)
	agent := newAgent("test-agent", nil, nil)
	mounts := []gitMount{noSyncMount("repo", "https://github.com/org/repo.git")}

	annotations, requeue, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations != nil {
		t.Errorf("expected nil annotations for no-sync contexts, got %v", annotations)
	}
	if requeue != 0 {
		t.Errorf("expected zero requeue, got %v", requeue)
	}
}

func TestReconcileGitSync_HotReloadOnly_ReturnsNil(t *testing.T) {
	r := newTestReconciler(nil, 0)
	agent := newAgent("test-agent", nil, nil)
	mounts := []gitMount{hotReloadMount("prompts", "https://github.com/org/prompts.git")}

	annotations, requeue, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if annotations != nil {
		t.Errorf("expected nil annotations for HotReload-only, got %v", annotations)
	}
	if requeue != 0 {
		t.Errorf("expected zero requeue, got %v", requeue)
	}
}

func TestReconcileGitSync_FirstRun_SetsHashWithoutRollout(t *testing.T) {
	remoteHash := "aaa111bbb222ccc333ddd444eee555fff666"
	r := newTestReconciler(
		mockLsRemote(map[string]string{"https://github.com/org/config.git": remoteHash}),
		0,
	)
	agent := newAgent("test-agent", nil, nil) // no existing status

	mounts := []gitMount{rolloutMount("config", "https://github.com/org/config.git", 10*time.Minute)}

	annotations, requeue, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First run: hash should be set in annotations AND status, but no rollout triggered
	expectedKey := GitHashAnnotationPrefix + "config"
	if annotations[expectedKey] != remoteHash {
		t.Errorf("expected annotation %s=%s, got %s", expectedKey, remoteHash, annotations[expectedKey])
	}
	if getStatusCommitHash(agent, "config") != remoteHash {
		t.Errorf("expected status hash to be set on first run, got %q", getStatusCommitHash(agent, "config"))
	}
	if requeue != 10*time.Minute {
		t.Errorf("expected requeue 10m, got %v", requeue)
	}
}

func TestReconcileGitSync_NoChange_KeepsCurrentHash(t *testing.T) {
	hash := "aaa111bbb222ccc333ddd444eee555fff666"
	r := newTestReconciler(
		mockLsRemote(map[string]string{"https://github.com/org/config.git": hash}),
		0,
	)
	agent := newAgent("test-agent", []kubeopenv1alpha1.GitSyncStatus{
		{Name: "config", CommitHash: hash},
	}, nil)

	mounts := []gitMount{rolloutMount("config", "https://github.com/org/config.git", 5*time.Minute)}

	annotations, _, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedKey := GitHashAnnotationPrefix + "config"
	if annotations[expectedKey] != hash {
		t.Errorf("expected annotation to keep current hash, got %q", annotations[expectedKey])
	}
	// No GitSyncPending condition should be set
	cond := meta.FindStatusCondition(agent.Status.Conditions, AgentConditionGitSyncPending)
	if cond != nil && cond.Status == metav1.ConditionTrue {
		t.Error("should not set GitSyncPending when no change detected")
	}
}

func TestReconcileGitSync_ChangeDetected_NoActiveTasks_Rollout(t *testing.T) {
	oldHash := "aaa111bbb222ccc333ddd444eee555fff666"
	newHash := "bbb222ccc333ddd444eee555fff666aaa111"
	r := newTestReconciler(
		mockLsRemote(map[string]string{"https://github.com/org/config.git": newHash}),
		0, // no active tasks
	)
	agent := newAgent("test-agent", []kubeopenv1alpha1.GitSyncStatus{
		{Name: "config", CommitHash: oldHash},
	}, nil)

	mounts := []gitMount{rolloutMount("config", "https://github.com/org/config.git", 5*time.Minute)}

	annotations, _, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// New hash should be in annotations (triggers rollout)
	expectedKey := GitHashAnnotationPrefix + "config"
	if annotations[expectedKey] != newHash {
		t.Errorf("expected new hash in annotations, got %q", annotations[expectedKey])
	}

	// Status should be updated to new hash
	if getStatusCommitHash(agent, "config") != newHash {
		t.Errorf("expected status hash updated to new hash, got %q", getStatusCommitHash(agent, "config"))
	}

	// GitSyncPending should be cleared
	cond := meta.FindStatusCondition(agent.Status.Conditions, AgentConditionGitSyncPending)
	if cond != nil && cond.Status == metav1.ConditionTrue {
		t.Error("GitSyncPending should be cleared after successful rollout")
	}
}

func TestReconcileGitSync_ChangeDetected_ActiveTasks_DelaysRollout(t *testing.T) {
	oldHash := "aaa111bbb222ccc333ddd444eee555fff666"
	newHash := "bbb222ccc333ddd444eee555fff666aaa111"
	r := newTestReconciler(
		mockLsRemote(map[string]string{"https://github.com/org/config.git": newHash}),
		3, // 3 active tasks
	)
	agent := newAgent("test-agent", []kubeopenv1alpha1.GitSyncStatus{
		{Name: "config", CommitHash: oldHash},
	}, nil)

	mounts := []gitMount{rolloutMount("config", "https://github.com/org/config.git", 10*time.Minute)}

	annotations, requeue, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Annotations should be REVERTED to old hash (no rollout)
	expectedKey := GitHashAnnotationPrefix + "config"
	if annotations[expectedKey] != oldHash {
		t.Errorf("expected old hash in annotations (rollout delayed), got %q", annotations[expectedKey])
	}

	// Status hash should NOT be updated (so change is re-detected next reconcile)
	if getStatusCommitHash(agent, "config") != oldHash {
		t.Errorf("expected status hash to remain old (for re-detection), got %q", getStatusCommitHash(agent, "config"))
	}

	// GitSyncPending should be set
	cond := meta.FindStatusCondition(agent.Status.Conditions, AgentConditionGitSyncPending)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatal("expected GitSyncPending condition to be True")
	}
	if cond.Reason != "WaitingForTasks" {
		t.Errorf("expected reason WaitingForTasks, got %q", cond.Reason)
	}

	// Requeue should be the shorter of sync interval and pending recheck
	if requeue != GitSyncPendingRecheck {
		t.Errorf("expected requeue %v (pending recheck), got %v", GitSyncPendingRecheck, requeue)
	}
}

func TestReconcileGitSync_DelayedRollout_TasksComplete_Proceeds(t *testing.T) {
	oldHash := "aaa111bbb222ccc333ddd444eee555fff666"
	newHash := "bbb222ccc333ddd444eee555fff666aaa111"
	r := newTestReconciler(
		mockLsRemote(map[string]string{"https://github.com/org/config.git": newHash}),
		0, // tasks completed
	)

	// Agent still has old hash (status wasn't updated during delay)
	agent := newAgent("test-agent", []kubeopenv1alpha1.GitSyncStatus{
		{Name: "config", CommitHash: oldHash},
	}, []metav1.Condition{
		{
			Type:               AgentConditionGitSyncPending,
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForTasks",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		},
	})

	mounts := []gitMount{rolloutMount("config", "https://github.com/org/config.git", 5*time.Minute)}

	annotations, _, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now rollout should proceed
	expectedKey := GitHashAnnotationPrefix + "config"
	if annotations[expectedKey] != newHash {
		t.Errorf("expected new hash in annotations after tasks complete, got %q", annotations[expectedKey])
	}
	if getStatusCommitHash(agent, "config") != newHash {
		t.Errorf("expected status hash updated, got %q", getStatusCommitHash(agent, "config"))
	}

	// GitSyncPending should be cleared
	cond := meta.FindStatusCondition(agent.Status.Conditions, AgentConditionGitSyncPending)
	if cond != nil && cond.Status == metav1.ConditionTrue {
		t.Error("GitSyncPending should be cleared after rollout proceeds")
	}
}

func TestReconcileGitSync_SafetyTimeout_ForcesRollout(t *testing.T) {
	oldHash := "aaa111bbb222ccc333ddd444eee555fff666"
	newHash := "bbb222ccc333ddd444eee555fff666aaa111"
	r := newTestReconciler(
		mockLsRemote(map[string]string{"https://github.com/org/config.git": newHash}),
		5, // tasks STILL active
	)

	// GitSyncPending has been True for over 1 hour
	agent := newAgent("test-agent", []kubeopenv1alpha1.GitSyncStatus{
		{Name: "config", CommitHash: oldHash},
	}, []metav1.Condition{
		{
			Type:               AgentConditionGitSyncPending,
			Status:             metav1.ConditionTrue,
			Reason:             "WaitingForTasks",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Hour)), // 2h ago
		},
	})

	mounts := []gitMount{rolloutMount("config", "https://github.com/org/config.git", 5*time.Minute)}

	annotations, _, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Safety timeout: rollout should be forced despite active tasks
	expectedKey := GitHashAnnotationPrefix + "config"
	if annotations[expectedKey] != newHash {
		t.Errorf("expected forced rollout with new hash, got %q", annotations[expectedKey])
	}
	if getStatusCommitHash(agent, "config") != newHash {
		t.Errorf("expected status hash updated on force, got %q", getStatusCommitHash(agent, "config"))
	}

	// GitSyncPending should be cleared
	cond := meta.FindStatusCondition(agent.Status.Conditions, AgentConditionGitSyncPending)
	if cond != nil && cond.Status == metav1.ConditionTrue {
		t.Error("GitSyncPending should be cleared after forced rollout")
	}
}

func TestReconcileGitSync_LsRemoteError_KeepsOldHash(t *testing.T) {
	oldHash := "aaa111bbb222ccc333ddd444eee555fff666"
	r := newTestReconciler(
		func(_ context.Context, _, _, _ string) (string, error) {
			return "", fmt.Errorf("network timeout")
		},
		0,
	)
	agent := newAgent("test-agent", []kubeopenv1alpha1.GitSyncStatus{
		{Name: "config", CommitHash: oldHash},
	}, nil)

	mounts := []gitMount{rolloutMount("config", "https://github.com/org/config.git", 5*time.Minute)}

	annotations, _, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// On error, keep old hash — no rollout
	expectedKey := GitHashAnnotationPrefix + "config"
	if annotations[expectedKey] != oldHash {
		t.Errorf("expected old hash preserved on error, got %q", annotations[expectedKey])
	}
	if getStatusCommitHash(agent, "config") != oldHash {
		t.Errorf("expected status hash unchanged, got %q", getStatusCommitHash(agent, "config"))
	}
}

func TestReconcileGitSync_MultipleContexts_ShortestInterval(t *testing.T) {
	r := newTestReconciler(
		mockLsRemote(map[string]string{
			"https://github.com/org/a.git": "hash-a",
			"https://github.com/org/b.git": "hash-b",
		}),
		0,
	)
	agent := newAgent("test-agent", []kubeopenv1alpha1.GitSyncStatus{
		{Name: "ctx-a", CommitHash: "hash-a"},
		{Name: "ctx-b", CommitHash: "hash-b"},
	}, nil)

	mounts := []gitMount{
		rolloutMount("ctx-a", "https://github.com/org/a.git", 10*time.Minute),
		rolloutMount("ctx-b", "https://github.com/org/b.git", 3*time.Minute),
	}

	_, requeue, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requeue != 3*time.Minute {
		t.Errorf("expected shortest interval 3m, got %v", requeue)
	}
}

func TestReconcileGitSync_MixedPolicies_OnlyRolloutChecked(t *testing.T) {
	callCount := 0
	r := &AgentReconciler{
		GitLsRemoteFn: func(_ context.Context, repo, _, _ string) (string, error) {
			callCount++
			return "hash-" + repo, nil
		},
		CountActiveTasksFn: func(_ context.Context, _, _ string) (int, error) {
			return 0, nil
		},
	}

	agent := newAgent("test-agent", nil, nil)
	mounts := []gitMount{
		hotReloadMount("prompts", "https://github.com/org/prompts.git"),
		rolloutMount("config", "https://github.com/org/config.git", 5*time.Minute),
		noSyncMount("docs", "https://github.com/org/docs.git"),
	}

	_, _, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the Rollout context should trigger gitLsRemote
	if callCount != 1 {
		t.Errorf("expected gitLsRemote called once (for Rollout only), got %d", callCount)
	}
}

func TestReconcileGitSync_CountActiveTasksError_ReturnsError(t *testing.T) {
	oldHash := "aaa111bbb222ccc333ddd444eee555fff666"
	newHash := "bbb222ccc333ddd444eee555fff666aaa111"
	r := &AgentReconciler{
		GitLsRemoteFn: mockLsRemote(map[string]string{"https://github.com/org/config.git": newHash}),
		CountActiveTasksFn: func(_ context.Context, _, _ string) (int, error) {
			return 0, fmt.Errorf("k8s api unavailable")
		},
	}

	agent := newAgent("test-agent", []kubeopenv1alpha1.GitSyncStatus{
		{Name: "config", CommitHash: oldHash},
	}, nil)
	mounts := []gitMount{rolloutMount("config", "https://github.com/org/config.git", 5*time.Minute)}

	_, _, err := r.reconcileGitSync(context.Background(), agent, mounts)
	if err == nil {
		t.Fatal("expected error when countActiveTasks fails")
	}
}
