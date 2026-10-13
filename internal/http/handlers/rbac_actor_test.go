package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kkjayakumar/AegIsOps/internal/config"
	"github.com/kkjayakumar/AegIsOps/internal/http/middleware"
)

func TestActorUserIDPrefersAuthContextOverSpoofedHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rbac/me", nil)
	req.Header.Set("X-User-Id", "999")
	req = req.WithContext(middleware.ContextWithAuthenticatedUser(req.Context(), 3))
	if got := ActorUserID(req, nil); got != 3 {
		t.Fatalf("ActorUserID = %d, want 3", got)
	}
}

func TestActorUserIDFallsBackToHeaderWhenAuthOff(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/rbac/me", nil)
	req.Header.Set("X-User-Id", "42")
	if got := ActorUserID(req, nil); got != 42 {
		t.Fatalf("ActorUserID = %d, want 42", got)
	}
}

func TestAPIAuthThenActorUserIDEndToEnd(t *testing.T) {
	token := "e2e-api-token-xxxx"
	cfg := &config.Config{Environment: "production", APIToken: token, AuthUserID: 5}
	var got int
	h := middleware.APIAuth(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = ActorUserID(r, nil)
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues", nil)
	req.Header.Set("X-User-Id", "999")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if got != 5 {
		t.Fatalf("ActorUserID = %d, want 5", got)
	}
}
