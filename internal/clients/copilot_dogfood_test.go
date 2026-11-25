package clients

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindIssueByRequestIDWithCopilotAPM(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/apm/errors", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"id":"group-1","title":"checkout failed"}]`)
	})
	mux.HandleFunc("/api/apm/errors/group-1", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":"group-1","title":"checkout failed"}`)
	})
	mux.HandleFunc("/api/apm/errors/group-1/events", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"event":{"tags":{"request_id":"newer-request"}}},{"event":{"tags":{"request_id":"pilot-request"}}}]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<html>Copilot dashboard</html>")
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	detail, err := NewCopilotClient(server.URL, "").FindIssueByRequestID(t.Context(), "pilot-request")
	if err != nil {
		t.Fatal(err)
	}
	if detail == nil || issueIDString(detail.Issue) != "group-1" || len(detail.Events) != 2 {
		t.Fatalf("expected the APM issue and its event history, got %+v", detail)
	}
}

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
