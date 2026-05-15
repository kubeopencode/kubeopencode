// Copyright Contributors to the KubeOpenCode project

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamEvents_SessionError(t *testing.T) {
	sessionID := "test-session-123"

	tests := []struct {
		name       string
		events     []string
		wantErrors []string
	}{
		{
			name: "collects session error with message",
			events: []string{
				sseEvent("session.error", map[string]interface{}{
					"sessionID": sessionID,
					"error": map[string]string{
						"name":    "ProviderModelNotFoundError",
						"message": "Model deepseek-v4-flash not found",
					},
				}),
			},
			wantErrors: []string{"Model deepseek-v4-flash not found"},
		},
		{
			name: "collects session error with name only when message is empty",
			events: []string{
				sseEvent("session.error", map[string]interface{}{
					"sessionID": sessionID,
					"error": map[string]string{
						"name":    "ProviderModelNotFoundError",
						"message": "",
					},
				}),
			},
			wantErrors: []string{"ProviderModelNotFoundError"},
		},
		{
			name: "collects multiple session errors",
			events: []string{
				sseEvent("session.error", map[string]interface{}{
					"sessionID": sessionID,
					"error": map[string]string{
						"name":    "Error1",
						"message": "First error",
					},
				}),
				sseEvent("session.error", map[string]interface{}{
					"sessionID": sessionID,
					"error": map[string]string{
						"name":    "Error2",
						"message": "Second error",
					},
				}),
			},
			wantErrors: []string{"First error", "Second error"},
		},
		{
			name: "ignores error for different session",
			events: []string{
				sseEvent("session.error", map[string]interface{}{
					"sessionID": "other-session",
					"error": map[string]string{
						"name":    "SomeError",
						"message": "Not our session",
					},
				}),
			},
			wantErrors: nil,
		},
		{
			name: "ignores empty error name and message",
			events: []string{
				sseEvent("session.error", map[string]interface{}{
					"sessionID": sessionID,
					"error": map[string]string{
						"name":    "",
						"message": "",
					},
				}),
			},
			wantErrors: nil,
		},
		{
			name: "prefers message over name when both present",
			events: []string{
				sseEvent("session.error", map[string]interface{}{
					"sessionID": sessionID,
					"error": map[string]string{
						"name":    "ProviderModelNotFoundError",
						"message": "Model not found",
					},
				}),
			},
			wantErrors: []string{"Model not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				flusher, ok := w.(http.Flusher)
				if !ok {
					t.Fatal("response writer must support flushing")
				}

				for _, event := range tt.events {
					writeSSE(w, flusher, event)
				}

				writeSSE(w, flusher, fmt.Sprintf("data: {\"type\":\"session.status\",\"properties\":{\"sessionID\":\"%s\",\"status\":{\"type\":\"idle\"}}}\n\n", sessionID))
			}))
			defer server.Close()

			done := make(chan struct{})
			sessionErrors := make(chan string, 16)

			go streamEvents(server.URL, sessionID, done, sessionErrors)

			var gotErrors []string
			timeout := time.After(5 * time.Second)
		collect:
			for {
				select {
				case e := <-sessionErrors:
					gotErrors = append(gotErrors, e)
				case <-timeout:
					break collect
				default:
					if len(gotErrors) >= len(tt.wantErrors) {
						break collect
					}
				}
			}

			if len(gotErrors) != len(tt.wantErrors) {
				t.Fatalf("expected %d errors, got %d: %v", len(tt.wantErrors), len(gotErrors), gotErrors)
			}
			for i, want := range tt.wantErrors {
				if gotErrors[i] != want {
					t.Errorf("error[%d]: expected %q, got %q", i, want, gotErrors[i])
				}
			}

			close(done)
		})
	}
}

func TestStreamEvents_DoneChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		for i := 0; i < 100; i++ {
			io.WriteString(w, "data: {\"type\":\"message.part.delta\",\"properties\":{\"sessionID\":\"s1\",\"part\":{\"type\":\"text\",\"text\":\"ping\"}}}\n\n")
			flusher.Flush()
		}
	}))
	defer server.Close()

	done := make(chan struct{})
	sessionErrors := make(chan string, 16)

	doneCalled := make(chan struct{})
	go func() {
		streamEvents(server.URL, "s1", done, sessionErrors)
		close(doneCalled)
	}()

	close(done)

	select {
	case <-doneCalled:
	case <-time.After(3 * time.Second):
		t.Fatal("streamEvents did not return after done channel was closed")
	}
}

func TestStreamEvents_ConnectionFailure(t *testing.T) {
	done := make(chan struct{})
	sessionErrors := make(chan string, 16)

	defer close(done)

	streamEvents("http://127.0.0.1:0/event", "s1", done, sessionErrors)

	select {
	case e := <-sessionErrors:
		t.Errorf("expected no errors on connection failure, got: %s", e)
	default:
	}
}

func TestStreamEvents_PermissionAskedEvent(t *testing.T) {
	sessionID := "test-session"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		writeSSE(w, flusher, sseEvent("permission.asked", map[string]interface{}{
			"sessionID":  sessionID,
			"permission": "file-write",
			"patterns":  []string{"/app/main.go"},
		}))

		writeSSE(w, flusher, fmt.Sprintf("data: {\"type\":\"session.status\",\"properties\":{\"sessionID\":\"%s\",\"status\":{\"type\":\"idle\"}}}\n\n", sessionID))
	}))
	defer server.Close()

	done := make(chan struct{})
	sessionErrors := make(chan string, 16)

	go streamEvents(server.URL, sessionID, done, sessionErrors)

	select {
	case <-sessionErrors:
		t.Error("permission.asked events should not produce errors")
	case <-time.After(2 * time.Second):
	}

	close(done)
}

func TestStreamErrorsChannelFull(t *testing.T) {
	sessionID := "test-session"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		for i := 0; i < 20; i++ {
			writeSSE(w, flusher, sseEvent("session.error", map[string]interface{}{
				"sessionID": sessionID,
				"error": map[string]string{
					"name":    fmt.Sprintf("Error%d", i),
					"message": fmt.Sprintf("Error message %d", i),
				},
			}))
		}

		writeSSE(w, flusher, fmt.Sprintf("data: {\"type\":\"session.status\",\"properties\":{\"sessionID\":\"%s\",\"status\":{\"type\":\"idle\"}}}\n\n", sessionID))
	}))
	defer server.Close()

	done := make(chan struct{})
	sessionErrors := make(chan string, 16)

	go streamEvents(server.URL, sessionID, done, sessionErrors)

	var collected []string
	timeout := time.After(5 * time.Second)
	waitMore := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case e := <-sessionErrors:
			collected = append(collected, e)
		case <-waitMore:
			break loop
		case <-timeout:
			break loop
		}
	}

	if len(collected) != 20 {
		t.Errorf("expected 20 errors, got %d: %v", len(collected), collected)
	}

	close(done)
}

func TestStreamEvents_InvalidJSON(t *testing.T) {
	sessionID := "test-session"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		io.WriteString(w, "data: {invalid json}\n\n")
		flusher.Flush()

		writeSSE(w, flusher, sseEvent("session.error", map[string]interface{}{
			"sessionID": sessionID,
			"error": map[string]string{
				"name":    "RealError",
				"message": "This should still work",
			},
		}))

		writeSSE(w, flusher, fmt.Sprintf("data: {\"type\":\"session.status\",\"properties\":{\"sessionID\":\"%s\",\"status\":{\"type\":\"idle\"}}}\n\n", sessionID))
	}))
	defer server.Close()

	done := make(chan struct{})
	sessionErrors := make(chan string, 16)

	go streamEvents(server.URL, sessionID, done, sessionErrors)

	var gotErrors []string
	timeout := time.After(5 * time.Second)
	waitMore := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case e := <-sessionErrors:
			gotErrors = append(gotErrors, e)
		case <-waitMore:
			break loop
		case <-timeout:
			break loop
		}
	}

	if len(gotErrors) != 1 {
		t.Fatalf("expected exactly 1 error (invalid JSON should be skipped), got %d: %v", len(gotErrors), gotErrors)
	}
	if gotErrors[0] != "This should still work" {
		t.Errorf("expected error message 'This should still work', got %q", gotErrors[0])
	}

	close(done)
}

func TestStreamEvents_SessionStatusIdle(t *testing.T) {
	sessionID := "test-session"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		writeSSE(w, flusher, sseEvent("session.error", map[string]interface{}{
			"sessionID": sessionID,
			"error": map[string]string{
				"name":    "TestError",
				"message": "Error before idle",
			},
		}))

		writeSSE(w, flusher, fmt.Sprintf("data: {\"type\":\"session.status\",\"properties\":{\"sessionID\":\"%s\",\"status\":{\"type\":\"idle\"}}}\n\n", sessionID))
	}))
	defer server.Close()

	done := make(chan struct{})
	sessionErrors := make(chan string, 16)

	streamFinished := make(chan struct{})
	go func() {
		streamEvents(server.URL, sessionID, done, sessionErrors)
		close(streamFinished)
	}()

	select {
	case <-streamFinished:
	case <-time.After(5 * time.Second):
		t.Fatal("streamEvents did not return after receiving session.status idle")
	}

	var gotErrors []string
	select {
	case e := <-sessionErrors:
		gotErrors = append(gotErrors, e)
	default:
	}

	if len(gotErrors) != 1 || gotErrors[0] != "Error before idle" {
		t.Errorf("expected 'Error before idle', got %v", gotErrors)
	}

	close(done)
}

func TestStreamEvents_NonDataLinesSkipped(t *testing.T) {
	sessionID := "test-session"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		io.WriteString(w, ": this is a comment\n")
		io.WriteString(w, "event: custom\n")
		io.WriteString(w, "id: 123\n")
		io.WriteString(w, "retry: 5000\n\n")
		flusher.Flush()

		writeSSE(w, flusher, sseEvent("session.error", map[string]interface{}{
			"sessionID": sessionID,
			"error": map[string]string{
				"name":    "AfterComments",
				"message": "Error after non-data lines",
			},
		}))

		writeSSE(w, flusher, fmt.Sprintf("data: {\"type\":\"session.status\",\"properties\":{\"sessionID\":\"%s\",\"status\":{\"type\":\"idle\"}}}\n\n", sessionID))
	}))
	defer server.Close()

	done := make(chan struct{})
	sessionErrors := make(chan string, 16)

	streamFinished := make(chan struct{})
	go func() {
		streamEvents(server.URL, sessionID, done, sessionErrors)
		close(streamFinished)
	}()

	select {
	case <-streamFinished:
	case <-time.After(5 * time.Second):
		t.Fatal("streamEvents did not return")
	}

	var gotErrors []string
	select {
	case e := <-sessionErrors:
		gotErrors = append(gotErrors, e)
	default:
	}

	if len(gotErrors) != 1 || !strings.Contains(gotErrors[0], "Error after non-data lines") {
		t.Errorf("expected error after non-data lines, got %v", gotErrors)
	}

	close(done)
}



func sseEvent(eventType string, properties interface{}) string {
	propsJSON, _ := json.Marshal(properties)
	data, _ := json.Marshal(map[string]interface{}{
		"type":       eventType,
		"properties": json.RawMessage(propsJSON),
	})
	return "data: " + string(data)
}

func writeSSE(w io.Writer, flusher http.Flusher, data string) {
	io.WriteString(w, data+"\n")
	flusher.Flush()
}