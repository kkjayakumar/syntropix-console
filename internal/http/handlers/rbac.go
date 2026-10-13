package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/http/middleware"
	"github.com/kkjayakumar/AegIsOps/internal/store"
)

type RBACHandler struct {
	Store *store.RBACStore
}

// ActorUserID resolves the request actor.
// When API token auth verified the request, spoofable X-User-* headers are ignored.
func ActorUserID(r *http.Request, rbac *store.RBACStore) int {
	if uid, ok := middleware.AuthUserIDFromContext(r.Context()); ok {
		return uid
	}
	if v := strings.TrimSpace(r.Header.Get("X-User-Id")); v != "" {
		if id, err := strconv.Atoi(v); err == nil && id > 0 {
			return id
		}
	}
	if rbac == nil {
		return 1
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if email := strings.TrimSpace(r.Header.Get("X-User-Email")); email != "" {
		if u, err := rbac.GetUserByEmail(ctx, email); err == nil {
			return u.ID
		}
	}
	if u, err := rbac.GetUserByEmail(ctx, "admin@syntropix.io"); err == nil {
		return u.ID
	}
	return 1
}

func (h *RBACHandler) Me(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	uid := ActorUserID(r, h.Store)
	u, err := h.Store.GetUserByID(ctx, uid)
	if err != nil {
		WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"user": u, "policies": store.DefaultRolePolicies()})
}

func (h *RBACHandler) ProjectAccessDecision(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	projectID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || projectID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	uid := ActorUserID(r, h.Store)
	dec, err := h.Store.EffectiveAccess(ctx, uid, projectID)
	if err != nil {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, dec)
}

func (h *RBACHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	users, err := h.Store.ListUsers(ctx)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list users"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (h *RBACHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	u, err := h.Store.CreateUser(ctx, body.Email, body.Name)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create user"})
		return
	}
	uid := ActorUserID(r, h.Store)
	_ = h.Store.Audit(ctx, &uid, "user.create", "user", strconv.Itoa(u.ID), nil, nil, map[string]any{"email": u.Email})
	WriteJSON(w, http.StatusCreated, map[string]any{"user": u})
}

func (h *RBACHandler) ListGroups(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	groups, err := h.Store.ListGroups(ctx, "syntropix")
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list groups"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (h *RBACHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	var body struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	g, err := h.Store.CreateGroup(ctx, "syntropix", body.Slug, body.Name)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create group"})
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{"group": g})
}

func (h *RBACHandler) ListAccess(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	projectID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || projectID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	uid := ActorUserID(r, h.Store)
	if dec, err := h.Store.EffectiveAccess(ctx, uid, projectID); err != nil || !dec.CanManageRBAC {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required to manage access"})
		return
	}
	rows, err := h.Store.ListProjectAccess(ctx, projectID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list access"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"access": rows, "policies": store.DefaultRolePolicies()})
}

func (h *RBACHandler) UpsertAccess(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	projectID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || projectID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project id"})
		return
	}
	var body struct {
		PrincipalType  string `json:"principalType"`
		PrincipalID    int    `json:"principalId"`
		Role           string `json:"role"`
		ChatLimitDaily int    `json:"chatLimitDaily"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	uid := ActorUserID(r, h.Store)
	if dec, err := h.Store.EffectiveAccess(ctx, uid, projectID); err != nil || !dec.CanManageRBAC {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
		return
	}
	a, err := h.Store.UpsertProjectAccess(ctx, projectID, body.PrincipalType, body.PrincipalID, body.Role, body.ChatLimitDaily)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to grant access"})
		return
	}
	_ = h.Store.Audit(ctx, &uid, "rbac.grant", "project_access", strconv.Itoa(a.ID), &projectID, nil, map[string]any{
		"role": body.Role, "principalType": body.PrincipalType, "principalId": body.PrincipalID,
	})
	WriteJSON(w, http.StatusOK, map[string]any{"access": a})
}

func (h *RBACHandler) GetPipeline(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	p, err := h.Store.GetPipeline(ctx, envID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load pipeline"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"pipeline": p, "editPolicy": "admin only"})
}

func (h *RBACHandler) UpsertPipeline(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	var body struct {
		ProjectID  int            `json:"projectId"`
		Name       string         `json:"name"`
		Definition map[string]any `json:"definition"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	uid := ActorUserID(r, h.Store)
	if body.ProjectID > 0 {
		dec, err := h.Store.EffectiveAccess(ctx, uid, body.ProjectID)
		if err != nil || !dec.CanEditPipeline {
			WriteJSON(w, http.StatusForbidden, map[string]string{"error": "only project admin can edit pipeline"})
			return
		}
	}
	name := body.Name
	if name == "" {
		name = "closed-loop"
	}
	p, err := h.Store.UpsertPipeline(ctx, envID, name, body.Definition, uid)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save pipeline"})
		return
	}
	_ = h.Store.Audit(ctx, &uid, "pipeline.update", "pipeline", strconv.Itoa(p.ID), &body.ProjectID, &envID, nil)
	WriteJSON(w, http.StatusOK, map[string]any{"pipeline": p})
}

func (h *RBACHandler) ListVariables(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	vars, err := h.Store.ListEnvVariables(ctx, envID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list variables"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"variables": vars})
}

func (h *RBACHandler) UpsertVariable(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	var body struct {
		KeyName   string `json:"keyName"`
		ValueText string `json:"valueText"`
		ProjectID int    `json:"projectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	uid := ActorUserID(r, h.Store)
	if body.ProjectID > 0 {
		dec, err := h.Store.EffectiveAccess(ctx, uid, body.ProjectID)
		if err != nil || !(dec.Role == store.RoleAdmin || dec.Role == store.RoleMaintainer) {
			WriteJSON(w, http.StatusForbidden, map[string]string{"error": "admin or maintainer required to edit variables"})
			return
		}
	}
	v, err := h.Store.UpsertEnvVariable(ctx, envID, body.KeyName, body.ValueText)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save variable"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"variable": v})
}
