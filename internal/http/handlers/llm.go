package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/llm"
	"github.com/kkjayakumar/AegIsOps/internal/store"
	"github.com/kkjayakumar/AegIsOps/internal/workspace"
)

type LLMHandler struct {
	Workspace  *store.WorkspaceStore
	Graph      *store.GraphStore
	Issues     *store.IssueStore
	RBAC       *store.RBACStore
	SecretsKey string
	Client     *llm.Client
}

func (h *LLMHandler) cipher() (*workspace.Cipher, error) {
	if workspace.IsInsecureDevKey(h.SecretsKey) {
		return workspace.NewCipherWithOptions(h.SecretsKey, workspace.Options{AllowDevFallback: true})
	}
	return workspace.NewCipherStrict(h.SecretsKey)
}

func (h *LLMHandler) resolveAPIKey(ctx context.Context, envID int, provider llm.Provider) (string, error) {
	ciph, err := h.cipher()
	if err != nil {
		return "", err
	}
	keyName := llm.SecretKeyFor(provider)
	ct, err := h.Workspace.GetSecretCiphertext(ctx, envID, keyName)
	if err != nil {
		return "", fmt.Errorf("missing secret %s — add it under Organization settings → Models & API keys for this environment", keyName)
	}
	return ciph.Decrypt(ct)
}

func (h *LLMHandler) Status(w http.ResponseWriter, r *http.Request) {
	if h.Workspace == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	keys, err := h.Workspace.ListSecretKeyNames(ctx, envID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list secrets"})
		return
	}
	providers := llm.DetectAvailableProviders(keys)
	modes := []map[string]any{
		{"id": "api", "label": "LLM API keys", "description": "Call OpenAI / Claude / Google / Cursor API directly — no IDE required"},
		{"id": "mcp", "label": "IDE / MCP", "description": "Optional Cursor MCP path when an agent is connected"},
	}
	graphNodes := 0
	if h.Graph != nil {
		if snap, gerr := h.Graph.Snapshot(ctx, envID); gerr == nil {
			graphNodes = snap.Stats.NodeCount
		}
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"providers":         providers,
		"secretKeysPresent": keys,
		"modes":             modes,
		"defaultRoute":      "api",
		"graphNodeCount":    graphNodes,
		"recommendedModels": map[string]string{
			"openai":    llm.DefaultModel(llm.ProviderOpenAI),
			"anthropic": llm.DefaultModel(llm.ProviderAnthropic),
			"google":    llm.DefaultModel(llm.ProviderGoogle),
			"cursor":    llm.DefaultModel(llm.ProviderCursor),
		},
	})
}

func (h *LLMHandler) Complete(w http.ResponseWriter, r *http.Request) {
	if h.Workspace == nil || h.Graph == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	var body struct {
		Provider       string `json:"provider"`
		Model          string `json:"model"`
		Prompt         string `json:"prompt"`
		IssueID        *int   `json:"issueId"`
		ProjectID      int    `json:"projectId"`
		UseGraph       *bool  `json:"useGraph"`
		MaxGraphTokens int    `json:"maxGraphTokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.Prompt = strings.TrimSpace(body.Prompt)
	if body.Prompt == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
		return
	}
	provider := llm.Provider(strings.ToLower(strings.TrimSpace(body.Provider)))
	if provider == "" {
		provider = llm.ProviderOpenAI
	}
	useGraph := true
	if body.UseGraph != nil {
		useGraph = *body.UseGraph
	}

	ctx, cancel := context.WithTimeout(r.Context(), 95*time.Second)
	defer cancel()

	// Strict isolation: when projectId is sent, env must belong to that project (403 otherwise).
	// Never fall back to another environment's graph.
	if body.ProjectID > 0 {
		if err := h.Workspace.AssertEnvironmentBelongsToProject(ctx, envID, body.ProjectID); err != nil {
			status := http.StatusForbidden
			if !errors.Is(err, store.ErrEnvironmentProjectMismatch) && !strings.Contains(err.Error(), "not found") {
				status = http.StatusBadRequest
			}
			WriteJSON(w, status, map[string]string{
				"error": "environment is not under the specified project — graph and chat context stay isolated per project/env",
			})
			return
		}
	}

	if h.RBAC != nil && body.ProjectID > 0 {
		uid := ActorUserID(r, h.RBAC)
		dec, err := h.RBAC.EffectiveAccess(ctx, uid, body.ProjectID)
		if err != nil {
			WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		if !dec.CanChat {
			WriteJSON(w, http.StatusForbidden, map[string]any{
				"error": "chat not allowed for your role or daily quota exceeded",
				"role":  dec.Role,
				"used":  dec.ChatUsedToday,
				"limit": dec.ChatLimitDaily,
			})
			return
		}
	}

	apiKey, err := h.resolveAPIKey(ctx, envID, provider)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var graphBlock string
	nodesUsed := 0
	graphAutoIngested := 0
	tokenNote := "Prompt used graph subgraph only — not the full repository."
	if useGraph {
		nodes, gerr := h.Graph.SubgraphForQuery(ctx, envID, body.Prompt, 14)
		if gerr == nil && len(nodes) == 0 {
			// Empty graph: seed compact project context before the LLM call.
			if n, ierr := seedEnvironmentGraph(ctx, h.Graph, h.Issues, envID); ierr == nil && n > 0 {
				graphAutoIngested = n
				nodes, gerr = h.Graph.SubgraphForQuery(ctx, envID, body.Prompt, 14)
			}
		}
		if gerr == nil {
			graphBlock, nodesUsed, _ = llm.BuildGraphContext(nodes, body.MaxGraphTokens)
		}
		if nodesUsed == 0 {
			tokenNote = "Graph had no matching nodes — reply without project subgraph. Run graph ingest from Workspace if context is thin."
		} else if graphAutoIngested > 0 {
			tokenNote = fmt.Sprintf("Auto-ingested %d graph nodes, then used a compact subgraph (not full source).", graphAutoIngested)
		}
	} else {
		tokenNote = "Graph context disabled for this request."
	}

	system := `You are AegisOps, an autonomous DevOps assistant.
Prefer the compact PROJECT CONTEXT GRAPH over inventing file contents.
Reply with concrete, actionable fix/feature guidance. Keep answers concise.`
	if graphBlock != "" {
		system = system + "\n\n" + graphBlock
	}

	userPrompt := body.Prompt
	// Issues are still global (no project/env FK). Never inject them into a project-scoped
	// chat — that would merge customer data across isolation boundaries.
	if body.IssueID != nil && h.Issues != nil && body.ProjectID <= 0 {
		if issue, ierr := h.Issues.GetByID(ctx, int64(*body.IssueID)); ierr == nil {
			desc := ""
			if issue.Description != nil {
				desc = *issue.Description
			}
			bundle := ""
			if issue.TelemetryBundle != nil {
				bundle = truncateRunes(*issue.TelemetryBundle, 2500)
			}
			userPrompt = fmt.Sprintf("Issue #%d [%s] %s\n\n%s\n\nTelemetry excerpt:\n%s\n\nUser request:\n%s",
				issue.ID, issue.IssueType, issue.Title, desc, bundle, body.Prompt)
		}
	}

	client := h.Client
	if client == nil {
		client = llm.NewClient()
	}
	resp, err := client.Complete(ctx, llm.Request{
		Provider:  provider,
		Model:     body.Model,
		APIKey:    apiKey,
		System:    system,
		User:      userPrompt,
		MaxTokens: 2048,
	})
	status := "ok"
	summary := truncateRunes(resp.Text, 500)
	if err != nil {
		status = "error"
		summary = err.Error()
		_, _ = h.Graph.RecordLLMRun(ctx, store.LLMRun{
			EnvironmentID:  envID,
			IssueID:        body.IssueID,
			Provider:       string(provider),
			Model:          body.Model,
			GraphNodesUsed: nodesUsed,
			Status:         status,
			ResponseSummary: summary,
		})
		WriteJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	if resp.PromptTokens == 0 {
		resp.PromptTokens = llm.Estimate(system + userPrompt)
	}
	if resp.CompletionTokens == 0 {
		resp.CompletionTokens = llm.Estimate(resp.Text)
	}
	run, _ := h.Graph.RecordLLMRun(ctx, store.LLMRun{
		EnvironmentID:    envID,
		IssueID:          body.IssueID,
		Provider:         string(resp.Provider),
		Model:            resp.Model,
		PromptTokens:     resp.PromptTokens,
		CompletionTokens: resp.CompletionTokens,
		GraphNodesUsed:   nodesUsed,
		Status:           status,
		ResponseSummary:  summary,
	})
	if h.RBAC != nil && body.ProjectID > 0 {
		uid := ActorUserID(r, h.RBAC)
		_ = h.RBAC.IncrementChatUsage(ctx, uid, body.ProjectID)
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"response": resp.Text,
		"provider": resp.Provider,
		"model":    resp.Model,
		"usage": map[string]int{
			"promptTokens":     resp.PromptTokens,
			"completionTokens": resp.CompletionTokens,
			"graphNodesUsed":   nodesUsed,
		},
		"run":               run,
		"tokenSavingNote":   tokenNote,
		"graphAutoIngested": graphAutoIngested,
	})
}

// seedEnvironmentGraph writes compact service/module summaries used by LLM retrieval.
// All nodes are written only into the target environment_id. Global/shared issue lists are
// intentionally NOT ingested — issues lack project/env scope and would leak across tenants.
func seedEnvironmentGraph(ctx context.Context, graph *store.GraphStore, _ *store.IssueStore, envID int) (int, error) {
	if envID <= 0 {
		return 0, fmt.Errorf("environment id is required")
	}
	if graph == nil {
		return 0, fmt.Errorf("graph store unavailable")
	}
	created := 0
	seed := []struct {
		kind, key, title, summary string
	}{
		{"service", "aegisops-api", "AegisOps API", "Go orchestrator on :5001 — issues, webhooks, workspace, LLM."},
		{"module", "orchestrator", "Orchestrator", "Correlates telemetry, opens fix branches, PRs, gateway approve/reject."},
		{"module", "workspace", "Workspace tenancy", "Tenant → project → env → connectors + encrypted secrets. Graphs never merge across envs."},
		{"module", "llm-graph", "LLM context graph", "Compact nodes/edges retrieved per prompt; scoped strictly to one environment_id."},
		{"module", "mcp", "MCP server", "Optional IDE tools; LLM API path works without Cursor."},
		{"path", "internal/http", "HTTP handlers", "REST /api/v1 for issues, deployments, workspace, llm, graph."},
		{"path", "web/src", "Dashboard UI", "Next.js embedded console — Kepler-style shell."},
	}
	var prev *store.GraphNode
	for _, s := range seed {
		n, err := graph.UpsertNode(ctx, envID, s.kind, s.key, s.title, s.summary, nil)
		if err != nil {
			continue
		}
		created++
		if prev != nil {
			_, _ = graph.UpsertEdge(ctx, envID, prev.ID, n.ID, "related_to")
		}
		prev = &n
	}
	return created, nil
}

type GraphHandler struct {
	Graph     *store.GraphStore
	Issues    *store.IssueStore
	Workspace *store.WorkspaceStore
}

func (h *GraphHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.Graph == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if !h.requireEnvProjectScope(w, r, ctx, envID) {
		return
	}
	snap, err := h.Graph.Snapshot(ctx, envID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load graph"})
		return
	}
	WriteJSON(w, http.StatusOK, snap)
}

func (h *GraphHandler) UpsertNode(w http.ResponseWriter, r *http.Request) {
	if h.Graph == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	var body struct {
		Kind      string         `json:"kind"`
		NodeKey   string         `json:"nodeKey"`
		Title     string         `json:"title"`
		Summary   string         `json:"summary"`
		Metadata  map[string]any `json:"metadata"`
		ProjectID int            `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.Kind = strings.TrimSpace(strings.ToLower(body.Kind))
	body.NodeKey = strings.TrimSpace(body.NodeKey)
	body.Title = strings.TrimSpace(body.Title)
	if body.Kind == "" || body.NodeKey == "" || body.Title == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "kind, nodeKey, and title are required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if body.ProjectID > 0 && h.Workspace != nil {
		if err := h.Workspace.AssertEnvironmentBelongsToProject(ctx, envID, body.ProjectID); err != nil {
			WriteJSON(w, http.StatusForbidden, map[string]string{
				"error": "environment is not under the specified project — graph writes stay isolated per project/env",
			})
			return
		}
	}
	n, err := h.Graph.UpsertNode(ctx, envID, body.Kind, body.NodeKey, body.Title, body.Summary, body.Metadata)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save node"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"node": n})
}

func (h *GraphHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	if h.Graph == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if !h.requireEnvProjectScope(w, r, ctx, envID) {
		return
	}

	created, err := seedEnvironmentGraph(ctx, h.Graph, h.Issues, envID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	snap, _ := h.Graph.Snapshot(ctx, envID)
	WriteJSON(w, http.StatusOK, map[string]any{
		"ingested": created,
		"graph":    snap,
		"note":     "Graph summaries are env-scoped only — not full source and never merged across projects/envs.",
	})
}

func (h *GraphHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	if h.Graph == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if !h.requireEnvProjectScope(w, r, ctx, envID) {
		return
	}
	runs, err := h.Graph.ListLLMRuns(ctx, envID, 30)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list runs"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

// requireEnvProjectScope enforces optional ?projectId= / X-Project-Id against the path env.
// Returns false after writing an error response when the check fails.
func (h *GraphHandler) requireEnvProjectScope(w http.ResponseWriter, r *http.Request, ctx context.Context, envID int) bool {
	if h.Workspace == nil {
		return true
	}
	projectID := 0
	if q := strings.TrimSpace(r.URL.Query().Get("projectId")); q != "" {
		projectID, _ = strconv.Atoi(q)
	}
	if projectID <= 0 {
		if hdr := strings.TrimSpace(r.Header.Get("X-Project-Id")); hdr != "" {
			projectID, _ = strconv.Atoi(hdr)
		}
	}
	if projectID <= 0 {
		return true
	}
	if err := h.Workspace.AssertEnvironmentBelongsToProject(ctx, envID, projectID); err != nil {
		WriteJSON(w, http.StatusForbidden, map[string]string{
			"error": "environment is not under the specified project — graph context stays isolated per project/env",
		})
		return false
	}
	return true
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
