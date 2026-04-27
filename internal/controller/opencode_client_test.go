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
