package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/PabloPavan/sniply_api/internal/telemetry"
	"github.com/PabloPavan/sniply_api/internal/users"
	"github.com/go-chi/chi/v5"
)

type UsersService interface {
	Create(ctx context.Context, req users.CreateUserRequest) (*users.User, error)
	Me(ctx context.Context) (*users.User, error)
	List(ctx context.Context, f users.UserFilter) ([]*users.User, error)
	UpdateSelf(ctx context.Context, input users.UpdateUserInput) error
	UpdateByID(ctx context.Context, targetID string, input users.UpdateUserInput) error
	DeleteSelf(ctx context.Context) error
	DeleteByID(ctx context.Context, targetID string) error
}

type UsersHandler struct {
	Service UsersService
}

// Create User
// @Summary Create user
// @Tags users
// @Accept json
// @Produce json
// @Param body body UserCreateDTO true "user"
// @Success 201 {object} users.UserResponse
// @Failure 400 {string} string
// @Failure 409 {string} string
// @Failure 500 {string} string
// @Router /users [post]
func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req UserCreateDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	u, err := h.Service.Create(r.Context(), users.CreateUserRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeAppError(w, err)
		return
	}

	resp := users.UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}

	telemetry.LogInfo(r.Context(), "user created",
		telemetry.LogString("event", "user.created"),
		telemetry.LogString("user.id", u.ID),
		telemetry.LogString("user.email", u.Email),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// List Users
// @Summary List users (admin)
// @Tags users
// @Produce json
// @Security SessionAuth
// @Security ApiKeyAuth
// @Param q query string false "search"
// @Param limit query int false "limit"
// @Param offset query int false "offset"
// @Success 200 {array} users.UserResponse
// @Failure 401 {string} string
// @Failure 403 {string} string
// @Failure 500 {string} string
// @Router /users [get]
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	limit := 0
	offset := 0

	if l := strings.TrimSpace(r.URL.Query().Get("limit")); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := strings.TrimSpace(r.URL.Query().Get("offset")); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	f := users.UserFilter{
		Query:  q,
		Limit:  limit,
		Offset: offset,
	}

	list, err := h.Service.List(r.Context(), f)
	if err != nil {
		writeAppError(w, err)
		return
	}

	resp := make([]users.UserResponse, 0, len(list))
	for _, u := range list {
		resp = append(resp, users.UserResponse{
			ID:        u.ID,
			Email:     u.Email,
			CreatedAt: u.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Me User
// @Summary Get current user
// @Tags users
// @Produce json
// @Security SessionAuth
// @Security ApiKeyAuth
// @Success 200 {object} users.UserResponse
// @Failure 401 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /users/me [get]
func (h *UsersHandler) Me(w http.ResponseWriter, r *http.Request) {
	u, err := h.Service.Me(r.Context())
	if err != nil {
		writeAppError(w, err)
		return
	}

	resp := users.UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// UpdateMe User
// @Summary Update current user
// @Tags users
// @Accept json
// @Security SessionAuth
// @Security ApiKeyAuth
// @Param body body UserUpdateDTO true "user"
// @Param X-CSRF-Token header string false "CSRF token (required for SessionAuth)"
// @Success 204
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 403 {string} string
// @Failure 500 {string} string
// @Router /users/me [put]
func (h *UsersHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var req UserUpdateDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.Service.UpdateSelf(r.Context(), users.UpdateUserInput{
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	}); err != nil {
		writeAppError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteMe User
// @Summary Delete current user
// @Tags users
// @Security SessionAuth
// @Security ApiKeyAuth
// @Param X-CSRF-Token header string false "CSRF token (required for SessionAuth)"
// @Success 204
// @Failure 401 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /users/me [delete]
func (h *UsersHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	if err := h.Service.DeleteSelf(r.Context()); err != nil {
		writeAppError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Update User
// @Summary Update user (admin or self)
// @Tags users
// @Accept json
// @Security SessionAuth
// @Security ApiKeyAuth
// @Param id path string true "user id"
// @Param body body UserUpdateDTO true "user"
// @Param X-CSRF-Token header string false "CSRF token (required for SessionAuth)"
// @Success 204
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 403 {string} string
// @Failure 500 {string} string
// @Router /users/{id} [put]
func (h *UsersHandler) Update(w http.ResponseWriter, r *http.Request) {
	targetID := strings.TrimSpace(chi.URLParam(r, "id"))

	var req UserUpdateDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.Service.UpdateByID(r.Context(), targetID, users.UpdateUserInput{
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	}); err != nil {
		writeAppError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Delete User
// @Summary Delete user (admin or self)
// @Tags users
// @Security SessionAuth
// @Security ApiKeyAuth
// @Param id path string true "user id"
// @Param X-CSRF-Token header string false "CSRF token (required for SessionAuth)"
// @Success 204
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 403 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /users/{id} [delete]
func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	targetID := strings.TrimSpace(chi.URLParam(r, "id"))

	if err := h.Service.DeleteByID(r.Context(), targetID); err != nil {
		writeAppError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
