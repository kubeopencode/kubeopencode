// Copyright Contributors to the KubeOpenCode project

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutUnlessStreaming(t *testing.T) {
	tests := []struct {
		name         string
		header       string
		value        string
		wantDeadline bool
	}{
		{name: "plain request", wantDeadline: true},
		{name: "JSON request", header: "Accept", value: "application/json", wantDeadline: true},
		{name: "server-sent events", header: "Accept", value: "text/event-stream", wantDeadline: false},
		{name: "websocket upgrade", header: "Upgrade", value: "websocket", wantDeadline: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hasDeadline bool
			handler := timeoutUnlessStreaming(time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, hasDeadline = r.Context().Deadline()
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/default/tasks/demo/logs", nil)
			if tt.header != "" {
				req.Header.Set(tt.header, tt.value)
			}
			handler.ServeHTTP(httptest.NewRecorder(), req)

			if hasDeadline != tt.wantDeadline {
				t.Errorf("request deadline set = %v, want %v", hasDeadline, tt.wantDeadline)
			}
		})
	}
}

func TestTimeoutUnlessStreaming_StreamOutlivesTimeout(t *testing.T) {
	var streamErr error
	handler := timeoutUnlessStreaming(10 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		streamErr = r.Context().Err()
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/default/tasks/demo/logs", nil)
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if streamErr != nil {
		t.Errorf("stream context ended after the timeout: %v", streamErr)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
