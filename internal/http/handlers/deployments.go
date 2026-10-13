package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/orchestrator"
	"github.com/kkjayakumar/AegIsOps/internal/store"
)

type DeploymentsHandler struct {
	Store *store.DeploymentStore
}

func (h *DeploymentsHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	deployments, err := h.Store.ListRecent(ctx, 50)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list deployments"})
		return
	}
	deployments = orchestrator.FilterRealDeployments(deployments)
	WriteJSON(w, http.StatusOK, map[string]any{"deployments": deployments, "total": len(deployments)})
}

func (h *DeploymentsHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	dep, err := h.Store.GetByID(ctx, id)
	if err != nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}
	WriteJSON(w, http.StatusOK, dep)
}

func (h *DeploymentsHandler) Logs(w http.ResponseWriter, r *http.Request) {
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
	dep, err := h.Store.GetByID(ctx, id)
	if err != nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}
	logs := ""
	if dep.FailureLogs != nil {
		logs = *dep.FailureLogs
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(logs))
}
