package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kkjayakumar/AegIsOps/internal/config"
	"github.com/kkjayakumar/AegIsOps/internal/store"
	"github.com/kkjayakumar/AegIsOps/internal/workspace"
)

type WorkspaceHandler struct {
	Store        *store.WorkspaceStore
	RBAC         *store.RBACStore
	Config       *config.Config
	SecretsKey   string
	StrictCrypto bool
}

func (h *WorkspaceHandler) tenantSlug(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Tenant-Slug")); v != "" {
		return v
	}
	return "syntropix"
}

func (h *WorkspaceHandler) cipher() (*workspace.Cipher, error) {
	if h.StrictCrypto {
		return workspace.NewCipherStrict(h.SecretsKey)
	}
	return workspace.NewCipherWithOptions(h.SecretsKey, workspace.Options{AllowDevFallback: true})
}

func (h *WorkspaceHandler) requireSecretWrite(ctx context.Context, r *http.Request, envID int) (int, int, error) {
	projectID, err := h.Store.ProjectIDForEnvironment(ctx, envID)
	if err != nil {
		return 0, 0, fmt.Errorf("environment not found")
	}
	if h.RBAC == nil {
		return ActorUserID(r, nil), projectID, nil
	}
	uid := ActorUserID(r, h.RBAC)
	dec, err := h.RBAC.EffectiveAccess(ctx, uid, projectID)
	if err != nil {
		return uid, projectID, err
	}
	if !dec.CanWriteSecret {
		return uid, projectID, fmt.Errorf("secret write requires admin or maintainer role")
	}
	return uid, projectID, nil
}

func (h *WorkspaceHandler) requireConnectorWrite(ctx context.Context, r *http.Request, envID int) (int, int, error) {
	projectID, err := h.Store.ProjectIDForEnvironment(ctx, envID)
	if err != nil {
		return 0, 0, fmt.Errorf("environment not found")
	}
	if h.RBAC == nil {
		return ActorUserID(r, nil), projectID, nil
	}
	uid := ActorUserID(r, h.RBAC)
	dec, err := h.RBAC.EffectiveAccess(ctx, uid, projectID)
	if err != nil {
		return uid, projectID, err
	}
	// Maintainers operate connectors; admins inherit via CanWriteSecret.
	if !dec.CanWriteSecret {
		return uid, projectID, fmt.Errorf("connector write requires admin or maintainer role")
	}
	return uid, projectID, nil
}

func (h *WorkspaceHandler) Bootstrap(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	boot, err := h.Store.Bootstrap(ctx, h.tenantSlug(r))
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load workspace"})
		return
	}
	WriteJSON(w, http.StatusOK, boot)
}

func (h *WorkspaceHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	projects, err := h.Store.ListProjects(ctx, h.tenantSlug(r))
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list projects"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"projects": projects})
}

func (h *WorkspaceHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	var body struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.Slug = strings.TrimSpace(strings.ToLower(body.Slug))
	body.Name = strings.TrimSpace(body.Name)
	if body.Slug == "" || body.Name == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "slug and name are required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	p, err := h.Store.CreateProject(ctx, h.tenantSlug(r), body.Slug, body.Name, body.Description)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create project"})
		return
	}
	WriteJSON(w, http.StatusCreated, map[string]any{"project": p})
}

func (h *WorkspaceHandler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
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
	envs, err := h.Store.ListEnvironments(ctx, projectID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list environments"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"environments": envs})
}

func (h *WorkspaceHandler) requireProjectWrite(ctx context.Context, r *http.Request, projectID int) (int, error) {
	if h.RBAC == nil {
		return ActorUserID(r, nil), nil
	}
	uid := ActorUserID(r, h.RBAC)
	dec, err := h.RBAC.EffectiveAccess(ctx, uid, projectID)
	if err != nil {
		return uid, err
	}
	if !dec.CanWriteSecret {
		return uid, fmt.Errorf("environment manage requires admin or maintainer role")
	}
	return uid, nil
}

func (h *WorkspaceHandler) CreateEnvironment(w http.ResponseWriter, r *http.Request) {
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
		Slug      string `json:"slug"`
		Name      string `json:"name"`
		SortOrder int    `json:"sortOrder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.Slug = strings.TrimSpace(strings.ToLower(body.Slug))
	body.Name = strings.TrimSpace(body.Name)
	if body.Slug == "" || body.Name == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "slug and name are required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	uid, err := h.requireProjectWrite(ctx, r, projectID)
	if err != nil {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	e, err := h.Store.CreateEnvironment(ctx, projectID, body.Slug, body.Name, body.SortOrder)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create environment"})
		return
	}
	// Seed disabled empty connector templates — never auto-mark Connected.
	_ = h.Store.EnsureEnvironmentConnectors(ctx, e.ID, nil)
	if h.RBAC != nil {
		_ = h.RBAC.Audit(ctx, &uid, "environment.create", "environment", body.Slug, &projectID, &e.ID, map[string]any{
			"name": body.Name,
		})
	}
	WriteJSON(w, http.StatusCreated, map[string]any{"environment": e})
}

func (h *WorkspaceHandler) DeleteEnvironment(w http.ResponseWriter, r *http.Request) {
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
	projectID, err := h.Store.ProjectIDForEnvironment(ctx, envID)
	if err != nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "environment not found"})
		return
	}
	uid, err := h.requireProjectWrite(ctx, r, projectID)
	if err != nil {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	if err := h.Store.DeleteEnvironment(ctx, envID); err != nil {
		if errors.Is(err, store.ErrLastEnvironment) {
			WriteJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if h.RBAC != nil {
		_ = h.RBAC.Audit(ctx, &uid, "environment.delete", "environment", strconv.Itoa(envID), &projectID, &envID, nil)
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *WorkspaceHandler) ListConnectors(w http.ResponseWriter, r *http.Request) {
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
	connectors, err := h.Store.ListConnectors(ctx, envID)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list connectors"})
		return
	}
	stages := store.StageConnectors(connectors)
	WriteJSON(w, http.StatusOK, map[string]any{
		"connectors": connectors,
		"stages":     stages,
		"types":      store.ConnectorTypes,
	})
}

// ConnectorDefaults returns non-secret platform defaults (never tokens/keys).
func (h *WorkspaceHandler) ConnectorDefaults(w http.ResponseWriter, r *http.Request) {
	if h.Config == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "config not available"})
		return
	}
	defaults := h.Config.ConnectorDefaults()
	fields := map[string][]string{}
	for ctype, cfg := range defaults {
		keys := make([]string, 0, len(cfg))
		for k := range cfg {
			keys = append(keys, k)
		}
		fields[ctype] = keys
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"defaults": defaults,
		"note":     "Non-secret platform settings only. Secrets stay in Workspace → Secrets / env. Applying defaults does not invent Connected state until you enable with required fields.",
		"types":    store.ConnectorTypes,
		"fields":   fields,
	})
}

// ApplyConnectorDefaults fills connector config from platform defaults; optionally enables when ready.
func (h *WorkspaceHandler) ApplyConnectorDefaults(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil || h.Config == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	var body struct {
		Types  []string `json:"types"`
		Enable bool     `json:"enable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	uid, projectID, err := h.requireConnectorWrite(ctx, r, envID)
	if err != nil {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	defaults := h.Config.ConnectorDefaults()
	connectors, err := h.Store.ApplyPlatformDefaults(ctx, envID, defaults, body.Types, body.Enable)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to apply defaults"})
		return
	}
	stages := store.StageConnectors(connectors)
	ready := 0
	for _, ctype := range store.ConnectorTypes {
		if c, ok := stages[ctype]; ok && c.Ready {
			ready++
		}
	}
	total := len(store.ConnectorTypes)
	if h.RBAC != nil {
		_ = h.RBAC.Audit(ctx, &uid, "connector.apply_defaults", "environment", strconv.Itoa(envID), &projectID, &envID, map[string]any{
			"enable": body.Enable,
			"types":  body.Types,
			"ready":  ready,
		})
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"connectors": connectors,
		"stages":     stages,
		"ready":      ready,
		"total":      total,
		"enabled":    body.Enable,
		"message":    fmt.Sprintf("Applied platform defaults; %d/%d stages ready", ready, total),
	})
}

func (h *WorkspaceHandler) UpsertConnector(w http.ResponseWriter, r *http.Request) {
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
		ConnectorType string         `json:"connectorType"`
		Name          string         `json:"name"`
		Enabled       bool           `json:"enabled"`
		Config        map[string]any `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.ConnectorType = strings.TrimSpace(body.ConnectorType)
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		body.Name = body.ConnectorType
	}
	if body.ConnectorType == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "connectorType is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	uid, projectID, err := h.requireConnectorWrite(ctx, r, envID)
	if err != nil {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	c, err := h.Store.UpsertConnector(ctx, envID, body.ConnectorType, body.Name, body.Enabled, body.Config)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save connector"})
		return
	}
	if h.RBAC != nil {
		_ = h.RBAC.Audit(ctx, &uid, "connector.upsert", "connector", body.ConnectorType, &projectID, &envID, map[string]any{
			"enabled": body.Enabled,
			"name":    body.Name,
		})
	}
	WriteJSON(w, http.StatusOK, map[string]any{"connector": c})
}

func (h *WorkspaceHandler) ListSecrets(w http.ResponseWriter, r *http.Request) {
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
	// Never decrypt on list — SOC2 industry practice
	secrets, err := h.Store.ListSecrets(ctx, envID, func(string) string { return "••••••••" })
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list secrets"})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"secrets":           secrets,
		"providers":         store.SecretProviders,
		"encryption":        "AES-256-GCM",
		"plaintextReadable": false,
	})
}

func (h *WorkspaceHandler) UpsertSecret(w http.ResponseWriter, r *http.Request) {
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
		Provider string `json:"provider"`
		KeyName  string `json:"keyName"`
		Value    string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	body.KeyName = strings.TrimSpace(strings.ToUpper(body.KeyName))
	body.Provider = strings.TrimSpace(strings.ToLower(body.Provider))
	if body.KeyName == "" || body.Value == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "keyName and value are required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	uid, projectID, err := h.requireSecretWrite(ctx, r, envID)
	if err != nil {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	ciph, err := h.cipher()
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "encryption not configured"})
		return
	}
	ciphertext, err := ciph.Encrypt(body.Value)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt secret"})
		return
	}
	sec, err := h.Store.UpsertSecret(ctx, envID, body.Provider, body.KeyName, ciphertext)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save secret"})
		return
	}
	sec.MaskedValue = workspace.MaskSecret(body.Value)
	if h.RBAC != nil {
		_ = h.RBAC.Audit(ctx, &uid, "secret.write", "secret", body.KeyName, &projectID, &envID, map[string]any{
			"provider": body.Provider,
		})
	}
	WriteJSON(w, http.StatusOK, map[string]any{"secret": sec, "encryption": "AES-256-GCM"})
}

func (h *WorkspaceHandler) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
		return
	}
	envID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || envID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid environment id"})
		return
	}
	secretID, err := strconv.Atoi(r.PathValue("secretId"))
	if err != nil || secretID <= 0 {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid secret id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	uid, projectID, err := h.requireSecretWrite(ctx, r, envID)
	if err != nil {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	if err := h.Store.DeleteSecret(ctx, envID, secretID); err != nil {
		WriteJSON(w, http.StatusNotFound, map[string]string{"error": "secret not found"})
		return
	}
	if h.RBAC != nil {
		_ = h.RBAC.Audit(ctx, &uid, "secret.delete", "secret", strconv.Itoa(secretID), &projectID, &envID, nil)
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
