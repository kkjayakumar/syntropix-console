package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/clients"
	githubclient "github.com/kkjayakumar/AegIsOps/internal/githubclient"
	"github.com/kkjayakumar/AegIsOps/internal/store"
)

type BundleInput struct {
	CorrelationID     string
	SentryIssueID     string
	SpottingReportID  string
	Copilot           *clients.CopilotClient
	Spotting          *clients.SpottingClient
	WebhookEvents     []store.WebhookEvent
	RecentDeployment  *store.Deployment
	IncludeDeployment bool
	GitHubClient      *githubclient.Client
	GitHubRepo        string
}

func BuildBundle(input BundleInput) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Incident %s\n\n", input.CorrelationID))
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339)))

	// Summary from webhooks
	b.WriteString("## Summary\n")
	for _, ev := range input.WebhookEvents {
		var payload map[string]any
		if json.Unmarshal(ev.Payload, &payload) == nil {
			if ev.Source == "copilot" {
				if issue, ok := payload["issue"].(map[string]any); ok {
					title, _ := issue["title"].(string)
					id := formatCopilotIssueID(issue["id"])
					b.WriteString(fmt.Sprintf("- Backend: **%s** (Sentry issue %s)\n", title, id))
				}
			}
			if ev.Source == "spotting" {
				b.WriteString(fmt.Sprintf("- Frontend: report `%v` severity `%v`\n", payload["reportId"], payload["severity"]))
				if page, ok := payload["pageUrl"].(string); ok && page != "" {
					b.WriteString(fmt.Sprintf("- Page: %s\n", page))
				}
				for _, line := range spottingNetworkFromPayload(payload) {
					b.WriteString(line)
				}
			}
		}
	}

	if input.SpottingReportID != "" {
		b.WriteString("\n## Frontend (Spotting)\n")
		b.WriteString(fmt.Sprintf("- Report ID: `%s`\n", input.SpottingReportID))
		networkLines := spottingNetworkFromEvents(input.WebhookEvents)
		if input.Spotting != nil {
			if report, err := input.Spotting.GetReport(context.Background(), input.SpottingReportID); err == nil {
				if title, ok := report["title"].(string); ok {
					b.WriteString(fmt.Sprintf("- Title: %s\n", title))
				}
				if url, ok := report["pageUrl"].(string); ok && url != "" {
					b.WriteString(fmt.Sprintf("- URL: %s\n", url))
				}
			}
			if network, err := input.Spotting.GetNetwork(context.Background(), input.SpottingReportID); err == nil && len(network) > 0 {
				networkLines = formatNetworkLines(network)
			}
		}
		for _, line := range networkLines {
			b.WriteString(line)
		}
	}

	if IsDogfoodCorrelation(input.CorrelationID) {
		b.WriteString("\n## Dogfood (intentional)\n")
		b.WriteString("- This incident was triggered by the Phase 4/9 dogfood path (`debug/trigger-error` or dashboard panel).\n")
		b.WriteString("- The backend 500 is expected — do not remove the endpoint; improve telemetry/titles instead.\n")
		if input.SentryIssueID == "" {
			b.WriteString("- Backend (Copilot) telemetry missing — set `SENTRY_DSN` with a reachable host (`COPILOT_INGEST_PUBLIC_URL=monitoring.syntropix.io`).\n")
		}
	}

	if input.SentryIssueID != "" && input.Copilot != nil {
		b.WriteString("\n## Backend (DevOps Copilot)\n")
		b.WriteString(fmt.Sprintf("- Sentry issue ID: `%s`\n", input.SentryIssueID))
		if detail, err := input.Copilot.GetIssue(context.Background(), input.SentryIssueID); err == nil && detail.Issue != nil {
			b.WriteString(fmt.Sprintf("- Title: %v\n", detail.Issue["title"]))
			b.WriteString(fmt.Sprintf("- Count: %v\n", detail.Issue["count"]))
			culprit, _ := detail.Issue["culprit"].(string)
			if culprit == "" || culprit == "unknown" {
				culprit = exceptionCulprit(input.WebhookEvents)
			}
			if culprit == "" && IsDogfoodCorrelation(input.CorrelationID) {
				culprit = "AegisOps dogfood (dashboard debug/trigger-error)"
			}
			if culprit != "" {
				b.WriteString(fmt.Sprintf("- Culprit: `%s`\n", culprit))
			}
			if len(detail.Events) > 0 {
				event := detail.Events[0]
				// Check for git-blame correlation
				if input.GitHubClient != nil && input.GitHubClient.Enabled() && input.GitHubRepo != "" {
					if filename, lineno, ok := ExtractCulpritFrame(event); ok {
						owner, name, err := githubclient.ParseRepo(input.GitHubRepo)
						if err == nil {
							ref := "main"
							if input.RecentDeployment != nil && input.RecentDeployment.CommitSHA != "" {
								ref = input.RecentDeployment.CommitSHA
							}
							blame, err := input.GitHubClient.GetLineBlame(context.Background(), owner, name, ref, filename, lineno)
							if err == nil && blame != nil {
								timeAgo := formatTimeAgo(blame.AuthoredDate)
								b.WriteString("\n### Git Regression Blame\n")
								b.WriteString(fmt.Sprintf("- File: `%s` (Line %d)\n", filename, lineno))
								b.WriteString(fmt.Sprintf("- Last modified by: **%s** (%s)\n", blame.AuthorName, blame.AuthorEmail))
								b.WriteString(fmt.Sprintf("- Commit: `%s` (%s)\n", shortSHA(blame.CommitSHA), timeAgo))
								b.WriteString(fmt.Sprintf("- Message: _%s_\n", strings.Split(blame.CommitMsg, "\n")[0]))
							}
						}
					}
				}

				if eventID, ok := event["eventId"].(string); ok {
					if logs, err := input.Copilot.GetEventLogs(context.Background(), eventID); err == nil && len(logs) > 0 {
						b.WriteString("\n### Correlated logs\n")
						for i, line := range prioritizeLogLines(logs, 40) {
							if i >= 40 {
								b.WriteString("...(truncated)\n")
								break
							}
							b.WriteString(line)
							b.WriteString("\n")
						}
					}
				}
			}
		}
	}

	if input.RecentDeployment != nil && input.IncludeDeployment {
		b.WriteString("\n## CI/CD\n")
		b.WriteString(fmt.Sprintf("- Workflow: %s (%s)\n", input.RecentDeployment.WorkflowName, input.RecentDeployment.Status))
		b.WriteString(fmt.Sprintf("- Branch: %s commit `%s`\n", input.RecentDeployment.Branch, input.RecentDeployment.CommitSHA))
		if input.RecentDeployment.FailureLogs != nil && *input.RecentDeployment.FailureLogs != "" {
			lines := strings.Split(*input.RecentDeployment.FailureLogs, "\n")
			b.WriteString("\n### Failure log (last lines)\n```\n")
			start := 0
			if len(lines) > 30 {
				start = len(lines) - 30
			}
			b.WriteString(strings.Join(lines[start:], "\n"))
			b.WriteString("\n```\n")
		}
	}

	out := b.String()
	if len(out) > 8192 {
		return out[:8192] + "\n...(truncated)"
	}
	return out
}

func exceptionCulprit(events []store.WebhookEvent) string {
	for _, ev := range events {
		if ev.Source != "copilot" {
			continue
		}
		var data map[string]any
		if json.Unmarshal(ev.Payload, &data) != nil {
			continue
		}
		if issue, ok := data["issue"].(map[string]any); ok {
			if c, ok := issue["culprit"].(string); ok && c != "" && c != "unknown" {
				return c
			}
		}
	}
	return ""
}

func DeriveTitle(events []store.WebhookEvent) string {
	var correlationID, copilotTitle, spottingTitle string
	for _, ev := range events {
		var payload map[string]any
		if json.Unmarshal(ev.Payload, &payload) != nil {
			continue
		}
		if cid, ok := payload["correlationId"].(string); ok && cid != "" {
			correlationID = cid
		}
		if ev.Source == "copilot" {
			if issue, ok := payload["issue"].(map[string]any); ok {
				if title, ok := issue["title"].(string); ok && title != "" {
					copilotTitle = title
				}
			}
		}
		if ev.Source == "spotting" {
			if failures, ok := payload["networkFailures"].([]any); ok && len(failures) > 0 {
				if f, ok := failures[0].(map[string]any); ok {
					spottingTitle = fmt.Sprintf("Network %v on %v", f["status"], f["url"])
				}
			}
		}
	}
	if dogfood := dogfoodTitle(correlationID); dogfood != "" {
		return dogfood
	}
	if spottingTitle != "" && isGenericCopilotTitle(copilotTitle) {
		return spottingTitle
	}
	if copilotTitle != "" && !isGenericCopilotTitle(copilotTitle) {
		// Prefer short title for dogfood-style long exception messages.
		if len(copilotTitle) > 80 && strings.Contains(strings.ToLower(copilotTitle), "dogfood") {
			return "AegisOps dogfood backend error"
		}
		return copilotTitle
	}
	if spottingTitle != "" {
		return spottingTitle
	}
	if copilotTitle != "" {
		return copilotTitle
	}
	return "Detected incident"
}

func IsDogfoodCorrelation(correlationID string) bool {
	return strings.HasPrefix(correlationID, "req_ui_") ||
		strings.HasPrefix(correlationID, "req_aegis_") ||
		strings.HasPrefix(correlationID, "req_phase9_")
}

func formatCopilotIssueID(id any) string {
	switch v := id.(type) {
	case float64:
		return fmt.Sprintf("#%.0f", v)
	case int:
		return fmt.Sprintf("#%d", v)
	case int64:
		return fmt.Sprintf("#%d", v)
	case string:
		if v != "" {
			return v
		}
	default:
		if id != nil {
			return fmt.Sprintf("%v", id)
		}
	}
	return "n/a"
}

func isGenericCopilotTitle(title string) bool {
	switch strings.TrimSpace(title) {
	case "", "Unknown Exception", "Error", "Exception":
		return true
	default:
		return false
	}
}

func dogfoodTitle(correlationID string) string {
	if IsDogfoodCorrelation(correlationID) {
		return "AegisOps dogfood backend error"
	}
	return ""
}

func spottingNetworkFromEvents(events []store.WebhookEvent) []string {
	var lines []string
	for _, ev := range events {
		if ev.Source != "spotting" {
			continue
		}
		var payload map[string]any
		if json.Unmarshal(ev.Payload, &payload) != nil {
			continue
		}
		lines = append(lines, spottingNetworkFromPayload(payload)...)
	}
	return lines
}

func spottingNetworkFromPayload(payload map[string]any) []string {
	failures, ok := payload["networkFailures"].([]any)
	if !ok || len(failures) == 0 {
		return nil
	}
	var lines []string
	for i, item := range failures {
		if i >= 5 {
			break
		}
		f, ok := item.(map[string]any)
		if !ok {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %v %v → %v\n", f["method"], f["url"], f["status"]))
	}
	return lines
}

func formatNetworkLines(network []map[string]any) []string {
	var lines []string
	for i, n := range network {
		if i >= 5 {
			break
		}
		lines = append(lines, fmt.Sprintf("- %v %v → %v\n", n["method"], n["url"], n["status"]))
	}
	return lines
}

func prioritizeLogLines(logs []string, max int) []string {
	if len(logs) == 0 {
		return logs
	}
	var priority, rest []string
	for _, line := range logs {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") ||
			strings.Contains(lower, "trigger-error") ||
			strings.Contains(lower, "dogfood") ||
			strings.Contains(lower, " 500") ||
			strings.Contains(lower, "level=error") {
			priority = append(priority, line)
		} else {
			rest = append(rest, line)
		}
	}
	out := append(priority, rest...)
	if len(out) > max {
		return out[:max]
	}
	return out
}

// AppendCICDToBundle adds a CI/CD section when missing and deployments exist.
func AppendCICDToBundle(bundle string, deps []store.Deployment) string {
	if len(deps) == 0 || strings.Contains(bundle, "## CI/CD") {
		return bundle
	}
	var b strings.Builder
	b.WriteString(bundle)
	if !strings.HasSuffix(bundle, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n## CI/CD\n")
	for i, d := range deps {
		if i >= 5 {
			break
		}
		b.WriteString(fmt.Sprintf("- **%s** (%s) — branch `%s` commit `%s`\n",
			d.WorkflowName, d.Status, d.Branch, shortSHA(d.CommitSHA)))
		if d.FailureLogs != nil && *d.FailureLogs != "" {
			b.WriteString(fmt.Sprintf("  - Logs: %s\n", strings.TrimSpace(*d.FailureLogs)))
		}
	}
	return b.String()
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func ExtractCulpritFrame(event map[string]any) (string, int, bool) {
	if event == nil {
		return "", 0, false
	}

	payloadMap := event
	if p, ok := event["payload"].(map[string]any); ok {
		payloadMap = p
	}

	excVal, ok := payloadMap["exception"].(map[string]any)
	if !ok {
		return "", 0, false
	}

	values, ok := excVal["values"].([]any)
	if !ok || len(values) == 0 {
		return "", 0, false
	}

	firstExc, ok := values[0].(map[string]any)
	if !ok {
		return "", 0, false
	}

	st, ok := firstExc["stacktrace"].(map[string]any)
	if !ok {
		return "", 0, false
	}

	frames, ok := st["frames"].([]any)
	if !ok || len(frames) == 0 {
		return "", 0, false
	}

	for i := len(frames) - 1; i >= 0; i-- {
		frame, ok := frames[i].(map[string]any)
		if !ok {
			continue
		}

		inApp, _ := frame["in_app"].(bool)
		filename, _ := frame["filename"].(string)
		filename = cleanFilename(filename)
		if filename == "" {
			continue
		}

		lineVal := frame["lineno"]
		var lineNum int
		switch v := lineVal.(type) {
		case float64:
			lineNum = int(v)
		case int:
			lineNum = v
		case int64:
			lineNum = int(v)
		}

		if lineNum > 0 {
			if inApp || i == len(frames)-1 {
				return filename, lineNum, true
			}
		}
	}

	return "", 0, false
}

func cleanFilename(fn string) string {
	fn = strings.TrimPrefix(fn, "app:///")
	fn = strings.TrimPrefix(fn, "/")
	fn = strings.TrimPrefix(fn, "webpack:///")
	return fn
}

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "yesterday"
	}
	if days < 30 {
		return fmt.Sprintf("%d days ago", days)
	}
	months := days / 30
	if months == 1 {
		return "1 month ago"
	}
	return fmt.Sprintf("%d months ago", months)
}
