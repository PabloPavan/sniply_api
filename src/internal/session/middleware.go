package session

import (
	"errors"
	"net/http"

	"github.com/PabloPavan/sniply_api/internal/identity"
)

func Middleware(mgr *Manager, cookieCfg CookieConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := cookieCfg.Name
			if name == "" {
				name = "sniply_session"
			}

			reqCookie, err := r.Cookie(name)
			if err != nil || reqCookie.Value == "" {
				http.Error(w, "missing session", http.StatusUnauthorized)
				return
			}

			sess, err := mgr.Get(r.Context(), reqCookie.Value)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			var refreshed bool
			sess, refreshed, err = mgr.Refresh(r.Context(), sess)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				http.Error(w, "failed to refresh session", http.StatusInternalServerError)
				return
			}

			if refreshed && sess != nil {
				cookieCfg.Write(w, sess.ID, sess.ExpiresAt)
			}

			ctx := identity.WithUser(r.Context(), sess.UserID, sess.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
