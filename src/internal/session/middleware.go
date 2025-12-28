package session

import (
	"net/http"

	"github.com/PabloPavan/Sniply/internal/identity"
)

func Middleware(mgr *Manager, cookieName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := cookieName
			if name == "" {
				name = "sniply_session"
			}

			cookie, err := r.Cookie(name)
			if err != nil || cookie.Value == "" {
				http.Error(w, "missing session", http.StatusUnauthorized)
				return
			}

			sess, err := mgr.Get(r.Context(), cookie.Value)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := identity.WithUser(r.Context(), sess.UserID, sess.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
