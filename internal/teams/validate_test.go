package teams

import "testing"

func TestValidateWebhookURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{"", true},
		{"https://outlook.webhook.office.com/webhookb2/abc", true},
		{"https://teams.microsoft.com/l/channel/19%3Afoo/dev-alerts", false},
		{"https://teams.microsoft.com/l/team/19%3Afoo/conversations", false},
		{"https://example.powerplatform.com/powerautomate/automations/direct/workflows/x/triggers/manual/paths/invoke", true},
		{"not-a-url", false},
	}
	for _, tc := range tests {
		ok, _ := ValidateWebhookURL(tc.url)
		if ok != tc.valid {
			t.Fatalf("ValidateWebhookURL(%q) = %v, want %v", tc.url, ok, tc.valid)
		}
	}
}
