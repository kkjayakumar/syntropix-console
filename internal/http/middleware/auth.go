package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kkjayakumar/AegIsOps/internal/config"
)

type authCtxKey string

const (
	authUserIDKey   authCtxKey = "auth_user_id"
	authVerifiedKey authCtxKey = "auth_verified"
)

// APIAuth enforces Bearer / X-AegisOps-Token when config.AuthEnforced().
// Public health, version, signed webhooks, and gateway link endpoints stay open.
func APIAuth(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg == nil || !cfg.AuthEnforced() || !requiresAPIAuth(r) {
			next.ServeHTTP(w, r)
			return
		}
		token := extractAPIToken(r)
		if token == "" || !tokenMatches(cfg.APIToken, token) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("WWW-Authenticate", `Bearer realm="aegisops"`)
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "unauthorized: provide Authorization: Bearer <AEGISOPS_API_TOKEN> or X-AegisOps-Token",
			})
			return
		}
		uid := cfg.AuthUserID
		if uid <= 0 {
			uid = 1
		}
		ctx := context.WithValue(r.Context(), authVerifiedKey, true)
		ctx = context.WithValue(ctx, authUserIDKey, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requiresAPIAuth(r *http.Request) bool {
	path := r.URL.Path
	switch path {
	case "/healthz", "/readyz", "/api/v1/version":
		return false
	case "/api/v1/gateway/approve", "/api/v1/gateway/reject":
		return false
	}
	if strings.HasPrefix(path, "/api/v1/webhooks/") {
		return false
	}
	if path == "/api/v1/github/webhook" {
		return false
	}
	return strings.HasPrefix(path, "/api/")
}

func extractAPIToken(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-AegisOps-Token")); v != "" {
		return v
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return strings.TrimSpace(auth[len(prefix):])
	}
	return ""
}

func tokenMatches(expected, got string) bool {
	if expected == "" || got == "" {
		return false
	}
	if len(expected) != len(got) {
		// Constant-time compare requires equal length; still avoid early true.
		_ = subtle.ConstantTimeCompare([]byte(expected), []byte(expected))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(got)) == 1
}

// ContextWithAuthenticatedUser attaches a verified API actor (tests + future session auth).
func ContextWithAuthenticatedUser(ctx context.Context, userID int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if userID <= 0 {
		userID = 1
	}
	ctx = context.WithValue(ctx, authVerifiedKey, true)
	return context.WithValue(ctx, authUserIDKey, userID)
}

// AuthUserIDFromContext returns the authenticated actor when API token auth succeeded.
func AuthUserIDFromContext(ctx context.Context) (int, bool) {
	if ctx == nil {
		return 0, false
	}
	verified, _ := ctx.Value(authVerifiedKey).(bool)
	if !verified {
		return 0, false
	}
	uid, ok := ctx.Value(authUserIDKey).(int)
	if !ok || uid <= 0 {
		return 0, false
	}
	return uid, true
}

// AuthVerified reports whether the request passed API token auth.
func AuthVerified(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(authVerifiedKey).(bool)
	return v
}
