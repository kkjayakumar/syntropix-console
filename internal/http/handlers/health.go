package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/config"
	"github.com/kkjayakumar/AegIsOps/internal/db"
	"github.com/kkjayakumar/AegIsOps/internal/orchestrator"
)

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type HealthHandler struct {
	DB           *db.DB
	Config       *config.Config
	Orchestrator *orchestrator.Service
	StartTime    time.Time
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "aegisops"})
}

func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "database not configured"})
		return
	}
	if err := h.DB.Ping(r.Context()); err != nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "database unreachable"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *HealthHandler) Version(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"service": "aegisops",
		"version": "0.1.0",
		"uptime":  time.Since(h.StartTime).String(),
	}
	if h.Config != nil {
		out["environment"] = h.Config.Environment
		out["auth"] = h.Config.AuthStatus()
		gh := h.Config.GitHubAuthStatus()
		if h.Orchestrator != nil {
			for k, v := range h.Orchestrator.GitHubRuntimeStatus() {
				gh[k] = v
			}
		}
		out["github"] = gh
	}
	WriteJSON(w, http.StatusOK, out)
}
