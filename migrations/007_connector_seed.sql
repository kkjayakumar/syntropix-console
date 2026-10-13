-- +goose Up
-- +goose StatementBegin
-- Seed closed-loop connector stage rows for every environment as disabled templates.
-- Only inserts when a connector_type is missing (preserves user-configured rows).
-- Does NOT mark stages Connected — enabled stays false with empty config.

INSERT INTO aegis_connectors (environment_id, connector_type, name, enabled, config)
SELECT e.id, v.ctype, v.cname, false, '{}'::jsonb
FROM aegis_environments e
CROSS JOIN (
    VALUES
        ('git', 'Source'),
        ('pull_request', 'Pull request'),
        ('cicd', 'CI / CD'),
        ('deployment', 'Deploy'),
        ('telemetry', 'Telemetry'),
        ('mcp', 'IDE / MCP'),
        ('issue_agent', 'AI agent')
) AS v(ctype, cname)
WHERE NOT EXISTS (
    SELECT 1 FROM aegis_connectors c
    WHERE c.environment_id = e.id AND c.connector_type = v.ctype
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Non-destructive down: leave connector rows in place (data may have been user-edited).
SELECT 1;
-- +goose StatementEnd
