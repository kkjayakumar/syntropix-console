package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GatewayToken struct {
	ID        int64     `json:"id"`
	IssueID   int64     `json:"issueId"`
	Action    string    `json:"action"`
	Token     string    `json:"token"`
	Used      bool      `json:"used"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type GatewayStore struct {
	pool *pgxpool.Pool
}

func NewGatewayStore(pool *pgxpool.Pool) *GatewayStore {
	return &GatewayStore{pool: pool}
}

func (s *GatewayStore) CreatePair(ctx context.Context, issueID int64, ttl time.Duration) (approve, reject GatewayToken, err error) {
	if s == nil || s.pool == nil {
		return GatewayToken{}, GatewayToken{}, fmt.Errorf("database not available")
	}
	expires := time.Now().Add(ttl)
	approve, err = s.insert(ctx, issueID, "approve", expires)
	if err != nil {
		return GatewayToken{}, GatewayToken{}, err
	}
	reject, err = s.insert(ctx, issueID, "reject", expires)
	return approve, reject, err
}

func (s *GatewayStore) insert(ctx context.Context, issueID int64, action string, expires time.Time) (GatewayToken, error) {
	token, err := randomToken()
	if err != nil {
		return GatewayToken{}, err
	}
	var out GatewayToken
	err = s.pool.QueryRow(ctx, `
		INSERT INTO aegis_gateway_tokens (issue_id, action, token, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, issue_id, action, token, used, expires_at, created_at
	`, issueID, action, token, expires).
		Scan(&out.ID, &out.IssueID, &out.Action, &out.Token, &out.Used, &out.ExpiresAt, &out.CreatedAt)
	return out, err
}

func (s *GatewayStore) ActiveToken(ctx context.Context, issueID int64, action string) (string, error) {
	if s == nil || s.pool == nil {
		return "", fmt.Errorf("database not available")
	}
	var token string
	err := s.pool.QueryRow(ctx, `
		SELECT token FROM aegis_gateway_tokens
		WHERE issue_id = $1 AND action = $2 AND used = false AND expires_at > now()
		ORDER BY created_at DESC
		LIMIT 1
	`, issueID, action).Scan(&token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("no active %s token for issue", action)
		}
		return "", err
	}
	return token, nil
}

func (s *GatewayStore) Consume(ctx context.Context, token string) (GatewayToken, error) {
	if s == nil || s.pool == nil {
		return GatewayToken{}, fmt.Errorf("database not available")
	}
	var out GatewayToken
	err := s.pool.QueryRow(ctx, `
		SELECT id, issue_id, action, token, used, expires_at, created_at
		FROM aegis_gateway_tokens WHERE token = $1
	`, token).Scan(&out.ID, &out.IssueID, &out.Action, &out.Token, &out.Used, &out.ExpiresAt, &out.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GatewayToken{}, fmt.Errorf("invalid token")
		}
		return GatewayToken{}, err
	}
	if out.Used {
		return GatewayToken{}, fmt.Errorf("token already used")
	}
	if time.Now().After(out.ExpiresAt) {
		return GatewayToken{}, fmt.Errorf("token expired")
	}
	_, err = s.pool.Exec(ctx, `UPDATE aegis_gateway_tokens SET used = TRUE WHERE id = $1`, out.ID)
	return out, err
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
