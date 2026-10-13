-- +goose Up
-- +goose StatementBegin
ALTER TABLE aegis_deployments ADD COLUMN IF NOT EXISTS verify_status TEXT NOT NULL DEFAULT '';
ALTER TABLE aegis_deployments ADD COLUMN IF NOT EXISTS verify_message TEXT;

CREATE TABLE IF NOT EXISTS aegis_sentinel_monitors (
    id SERIAL PRIMARY KEY,
    deployment_id INT REFERENCES aegis_deployments(id) ON DELETE CASCADE,
    copilot_monitor_id INT,
    verify_url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_aegis_sentinel_expires ON aegis_sentinel_monitors (expires_at);
CREATE INDEX IF NOT EXISTS idx_aegis_sentinel_status ON aegis_sentinel_monitors (status);

CREATE TABLE IF NOT EXISTS aegis_gateway_tokens (
    id SERIAL PRIMARY KEY,
    issue_id INT NOT NULL REFERENCES aegis_issues(id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    token TEXT NOT NULL UNIQUE,
    used BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_aegis_gateway_token ON aegis_gateway_tokens (token);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS aegis_gateway_tokens;
DROP TABLE IF EXISTS aegis_sentinel_monitors;
ALTER TABLE aegis_deployments DROP COLUMN IF EXISTS verify_message;
ALTER TABLE aegis_deployments DROP COLUMN IF EXISTS verify_status;
-- +goose StatementEnd
