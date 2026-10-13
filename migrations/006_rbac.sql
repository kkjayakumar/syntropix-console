-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS aegis_users (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS aegis_groups (
    id SERIAL PRIMARY KEY,
    tenant_id INT NOT NULL REFERENCES aegis_tenants(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, slug)
);

CREATE TABLE IF NOT EXISTS aegis_group_members (
    group_id INT NOT NULL REFERENCES aegis_groups(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES aegis_users(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);

-- role: admin | maintainer | developer | readonly
CREATE TABLE IF NOT EXISTS aegis_project_access (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL REFERENCES aegis_projects(id) ON DELETE CASCADE,
    principal_type TEXT NOT NULL CHECK (principal_type IN ('user', 'group')),
    principal_id INT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'maintainer', 'developer', 'readonly')),
    chat_limit_daily INT NOT NULL DEFAULT 50,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, principal_type, principal_id)
);

CREATE TABLE IF NOT EXISTS aegis_chat_usage (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES aegis_users(id) ON DELETE CASCADE,
    project_id INT NOT NULL REFERENCES aegis_projects(id) ON DELETE CASCADE,
    usage_date DATE NOT NULL DEFAULT CURRENT_DATE,
    message_count INT NOT NULL DEFAULT 0,
    UNIQUE (user_id, project_id, usage_date)
);

CREATE TABLE IF NOT EXISTS aegis_env_variables (
    id SERIAL PRIMARY KEY,
    environment_id INT NOT NULL REFERENCES aegis_environments(id) ON DELETE CASCADE,
    key_name TEXT NOT NULL,
    value_text TEXT NOT NULL DEFAULT '',
    is_secret BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (environment_id, key_name)
);

CREATE TABLE IF NOT EXISTS aegis_pipelines (
    id SERIAL PRIMARY KEY,
    environment_id INT NOT NULL UNIQUE REFERENCES aegis_environments(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT 'closed-loop',
    definition JSONB NOT NULL DEFAULT '{}',
    updated_by INT REFERENCES aegis_users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS aegis_audit_events (
    id SERIAL PRIMARY KEY,
    actor_user_id INT REFERENCES aegis_users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL DEFAULT '',
    project_id INT REFERENCES aegis_projects(id) ON DELETE SET NULL,
    environment_id INT REFERENCES aegis_environments(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_aegis_audit_created ON aegis_audit_events (created_at DESC);

-- Seed default admin + access on aegis project
INSERT INTO aegis_users (email, display_name)
VALUES ('admin@syntropix.io', 'Platform Admin')
ON CONFLICT (email) DO NOTHING;

INSERT INTO aegis_groups (tenant_id, slug, name)
SELECT t.id, 'platform-admins', 'Platform Admins'
FROM aegis_tenants t WHERE t.slug = 'syntropix'
ON CONFLICT (tenant_id, slug) DO NOTHING;

INSERT INTO aegis_group_members (group_id, user_id)
SELECT g.id, u.id
FROM aegis_groups g
JOIN aegis_tenants t ON t.id = g.tenant_id AND t.slug = 'syntropix'
JOIN aegis_users u ON u.email = 'admin@syntropix.io'
WHERE g.slug = 'platform-admins'
ON CONFLICT DO NOTHING;

INSERT INTO aegis_project_access (project_id, principal_type, principal_id, role, chat_limit_daily)
SELECT p.id, 'user', u.id, 'admin', 1000
FROM aegis_projects p
JOIN aegis_tenants t ON t.id = p.tenant_id AND t.slug = 'syntropix'
JOIN aegis_users u ON u.email = 'admin@syntropix.io'
WHERE p.slug = 'aegis'
ON CONFLICT (project_id, principal_type, principal_id) DO NOTHING;

INSERT INTO aegis_pipelines (environment_id, name, definition)
SELECT e.id, 'closed-loop', '{"steps":["detect","correlate","approve","llm_or_mcp","branch","pr","ci","deploy","verify","sentinel"],"editableBy":"admin"}'::jsonb
FROM aegis_environments e
JOIN aegis_projects p ON p.id = e.project_id AND p.slug = 'aegis'
ON CONFLICT (environment_id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS aegis_audit_events;
DROP TABLE IF EXISTS aegis_pipelines;
DROP TABLE IF EXISTS aegis_env_variables;
DROP TABLE IF EXISTS aegis_chat_usage;
DROP TABLE IF EXISTS aegis_project_access;
DROP TABLE IF EXISTS aegis_group_members;
DROP TABLE IF EXISTS aegis_groups;
DROP TABLE IF EXISTS aegis_users;
-- +goose StatementEnd
