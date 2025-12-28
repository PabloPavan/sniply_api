package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/PabloPavan/sniply_api/internal"
	"github.com/PabloPavan/sniply_api/internal/identity"
	"github.com/PabloPavan/sniply_api/internal/telemetry"
	"github.com/PabloPavan/sniply_api/internal/users"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
)

type UsersRepo interface {
	Create(ctx context.Context, u *users.User) error
	GetByID(ctx context.Context, id string) (*users.User, error)
	List(ctx context.Context, f users.UserFilter) ([]*users.User, error)
	Update(ctx context.Context, u *users.UpdateUserRequest) error
	Delete(ctx context.Context, id string) error
}
type UsersHandler struct {
	Repo           UsersRepo
	PasswordHasher func(plain string) (string, error)
}

type UserUpdateRequest struct {
	Email    string  `json:"email,omitempty"`
	Password string  `json:"password,omitempty"`
	Role     *string `json:"role,omitempty"`
}

// Create User
// @Summary Create user
// @Tags users
// @Accept json
// @Produce json
// @Param body body users.CreateUserRequest true "user"
// @Success 201 {object} users.UserResponse
// @Failure 400 {string} string
// @Failure 409 {string} string
// @Failure 500 {string} string
// @Router /users [post]
func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req users.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Password = strings.TrimSpace(req.Password)

	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}
	if !strings.Contains(req.Email, "@") {
		http.Error(w, "invalid email", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	hasher := h.PasswordHasher
	if hasher == nil {
		hasher = internal.DefaultPasswordHasher
	}

	_, span := telemetry.StartSpan(ctx, "users.hash_password",
		attribute.String("user.email", req.Email),
	)
	hash, err := hasher(req.Password)
	span.End()
	if err != nil {
		http.Error(w, "failed to process password", http.StatusInternalServerError)
		return
	}

	u := &users.User{
		ID:           "usr_" + internal.RandomHex(12),
		Email:        req.Email,
		PasswordHash: hash,
	}

	createCtx, span := telemetry.StartSpan(ctx, "users.create",
		attribute.String("user.id", u.ID),
		attribute.String("user.email", u.Email),
	)
	err = h.Repo.Create(createCtx, u)
	span.End()
	if err != nil {
		if users.IsUniqueViolationEmail(err) {
			http.Error(w, "email already exists", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create user", http.StatusInternalServerError)
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
// @Param q query string false "search"
// @Param limit query int false "limit"
// @Param offset query int false "offset"
// @Success 200 {array} users.UserResponse
// @Failure 401 {string} string
// @Failure 403 {string} string
// @Failure 500 {string} string
// @Router /users [get]
func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	_, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if !identity.IsAdmin(r.Context()) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))

	limit := 100
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

	list, err := h.Repo.List(r.Context(), f)
	if err != nil {
		http.Error(w, "failed to list users", http.StatusInternalServerError)
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
// @Success 200 {object} users.UserResponse
// @Failure 401 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /users/me [get]
func (h *UsersHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	u, err := h.Repo.GetByID(r.Context(), userID)
	if err != nil {
		if users.IsNotFound(err) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load user", http.StatusInternalServerError)
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
// @Param body body UserUpdateRequest true "user"
// @Success 204
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 403 {string} string
// @Failure 500 {string} string
// @Router /users/me [put]
func (h *UsersHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	h.updateUserByID(w, r, userID)
}

// DeleteMe User
// @Summary Delete current user
// @Tags users
// @Security SessionAuth
// @Success 204
// @Failure 401 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /users/me [delete]
func (h *UsersHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	h.deleteUserByID(w, r, userID)
}

// Update User
// @Summary Update user (admin or self)
// @Tags users
// @Accept json
// @Security SessionAuth
// @Param id path string true "user id"
// @Param body body UserUpdateRequest true "user"
// @Success 204
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 403 {string} string
// @Failure 500 {string} string
// @Router /users/{id} [put]
func (h *UsersHandler) Update(w http.ResponseWriter, r *http.Request) {
	targetID := strings.TrimSpace(chi.URLParam(r, "id"))
	if targetID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	requesterID, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if !h.isAllowedToMutateUser(r.Context(), requesterID, targetID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	h.updateUserByID(w, r, targetID)
}

// Delete User
// @Summary Delete user (admin or self)
// @Tags users
// @Security SessionAuth
// @Param id path string true "user id"
// @Success 204
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 403 {string} string
// @Failure 404 {string} string
// @Failure 500 {string} string
// @Router /users/{id} [delete]
func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	targetID := strings.TrimSpace(chi.URLParam(r, "id"))
	if targetID == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	requesterID, ok := identity.UserID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if !h.isAllowedToMutateUser(r.Context(), requesterID, targetID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	h.deleteUserByID(w, r, targetID)
}

func (h *UsersHandler) isAllowedToMutateUser(ctx context.Context, requesterID, targetID string) bool {
	if requesterID == targetID {
		return true
	}

	return identity.IsAdmin(ctx)
}

func (h *UsersHandler) updateUserByID(w http.ResponseWriter, r *http.Request, targetID string) {
	ctx := r.Context()

	isAdmin := identity.IsAdmin(ctx)

	var raw UserUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if raw.Role != nil && !isAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	hasher := h.PasswordHasher
	if hasher == nil {
		hasher = internal.DefaultPasswordHasher
	}

	hash, err := hasher(raw.Password)
	if err != nil {
		http.Error(w, "failed to process password", http.StatusInternalServerError)
		return
	}

	req := users.UpdateUserRequest{
		ID:           targetID,
		Email:        raw.Email,
		PasswordHash: hash,
	}

	if raw.Role != nil {
		role, err := users.ParseUserRole(*raw.Role)
		if err != nil {
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
		req.Role = role
	}

	if err := h.Repo.Update(ctx, &req); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UsersHandler) deleteUserByID(w http.ResponseWriter, r *http.Request, targetID string) {
	if err := h.Repo.Delete(r.Context(), targetID); err != nil {
		if users.IsNotFound(err) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to delete user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
