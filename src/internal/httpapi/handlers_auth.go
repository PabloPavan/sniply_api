package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/PabloPavan/sniply_api/internal/auth"
	"github.com/PabloPavan/sniply_api/internal/session"
	"github.com/PabloPavan/sniply_api/internal/telemetry"
)

type AuthService interface {
	Login(ctx context.Context, input auth.LoginInput) (auth.LoginResult, error)
	Logout(ctx context.Context, sessionID string) error
}

type AuthHandler struct {
	Service AuthService
	Cookie  session.CookieConfig
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	SessionExpiresAt string `json:"session_expires_at"` // RFC3339
	CSRFToken        string `json:"csrf_token"`
}

// Login Auth
// @Summary Login
// @Tags auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "credentials"
// @Success 200 {object} LoginResponse
// @Failure 400 {string} string
// @Failure 401 {string} string
// @Failure 500 {string} string
// @Router /auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "auth not configured", http.StatusInternalServerError)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	result, err := h.Service.Login(ctx, auth.LoginInput{
		Email:    req.Email,
		Password: req.Password,
		ClientIP: clientIP(r),
	})
	if err != nil {
		writeAppError(w, err)
		return
	}

	h.Cookie.Write(w, result.Session.ID, result.Session.ExpiresAt)

	resp := LoginResponse{
		SessionExpiresAt: result.Session.ExpiresAt.UTC().Format(time.RFC3339),
		CSRFToken:        result.Session.CSRFToken,
	}

	telemetry.LogInfo(r.Context(), "user login",
		telemetry.LogString("event", "user.login"),
		telemetry.LogString("user.id", result.UserID),
		telemetry.LogString("user.email", result.UserEmail),
	)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// Logout Auth
// @Summary Logout
// @Tags auth
// @Produce json
// @Success 204
// @Failure 500 {string} string
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if h.Service == nil {
		http.Error(w, "auth not configured", http.StatusInternalServerError)
		return
	}

	name := h.Cookie.Name
	if name == "" {
		name = "sniply_session"
	}

	cookie, err := r.Cookie(name)
	if err == nil && cookie.Value != "" {
		if err := h.Service.Logout(r.Context(), cookie.Value); err != nil {
			writeAppError(w, err)
			return
		}
	}

	h.Cookie.Clear(w)
	w.WriteHeader(http.StatusNoContent)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
