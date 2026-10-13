package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kkjayakumar/AegIsOps/internal/config"
)

func TestAPIAuthAllowsPublicPathsWithoutToken(t *testing.T) {
	cfg := &config.Config{
		Environment: "production",
		APIToken:    "prod-token-value-32chars-xxxxxx",
	}
	h := APIAuth(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	for _, path := range []string{"/healthz", "/readyz", "/api/v1/version", "/api/v1/webhooks/copilot", "/api/v1/github/webhook", "/api/v1/gateway/approve"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if path == "/api/v1/webhooks/copilot" || path == "/api/v1/github/webhook" {
			req = httptest.NewRequest(http.MethodPost, path, nil)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: got %d want 200", path, rr.Code)
		}
	}
}

func TestAPIAuthDeniesProtectedWithoutToken(t *testing.T) {
	cfg := &config.Config{
		Environment: "production",
		APIToken:    "prod-token-value-32chars-xxxxxx",
	}
	h := APIAuth(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want 401", rr.Code)
	}
}

func TestAPIAuthAllowsBearerAndCustomHeader(t *testing.T) {
	token := "prod-token-value-32chars-xxxxxx"
	cfg := &config.Config{
		Environment: "production",
		APIToken:    token,
		AuthUserID:  7,
	}
	h := APIAuth(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := AuthUserIDFromContext(r.Context())
		if !ok || uid != 7 {
			t.Fatalf("auth user = %d ok=%v", uid, ok)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/issues", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("bearer: got %d want 200", rr.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	req2.Header.Set("X-AegisOps-Token", token)
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("custom header: got %d want 200", rr2.Code)
	}
}

func TestAPIAuthSkippedWhenInsecureAllowed(t *testing.T) {
	cfg := &config.Config{
		Environment:       "production",
		APIToken:          "prod-token-value-32chars-xxxxxx",
		AllowInsecureAuth: true,
	}
	h := APIAuth(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if AuthVerified(r.Context()) {
			t.Fatal("expected no auth context in insecure mode")
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d want 200", rr.Code)
	}
}

func TestAPIAuthSkippedInDevelopmentWithoutToken(t *testing.T) {
	cfg := &config.Config{Environment: "development"}
	h := APIAuth(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspace/environments/1", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d want 200", rr.Code)
	}
}

func TestAPIAuthRejectsWrongToken(t *testing.T) {
	cfg := &config.Config{
		Environment: "production",
		APIToken:    "correct-token-abcdefghijklmnop",
	}
	h := APIAuth(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues", nil)
	req.Header.Set("Authorization", "Bearer wrong-token-abcdefghijklmnopxx")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want 401", rr.Code)
	}
}
