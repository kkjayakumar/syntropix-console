package sentinel

import (
	"context"
	"log/slog"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/clients"
	"github.com/kkjayakumar/AegIsOps/internal/store"
)

type Worker struct {
	store   *store.SentinelStore
	copilot *clients.CopilotClient
	interval time.Duration
}

func NewWorker(sentinel *store.SentinelStore, copilot *clients.CopilotClient) *Worker {
	return &Worker{
		store:    sentinel,
		copilot:  copilot,
		interval: 5 * time.Minute,
	}
}

func (w *Worker) Start(ctx context.Context) {
	if w == nil || w.store == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.cleanup(ctx)
			}
		}
	}()
}

func (w *Worker) cleanup(ctx context.Context) {
	expired, err := w.store.ListExpired(ctx, time.Now())
	if err != nil {
		slog.Error("sentinel list expired failed", "error", err)
		return
	}
	for _, m := range expired {
		if m.CopilotMonitorID != nil && w.copilot != nil {
			if err := w.copilot.DeleteUptimeMonitor(ctx, *m.CopilotMonitorID); err != nil {
				slog.Warn("remove temp monitor failed", "monitorId", *m.CopilotMonitorID, "error", err)
			}
		}
		_ = w.store.UpdateStatus(ctx, m.ID, "removed")
		slog.Info("sentinel temp monitor removed", "id", m.ID)
	}
}
