// Copyright Contributors to the KubeOpenCode project

package handlers

import (
	"net/http"
	"net/url"
	"testing"
)

func TestParseFilterOptions(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{RawQuery: ""}}
		opts, err := ParseFilterOptions(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Limit != 20 {
			t.Errorf("default limit = %d, want 20", opts.Limit)
		}
		if opts.Offset != 0 {
			t.Errorf("default offset = %d, want 0", opts.Offset)
		}
		if opts.SortOrder != "desc" {
			t.Errorf("default sortOrder = %q, want desc", opts.SortOrder)
		}
		if opts.Name != "" {
			t.Errorf("default name = %q, want empty", opts.Name)
		}
		if opts.Phase != "" {
			t.Errorf("default phase = %q, want empty", opts.Phase)
		}
	})

	t.Run("all params", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{
			RawQuery: "name=foo&phase=Running&limit=50&offset=10&sortOrder=asc",
		}}
		opts, err := ParseFilterOptions(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Name != "foo" {
			t.Errorf("name = %q, want foo", opts.Name)
		}
		if opts.Phase != "Running" {
			t.Errorf("phase = %q, want Running", opts.Phase)
		}
		if opts.Limit != 50 {
			t.Errorf("limit = %d, want 50", opts.Limit)
		}
		if opts.Offset != 10 {
			t.Errorf("offset = %d, want 10", opts.Offset)
		}
		if opts.SortOrder != "asc" {
			t.Errorf("sortOrder = %q, want asc", opts.SortOrder)
		}
	})

	t.Run("limit capped at 100", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{RawQuery: "limit=500"}}
		opts, err := ParseFilterOptions(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Limit != 100 {
			t.Errorf("limit = %d, want 100 (capped)", opts.Limit)
		}
	})

	t.Run("invalid limit uses default", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{RawQuery: "limit=abc"}}
		opts, err := ParseFilterOptions(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Limit != 20 {
			t.Errorf("limit = %d, want 20 (default for invalid)", opts.Limit)
		}
	})

	t.Run("negative limit uses default", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{RawQuery: "limit=-5"}}
		opts, err := ParseFilterOptions(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Limit != 20 {
			t.Errorf("limit = %d, want 20 (default for negative)", opts.Limit)
		}
	})

	t.Run("negative offset uses default", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{RawQuery: "offset=-1"}}
		opts, err := ParseFilterOptions(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.Offset != 0 {
			t.Errorf("offset = %d, want 0 (default for negative)", opts.Offset)
		}
	})

	t.Run("invalid sortOrder uses default", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{RawQuery: "sortOrder=invalid"}}
		opts, err := ParseFilterOptions(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.SortOrder != "desc" {
			t.Errorf("sortOrder = %q, want desc (default for invalid)", opts.SortOrder)
		}
	})

	t.Run("valid labelSelector", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{RawQuery: "labelSelector=app%3Dtest"}}
		opts, err := ParseFilterOptions(r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if opts.LabelSelector == nil {
			t.Fatal("expected non-nil LabelSelector")
		}
	})

	t.Run("invalid labelSelector returns error", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{RawQuery: "labelSelector=%21%21invalid"}}
		_, err := ParseFilterOptions(r)
		if err == nil {
			t.Fatal("expected error for invalid labelSelector")
		}
	})
}

func TestBuildListOptions(t *testing.T) {
	t.Run("with namespace", func(t *testing.T) {
		listOpts := BuildListOptions("my-ns", nil)
		if len(listOpts) != 1 {
			t.Errorf("expected 1 list option, got %d", len(listOpts))
		}
	})

	t.Run("empty namespace", func(t *testing.T) {
		listOpts := BuildListOptions("", nil)
		if len(listOpts) != 0 {
			t.Errorf("expected 0 list options for empty namespace, got %d", len(listOpts))
		}
	})

	t.Run("with namespace and label selector", func(t *testing.T) {
		r := &http.Request{URL: &url.URL{RawQuery: "labelSelector=app%3Dtest"}}
		opts, _ := ParseFilterOptions(r)
		listOpts := BuildListOptions("my-ns", opts)
		if len(listOpts) != 2 {
			t.Errorf("expected 2 list options (namespace + labels), got %d", len(listOpts))
		}
	})

	t.Run("nil opts with namespace", func(t *testing.T) {
		listOpts := BuildListOptions("ns", nil)
		if len(listOpts) != 1 {
			t.Errorf("expected 1 list option, got %d", len(listOpts))
		}
	})
}

func TestMatchesNameFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		want   bool
	}{
		{"test-agent", "", true},      // empty filter matches all
		{"test-agent", "test", true},  // substring match
		{"test-agent", "AGENT", true}, // case insensitive
		{"test-agent", "prod", false}, // no match
		{"my-task-123", "task", true}, // middle substring
		{"UPPERCASE", "upper", true},  // case insensitive both ways
		{"", "anything", false},       // empty name
		{"", "", true},                // both empty
	}

	for _, tt := range tests {
		t.Run(tt.name+"_"+tt.filter, func(t *testing.T) {
			got := MatchesNameFilter(tt.name, tt.filter)
			if got != tt.want {
				t.Errorf("MatchesNameFilter(%q, %q) = %v, want %v", tt.name, tt.filter, got, tt.want)
			}
		})
	}
}

func TestMatchesPhaseFilter(t *testing.T) {
	tests := []struct {
		phase  string
		filter string
		want   bool
	}{
		{"Running", "", true},                  // empty filter matches all
		{"Running", "Running", true},           // exact match
		{"running", "Running", true},           // case insensitive
		{"Running", "running", true},           // case insensitive other way
		{"Running", "Failed", false},           // no match
		{"Running", "Running,Failed", true},    // comma-separated first
		{"Failed", "Running,Failed", true},     // comma-separated second
		{"Completed", "Running,Failed", false}, // comma-separated no match
		{"Running", " Running ", true},         // trimmed whitespace
		{"Failed", "Running, Failed", true},    // comma with space
	}

	for _, tt := range tests {
		t.Run(tt.phase+"_"+tt.filter, func(t *testing.T) {
			got := MatchesPhaseFilter(tt.phase, tt.filter)
			if got != tt.want {
				t.Errorf("MatchesPhaseFilter(%q, %q) = %v, want %v", tt.phase, tt.filter, got, tt.want)
			}
		})
	}
}
