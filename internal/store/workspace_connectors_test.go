package store

import (
	"strings"
	"testing"
)

func TestPlatformConnectorDefaultsNoSecrets(t *testing.T) {
	defs := PlatformConnectorDefaults(
		"acme/app", "develop", "https://app.example/healthz",
		"http://copilot:5000", "http://spotting:3000",
		"ap-south-1", "i-abc", "prod/secrets", "rollback.yml", true,
	)
	git := defs[ConnectorGit]
	if git["repository"] != "acme/app" {
		t.Fatalf("repo=%v", git["repository"])
	}
	dep := defs[ConnectorDeployment]
	if dep["target"] != "i-abc" {
		t.Fatalf("target=%v", dep["target"])
	}
	for ctype, cfg := range defs {
		for k := range cfg {
			lk := strings.ToLower(k)
			if k == "secretsManagerId" {
				continue
			}
			if strings.Contains(lk, "token") || strings.Contains(lk, "password") || strings.Contains(lk, "apikey") {
				t.Fatalf("unexpected secret-like key %s.%s", ctype, k)
			}
			if strings.Contains(lk, "secret") {
				t.Fatalf("unexpected secret-like key %s.%s", ctype, k)
			}
		}
	}
}

func TestPlatformConnectorDefaultsOmitsEmptyTarget(t *testing.T) {
	defs := PlatformConnectorDefaults("", "", "", "", "", "", "", "", "", false)
	dep := defs[ConnectorDeployment]
	if _, ok := dep["target"]; ok {
		t.Fatal("empty target should be omitted so deployment stays Not Ready")
	}
	probe := Connector{ConnectorType: ConnectorDeployment, Enabled: true, Config: dep}
	if ConnectorReady(probe) {
		t.Fatal("deployment without target must not be ready")
	}
}

func TestMergeConnectorConfigKeepsUserValues(t *testing.T) {
	merged := MergeConnectorConfig(
		map[string]any{"repository": "user/repo", "defaultBranch": ""},
		map[string]any{"repository": "platform/repo", "defaultBranch": "main", "provider": "github"},
	)
	if merged["repository"] != "user/repo" {
		t.Fatalf("user repo overwritten: %v", merged["repository"])
	}
	if merged["defaultBranch"] != "main" {
		t.Fatalf("empty user branch should take default: %v", merged["defaultBranch"])
	}
	if merged["provider"] != "github" {
		t.Fatalf("missing provider: %v", merged["provider"])
	}
}

func TestConnectorReadyGit(t *testing.T) {
	c := Connector{ConnectorType: ConnectorGit, Enabled: true, Config: map[string]any{"repository": "a/b"}}
	if !ConnectorReady(c) {
		t.Fatal("expected ready")
	}
	c.Enabled = false
	if ConnectorReady(c) {
		t.Fatal("disabled must not be ready")
	}
}

func TestCanonicalStageConnectorPrefersDisplayName(t *testing.T) {
	list := []Connector{
		{ID: 1, ConnectorType: ConnectorMCP, Name: "cursor", Enabled: true, Config: map[string]any{"ide": "cursor"}},
		{ID: 2, ConnectorType: ConnectorMCP, Name: "IDE / MCP", Enabled: false, Config: map[string]any{}},
	}
	c, ok := CanonicalStageConnector(list, ConnectorMCP)
	if !ok || c.ID != 2 {
		t.Fatalf("want canonical IDE / MCP row, got %+v ok=%v", c, ok)
	}
	list[1].Config = map[string]any{"ide": "cursor"}
	list[1].Enabled = true
	if !ConnectorReady(list[1]) {
		t.Fatal("canonical row should be ready after config")
	}
}

func TestStageConnectorsOnePerType(t *testing.T) {
	list := []Connector{
		{ConnectorType: ConnectorGit, Name: "Source", Enabled: true, Config: map[string]any{"repository": "a/b"}},
		{ConnectorType: ConnectorMCP, Name: "cursor", Enabled: true, Config: map[string]any{"ide": "cursor"}},
		{ConnectorType: ConnectorMCP, Name: "IDE / MCP", Enabled: true, Config: map[string]any{"ide": "cursor"}},
	}
	stages := StageConnectors(list)
	if len(stages) != 2 {
		t.Fatalf("stages=%d", len(stages))
	}
	if stages[ConnectorMCP].Name != "IDE / MCP" {
		t.Fatalf("mcp stage=%q", stages[ConnectorMCP].Name)
	}
}
