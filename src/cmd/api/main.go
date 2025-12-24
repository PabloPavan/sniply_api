package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/PabloPavan/Sniply/internal"
	"github.com/PabloPavan/Sniply/internal/db"
	"github.com/PabloPavan/Sniply/internal/httpapi"
	"github.com/PabloPavan/Sniply/internal/snippets"
	"github.com/PabloPavan/Sniply/internal/users"
)

func main() {
	port := internal.Env("APP_PORT", "8080")
	databaseURL := internal.MustEnv("DATABASE_URL")

	ctx := context.Background()

	d, err := db.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	defer d.Close()

	dbBase := db.NewBase(d.Pool, 3*time.Second)
	snRepo := snippets.NewRepository(dbBase)
	usrRepo := users.NewRepository(dbBase)

	app := &httpapi.App{
		Health:   &httpapi.HealthHandler{DB: d.Pool},
		Snippets: &httpapi.SnippetsHandler{Repo: snRepo},
		Users:    &httpapi.UsersHandler{Repo: usrRepo},
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewRouter(app),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("api listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
