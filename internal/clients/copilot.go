package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type CopilotClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func NewCopilotClient(baseURL, token string) *CopilotClient {
	return &CopilotClient{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

type CopilotIssueDetail struct {
	Issue  map[string]any   `json:"issue"`
	Events []map[string]any `json:"events"`
}

func (c *CopilotClient) GetIssue(ctx context.Context, issueID string) (*CopilotIssueDetail, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("copilot client not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/sentry/issues/%s", c.BaseURL, issueID), nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("copilot issue fetch %d: %s", res.StatusCode, string(body))
	}
	var out CopilotIssueDetail
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *CopilotClient) GetEventLogs(ctx context.Context, eventID string) ([]string, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("copilot client not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/sentry/events/%s/logs", c.BaseURL, eventID), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copilot logs fetch %d", res.StatusCode)
	}
	var payload struct {
		Logs []string `json:"logs"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Logs, nil
}

type UptimeMonitorInput struct {
	Name     string
	Type     string
	URL      string
	Interval int
	Enabled  int
}

func (c *CopilotClient) CreateUptimeMonitor(ctx context.Context, in UptimeMonitorInput) (map[string]any, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("copilot client not configured")
	}
	if in.Type == "" {
		in.Type = "http"
	}
	if in.Interval <= 0 {
		in.Interval = 60
	}
	if in.Enabled == 0 {
		in.Enabled = 1
	}
	payload := map[string]any{
		"name":     in.Name,
		"type":     in.Type,
		"url":      in.URL,
		"interval": in.Interval,
		"enabled":  in.Enabled,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/uptime/monitors", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("copilot uptime monitor %d: %s", res.StatusCode, string(respBody))
	}
	var out map[string]any
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *CopilotClient) DeleteUptimeMonitor(ctx context.Context, id int) error {
	if c == nil || c.BaseURL == "" {
		return fmt.Errorf("copilot client not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/api/uptime/monitors/%d", c.BaseURL, id), nil)
	if err != nil {
		return err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("copilot delete monitor %d: %s", res.StatusCode, string(body))
	}
	return nil
}

func (c *CopilotClient) ListContainers(ctx context.Context) ([]map[string]any, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("copilot client not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/containers", nil)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("copilot containers %d", res.StatusCode)
	}
	var out []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *CopilotClient) GetContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	if c == nil || c.BaseURL == "" {
		return "", fmt.Errorf("copilot client not configured")
	}
	url := fmt.Sprintf("%s/api/containers/%s/logs", c.BaseURL, containerID)
	if tail > 0 {
		url = fmt.Sprintf("%s?tail=%d", url, tail)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("copilot logs %d: %s", res.StatusCode, string(body))
	}
	return string(body), nil
}
