// Copyright Contributors to the KubeOpenCode project

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sessionStatusPayload builds a /session/status body. OpenCode reports a map of
// session ID to state, where "busy" means a turn is generating.
func sessionStatusPayload(types ...string) string {
	statuses := make(map[string]map[string]string, len(types))
	for i, t := range types {
		statuses["ses_"+string(rune('a'+i))] = map[string]string{"type": t}
	}
	body, _ := json.Marshal(statuses)
	return string(body)
}

func TestAnySessionBusy(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		body        string
		wantBusy    bool
		wantErr     bool
		description string
	}{
		{
			name:       "idle server reports no sessions",
			statusCode: http.StatusOK,
			body:       "{}",
			wantBusy:   false,
		},
		{
			name:       "busy session is detected",
			statusCode: http.StatusOK,
			body:       sessionStatusPayload("busy"),
			wantBusy:   true,
		},
		{
			name:       "idle session is not busy",
			statusCode: http.StatusOK,
			body:       sessionStatusPayload("idle"),
			wantBusy:   false,
		},
		{
			name:       "one busy among several is busy",
			statusCode: http.StatusOK,
			body:       sessionStatusPayload("idle", "busy", "idle"),
			wantBusy:   true,
		},
		{
			name:       "non-200 is an error",
			statusCode: http.StatusInternalServerError,
			body:       "",
			wantErr:    true,
		},
		{
			// Fail safe: an unrecognized payload must not be read as idle, or a
			// server API change could cause reloads during active work.
			name:        "malformed payload is treated as busy",
			statusCode:  http.StatusOK,
			body:        "not json",
			wantBusy:    true,
			wantErr:     true,
			description: "an unparseable response must fail closed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != sessionStatusPath {
					t.Errorf("expected path %q, got %q", sessionStatusPath, r.URL.Path)
				}
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			busy, err := anySessionBusy(srv.Client(), srv.URL)
			if tc.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if busy != tc.wantBusy {
				t.Errorf("busy = %v, want %v", busy, tc.wantBusy)
			}
		})
	}
}

func TestReloadOpenCodeServer(t *testing.T) {
	t.Run("disposes when idle", func(t *testing.T) {
		var disposed bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case sessionStatusPath:
				_, _ = w.Write([]byte("{}"))
			case instanceDisposePath:
				if r.Method != http.MethodPost {
					t.Errorf("dispose must use POST, got %s", r.Method)
				}
				disposed = true
				w.WriteHeader(http.StatusOK)
			default:
				t.Errorf("unexpected path %q", r.URL.Path)
			}
		}))
		defer srv.Close()

		if err := reloadOpenCodeServer(srv.URL); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !disposed {
			t.Error("expected the server to be disposed when idle")
		}
	})

	t.Run("does not dispose while busy", func(t *testing.T) {
		var disposed bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case sessionStatusPath:
				_, _ = w.Write([]byte(sessionStatusPayload("busy")))
			case instanceDisposePath:
				disposed = true
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer srv.Close()

		err := reloadOpenCodeServer(srv.URL)
		if err == nil {
			t.Fatal("expected an error while a session is busy")
		}
		if disposed {
			t.Error("must not dispose the instance while a session is busy")
		}
	})

	t.Run("propagates disposal failure", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == instanceDisposePath {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte("{}"))
		}))
		defer srv.Close()

		if err := reloadOpenCodeServer(srv.URL); err == nil {
			t.Fatal("expected an error for a non-2xx dispose response")
		}
	})

	t.Run("errors when the status endpoint is unreachable", func(t *testing.T) {
		// Nothing is listening: the caller retries next cycle instead of
		// reloading blindly.
		if err := reloadOpenCodeServer("http://127.0.0.1:1"); err == nil {
			t.Fatal("expected an error when the server is unreachable")
		}
	})
}

func TestReadWriteReloadState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")

	if _, ok := readReloadState(path); ok {
		t.Error("expected no state for a missing file")
	}

	if err := writeReloadState(path, "abc123"); err != nil {
		t.Fatalf("failed to write state: %v", err)
	}
	got, ok := readReloadState(path)
	if !ok {
		t.Fatal("expected state to be present after writing")
	}
	if got != "abc123" {
		t.Errorf("state = %q, want %q", got, "abc123")
	}

	// The file is written world-writable because a restarted sidecar may run as
	// a different random UID in SCC environments.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat state file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o666 {
		t.Errorf("state file mode = %o, want 666 so a restart under a new UID can write it", perm)
	}
}

func TestReadReloadStateIgnoresEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(path, []byte("\n"), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, ok := readReloadState(path); ok {
		t.Error("expected an empty file to be treated as no state")
	}
}

// TestReloadEnvVarName guards the contract between the controller, which sets
// OPENCODE_RELOAD_URL on the sidecar, and this command, which reads it.
func TestReloadEnvVarName(t *testing.T) {
	if envReloadURL != "OPENCODE_RELOAD_URL" {
		t.Errorf("envReloadURL = %q, want OPENCODE_RELOAD_URL", envReloadURL)
	}
	if !strings.HasPrefix(sessionStatusPath, "/") || !strings.HasPrefix(instanceDisposePath, "/") {
		t.Error("OpenCode API paths must be absolute")
	}
}
