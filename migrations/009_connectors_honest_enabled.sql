-- +goose Up
-- +goose StatementBegin
-- Honest enabled state: stages enabled without required config are not Connected.
UPDATE aegis_connectors
SET
    enabled = false,
    updated_at = NOW()
WHERE enabled = true
  AND (config IS NULL OR config = '{}'::jsonb);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
