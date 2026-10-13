package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/store"
)

type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGoogle    Provider = "google"
	ProviderCursor    Provider = "cursor" // OpenAI-compatible when base URL set
)

type Request struct {
	Provider Provider
	Model    string
	APIKey   string
	BaseURL  string // optional override
	System   string
	User     string
	MaxTokens int
}

type Response struct {
	Text             string
	Provider         Provider
	Model            string
	PromptTokens     int
	CompletionTokens int
}

type Client struct {
	HTTP *http.Client
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 90 * time.Second}}
}

func (c *Client) Complete(ctx context.Context, req Request) (Response, error) {
	if c == nil {
		c = NewClient()
	}
	if strings.TrimSpace(req.APIKey) == "" {
		return Response{}, fmt.Errorf("API key is required — store it in Workspace secrets")
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 2048
	}
	switch req.Provider {
	case ProviderOpenAI, ProviderCursor:
		return c.completeOpenAICompat(ctx, req)
	case ProviderAnthropic:
		return c.completeAnthropic(ctx, req)
	case ProviderGoogle:
		return c.completeGoogle(ctx, req)
	default:
		return Response{}, fmt.Errorf("unsupported provider %q (use openai|anthropic|google|cursor)", req.Provider)
	}
}

func DefaultModel(p Provider) string {
	switch p {
	case ProviderAnthropic:
		return "claude-sonnet-4-20250514"
	case ProviderGoogle:
		return "gemini-2.0-flash"
	case ProviderCursor:
		return "cursor-small"
	default:
		return "gpt-4o-mini"
	}
}

func SecretKeyFor(p Provider) string {
	switch p {
	case ProviderAnthropic:
		return "ANTHROPIC_API_KEY"
	case ProviderGoogle:
		return "GOOGLE_API_KEY"
	case ProviderCursor:
		return "CURSOR_API_KEY"
	default:
		return "OPENAI_API_KEY"
	}
}

func DetectAvailableProviders(keys []string) []Provider {
	set := map[string]bool{}
	for _, k := range keys {
		set[strings.ToUpper(k)] = true
	}
	out := []Provider{}
	if set["OPENAI_API_KEY"] {
		out = append(out, ProviderOpenAI)
	}
	if set["ANTHROPIC_API_KEY"] {
		out = append(out, ProviderAnthropic)
	}
	if set["GOOGLE_API_KEY"] {
		out = append(out, ProviderGoogle)
	}
	if set["CURSOR_API_KEY"] {
		out = append(out, ProviderCursor)
	}
	return out
}

// BuildGraphContext packs only retrieved graph nodes into a compact system section.
func BuildGraphContext(nodes []store.GraphNode, maxTokens int) (contextBlock string, nodesUsed int, tokens int) {
	if maxTokens <= 0 {
		maxTokens = 1800
	}
	var b strings.Builder
	b.WriteString("PROJECT CONTEXT GRAPH (compact — do not assume unread files):\n")
	used := 0
	tok := Estimate(b.String())
	for _, n := range nodes {
		chunk := fmt.Sprintf("- [%s] %s (%s): %s\n", n.Kind, n.Title, n.NodeKey, truncate(n.Summary, 420))
		ct := Estimate(chunk)
		if tok+ct > maxTokens {
			break
		}
		b.WriteString(chunk)
		tok += ct
		used++
	}
	if used == 0 {
		b.WriteString("(empty graph — ask user to ingest Workspace → Graph)\n")
	}
	return b.String(), used, tok
}

func Estimate(s string) int {
	return store.EstimateTokens(s)
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func (c *Client) completeOpenAICompat(ctx context.Context, req Request) (Response, error) {
	base := strings.TrimRight(req.BaseURL, "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := req.Model
	if model == "" {
		model = DefaultModel(req.Provider)
	}
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": req.System},
			{"role": "user", "content": req.User},
		},
		"max_tokens": req.MaxTokens,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return Response{}, fmt.Errorf("openai-compatible API %d: %s", res.StatusCode, truncate(string(data), 400))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return Response{}, err
	}
	text := ""
	if len(parsed.Choices) > 0 {
		text = parsed.Choices[0].Message.Content
	}
	return Response{
		Text:             text,
		Provider:         req.Provider,
		Model:            firstNonEmpty(parsed.Model, model),
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
	}, nil
}

func (c *Client) completeAnthropic(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = DefaultModel(ProviderAnthropic)
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": req.MaxTokens,
		"system":     req.System,
		"messages": []map[string]string{
			{"role": "user", "content": req.User},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", req.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return Response{}, fmt.Errorf("anthropic API %d: %s", res.StatusCode, truncate(string(data), 400))
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return Response{}, err
	}
	var text strings.Builder
	for _, c := range parsed.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	return Response{
		Text:             text.String(),
		Provider:         ProviderAnthropic,
		Model:            firstNonEmpty(parsed.Model, model),
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
	}, nil
}

func (c *Client) completeGoogle(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = DefaultModel(ProviderGoogle)
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	prompt := req.System + "\n\n" + req.User
	body := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": prompt}}},
		},
		"generationConfig": map[string]any{"maxOutputTokens": req.MaxTokens},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", req.APIKey)
	res, err := c.HTTP.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return Response{}, fmt.Errorf("google API %d: %s", res.StatusCode, truncate(string(data), 400))
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return Response{}, err
	}
	text := ""
	if len(parsed.Candidates) > 0 && len(parsed.Candidates[0].Content.Parts) > 0 {
		text = parsed.Candidates[0].Content.Parts[0].Text
	}
	return Response{
		Text:             text,
		Provider:         ProviderGoogle,
		Model:            model,
		PromptTokens:     parsed.UsageMetadata.PromptTokenCount,
		CompletionTokens: parsed.UsageMetadata.CandidatesTokenCount,
	}, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
