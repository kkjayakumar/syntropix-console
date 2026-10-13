package correlation

import (
	"encoding/json"
	"strconv"
)

type ParsedEvent struct {
	Source          string
	EventType       string
	CorrelationID   string
	SentryIssueID   string
	SpottingReportID string
	Severity        string
	ShouldCreate    bool
}

func ParseWebhookEvent(source string, payload []byte) ParsedEvent {
	var data map[string]any
	_ = json.Unmarshal(payload, &data)

	out := ParsedEvent{Source: source}
	out.EventType, _ = data["event"].(string)
	out.CorrelationID, _ = data["correlationId"].(string)
	if out.CorrelationID == "" {
		out.CorrelationID, _ = data["correlation_id"].(string)
	}

	switch source {
	case "copilot":
		if issue, ok := data["issue"].(map[string]any); ok {
			if id, ok := issue["id"].(float64); ok {
				out.SentryIssueID = strconv.FormatInt(int64(id), 10)
			}
		}
		out.ShouldCreate = out.EventType == "sentry.issue.created" || out.EventType == "sentry.issue.regressed"
	case "spotting":
		if id, ok := data["reportId"].(string); ok {
			out.SpottingReportID = id
		}
		out.Severity, _ = data["severity"].(string)
		out.ShouldCreate = out.EventType == "spotting.report.created"
	}

	return out
}

func MergeTelemetry(existingSentry, existingSpotting string, parsed ParsedEvent) (sentryID, spottingID string, correlated bool) {
	sentryID = existingSentry
	spottingID = existingSpotting
	if parsed.SentryIssueID != "" {
		sentryID = parsed.SentryIssueID
	}
	if parsed.SpottingReportID != "" {
		spottingID = parsed.SpottingReportID
	}
	correlated = sentryID != "" && spottingID != ""
	return
}
