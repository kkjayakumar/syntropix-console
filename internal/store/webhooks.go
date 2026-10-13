package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WebhookEvent struct {
	ID            int64           `json:"id"`
	Source        string          `json:"source"`
	EventType     string          `json:"eventType"`
	CorrelationID string          `json:"correlationId"`
	Payload       json.RawMessage `json:"payload"`
	Processed     bool            `json:"processed"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type WebhookStore struct {
	pool *pgxpool.Pool
}

func NewWebhookStore(pool *pgxpool.Pool) *WebhookStore {
	return &WebhookStore{pool: pool}
}

func (s *WebhookStore) Insert(ctx context.Context, source, eventType, correlationID string, payload []byte) (int64, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("database not available")
	}
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_webhook_events (source, event_type, correlation_id, payload)
		VALUES ($1, $2, NULLIF($3, ''), $4::jsonb)
		RETURNING id
	`, source, eventType, correlationID, string(payload)).Scan(&id)
	return id, err
}

func (s *WebhookStore) ListRecent(ctx context.Context, limit int) ([]WebhookEvent, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, source, event_type, COALESCE(correlation_id, ''), payload, processed, created_at
		FROM aegis_webhook_events ORDER BY created_at DESC LIMIT $1
	`, limit)
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