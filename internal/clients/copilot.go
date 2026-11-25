package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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

var errCopilotRouteUnavailable = errors.New("copilot API route unavailable")

func (c *CopilotClient) GetIssue(ctx context.Context, issueID string) (*CopilotIssueDetail, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("copilot client not configured")
	}
	id := url.PathEscape(issueID)
	var out CopilotIssueDetail
	if err := c.getIssueJSON(ctx, "/api/sentry/issues/"+id, &out); !errors.Is(err, errCopilotRouteUnavailable) {
		if err != nil {
			return nil, err
		}
		return &out, nil
	}

	path := "/api/apm/errors/" + id
	var issue map[string]any
	if err := c.getIssueJSON(ctx, path, &issue); err != nil {
		return nil, err
	}
	var events []struct {
		Event map[string]any `json:"event"`
	}
	if err := c.getIssueJSON(ctx, path+"/events", &events); err != nil {
		return nil, err
	}
	out.Issue = issue
	for _, event := range events {
		if event.Event != nil {
			out.Events = append(out.Events, map[string]any{"payload": event.Event})
		}
	}
	return &out, nil
}

func (c *CopilotClient) getIssueJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// Some versions serve the dashboard HTML for an unknown API route.
	if res.StatusCode == http.StatusNotFound || (res.StatusCode == http.StatusOK &&
		strings.HasPrefix(strings.ToLower(res.Header.Get("Content-Type")), "text/html")) {
		return fmt.Errorf("%w: %s", errCopilotRouteUnavailable, path)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("copilot %s returned HTTP %d", path, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("decode copilot %s: %w", path, err)
	}
	return nil
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
