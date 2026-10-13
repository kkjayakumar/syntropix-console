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
	"strings"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/orchestrator"
	"github.com/kkjayakumar/AegIsOps/internal/store"
)

type WebhooksHandler struct {
	Secret       string
	Store        *store.WebhookStore
	Orchestrator *orchestrator.Service
}

func (h *WebhooksHandler) Copilot(w http.ResponseWriter, r *http.Request) { h.ingest(w, r, "copilot") }
func (h *WebhooksHandler) Spotting(w http.ResponseWriter, r *http.Request) { h.ingest(w, r, "spotting") }

func (h *WebhooksHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	events, err := h.Store.ListRecent(ctx, 50)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list events"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"events": events, "total": len(events)})
}

func (h *WebhooksHandler) ingest(w http.ResponseWriter, r *http.Request, source string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	if h.Secret != "" {
		if !verifyHMAC(body, h.Secret, r.Header.Get("X-Aegis-Signature")) {
			WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
			return
		}
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	eventType, _ := payload["event"].(string)
	correlationID, _ := payload["correlationId"].(string)
	if correlationID == "" {
		correlationID, _ = payload["correlation_id"].(string)
	}
	var eventID int64
	if h.Store != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		eventID, err = h.Store.Insert(ctx, source, eventType, correlationID, body)
		cancel()
		if err != nil {
			slog.Error("webhook store failed", "source", source, "error", err)
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist event"})
			return
		}
	}

	if h.Orchestrator != nil && eventID > 0 {
		go h.Orchestrator.ProcessWebhookEvent(context.Background(), eventID, source, body)
	}

	slog.Info("webhook received", "source", source, "event", eventType, "correlationId", correlationID, "id", eventID)
	WriteJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "id": eventID, "source": source, "event": eventType, "correlationId": correlationID})
}

func verifyHMAC(body []byte, secret, header string) bool {
	if header == "" {
		return false
	}
	header = strings.TrimPrefix(header, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal([]byte(hex.EncodeToString(mac.Sum(nil))), []byte(header))
}
