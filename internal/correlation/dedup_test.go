package correlation

import (
	"encoding/json"
	"testing"

	"github.com/kkjayakumar/AegIsOps/internal/store"
)

func TestFingerprintFromEventsCopilot(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"issue": map[string]any{
			"fingerprint": "abc123",
			"title":       "Test",
		},
	})
	fp := FingerprintFromEvents([]store.WebhookEvent{
		{Source: "copilot", Payload: payload},
	})
	if fp == "" {
		t.Fatal("expected fingerprint")
	}
	if fp == "abc123" {
		t.Fatal("short fingerprint should be hashed")
	}
}

func TestFingerprintFromEventsSpotting(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"networkFailures": []any{
			map[string]any{"method": "GET", "url": "/api/x", "status": 500},
		},
	})
	fp := FingerprintFromEvents([]store.WebhookEvent{
		{Source: "spotting", Payload: payload},
	})
	if fp == "" {
		t.Fatal("expected fingerprint from network failure")
	}
}
