package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/kkjayakumar/AegIsOps/internal/clients"
	"github.com/kkjayakumar/AegIsOps/internal/http/middleware"
	"github.com/kkjayakumar/AegIsOps/internal/orchestrator"
	"github.com/kkjayakumar/AegIsOps/internal/telemetry"
)

type DebugHandler struct {
	AllowDebug   bool
	Orchestrator *orchestrator.Service
	Copilot      *clients.CopilotClient
	PublicURL    string
}

func (h *DebugHandler) TriggerError(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.AllowDebug {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	requestID := middleware.FromContext(r.Context())
	err := errors.New("AegisOps dogfood: intentional backend error for telemetry correlation")
	telemetry.CaptureError(err, requestID)
	telemetry.Flush()

	if h.Orchestrator != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()

		if h.Copilot != nil {
			if detail, err := h.Copilot.PollIssueByRequestID(ctx, requestID); err != nil {
				slog.Warn("dogfood copilot poll failed", "requestId", requestID, "error", err)
			} else if detail != nil && detail.Issue != nil {
				copilotPayload := clients.DogfoodCopilotWebhookPayload(requestID, detail.Issue, true)
				if err := h.Orchestrator.IngestInternalWebhook(ctx, "copilot", copilotPayload); err != nil {
					slog.Warn("dogfood copilot ingest failed", "requestId", requestID, "error", err)
				}
			} else {
				slog.Warn("dogfood copilot issue not found after poll", "requestId", requestID)
			}
		}

		reportID := "rpt_dogfood_" + strings.ReplaceAll(requestID, "_", "")
		if len(reportID) > 40 {
			reportID = reportID[:40]
		}
		pageURL := strings.TrimSuffix(h.PublicURL, "/") + "/"
		spottingPayload := map[string]any{
			"event":         "spotting.report.created",
			"correlationId": requestID,
			"reportId":      reportID,
			"severity":      "error",
			"pageUrl":       pageURL,
			"networkFailures": []map[string]any{{
				"method": "POST",
				"url":    "/api/v1/debug/trigger-error",
				"status": 500,
			}},
			"consoleErrors": 1,
		}
		_ = h.Orchestrator.IngestInternalWebhook(ctx, "spotting", spottingPayload)
	}

	WriteJSON(w, http.StatusInternalServerError, map[string]any{
		"error":     err.Error(),
		"requestId": requestID,
		"service":   "aegisops",
		"dogfood":   true,
		"hint":      fmt.Sprintf("Copilot + Spotting webhooks expected for %s", requestID),
	})
}
