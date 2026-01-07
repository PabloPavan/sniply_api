package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/PabloPavan/sniply_api/internal/apikeys"
	"github.com/go-chi/chi/v5"
)

type APIKeysService interface {
	Create(ctx context.Context, input apikeys.CreateInput) (*apikeys.Key, string, error)
	List(ctx context.Context) ([]*apikeys.Key, error)
	Revoke(ctx context.Context, id string) error
}

type APIKeysHandler struct {
	Service APIKeysService
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
// @Param body body apikeys.CreateInput true "api key"
// @Param X-CSRF-Token header string false "CSRF token (required for SessionAuth)"
// @Success 201 {object} APIKeyCreateResponse
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 500 {string} string
// @Router /auth/api-keys [post]
func (h *APIKeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req apikeys.CreateInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	key, token, err := h.Service.Create(r.Context(), req)
	if err != nil {
		writeAppError(w, err)
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
	keys, err := h.Service.List(r.Context())
	if err != nil {
		writeAppError(w, err)
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
	id := strings.TrimSpace(chi.URLParam(r, "id"))

	if err := h.Service.Revoke(r.Context(), id); err != nil {
		writeAppError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
