-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS aegis_tenants (
    id SERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS aegis_projects (
    id SERIAL PRIMARY KEY,
    tenant_id INT NOT NULL REFERENCES aegis_tenants(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, slug)
);

CREATE TABLE IF NOT EXISTS aegis_environments (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL REFERENCES aegis_projects(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, slug)
);

CREATE TABLE IF NOT EXISTS aegis_connectors (
    id SERIAL PRIMARY KEY,
    environment_id INT NOT NULL REFERENCES aegis_environments(id) ON DELETE CASCADE,
    connector_type TEXT NOT NULL,
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT false,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (environment_id, connector_type, name)
);
CREATE INDEX IF NOT EXISTS idx_aegis_connectors_env ON aegis_connectors (environment_id);

CREATE TABLE IF NOT EXISTS aegis_secrets (
    id SERIAL PRIMARY KEY,
    environment_id INT NOT NULL REFERENCES aegis_environments(id) ON DELETE CASCADE,
    provider TEXT NOT NULL DEFAULT 'custom',
    key_name TEXT NOT NULL,
    value_ciphertext TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (environment_id, key_name)
);
CREATE INDEX IF NOT EXISTS idx_aegis_secrets_env ON aegis_secrets (environment_id);

-- Default tenant + aegis project + dev/qa/prod
INSERT INTO aegis_tenants (slug, name) VALUES ('syntropix', 'Syntropix')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO aegis_projects (tenant_id, slug, name, description)
SELECT t.id, 'aegis', 'AegisOps', 'Autonomous DevOps orchestrator'
FROM aegis_tenants t WHERE t.slug = 'syntropix'
ON CONFLICT (tenant_id, slug) DO NOTHING;

INSERT INTO aegis_environments (project_id, slug, name, sort_order)
SELECT p.id, v.slug, v.name, v.ord
FROM aegis_projects p
CROSS JOIN (VALUES ('dev', 'Development', 1), ('qa', 'QA', 2), ('prod', 'Production', 3)) AS v(slug, name, ord)
WHERE p.slug = 'aegis'
ON CONFLICT (project_id, slug) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS aegis_secrets;
DROP TABLE IF EXISTS aegis_connectors;
DROP TABLE IF EXISTS aegis_environments;
DROP TABLE IF EXISTS aegis_projects;
DROP TABLE IF EXISTS aegis_tenants;
-- +goose StatementEnd
