package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/orchestrator"
)

type GitHubWebhookHandler struct {
	Secret       string
	Orchestrator *orchestrator.Service
}

func (h *GitHubWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 5<<20))
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}

	if h.Secret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !verifyGitHubSignature(body, h.Secret, sig) {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
			return
		}
	}

	event := r.Header.Get("X-GitHub-Event")
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch event {
	case "workflow_run":
		h.handleWorkflowRun(ctx, payload)
	case "pull_request":
		h.handlePullRequest(ctx, payload)
	case "push":
		h.handlePush(ctx, payload)
	case "issues":
		slog.Info("github issue event received", "action", payload["action"])
	}

	WriteJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "event": event})
}

func (h *GitHubWebhookHandler) handleWorkflowRun(ctx context.Context, payload map[string]any) {
	run, _ := payload["workflow_run"].(map[string]any)
	if run == nil {
		return
	}
	repoObj, _ := payload["repository"].(map[string]any)
	repoFull := ""
	if repoObj != nil {
		repoFull, _ = repoObj["full_name"].(string)
	}
	status, _ := run["conclusion"].(string)
	if status == "" {
		status, _ = run["status"].(string)
	}
	runID := stringify(run["id"])
	workflow, _ := run["name"].(string)
	sha, _ := run["head_sha"].(string)
	branch, _ := run["head_branch"].(string)
	logs := ""
	if status == "failure" {
		logs = "Workflow run failed. Fetch full logs via GitHub Actions UI."
	}
	if h.Orchestrator != nil {
		h.Orchestrator.HandleWorkflowRun(ctx, repoFull, runID, workflow, sha, branch, status, logs)
	}
}

func (h *GitHubWebhookHandler) handlePullRequest(ctx context.Context, payload map[string]any) {
	action, _ := payload["action"].(string)
	pr, _ := payload["pull_request"].(map[string]any)
	if pr == nil || h.Orchestrator == nil {
		return
	}
	h.Orchestrator.HandlePullRequest(ctx, action, pr)
}

func (h *GitHubWebhookHandler) handlePush(ctx context.Context, payload map[string]any) {
	if h.Orchestrator == nil {
		return
	}
	if deleted, _ := payload["deleted"].(bool); deleted {
		return
	}
	ref, _ := payload["ref"].(string)
	if !strings.HasPrefix(ref, "refs/heads/aegis/") {
		return
	}
	issueID := orchestrator.ParseAegisBranchFromRef(ref)
	if issueID == 0 {
		return
	}
	if _, err := h.Orchestrator.OpenPRForIssue(ctx, issueID); err != nil {
		slog.Warn("auto open PR on push failed", "issueId", issueID, "ref", ref, "error", err)
	} else {
		slog.Info("auto opened PR on push", "issueId", issueID, "ref", ref)
	}
}

func verifyGitHubSignature(body []byte, secret, header string) bool {
	if header == "" || !strings.HasPrefix(header, "sha256=") {
		return false
	}
	expectedMAC := strings.TrimPrefix(header, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(expectedMAC))
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	default:
		b, _ := json.Marshal(v)
		return strings.Trim(string(b), "\"")
	}
}
