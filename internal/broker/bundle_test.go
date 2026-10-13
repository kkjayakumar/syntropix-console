package broker

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kkjayakumar/AegIsOps/internal/store"
)

func TestDeriveTitleDogfoodShortTitle(t *testing.T) {
	copilot, _ := json.Marshal(map[string]any{
		"correlationId": "req_ui_abc123",
		"issue": map[string]any{
			"title": "AegisOps dogfood: intentional backend error for telemetry correlation",
			"id":    float64(5),
		},
	})
	got := DeriveTitle([]store.WebhookEvent{{Source: "copilot", Payload: copilot}})
	if got != "AegisOps dogfood backend error" {
		t.Fatalf("DeriveTitle() = %q", got)
	}
}

func TestDeriveTitlePhase9DogfoodCorrelation(t *testing.T) {
	spotting, _ := json.Marshal(map[string]any{
		"correlationId": "req_phase9_probe",
		"networkFailures": []map[string]any{{
			"method": "POST", "url": "/api/v1/debug/trigger-error", "status": 500,
		}},
	})
	got := DeriveTitle([]store.WebhookEvent{{Source: "spotting", Payload: spotting}})
	if got != "AegisOps dogfood backend error" {
		t.Fatalf("DeriveTitle() = %q, want dogfood short title for req_phase9_", got)
	}
}

func TestIsDogfoodCorrelationPhase9(t *testing.T) {
	if !IsDogfoodCorrelation("req_phase9_probe") {
		t.Fatal("req_phase9_ prefix should be dogfood")
	}
}

func TestBuildBundleDogfoodIncludesNetworkAndHint(t *testing.T) {
	spotting, _ := json.Marshal(map[string]any{
		"correlationId": "req_phase9_probe",
		"reportId":      "rpt_dogfood_reqphase9probe",
		"severity":      "error",
		"pageUrl":       "https://aegisops.syntropix.io/",
		"networkFailures": []map[string]any{{
			"method": "POST", "url": "/api/v1/debug/trigger-error", "status": 500,
		}},
	})
	bundle := BuildBundle(BundleInput{
		CorrelationID:    "req_phase9_probe",
		SpottingReportID: "rpt_dogfood_reqphase9probe",
		WebhookEvents:    []store.WebhookEvent{{Source: "spotting", Payload: spotting}},
	})
	if !strings.Contains(bundle, "POST /api/v1/debug/trigger-error") {
		t.Fatalf("bundle missing network failure: %s", bundle)
	}
	if !strings.Contains(bundle, "## Dogfood (intentional)") {
		t.Fatalf("bundle missing dogfood section: %s", bundle)
	}
	if !strings.Contains(bundle, "SENTRY_DSN") {
		t.Fatalf("bundle missing backend hint: %s", bundle)
	}
}

func TestBuildBundleDogfoodWithSentryOmitsMissingHint(t *testing.T) {
	spotting, _ := json.Marshal(map[string]any{
		"correlationId": "req_phase9_probe",
		"reportId":      "rpt_dogfood",
		"severity":      "error",
	})
	bundle := BuildBundle(BundleInput{
		CorrelationID:    "req_phase9_71a6b06d78",
		SentryIssueID:    "9001",
		SpottingReportID: "rpt_dogfood",
		WebhookEvents:    []store.WebhookEvent{{Source: "spotting", Payload: spotting}},
	})
	if strings.Contains(bundle, "Backend (Copilot) telemetry missing") {
		t.Fatalf("bundle should not warn when SentryIssueID set: %s", bundle)
	}
}

func TestAppendCICDToBundle(t *testing.T) {
	deps := []store.Deployment{{
		WorkflowName: "CI",
		Status:       "success",
		Branch:       "main",
		CommitSHA:    "abc1234def",
	}}
	got := AppendCICDToBundle("# Incident\n", deps)
	if !strings.Contains(got, "## CI/CD") || !strings.Contains(got, "CI") {
		t.Fatalf("AppendCICDToBundle() = %q", got)
	}
	if AppendCICDToBundle("# x\n\n## CI/CD\n", deps) != "# x\n\n## CI/CD\n" {
		t.Fatal("should not duplicate CI/CD section")
	}
}

func TestDeriveTitleDogfoodWithSpottingPrefersShortDogfoodTitle(t *testing.T) {
	copilot, _ := json.Marshal(map[string]any{
		"correlationId": "req_ui_abc123",
		"issue":         map[string]any{"title": "Unknown Exception"},
	})
	spotting, _ := json.Marshal(map[string]any{
		"correlationId": "req_ui_abc123",
		"networkFailures": []map[string]any{{
			"method": "POST", "url": "/api/v1/debug/trigger-error", "status": 500,
		}},
	})
	events := []store.WebhookEvent{
		{Source: "copilot", Payload: copilot},
		{Source: "spotting", Payload: spotting},
	}
	got := DeriveTitle(events)
	if got != "AegisOps dogfood backend error" {
		t.Fatalf("DeriveTitle() = %q, want dogfood short title", got)
	}
}
