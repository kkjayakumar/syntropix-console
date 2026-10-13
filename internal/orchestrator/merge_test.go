package orchestrator

import (
	"testing"

	"github.com/kkjayakumar/AegIsOps/internal/store"
)

func TestMergeWebhookEventsDedupesByID(t *testing.T) {
	prior := []store.WebhookEvent{{ID: 1, Source: "copilot"}, {ID: 2, Source: "spotting"}}
	incoming := []store.WebhookEvent{{ID: 2, Source: "spotting"}, {ID: 3, Source: "spotting"}}
	got := mergeWebhookEvents(prior, incoming)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].ID != 1 || got[1].ID != 2 || got[2].ID != 3 {
		t.Fatalf("unexpected order/ids: %+v", got)
	}
}

func TestBackfillTelemetryIDsFromLinks(t *testing.T) {
	sentry := "9001"
	spotting := "rpt_dogfood"
	links := []store.TelemetryLink{
		{SentryIssueID: &sentry, SpottingReportID: &spotting},
	}
	hasCopilot, hasSpotting, sentryID, spottingID := backfillTelemetryIDsFromLinks(links, false, false, "", "")
	if !hasCopilot || !hasSpotting || sentryID != "9001" || spottingID != "rpt_dogfood" {
		t.Fatalf("backfill = copilot=%v spotting=%v sentry=%q spotting=%q", hasCopilot, hasSpotting, sentryID, spottingID)
	}
}

func TestBackfillTelemetryIDsFromLinksDoesNotOverwrite(t *testing.T) {
	stored := "9000"
	links := []store.TelemetryLink{{SentryIssueID: &stored}}
	_, _, sentryID, _ := backfillTelemetryIDsFromLinks(links, true, false, "9001", "")
	if sentryID != "9001" {
		t.Fatalf("sentryID = %q, want existing event id preserved", sentryID)
	}
}
