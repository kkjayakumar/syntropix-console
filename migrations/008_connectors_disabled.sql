-- +goose Up
-- +goose StatementBegin
-- Stop presenting seeded connectors as Connected. Reset to disabled empty templates
-- so stages stay Needs setup until the user explicitly configures and enables them.
UPDATE aegis_connectors
SET
    enabled = false,
    config = '{}'::jsonb,
    updated_at = NOW();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Irreversible intentionally: we do not restore prior connector configs on down.
SELECT 1;
-- +goose StatementEnd
