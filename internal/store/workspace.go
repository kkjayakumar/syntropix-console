package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ConnectorGit         = "git"
	ConnectorPullRequest = "pull_request"
	ConnectorCICD        = "cicd"
	ConnectorDeployment  = "deployment"
	ConnectorTelemetry   = "telemetry"
	ConnectorMCP         = "mcp"
	ConnectorIssueAgent  = "issue_agent"
)

var ConnectorTypes = []string{
	ConnectorGit, ConnectorPullRequest, ConnectorCICD, ConnectorDeployment,
	ConnectorTelemetry, ConnectorMCP, ConnectorIssueAgent,
}

var SecretProviders = []string{
	"openai", "anthropic", "google", "cursor", "github", "aws", "webhook", "custom",
}

type Tenant struct {
	ID        int       `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Project struct {
	ID          int       `json:"id"`
	TenantID    int       `json:"tenantId"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Environment struct {
	ID        int       `json:"id"`
	ProjectID int       `json:"projectId"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
}

type Connector struct {
	ID            int            `json:"id"`
	EnvironmentID int            `json:"environmentId"`
	ConnectorType string         `json:"connectorType"`
	Name          string         `json:"name"`
	Enabled       bool           `json:"enabled"`
	Config        map[string]any `json:"config"`
	Ready         bool           `json:"ready"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

// ConnectorDisplayNames are the canonical stage labels used by the workspace UI.
var ConnectorDisplayNames = map[string]string{
	ConnectorGit:         "Source",
	ConnectorPullRequest: "Pull request",
	ConnectorCICD:        "CI / CD",
	ConnectorDeployment:  "Deploy",
	ConnectorTelemetry:   "Telemetry",
	ConnectorMCP:         "IDE / MCP",
	ConnectorIssueAgent:  "AI agent",
}

type Secret struct {
	ID            int       `json:"id"`
	EnvironmentID int       `json:"environmentId"`
	Provider      string    `json:"provider"`
	KeyName       string    `json:"keyName"`
	MaskedValue   string    `json:"maskedValue"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type WorkspaceBootstrap struct {
	Tenant       Tenant        `json:"tenant"`
	Projects     []Project     `json:"projects"`
	Environments []Environment `json:"environments"`
}

type WorkspaceStore struct {
	pool *pgxpool.Pool
}

func NewWorkspaceStore(pool *pgxpool.Pool) *WorkspaceStore {
	return &WorkspaceStore{pool: pool}
}

func (s *WorkspaceStore) Bootstrap(ctx context.Context, tenantSlug string) (WorkspaceBootstrap, error) {
	if s == nil || s.pool == nil {
		return WorkspaceBootstrap{}, fmt.Errorf("database not available")
	}
	if tenantSlug == "" {
		tenantSlug = "syntropix"
	}
	var out WorkspaceBootstrap
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, name, created_at FROM aegis_tenants WHERE slug = $1
	`, tenantSlug).Scan(&out.Tenant.ID, &out.Tenant.Slug, &out.Tenant.Name, &out.Tenant.CreatedAt)
	if err != nil {
		return out, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, slug, name, description, created_at
		FROM aegis_projects WHERE tenant_id = $1 ORDER BY slug
	`, out.Tenant.ID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return out, err
		}
		out.Projects = append(out.Projects, p)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if len(out.Projects) == 0 {
		return out, nil
	}
	ids := make([]int, len(out.Projects))
	for i, p := range out.Projects {
		ids[i] = p.ID
	}
	envRows, err := s.pool.Query(ctx, `
		SELECT id, project_id, slug, name, sort_order, created_at
		FROM aegis_environments WHERE project_id = ANY($1) ORDER BY sort_order, slug
	`, ids)
	if err != nil {
		return out, err
	}
	defer envRows.Close()
	for envRows.Next() {
		var e Environment
		if err := envRows.Scan(&e.ID, &e.ProjectID, &e.Slug, &e.Name, &e.SortOrder, &e.CreatedAt); err != nil {
			return out, err
		}
		out.Environments = append(out.Environments, e)
	}
	return out, envRows.Err()
}

func (s *WorkspaceStore) ListProjects(ctx context.Context, tenantSlug string) ([]Project, error) {
	boot, err := s.Bootstrap(ctx, tenantSlug)
	if err != nil {
		return nil, err
	}
	return boot.Projects, nil
}

func (s *WorkspaceStore) CreateProject(ctx context.Context, tenantSlug, slug, name, description string) (Project, error) {
	if s == nil || s.pool == nil {
		return Project{}, fmt.Errorf("database not available")
	}
	var p Project
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_projects (tenant_id, slug, name, description)
		SELECT t.id, $2, $3, NULLIF($4, '')
		FROM aegis_tenants t WHERE t.slug = $1
		ON CONFLICT (tenant_id, slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, tenant_id, slug, name, description, created_at
	`, tenantSlug, slug, name, description).Scan(&p.ID, &p.TenantID, &p.Slug, &p.Name, &p.Description, &p.CreatedAt)
	return p, err
}

func (s *WorkspaceStore) ListEnvironments(ctx context.Context, projectID int) ([]Environment, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, project_id, slug, name, sort_order, created_at
		FROM aegis_environments WHERE project_id = $1 ORDER BY sort_order, slug
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Environment{}
	for rows.Next() {
		var e Environment
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Slug, &e.Name, &e.SortOrder, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *WorkspaceStore) CreateEnvironment(ctx context.Context, projectID int, slug, name string, sortOrder int) (Environment, error) {
	if s == nil || s.pool == nil {
		return Environment{}, fmt.Errorf("database not available")
	}
	var e Environment
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_environments (project_id, slug, name, sort_order)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (project_id, slug) DO UPDATE SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order
		RETURNING id, project_id, slug, name, sort_order, created_at
	`, projectID, slug, name, sortOrder).Scan(&e.ID, &e.ProjectID, &e.Slug, &e.Name, &e.SortOrder, &e.CreatedAt)
	return e, err
}

// ErrLastEnvironment is returned when deleting would leave a project with zero environments.
var ErrLastEnvironment = fmt.Errorf("cannot delete the last environment for a project")

// DeleteEnvironment removes an environment and cascades connectors/secrets/etc via FK.
// Refuses to delete the project's sole remaining environment.
func (s *WorkspaceStore) DeleteEnvironment(ctx context.Context, environmentID int) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	var projectID int
	err := s.pool.QueryRow(ctx, `SELECT project_id FROM aegis_environments WHERE id = $1`, environmentID).Scan(&projectID)
	if err != nil {
		return fmt.Errorf("environment not found")
	}
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM aegis_environments WHERE project_id = $1`, projectID).Scan(&count); err != nil {
		return err
	}
	if count <= 1 {
		return ErrLastEnvironment
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM aegis_environments WHERE id = $1`, environmentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("environment not found")
	}
	return nil
}

func (s *WorkspaceStore) ListConnectors(ctx context.Context, environmentID int) ([]Connector, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, environment_id, connector_type, name, enabled, config, created_at, updated_at
		FROM aegis_connectors WHERE environment_id = $1 ORDER BY connector_type, name
	`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Connector{}
	for rows.Next() {
		var c Connector
		var raw []byte
		if err := rows.Scan(&c.ID, &c.EnvironmentID, &c.ConnectorType, &c.Name, &c.Enabled, &raw, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Config = map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &c.Config)
		}
		c.Ready = ConnectorReady(c)
		out = append(out, c)
	}
	return out, rows.Err()
}

// CanonicalStageConnector picks the closed-loop stage row for a connector type.
// Prefers the canonical display name, then any ready row, then the first match.
func CanonicalStageConnector(connectors []Connector, connectorType string) (Connector, bool) {
	canonicalName := ConnectorDisplayNames[connectorType]
	var fallback Connector
	var hasFallback bool
	var ready Connector
	var hasReady bool
	for _, c := range connectors {
		if c.ConnectorType != connectorType {
			continue
		}
		if canonicalName != "" && c.Name == canonicalName {
			return c, true
		}
		if ConnectorReady(c) && !hasReady {
			ready = c
			hasReady = true
		}
		if !hasFallback {
			fallback = c
			hasFallback = true
		}
	}
	if hasReady {
		return ready, true
	}
	if hasFallback {
		return fallback, true
	}
	return Connector{}, false
}

// StageConnectors returns one canonical connector per closed-loop stage type.
func StageConnectors(connectors []Connector) map[string]Connector {
	out := make(map[string]Connector, len(ConnectorTypes))
	for _, ctype := range ConnectorTypes {
		if c, ok := CanonicalStageConnector(connectors, ctype); ok {
			out[ctype] = c
		}
	}
	return out
}

// ConnectorReady reports whether a stage is enabled and has required non-secret config.
func ConnectorReady(c Connector) bool {
	if !c.Enabled {
		return false
	}
	cfg := c.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	switch c.ConnectorType {
	case ConnectorGit:
		return configString(cfg, "repository") != ""
	case ConnectorPullRequest:
		return configString(cfg, "baseBranch") != ""
	case ConnectorCICD:
		return configString(cfg, "provider") != "" && configHasNonEmptyList(cfg, "workflows")
	case ConnectorDeployment:
		return configString(cfg, "region") != "" && configString(cfg, "target") != ""
	case ConnectorTelemetry:
		return configString(cfg, "healthUrl") != ""
	case ConnectorMCP:
		return configString(cfg, "ide") != ""
	case ConnectorIssueAgent:
		return configString(cfg, "provider") != "" || configString(cfg, "route") != ""
	default:
		return true
	}
}

func configString(cfg map[string]any, key string) string {
	v, ok := cfg[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func configHasNonEmptyList(cfg map[string]any, key string) bool {
	v, ok := cfg[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case []any:
		return len(t) > 0
	case []string:
		return len(t) > 0
	case string:
		return strings.TrimSpace(t) != ""
	default:
		return false
	}
}

func (s *WorkspaceStore) UpsertConnector(ctx context.Context, environmentID int, connectorType, name string, enabled bool, config map[string]any) (Connector, error) {
	if s == nil || s.pool == nil {
		return Connector{}, fmt.Errorf("database not available")
	}
	if config == nil {
		config = map[string]any{}
	}
	probe := Connector{ConnectorType: connectorType, Enabled: enabled, Config: config}
	if enabled && !ConnectorReady(probe) {
		enabled = false
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return Connector{}, err
	}
	var c Connector
	var cfgOut []byte
	err = s.pool.QueryRow(ctx, `
		INSERT INTO aegis_connectors (environment_id, connector_type, name, enabled, config)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (environment_id, connector_type, name) DO UPDATE
		SET enabled = EXCLUDED.enabled, config = EXCLUDED.config, updated_at = NOW()
		RETURNING id, environment_id, connector_type, name, enabled, config, created_at, updated_at
	`, environmentID, connectorType, name, enabled, raw).Scan(
		&c.ID, &c.EnvironmentID, &c.ConnectorType, &c.Name, &c.Enabled, &cfgOut, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return c, err
	}
	c.Config = map[string]any{}
	if len(cfgOut) > 0 {
		_ = json.Unmarshal(cfgOut, &c.Config)
	}
	c.Ready = ConnectorReady(c)
	return c, nil
}

// PlatformConnectorDefaults builds non-secret connector configs from runtime platform settings.
// Secrets (tokens, keys) are never included — only public URLs, repo names, and infra identifiers.
func PlatformConnectorDefaults(
	repo, branch, healthURL, copilotURL, spottingURL, deployRegion, deployTarget, secretsManagerID, rollbackWorkflow string,
	sentryConfigured bool,
) map[string]map[string]any {
	if repo == "" {
		repo = "kkjayakumar/AegIsOps"
	}
	if branch == "" {
		branch = "main"
	}
	if healthURL == "" {
		healthURL = "/healthz"
	}
	if copilotURL == "" {
		copilotURL = "http://localhost:5000"
	}
	if spottingURL == "" {
		spottingURL = "http://localhost:3000"
	}
	if deployRegion == "" {
		deployRegion = "ap-south-1"
	}
	if secretsManagerID == "" {
		secretsManagerID = "dev/syntropix"
	}
	workflows := []any{"ci.yml", "deploy-ec2.yml"}
	rbWorkflow := "rollback.yml"
	if rollbackWorkflow != "" {
		rbWorkflow = rollbackWorkflow
	}
	workflows = append(workflows, rbWorkflow)
	deployCfg := map[string]any{
		"provider": "ec2_ssm", "region": deployRegion,
		"verifyUrl": healthURL, "secretsManagerId": secretsManagerID,
	}
	// Only include target when known — empty target keeps deployment Not Ready (honest).
	if deployTarget != "" {
		deployCfg["target"] = deployTarget
	}
	return map[string]map[string]any{
		ConnectorGit: {
			"provider": "github", "repository": repo, "defaultBranch": branch,
		},
		ConnectorPullRequest: {
			"autoOpen": true, "requiredApprovals": 1, "baseBranch": branch,
		},
		ConnectorCICD: {
			"provider": "github_actions", "workflows": workflows, "rollbackWorkflow": rbWorkflow,
		},
		ConnectorDeployment: deployCfg,
		ConnectorTelemetry: {
			"provider": "syntropix", "healthUrl": healthURL,
			"copilotApiUrl": copilotURL, "spottingApiUrl": spottingURL,
			"sentryConfigured": sentryConfigured,
		},
		ConnectorMCP: {
			"ide": "cursor", "transport": "stdio", "tools": "aegisops",
		},
		ConnectorIssueAgent: {
			"route": "api", "provider": "openai", "useGraph": true, "allowAnyAgent": true,
		},
	}
}

// MergeConnectorConfig overlays defaults onto existing without wiping user keys.
func MergeConnectorConfig(existing, defaults map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range defaults {
		out[k] = v
	}
	for k, v := range existing {
		if v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) == "" {
				continue
			}
		case []any:
			if len(t) == 0 {
				continue
			}
		}
		out[k] = v
	}
	return out
}

// GetConnectorByType returns the canonical closed-loop stage connector for an environment.
func (s *WorkspaceStore) GetConnectorByType(ctx context.Context, environmentID int, connectorType string) (Connector, error) {
	list, err := s.ListConnectors(ctx, environmentID)
	if err != nil {
		return Connector{}, err
	}
	if c, ok := CanonicalStageConnector(list, connectorType); ok {
		return c, nil
	}
	return Connector{}, fmt.Errorf("connector %s not found", connectorType)
}

// FindEnvironmentByGitRepo finds an environment whose git connector repository matches repo.
func (s *WorkspaceStore) FindEnvironmentByGitRepo(ctx context.Context, repo string) (int, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("database not available")
	}
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return 0, fmt.Errorf("empty repo")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT environment_id, config FROM aegis_connectors
		WHERE connector_type = $1 AND enabled = true
		ORDER BY environment_id
	`, ConnectorGit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var envID int
		var raw []byte
		if err := rows.Scan(&envID, &raw); err != nil {
			return 0, err
		}
		cfg := map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &cfg)
		}
		if strings.EqualFold(configString(cfg, "repository"), repo) {
			return envID, nil
		}
	}
	return 0, fmt.Errorf("no enabled git connector for %s", repo)
}

// DefaultEnvironmentID returns the first environment id (stable fallback for single-tenant).
func (s *WorkspaceStore) DefaultEnvironmentID(ctx context.Context) (int, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("database not available")
	}
	var id int
	err := s.pool.QueryRow(ctx, `
		SELECT id FROM aegis_environments ORDER BY sort_order, id LIMIT 1
	`).Scan(&id)
	return id, err
}

// ApplyPlatformDefaults writes non-secret defaults into connector configs.
// When enable is true, sets enabled only if ConnectorReady would pass after the merge.
func (s *WorkspaceStore) ApplyPlatformDefaults(
	ctx context.Context,
	environmentID int,
	defaults map[string]map[string]any,
	types []string,
	enable bool,
) ([]Connector, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	if err := s.EnsureEnvironmentConnectors(ctx, environmentID, nil); err != nil {
		return nil, err
	}
	existing, err := s.ListConnectors(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	if len(types) == 0 {
		for _, t := range ConnectorTypes {
			want[t] = true
		}
	} else {
		for _, t := range types {
			want[strings.TrimSpace(t)] = true
		}
	}
	updatedByID := map[int]Connector{}
	for _, ctype := range ConnectorTypes {
		if !want[ctype] {
			continue
		}
		c, ok := CanonicalStageConnector(existing, ctype)
		if !ok {
			continue
		}
		def := defaults[ctype]
		if def == nil {
			continue
		}
		merged := MergeConnectorConfig(c.Config, def)
		nextEnabled := c.Enabled
		if enable {
			probe := Connector{ConnectorType: ctype, Enabled: true, Config: merged}
			nextEnabled = ConnectorReady(probe)
		}
		name := c.Name
		if name == "" {
			name = ConnectorDisplayNames[ctype]
		}
		updated, err := s.UpsertConnector(ctx, environmentID, ctype, name, nextEnabled, merged)
		if err != nil {
			return nil, err
		}
		updatedByID[updated.ID] = updated
	}
	out := make([]Connector, 0, len(existing))
	for _, c := range existing {
		if u, ok := updatedByID[c.ID]; ok {
			out = append(out, u)
		} else {
			out = append(out, c)
		}
	}
	return out, nil
}

// EnsureEnvironmentConnectors creates missing stage rows as disabled templates with empty config.
// It does not auto-enable or fill platform defaults — Connected requires explicit user enable + config.
func (s *WorkspaceStore) EnsureEnvironmentConnectors(ctx context.Context, environmentID int, _ map[string]map[string]any) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	existing, err := s.ListConnectors(ctx, environmentID)
	if err != nil {
		return err
	}
	byType := map[string][]Connector{}
	for _, c := range existing {
		byType[c.ConnectorType] = append(byType[c.ConnectorType], c)
	}
	for _, ctype := range ConnectorTypes {
		if len(byType[ctype]) > 0 {
			continue
		}
		name := ConnectorDisplayNames[ctype]
		if name == "" {
			name = ctype
		}
		if _, err := s.UpsertConnector(ctx, environmentID, ctype, name, false, map[string]any{}); err != nil {
			return err
		}
	}
	return nil
}

// EnsureAllConnectors applies platform defaults across every environment.
func (s *WorkspaceStore) EnsureAllConnectors(ctx context.Context, defaults map[string]map[string]any) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	rows, err := s.pool.Query(ctx, `SELECT id FROM aegis_environments ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.EnsureEnvironmentConnectors(ctx, id, defaults); err != nil {
			return fmt.Errorf("environment %d: %w", id, err)
		}
	}
	return nil
}

func (s *WorkspaceStore) ProjectIDForEnvironment(ctx context.Context, environmentID int) (int, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("database not available")
	}
	var projectID int
	err := s.pool.QueryRow(ctx, `SELECT project_id FROM aegis_environments WHERE id = $1`, environmentID).Scan(&projectID)
	return projectID, err
}

// ErrEnvironmentProjectMismatch is returned when an environment is not under the claimed project.
// Used for strict tenant→project→env isolation (finance/healthcare compliance).
var ErrEnvironmentProjectMismatch = fmt.Errorf("environment does not belong to the specified project")

// AssertEnvironmentBelongsToProject verifies env → project. Never fall back to another env.
func (s *WorkspaceStore) AssertEnvironmentBelongsToProject(ctx context.Context, environmentID, projectID int) error {
	if projectID <= 0 {
		return fmt.Errorf("project id is required")
	}
	if environmentID <= 0 {
		return fmt.Errorf("environment id is required")
	}
	got, err := s.ProjectIDForEnvironment(ctx, environmentID)
	if err != nil {
		return fmt.Errorf("environment not found")
	}
	if got != projectID {
		return ErrEnvironmentProjectMismatch
	}
	return nil
}

// TenantIDForProject returns the owning tenant for a project (defense-in-depth chain checks).
func (s *WorkspaceStore) TenantIDForProject(ctx context.Context, projectID int) (int, error) {
	if s == nil || s.pool == nil {
		return 0, fmt.Errorf("database not available")
	}
	var tenantID int
	err := s.pool.QueryRow(ctx, `SELECT tenant_id FROM aegis_projects WHERE id = $1`, projectID).Scan(&tenantID)
	return tenantID, err
}

// AssertEnvironmentChain verifies environment → project → tenant when tenantID > 0.
func (s *WorkspaceStore) AssertEnvironmentChain(ctx context.Context, environmentID, projectID, tenantID int) error {
	if err := s.AssertEnvironmentBelongsToProject(ctx, environmentID, projectID); err != nil {
		return err
	}
	if tenantID <= 0 {
		return nil
	}
	got, err := s.TenantIDForProject(ctx, projectID)
	if err != nil {
		return fmt.Errorf("project not found")
	}
	if got != tenantID {
		return fmt.Errorf("project does not belong to the specified tenant")
	}
	return nil
}

func (s *WorkspaceStore) ListSecrets(ctx context.Context, environmentID int, maskFn func(string) string) ([]Secret, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, environment_id, provider, key_name, value_ciphertext, created_at, updated_at
		FROM aegis_secrets WHERE environment_id = $1 ORDER BY provider, key_name
	`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Secret{}
	for rows.Next() {
		var sec Secret
		var cipher string
		if err := rows.Scan(&sec.ID, &sec.EnvironmentID, &sec.Provider, &sec.KeyName, &cipher, &sec.CreatedAt, &sec.UpdatedAt); err != nil {
			return nil, err
		}
		// SOC2: list never decrypts — fixed mask only (ciphertext stays unused).
		_ = cipher
		sec.MaskedValue = "••••••••"
		if maskFn != nil {
			// Optional hint mask without exposing prefix; callers should pass nil or fixed mask.
			sec.MaskedValue = maskFn("")
			if sec.MaskedValue == "" {
				sec.MaskedValue = "••••••••"
			}
		}
		out = append(out, sec)
	}
	return out, rows.Err()
}

func (s *WorkspaceStore) UpsertSecret(ctx context.Context, environmentID int, provider, keyName, ciphertext string) (Secret, error) {
	if s == nil || s.pool == nil {
		return Secret{}, fmt.Errorf("database not available")
	}
	if provider == "" {
		provider = "custom"
	}
	var sec Secret
	err := s.pool.QueryRow(ctx, `
		INSERT INTO aegis_secrets (environment_id, provider, key_name, value_ciphertext)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (environment_id, key_name) DO UPDATE
		SET provider = EXCLUDED.provider, value_ciphertext = EXCLUDED.value_ciphertext, updated_at = NOW()
		RETURNING id, environment_id, provider, key_name, created_at, updated_at
	`, environmentID, provider, keyName, ciphertext).Scan(
		&sec.ID, &sec.EnvironmentID, &sec.Provider, &sec.KeyName, &sec.CreatedAt, &sec.UpdatedAt,
	)
	sec.MaskedValue = "••••••••"
	return sec, err
}

func (s *WorkspaceStore) DeleteSecret(ctx context.Context, environmentID, secretID int) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("database not available")
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM aegis_secrets WHERE id = $1 AND environment_id = $2`, secretID, environmentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("secret not found")
	}
	return nil
}
