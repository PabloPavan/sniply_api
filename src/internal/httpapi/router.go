package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type App struct {
	Health   *HealthHandler
	Snippets *SnippetsHandler
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
			r.Post("/", app.Snippets.Create)
			r.Get("/{id}", app.Snippets.GetByID)
		})
	})

	return r
}
