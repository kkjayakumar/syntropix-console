-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS aegis_graph_nodes (
    id SERIAL PRIMARY KEY,
    environment_id INT NOT NULL REFERENCES aegis_environments(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    node_key TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}',
    tokens_estimate INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (environment_id, kind, node_key)
);
CREATE INDEX IF NOT EXISTS idx_aegis_graph_nodes_env ON aegis_graph_nodes (environment_id);
CREATE INDEX IF NOT EXISTS idx_aegis_graph_nodes_kind ON aegis_graph_nodes (environment_id, kind);

CREATE TABLE IF NOT EXISTS aegis_graph_edges (
    id SERIAL PRIMARY KEY,
    environment_id INT NOT NULL REFERENCES aegis_environments(id) ON DELETE CASCADE,
    from_node_id INT NOT NULL REFERENCES aegis_graph_nodes(id) ON DELETE CASCADE,
    to_node_id INT NOT NULL REFERENCES aegis_graph_nodes(id) ON DELETE CASCADE,
    relation TEXT NOT NULL DEFAULT 'related_to',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (environment_id, from_node_id, to_node_id, relation)
);
CREATE INDEX IF NOT EXISTS idx_aegis_graph_edges_env ON aegis_graph_edges (environment_id);

CREATE TABLE IF NOT EXISTS aegis_llm_runs (
    id SERIAL PRIMARY KEY,
    environment_id INT NOT NULL REFERENCES aegis_environments(id) ON DELETE CASCADE,
    issue_id INT REFERENCES aegis_issues(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL DEFAULT '',
    prompt_tokens INT NOT NULL DEFAULT 0,
    completion_tokens INT NOT NULL DEFAULT 0,
    graph_nodes_used INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'ok',
    response_summary TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_aegis_llm_runs_env ON aegis_llm_runs (environment_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS aegis_llm_runs;
DROP TABLE IF EXISTS aegis_graph_edges;
DROP TABLE IF EXISTS aegis_graph_nodes;
-- +goose StatementEnd
