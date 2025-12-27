package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/PabloPavan/Sniply/internal"
	"github.com/PabloPavan/Sniply/internal/auth"
	"github.com/PabloPavan/Sniply/internal/db"
	"github.com/PabloPavan/Sniply/internal/httpapi"
	"github.com/PabloPavan/Sniply/internal/snippets"
	"github.com/PabloPavan/Sniply/internal/telemetry"
	"github.com/PabloPavan/Sniply/internal/users"
)

func main() {
	port := internal.Env("APP_PORT", "8080")
	databaseURL := internal.MustEnv("DATABASE_URL")
	jwtSecret := internal.MustEnv("JWT_SECRET")

	ctx := context.Background()

	shutdown := telemetry.InitTracer("sniply-api")
	defer shutdown(context.Background())
	shutdownMetrics := telemetry.InitMetrics("sniply-api")
	defer shutdownMetrics(context.Background())
	shutdownLogger := telemetry.InitLogger("sniply-api")
	defer shutdownLogger(context.Background())
	db.InitTelemetry("sniply-api")

	d, err := db.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("db connect error: %v", err)
	}
	defer d.Close()

	dbBase := db.NewBase(d.Pool, 3*time.Second)
	snRepo := snippets.NewRepository(dbBase)
	usrRepo := users.NewRepository(dbBase)

	authSvc := &auth.Service{
		Secret:    []byte(jwtSecret),
		Issuer:    "sniply-api",
		Audience:  "sniply-client",
		AccessTTL: 20 * time.Hour,
	}

	app := &httpapi.App{
		Health:   &httpapi.HealthHandler{DB: d.Pool},
		Snippets: &httpapi.SnippetsHandler{Repo: snRepo, RepoUser: usrRepo},
		Users:    &httpapi.UsersHandler{Repo: usrRepo},
		Auth:     &httpapi.AuthHandler{Users: usrRepo, Auth: authSvc},
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
