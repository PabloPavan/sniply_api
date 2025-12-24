package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type App struct {
	Health   *HealthHandler
	Snippets *SnippetsHandler
	Users    *UsersHandler
}

func NewRouter(app *App) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/health", app.Health.Get)

	r.Route("/v1", func(r chi.Router) {

		r.Route("/snippets", func(r chi.Router) {
			r.Get("/", app.Snippets.List)
			r.Post("/", app.Snippets.Create)
			r.Get("/{id}", app.Snippets.GetByID)
			r.Put("/{id}", app.Snippets.Update)
			r.Delete("/{id}", app.Snippets.Delete)
		})

		r.Route("/users", func(r chi.Router) {
			r.Post("/", app.Users.Create)
			r.Get("/", app.Users.List)
			r.Put("/{id}", app.Users.Update)
			r.Delete("/{id}", app.Users.Delete)
		})
	})
	return r
}
