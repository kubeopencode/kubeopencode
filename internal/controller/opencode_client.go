// Copyright Contributors to the KubeOpenCode project

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// OpenCodeClient provides an HTTP client for interacting with the OpenCode server API.
// It is used by the Task controller to query session information from an Agent's
// OpenCode server instance.
type OpenCodeClient struct {
	httpClient *http.Client
}

// NewOpenCodeClient creates a new OpenCode API client with sensible defaults.
func NewOpenCodeClient() *OpenCodeClient {
	return &OpenCodeClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// OpenCodeSession represents a session returned by the OpenCode API.
// Only the fields relevant to KubeOpenCode are included.
type OpenCodeSession struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Slug    string `json:"slug"`
	Summary *struct {
		Additions int32 `json:"additions"`
		Deletions int32 `json:"deletions"`
		Files     int32 `json:"files"`
	} `json:"summary,omitempty"`
	Time struct {
		Created  int64  `json:"created"`
		Updated  int64  `json:"updated"`
		Archived *int64 `json:"archived,omitempty"`
	} `json:"time"`
}

// OpenCodeMessage represents a message returned by the OpenCode API.
// The actual API structure wraps role, cost, and tokens inside an "info" object,
// with message content in a separate "parts" array.
type OpenCodeMessage struct {
	Info  OpenCodeMessageInfo `json:"info"`
	Parts json.RawMessage     `json:"parts"`
}

// OpenCodeMessageInfo contains metadata from a message's info field.
type OpenCodeMessageInfo struct {
	ID     string  `json:"id"`
	Role   string  `json:"role"` // "user" or "assistant"
	Cost   float64 `json:"cost"`
	Tokens struct {
		Input     int64              `json:"input"`
		Output    int64              `json:"output"`
		Reasoning int64              `json:"reasoning"`
		Cache     OpenCodeCacheUsage `json:"cache"`
	} `json:"tokens"`
}

// OpenCodeCacheUsage represents the cache token usage (read/write).
type OpenCodeCacheUsage struct {
	Read  int64 `json:"read"`
	Write int64 `json:"write"`
}

// AssistantMessageData is the aggregated stats output used by resolveSessionInfo.
type AssistantMessageData struct {
	Cost   float64
	Tokens struct {
		Input     int64
		Output    int64
		Reasoning int64
		Cache     int64
	}
}

// Note: OpenCode API returns sessions as a plain array (not wrapped in {data: [...]}).
// Messages are also returned as a plain array.

// FindSessionByTitle searches for a session by title on the given OpenCode server.
// Returns nil if no matching session is found (not an error).
func (c *OpenCodeClient) FindSessionByTitle(ctx context.Context, serverURL, title string) (*OpenCodeSession, error) {
	reqURL := fmt.Sprintf("%s/session?search=%s", serverURL, url.QueryEscape(title))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var sessions []OpenCodeSession
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Find exact title match. When multiple sessions share the same title
	// (e.g., Task deleted and recreated with the same name), return the
	// most recently created session.
	var latest *OpenCodeSession
	for i := range sessions {
		if sessions[i].Title == title {
			if latest == nil || sessions[i].Time.Created > latest.Time.Created {
				latest = &sessions[i]
			}
		}
	}

	return latest, nil
}

// GetSession retrieves a session by ID from the OpenCode server.
func (c *OpenCodeClient) GetSession(ctx context.Context, serverURL, sessionID string) (*OpenCodeSession, error) {
	reqURL := fmt.Sprintf("%s/session/%s", serverURL, url.PathEscape(sessionID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var session OpenCodeSession
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &session, nil
}

// GetSessionMessages retrieves all messages for a session.
// Used to aggregate token usage and cost information.
func (c *OpenCodeClient) GetSessionMessages(ctx context.Context, serverURL, sessionID string) ([]OpenCodeMessage, error) {
	reqURL := fmt.Sprintf("%s/session/%s/message", serverURL, url.PathEscape(sessionID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getting messages: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var messages []OpenCodeMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return messages, nil
}

// AggregateMessageStats fetches messages for a session and aggregates
// token usage, cost, and message count from assistant messages.
func (c *OpenCodeClient) AggregateMessageStats(ctx context.Context, serverURL, sessionID string) (*AssistantMessageData, int32, error) {
	messages, err := c.GetSessionMessages(ctx, serverURL, sessionID)
	if err != nil {
		return nil, 0, fmt.Errorf("getting messages: %w", err)
	}

	var aggregated AssistantMessageData
	var messageCount int32

	for _, msg := range messages {
		messageCount++
		if msg.Info.Role != "assistant" {
			continue
		}

		aggregated.Cost += msg.Info.Cost
		aggregated.Tokens.Input += msg.Info.Tokens.Input
		aggregated.Tokens.Output += msg.Info.Tokens.Output
		aggregated.Tokens.Reasoning += msg.Info.Tokens.Reasoning
		aggregated.Tokens.Cache += msg.Info.Tokens.Cache.Read + msg.Info.Tokens.Cache.Write
	}

	return &aggregated, messageCount, nil
}
