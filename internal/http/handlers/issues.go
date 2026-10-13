package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/broker"
	"github.com/kkjayakumar/AegIsOps/internal/config"
	"github.com/kkjayakumar/AegIsOps/internal/orchestrator"
	"github.com/kkjayakumar/AegIsOps/internal/store"
)

type IssuesHandler struct {
	Store        *store.IssueStore
	Deployments  *store.DeploymentStore
	Gateway      *store.GatewayStore
	Orchestrator *orchestrator.Service
	Config       *config.Config
}

type createIssueRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	IssueType   string `json:"issueType"`
}

func (h *IssuesHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	issueType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	issues, err := h.Store.List(ctx, issueType, status, 50)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list issues"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"issues": issues, "total": len(issues)})
}

func (h *IssuesHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	issue, err := h.Store.GetByID(ctx, id)
	if err != nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "issue not found"})
		return
	}
	links, _ := h.Store.GetTelemetryLinks(ctx, id)
	var recentDeployments []store.Deployment
	if h.Deployments != nil {
		recentDeployments = orchestrator.RecentDeploymentsForIssue(ctx, h.Deployments, issue.CorrelationID, 5)
	}
	kind := "fix"
	if issue.IssueType == "feat" {
		kind = "feat"
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"issue":             issue,
		"telemetryLinks":    links,
		"recentDeployments": recentDeployments,
		"fixBranch":         fmt.Sprintf("aegis/%s-%d", kind, issue.ID),
	})
}

func (h *IssuesHandler) Bundle(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	issue, err := h.Store.GetByID(ctx, id)
	if err != nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "issue not found"})
		return
	}
	bundle := ""
	if issue.TelemetryBundle != nil {
		bundle = *issue.TelemetryBundle
	}
	if h.Deployments != nil {
		deps := orchestrator.RecentDeploymentsForIssue(ctx, h.Deployments, issue.CorrelationID, 5)
		bundle = broker.AppendCICDToBundle(bundle, deps)
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(bundle))
}

func (h *IssuesHandler) GatewayApprove(w http.ResponseWriter, r *http.Request) {
	h.gatewayAction(w, r, "approve")
}

func (h *IssuesHandler) GatewayReject(w http.ResponseWriter, r *http.Request) {
	h.gatewayAction(w, r, "reject")
}

func (h *IssuesHandler) gatewayAction(w http.ResponseWriter, r *http.Request, expected string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Orchestrator == nil || h.Gateway == nil || h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gateway unavailable"})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	token, err := h.Gateway.ActiveToken(ctx, id, expected)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	issueID, action, err := h.Orchestrator.HandleGatewayAction(ctx, token)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if action != expected {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "token action mismatch"})
		return
	}
	issue, _ := h.Store.GetByID(ctx, issueID)
	msg := "Issue approved — ready for IDE via MCP"
	if action == "reject" {
		msg = "Issue rejected and closed"
	}
	WriteJSON(w, http.StatusOK, map[string]any{"issue": issue, "action": action, "message": msg})
}

func (h *IssuesHandler) StartFix(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	issue, err := h.Store.GetByID(ctx, id)
	if err != nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "issue not found"})
		return
	}

	requireGateway := h.Config != nil && h.Config.RequireGatewayApproval
	if requireGateway {
		switch strings.ToLower(issue.Status) {
		case "in_progress", "pr_created", "fixed":
			// already past gateway (or already working)
		default:
			WriteJSON(w, http.StatusConflict, map[string]any{
				"error":   "gateway approval required before start-fix (AEGISOPS_REQUIRE_GATEWAY=true)",
				"issue":   issue,
				"nextStep": "Approve via Teams/Slack gateway link or POST /api/v1/issues/{id}/gateway/approve with token",
			})
			return
		}
	}

	if issue.Status != "in_progress" && issue.Status != "pr_created" && issue.Status != "fixed" {
		if err := h.Store.UpdateStatus(ctx, id, "in_progress"); err != nil {
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update status"})
			return
		}
		issue, _ = h.Store.GetByID(ctx, id)
	}

	kind := "fix"
	if issue.IssueType == "feat" {
		kind = "feat"
	}
	branch := fmt.Sprintf("aegis/%s-%d", kind, id)

	rt := orchestrator.RuntimeSettings{AgentRoute: "mcp", Source: "config"}
	if h.Orchestrator != nil {
		rt = h.Orchestrator.ResolveRuntime(ctx, issue.GitHubRepo)
	}

	nextSteps := []string{
		fmt.Sprintf("Create/checkout branch %s", branch),
		"Implement the fix using the issue telemetry bundle",
		"Push the branch — AegisOps opens a PR when the GitHub webhook fires",
	}
	switch strings.ToLower(rt.AgentRoute) {
	case "api":
		nextSteps = append([]string{
			"issue_agent route=api — use Workspace LLM chat or POST /api/v1/workspace/environments/{id}/llm/complete with this issueId",
		}, nextSteps...)
	default:
		nextSteps = append([]string{
			"issue_agent route=mcp — in Cursor call MCP start_fix_from_issue with this issue id",
		}, nextSteps...)
	}
	if !rt.AgentReady {
		nextSteps = append(nextSteps, "Connect AI agent stage (Connections → Apply platform defaults) so route/provider are explicit")
	}
	if !rt.GitReady && rt.Repo == "" {
		nextSteps = append(nextSteps, "Connect git stage or set GITHUB_DEFAULT_REPO")
	}

	msg := fmt.Sprintf("ready for IDE — branch %s; push opens PR automatically", branch)
	if rt.Source == "connector" {
		msg += fmt.Sprintf(" (repo from workspace connector: %s)", rt.Repo)
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"issue":         issue,
		"message":       msg,
		"branch":        branch,
		"repo":          rt.Repo,
		"agentRoute":    rt.AgentRoute,
		"agentProvider": rt.AgentProvider,
		"agentReady":    rt.AgentReady,
		"runtimeSource": rt.Source,
		"environmentId": rt.EnvironmentID,
		"nextSteps":     nextSteps,
		"gatewayBypass": !requireGateway,
		"note":          "Set AEGISOPS_REQUIRE_GATEWAY=true to block start-fix until gateway approve.",
	})
}

func (h *IssuesHandler) Close(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	issue, err := h.Store.GetByID(ctx, id)
	if err != nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "issue not found"})
		return
	}
	if issue.Status == "fixed" || issue.Status == "closed" {
		WriteJSON(w, http.StatusOK, map[string]any{"issue": issue, "message": "issue already closed"})
		return
	}
	if err := h.Store.UpdateStatus(ctx, id, "closed"); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update status"})
		return
	}
	issue, _ = h.Store.GetByID(ctx, id)
	WriteJSON(w, http.StatusOK, map[string]any{"issue": issue, "message": "issue closed"})
}

func (h *IssuesHandler) OpenPR(w http.ResponseWriter, r *http.Request) {
	if h.Orchestrator == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "orchestrator unavailable"})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	result, err := h.Orchestrator.OpenPRForIssue(ctx, id)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	issue, _ := h.Store.GetByID(ctx, id)
	WriteJSON(w, http.StatusOK, map[string]any{
		"issue":       issue,
		"pullRequest": result,
		"message":     fmt.Sprintf("PR #%d opened — merges automatically update issue status", result.Number),
	})
}

func (h *IssuesHandler) StartFeature(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.Store.UpdateStatus(ctx, id, "in_progress"); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update status"})
		return
	}
	issue, _ := h.Store.GetByID(ctx, id)
	WriteJSON(w, http.StatusOK, map[string]any{"issue": issue, "message": "ready for IDE feature work via MCP"})
}

func (h *IssuesHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	var req createIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Title == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "title required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	correlationID := fmt.Sprintf("feat_manual_%d", time.Now().Unix())
	var issue store.Issue
	var err error
	if req.IssueType == "feat" {
		issue, err = h.Store.CreateFeat(ctx, req.Title, req.Description, correlationID)
	} else {
		title := req.Title
		if req.IssueType == "fix" && len(title) >= 4 && title[:4] != "fix:" {
			title = "fix: " + title
		}
		bundle := req.Description
		issue, err = h.Store.Create(ctx, store.Issue{
			Title:           title,
			IssueType:       req.IssueType,
			Description:     &req.Description,
			Status:          "detected",
			CorrelationID:   correlationID,
			TelemetryBundle: &bundle,
		})
	}
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create issue"})
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{"issue": issue})
}
