// Copyright Contributors to the KubeOpenCode project

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	// pollInterval is how often to poll session status.
	pollInterval = 3 * time.Second

	// httpTimeout is the timeout for non-SSE HTTP requests.
	httpTimeout = 30 * time.Second
)

// sessionCreateResponse is the response from POST /session.
type sessionCreateResponse struct {
	ID string `json:"id"`
}

// sessionStatusMap is the response from GET /session/status.
// Maps session ID to status info.
type sessionStatusMap map[string]sessionStatusInfo

type sessionStatusInfo struct {
	Type string `json:"type"` // "idle", "busy", "retry"
}

var taskSubmitCmd = &cobra.Command{
	Use:   "task-submit",
	Short: "Submit a task to an OpenCode server and wait for completion",
	Long: `Submit a task prompt to an OpenCode server via HTTP API and wait for the
session to become idle. This command does NOT handle permission requests —
permissions must be approved via the Web UI or opencode attach TUI.

This is used internally by KubeOpenCode for Server-mode Task Pods.`,
	RunE: runTaskSubmit,
}

var (
	taskSubmitServerURL string
	taskSubmitTaskFile  string
)

func init() {
	taskSubmitCmd.Flags().StringVar(&taskSubmitServerURL, "url", "", "OpenCode server URL (e.g., http://server:4096)")
	taskSubmitCmd.Flags().StringVar(&taskSubmitTaskFile, "task-file", "", "Path to the task prompt file")
	_ = taskSubmitCmd.MarkFlagRequired("url")
	_ = taskSubmitCmd.MarkFlagRequired("task-file")
	rootCmd.AddCommand(taskSubmitCmd)
}

func runTaskSubmit(cmd *cobra.Command, args []string) error {
	// Read task prompt
	taskContent, err := os.ReadFile(taskSubmitTaskFile) //nolint:gosec // User-provided CLI flag, not untrusted input
	if err != nil {
		return fmt.Errorf("failed to read task file %s: %w", taskSubmitTaskFile, err)
	}

	prompt := strings.TrimSpace(string(taskContent))
	if prompt == "" {
		return fmt.Errorf("task file is empty: %s", taskSubmitTaskFile)
	}

	serverURL := strings.TrimRight(taskSubmitServerURL, "/")
	client := &http.Client{Timeout: httpTimeout}

	fmt.Printf("[task-submit] server: %s\n", serverURL)
	fmt.Printf("[task-submit] task file: %s (%d bytes)\n", taskSubmitTaskFile, len(prompt))

	// Step 1: Create a session
	fmt.Println("[task-submit] creating session...")
	sessionID, err := createSession(client, serverURL)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	fmt.Printf("[task-submit] session created: %s\n", sessionID)

	// Step 2: Start SSE event streaming in background for logging
	// Collect session errors from the event stream so we can report them
	done := make(chan struct{})
	streamDone := make(chan struct{})
	sessionErrors := make(chan string, 16)
	go func() {
		streamEvents(serverURL, sessionID, done, sessionErrors)
		close(streamDone)
	}()

	// Step 3: Submit the prompt
	fmt.Println("[task-submit] submitting prompt...")
	if err := submitPrompt(client, serverURL, sessionID, prompt); err != nil {
		return fmt.Errorf("failed to submit prompt: %w", err)
	}
	fmt.Println("[task-submit] prompt submitted, waiting for completion...")

	// Step 4: Poll session status until idle
	if err := waitForIdle(client, serverURL, sessionID); err != nil {
		return fmt.Errorf("session failed: %w", err)
	}

	close(done)
	// Wait for the SSE goroutine to finish so all errors are in the channel
	<-streamDone

	// Step 5: Check if any session errors occurred during execution.
	// Session errors (e.g., ProviderModelNotFoundError) do not cause the status
	// to become non-idle — the session transitions to "idle" after an error.
	// We must check the error channel to detect these failures.
	var errs []string
collectErrors:
	for {
		select {
		case e := <-sessionErrors:
			errs = append(errs, e)
		default:
			break collectErrors
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("session completed with errors: %s", strings.Join(errs, "; "))
	}

	fmt.Println("[task-submit] session completed successfully")
	return nil
}

func createSession(client *http.Client, serverURL string) (string, error) {
	// Create session with all permissions allowed.
	// Tasks are non-interactive — they run to completion.
	// For interactive sessions, users use `opencode attach` directly.
	payload := `{"permission":{"*":"allow"}}`
	resp, err := client.Post(serverURL+"/session", "application/json", bytes.NewBufferString(payload))
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result sessionCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode session response: %w", err)
	}

	if result.ID == "" {
		return "", fmt.Errorf("session ID is empty")
	}

	return result.ID, nil
}

func submitPrompt(client *http.Client, serverURL, sessionID, prompt string) error {
	// Build the prompt_async payload
	payload := map[string]interface{}{
		"parts": []map[string]interface{}{
			{
				"type": "text",
				"text": prompt,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := client.Post(
		fmt.Sprintf("%s/session/%s/prompt_async", serverURL, sessionID),
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// prompt_async returns 204 No Content on success
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func waitForIdle(client *http.Client, serverURL, sessionID string) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		<-ticker.C

		resp, err := client.Get(serverURL + "/session/status")
		if err != nil {
			fmt.Printf("[task-submit] warning: failed to poll status: %v\n", err)
			continue
		}

		var statuses sessionStatusMap
		if err := json.NewDecoder(resp.Body).Decode(&statuses); err != nil {
			_ = resp.Body.Close()
			fmt.Printf("[task-submit] warning: failed to decode status: %v\n", err)
			continue
		}
		_ = resp.Body.Close()

		status, ok := statuses[sessionID]
		if !ok {
			// Session not found in status — might be idle (default)
			fmt.Println("[task-submit] session completed (not in status map)")
			return nil
		}

		switch status.Type {
		case "idle":
			return nil
		case "busy":
			// Still running, continue polling
		case "retry":
			fmt.Println("[task-submit] session is retrying...")
		default:
			fmt.Printf("[task-submit] unknown status: %s\n", status.Type)
		}
	}
}

// streamEvents connects to the SSE endpoint and prints events for logging.
// It does NOT handle permission.asked events — those are handled by the Web UI or TUI.
// Session errors are sent to sessionErrors channel for the caller to inspect after completion.
func streamEvents(serverURL, sessionID string, done chan struct{}, sessionErrors chan<- string) {
	// Use a client with no timeout for SSE
	client := &http.Client{Timeout: 0}

	resp, err := client.Get(serverURL + "/event")
	if err != nil {
		fmt.Printf("[task-submit] warning: failed to connect to SSE: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)
	// Increase buffer for large events
	scanner.Buffer(make([]byte, 0, 256*1024), 256*1024)

	for {
		select {
		case <-done:
			return
		default:
		}

		if !scanner.Scan() {
			return
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event struct {
			Type       string          `json:"type"`
			Properties json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		// Filter events for our session and print relevant ones
		switch event.Type {
		case "message.part.delta":
			var props struct {
				SessionID string `json:"sessionID"`
				Part      struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"part"`
			}
			if json.Unmarshal(event.Properties, &props) == nil && props.SessionID == sessionID {
				if props.Part.Type == "text" {
					fmt.Print(props.Part.Text)
				}
			}

		case "permission.asked":
			var props struct {
				SessionID  string   `json:"sessionID"`
				Permission string   `json:"permission"`
				Patterns   []string `json:"patterns"`
			}
			if json.Unmarshal(event.Properties, &props) == nil && props.SessionID == sessionID {
				fmt.Printf("\n[task-submit] WAITING FOR PERMISSION: %s (%s) — approve via Web UI or opencode attach\n",
					props.Permission, strings.Join(props.Patterns, ", "))
			}

		case "session.error":
			var props struct {
				SessionID string `json:"sessionID"`
				Error     struct {
					Name    string `json:"name"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(event.Properties, &props) == nil && props.SessionID == sessionID {
				errMsg := props.Error.Name
				if props.Error.Message != "" {
					errMsg = props.Error.Message
				}
				if errMsg != "" {
					select {
					case sessionErrors <- errMsg:
					default:
					}
				}
			}

		case "session.status":
			var props struct {
				SessionID string `json:"sessionID"`
				Status    struct {
					Type string `json:"type"`
				} `json:"status"`
			}
			if json.Unmarshal(event.Properties, &props) == nil && props.SessionID == sessionID {
				if props.Status.Type == "idle" {
					return
				}
			}
		}
	}
}
