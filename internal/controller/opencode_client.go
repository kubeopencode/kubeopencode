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
// Supports both OpenCode 1.x and 2.x fields.
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
	Cost   *float64 `json:"cost,omitempty"`
	Tokens *struct {
		Input     int64              `json:"input"`
		Output    int64              `json:"output"`
		Reasoning int64              `json:"reasoning"`
		Cache     OpenCodeCacheUsage `json:"cache"`
	} `json:"tokens,omitempty"`
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

// OpenCodeFileDiff represents file changes in a session (OpenCode 2.x).
type OpenCodeFileDiff struct {
	Path      string `json:"path"`
	Additions int32  `json:"additions"`
	Deletions int32  `json:"deletions"`
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

// getWithFallback performs a GET request against apiURL, falling back to legacyURL if apiURL returns 404.
func (c *OpenCodeClient) getWithFallback(ctx context.Context, apiURL, legacyURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound && legacyURL != "" && legacyURL != apiURL {
		legacyReq, err := http.NewRequestWithContext(ctx, http.MethodGet, legacyURL, nil)
		if err == nil {
			legacyResp, err := c.httpClient.Do(legacyReq)
			if err == nil {
				defer func() { _ = legacyResp.Body.Close() }()
				if legacyResp.StatusCode == http.StatusOK {
					return io.ReadAll(legacyResp.Body)
				}
			}
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// decodeSessions decodes session list responses in either 2.x envelope ({ "data": [...] })
// or 1.x plain array format ([...]).
func decodeSessions(body []byte) ([]OpenCodeSession, error) {
	var env struct {
		Data []OpenCodeSession `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Data != nil {
		return env.Data, nil
	}

	var list []OpenCodeSession
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("decoding sessions response: %w", err)
	}
	return list, nil
}

// decodeSession decodes a single session response in either 2.x envelope ({ "data": {...} })
// or 1.x plain object format ({...}).
func decodeSession(body []byte) (*OpenCodeSession, error) {
	var env struct {
		Data OpenCodeSession `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Data.ID != "" {
		return &env.Data, nil
	}

	var session OpenCodeSession
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, fmt.Errorf("decoding session response: %w", err)
	}
	return &session, nil
}

// decodeMessages decodes message list responses in either 2.x envelope ({ "data": [...] })
// or 1.x plain array format ([...]).
func decodeMessages(body []byte) ([]OpenCodeMessage, error) {
	var env struct {
		Data []OpenCodeMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Data != nil {
		return env.Data, nil
	}

	var list []OpenCodeMessage
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("decoding messages response: %w", err)
	}
	return list, nil
}

// decodeFileDiffs decodes file diffs in 2.x envelope or plain array format.
func decodeFileDiffs(body []byte) ([]OpenCodeFileDiff, error) {
	var env struct {
		Data []OpenCodeFileDiff `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Data != nil {
		return env.Data, nil
	}

	var list []OpenCodeFileDiff
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("decoding diff response: %w", err)
	}
	return list, nil
}

// FindSessionByTitle searches for a session by title on the given OpenCode server.
// Returns nil if no matching session is found (not an error).
func (c *OpenCodeClient) FindSessionByTitle(ctx context.Context, serverURL, title string) (*OpenCodeSession, error) {
	apiURL := fmt.Sprintf("%s/api/session?search=%s", serverURL, url.QueryEscape(title))
	legacyURL := fmt.Sprintf("%s/session?search=%s", serverURL, url.QueryEscape(title))

	body, err := c.getWithFallback(ctx, apiURL, legacyURL)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}

	sessions, err := decodeSessions(body)
	if err != nil {
		return nil, err
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
	apiURL := fmt.Sprintf("%s/api/session/%s", serverURL, url.PathEscape(sessionID))
	legacyURL := fmt.Sprintf("%s/session/%s", serverURL, url.PathEscape(sessionID))

	body, err := c.getWithFallback(ctx, apiURL, legacyURL)
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}

	return decodeSession(body)
}

// GetSessionMessages retrieves all messages for a session.
// Used to aggregate token usage and cost information.
func (c *OpenCodeClient) GetSessionMessages(ctx context.Context, serverURL, sessionID string) ([]OpenCodeMessage, error) {
	apiURL := fmt.Sprintf("%s/api/session/%s/message", serverURL, url.PathEscape(sessionID))
	legacyURL := fmt.Sprintf("%s/session/%s/message", serverURL, url.PathEscape(sessionID))

	body, err := c.getWithFallback(ctx, apiURL, legacyURL)
	if err != nil {
		return nil, fmt.Errorf("getting messages: %w", err)
	}

	return decodeMessages(body)
}

// GetSessionDiff retrieves file diffs for a session (OpenCode 2.x).
func (c *OpenCodeClient) GetSessionDiff(ctx context.Context, serverURL, sessionID string) ([]OpenCodeFileDiff, error) {
	apiURL := fmt.Sprintf("%s/api/session/%s/diff", serverURL, url.PathEscape(sessionID))
	legacyURL := fmt.Sprintf("%s/session/%s/diff", serverURL, url.PathEscape(sessionID))

	body, err := c.getWithFallback(ctx, apiURL, legacyURL)
	if err != nil {
		return nil, fmt.Errorf("getting diff: %w", err)
	}

	return decodeFileDiffs(body)
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
