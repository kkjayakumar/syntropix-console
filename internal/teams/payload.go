package teams

import (
	"fmt"
	"strings"
)

// IsPowerAutomateURL is true for Power Automate / Power Platform HTTP trigger URLs.
func IsPowerAutomateURL(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, "powerplatform.com") ||
		strings.Contains(lower, "logic.azure.com") ||
		strings.Contains(lower, "powerautomate")
}

func powerAutomateAlertPayload(title, message, severity string) map[string]any {
	return map[string]any{
		"kind":     "alert",
		"title":    title,
		"message":  message,
		"severity": severity,
	}
}

func powerAutomateApprovalPayload(title, message, approveURL, rejectURL string) map[string]any {
	// Flows that only post `message` still get clickable links (markdown + raw URLs).
	linked := fmt.Sprintf(
		"%s\n\n**Approve Fix:** [link](%s)\n**Reject:** [link](%s)\n\nOr copy:\nApprove: %s\nReject: %s",
		message, approveURL, rejectURL, approveURL, rejectURL,
	)
	return map[string]any{
		"kind":       "approval",
		"title":      title,
		"message":    linked,
		"severity":   "warning",
		"approveUrl": approveURL,
		"rejectUrl":  rejectURL,
		"actions": []map[string]string{
			{"title": "Approve Fix", "url": approveURL},
			{"title": "Reject", "url": rejectURL},
		},
	}
}
