package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Deployment struct {
	ID            int64     `json:"id"`
	GitHubRepo    string    `json:"githubRepo"`
	RunID         string    `json:"runId"`
	WorkflowName  string    `json:"workflowName"`
	CommitSHA     string    `json:"commitSha"`
	Branch        string    `json:"branch"`
	Status        string    `json:"status"`
	FailureLogs   *string   `json:"failureLogs,omitempty"`
	VerifyStatus  string    `json:"verifyStatus,omitempty"`
	VerifyMessage *string   `json:"verifyMessage,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

type DeploymentStore struct {
	pool *pgxpool.Pool
}

func NewDeploymentStore(pool *pgxpool.Pool) *DeploymentStore {
	return &DeploymentStore{pool: pool}
}

func (s *DeploymentStore) Upsert(ctx context.Context, d Deployment) (Deployment, error) {
	if s == nil || s.pool == nil {
		return Deployment{}, fmt.Errorf("database not available")
	}
	var out Deployment
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_deployments (github_repo, run_id, workflow_name, commit_sha, branch, status, failure_logs)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (github_repo, run_id) DO UPDATE SET
			status = EXCLUDED.status,
			failure_logs = COALESCE(EXCLUDED.failure_logs, aegis_deployments.failure_logs)
		RETURNING id, github_repo, run_id, workflow_name, commit_sha, branch, status, failure_logs, verify_status, verify_message, created_at
	`, d.GitHubRepo, d.RunID, d.WorkflowName, d.CommitSHA, d.Branch, d.Status, d.FailureLogs).
		Scan(&out.ID, &out.GitHubRepo, &out.RunID, &out.WorkflowName, &out.CommitSHA, &out.Branch, &out.Status, &out.FailureLogs, &out.VerifyStatus, &out.VerifyMessage, &out.CreatedAt)
	return out, err
}

func (s *DeploymentStore) ListRecent(ctx context.Context, limit int) ([]Deployment, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, github_repo, run_id, workflow_name, commit_sha, branch, status, failure_logs, verify_status, verify_message, created_at
		FROM aegis_deployments ORDER BY created_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Deployment
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.GitHubRepo, &d.RunID, &d.WorkflowName, &d.CommitSHA, &d.Branch, &d.Status, &d.FailureLogs, &d.VerifyStatus, &d.VerifyMessage, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *DeploymentStore) GetByID(ctx context.Context, id int64) (Deployment, error) {
	if s == nil || s.pool == nil {
		return Deployment{}, fmt.Errorf("database not available")
	}
	var d Deployment
	err := s.pool.QueryRow(ctx, `
		SELECT id, github_repo, run_id, workflow_name, commit_sha, branch, status, failure_logs, verify_status, verify_message, created_at
		FROM aegis_deployments WHERE id = $1
	`, id).Scan(&d.ID, &d.GitHubRepo, &d.RunID, &d.WorkflowName, &d.CommitSHA, &d.Branch, &d.Status, &d.FailureLogs, &d.VerifyStatus, &d.VerifyMessage, &d.CreatedAt)
	return d, err
}
