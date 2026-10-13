package httpserver

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/clients"
	"github.com/kkjayakumar/AegIsOps/internal/config"
	"github.com/kkjayakumar/AegIsOps/internal/db"
	"github.com/kkjayakumar/AegIsOps/internal/http/handlers"
	"github.com/kkjayakumar/AegIsOps/internal/http/middleware"
	"github.com/kkjayakumar/AegIsOps/internal/orchestrator"
	"github.com/kkjayakumar/AegIsOps/internal/store"
)

type Server struct {
	cfg   *config.Config
	db    *db.DB
	start time.Time
	mux   *http.ServeMux
	http  *http.Server
}

func New(cfg *config.Config, database *db.DB) *Server {
	s := &Server{cfg: cfg, db: database, start: time.Now(), mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	var (
		webhookStore    *store.WebhookStore
		issueStore      *store.IssueStore
		deploymentStore *store.DeploymentStore
		sentinelStore   *store.SentinelStore
		gatewayStore    *store.GatewayStore
		statsStore      *store.StatsStore
		workspaceStore  *store.WorkspaceStore
		graphStore      *store.GraphStore
		rbacStore       *store.RBACStore
		orch            *orchestrator.Service
	)
	if s.db != nil && s.db.Pool != nil {
		webhookStore = store.NewWebhookStore(s.db.Pool)
		issueStore = store.NewIssueStore(s.db.Pool)
		deploymentStore = store.NewDeploymentStore(s.db.Pool)
		sentinelStore = store.NewSentinelStore(s.db.Pool)
		gatewayStore = store.NewGatewayStore(s.db.Pool)
		statsStore = store.NewStatsStore(s.db.Pool)
		workspaceStore = store.NewWorkspaceStore(s.db.Pool)
		graphStore = store.NewGraphStore(s.db.Pool)
		rbacStore = store.NewRBACStore(s.db.Pool)
		orch = orchestrator.New(s.cfg, issueStore, webhookStore, deploymentStore, sentinelStore, gatewayStore, workspaceStore)
	}

	health := &handlers.HealthHandler{DB: s.db, Config: s.cfg, Orchestrator: orch, StartTime: s.start}

	issues := &handlers.IssuesHandler{Store: issueStore, Deployments: deploymentStore, Gateway: gatewayStore, Orchestrator: orch, Config: s.cfg}
	deployments := &handlers.DeploymentsHandler{Store: deploymentStore}
	webhooks := &handlers.WebhooksHandler{Secret: s.cfg.AegisWebhookSecret, Store: webhookStore, Orchestrator: orch}
	githubHook := &handlers.GitHubWebhookHandler{Secret: s.cfg.GitHubWebhookSecret, Orchestrator: orch}
	gateway := &handlers.GatewayHandler{Orchestrator: orch, PublicURL: s.cfg.PublicURL}
	stats := &handlers.StatsHandler{Store: statsStore}
	workspace := &handlers.WorkspaceHandler{
		Store:        workspaceStore,
		RBAC:         rbacStore,
		Config:       s.cfg,
		SecretsKey:   s.cfg.WorkspaceSecretsKey,
		StrictCrypto: s.cfg.IsProduction(),
	}
	llmAPI := &handlers.LLMHandler{
		Workspace:  workspaceStore,
		Graph:      graphStore,
		Issues:     issueStore,
		RBAC:       rbacStore,
		SecretsKey: s.cfg.WorkspaceSecretsKey,
	}
	graphAPI := &handlers.GraphHandler{Graph: graphStore, Issues: issueStore, Workspace: workspaceStore}
	rbacAPI := &handlers.RBACHandler{Store: rbacStore}
	debug := &handlers.DebugHandler{
		AllowDebug:   !s.cfg.IsProduction() || s.cfg.AllowDogfood,
		Orchestrator: orch,
		Copilot:      clients.NewCopilotClient(s.cfg.CopilotAPIURL, s.cfg.CopilotServiceToken),
		PublicURL:    s.cfg.PublicURL,
	}

	s.mux.HandleFunc("GET /healthz", health.Healthz)
	s.mux.HandleFunc("GET /readyz", health.Readyz)
	s.mux.HandleFunc("GET /api/v1/version", health.Version)
	s.mux.HandleFunc("GET /api/v1/stats", stats.Get)

	s.mux.HandleFunc("GET /api/v1/workspace/bootstrap", workspace.Bootstrap)
	s.mux.HandleFunc("GET /api/v1/workspace/projects", workspace.ListProjects)
	s.mux.HandleFunc("POST /api/v1/workspace/projects", workspace.CreateProject)
	s.mux.HandleFunc("GET /api/v1/workspace/projects/{id}/environments", workspace.ListEnvironments)
	s.mux.HandleFunc("POST /api/v1/workspace/projects/{id}/environments", workspace.CreateEnvironment)
	s.mux.HandleFunc("DELETE /api/v1/workspace/environments/{id}", workspace.DeleteEnvironment)
	s.mux.HandleFunc("GET /api/v1/workspace/connectors/defaults", workspace.ConnectorDefaults)
	s.mux.HandleFunc("GET /api/v1/workspace/environments/{id}/connectors", workspace.ListConnectors)
	s.mux.HandleFunc("POST /api/v1/workspace/environments/{id}/connectors", workspace.UpsertConnector)
	s.mux.HandleFunc("POST /api/v1/workspace/environments/{id}/connectors/apply-defaults", workspace.ApplyConnectorDefaults)
	s.mux.HandleFunc("GET /api/v1/workspace/environments/{id}/secrets", workspace.ListSecrets)
	s.mux.HandleFunc("POST /api/v1/workspace/environments/{id}/secrets", workspace.UpsertSecret)
	s.mux.HandleFunc("DELETE /api/v1/workspace/environments/{id}/secrets/{secretId}", workspace.DeleteSecret)

	s.mux.HandleFunc("GET /api/v1/workspace/environments/{id}/llm/status", llmAPI.Status)
	s.mux.HandleFunc("POST /api/v1/workspace/environments/{id}/llm/complete", llmAPI.Complete)
	s.mux.HandleFunc("GET /api/v1/workspace/environments/{id}/llm/runs", graphAPI.ListRuns)
	s.mux.HandleFunc("GET /api/v1/workspace/environments/{id}/graph", graphAPI.Get)
	s.mux.HandleFunc("POST /api/v1/workspace/environments/{id}/graph/nodes", graphAPI.UpsertNode)
	s.mux.HandleFunc("POST /api/v1/workspace/environments/{id}/graph/ingest", graphAPI.Ingest)

	s.mux.HandleFunc("GET /api/v1/rbac/me", rbacAPI.Me)
	s.mux.HandleFunc("GET /api/v1/rbac/users", rbacAPI.ListUsers)
	s.mux.HandleFunc("POST /api/v1/rbac/users", rbacAPI.CreateUser)
	s.mux.HandleFunc("GET /api/v1/rbac/groups", rbacAPI.ListGroups)
	s.mux.HandleFunc("POST /api/v1/rbac/groups", rbacAPI.CreateGroup)
	s.mux.HandleFunc("GET /api/v1/rbac/projects/{id}/access", rbacAPI.ListAccess)
	s.mux.HandleFunc("POST /api/v1/rbac/projects/{id}/access", rbacAPI.UpsertAccess)
	s.mux.HandleFunc("GET /api/v1/rbac/projects/{id}/decision", rbacAPI.ProjectAccessDecision)
	s.mux.HandleFunc("GET /api/v1/workspace/environments/{id}/pipeline", rbacAPI.GetPipeline)
	s.mux.HandleFunc("POST /api/v1/workspace/environments/{id}/pipeline", rbacAPI.UpsertPipeline)
	s.mux.HandleFunc("GET /api/v1/workspace/environments/{id}/variables", rbacAPI.ListVariables)
	s.mux.HandleFunc("POST /api/v1/workspace/environments/{id}/variables", rbacAPI.UpsertVariable)

	s.mux.HandleFunc("GET /api/v1/issues", issues.List)
	s.mux.HandleFunc("POST /api/v1/issues", issues.Create)
	s.mux.HandleFunc("GET /api/v1/issues/{id}", issues.Get)
	s.mux.HandleFunc("GET /api/v1/issues/{id}/bundle", issues.Bundle)
	s.mux.HandleFunc("POST /api/v1/issues/{id}/start-fix", issues.StartFix)
	s.mux.HandleFunc("POST /api/v1/issues/{id}/open-pr", issues.OpenPR)
	s.mux.HandleFunc("POST /api/v1/issues/{id}/start-feature", issues.StartFeature)
	s.mux.HandleFunc("POST /api/v1/issues/{id}/gateway/approve", issues.GatewayApprove)
	s.mux.HandleFunc("POST /api/v1/issues/{id}/gateway/reject", issues.GatewayReject)
	s.mux.HandleFunc("POST /api/v1/issues/{id}/close", issues.Close)

	s.mux.HandleFunc("GET /api/v1/deployments", deployments.List)
	s.mux.HandleFunc("GET /api/v1/deployments/{id}", deployments.Get)
	s.mux.HandleFunc("GET /api/v1/deployments/{id}/logs", deployments.Logs)

	s.mux.HandleFunc("GET /api/v1/gateway/approve", gateway.Approve)
	s.mux.HandleFunc("GET /api/v1/gateway/reject", gateway.Reject)

	s.mux.HandleFunc("GET /api/v1/webhooks/events", webhooks.List)
	s.mux.Handle("POST /api/v1/webhooks/copilot", s.rateLimitWebhooks(http.HandlerFunc(webhooks.Copilot)))
	s.mux.Handle("POST /api/v1/webhooks/spotting", s.rateLimitWebhooks(http.HandlerFunc(webhooks.Spotting)))
	s.mux.Handle("POST /api/v1/github/webhook", s.rateLimitWebhooks(http.HandlerFunc(githubHook.Handle)))

	s.mux.HandleFunc("POST /api/v1/debug/trigger-error", debug.TriggerError)
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%s", s.cfg.Port)
	s.http = &http.Server{
		Addr:         addr,
		Handler:      middleware.RequestID(s.logging(middleware.APIAuth(s.cfg, s.rootHandler()))),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	slog.Info("aegisops listening", "addr", addr)
	return s.http.ListenAndServe()
}

func (s *Server) Close() error {
	if s.http == nil {
		return nil
	}
	return s.http.Close()
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
	})
}
