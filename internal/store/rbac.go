package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RoleAdmin      = "admin"
	RoleMaintainer = "maintainer"
	RoleDeveloper  = "developer"
	RoleReadonly   = "readonly"
)

var roleRank = map[string]int{
	RoleReadonly:   1,
	RoleDeveloper:  2,
	RoleMaintainer: 3,
	RoleAdmin:      4,
}

type User struct {
	ID          int       `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Group struct {
	ID        int       `json:"id"`
	TenantID  int       `json:"tenantId"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type ProjectAccess struct {
	ID             int       `json:"id"`
	ProjectID      int       `json:"projectId"`
	PrincipalType  string    `json:"principalType"`
	PrincipalID    int       `json:"principalId"`
	PrincipalLabel string    `json:"principalLabel,omitempty"`
	Role           string    `json:"role"`
	ChatLimitDaily int       `json:"chatLimitDaily"`
	CreatedAt      time.Time `json:"createdAt"`
}

type AccessDecision struct {
	UserID         int    `json:"userId"`
	ProjectID      int    `json:"projectId"`
	Role           string `json:"role"`
	ChatLimitDaily int    `json:"chatLimitDaily"`
	ChatUsedToday  int    `json:"chatUsedToday"`
	CanChat        bool   `json:"canChat"`
	CanWriteSecret bool   `json:"canWriteSecret"`
	CanEditPipeline bool  `json:"canEditPipeline"`
	CanManageRBAC  bool   `json:"canManageRBAC"`
	CanReadSecretValue bool `json:"canReadSecretValue"` // always false — SOC2
	Policies       []RolePolicy `json:"policies"`
}

type RolePolicy struct {
	Role        string   `json:"role"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Allows      []string `json:"allows"`
	Denies      []string `json:"denies"`
}

func DefaultRolePolicies() []RolePolicy {
	return []RolePolicy{
		{
			Role: RoleAdmin, Label: "Admin",
			Description: "Full project control except reading secret plaintext (SOC2).",
			Allows: []string{"manage RBAC", "edit pipeline", "write secrets (names+values in)", "chat unlimited/high quota", "connectors", "envs"},
			Denies: []string{"read secret plaintext after save"},
		},
		{
			Role: RoleMaintainer, Label: "Maintainer",
			Description: "Operate integrations and rotate secrets; cannot change pipeline or RBAC.",
			Allows: []string{"write secrets", "connectors", "chat", "view pipeline"},
			Denies: []string{"edit pipeline", "manage RBAC", "read secret plaintext"},
		},
		{
			Role: RoleDeveloper, Label: "Developer",
			Description: "Build/fix via chat and issues within daily chat quota.",
			Allows: []string{"chat (quota)", "view secrets metadata", "view pipeline", "issues"},
			Denies: []string{"write secrets", "edit pipeline", "RBAC", "read secret plaintext"},
		},
		{
			Role: RoleReadonly, Label: "Read only",
			Description: "Observe project health; no chat mutations or secret writes.",
			Allows: []string{"view project", "view pipeline", "view secret names"},
			Denies: []string{"chat", "write secrets", "edit anything", "read secret plaintext"},
		},
	}
}

type RBACStore struct {
	pool *pgxpool.Pool
}

func NewRBACStore(pool *pgxpool.Pool) *RBACStore {
	return &RBACStore{pool: pool}
}

func (s *RBACStore) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, display_name, status, created_at FROM aegis_users WHERE email = $1
	`, strings.ToLower(strings.TrimSpace(email))).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt)
	return u, err
}

func (s *RBACStore) GetUserByID(ctx context.Context, id int) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, display_name, status, created_at FROM aegis_users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt)
	return u, err
}

func (s *RBACStore) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, email, display_name, status, created_at FROM aegis_users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *RBACStore) CreateUser(ctx context.Context, email, name string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_users (email, display_name) VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id, email, display_name, status, created_at
	`, strings.ToLower(strings.TrimSpace(email)), strings.TrimSpace(name)).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &u.CreatedAt)
	return u, err
}

func (s *RBACStore) ListGroups(ctx context.Context, tenantSlug string) ([]Group, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT g.id, g.tenant_id, g.slug, g.name, g.created_at
		FROM aegis_groups g JOIN aegis_tenants t ON t.id = g.tenant_id
		WHERE t.slug = $1 ORDER BY g.slug
	`, tenantSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.TenantID, &g.Slug, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *RBACStore) CreateGroup(ctx context.Context, tenantSlug, slug, name string) (Group, error) {
	var g Group
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_groups (tenant_id, slug, name)
		SELECT t.id, $2, $3 FROM aegis_tenants t WHERE t.slug = $1
		ON CONFLICT (tenant_id, slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, tenant_id, slug, name, created_at
	`, tenantSlug, slug, name).Scan(&g.ID, &g.TenantID, &g.Slug, &g.Name, &g.CreatedAt)
	return g, err
}

func (s *RBACStore) AddGroupMember(ctx context.Context, groupID, userID int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO aegis_group_members (group_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING
	`, groupID, userID)
	return err
}

func (s *RBACStore) ListProjectAccess(ctx context.Context, projectID int) ([]ProjectAccess, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.project_id, a.principal_type, a.principal_id, a.role, a.chat_limit_daily, a.created_at,
			COALESCE(u.email, g.name, '')
		FROM aegis_project_access a
		LEFT JOIN aegis_users u ON a.principal_type = 'user' AND u.id = a.principal_id
		LEFT JOIN aegis_groups g ON a.principal_type = 'group' AND g.id = a.principal_id
		WHERE a.project_id = $1 ORDER BY a.role DESC, a.id
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ProjectAccess{}
	for rows.Next() {
		var a ProjectAccess
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.PrincipalType, &a.PrincipalID, &a.Role, &a.ChatLimitDaily, &a.CreatedAt, &a.PrincipalLabel); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *RBACStore) UpsertProjectAccess(ctx context.Context, projectID int, principalType string, principalID int, role string, chatLimit int) (ProjectAccess, error) {
	if chatLimit <= 0 {
		switch role {
		case RoleAdmin:
			chatLimit = 1000
		case RoleMaintainer:
			chatLimit = 200
		case RoleDeveloper:
			chatLimit = 50
		default:
			chatLimit = 0
		}
	}
	var a ProjectAccess
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_project_access (project_id, principal_type, principal_id, role, chat_limit_daily)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (project_id, principal_type, principal_id) DO UPDATE
		SET role = EXCLUDED.role, chat_limit_daily = EXCLUDED.chat_limit_daily
		RETURNING id, project_id, principal_type, principal_id, role, chat_limit_daily, created_at
	`, projectID, principalType, principalID, role, chatLimit).Scan(
		&a.ID, &a.ProjectID, &a.PrincipalType, &a.PrincipalID, &a.Role, &a.ChatLimitDaily, &a.CreatedAt,
	)
	return a, err
}

func (s *RBACStore) DeleteProjectAccess(ctx context.Context, id int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM aegis_project_access WHERE id = $1`, id)
	return err
}

func (s *RBACStore) EffectiveAccess(ctx context.Context, userID, projectID int) (AccessDecision, error) {
	out := AccessDecision{
		UserID:             userID,
		ProjectID:          projectID,
		CanReadSecretValue: false,
		Policies:           DefaultRolePolicies(),
	}
	rows, err := s.pool.Query(ctx, `
		SELECT role, chat_limit_daily FROM aegis_project_access
		WHERE project_id = $1 AND principal_type = 'user' AND principal_id = $2
		UNION ALL
		SELECT a.role, a.chat_limit_daily FROM aegis_project_access a
		JOIN aegis_group_members m ON a.principal_type = 'group' AND a.principal_id = m.group_id
		WHERE a.project_id = $1 AND m.user_id = $2
	`, projectID, userID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	best := 0
	limit := 0
	for rows.Next() {
		var role string
		var chatLimit int
		if err := rows.Scan(&role, &chatLimit); err != nil {
			return out, err
		}
		if roleRank[role] > best {
			best = roleRank[role]
			out.Role = role
			limit = chatLimit
		}
		if chatLimit > limit {
			limit = chatLimit
		}
	}
	if out.Role == "" {
		return out, fmt.Errorf("no access to project")
	}
	out.ChatLimitDaily = limit
	out.CanChat = out.Role != RoleReadonly && limit > 0
	out.CanWriteSecret = out.Role == RoleAdmin || out.Role == RoleMaintainer
	out.CanEditPipeline = out.Role == RoleAdmin
	out.CanManageRBAC = out.Role == RoleAdmin

	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(message_count, 0) FROM aegis_chat_usage
		WHERE user_id = $1 AND project_id = $2 AND usage_date = CURRENT_DATE
	`, userID, projectID).Scan(&out.ChatUsedToday)
	if out.CanChat && out.ChatUsedToday >= out.ChatLimitDaily {
		out.CanChat = false
	}
	return out, nil
}

func (s *RBACStore) IncrementChatUsage(ctx context.Context, userID, projectID int) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO aegis_chat_usage (user_id, project_id, usage_date, message_count)
		VALUES ($1, $2, CURRENT_DATE, 1)
		ON CONFLICT (user_id, project_id, usage_date) DO UPDATE
		SET message_count = aegis_chat_usage.message_count + 1
	`, userID, projectID)
	return err
}

func (s *RBACStore) Audit(ctx context.Context, actorID *int, action, resourceType, resourceID string, projectID, envID *int, meta map[string]any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	raw, _ := json.Marshal(meta)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO aegis_audit_events (actor_user_id, action, resource_type, resource_id, project_id, environment_id, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, actorID, action, resourceType, resourceID, projectID, envID, raw)
	return err
}

type Pipeline struct {
	ID            int            `json:"id"`
	EnvironmentID int            `json:"environmentId"`
	Name          string         `json:"name"`
	Definition    map[string]any `json:"definition"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

func (s *RBACStore) GetPipeline(ctx context.Context, envID int) (Pipeline, error) {
	var p Pipeline
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT id, environment_id, name, definition, updated_at FROM aegis_pipelines WHERE environment_id = $1
	`, envID).Scan(&p.ID, &p.EnvironmentID, &p.Name, &raw, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		def := map[string]any{
			"steps":      []string{"detect", "correlate", "approve", "llm_or_mcp", "branch", "pr", "ci", "deploy", "verify", "sentinel"},
			"editableBy": "admin",
		}
		raw, _ = json.Marshal(def)
		err = s.pool.QueryRow(ctx, `
			INSERT INTO aegis_pipelines (environment_id, name, definition) VALUES ($1, 'closed-loop', $2)
			ON CONFLICT (environment_id) DO UPDATE SET name = aegis_pipelines.name
			RETURNING id, environment_id, name, definition, updated_at
		`, envID, raw).Scan(&p.ID, &p.EnvironmentID, &p.Name, &raw, &p.UpdatedAt)
	}
	p.Definition = map[string]any{}
	_ = json.Unmarshal(raw, &p.Definition)
	return p, err
}

func (s *RBACStore) UpsertPipeline(ctx context.Context, envID int, name string, def map[string]any, userID int) (Pipeline, error) {
	if def == nil {
		def = map[string]any{}
	}
	raw, err := json.Marshal(def)
	if err != nil {
		return Pipeline{}, err
	}
	var p Pipeline
	var out []byte
	err = s.pool.QueryRow(ctx, `
		INSERT INTO aegis_pipelines (environment_id, name, definition, updated_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (environment_id) DO UPDATE
		SET name = EXCLUDED.name, definition = EXCLUDED.definition, updated_by = EXCLUDED.updated_by, updated_at = NOW()
		RETURNING id, environment_id, name, definition, updated_at
	`, envID, name, raw, userID).Scan(&p.ID, &p.EnvironmentID, &p.Name, &out, &p.UpdatedAt)
	p.Definition = map[string]any{}
	_ = json.Unmarshal(out, &p.Definition)
	return p, err
}

type EnvVariable struct {
	ID            int       `json:"id"`
	EnvironmentID int       `json:"environmentId"`
	KeyName       string    `json:"keyName"`
	ValueText     string    `json:"valueText,omitempty"`
	IsSecret      bool      `json:"isSecret"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (s *RBACStore) ListEnvVariables(ctx context.Context, envID int) ([]EnvVariable, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, environment_id, key_name, value_text, is_secret, updated_at
		FROM aegis_env_variables WHERE environment_id = $1 AND is_secret = false ORDER BY key_name
	`, envID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EnvVariable{}
	for rows.Next() {
		var v EnvVariable
		if err := rows.Scan(&v.ID, &v.EnvironmentID, &v.KeyName, &v.ValueText, &v.IsSecret, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *RBACStore) UpsertEnvVariable(ctx context.Context, envID int, key, value string) (EnvVariable, error) {
	var v EnvVariable
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_env_variables (environment_id, key_name, value_text, is_secret)
		VALUES ($1, $2, $3, false)
		ON CONFLICT (environment_id, key_name) DO UPDATE
		SET value_text = EXCLUDED.value_text, updated_at = NOW()
		RETURNING id, environment_id, key_name, value_text, is_secret, updated_at
	`, envID, strings.ToUpper(strings.TrimSpace(key)), value).Scan(
		&v.ID, &v.EnvironmentID, &v.KeyName, &v.ValueText, &v.IsSecret, &v.UpdatedAt,
	)
	return v, err
}
