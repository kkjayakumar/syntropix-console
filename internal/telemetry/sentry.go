package telemetry

import (
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
)

func InitSentry(dsn, environment string) error {
	if dsn == "" {
		return nil
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      environment,
		AttachStacktrace: true,
		TracesSampleRate: 0,
	})
	if err != nil {
		return err
	}
	slog.Info("sentry initialized", "environment", environment)
	return nil
}

func Flush() {
	sentry.Flush(2 * time.Second)
}

func CaptureError(err error, requestID string) {
	if err == nil {
		return
	}
	msg := err.Error()
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("request_id", requestID)
		scope.SetTag("service", "aegisops")
		scope.SetLevel(sentry.LevelError)
		sentry.CaptureEvent(&sentry.Event{
			Message: msg,
			Level:   sentry.LevelError,
			Exception: []sentry.Exception{{
				Type:  "DogfoodError",
				Value: msg,
			}},
			Tags: map[string]string{
				"request_id": requestID,
				"service":    "aegisops",
			},
		})
	})
}
