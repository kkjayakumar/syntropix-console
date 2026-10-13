package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/orchestrator"
)

type GatewayHandler struct {
	Orchestrator *orchestrator.Service
	PublicURL    string
}

func (h *GatewayHandler) Approve(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r, "approve")
}

func (h *GatewayHandler) Reject(w http.ResponseWriter, r *http.Request) {
	h.handle(w, r, "reject")
}

func (h *GatewayHandler) handle(w http.ResponseWriter, r *http.Request, expected string) {
	if h.Orchestrator == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gateway unavailable"})
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "token required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	issueID, action, err := h.Orchestrator.HandleGatewayAction(ctx, token)
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if action != expected {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "token action mismatch"})
		return
	}
	if r.Header.Get("Accept") == "text/html" || r.URL.Query().Get("format") == "html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := `<!DOCTYPE html><html><body style="font-family:sans-serif;background:#0b0f17;color:#e5e7eb;padding:2rem">
<h1>Gateway: ` + action + `</h1><p>Issue #` + strconv.FormatInt(issueID, 10) + ` updated.</p>
<p><a href="` + h.PublicURL + `/issues" style="color:#67e8f9">Return to dashboard</a></p></body></html>`
		_, _ = w.Write([]byte(html))
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"issueId": issueID, "action": action, "status": "ok"})
}
