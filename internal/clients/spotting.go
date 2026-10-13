package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type SpottingClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func NewSpottingClient(baseURL, token string) *SpottingClient {
	return &SpottingClient{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *SpottingClient) GetReport(ctx context.Context, reportID string) (map[string]any, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("spotting client not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/reports/%s", c.BaseURL, reportID), nil)
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
		return nil, fmt.Errorf("spotting report fetch %d: %s", res.StatusCode, string(body))
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *SpottingClient) GetNetwork(ctx context.Context, reportID string) ([]map[string]any, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("spotting client not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v1/reports/%s/network", c.BaseURL, reportID), nil)
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
		return nil, fmt.Errorf("spotting network fetch %d", res.StatusCode)
	}
	var payload struct {
		Requests []map[string]any `json:"requests"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Requests, nil
}
