package teams

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
	var payload map[string]any
	if IsPowerAutomateURL(n.WebhookURL) {
		payload = powerAutomateAlertPayload(title, message, severity)
	} else {
		color := "2EB886"
		switch severity {
		case "warning":
			color = "EBB424"
		case "critical":
			color = "A30101"
		}
		payload = map[string]any{
			"@type":      "MessageCard",
			"@context":   "http://schema.org/extensions",
			"themeColor": color,
			"summary":    title,
			"sections": []map[string]any{
				{"activityTitle": title, "text": message},
			},
		}
	}
	return n.post(ctx, payload)
}

func (n *Notifier) SendApprovalCard(ctx context.Context, title, message, approveURL, rejectURL string) error {
	if !n.Enabled() {
		return nil
	}
	var payload map[string]any
	if IsPowerAutomateURL(n.WebhookURL) {
		payload = powerAutomateApprovalPayload(title, message, approveURL, rejectURL)
	} else {
		payload = map[string]any{
			"@type":      "MessageCard",
			"@context":   "http://schema.org/extensions",
			"themeColor": "0078D7",
			"summary":    title,
			"sections": []map[string]any{
				{"activityTitle": title, "text": message},
			},
			"potentialAction": []map[string]any{
				{
					"@type": "OpenUri",
					"name":  "Approve Fix",
					"targets": []map[string]string{
						{"os": "default", "uri": approveURL},
					},
				},
				{
					"@type": "OpenUri",
					"name":  "Reject",
					"targets": []map[string]string{
						{"os": "default", "uri": rejectURL},
					},
				},
			},
		}
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
		return fmt.Errorf("teams webhook status %d", res.StatusCode)
	}
	return nil
}
