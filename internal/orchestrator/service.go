package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/broker"
	"github.com/kkjayakumar/AegIsOps/internal/clients"
	"github.com/kkjayakumar/AegIsOps/internal/config"
	"github.com/kkjayakumar/AegIsOps/internal/correlation"
	githubclient "github.com/kkjayakumar/AegIsOps/internal/githubclient"
	"github.com/kkjayakumar/AegIsOps/internal/slack"
	"github.com/kkjayakumar/AegIsOps/internal/store"
	"github.com/kkjayakumar/AegIsOps/internal/teams"
)

type Service struct {
	cfg           *config.Config
	issues        *store.IssueStore
	webhooks      *store.WebhookStore
	deployments   *store.DeploymentStore
	sentinel      *store.SentinelStore
	gateway       *store.GatewayStore
	workspace     *store.WorkspaceStore
	copilot       *clients.CopilotClient
	spotting      *clients.SpottingClient
	github        *githubclient.Client
	githubInitErr error
	teams         *teams.Notifier
	slack         *slack.Notifier
	publicURL     string
	defaultRepo   string
	defaultBranch string
}

func New(
	cfg *config.Config,
	issues *store.IssueStore,
	webhooks *store.WebhookStore,
	deployments *store.DeploymentStore,
	sentinel *store.SentinelStore,
	gateway *store.GatewayStore,
	workspace *store.WorkspaceStore,
) *Service {
	var appAuth *githubclient.AppAuth
	var initErr error
	if cfg != nil && cfg.GitHubAppID != "" && cfg.GitHubInstallationID != "" && cfg.GitHubPrivateKeyPath != "" {
		auth, err := githubclient.NewAppAuth(cfg.GitHubAppID, cfg.GitHubInstallationID, cfg.GitHubPrivateKeyPath)
		if err != nil {
			slog.Error("github app auth init failed — issue/PR automation disabled", "error", err, "keyPath", cfg.GitHubPrivateKeyPath)
			initErr = err
		} else {
			appAuth = auth
			slog.Info("github app auth ready", "installationId", cfg.GitHubInstallationID)
		}
	}
	return &Service{
		cfg:           cfg,
		issues:        issues,
		webhooks:      webhooks,
		deployments:   deployments,
		sentinel:      sentinel,
		gateway:       gateway,
		workspace:     workspace,
		copilot:       clients.NewCopilotClient(cfg.CopilotAPIURL, cfg.CopilotServiceToken),
		spotting:      clients.NewSpottingClient(cfg.SpottingAPIURL, cfg.SpottingServiceToken),
		github:        githubclient.NewClient(cfg.GitHubToken, appAuth),
		githubInitErr: initErr,
		teams:         teams.NewNotifier(cfg.TeamsWebhookURL),
		slack:         slack.NewNotifier(cfg.SlackWebhookURL),
		publicURL:     cfg.PublicURL,
		defaultRepo:   cfg.GitHubDefaultRepo,
		defaultBranch: cfg.GitHubDefaultBranch,
	}
}

func (s *Service) ProcessWebhookEvent(ctx context.Context, eventID int64, source string, payload []byte) {
	if s == nil || s.issues == nil || s.webhooks == nil {
		return
	}

	parsed := correlation.ParseWebhookEvent(source, payload)
	if parsed.CorrelationID == "" {
		slog.Warn("webhook missing correlation id", "source", source, "eventId", eventID)
		_ = s.webhooks.MarkProcessed(ctx, eventID)
		return
	}

	events, err := s.webhooks.ListByCorrelationID(ctx, parsed.CorrelationID)
	if err != nil {
		slog.Error("list correlation events failed", "error", err)
		return
	}

	fingerprint := correlation.FingerprintFromEvents(events)

	existing, err := s.issues.FindOpenByCorrelationID(ctx, parsed.CorrelationID)
	if err != nil {
		slog.Error("find issue failed", "error", err)
		return
	}
	if existing == nil && fingerprint != "" && !broker.IsDogfoodCorrelation(parsed.CorrelationID) {
		existing, err = s.issues.FindOpenByFingerprint(ctx, fingerprint)
		if err != nil {
			slog.Error("find issue by fingerprint failed", "error", err)
			return
		}
		if existing != nil {
			slog.Info("deduplicated incident by fingerprint", "fingerprint", fingerprint, "issueId", existing.ID)
		}
	}
	if existing != nil {
		s.mergeIntoExisting(ctx, existing, events)
		_ = s.webhooks.MarkProcessed(ctx, eventID)
		return
	}

	hasCopilot, hasSpotting := false, false
	var sentryID, spottingID string
	for _, ev := range events {
		p := correlation.ParseWebhookEvent(ev.Source, ev.Payload)
		if p.SentryIssueID != "" {
			sentryID = p.SentryIssueID
			hasCopilot = true
		}
		if p.SpottingReportID != "" {
			spottingID = p.SpottingReportID
			hasSpotting = true
		}
	}

	shouldCreate := hasCopilot && hasSpotting
	if !shouldCreate && hasSpotting && parsed.Source == "spotting" && parsed.Severity == "error" && !hasCopilot {
		shouldCreate = true // frontend-only incident
	}
	if !shouldCreate {
		slog.Info("waiting for correlation", "correlationId", parsed.CorrelationID, "copilot", hasCopilot, "spotting", hasSpotting)
		return
	}

	status := "detected"
	if hasCopilot && hasSpotting {
		status = "correlated"
	}

	title := broker.DeriveTitle(events)
	if !strings.HasPrefix(title, "fix:") {
		title = "fix: " + title
	}

	recentDep := deploymentForBundle(ctx, s.deployments, parsed.CorrelationID)
	rt := s.resolveRuntime(ctx, s.defaultRepo)
	repo := rt.Repo

	bundle := broker.BuildBundle(broker.BundleInput{
		CorrelationID:     parsed.CorrelationID,
		SentryIssueID:     sentryID,
		SpottingReportID:  spottingID,
		Copilot:           s.CopilotClientFor(rt),
		Spotting:          s.SpottingClientFor(rt),
		WebhookEvents:     events,
		RecentDeployment:  recentDep,
		IncludeDeployment: recentDep != nil,
		GitHubClient:      s.github,
		GitHubRepo:        repo,
	})

	issue, err := s.issues.Create(ctx, store.Issue{
		GitHubRepo:      repo,
		Title:           title,
		IssueType:       "fix",
		Status:          status,
		CorrelationID:   parsed.CorrelationID,
		Fingerprint:     fingerprint,
		TelemetryBundle: &bundle,
	})
	if err != nil {
		slog.Error("create aegis issue failed", "error", err)
		return
	}

	if sentryID != "" || spottingID != "" {
		var sID, spID *string
		if sentryID != "" {
			sID = &sentryID
		}
		if spottingID != "" {
			spID = &spottingID
		}
		_ = s.issues.AddTelemetryLink(ctx, issue.ID, spID, sID)
	}

	if s.github.Enabled() && repo != "" {
		owner, name, err := githubclient.ParseRepo(repo)
		if err == nil {
			ghBody := bundle + fmt.Sprintf("\n\n---\n_AegisOps incident #%d · correlation `%s`_", issue.ID, parsed.CorrelationID)
			if result, err := s.createGitHubIssue(ctx, githubclient.CreateIssueInput{
				Owner:  owner,
				Repo:   name,
				Title:  title,
				Body:   ghBody,
				Labels: githubIssueLabels(parsed.CorrelationID),
			}); err == nil && result != nil {
				_ = s.issues.UpdateGitHubIssueNumber(ctx, issue.ID, repo, result.Number)
				slog.Info("github issue created", "number", result.Number, "url", result.URL)
			} else {
				slog.Error("github issue create failed", "error", err)
			}
		}
	}

	dashboardURL := strings.TrimSuffix(s.publicURL, "/") + fmt.Sprintf("/issues/%d", issue.ID)
	s.notifyGateway(ctx, issue.ID, title, parsed.CorrelationID, dashboardURL)
	notifyMsg := fmt.Sprintf("**%s**\n\nCorrelation: `%s`\n\n[Open in AegisOps dashboard](%s)", title, parsed.CorrelationID, dashboardURL)
	// Power Automate gets one approval payload (with links); skip duplicate alert card.
	if s.cfg == nil || !teams.IsPowerAutomateURL(s.cfg.TeamsWebhookURL) {
		_ = s.teams.Send(ctx, "AegisOps: new fix issue", notifyMsg, "warning")
	}
	_ = s.slack.Send(ctx, "AegisOps: new fix issue", notifyMsg, "warning")

	for _, ev := range events {
		_ = s.webhooks.MarkProcessed(ctx, ev.ID)
	}

	slog.Info("incident created", "issueId", issue.ID, "correlationId", parsed.CorrelationID, "status", status)
}

func (s *Service) HandleWorkflowRun(ctx context.Context, repo, runID, workflow, sha, branch, status, logs string) {
	if s.deployments == nil {
		return
	}
	candidate := store.Deployment{
		GitHubRepo:   repo,
		RunID:        runID,
		WorkflowName: workflow,
		CommitSHA:    sha,
		Branch:       branch,
		Status:       status,
		FailureLogs:  optionalString(logs),
	}
	if IsSyntheticDeployment(candidate) {
		if s.cfg != nil && s.cfg.IsProduction() {
			slog.Info("ignored synthetic workflow_run in production", "runId", runID, "sha", sha, "workflow", workflow)
			return
		}
		slog.Warn("recording synthetic workflow_run (non-production only)", "runId", runID, "sha", sha)
	}
	dep, err := s.deployments.Upsert(ctx, candidate)
	if err != nil {
		slog.Error("deployment upsert failed", "error", err)
		return
	}

	severity := "info"
	title := fmt.Sprintf("CI %s: %s", status, workflow)
	msg := fmt.Sprintf("Repo **%s**\nBranch `%s`\nCommit `%s`", repo, branch, sha)
	if status == "failure" {
		severity = "critical"
		title = fmt.Sprintf("CI failed: %s", workflow)
		if logs != "" {
			lines := strings.Split(logs, "\n")
			if len(lines) > 10 {
				lines = lines[len(lines)-10:]
			}
			msg += "\n\n```\n" + strings.Join(lines, "\n") + "\n```"
		}
		correlationID := "ci_" + runID
		existing, _ := s.issues.FindOpenByCorrelationID(ctx, correlationID)
		rt := s.resolveRuntime(ctx, repo)
		if rt.Repo != "" {
			repo = rt.Repo
		}
		if existing == nil && repo != "" {
			bundle := fmt.Sprintf("# CI Failure\n\nWorkflow: %s\nBranch: %s\nCommit: %s\n\n```\n%s\n```", workflow, branch, sha, logs)
			fixTitle := fmt.Sprintf("fix: CI failure in %s", workflow)
			issue, err := s.issues.Create(ctx, store.Issue{
				GitHubRepo:      repo,
				Title:           fixTitle,
				IssueType:       "fix",
				Status:          "detected",
				CorrelationID:   correlationID,
				TelemetryBundle: &bundle,
			})
			if err == nil && s.github.Enabled() {
				owner, name, _ := githubclient.ParseRepo(repo)
				if result, err := s.createGitHubIssue(ctx, githubclient.CreateIssueInput{
					Owner: owner, Repo: name, Title: fixTitle, Body: bundle, Labels: []string{"aegisops-detected", "ci"},
				}); err == nil && result != nil {
					_ = s.issues.UpdateGitHubIssueNumber(ctx, issue.ID, repo, result.Number)
				}
			}
		}
	}
	_ = s.teams.Send(ctx, title, msg, severity)
	slog.Info("deployment recorded", "id", dep.ID, "status", status)
	if status == "success" {
		s.resolveCIFailuresForBranch(ctx, workflow, branch, sha)
		depID := dep.ID
		go func() {
			vctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			s.verifyDeploy(vctx, depID, repo, runID, workflow, branch, sha)
		}()
	}
}

func optionalString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func mergeWebhookEvents(prior, incoming []store.WebhookEvent) []store.WebhookEvent {
	seen := make(map[int64]struct{}, len(prior)+len(incoming))
	out := make([]store.WebhookEvent, 0, len(prior)+len(incoming))
	appendUnique := func(ev store.WebhookEvent) {
		if _, ok := seen[ev.ID]; ok {
			return
		}
		seen[ev.ID] = struct{}{}
		out = append(out, ev)
	}
	for _, ev := range prior {
		appendUnique(ev)
	}
	for _, ev := range incoming {
		appendUnique(ev)
	}
	return out
}

func telemetryIDsFromEvents(events []store.WebhookEvent) (hasCopilot, hasSpotting bool, sentryID, spottingID string) {
	for _, ev := range events {
		p := correlation.ParseWebhookEvent(ev.Source, ev.Payload)
		if p.SentryIssueID != "" {
			sentryID = p.SentryIssueID
			hasCopilot = true
		}
		if p.SpottingReportID != "" {
			spottingID = p.SpottingReportID
			hasSpotting = true
		}
	}
	return hasCopilot, hasSpotting, sentryID, spottingID
}

func backfillTelemetryIDsFromLinks(links []store.TelemetryLink, hasCopilot, hasSpotting bool, sentryID, spottingID string) (bool, bool, string, string) {
	for _, l := range links {
		if sentryID == "" && l.SentryIssueID != nil && *l.SentryIssueID != "" {
			sentryID = *l.SentryIssueID
			hasCopilot = true
		}
		if spottingID == "" && l.SpottingReportID != nil && *l.SpottingReportID != "" {
			spottingID = *l.SpottingReportID
			hasSpotting = true
		}
	}
	return hasCopilot, hasSpotting, sentryID, spottingID
}

func (s *Service) mergeIntoExisting(ctx context.Context, existing *store.Issue, events []store.WebhookEvent) {
	mergedEvents := events
	if existing.CorrelationID != "" {
		if prior, err := s.webhooks.ListByCorrelationID(ctx, existing.CorrelationID); err == nil && len(prior) > 0 {
			mergedEvents = mergeWebhookEvents(prior, events)
		}
	}

	hasCopilot, hasSpotting, sentryID, spottingID := telemetryIDsFromEvents(mergedEvents)
	if links, err := s.issues.GetTelemetryLinks(ctx, existing.ID); err == nil {
		hasCopilot, hasSpotting, sentryID, spottingID = backfillTelemetryIDsFromLinks(links, hasCopilot, hasSpotting, sentryID, spottingID)
	}
	if hasCopilot && hasSpotting && existing.Status != "correlated" {
		_ = s.issues.UpdateStatus(ctx, existing.ID, "correlated")
	}
	var sID, spID *string
	if sentryID != "" {
		sID = &sentryID
	}
	if spottingID != "" {
		spID = &spottingID
	}
	if sID != nil || spID != nil {
		_ = s.issues.AddTelemetryLink(ctx, existing.ID, spID, sID)
	}
	recentDep := deploymentForBundle(ctx, s.deployments, existing.CorrelationID)
	rt := s.resolveRuntime(ctx, existing.GitHubRepo)
	bundle := broker.BuildBundle(broker.BundleInput{
		CorrelationID:     existing.CorrelationID,
		SentryIssueID:     sentryID,
		SpottingReportID:  spottingID,
		Copilot:           s.CopilotClientFor(rt),
		Spotting:          s.SpottingClientFor(rt),
		WebhookEvents:     mergedEvents,
		RecentDeployment:  recentDep,
		IncludeDeployment: recentDep != nil,
		GitHubClient:      s.github,
		GitHubRepo:        existing.GitHubRepo,
	})
	title := broker.DeriveTitle(mergedEvents)
	if !strings.HasPrefix(title, "fix:") {
		title = "fix: " + title
	}
	_ = s.issues.UpdateBundle(ctx, existing.ID, bundle)
	shouldUpdateTitle := (hasCopilot && hasSpotting) || broker.IsDogfoodCorrelation(existing.CorrelationID)
	if shouldUpdateTitle {
		_ = s.issues.UpdateTitle(ctx, existing.ID, title)
		s.syncGitHubIssue(ctx, existing, title, bundle)
	}
	for _, ev := range events {
		_ = s.webhooks.MarkProcessed(ctx, ev.ID)
	}
	slog.Info("merged telemetry into existing issue", "issueId", existing.ID, "correlated", hasCopilot && hasSpotting)
}

func (s *Service) syncGitHubIssue(ctx context.Context, issue *store.Issue, title, bundle string) {
	if s == nil || issue == nil || !s.github.Enabled() || issue.GitHubIssueNumber == nil || issue.GitHubRepo == "" {
		return
	}
	owner, name, err := githubclient.ParseRepo(issue.GitHubRepo)
	if err != nil {
		return
	}
	ghBody := bundle + fmt.Sprintf("\n\n---\n_AegisOps incident #%d · correlation `%s`_", issue.ID, issue.CorrelationID)
	if err := s.github.UpdateIssue(ctx, githubclient.UpdateIssueInput{
		Owner:  owner,
		Repo:   name,
		Number: *issue.GitHubIssueNumber,
		Title:  title,
		Body:   ghBody,
	}); err != nil {
		slog.Warn("github issue update failed", "number", *issue.GitHubIssueNumber, "error", err)
	} else {
		slog.Info("github issue updated", "number", *issue.GitHubIssueNumber)
	}
}

// GitHubRuntimeStatus reports whether the GitHub API client is usable (not just env/config).
func (s *Service) GitHubRuntimeStatus() map[string]any {
	if s == nil || s.github == nil {
		return map[string]any{"runtimeEnabled": false}
	}
	out := map[string]any{"runtimeEnabled": s.github.Enabled()}
	if s.githubInitErr != nil {
		out["initError"] = s.githubInitErr.Error()
	}
	return out
}

func (s *Service) resolveCIFailuresForBranch(ctx context.Context, workflow, branch, sha string) {
	if s == nil || s.issues == nil || workflow != "CI" || branch == "" {
		return
	}
	open, err := s.issues.ListOpenByCorrelationPrefix(ctx, "ci_", 50)
	if err != nil {
		slog.Warn("list open CI issues failed", "error", err)
		return
	}
	needle := "Branch: " + branch
	for _, issue := range open {
		if issue.TelemetryBundle == nil || !strings.Contains(*issue.TelemetryBundle, needle) {
			continue
		}
		s.updateIssueLifecycle(ctx, issue.ID, "fixed", fmt.Sprintf("CI succeeded on `%s` at `%s` — recovered issue #%d", branch, sha, issue.ID))
	}
}
