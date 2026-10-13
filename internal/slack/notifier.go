package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Notifier struct {
	WebhookURL string
	HTTP       *http.Client
}

func NewNotifier(webhookURL string) *Notifier {
	return &Notifier{
		WebhookURL: webhookURL,
		HTTP:       &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *Notifier) Enabled() bool {
	return n != nil && n.WebhookURL != ""
}

func (n *Notifier) Send(ctx context.Context, title, message, severity string) error {
	if !n.Enabled() {
		return nil
	}
	emoji := ":information_source:"
	switch severity {
	case "warning":
		emoji = ":warning:"
	case "critical":
		emoji = ":rotating_light:"
	}
	payload := map[string]any{
		"text": fmt.Sprintf("%s *%s*\n%s", emoji, title, message),
	}
	return n.post(ctx, payload)
}

func (n *Notifier) SendApprovalCard(ctx context.Context, title, message, approveURL, rejectURL string) error {
	if !n.Enabled() {
		return nil
	}
	payload := map[string]any{
		"blocks": []map[string]any{
			{"type": "section", "text": map[string]string{"type": "mrkdwn", "text": fmt.Sprintf("*%s*\n%s", title, message)}},
			{
				"type": "actions",
				"elements": []map[string]any{
					{"type": "button", "text": map[string]string{"type": "plain_text", "text": "Approve Fix"}, "url": approveURL, "style": "primary"},
					{"type": "button", "text": map[string]string{"type": "plain_text", "text": "Reject"}, "url": rejectURL, "style": "danger"},
				},
			},
		},
	}
	return n.post(ctx, payload)
}

func (n *Notifier) post(ctx context.Context, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := n.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("slack webhook status %d", res.StatusCode)
	}
	return nil
}
