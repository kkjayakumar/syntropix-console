package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	githubclient "github.com/kkjayakumar/AegIsOps/internal/githubclient"
)

type OpenPRResult struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Branch string `json:"branch"`
}

func (s *Service) OpenPRForIssue(ctx context.Context, issueID int64) (*OpenPRResult, error) {
	if s == nil || s.issues == nil {
		return nil, fmt.Errorf("orchestrator not configured")
	}
	if !s.github.Enabled() {
		return nil, fmt.Errorf("github not configured (set GITHUB_APP_* or GITHUB_TOKEN)")
	}

	issue, err := s.issues.GetByID(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("issue not found: %w", err)
	}

	repo := issue.GitHubRepo
	if repo == "" {
		repo = s.defaultRepo
	}
	rt := s.resolveRuntime(ctx, repo)
	if rt.Repo != "" {
		repo = rt.Repo
	}
	if repo == "" {
		return nil, fmt.Errorf("github repo not configured")
	}

	owner, name, err := githubclient.ParseRepo(repo)
	if err != nil {
		return nil, err
	}

	kind := "fix"
	if issue.IssueType == "feat" {
		kind = "feat"
	}
	branch := fmt.Sprintf("aegis/%s-%d", kind, issueID)

	existing, err := s.github.FindOpenPullRequest(ctx, owner, name, branch)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		s.updateIssueLifecycle(ctx, issueID, "pr_created", fmt.Sprintf("PR #%d already open: %s", existing.Number, existing.URL))
		return &OpenPRResult{Number: existing.Number, URL: existing.URL, Branch: branch}, nil
	}

	base := "main"
	if s.cfg != nil && s.cfg.GitHubDefaultBranch != "" {
		base = s.cfg.GitHubDefaultBranch
	} else if s.defaultBranch != "" {
		base = s.defaultBranch
	}

	body := fmt.Sprintf("Fixes #%d\n\n---\n_AegisOps incident #%d · correlation `%s`_", issueID, issueID, issue.CorrelationID)
	created, err := s.github.CreatePullRequest(ctx, githubclient.CreatePRInput{
		Owner: owner,
		Repo:  name,
		Title: issue.Title,
		Head:  branch,
		Base:  base,
		Body:  body,
	})
	if err != nil {
		msg := err.Error()
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "head") || strings.Contains(msg, "422") || strings.Contains(lower, "not found") {
			return nil, fmt.Errorf("branch %s not on GitHub yet — push it first, then open PR (or rely on push webhook)", branch)
		}
		return nil, err
	}

	s.updateIssueLifecycle(ctx, issueID, "pr_created", fmt.Sprintf("PR #%d opened: %s", created.Number, created.URL))
	slog.Info("pull request opened", "issueId", issueID, "number", created.Number, "url", created.URL)
	return &OpenPRResult{Number: created.Number, URL: created.URL, Branch: branch}, nil
}
