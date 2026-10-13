package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kkjayakumar/AegIsOps/internal/clients"
	"github.com/kkjayakumar/AegIsOps/internal/store"
)

// RuntimeSettings are resolved per issue/repo: workspace connectors first, then global config.
type RuntimeSettings struct {
	Repo             string
	DefaultBranch    string
	VerifyURL        string
	CopilotAPIURL    string
	SpottingAPIURL   string
	RollbackWorkflow string
	EnvironmentID    int
	Source           string // "connector" | "config"
	GitReady         bool
	TelemetryReady   bool
	AgentRoute       string
	AgentProvider    string
	AgentReady       bool
}

func (s *Service) ResolveRuntime(ctx context.Context, preferredRepo string) RuntimeSettings {
	return s.resolveRuntime(ctx, preferredRepo)
}

func (s *Service) resolveRuntime(ctx context.Context, preferredRepo string) RuntimeSettings {
	out := RuntimeSettings{
		Repo:          preferredRepo,
		DefaultBranch: s.defaultBranch,
		Source:        "config",
	}
	if out.Repo == "" {
		out.Repo = s.defaultRepo
	}
	if s.cfg != nil {
		out.VerifyURL = s.cfg.DeployVerifyURL
		out.CopilotAPIURL = s.cfg.CopilotAPIURL
		out.SpottingAPIURL = s.cfg.SpottingAPIURL
		out.RollbackWorkflow = s.cfg.RollbackWorkflow
		if out.DefaultBranch == "" {
			out.DefaultBranch = s.cfg.GitHubDefaultBranch
		}
	}
	if s.workspace == nil {
		return out
	}

	envID, err := s.resolveEnvironmentID(ctx, out.Repo)
	if err != nil || envID <= 0 {
		return out
	}
	out.EnvironmentID = envID
	usedConnector := false

	if git, gerr := s.workspace.GetConnectorByType(ctx, envID, store.ConnectorGit); gerr == nil && store.ConnectorReady(git) {
		if repo := connectorConfigString(git.Config, "repository"); repo != "" {
			out.Repo = repo
			out.GitReady = true
			usedConnector = true
		}
		if branch := connectorConfigString(git.Config, "defaultBranch"); branch != "" {
			out.DefaultBranch = branch
		}
	}

	verifyFromConnector := ""
	if dep, derr := s.workspace.GetConnectorByType(ctx, envID, store.ConnectorDeployment); derr == nil && dep.Enabled {
		if u := connectorConfigString(dep.Config, "verifyUrl"); u != "" {
			verifyFromConnector = u
		}
	}
	if tel, terr := s.workspace.GetConnectorByType(ctx, envID, store.ConnectorTelemetry); terr == nil {
		out.TelemetryReady = store.ConnectorReady(tel)
		if verifyFromConnector == "" && tel.Enabled {
			if u := connectorConfigString(tel.Config, "healthUrl"); u != "" {
				verifyFromConnector = u
			}
		}
		copilotURL, spottingURL, telURLs := telemetryAPIURLsFromConnector(tel, out.CopilotAPIURL, out.SpottingAPIURL)
		out.CopilotAPIURL = copilotURL
		out.SpottingAPIURL = spottingURL
		if telURLs {
			usedConnector = true
		}
	}
	if verifyFromConnector != "" {
		out.VerifyURL = verifyFromConnector
		usedConnector = true
	}
	if out.VerifyURL != "" && strings.HasPrefix(out.VerifyURL, "/") && s.cfg != nil && s.cfg.PublicURL != "" {
		out.VerifyURL = strings.TrimSuffix(s.cfg.PublicURL, "/") + out.VerifyURL
	}

	if cicd, cerr := s.workspace.GetConnectorByType(ctx, envID, store.ConnectorCICD); cerr == nil && cicd.Enabled {
		if wf, fromCICD := rollbackWorkflowFromConnector(cicd, out.RollbackWorkflow); fromCICD {
			out.RollbackWorkflow = wf
			usedConnector = true
		}
	}

	if agent, aerr := s.workspace.GetConnectorByType(ctx, envID, store.ConnectorIssueAgent); aerr == nil {
		out.AgentReady = store.ConnectorReady(agent)
		out.AgentRoute = connectorConfigString(agent.Config, "route")
		out.AgentProvider = connectorConfigString(agent.Config, "provider")
		if out.AgentRoute == "" {
			out.AgentRoute = "mcp"
		}
	}

	if usedConnector {
		out.Source = "connector"
	}

	slog.Debug("runtime settings resolved",
		"source", out.Source,
		"repo", out.Repo,
		"verifyUrl", out.VerifyURL,
		"copilotApiUrl", out.CopilotAPIURL,
		"spottingApiUrl", out.SpottingAPIURL,
		"rollbackWorkflow", out.RollbackWorkflow,
		"envId", out.EnvironmentID,
	)
	return out
}

// rollbackWorkflowFromConnector prefers cicd.rollbackWorkflow, then a workflows[] entry
// whose name contains "rollback", else returns the global fallback.
func rollbackWorkflowFromConnector(cicd store.Connector, fallback string) (workflow string, used bool) {
	workflow = fallback
	if !cicd.Enabled {
		return
	}
	if explicit := connectorConfigString(cicd.Config, "rollbackWorkflow"); explicit != "" {
		return explicit, true
	}
	if name := firstWorkflowContaining(cicd.Config, "rollback"); name != "" {
		return name, true
	}
	return
}

func firstWorkflowContaining(cfg map[string]any, needle string) string {
	if cfg == nil {
		return ""
	}
	v, ok := cfg["workflows"]
	if !ok || v == nil {
		return ""
	}
	needle = strings.ToLower(needle)
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			name := strings.TrimSpace(fmt.Sprint(item))
			if name != "" && strings.Contains(strings.ToLower(name), needle) {
				return name
			}
		}
	case []string:
		for _, item := range t {
			name := strings.TrimSpace(item)
			if name != "" && strings.Contains(strings.ToLower(name), needle) {
				return name
			}
		}
	case string:
		name := strings.TrimSpace(t)
		if name != "" && strings.Contains(strings.ToLower(name), needle) {
			return name
		}
	}
	return ""
}

// telemetryAPIURLsFromConnector prefers non-secret telemetry connector URLs when the
// connector is enabled+ready; otherwise returns the provided global fallbacks.
// Returns usedConnector=true when at least one URL came from the connector.
func telemetryAPIURLsFromConnector(tel store.Connector, fallbackCopilot, fallbackSpotting string) (copilot, spotting string, usedConnector bool) {
	copilot, spotting = fallbackCopilot, fallbackSpotting
	if !store.ConnectorReady(tel) {
		return
	}
	if u := connectorConfigString(tel.Config, "copilotApiUrl"); u != "" {
		copilot = u
		usedConnector = true
	}
	if u := connectorConfigString(tel.Config, "spottingApiUrl"); u != "" {
		spotting = u
		usedConnector = true
	}
	return
}

func (s *Service) resolveEnvironmentID(ctx context.Context, repo string) (int, error) {
	if s.workspace == nil {
		return 0, nil
	}
	if repo != "" {
		if id, err := s.workspace.FindEnvironmentByGitRepo(ctx, repo); err == nil && id > 0 {
			return id, nil
		}
	}
	return s.workspace.DefaultEnvironmentID(ctx)
}

func connectorConfigString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
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

// CopilotClientFor returns an HTTP client pointed at the resolved Copilot API URL.
// Tokens always come from global config (never from connector config).
func (s *Service) CopilotClientFor(rt RuntimeSettings) *clients.CopilotClient {
	url := strings.TrimSpace(rt.CopilotAPIURL)
	token := ""
	if s.cfg != nil {
		if url == "" {
			url = s.cfg.CopilotAPIURL
		}
		token = s.cfg.CopilotServiceToken
	}
	if s.copilot != nil && s.copilot.BaseURL == url {
		return s.copilot
	}
	return clients.NewCopilotClient(url, token)
}

// SpottingClientFor returns an HTTP client pointed at the resolved Spotting API URL.
// Tokens always come from global config (never from connector config).
func (s *Service) SpottingClientFor(rt RuntimeSettings) *clients.SpottingClient {
	url := strings.TrimSpace(rt.SpottingAPIURL)
	token := ""
	if s.cfg != nil {
		if url == "" {
			url = s.cfg.SpottingAPIURL
		}
		token = s.cfg.SpottingServiceToken
	}
	if s.spotting != nil && s.spotting.BaseURL == url {
		return s.spotting
	}
	return clients.NewSpottingClient(url, token)
}
