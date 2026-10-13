package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/clients"
	githubclient "github.com/kkjayakumar/AegIsOps/internal/githubclient"
)

func (s *Service) verifyDeploy(ctx context.Context, depID int64, repo, runID, workflow, branch, sha string) {
	rt := s.resolveRuntime(ctx, repo)
	verifyURL := rt.VerifyURL
	if verifyURL == "" {
		slog.Warn("deploy verify skipped", "runId", runID, "workflow", workflow, "reason", "no verify URL from connectors or DEPLOY_VERIFY_URL")
		return
	}

	healthOK, healthMsg := checkHTTPHealth(ctx, verifyURL)
	if !healthOK {
		_ = s.deployments.UpdateVerify(ctx, depID, "failed", healthMsg)
		s.triggerRollback(ctx, rt, runID, sha, workflow, healthMsg)
		_ = s.teams.Send(ctx, "Deploy verification FAILED", fmt.Sprintf(
			"Workflow **%s** succeeded but health check failed.\n\n%s\n\nRollback triggered.",
			workflow, healthMsg,
		), "critical")
		slog.Error("deploy health check failed", "runId", runID, "msg", healthMsg, "url", verifyURL, "source", rt.Source)
		return
	}

	var copilotMonitorID *int
	name := fmt.Sprintf("deploy-verify-%s", runID)
	monitor, err := s.CopilotClientFor(rt).CreateUptimeMonitor(ctx, clients.UptimeMonitorInput{
		Name:     name,
		URL:      verifyURL,
		Interval: 60,
		Enabled:  1,
	})
	if err != nil {
		slog.Warn("copilot monitor registration failed", "error", err)
	} else if id := intFromAny(monitor["id"]); id > 0 {
		copilotMonitorID = &id
	}

	ttl := time.Duration(s.cfg.DeployVerifyTTLMin) * time.Minute
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if s.sentinel != nil {
		if _, err := s.sentinel.Create(ctx, depID, copilotMonitorID, verifyURL, "passed", time.Now().Add(ttl)); err != nil {
			slog.Warn("sentinel monitor create failed", "deploymentId", depID, "error", err)
		}
	}
	_ = s.deployments.UpdateVerify(ctx, depID, "passed", healthMsg)
	_ = s.teams.Send(ctx, "Post-deploy verification passed", fmt.Sprintf(
		"Workflow **%s** on `%s` commit `%s` passed health check.\nTemp monitor expires in %d minutes.",
		workflow, branch, shortSHA(sha), int(ttl.Minutes()),
	), "info")
	slog.Info("post-deploy verification passed", "runId", runID, "workflow", workflow, "branch", branch, "deploymentId", depID, "url", verifyURL, "source", rt.Source)
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func (s *Service) triggerRollback(ctx context.Context, rt RuntimeSettings, runID, sha, workflow, reason string) {
	repo := rt.Repo
	rollbackWF := strings.TrimSpace(rt.RollbackWorkflow)
	if rollbackWF == "" && s.cfg != nil {
		rollbackWF = strings.TrimSpace(s.cfg.RollbackWorkflow)
	}
	branch := strings.TrimSpace(rt.DefaultBranch)
	if branch == "" {
		branch = "main"
	}
	if rollbackWF != "" && s.github.Enabled() && repo != "" {
		owner, name, err := githubclient.ParseRepo(repo)
		if err == nil {
			inputs := map[string]string{"run_id": runID, "sha": sha, "reason": reason}
			if err := s.github.TriggerWorkflowDispatch(ctx, owner, name, rollbackWF, branch, inputs); err != nil {
				slog.Error("github rollback dispatch failed", "error", err, "workflow", rollbackWF)
			} else {
				slog.Info("rollback workflow dispatched", "workflow", rollbackWF, "repo", repo, "branch", branch, "source", rt.Source)
				return
			}
		}
	}
	if s.cfg == nil || s.cfg.RollbackWebhookURL == "" {
		return
	}
	payload := fmt.Sprintf(`{"repo":%q,"run_id":%q,"sha":%q,"workflow":%q,"reason":%q,"rollback_workflow":%q}`,
		repo, runID, sha, workflow, reason, rollbackWF)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.RollbackWebhookURL, bytes.NewReader([]byte(payload)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("rollback webhook failed", "error", err)
		return
	}
	res.Body.Close()
}

func checkHTTPHealth(ctx context.Context, url string) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err.Error()
	}
	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("health check request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return true, fmt.Sprintf("HTTP %d OK", res.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	return false, fmt.Sprintf("HTTP %d: %s", res.StatusCode, string(body))
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}
