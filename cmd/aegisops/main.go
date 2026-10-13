package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/kkjayakumar/AegIsOps/internal/clients"
	"github.com/kkjayakumar/AegIsOps/internal/config"
	"github.com/kkjayakumar/AegIsOps/internal/db"
	httpserver "github.com/kkjayakumar/AegIsOps/internal/http"
	"github.com/kkjayakumar/AegIsOps/internal/sentinel"
	"github.com/kkjayakumar/AegIsOps/internal/store"
	"github.com/kkjayakumar/AegIsOps/internal/teams"
	"github.com/kkjayakumar/AegIsOps/internal/telemetry"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("config validation failed", "error", err)
		os.Exit(1)
	}
	if ok, hint := teams.ValidateWebhookURL(cfg.TeamsWebhookURL); !ok {
		slog.Warn("TEAMS_WEBHOOK_URL misconfigured", "hint", hint)
	}
	if err := telemetry.InitSentry(cfg.SentryDSN, cfg.Environment); err != nil {
		slog.Warn("sentry init failed", "error", err)
	}
	defer telemetry.Flush()

	ctx := context.Background()

	if err := db.RunMigrations(ctx, cfg.DatabaseURL); err != nil {
		slog.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Warn("database unavailable, starting without db", "error", err)
	}

	server := httpserver.New(cfg, database)

	if database != nil && database.Pool != nil {
		ws := store.NewWorkspaceStore(database.Pool)
		// Ensure disabled template rows exist; do not auto-enable from platform secrets.
		if err := ws.EnsureAllConnectors(ctx, nil); err != nil {
			slog.Warn("connector bootstrap", "error", err)
		} else {
			slog.Info("connector bootstrap complete")
		}

		copilot := clients.NewCopilotClient(cfg.CopilotAPIURL, cfg.CopilotServiceToken)
		sentinelStore := store.NewSentinelStore(database.Pool)
		sentinel.NewWorker(sentinelStore, copilot).Start(ctx)
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	_ = server.Close()
	if database != nil {
		database.Close()
	}
	slog.Info("shutdown complete")
}
