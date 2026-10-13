-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS aegis_issues (
    id SERIAL PRIMARY KEY,
    github_issue_number INT,
    github_repo TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    issue_type TEXT NOT NULL DEFAULT 'fix',
    description TEXT,
    status TEXT NOT NULL DEFAULT 'detected',
    correlation_id TEXT NOT NULL,
    telemetry_bundle TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_aegis_issues_correlation_id ON aegis_issues (correlation_id);
CREATE INDEX IF NOT EXISTS idx_aegis_issues_status ON aegis_issues (status);
CREATE INDEX IF NOT EXISTS idx_aegis_issues_issue_type ON aegis_issues (issue_type);

CREATE TABLE IF NOT EXISTS aegis_deployments (
    id SERIAL PRIMARY KEY,
    github_repo TEXT NOT NULL,
    run_id TEXT NOT NULL,
    workflow_name TEXT NOT NULL,
    commit_sha TEXT NOT NULL,
    branch TEXT NOT NULL,
    status TEXT NOT NULL,
    failure_logs TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_aegis_deployments_run ON aegis_deployments (github_repo, run_id);

CREATE TABLE IF NOT EXISTS aegis_telemetry_links (
    id SERIAL PRIMARY KEY,
    aegis_issue_id INT REFERENCES aegis_issues(id) ON DELETE CASCADE,
    spotting_report_id TEXT,
    sentry_issue_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS aegis_webhook_events (
    id SERIAL PRIMARY KEY,
    source TEXT NOT NULL,
    event_type TEXT NOT NULL,
    correlation_id TEXT,
    payload JSONB NOT NULL,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_aegis_webhook_events_correlation ON aegis_webhook_events (correlation_id);
CREATE INDEX IF NOT EXISTS idx_aegis_webhook_events_processed ON aegis_webhook_events (processed);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS aegis_webhook_events;
DROP TABLE IF EXISTS aegis_telemetry_links;
DROP TABLE IF EXISTS aegis_deployments;
DROP TABLE IF EXISTS aegis_issues;
-- +goose StatementEnd
