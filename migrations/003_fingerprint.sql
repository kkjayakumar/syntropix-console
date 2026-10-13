-- +goose Up
-- +goose StatementBegin
ALTER TABLE aegis_issues ADD COLUMN IF NOT EXISTS fingerprint TEXT;
CREATE INDEX IF NOT EXISTS idx_aegis_issues_fingerprint ON aegis_issues (fingerprint) WHERE fingerprint IS NOT NULL AND fingerprint <> '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_aegis_issues_fingerprint;
ALTER TABLE aegis_issues DROP COLUMN IF EXISTS fingerprint;
-- +goose StatementEnd
