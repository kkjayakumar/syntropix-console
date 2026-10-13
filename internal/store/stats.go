package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type StatsSnapshot struct {
	IssuesTotal      int            `json:"issuesTotal"`
	IssuesByStatus   map[string]int `json:"issuesByStatus"`
	IssuesFixCount   int            `json:"issuesFixCount"`
	IssuesFeatCount  int            `json:"issuesFeatCount"`
	DeploymentsTotal int            `json:"deploymentsTotal"`
	DeployFailures   int            `json:"deployFailures"`
	OpenIncidents    int            `json:"openIncidents"`
}

type StatsStore struct {
	pool *pgxpool.Pool
}

func NewStatsStore(pool *pgxpool.Pool) *StatsStore {
	return &StatsStore{pool: pool}
}

func (s *StatsStore) Snapshot(ctx context.Context) (StatsSnapshot, error) {
	if s == nil || s.pool == nil {
		return StatsSnapshot{}, fmt.Errorf("database not available")
	}
	out := StatsSnapshot{IssuesByStatus: make(map[string]int)}
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM aegis_issues`).Scan(&out.IssuesTotal); err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `SELECT status, COUNT(*) FROM aegis_issues GROUP BY status`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return out, err
		}
		out.IssuesByStatus[status] = count
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM aegis_issues WHERE issue_type = 'fix'`).Scan(&out.IssuesFixCount)
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM aegis_issues WHERE issue_type = 'feat'`).Scan(&out.IssuesFeatCount)
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM aegis_deployments`).Scan(&out.DeploymentsTotal)
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM aegis_deployments WHERE status = 'failure'`).Scan(&out.DeployFailures)
	_ = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM aegis_issues WHERE status NOT IN ('fixed', 'closed')
	`).Scan(&out.OpenIncidents)
	return out, nil
}

func (s *IssueStore) CreateFeat(ctx context.Context, title, description, correlationID string) (Issue, error) {
	if !strings.HasPrefix(title, "feat:") {
		title = "feat: " + title
	}
	bundle := fmt.Sprintf("# Feature request\n\n%s\n", description)
	return s.Create(ctx, Issue{
		Title:           title,
		IssueType:       "feat",
		Description:     &description,
		Status:          "detected",
		CorrelationID:   correlationID,
		TelemetryBundle: &bundle,
	})
}
