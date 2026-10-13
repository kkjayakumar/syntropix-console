package orchestrator

import (
	"testing"

	"github.com/kkjayakumar/AegIsOps/internal/store"
)

func TestTelemetryAPIURLsFromConnectorPrefersReadyURLs(t *testing.T) {
	tel := store.Connector{
		ConnectorType: store.ConnectorTelemetry,
		Enabled:       true,
		Config: map[string]any{
			"healthUrl":      "https://app.example/healthz",
			"copilotApiUrl":  "http://copilot.workspace:5000",
			"spottingApiUrl": "http://spotting.workspace:3000",
		},
	}
	copilot, spotting, used := telemetryAPIURLsFromConnector(tel, "http://global-copilot", "http://global-spotting")
	if !used {
		t.Fatal("expected connector URLs to be used")
	}
	if copilot != "http://copilot.workspace:5000" {
		t.Fatalf("copilot=%q", copilot)
	}
	if spotting != "http://spotting.workspace:3000" {
		t.Fatalf("spotting=%q", spotting)
	}
}

func TestTelemetryAPIURLsFromConnectorFallsBackWhenNotReady(t *testing.T) {
	tel := store.Connector{
		ConnectorType: store.ConnectorTelemetry,
		Enabled:       true,
		Config: map[string]any{
			// missing healthUrl → ConnectorReady false
			"copilotApiUrl":  "http://copilot.workspace:5000",
			"spottingApiUrl": "http://spotting.workspace:3000",
		},
	}
	copilot, spotting, used := telemetryAPIURLsFromConnector(tel, "http://global-copilot", "http://global-spotting")
	if used {
		t.Fatal("not-ready connector must not override globals")
	}
	if copilot != "http://global-copilot" || spotting != "http://global-spotting" {
		t.Fatalf("copilot=%q spotting=%q", copilot, spotting)
	}
}

func TestTelemetryAPIURLsFromConnectorPartialOverride(t *testing.T) {
	tel := store.Connector{
		ConnectorType: store.ConnectorTelemetry,
		Enabled:       true,
		Config: map[string]any{
			"healthUrl":     "/healthz",
			"copilotApiUrl": "http://only-copilot:5000",
		},
	}
	copilot, spotting, used := telemetryAPIURLsFromConnector(tel, "http://global-copilot", "http://global-spotting")
	if !used {
		t.Fatal("expected partial connector override")
	}
	if copilot != "http://only-copilot:5000" {
		t.Fatalf("copilot=%q", copilot)
	}
	if spotting != "http://global-spotting" {
		t.Fatalf("spotting fallback lost: %q", spotting)
	}
}

func TestTelemetryAPIURLsFromConnectorDisabled(t *testing.T) {
	tel := store.Connector{
		ConnectorType: store.ConnectorTelemetry,
		Enabled:       false,
		Config: map[string]any{
			"healthUrl":      "/healthz",
			"copilotApiUrl":  "http://copilot.workspace:5000",
			"spottingApiUrl": "http://spotting.workspace:3000",
		},
	}
	_, _, used := telemetryAPIURLsFromConnector(tel, "http://g1", "http://g2")
	if used {
		t.Fatal("disabled connector must not override")
	}
}

func TestRollbackWorkflowFromConnectorExplicit(t *testing.T) {
	cicd := store.Connector{
		ConnectorType: store.ConnectorCICD,
		Enabled:       true,
		Config: map[string]any{
			"provider":         "github_actions",
			"rollbackWorkflow": "custom-rollback.yml",
			"workflows":        []any{"ci.yml", "deploy-ec2.yml"},
		},
	}
	wf, used := rollbackWorkflowFromConnector(cicd, "rollback.yml")
	if !used || wf != "custom-rollback.yml" {
		t.Fatalf("wf=%q used=%v", wf, used)
	}
}

func TestRollbackWorkflowFromConnectorWorkflowsList(t *testing.T) {
	cicd := store.Connector{
		ConnectorType: store.ConnectorCICD,
		Enabled:       true,
		Config: map[string]any{
			"provider":  "github_actions",
			"workflows": []any{"ci.yml", "deploy-ec2.yml", "rollback.yml"},
		},
	}
	wf, used := rollbackWorkflowFromConnector(cicd, "global-rollback.yml")
	if !used || wf != "rollback.yml" {
		t.Fatalf("wf=%q used=%v", wf, used)
	}
}

func TestRollbackWorkflowFromConnectorFallback(t *testing.T) {
	cicd := store.Connector{
		ConnectorType: store.ConnectorCICD,
		Enabled:       true,
		Config: map[string]any{
			"provider":  "github_actions",
			"workflows": []any{"ci.yml", "deploy-ec2.yml"},
		},
	}
	wf, used := rollbackWorkflowFromConnector(cicd, "rollback.yml")
	if used || wf != "rollback.yml" {
		t.Fatalf("wf=%q used=%v want fallback only", wf, used)
	}
}
