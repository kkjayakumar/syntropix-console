package config

import (
	"os"
	"testing"
)

func TestValidateDevelopmentAllowsDefaults(t *testing.T) {
	cfg := &Config{
		Environment:        "development",
		AegisWebhookSecret: "dev-webhook-secret",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeSentryDSNStagingHost(t *testing.T) {
	raw := "http://abc123@127.0.0.1:5000/5d730f57"
	got := NormalizeSentryDSN(raw, "https://monitoring.syntropix.io", "http://host.docker.internal:5000")
	want := "http://abc123@host.docker.internal:5000/5d730f57"
	if got != want {
		t.Fatalf("NormalizeSentryDSN() = %q, want %q", got, want)
	}
}

func TestNormalizeSentryDSNPublicHostToDockerInternal(t *testing.T) {
	raw := "http://abc123@monitoring.syntropix.io/5d730f57"
	got := NormalizeSentryDSN(raw, "https://monitoring.syntropix.io", "http://host.docker.internal:5000")
	want := "http://abc123@host.docker.internal:5000/5d730f57"
	if got != want {
		t.Fatalf("NormalizeSentryDSN() = %q, want %q", got, want)
	}
}

func TestNormalizeSentryDSDockerFallback(t *testing.T) {
	raw := "http://abc123@localhost:5000/5d730f57"
	got := NormalizeSentryDSN(raw, "", "http://host.docker.internal:5000")
	want := "http://abc123@host.docker.internal:5000/5d730f57"
	if got != want {
		t.Fatalf("NormalizeSentryDSN() = %q, want %q", got, want)
	}
}

func TestValidateProductionRequiresGitHubAuth(t *testing.T) {
	cfg := &Config{
		Environment:         "production",
		AegisWebhookSecret:  "real-secret",
		GitHubWebhookSecret: "gh-secret",
		WorkspaceSecretsKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		APIToken:            "test-api-token-16",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production validation error without github auth")
	}
	cfg.GitHubToken = "ghp_test"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error with PAT: %v", err)
	}
}

func TestValidateProductionRejectsChangeMe(t *testing.T) {
	cfg := &Config{
		Environment:         "production",
		AegisWebhookSecret:  "CHANGE_ME",
		GitHubWebhookSecret: "gh-secret",
		GitHubToken:         "ghp_test",
		WorkspaceSecretsKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		APIToken:            "test-api-token-16",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected CHANGE_ME rejection")
	}
}

func TestValidateProductionRejectsDefaults(t *testing.T) {
	cfg := &Config{
		Environment:        "production",
		AegisWebhookSecret: "dev-webhook-secret",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production validation error")
	}
}

func TestValidateProductionRequiresWorkspaceSecretsKey(t *testing.T) {
	cfg := &Config{
		Environment:         "production",
		AegisWebhookSecret:  "real-secret",
		GitHubWebhookSecret: "gh-secret",
		GitHubToken:         "ghp_test",
		APIToken:            "test-api-token-16",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing WORKSPACE_SECRETS_KEY error")
	}
}

func TestValidateProductionRequiresAPIToken(t *testing.T) {
	cfg := &Config{
		Environment:         "production",
		AegisWebhookSecret:  "real-secret",
		GitHubWebhookSecret: "gh-secret",
		GitHubToken:         "ghp_test",
		WorkspaceSecretsKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected missing AEGISOPS_API_TOKEN error")
	}
	cfg.AllowInsecureAuth = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("insecure allow should skip token requirement: %v", err)
	}
	cfg.AllowInsecureAuth = false
	cfg.APIToken = "short"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected short token rejection")
	}
	cfg.APIToken = "long-enough-token"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestAuthEnforcedModes(t *testing.T) {
	dev := &Config{Environment: "development"}
	if dev.AuthEnforced() {
		t.Fatal("dev without token should not enforce")
	}
	dev.APIToken = "local-dev-token!!"
	if !dev.AuthEnforced() {
		t.Fatal("dev with token should enforce")
	}
	prod := &Config{Environment: "production", APIToken: "prod-token-value"}
	if !prod.AuthEnforced() {
		t.Fatal("prod with token should enforce")
	}
	prod.AllowInsecureAuth = true
	if prod.AuthEnforced() {
		t.Fatal("insecure allow should disable enforcement")
	}
}

func TestValidateProductionRequiresReadableAppKey(t *testing.T) {
	cfg := &Config{
		Environment:          "production",
		AegisWebhookSecret:   "real-secret",
		GitHubWebhookSecret:  "gh-secret",
		GitHubAppID:          "123",
		GitHubInstallationID: "456",
		GitHubPrivateKeyPath: "/no/such/github-app.pem",
		WorkspaceSecretsKey:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		APIToken:             "test-api-token-16",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error when app key path missing")
	}
}

func TestValidateProductionRejectsOAuthClientIDAsInstallationID(t *testing.T) {
	f := t.TempDir() + "/key.pem"
	if err := os.WriteFile(f, []byte("pem"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		Environment:          "production",
		AegisWebhookSecret:   "real-secret",
		GitHubWebhookSecret:  "gh-secret",
		GitHubAppID:          "4070224",
		GitHubInstallationID: "Iv23li7gWYid0ZmHH6WC",
		GitHubPrivateKeyPath: f,
		WorkspaceSecretsKey:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		APIToken:             "test-api-token-16",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error when installation id is OAuth client id")
	}
}

func TestGitHubAuthStatusAppKeyReadable(t *testing.T) {
	f := t.TempDir() + "/key.pem"
	if err := os.WriteFile(f, []byte("-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBALRiMLAHudeSA\n-----END RSA PRIVATE KEY-----\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		GitHubAppID:          "1",
		GitHubInstallationID: "2",
		GitHubPrivateKeyPath: f,
	}
	st := cfg.GitHubAuthStatus()
	if st["mode"] != "app" || st["keyReadable"] != true {
		t.Fatalf("status = %#v", st)
	}
}
