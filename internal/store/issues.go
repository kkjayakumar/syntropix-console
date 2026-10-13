package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Issue struct {
	ID                int64     `json:"id"`
	GitHubIssueNumber *int      `json:"githubIssueNumber,omitempty"`
	GitHubRepo        string    `json:"githubRepo"`
	Title             string    `json:"title"`
	IssueType         string    `json:"issueType"`
	Description       *string   `json:"description,omitempty"`
	Status            string    `json:"status"`
	CorrelationID     string    `json:"correlationId"`
	Fingerprint       string    `json:"fingerprint,omitempty"`
	TelemetryBundle   *string   `json:"telemetryBundle,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type TelemetryLink struct {
	ID               int64   `json:"id"`
	AegisIssueID     int64   `json:"aegisIssueId"`
	SpottingReportID *string `json:"spottingReportId,omitempty"`
	SentryIssueID    *string `json:"sentryIssueId,omitempty"`
}

type IssueStore struct {
	pool *pgxpool.Pool
}

func NewIssueStore(pool *pgxpool.Pool) *IssueStore {
	return &IssueStore{pool: pool}
}

func (s *IssueStore) Create(ctx context.Context, issue Issue) (Issue, error) {
	if s == nil || s.pool == nil {
		return Issue{}, fmt.Errorf("database not available")
	}
	var out Issue
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_issues (github_issue_number, github_repo, title, issue_type, description, status, correlation_id, fingerprint, telemetry_bundle)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, github_issue_number, github_repo, title, issue_type, description, status, correlation_id, telemetry_bundle, created_at, updated_at
	`, issue.GitHubIssueNumber, issue.GitHubRepo, issue.Title, issue.IssueType, issue.Description, issue.Status, issue.CorrelationID, nullIfEmpty(issue.Fingerprint), issue.TelemetryBundle).
		Scan(&out.ID, &out.GitHubIssueNumber, &out.GitHubRepo, &out.Title, &out.IssueType, &out.Description, &out.Status, &out.CorrelationID, &out.TelemetryBundle, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (s *IssueStore) UpdateGitHubIssueNumber(ctx context.Context, id int64, repo string, number int) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE aegis_issues SET github_issue_number = $1, github_repo = $2, updated_at = NOW() WHERE id = $3
	`, number, repo, id)
	return err
}

func (s *IssueStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := s.pool.Exec(ctx, `UPDATE aegis_issues SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}

func (s *IssueStore) UpdateBundle(ctx context.Context, id int64, bundle string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := s.pool.Exec(ctx, `UPDATE aegis_issues SET telemetry_bundle = $1, updated_at = NOW() WHERE id = $2`, bundle, id)
	return err
}

func (s *IssueStore) UpdateTitle(ctx context.Context, id int64, title string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := s.pool.Exec(ctx, `UPDATE aegis_issues SET title = $1, updated_at = NOW() WHERE id = $2`, title, id)
	return err
}

func (s *IssueStore) GetByID(ctx context.Context, id int64) (Issue, error) {
	if s == nil || s.pool == nil {
		return Issue{}, fmt.Errorf("database not available")
	}
	var out Issue
	err := s.pool.QueryRow(ctx, `
		SELECT id, github_issue_number, github_repo, title, issue_type, description, status, correlation_id, telemetry_bundle, created_at, updated_at
		FROM aegis_issues WHERE id = $1
	`, id).Scan(&out.ID, &out.GitHubIssueNumber, &out.GitHubRepo, &out.Title, &out.IssueType, &out.Description, &out.Status, &out.CorrelationID, &out.TelemetryBundle, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (s *IssueStore) FindOpenByCorrelationID(ctx context.Context, correlationID string) (*Issue, error) {
	if s == nil || s.pool == nil || correlationID == "" {
		return nil, nil
	}
	var out Issue
	err := s.pool.QueryRow(ctx, `
		SELECT id, github_issue_number, github_repo, title, issue_type, description, status, correlation_id, telemetry_bundle, created_at, updated_at
		FROM aegis_issues
		WHERE correlation_id = $1 AND status NOT IN ('fixed', 'closed')
		ORDER BY created_at DESC LIMIT 1
	`, correlationID).Scan(&out.ID, &out.GitHubIssueNumber, &out.GitHubRepo, &out.Title, &out.IssueType, &out.Description, &out.Status, &out.CorrelationID, &out.TelemetryBundle, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

func (s *IssueStore) ListOpenByCorrelationPrefix(ctx context.Context, prefix string, limit int) ([]Issue, error) {
	if s == nil || s.pool == nil || prefix == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, github_issue_number, github_repo, title, issue_type, description, status, correlation_id, telemetry_bundle, created_at, updated_at
		FROM aegis_issues
		WHERE correlation_id LIKE $1 AND status NOT IN ('fixed', 'closed')
		ORDER BY created_at DESC LIMIT $2
	`, prefix+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Issue
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.ID, &issue.GitHubIssueNumber, &issue.GitHubRepo, &issue.Title, &issue.IssueType, &issue.Description, &issue.Status, &issue.CorrelationID, &issue.TelemetryBundle, &issue.CreatedAt, &issue.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, issue)
	}
	return out, rows.Err()
}

func (s *IssueStore) FindOpenByFingerprint(ctx context.Context, fingerprint string) (*Issue, error) {
	if s == nil || s.pool == nil || fingerprint == "" {
		return nil, nil
	}
	var out Issue
	err := s.pool.QueryRow(ctx, `
		SELECT id, github_issue_number, github_repo, title, issue_type, description, status, correlation_id, telemetry_bundle, created_at, updated_at
		FROM aegis_issues
		WHERE fingerprint = $1 AND status NOT IN ('fixed', 'closed')
		ORDER BY created_at DESC LIMIT 1
	`, fingerprint).Scan(&out.ID, &out.GitHubIssueNumber, &out.GitHubRepo, &out.Title, &out.IssueType, &out.Description, &out.Status, &out.CorrelationID, &out.TelemetryBundle, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (s *IssueStore) List(ctx context.Context, issueType, status string, limit int) ([]Issue, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, github_issue_number, github_repo, title, issue_type, description, status, correlation_id, telemetry_bundle, created_at, updated_at
		FROM aegis_issues WHERE 1=1`
	args := []any{}
	n := 1
	if issueType != "" {
		query += fmt.Sprintf(" AND issue_type = $%d", n)
		args = append(args, issueType)
		n++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status = $%d", n)
		args = append(args, status)
		n++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", n)
	args = append(args, limit)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var issues []Issue
	for rows.Next() {
		var i Issue
		if err := rows.Scan(&i.ID, &i.GitHubIssueNumber, &i.GitHubRepo, &i.Title, &i.IssueType, &i.Description, &i.Status, &i.CorrelationID, &i.TelemetryBundle, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		issues = append(issues, i)
	}
	return issues, rows.Err()
}

func (s *IssueStore) AddTelemetryLink(ctx context.Context, issueID int64, spottingReportID, sentryIssueID *string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO aegis_telemetry_links (aegis_issue_id, spotting_report_id, sentry_issue_id)
		VALUES ($1, $2, $3)
	`, issueID, spottingReportID, sentryIssueID)
	return err
}

func (s *IssueStore) GetTelemetryLinks(ctx context.Context, issueID int64) ([]TelemetryLink, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, aegis_issue_id, spotting_report_id, sentry_issue_id
		FROM aegis_telemetry_links WHERE aegis_issue_id = $1
	`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []TelemetryLink
	for rows.Next() {
		var l TelemetryLink
		if err := rows.Scan(&l.ID, &l.AegisIssueID, &l.SpottingReportID, &l.SentryIssueID); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (s *IssueStore) FindByGitHubIssueNumber(ctx context.Context, repo string, number int) (*Issue, error) {
	if s == nil || s.pool == nil || number <= 0 {
		return nil, nil
	}
	var out Issue
	err := s.pool.QueryRow(ctx, `
		SELECT id, github_issue_number, github_repo, title, issue_type, description, status, correlation_id, telemetry_bundle, created_at, updated_at
		FROM aegis_issues
		WHERE github_issue_number = $1 AND ($2 = '' OR github_repo = $2)
		ORDER BY updated_at DESC LIMIT 1
	`, number, repo).Scan(&out.ID, &out.GitHubIssueNumber, &out.GitHubRepo, &out.Title, &out.IssueType, &out.Description, &out.Status, &out.CorrelationID, &out.TelemetryBundle, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &out, nil
}
