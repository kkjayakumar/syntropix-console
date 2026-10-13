package store

import (
	"context"
	"fmt"
)

func (s *WebhookStore) MarkProcessed(ctx context.Context, id int64) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := s.pool.Exec(ctx, `UPDATE aegis_webhook_events SET processed = TRUE WHERE id = $1`, id)
	return err
}

func (s *WebhookStore) ListByCorrelationID(ctx context.Context, correlationID string) ([]WebhookEvent, error) {
	if s == nil || s.pool == nil || correlationID == "" {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, source, event_type, COALESCE(correlation_id, ''), payload, processed, created_at
		FROM aegis_webhook_events WHERE correlation_id = $1 ORDER BY created_at ASC
	`, correlationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []WebhookEvent
	for rows.Next() {
		var e WebhookEvent
		if err := rows.Scan(&e.ID, &e.Source, &e.EventType, &e.CorrelationID, &e.Payload, &e.Processed, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
