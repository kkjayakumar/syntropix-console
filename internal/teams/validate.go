package teams

import (
	"net/url"
	"strings"
)

// ValidateWebhookURL checks TEAMS_WEBHOOK_URL is an Incoming Webhook (not a channel deep link).
func ValidateWebhookURL(raw string) (valid bool, hint string) {
	if strings.TrimSpace(raw) == "" {
		return true, ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false, "TEAMS_WEBHOOK_URL must be an https URL"
	}
	host := strings.ToLower(u.Host)
	path := strings.ToLower(u.Path)

	if host == "teams.microsoft.com" && (strings.Contains(path, "/l/channel/") || strings.Contains(path, "/l/team/")) {
		return false, "this is a Teams channel/team link, not a webhook — use Incoming Webhook or a Power Automate HTTP trigger URL"
	}
	if strings.Contains(host, "webhook.office.com") {
		return true, ""
	}
	if strings.Contains(host, "office.com") && strings.Contains(path, "webhook") {
		return true, ""
	}
	if IsPowerAutomateURL(raw) {
		return true, ""
	}
	return false, "expected Incoming Webhook (*.webhook.office.com) or Power Automate HTTP trigger URL"
}
