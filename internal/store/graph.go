package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
)

type GraphNode struct {
	ID             int            `json:"id"`
	EnvironmentID  int            `json:"environmentId"`
	Kind           string         `json:"kind"`
	NodeKey        string         `json:"nodeKey"`
	Title          string         `json:"title"`
	Summary        string         `json:"summary"`
	Metadata       map[string]any `json:"metadata"`
	TokensEstimate int            `json:"tokensEstimate"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type GraphEdge struct {
	ID            int       `json:"id"`
	EnvironmentID int       `json:"environmentId"`
	FromNodeID    int       `json:"fromNodeId"`
	ToNodeID      int       `json:"toNodeId"`
	Relation      string    `json:"relation"`
	CreatedAt     time.Time `json:"createdAt"`
}

type GraphSnapshot struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
	Stats GraphStats  `json:"stats"`
}

type GraphStats struct {
	NodeCount      int `json:"nodeCount"`
	EdgeCount      int `json:"edgeCount"`
	TokensEstimate int `json:"tokensEstimate"`
}

type LLMRun struct {
	ID               int       `json:"id"`
	EnvironmentID    int       `json:"environmentId"`
	IssueID          *int      `json:"issueId,omitempty"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	GraphNodesUsed   int       `json:"graphNodesUsed"`
	Status           string    `json:"status"`
	ResponseSummary  string    `json:"responseSummary"`
	CreatedAt        time.Time `json:"createdAt"`
}

type GraphStore struct {
	pool *pgxpool.Pool
}

func NewGraphStore(pool *pgxpool.Pool) *GraphStore {
	return &GraphStore{pool: pool}
}

func EstimateTokens(s string) int {
	n := utf8.RuneCountInString(s)
	if n == 0 {
		return 0
	}
	return (n + 3) / 4
}

// ErrGraphIsolation is returned when a read/write would cross environment boundaries.
var ErrGraphIsolation = fmt.Errorf("graph isolation violation: data does not belong to the requested environment")

// assertNodesBelongToEnvironment rejects any node whose environment_id differs from the request scope.
func assertNodesBelongToEnvironment(nodes []GraphNode, environmentID int) error {
	for _, n := range nodes {
		if n.EnvironmentID != environmentID {
			return ErrGraphIsolation
		}
	}
	return nil
}

func assertEdgesBelongToEnvironment(edges []GraphEdge, environmentID int) error {
	for _, e := range edges {
		if e.EnvironmentID != environmentID {
			return ErrGraphIsolation
		}
	}
	return nil
}

func (s *GraphStore) Snapshot(ctx context.Context, environmentID int) (GraphSnapshot, error) {
	if s == nil || s.pool == nil {
		return GraphSnapshot{}, fmt.Errorf("database not available")
	}
	if environmentID <= 0 {
		return GraphSnapshot{}, fmt.Errorf("environment id is required")
	}
	out := GraphSnapshot{}
	rows, err := s.pool.Query(ctx, `
		SELECT id, environment_id, kind, node_key, title, summary, metadata, tokens_estimate, created_at, updated_at
		FROM aegis_graph_nodes WHERE environment_id = $1 ORDER BY kind, title
	`, environmentID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var n GraphNode
		var raw []byte
		if err := rows.Scan(&n.ID, &n.EnvironmentID, &n.Kind, &n.NodeKey, &n.Title, &n.Summary, &raw, &n.TokensEstimate, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return out, err
		}
		n.Metadata = map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &n.Metadata)
		}
		out.Nodes = append(out.Nodes, n)
		out.Stats.TokensEstimate += n.TokensEstimate
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if err := assertNodesBelongToEnvironment(out.Nodes, environmentID); err != nil {
		return GraphSnapshot{}, err
	}
	out.Stats.NodeCount = len(out.Nodes)

	erows, err := s.pool.Query(ctx, `
		SELECT id, environment_id, from_node_id, to_node_id, relation, created_at
		FROM aegis_graph_edges WHERE environment_id = $1 ORDER BY id
	`, environmentID)
	if err != nil {
		return out, err
	}
	defer erows.Close()
	for erows.Next() {
		var e GraphEdge
		if err := erows.Scan(&e.ID, &e.EnvironmentID, &e.FromNodeID, &e.ToNodeID, &e.Relation, &e.CreatedAt); err != nil {
			return out, err
		}
		out.Edges = append(out.Edges, e)
	}
	if err := erows.Err(); err != nil {
		return out, err
	}
	if err := assertEdgesBelongToEnvironment(out.Edges, environmentID); err != nil {
		return GraphSnapshot{}, err
	}
	out.Stats.EdgeCount = len(out.Edges)
	return out, nil
}

func (s *GraphStore) UpsertNode(ctx context.Context, environmentID int, kind, key, title, summary string, metadata map[string]any) (GraphNode, error) {
	if s == nil || s.pool == nil {
		return GraphNode{}, fmt.Errorf("database not available")
	}
	if environmentID <= 0 {
		return GraphNode{}, fmt.Errorf("environment id is required")
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return GraphNode{}, err
	}
	tokens := EstimateTokens(title + "\n" + summary)
	var n GraphNode
	var metaOut []byte
	err = s.pool.QueryRow(ctx, `
		INSERT INTO aegis_graph_nodes (environment_id, kind, node_key, title, summary, metadata, tokens_estimate)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (environment_id, kind, node_key) DO UPDATE
		SET title = EXCLUDED.title, summary = EXCLUDED.summary, metadata = EXCLUDED.metadata,
		    tokens_estimate = EXCLUDED.tokens_estimate, updated_at = NOW()
		RETURNING id, environment_id, kind, node_key, title, summary, metadata, tokens_estimate, created_at, updated_at
	`, environmentID, kind, key, title, summary, raw, tokens).Scan(
		&n.ID, &n.EnvironmentID, &n.Kind, &n.NodeKey, &n.Title, &n.Summary, &metaOut, &n.TokensEstimate, &n.CreatedAt, &n.UpdatedAt,
	)
	n.Metadata = map[string]any{}
	if len(metaOut) > 0 {
		_ = json.Unmarshal(metaOut, &n.Metadata)
	}
	return n, err
}

func (s *GraphStore) UpsertEdge(ctx context.Context, environmentID, fromID, toID int, relation string) (GraphEdge, error) {
	if s == nil || s.pool == nil {
		return GraphEdge{}, fmt.Errorf("database not available")
	}
	if environmentID <= 0 {
		return GraphEdge{}, fmt.Errorf("environment id is required")
	}
	if relation == "" {
		relation = "related_to"
	}
	// Refuse edges that would link nodes from another environment (cross-project leak).
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM aegis_graph_nodes
		WHERE environment_id = $1 AND id IN ($2, $3)
	`, environmentID, fromID, toID).Scan(&n)
	if err != nil {
		return GraphEdge{}, err
	}
	want := 2
	if fromID == toID {
		want = 1
	}
	if n != want {
		return GraphEdge{}, ErrGraphIsolation
	}
	var e GraphEdge
	err = s.pool.QueryRow(ctx, `
		INSERT INTO aegis_graph_edges (environment_id, from_node_id, to_node_id, relation)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (environment_id, from_node_id, to_node_id, relation) DO UPDATE SET relation = EXCLUDED.relation
		RETURNING id, environment_id, from_node_id, to_node_id, relation, created_at
	`, environmentID, fromID, toID, relation).Scan(&e.ID, &e.EnvironmentID, &e.FromNodeID, &e.ToNodeID, &e.Relation, &e.CreatedAt)
	return e, err
}

// graphQueryTokens extracts significant lowercase keywords from a user prompt.
// Full-prompt LIKE almost never matches; per-word matching recovers relevant nodes.
func graphQueryTokens(query string) []string {
	stop := map[string]bool{
		"a": true, "an": true, "the": true, "and": true, "or": true, "to": true, "of": true,
		"in": true, "on": true, "for": true, "with": true, "is": true, "are": true, "be": true,
		"this": true, "that": true, "it": true, "we": true, "me": true, "my": true, "our": true,
		"please": true, "help": true, "about": true, "from": true, "into": true, "by": true,
	}
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '/' || r == '-' || r == '_')
	})
	seen := map[string]bool{}
	out := make([]string, 0, len(fields))
	for _, t := range fields {
		if len(t) < 3 || stop[t] || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
		if len(out) >= 8 {
			break
		}
	}
	return out
}

// SubgraphForQuery returns a compact slice of nodes matching keywords (token-saving retrieval).
// Always scoped to a single environment_id — never merges across projects/envs/tenants.
func (s *GraphStore) SubgraphForQuery(ctx context.Context, environmentID int, query string, limit int) ([]GraphNode, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	if environmentID <= 0 {
		return nil, fmt.Errorf("environment id is required")
	}
	if limit <= 0 {
		limit = 12
	}
	tokens := graphQueryTokens(query)
	if len(tokens) == 0 {
		rows, err := s.pool.Query(ctx, `
			SELECT id, environment_id, kind, node_key, title, summary, metadata, tokens_estimate, created_at, updated_at
			FROM aegis_graph_nodes WHERE environment_id = $1
			ORDER BY tokens_estimate ASC, updated_at DESC LIMIT $2
		`, environmentID, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		nodes, err := scanGraphNodes(rows)
		if err != nil {
			return nil, err
		}
		return nodes, assertNodesBelongToEnvironment(nodes, environmentID)
	}

	// Match any significant keyword so prompts retrieve related modules/issues, not empty context.
	var b strings.Builder
	args := []any{environmentID}
	b.WriteString(`
		SELECT id, environment_id, kind, node_key, title, summary, metadata, tokens_estimate, created_at, updated_at
		FROM aegis_graph_nodes
		WHERE environment_id = $1 AND (`)
	for i, tok := range tokens {
		if i > 0 {
			b.WriteString(" OR ")
		}
		args = append(args, "%"+tok+"%")
		n := len(args)
		fmt.Fprintf(&b, "lower(title) LIKE $%d OR lower(summary) LIKE $%d OR lower(node_key) LIKE $%d OR lower(kind) LIKE $%d", n, n, n, n)
	}
	args = append(args, limit)
	fmt.Fprintf(&b, `) ORDER BY tokens_estimate ASC, updated_at DESC LIMIT $%d`, len(args))

	rows, err := s.pool.Query(ctx, b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes, err := scanGraphNodes(rows)
	if err != nil {
		return nil, err
	}
	if err := assertNodesBelongToEnvironment(nodes, environmentID); err != nil {
		return nil, err
	}
	if len(nodes) > 0 {
		return nodes, nil
	}
	// Fallback: newest compact nodes so LLM always has some project context (same env only).
	rows2, err := s.pool.Query(ctx, `
		SELECT id, environment_id, kind, node_key, title, summary, metadata, tokens_estimate, created_at, updated_at
		FROM aegis_graph_nodes WHERE environment_id = $1
		ORDER BY tokens_estimate ASC, updated_at DESC LIMIT $2
	`, environmentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	nodes, err = scanGraphNodes(rows2)
	if err != nil {
		return nil, err
	}
	return nodes, assertNodesBelongToEnvironment(nodes, environmentID)
}

func scanGraphNodes(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]GraphNode, error) {
	out := []GraphNode{}
	for rows.Next() {
		var n GraphNode
		var raw []byte
		if err := rows.Scan(&n.ID, &n.EnvironmentID, &n.Kind, &n.NodeKey, &n.Title, &n.Summary, &raw, &n.TokensEstimate, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		n.Metadata = map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &n.Metadata)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *GraphStore) RecordLLMRun(ctx context.Context, run LLMRun) (LLMRun, error) {
	if s == nil || s.pool == nil {
		return LLMRun{}, fmt.Errorf("database not available")
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_llm_runs (
			environment_id, issue_id, provider, model, prompt_tokens, completion_tokens,
			graph_nodes_used, status, response_summary
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, environment_id, issue_id, provider, model, prompt_tokens, completion_tokens,
		          graph_nodes_used, status, response_summary, created_at
	`, run.EnvironmentID, run.IssueID, run.Provider, run.Model, run.PromptTokens, run.CompletionTokens,
		run.GraphNodesUsed, run.Status, run.ResponseSummary,
	).Scan(&run.ID, &run.EnvironmentID, &run.IssueID, &run.Provider, &run.Model, &run.PromptTokens, &run.CompletionTokens,
		&run.GraphNodesUsed, &run.Status, &run.ResponseSummary, &run.CreatedAt)
	return run, err
}

func (s *GraphStore) ListLLMRuns(ctx context.Context, environmentID, limit int) ([]LLMRun, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	if environmentID <= 0 {
		return nil, fmt.Errorf("environment id is required")
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, environment_id, issue_id, provider, model, prompt_tokens, completion_tokens,
		       graph_nodes_used, status, response_summary, created_at
		FROM aegis_llm_runs WHERE environment_id = $1 ORDER BY created_at DESC LIMIT $2
	`, environmentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LLMRun{}
	for rows.Next() {
		var r LLMRun
		if err := rows.Scan(&r.ID, &r.EnvironmentID, &r.IssueID, &r.Provider, &r.Model, &r.PromptTokens, &r.CompletionTokens,
			&r.GraphNodesUsed, &r.Status, &r.ResponseSummary, &r.CreatedAt); err != nil {
			return nil, err
		}
		if r.EnvironmentID != environmentID {
			return nil, ErrGraphIsolation
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetSecretCiphertext returns encrypted value for internal decrypt (never expose raw via API).
func (s *WorkspaceStore) GetSecretCiphertext(ctx context.Context, environmentID int, keyName string) (string, error) {
	if s == nil || s.pool == nil {
		return "", fmt.Errorf("database not available")
	}
	var cipher string
	err := s.pool.QueryRow(ctx, `
		SELECT value_ciphertext FROM aegis_secrets
		WHERE environment_id = $1 AND key_name = $2
	`, environmentID, strings.ToUpper(strings.TrimSpace(keyName))).Scan(&cipher)
	return cipher, err
}

func (s *WorkspaceStore) ListSecretKeyNames(ctx context.Context, environmentID int) ([]string, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT key_name FROM aegis_secrets WHERE environment_id = $1 ORDER BY key_name
	`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
