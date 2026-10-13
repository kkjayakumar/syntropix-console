package middleware

import (
	"context"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/google/uuid"
)

type ctxKey string

const RequestIDKey ctxKey = "request_id"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = "req_" + uuid.NewString()[:12]
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		if hub := sentry.CurrentHub(); hub != nil {
			hub.Scope().SetTag("request_id", id)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func FromContext(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}
