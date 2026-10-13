package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kkjayakumar/AegIsOps/internal/store"
	"github.com/kkjayakumar/AegIsOps/internal/workspace"
)

type Config struct {
	Port                   string
	DatabaseURL            string
	TeamsWebhookURL        string
	SlackWebhookURL        string
	AegisWebhookSecret     string
	CopilotAPIURL          string
	CopilotServiceToken    string
	SpottingAPIURL         string
	SpottingServiceToken   string
	GitHubAppID            string
	GitHubInstallationID   string
	GitHubWebhookSecret    string
	GitHubPrivateKeyPath   string
	GitHubToken            string
	GitHubDefaultRepo      string
	GitHubDefaultBranch    string
	PublicURL              string
	DeployVerifyURL        string
	DeployVerifyTTLMin     int
	RollbackWebhookURL     string
	RollbackWorkflow       string
	WebhookRateLimit       int
	Environment            string
	SentryDSN              string
	AllowDogfood           bool
	WorkspaceSecretsKey    string
	AWSRegion              string
	EC2InstanceID          string
	SecretsManagerID       string
	RequireGatewayApproval bool
	APIToken               string
	AllowInsecureAuth      bool
	AuthUserID             int
}

func Load() (*Config, error) {
	port := envOr("PORT", "5001")
	if _, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("invalid PORT %q: %w", port, err)
	}
	rateLimit := 120
	if v := os.Getenv("WEBHOOK_RATE_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid WEBHOOK_RATE_LIMIT %q", v)
		}
		rateLimit = n
	}
	verifyTTL := 30
	if v := os.Getenv("DEPLOY_VERIFY_TTL_MIN"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			verifyTTL = n
		}
	}
	return &Config{
		Port:                 port,
		DatabaseURL:          envOr("DATABASE_URL", "postgres://aegisops:aegisops@localhost:5432/aegisops?sslmode=disable"),
		TeamsWebhookURL:      os.Getenv("TEAMS_WEBHOOK_URL"),
		SlackWebhookURL:      os.Getenv("SLACK_WEBHOOK_URL"),
		AegisWebhookSecret:   envOr("AEGIS_WEBHOOK_SECRET", "dev-webhook-secret"),
		CopilotAPIURL:        envOr("COPILOT_API_URL", "http://localhost:5000"),
		CopilotServiceToken:  os.Getenv("COPILOT_SERVICE_TOKEN"),
		SpottingAPIURL:       envOr("SPOTTING_API_URL", "http://localhost:3000"),
		SpottingServiceToken: os.Getenv("SPOTTING_SERVICE_TOKEN"),
		GitHubAppID:          os.Getenv("GITHUB_APP_ID"),
		GitHubInstallationID: os.Getenv("GITHUB_APP_INSTALLATION_ID"),
		GitHubWebhookSecret:  os.Getenv("GITHUB_WEBHOOK_SECRET"),
		GitHubPrivateKeyPath: os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH"),
		GitHubToken:          os.Getenv("GITHUB_TOKEN"),
		GitHubDefaultRepo:    envOr("GITHUB_DEFAULT_REPO", ""),
		GitHubDefaultBranch:  envOr("GITHUB_DEFAULT_BRANCH", "main"),
		PublicURL:            envOr("AEGISOPS_PUBLIC_URL", "http://localhost:5001"),
		DeployVerifyURL:      os.Getenv("DEPLOY_VERIFY_URL"),
		DeployVerifyTTLMin:   verifyTTL,
		RollbackWebhookURL:   os.Getenv("ROLLBACK_WEBHOOK_URL"),
		RollbackWorkflow:     os.Getenv("ROLLBACK_WORKFLOW"),
		WebhookRateLimit:     rateLimit,
		Environment:          envOr("AEGISOPS_ENV", "development"),
		SentryDSN: NormalizeSentryDSN(
			os.Getenv("SENTRY_DSN"),
			os.Getenv("COPILOT_INGEST_PUBLIC_URL"),
			envOr("COPILOT_API_URL", "http://localhost:5000"),
		),
		AllowDogfood:           envTruthy("AEGISOPS_ALLOW_DOGFOOD"),
		WorkspaceSecretsKey:    os.Getenv("WORKSPACE_SECRETS_KEY"),
		AWSRegion:              envOr("AWS_REGION", "ap-south-1"),
		EC2InstanceID:          os.Getenv("EC2_INSTANCE_ID"),
		SecretsManagerID:       envOr("SECRETS_MANAGER_ID", "dev/syntropix"),
		RequireGatewayApproval: envTruthy("AEGISOPS_REQUIRE_GATEWAY"),
		APIToken:               strings.TrimSpace(os.Getenv("AEGISOPS_API_TOKEN")),
		AllowInsecureAuth:      envTruthy("AEGISOPS_ALLOW_INSECURE_AUTH"),
		AuthUserID:             envIntOr("AEGISOPS_AUTH_USER_ID", 1),
	}, nil
}

// ConnectorDefaults returns non-secret platform defaults for workspace connectors.
// Never includes tokens, webhook secrets, or API keys.
func (c *Config) ConnectorDefaults() map[string]map[string]any {
	if c == nil {
		return map[string]map[string]any{}
	}
	health := c.DeployVerifyURL
	if health == "" {
		health = strings.TrimSuffix(c.PublicURL, "/") + "/healthz"
	}
	return store.PlatformConnectorDefaults(
		c.GitHubDefaultRepo,
		c.GitHubDefaultBranch,
		health,
		c.CopilotAPIURL,
		c.SpottingAPIURL,
		c.AWSRegion,
		c.EC2InstanceID,
		c.SecretsManagerID,
		c.RollbackWorkflow,
		c.SentryDSN != "",
	)
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if !c.IsProduction() {
		return nil
	}
	var problems []string
	if c.AegisWebhookSecret == "" || c.AegisWebhookSecret == "dev-webhook-secret" {
		problems = append(problems, "AEGIS_WEBHOOK_SECRET must be set to a non-default value in production")
	}
	if c.GitHubWebhookSecret == "" {
		problems = append(problems, "GITHUB_WEBHOOK_SECRET is required in production")
	}
	if c.GitHubWebhookSecret == "CHANGE_ME" {
		problems = append(problems, "GITHUB_WEBHOOK_SECRET must not be CHANGE_ME in production")
	}
	if c.AegisWebhookSecret == "CHANGE_ME" {
		problems = append(problems, "AEGIS_WEBHOOK_SECRET must not be CHANGE_ME in production")
	}
	if c.IsProduction() && c.githubAuthMissing() {
		problems = append(problems, "configure GITHUB_APP_ID + GITHUB_APP_INSTALLATION_ID + GITHUB_APP_PRIVATE_KEY_PATH or GITHUB_TOKEN for PR automation")
	}
	if c.IsProduction() {
		if workspace.IsInsecureDevKey(c.WorkspaceSecretsKey) {
			problems = append(problems, "WORKSPACE_SECRETS_KEY must be set to a strong non-default value in production (openssl rand -hex 32)")
		}
		if !c.AllowInsecureAuth {
			if c.APIToken == "" {
				problems = append(problems, "AEGISOPS_API_TOKEN is required in production (or set AEGISOPS_ALLOW_INSECURE_AUTH=true for emergency only)")
			} else if len(c.APIToken) < 16 {
				problems = append(problems, "AEGISOPS_API_TOKEN must be at least 16 characters")
			}
		}
	}
	if c.usesGitHubApp() {
		if _, err := os.Stat(c.GitHubPrivateKeyPath); err != nil {
			problems = append(problems, fmt.Sprintf("GITHUB_APP_PRIVATE_KEY_PATH not readable: %s", c.GitHubPrivateKeyPath))
		}
		if _, err := strconv.ParseInt(c.GitHubAppID, 10, 64); err != nil {
			problems = append(problems, "GITHUB_APP_ID must be numeric")
		}
		if _, err := strconv.ParseInt(c.GitHubInstallationID, 10, 64); err != nil {
			problems = append(problems, "GITHUB_APP_INSTALLATION_ID must be the numeric id from github.com/settings/installations/<id> (not the OAuth Client ID starting with Iv)")
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.Environment, "production") || strings.EqualFold(c.Environment, "prod")
}

// AuthEnforced is true when API requests must present AEGISOPS_API_TOKEN.
// Disabled for local/dev without a token, or when AEGISOPS_ALLOW_INSECURE_AUTH=true.
func (c *Config) AuthEnforced() bool {
	if c == nil || c.AllowInsecureAuth {
		return false
	}
	if c.APIToken != "" {
		return true
	}
	return c.IsProduction()
}

// AuthStatus is safe to expose on /api/v1/version (never includes the token).
func (c *Config) AuthStatus() map[string]any {
	if c == nil {
		return map[string]any{"enforced": false, "mode": "none"}
	}
	if c.AllowInsecureAuth {
		return map[string]any{"enforced": false, "mode": "insecure_allowed"}
	}
	if c.AuthEnforced() {
		return map[string]any{"enforced": true, "mode": "api_token", "tokenConfigured": c.APIToken != ""}
	}
	return map[string]any{"enforced": false, "mode": "development"}
}

func (c *Config) githubAuthMissing() bool {
	if c == nil {
		return true
	}
	if c.GitHubToken != "" {
		return false
	}
	return c.GitHubAppID == "" || c.GitHubInstallationID == "" || c.GitHubPrivateKeyPath == ""
}

func (c *Config) usesGitHubApp() bool {
	return c != nil && c.GitHubToken == "" && !c.githubAuthMissing()
}

// GitHubAuthStatus reports how GitHub API auth is configured (for /api/v1/version).
func (c *Config) GitHubAuthStatus() map[string]any {
	if c == nil {
		return map[string]any{"enabled": false, "mode": "none"}
	}
	if c.GitHubToken != "" {
		return map[string]any{"enabled": true, "mode": "pat"}
	}
	if c.githubAuthMissing() {
		return map[string]any{"enabled": false, "mode": "none"}
	}
	keyOK := githubKeyReadable(c.GitHubPrivateKeyPath)
	return map[string]any{
		"enabled":     keyOK,
		"mode":        "app",
		"keyPath":     c.GitHubPrivateKeyPath,
		"keyReadable": keyOK,
		"appId":       c.GitHubAppID,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func githubKeyReadable(path string) bool {
	if path == "" {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func envTruthy(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
