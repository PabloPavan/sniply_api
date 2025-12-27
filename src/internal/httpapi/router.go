package httpapi

import (
	"net/http"

	_ "github.com/PabloPavan/Sniply/docs"
	"github.com/PabloPavan/Sniply/internal/auth"
	"github.com/PabloPavan/Sniply/internal/telemetry"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type App struct {
	Health   *HealthHandler
	Snippets *SnippetsHandler
	Users    *UsersHandler
	Auth     *AuthHandler
}

func NewRouter(app *App) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(telemetry.ChiTraceMiddleware("sniply-api"))
	r.Use(telemetry.ChiLogMiddleware("sniply-api"))
	r.Use(telemetry.ChiMetricsMiddleware)

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	r.Route("/v1", func(r chi.Router) {
		// Health endpoint
		r.Get("/health", app.Health.Get)

		// Auth endpoints
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", app.Auth.Login)
		})

		r.Route("/snippets", func(r chi.Router) {
			// Protected
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(app.Auth.Auth))
				r.Post("/", app.Snippets.Create)
				r.Get("/", app.Snippets.List)
				r.Get("/{id}", app.Snippets.GetByID)
				r.Put("/{id}", app.Snippets.Update)
				r.Delete("/{id}", app.Snippets.Delete)
			})
		})

		r.Route("/users", func(r chi.Router) {
			// Public
			r.Post("/", app.Users.Create)

			// Protected
			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(app.Auth.Auth))

				// Self endpoints
				r.Get("/me", app.Users.Me)
				r.Put("/me", app.Users.UpdateMe)
				r.Delete("/me", app.Users.DeleteMe)

				// Admin endpoints
				r.Get("/", app.Users.List)
				r.Put("/{id}", app.Users.Update)
				r.Delete("/{id}", app.Users.Delete)
			})
		})

	})
	return r
}
