// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newSession creates an OpenCodeSession for testing.
func newSession(id, title string, created int64) OpenCodeSession {
	s := OpenCodeSession{
		ID:    id,
		Title: title,
	}
	s.Time.Created = created
	s.Time.Updated = created
	return s
}

func TestFindSessionByTitle_ReturnsLatestWhenMultipleMatch(t *testing.T) {
	sessions := []OpenCodeSession{
		newSession("ses_old", "kubeopencode/default/my-task", 1000),
		newSession("ses_new", "kubeopencode/default/my-task", 2000),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sessions)
	}))
	defer srv.Close()

	client := NewOpenCodeClient()
	session, err := client.FindSessionByTitle(context.Background(), srv.URL, "kubeopencode/default/my-task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.ID != "ses_new" {
		t.Errorf("expected latest session ses_new, got %s", session.ID)
	}
}

func TestFindSessionByTitle_ReturnsLatestRegardlessOfOrder(t *testing.T) {
	// Sessions returned in arbitrary order — should still pick latest by Created time.
	sessions := []OpenCodeSession{
		newSession("ses_newest", "kubeopencode/default/task", 3000),
		newSession("ses_oldest", "kubeopencode/default/task", 1000),
		newSession("ses_middle", "kubeopencode/default/task", 2000),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sessions)
	}))
	defer srv.Close()

	client := NewOpenCodeClient()
	session, err := client.FindSessionByTitle(context.Background(), srv.URL, "kubeopencode/default/task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.ID != "ses_newest" {
		t.Errorf("expected latest session ses_newest, got %s", session.ID)
	}
}

func TestFindSessionByTitle_NoMatch(t *testing.T) {
	sessions := []OpenCodeSession{
		newSession("ses_1", "kubeopencode/default/other-task", 1000),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sessions)
	}))
	defer srv.Close()

	client := NewOpenCodeClient()
	session, err := client.FindSessionByTitle(context.Background(), srv.URL, "kubeopencode/default/my-task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session != nil {
		t.Errorf("expected nil session for non-matching title, got %+v", session)
	}
}

func TestFindSessionByTitle_SingleMatch(t *testing.T) {
	sessions := []OpenCodeSession{
		newSession("ses_only", "kubeopencode/default/my-task/abcdef12", 1000),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sessions)
	}))
	defer srv.Close()

	client := NewOpenCodeClient()
	session, err := client.FindSessionByTitle(context.Background(), srv.URL, "kubeopencode/default/my-task/abcdef12")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session == nil {
		t.Fatal("expected session, got nil")
	}
	if session.ID != "ses_only" {
		t.Errorf("expected ses_only, got %s", session.ID)
	}
}

func TestFindSessionByTitle_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]OpenCodeSession{})
	}))
	defer srv.Close()

	client := NewOpenCodeClient()
	session, err := client.FindSessionByTitle(context.Background(), srv.URL, "kubeopencode/default/my-task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session != nil {
		t.Errorf("expected nil session for empty response, got %+v", session)
	}
}

func TestFindSessionByTitle_V2Envelope(t *testing.T) {
	// OpenCode 2.x returns { "data": [...] }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []OpenCodeSession{
				newSession("ses_v2", "kubeopencode/default/v2-task", 5000),
			},
		})
	}))
	defer srv.Close()

	client := NewOpenCodeClient()
	session, err := client.FindSessionByTitle(context.Background(), srv.URL, "kubeopencode/default/v2-task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session == nil || session.ID != "ses_v2" {
		t.Fatalf("expected ses_v2, got %+v", session)
	}
}

func TestFindSessionByTitle_LegacyFallback(t *testing.T) {
	// Server returns 404 on /api/session but 200 on /session (1.x server)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/session" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]OpenCodeSession{
				newSession("ses_v1", "kubeopencode/default/v1-task", 3000),
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewOpenCodeClient()
	session, err := client.FindSessionByTitle(context.Background(), srv.URL, "kubeopencode/default/v1-task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session == nil || session.ID != "ses_v1" {
		t.Fatalf("expected ses_v1 via fallback, got %+v", session)
	}
}

func TestGetSession_V2AndLegacy(t *testing.T) {
	t.Run("OpenCode 2.x envelope format", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/session/ses_123" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": newSession("ses_123", "task-title", 1000),
			})
		}))
		defer srv.Close()

		client := NewOpenCodeClient()
		session, err := client.GetSession(context.Background(), srv.URL, "ses_123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session.ID != "ses_123" {
			t.Errorf("expected ses_123, got %s", session.ID)
		}
	})

	t.Run("OpenCode 1.x legacy fallback", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/session/ses_123" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(newSession("ses_123", "task-title", 1000))
				return
			}
			http.NotFound(w, r)
		}))
		defer srv.Close()

		client := NewOpenCodeClient()
		session, err := client.GetSession(context.Background(), srv.URL, "ses_123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session.ID != "ses_123" {
			t.Errorf("expected ses_123, got %s", session.ID)
		}
	})
}

func TestGetSessionMessages_V2AndLegacy(t *testing.T) {
	msg := OpenCodeMessage{
		Info: OpenCodeMessageInfo{
			ID:   "msg_1",
			Role: "assistant",
			Cost: 0.05,
		},
	}
	msg.Info.Tokens.Input = 100
	msg.Info.Tokens.Output = 50

	t.Run("OpenCode 2.x envelope format", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/session/ses_1/message" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []OpenCodeMessage{msg},
			})
		}))
		defer srv.Close()

		client := NewOpenCodeClient()
		messages, err := client.GetSessionMessages(context.Background(), srv.URL, "ses_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(messages) != 1 || messages[0].Info.ID != "msg_1" {
			t.Fatalf("expected 1 message msg_1, got %+v", messages)
		}
	})

	t.Run("AggregateMessageStats", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []OpenCodeMessage{msg},
			})
		}))
		defer srv.Close()

		client := NewOpenCodeClient()
		stats, count, err := client.AggregateMessageStats(context.Background(), srv.URL, "ses_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 1 {
			t.Errorf("expected count 1, got %d", count)
		}
		if stats.Cost != 0.05 || stats.Tokens.Input != 100 || stats.Tokens.Output != 50 {
			t.Errorf("unexpected stats: %+v", stats)
		}
	})
}

func TestGetSessionDiff_V2(t *testing.T) {
	diffs := []OpenCodeFileDiff{
		{Path: "main.go", Additions: 10, Deletions: 2},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session/ses_1/diff" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": diffs,
		})
	}))
	defer srv.Close()

	client := NewOpenCodeClient()
	got, err := client.GetSessionDiff(context.Background(), srv.URL, "ses_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Path != "main.go" || got[0].Additions != 10 {
		t.Errorf("expected diff for main.go, got %+v", got)
	}
}
