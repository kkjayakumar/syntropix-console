package orchestrator

import (
	"context"
	"encoding/json"
	"log/slog"
)

// IngestInternalWebhook persists and processes a webhook (dogfood / tests).
func (s *Service) IngestInternalWebhook(ctx context.Context, source string, payload map[string]any) error {
	if s == nil || s.webhooks == nil {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	eventType, _ := payload["event"].(string)
	correlationID, _ := payload["correlationId"].(string)
	if correlationID == "" {
		correlationID, _ = payload["correlation_id"].(string)
	}
	eventID, err := s.webhooks.Insert(ctx, source, eventType, correlationID, body)
	if err != nil {
		return err
	}
	if eventID > 0 {
		go s.ProcessWebhookEvent(context.Background(), eventID, source, body)
	}
	slog.Info("internal webhook ingested", "source", source, "correlationId", correlationID, "eventId", eventID)
	return nil
}
