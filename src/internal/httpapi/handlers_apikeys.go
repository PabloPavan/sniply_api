package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/PabloPavan/sniply_api/internal"
	"github.com/PabloPavan/sniply_api/internal/apikeys"
	"github.com/PabloPavan/sniply_api/internal/identity"
	"github.com/go-chi/chi/v5"
)

type APIKeysRepo interface {
	Create(ctx context.Context, k *apikeys.Key) error
	ListByUser(ctx context.Context, userID string) ([]*apikeys.Key, error)
	Revoke(ctx context.Context, id, userID string) (bool, error)
}

type APIKeysHandler struct {
	Repo APIKeysRepo
}

type APIKeyCreateRequest struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
}

type APIKeyCreateResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Scope       string    `json:"scope"`
	Token       string    `json:"token"`
	TokenPrefix string    `json:"token_prefix"`
	CreatedAt   time.Time `json:"created_at"`
}

type APIKeyResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Scope       string     `json:"scope"`
	TokenPrefix string     `json:"token_prefix"`
	CreatedAt   time.Time  `json:"created_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

// Create API Key
// @Summary Create API key
// @Tags auth
// @Accept json
// @Produce json
// @Security SessionAuth
// @Param body body APIKeyCreateRequest true "api key"
// @Param X-CSRF-Token header string false "CSRF token (required for SessionAuth)"
// @Success 201 {object} APIKeyCreateResponse
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 500 {string} string
// @Router /auth/api-keys [post]
func (h *APIKeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req APIKeyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	scope := apikeys.Scope(strings.TrimSpace(req.Scope))
	if scope == "" {
		scope = apikeys.ScopeReadWrite
	}
	if !scope.Valid() {
		http.Error(w, "invalid scope", http.StatusBadRequest)
		return
	}

	token := apikeys.GenerateToken()
	key := &apikeys.Key{
		ID:          "key_" + internal.RandomHex(12),
		UserID:      userID,
		Name:        req.Name,
		Scope:       scope,
		TokenHash:   apikeys.HashToken(token),
		TokenPrefix: apikeys.TokenPrefix(token),
	}

	if err := h.Repo.Create(r.Context(), key); err != nil {
		http.Error(w, "failed to create api key", http.StatusInternalServerError)
		return
	}

	resp := APIKeyCreateResponse{
		ID:          key.ID,
		Name:        key.Name,
		Scope:       string(key.Scope),
		Token:       token,
		TokenPrefix: key.TokenPrefix,
		CreatedAt:   key.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// List API Keys
// @Summary List API keys
// @Tags auth
// @Produce json
// @Security SessionAuth
// @Success 200 {array} APIKeyResponse
// @Failure 401 {string} string
// @Failure 500 {string} string
// @Router /auth/api-keys [get]
func (h *APIKeysHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	keys, err := h.Repo.ListByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "failed to list api keys", http.StatusInternalServerError)
		return
	}

	resp := make([]APIKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, APIKeyResponse{
			ID:          k.ID,
			Name:        k.Name,
			Scope:       string(k.Scope),
			TokenPrefix: k.TokenPrefix,
			CreatedAt:   k.CreatedAt,
			RevokedAt:   k.RevokedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Revoke API Key
// @Summary Revoke API key
// @Tags auth
// @Produce json
// @Security SessionAuth
// @Param id path string true "api key id"
// @Param X-CSRF-Token header string false "CSRF token (required for SessionAuth)"
// @Success 204
// @Failure 401 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /auth/api-keys/{id} [delete]
func (h *APIKeysHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	ok, err := h.Repo.Revoke(r.Context(), id, userID)
	if err != nil {
		http.Error(w, "failed to revoke api key", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "api key not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
