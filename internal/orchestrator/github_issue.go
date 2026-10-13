package orchestrator

import (
	"context"
	"log/slog"
	"strings"

	githubclient "github.com/kkjayakumar/AegIsOps/internal/githubclient"
)

func shouldRetryGitHubIssueWithoutLabels(err error, labels []string) bool {
	if err == nil || len(labels) == 0 {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "422") || strings.Contains(msg, "label")
}

func (s *Service) createGitHubIssue(ctx context.Context, in githubclient.CreateIssueInput) (*githubclient.CreateIssueResult, error) {
	if s == nil || !s.github.Enabled() {
		return nil, nil
	}
	result, err := s.github.CreateIssue(ctx, in)
	if !shouldRetryGitHubIssueWithoutLabels(err, in.Labels) {
		return result, err
	}
	slog.Warn("github issue create with labels failed, retrying without labels", "error", err)
	retry := in
	retry.Labels = nil
	return s.github.CreateIssue(ctx, retry)
}
