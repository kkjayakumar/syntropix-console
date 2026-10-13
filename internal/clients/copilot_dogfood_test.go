package clients

import "testing"

func TestEventMatchesRequestIDMapTags(t *testing.T) {
	event := map[string]any{
		"payload": map[string]any{
			"tags": map[string]any{"request_id": "req_phase9_abc"},
		},
	}
	if !eventMatchesRequestID(event, "req_phase9_abc") {
		t.Fatal("expected map tags to match")
	}
}

func TestEventMatchesRequestIDArrayTags(t *testing.T) {
	event := map[string]any{
		"payload": map[string]any{
			"tags": []any{
				map[string]any{"key": "request_id", "value": "req_phase9_xyz"},
			},
		},
	}
	if !eventMatchesRequestID(event, "req_phase9_xyz") {
		t.Fatal("expected array tags to match")
	}
}

func TestDogfoodCopilotWebhookPayload(t *testing.T) {
	issue := map[string]any{"id": float64(9001), "title": "dogfood"}
	payload := DogfoodCopilotWebhookPayload("req_phase9_test", issue, true)
	if payload["event"] != "sentry.issue.created" {
		t.Fatalf("event = %v", payload["event"])
	}
	if payload["correlationId"] != "req_phase9_test" {
		t.Fatalf("correlationId = %v", payload["correlationId"])
	}
}
