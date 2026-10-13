package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kkjayakumar/AegIsOps/internal/clients"
	"github.com/kkjayakumar/AegIsOps/internal/orchestrator"
	"github.com/kkjayakumar/AegIsOps/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	issues       *store.IssueStore
	copilot      *clients.CopilotClient
	orchestrator *orchestrator.Service
}

func New(issues *store.IssueStore, copilot *clients.CopilotClient, orch *orchestrator.Service) *Server {
	return &Server{issues: issues, copilot: copilot, orchestrator: orch}
}

func (s *Server) MCPServer() *server.MCPServer {
	mcpServer := server.NewMCPServer(
		"aegisops",
		"0.3.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)
	s.registerTools(mcpServer)
	return mcpServer
}

func (s *Server) registerTools(mcpServer *server.MCPServer) {
	tools := []struct {
		tool    mcp.Tool
		handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{mcp.NewTool("get_incident_bundle",
			mcp.WithDescription("Fetch markdown telemetry bundle for an issue"),
			mcp.WithNumber("issue_id", mcp.Required())), s.handleGetIncidentBundle},
		{mcp.NewTool("start_fix_from_issue",
			mcp.WithDescription("Mark fix issue in_progress and return bundle"),
			mcp.WithNumber("issue_id", mcp.Required())), s.handleStartFixFromIssue},
		{mcp.NewTool("start_feature_from_issue",
			mcp.WithDescription("Mark feat issue in_progress and return bundle"),
			mcp.WithNumber("issue_id", mcp.Required())), s.handleStartFeatureFromIssue},
		{mcp.NewTool("open_pr_from_issue",
			mcp.WithDescription("Open GitHub PR for aegis/fix-<id> branch (also auto-opens on git push)"),
			mcp.WithNumber("issue_id", mcp.Required())), s.handleOpenPRFromIssue},
		{mcp.NewTool("get_recent_issues",
			mcp.WithDescription("List recent open issues"),
			mcp.WithString("type", mcp.Description("filter: fix or feat")),
			mcp.WithString("status", mcp.Description("filter by status"))), s.handleGetRecentIssues},
		{mcp.NewTool("get_live_logs",
			mcp.WithDescription("Fetch Docker container logs via DevOps Copilot"),
			mcp.WithString("container_id", mcp.Required()),
			mcp.WithNumber("tail", mcp.Description("line count, default 200"))), s.handleGetLiveLogs},
		{mcp.NewTool("get_sentry_issue",
			mcp.WithDescription("Fetch Sentry issue detail from DevOps Copilot"),
			mcp.WithString("sentry_issue_id", mcp.Required())), s.handleGetSentryIssue},
	}
	for _, t := range tools {
		mcpServer.AddTool(t.tool, t.handler)
	}
}

func (s *Server) GetIncidentBundle(ctx context.Context, issueID int64) (string, error) {
	if s == nil || s.issues == nil {
		return "", fmt.Errorf("database not available")
	}
	issue, err := s.issues.GetByID(ctx, issueID)
	if err != nil {
		return "", fmt.Errorf("issue not found: %w", err)
	}
	if issue.TelemetryBundle == nil || *issue.TelemetryBundle == "" {
		return "", fmt.Errorf("issue has no telemetry bundle")
	}
	return *issue.TelemetryBundle, nil
}

func (s *Server) StartFixFromIssue(ctx context.Context, issueID int64) (store.Issue, string, error) {
	return s.startIssue(ctx, issueID, "in_progress")
}

func (s *Server) StartFeatureFromIssue(ctx context.Context, issueID int64) (store.Issue, string, error) {
	return s.startIssue(ctx, issueID, "in_progress")
}

func (s *Server) startIssue(ctx context.Context, issueID int64, status string) (store.Issue, string, error) {
	if s == nil || s.issues == nil {
		return store.Issue{}, "", fmt.Errorf("database not available")
	}
	if err := s.issues.UpdateStatus(ctx, issueID, status); err != nil {
		return store.Issue{}, "", err
	}
	issue, err := s.issues.GetByID(ctx, issueID)
	if err != nil {
		return store.Issue{}, "", err
	}
	bundle := ""
	if issue.TelemetryBundle != nil {
		bundle = *issue.TelemetryBundle
	}
	return issue, bundle, nil
}

func fixBranch(issue store.Issue) string {
	kind := "fix"
	if issue.IssueType == "feat" {
		kind = "feat"
	}
	return fmt.Sprintf("aegis/%s-%d", kind, issue.ID)
}

func (s *Server) handleGetIncidentBundle(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	issueID, err := req.RequireInt("issue_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	bundle, err := s.GetIncidentBundle(ctx, int64(issueID))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(bundle), nil
}

func (s *Server) handleStartFixFromIssue(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.handleStart(ctx, req, "fix")
}

func (s *Server) handleStartFeatureFromIssue(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.handleStart(ctx, req, "feature")
}

func (s *Server) handleStart(ctx context.Context, req mcp.CallToolRequest, kind string) (*mcp.CallToolResult, error) {
	issueID, err := req.RequireInt("issue_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	issue, bundle, err := s.startIssue(ctx, int64(issueID), "in_progress")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	branch := fixBranch(issue)
	msg := fmt.Sprintf(`Started %s for issue #%d (%s).
Status: %s
Branch: %s

After implementing the fix, push the branch — AegisOps opens the PR automatically via GitHub webhook.
Or call open_pr_from_issue(issue_id=%d) after push.

--- Bundle ---

%s`, kind, issue.ID, issue.Title, issue.Status, branch, issue.ID, bundle)
	return mcp.NewToolResultText(msg), nil
}

func (s *Server) handleOpenPRFromIssue(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.orchestrator == nil {
		return mcp.NewToolResultError("orchestrator not configured"), nil
	}
	issueID, err := req.RequireInt("issue_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	result, err := s.orchestrator.OpenPRForIssue(ctx, int64(issueID))
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	msg := fmt.Sprintf("PR #%d opened: %s\nBranch: %s\nIssue status -> pr_created", result.Number, result.URL, result.Branch)
	return mcp.NewToolResultText(msg), nil
}

func (s *Server) handleGetRecentIssues(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.issues == nil {
		return mcp.NewToolResultError("database not available"), nil
	}
	issueType, _ := req.RequireString("type")
	status, _ := req.RequireString("status")
	issues, err := s.issues.List(ctx, issueType, status, 20)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b, _ := json.MarshalIndent(map[string]any{"issues": issues, "total": len(issues)}, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleGetLiveLogs(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	copilot := s.resolvedCopilot(ctx, "")
	if copilot == nil {
		return mcp.NewToolResultError("copilot not configured"), nil
	}
	containerID, err := req.RequireString("container_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	tail := 200
	if t, err := req.RequireInt("tail"); err == nil && t > 0 {
		tail = t
	}
	logs, err := copilot.GetContainerLogs(ctx, containerID, tail)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(logs), nil
}

func (s *Server) handleGetSentryIssue(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	copilot := s.resolvedCopilot(ctx, "")
	if copilot == nil {
		return mcp.NewToolResultError("copilot not configured"), nil
	}
	id, err := req.RequireString("sentry_issue_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	detail, err := copilot.GetIssue(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	b, _ := json.MarshalIndent(detail, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

// resolvedCopilot prefers workspace telemetry connector URLs when ready; else the process-global client.
func (s *Server) resolvedCopilot(ctx context.Context, preferredRepo string) *clients.CopilotClient {
	if s.orchestrator != nil {
		rt := s.orchestrator.ResolveRuntime(ctx, preferredRepo)
		if c := s.orchestrator.CopilotClientFor(rt); c != nil && c.BaseURL != "" {
			return c
		}
	}
	return s.copilot
}
