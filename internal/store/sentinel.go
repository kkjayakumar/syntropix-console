package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SentinelMonitor struct {
	ID               int64     `json:"id"`
	DeploymentID     int64     `json:"deploymentId"`
	CopilotMonitorID *int      `json:"copilotMonitorId,omitempty"`
	VerifyURL        string    `json:"verifyUrl"`
	Status           string    `json:"status"`
	ExpiresAt        time.Time `json:"expiresAt"`
	CreatedAt        time.Time `json:"createdAt"`
}

type SentinelStore struct {
	pool *pgxpool.Pool
}

func NewSentinelStore(pool *pgxpool.Pool) *SentinelStore {
	return &SentinelStore{pool: pool}
}

func (s *SentinelStore) Create(ctx context.Context, deploymentID int64, copilotMonitorID *int, verifyURL, status string, expiresAt time.Time) (SentinelMonitor, error) {
	if s == nil || s.pool == nil {
		return SentinelMonitor{}, fmt.Errorf("database not available")
	}
	var out SentinelMonitor
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_sentinel_monitors (deployment_id, copilot_monitor_id, verify_url, status, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, deployment_id, copilot_monitor_id, verify_url, status, expires_at, created_at
	`, deploymentID, copilotMonitorID, verifyURL, status, expiresAt).
		Scan(&out.ID, &out.DeploymentID, &out.CopilotMonitorID, &out.VerifyURL, &out.Status, &out.ExpiresAt, &out.CreatedAt)
	return out, err
}

func (s *SentinelStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := s.pool.Exec(ctx, `UPDATE aegis_sentinel_monitors SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (s *SentinelStore) ListExpired(ctx context.Context, before time.Time) ([]SentinelMonitor, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, deployment_id, copilot_monitor_id, verify_url, status, expires_at, created_at
		FROM aegis_sentinel_monitors
		WHERE expires_at <= $1 AND status NOT IN ('removed', 'failed')
	`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SentinelMonitor
	for rows.Next() {
		var m SentinelMonitor
		if err := rows.Scan(&m.ID, &m.DeploymentID, &m.CopilotMonitorID, &m.VerifyURL, &m.Status, &m.ExpiresAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *DeploymentStore) UpdateVerify(ctx context.Context, id int64, status, message string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE aegis_deployments SET verify_status = $1, verify_message = $2 WHERE id = $3
	`, status, message, id)
	return err
}
