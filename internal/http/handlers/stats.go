package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/store"
)

type StatsHandler struct {
	Store *store.StatsStore
}

func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	stats, err := h.Store.Snapshot(ctx)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load stats"})
		return
	}
	WriteJSON(w, http.StatusOK, stats)
}
