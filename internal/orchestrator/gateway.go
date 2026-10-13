package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Service) notifyGateway(ctx context.Context, issueID int64, title, correlationID, dashboardURL string) {
	if s == nil || s.gateway == nil {
		return
	}
	ttl := 72 * time.Hour
	approve, reject, err := s.gateway.CreatePair(ctx, issueID, ttl)
	if err != nil {
		return
	}
	base := strings.TrimSuffix(s.publicURL, "/")
	approveURL := fmt.Sprintf("%s/api/v1/gateway/approve?token=%s&format=html", base, approve.Token)
	rejectURL := fmt.Sprintf("%s/api/v1/gateway/reject?token=%s&format=html", base, reject.Token)
	msg := fmt.Sprintf(
		"**%s**\n\nCorrelation: `%s`\n\n[Open dashboard](%s)",
		title, correlationID, dashboardURL,
	)
	_ = s.teams.SendApprovalCard(ctx, "AegisOps: approve fix?", msg, approveURL, rejectURL)
	_ = s.slack.SendApprovalCard(ctx, "AegisOps: approve fix?", msg, approveURL, rejectURL)
}

func (s *Service) HandleGatewayAction(ctx context.Context, token string) (storeIssueID int64, action string, err error) {
	if s == nil || s.gateway == nil || s.issues == nil {
		return 0, "", fmt.Errorf("gateway unavailable")
	}
	tok, err := s.gateway.Consume(ctx, token)
	if err != nil {
		return 0, "", err
	}
	switch tok.Action {
	case "approve":
		if err := s.issues.UpdateStatus(ctx, tok.IssueID, "in_progress"); err != nil {
			return tok.IssueID, tok.Action, err
		}
	case "reject":
		if err := s.issues.UpdateStatus(ctx, tok.IssueID, "closed"); err != nil {
			return tok.IssueID, tok.Action, err
		}
	default:
		return tok.IssueID, tok.Action, fmt.Errorf("unknown action %q", tok.Action)
	}
	return tok.IssueID, tok.Action, nil
}
