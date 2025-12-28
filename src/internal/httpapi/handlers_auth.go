package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PabloPavan/sniply_api/internal/ratelimit"
	"github.com/PabloPavan/sniply_api/internal/session"
	"github.com/PabloPavan/sniply_api/internal/telemetry"
	"github.com/PabloPavan/sniply_api/internal/users"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/crypto/bcrypt"
)

type AuthUsersRepo interface {
	GetByEmail(ctx context.Context, email string) (users.User, error)
}

type AuthHandler struct {
	Users        AuthUsersRepo
	Sessions     *session.Manager
	Cookie       session.CookieConfig
	LoginLimiter *ratelimit.Limiter
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	SessionExpiresAt string `json:"session_expires_at"` // RFC3339
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
	if h.Users == nil || h.Sessions == nil {
		http.Error(w, "auth not configured", http.StatusInternalServerError)
		return
	}

	var req LoginRequest
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

	if h.LoginLimiter != nil {
		ip := clientIP(r)
		if ip != "" {
			allowed, retryAfter, err := h.LoginLimiter.Allow(ctx, "login:ip:"+ip)
			if err != nil {
				http.Error(w, "rate limit error", http.StatusInternalServerError)
				return
			}
			if !allowed {
				writeRateLimit(w, retryAfter)
				return
			}
		}

		allowed, retryAfter, err := h.LoginLimiter.Allow(ctx, "login:email:"+req.Email)
		if err != nil {
			http.Error(w, "rate limit error", http.StatusInternalServerError)
			return
		}
		if !allowed {
			writeRateLimit(w, retryAfter)
			return
		}
	}

	u, err := h.Users.GetByEmail(ctx, req.Email)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	_, span := telemetry.StartSpan(ctx, "auth.verify_password",
		attribute.String("user.id", u.ID),
		attribute.String("user.email", u.Email),
	)
	err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password))
	span.End()
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	_, span = telemetry.StartSpan(ctx, "auth.create_session",
		attribute.String("user.id", u.ID),
		attribute.String("user.role", string(u.Role)),
	)
	sess, err := h.Sessions.Create(ctx, u.ID, string(u.Role))
	span.End()
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	h.Cookie.Write(w, sess.ID, sess.ExpiresAt)

	resp := LoginResponse{
		SessionExpiresAt: sess.ExpiresAt.UTC().Format(time.RFC3339),
	}

	telemetry.LogInfo(r.Context(), "user login",
		telemetry.LogString("event", "user.login"),
		telemetry.LogString("user.id", u.ID),
		telemetry.LogString("user.email", u.Email),
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
	if h.Sessions == nil {
		http.Error(w, "auth not configured", http.StatusInternalServerError)
		return
	}

	name := h.Cookie.Name
	if name == "" {
		name = "sniply_session"
	}

	cookie, err := r.Cookie(name)
	if err == nil && cookie.Value != "" {
		_ = h.Sessions.Delete(r.Context(), cookie.Value)
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

func writeRateLimit(w http.ResponseWriter, retryAfter time.Duration) {
	if retryAfter > 0 {
		seconds := int(retryAfter.Seconds())
		if seconds <= 0 {
			seconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
	}
	http.Error(w, "too many requests", http.StatusTooManyRequests)
}
