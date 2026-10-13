package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const dogfoodCopilotPollAttempts = 8

// FindIssueByRequestID scans recent Copilot Sentry issues for an event tagged with requestID.
func (c *CopilotClient) FindIssueByRequestID(ctx context.Context, requestID string) (*CopilotIssueDetail, error) {
	if c == nil || c.BaseURL == "" || requestID == "" {
		return nil, fmt.Errorf("copilot client not configured")
	}
	issues, err := c.listIssues(ctx)
	if err != nil {
		return nil, err
	}
	limit := 25
	if len(issues) < limit {
		limit = len(issues)
	}
	for i := 0; i < limit; i++ {
		id := issueIDString(issues[i])
		if id == "" {
			continue
		}
		detail, err := c.GetIssue(ctx, id)
		if err != nil {
			continue
		}
		for _, ev := range detail.Events {
			if eventMatchesRequestID(ev, requestID) {
				return detail, nil
			}
		}
	}
	return nil, nil
}

// PollIssueByRequestID retries FindIssueByRequestID while Copilot ingests the Sentry event.
func (c *CopilotClient) PollIssueByRequestID(ctx context.Context, requestID string) (*CopilotIssueDetail, error) {
	var lastErr error
	for attempt := 0; attempt < dogfoodCopilotPollAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		detail, err := c.FindIssueByRequestID(ctx, requestID)
		if err != nil {
			lastErr = err
		} else if detail != nil && detail.Issue != nil {
			return detail, nil
		}
		if attempt+1 < dogfoodCopilotPollAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(400 * time.Millisecond):
			}
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}

// DogfoodCopilotWebhookPayload builds the webhook shape AegisOps expects from a Copilot issue.
func DogfoodCopilotWebhookPayload(correlationID string, issue map[string]any, isNew bool) map[string]any {
	event := "sentry.issue.regressed"
	if isNew {
		event = "sentry.issue.created"
	}
	return map[string]any{
		"event":         event,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
		"correlationId": correlationID,
		"issue":         issue,
	}
}

func (c *CopilotClient) listIssues(ctx context.Context) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/sentry/issues", nil)
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
		return nil, fmt.Errorf("copilot issues list %d: %s", res.StatusCode, string(body))
	}
	var out []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func issueIDString(issue map[string]any) string {
	switch id := issue["id"].(type) {
	case float64:
		return strconv.FormatInt(int64(id), 10)
	case int:
		return strconv.Itoa(id)
	case int64:
		return strconv.FormatInt(id, 10)
	case string:
		return id
	default:
		return ""
	}
}

func eventMatchesRequestID(event map[string]any, requestID string) bool {
	payload, ok := event["payload"].(map[string]any)
	if !ok {
		var raw map[string]any
		if b, err := json.Marshal(event); err == nil {
			_ = json.Unmarshal(b, &raw)
			if p, ok := raw["payload"].(map[string]any); ok {
				payload = p
			} else {
				payload = raw
			}
		}
	}
	if payload == nil {
		return false
	}
	return tagValue(payload, "request_id") == requestID ||
		tagValue(payload, "requestId") == requestID ||
		tagValue(payload, "correlation_id") == requestID ||
		tagValue(payload, "correlationId") == requestID
}

func tagValue(event map[string]any, key string) string {
	tags, ok := event["tags"].(map[string]any)
	if ok {
		if v, ok := tags[key].(string); ok {
			return v
		}
	}
	if arr, ok := event["tags"].([]any); ok {
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			k, _ := m["key"].(string)
			if k != key {
				continue
			}
			if v, ok := m["value"].(string); ok {
				return v
			}
		}
	}
	return ""
}
