// SPDX-FileCopyrightText: 2026 Vedran Lebo <vedran@flyingpenguin.tech>
// SPDX-License-Identifier: MIT

// Package cloud provides integration with the ctx-cloud server for team features.
package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client handles communication with the ctx-cloud server.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new cloud client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// IsConfigured returns true if the client has valid configuration.
func (c *Client) IsConfigured() bool {
	return c.baseURL != "" && c.apiKey != ""
}

// AuditEvent represents an audit event to send to the cloud server.
type AuditEvent struct {
	Action       string         `json:"action"`
	ContextName  string         `json:"context_name,omitempty"`
	Environment  string         `json:"environment,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
	Success      bool           `json:"success"`
	ErrorMessage string         `json:"error_message,omitempty"`
}

// HeartbeatInput represents heartbeat data to send to the cloud server.
type HeartbeatInput struct {
	ContextName  string   `json:"context_name"`
	Environment  string   `json:"environment,omitempty"`
	VPNConnected bool     `json:"vpn_connected"`
	Tunnels      []string `json:"tunnels,omitempty"`
}

// SharedContext represents a context fetched from the cloud server.
type SharedContext struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Environment string         `json:"environment,omitempty"`
	Config      map[string]any `json:"config"`
	Version     int            `json:"version"`
	IsAbstract  bool           `json:"is_abstract"`
	Extends     string         `json:"extends,omitempty"`
	UpdatedAt   string         `json:"updated_at"`
}

// apiResponse wraps API responses.
type apiResponse struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Error *apiError       `json:"error,omitempty"`
}

type apiError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// request makes an HTTP request to the cloud server.
func (c *Client) request(method, path string, body any) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "ApiKey "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiResp apiResponse
		if json.Unmarshal(respBody, &apiResp) == nil && apiResp.Error != nil {
			// Include details in error message if present (e.g., partial git sync results)
			if len(apiResp.Error.Details) > 0 {
				detailsJSON, _ := json.Marshal(apiResp.Error.Details)
				return nil, fmt.Errorf("API error [%s]: %s (details: %s)", apiResp.Error.Code, apiResp.Error.Message, string(detailsJSON))
			}
			return nil, fmt.Errorf("API error [%s]: %s", apiResp.Error.Code, apiResp.Error.Message)
		}
		return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return respBody, nil
}

// SendAuditEvent sends an audit event to the cloud server.
func (c *Client) SendAuditEvent(event *AuditEvent) error {
	if !c.IsConfigured() {
		return nil // Silently skip if not configured
	}

	_, err := c.request("POST", "/api/v1/cli/audit", event)
	return err
}

// SendHeartbeat sends a heartbeat to the cloud server.
func (c *Client) SendHeartbeat(input *HeartbeatInput) error {
	if !c.IsConfigured() {
		return nil
	}

	_, err := c.request("POST", "/api/v1/cli/heartbeat", input)
	return err
}

// Deactivate notifies the cloud server that the user's session is ending.
func (c *Client) Deactivate(contextName string) error {
	if !c.IsConfigured() {
		return nil
	}

	body := map[string]string{"context_name": contextName}
	_, err := c.request("DELETE", "/api/v1/sessions/me", body)
	return err
}

// GetSharedContexts fetches shared contexts from the cloud server.
func (c *Client) GetSharedContexts() ([]*SharedContext, error) {
	if !c.IsConfigured() {
		return nil, nil
	}

	respBody, err := c.request("GET", "/api/v1/contexts", nil)
	if err != nil {
		return nil, err
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var contexts []*SharedContext
	if err := json.Unmarshal(apiResp.Data, &contexts); err != nil {
		return nil, fmt.Errorf("failed to parse contexts: %w", err)
	}

	return contexts, nil
}

// SyncContext fetches a specific context by name with resolved inheritance.
func (c *Client) SyncContext(name string) (*SharedContext, error) {
	if !c.IsConfigured() {
		return nil, nil
	}

	respBody, err := c.request("GET", "/api/v1/cli/sync/"+name, nil)
	if err != nil {
		return nil, err
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var ctx SharedContext
	if err := json.Unmarshal(apiResp.Data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to parse context: %w", err)
	}

	return &ctx, nil
}

// TestConnection tests the connection to the cloud server.
func (c *Client) TestConnection() error {
	if !c.IsConfigured() {
		return fmt.Errorf("cloud integration not configured")
	}

	// Use sessions/stats endpoint which works with API key auth and always returns data
	_, err := c.request("GET", "/api/v1/sessions/stats", nil)
	return err
}
