package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/PabloPavan/sniply_api/internal/auth"
	"github.com/PabloPavan/sniply_api/internal/identity"
	"github.com/PabloPavan/sniply_api/internal/session"
)

type Authenticator interface {
	AuthenticateAPIKey(ctx context.Context, token string, method string) (auth.Principal, error)
	AuthenticateSession(ctx context.Context, sessionID, csrfToken, method string) (auth.SessionInfo, bool, error)
}

type AuthOptions struct {
	AllowAPIKey  bool
	AllowSession bool
	Cookie       session.CookieConfig
}

func AuthMiddleware(authenticator Authenticator, opts AuthOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authenticator == nil {
				http.Error(w, "auth not configured", http.StatusInternalServerError)
				return
			}

			if opts.AllowAPIKey {
				if token := apiKeyFromRequest(r); token != "" {
					principal, err := authenticator.AuthenticateAPIKey(r.Context(), token, r.Method)
					if err != nil {
						writeAppError(w, err)
						return
					}

					ctx := identity.WithUser(r.Context(), principal.UserID, principal.Role)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			if !opts.AllowSession {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			name := opts.Cookie.Name
			if name == "" {
				name = "sniply_session"
			}

			sessionID := ""
			if reqCookie, err := r.Cookie(name); err == nil {
				sessionID = reqCookie.Value
			}

			csrfToken := r.Header.Get("X-CSRF-Token")
			sess, refreshed, err := authenticator.AuthenticateSession(r.Context(), sessionID, csrfToken, r.Method)
			if err != nil {
				writeAppError(w, err)
				return
			}

			if refreshed {
				opts.Cookie.Write(w, sess.ID, sess.ExpiresAt)
			}

			ctx := identity.WithUser(r.Context(), sess.UserID, sess.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func apiKeyFromRequest(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-API-Key")); v != "" {
		return v
	}

	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return ""
	}
	parts := strings.Fields(auth)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
