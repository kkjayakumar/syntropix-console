package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pressly/goose/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func RunMigrations(ctx context.Context, databaseURL string) error {
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required for migrations")
	}

	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = filepath.Join("migrations")
	}
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	const maxAttempts = 15
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = runMigrationsOnce(ctx, databaseURL, migrationsDir)
		if lastErr == nil {
			return nil
		}
		if attempt == maxAttempts || !isRetryableMigrationErr(lastErr) {
			break
		}
		slog.Warn("migrations waiting for postgres", "attempt", attempt, "error", lastErr)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return lastErr
}

func runMigrationsOnce(ctx context.Context, databaseURL, migrationsDir string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer db.Close()

	if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

func isRetryableMigrationErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connect: connection") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "starting up")
}
