package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

func (s *Service) HandlePullRequest(ctx context.Context, action string, pr map[string]any) {
	if s == nil || s.issues == nil || pr == nil {
		return
	}

	branch := nestedString(pr, "head", "ref")
	body, _ := pr["body"].(string)
	repo := nestedString(pr, "base", "repo", "full_name")
	merged, _ := pr["merged"].(bool)

	issueID := parseAegisBranch(branch)
	if issueID == 0 {
		if ghNum := parseFixesIssueReference(body); ghNum > 0 {
			found, err := s.issues.FindByGitHubIssueNumber(ctx, repo, ghNum)
			if err != nil {
				slog.Error("find issue by github number failed", "error", err)
				return
			}
			if found != nil {
				issueID = found.ID
			}
		}
	}
	if issueID == 0 {
		slog.Info("pull_request ignored", "action", action, "branch", branch)
		return
	}

	switch action {
	case "opened", "reopened", "ready_for_review":
		s.updateIssueLifecycle(ctx, issueID, "pr_created", fmt.Sprintf("PR opened for issue #%d (`%s`)", issueID, branch))
	case "closed":
		if merged {
			s.updateIssueLifecycle(ctx, issueID, "fixed", fmt.Sprintf("PR merged - issue #%d marked fixed", issueID))
		} else {
			s.updateIssueLifecycle(ctx, issueID, "closed", fmt.Sprintf("PR closed without merge - issue #%d marked closed", issueID))
		}
	default:
		slog.Info("pull_request action skipped", "action", action, "issueId", issueID)
	}
}

func (s *Service) updateIssueLifecycle(ctx context.Context, issueID int64, status, teamsMsg string) {
	if err := s.issues.UpdateStatus(ctx, issueID, status); err != nil {
		slog.Error("update issue status failed", "issueId", issueID, "status", status, "error", err)
		return
	}
	dashboardURL := strings.TrimSuffix(s.publicURL, "/") + "/issues"
	msg := teamsMsg + "\n\n[Open dashboard](" + dashboardURL + ")"
	_ = s.teams.Send(ctx, "AegisOps: issue "+status, msg, "info")
	_ = s.slack.Send(ctx, "AegisOps: issue "+status, msg, "info")
	slog.Info("issue status updated", "issueId", issueID, "status", status)
}

func nestedString(root map[string]any, keys ...string) string {
	cur := any(root)
	for _, key := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = obj[key]
	}
	s, _ := cur.(string)
	return s
}
