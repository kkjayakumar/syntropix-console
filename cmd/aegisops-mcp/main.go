package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/kkjayakumar/AegIsOps/internal/clients"
	"github.com/kkjayakumar/AegIsOps/internal/config"
	"github.com/kkjayakumar/AegIsOps/internal/db"
	"github.com/kkjayakumar/AegIsOps/internal/mcpserver"
	"github.com/kkjayakumar/AegIsOps/internal/orchestrator"
	"github.com/kkjayakumar/AegIsOps/internal/store"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	issues := store.NewIssueStore(database.Pool)
	webhooks := store.NewWebhookStore(database.Pool)
	deployments := store.NewDeploymentStore(database.Pool)
	sentinel := store.NewSentinelStore(database.Pool)
	gateway := store.NewGatewayStore(database.Pool)
	workspace := store.NewWorkspaceStore(database.Pool)
	orch := orchestrator.New(cfg, issues, webhooks, deployments, sentinel, gateway, workspace)
	copilot := clients.NewCopilotClient(cfg.CopilotAPIURL, cfg.CopilotServiceToken)
	mcpSrv := mcpserver.New(issues, copilot, orch)

	slog.Info("aegisops-mcp starting", "transport", "stdio")
	if err := server.ServeStdio(mcpSrv.MCPServer()); err != nil {
		slog.Error("mcp server", "error", err)
		os.Exit(1)
	}
}
