package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/PabloPavan/sniply_api/internal/apikeys"
	"github.com/PabloPavan/sniply_api/internal/apperrors"
	"github.com/PabloPavan/sniply_api/internal/session"
	"github.com/PabloPavan/sniply_api/internal/users"
	"golang.org/x/crypto/bcrypt"
)

type UserStore interface {
	GetByEmail(ctx context.Context, email string) (users.User, error)
}

type SessionManager interface {
	Create(ctx context.Context, userID, role string) (*session.Session, error)
	Get(ctx context.Context, id string) (*session.Session, error)
	Refresh(ctx context.Context, sess *session.Session) (*session.Session, bool, error)
	Delete(ctx context.Context, id string) error
}

type APIKeyStore interface {
	GetByTokenHash(ctx context.Context, hash string) (*apikeys.Key, error)
}

type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, time.Duration, error)
}

type Service struct {
	Users            UserStore
	Sessions         SessionManager
	APIKeys          APIKeyStore
	LoginLimiter     RateLimiter
	PasswordVerifier func(hashed, plain string) error
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	ClientIP string `json:"-"`
}

type SessionInfo struct {
	ID        string
	UserID    string
	Role      string
	CSRFToken string
	ExpiresAt time.Time
}

type LoginResult struct {
	UserID    string
	UserEmail string
	UserRole  string
	Session   SessionInfo
}

type Principal struct {
	UserID string
	Role   string
}

func (s *Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	if s.Users == nil || s.Sessions == nil {
		return LoginResult{}, apperrors.New(apperrors.KindInternal, "auth not configured")
	}

	email := strings.TrimSpace(strings.ToLower(input.Email))
	password := strings.TrimSpace(input.Password)
	if email == "" || password == "" {
		return LoginResult{}, apperrors.New(apperrors.KindInvalidInput, "email and password are required")
	}
	if !strings.Contains(email, "@") {
		return LoginResult{}, apperrors.New(apperrors.KindInvalidInput, "invalid email")
	}

	if s.LoginLimiter != nil {
		if strings.TrimSpace(input.ClientIP) != "" {
			allowed, retryAfter, err := s.LoginLimiter.Allow(ctx, "login:ip:"+input.ClientIP)
			if err != nil {
				return LoginResult{}, apperrors.New(apperrors.KindInternal, "rate limit error")
			}
			if !allowed {
				return LoginResult{}, apperrors.RateLimit("too many requests", retryAfter)
			}
		}

		allowed, retryAfter, err := s.LoginLimiter.Allow(ctx, "login:email:"+email)
		if err != nil {
			return LoginResult{}, apperrors.New(apperrors.KindInternal, "rate limit error")
		}
		if !allowed {
			return LoginResult{}, apperrors.RateLimit("too many requests", retryAfter)
		}
	}

	u, err := s.Users.GetByEmail(ctx, email)
	if err != nil {
		return LoginResult{}, apperrors.New(apperrors.KindUnauthorized, "invalid credentials")
	}

	verifier := s.PasswordVerifier
	if verifier == nil {
		verifier = func(hashed, plain string) error {
			return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain))
		}
	}

	if err := verifier(u.PasswordHash, password); err != nil {
		return LoginResult{}, apperrors.New(apperrors.KindUnauthorized, "invalid credentials")
	}

	sess, err := s.Sessions.Create(ctx, u.ID, string(u.Role))
	if err != nil {
		return LoginResult{}, apperrors.New(apperrors.KindInternal, "failed to create session")
	}

	return LoginResult{
		UserID:    u.ID,
		UserEmail: u.Email,
		UserRole:  string(u.Role),
		Session: SessionInfo{
			ID:        sess.ID,
			UserID:    sess.UserID,
			Role:      sess.Role,
			CSRFToken: sess.CSRFToken,
			ExpiresAt: sess.ExpiresAt,
		},
	}, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if s.Sessions == nil {
		return apperrors.New(apperrors.KindInternal, "auth not configured")
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	if err := s.Sessions.Delete(ctx, sessionID); err != nil {
		return apperrors.New(apperrors.KindInternal, "failed to logout")
	}
	return nil
}

func (s *Service) AuthenticateAPIKey(ctx context.Context, token string, method string) (Principal, error) {
	if strings.TrimSpace(token) == "" {
		return Principal{}, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}
	if s.APIKeys == nil {
		return Principal{}, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}

	key, err := s.APIKeys.GetByTokenHash(ctx, apikeys.HashToken(token))
	if err != nil {
		if apikeys.IsNotFound(err) {
			return Principal{}, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
		}
		return Principal{}, apperrors.New(apperrors.KindInternal, "failed to authenticate")
	}
	if key.RevokedAt != nil {
		return Principal{}, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}

	if !key.Scope.AllowsMethod(method) {
		return Principal{}, apperrors.New(apperrors.KindForbidden, "forbidden")
	}

	return Principal{UserID: key.UserID, Role: key.UserRole}, nil
}

func (s *Service) AuthenticateSession(ctx context.Context, sessionID, csrfToken, method string) (SessionInfo, bool, error) {
	if s.Sessions == nil {
		return SessionInfo{}, false, apperrors.New(apperrors.KindInternal, "auth not configured")
	}
	if strings.TrimSpace(sessionID) == "" {
		return SessionInfo{}, false, apperrors.New(apperrors.KindUnauthorized, "missing session")
	}

	sess, err := s.Sessions.Get(ctx, sessionID)
	if err != nil {
		return SessionInfo{}, false, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
	}

	if requiresCSRFToken(method) {
		if csrfToken == "" || csrfToken != sess.CSRFToken {
			return SessionInfo{}, false, apperrors.New(apperrors.KindForbidden, "forbidden")
		}
	}

	refreshed := false
	sess, refreshed, err = s.Sessions.Refresh(ctx, sess)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return SessionInfo{}, false, apperrors.New(apperrors.KindUnauthorized, "unauthorized")
		}
		return SessionInfo{}, false, apperrors.New(apperrors.KindInternal, "failed to refresh session")
	}

	info := SessionInfo{
		ID:        sess.ID,
		UserID:    sess.UserID,
		Role:      sess.Role,
		CSRFToken: sess.CSRFToken,
		ExpiresAt: sess.ExpiresAt,
	}
	return info, refreshed, nil
}

func requiresCSRFToken(method string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return false
	default:
		return true
	}
}
