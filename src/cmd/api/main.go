package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/PabloPavan/sniply_api/internal"
	"github.com/PabloPavan/sniply_api/internal/apikeys"
	"github.com/PabloPavan/sniply_api/internal/auth"
	"github.com/PabloPavan/sniply_api/internal/db"
	"github.com/PabloPavan/sniply_api/internal/httpapi"
	"github.com/PabloPavan/sniply_api/internal/ratelimit"
	"github.com/PabloPavan/sniply_api/internal/session"
	"github.com/PabloPavan/sniply_api/internal/snippets"
	"github.com/PabloPavan/sniply_api/internal/telemetry"
	"github.com/PabloPavan/sniply_api/internal/users"
	"github.com/redis/go-redis/v9"
)

func main() {
	port := internal.Env("APP_PORT", "8080")
	databaseURL := internal.MustEnv("DATABASE_URL")
	redisURL := internal.MustEnv("REDIS_URL")

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

	redisOpt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("redis url error: %v", err)
	}
	redisClient := redis.NewClient(redisOpt)
	defer redisClient.Close()

	dbBase := db.NewBase(d.Pool, 3*time.Second)
	snRepo := snippets.NewRepository(dbBase)
	usrRepo := users.NewRepository(dbBase)
	apiKeysRepo := apikeys.NewRepository(dbBase)

	sessionPrefix := internal.Env("SESSION_REDIS_PREFIX", "sniply:session:")
	sessionTTL := internal.ParseDurationEnv("SESSION_TTL", 7*24*time.Hour)
	sessionMaxAge := internal.ParseDurationEnv("SESSION_MAX_AGE", 8*time.Hour)
	sessionRefreshBefore := internal.ParseDurationEnv("SESSION_REFRESH_BEFORE", 10*time.Minute)
	sessionManager := &session.Manager{
		Store:         session.NewRedisStore(redisClient, sessionPrefix),
		TTL:           sessionTTL,
		MaxAge:        sessionMaxAge,
		RefreshBefore: sessionRefreshBefore,
		IDBytes:       32,
	}

	cookieSecure := internal.ParseBoolEnv("SESSION_COOKIE_SECURE", true)
	cookieSameSite := internal.ParseSameSiteEnv("SESSION_COOKIE_SAMESITE", http.SameSiteLaxMode)
	cookie := session.CookieConfig{
		Name:     internal.Env("SESSION_COOKIE_NAME", "sniply_session"),
		Path:     internal.Env("SESSION_COOKIE_PATH", "/"),
		Domain:   internal.Env("SESSION_COOKIE_DOMAIN", ""),
		Secure:   cookieSecure,
		SameSite: cookieSameSite,
	}
	csrfCookie := session.CSRFCookieConfig{
		Name:     internal.Env("SESSION_CSRF_COOKIE_NAME", "sniply_csrf"),
		Path:     cookie.Path,
		Domain:   cookie.Domain,
		Secure:   cookie.Secure,
		SameSite: cookie.SameSite,
	}

	loginLimit := internal.ParseIntEnv("LOGIN_RATE_LIMIT", 5)
	loginWindow := internal.ParseDurationEnv("LOGIN_RATE_WINDOW", time.Minute)
	loginLimiter := &ratelimit.Limiter{
		Client: redisClient,
		Prefix: "sniply:ratelimit:",
		Limit:  loginLimit,
		Window: loginWindow,
	}

	cacheTTL := internal.ParseDurationEnv("SNIPPETS_CACHE_TTL", 2*time.Minute)
	listCacheTTL := internal.ParseDurationEnv("SNIPPETS_LIST_CACHE_TTL", 30*time.Second)
	snippetsCache := snippets.NewRedisCache(redisClient, "sniply:cache:")
	telemetry.InitAppMetrics("sniply-api", d.Pool, redisClient, sessionPrefix)

	usersService := &users.Service{Store: usrRepo}
	snippetsService := &snippets.Service{
		Store:        snRepo,
		Users:        usrRepo,
		Cache:        snippetsCache,
		CacheTTL:     cacheTTL,
		ListCacheTTL: listCacheTTL,
	}
	apiKeysService := &apikeys.Service{Store: apiKeysRepo}
	authService := &auth.Service{
		Users:        usrRepo,
		Sessions:     sessionManager,
		APIKeys:      apiKeysRepo,
		LoginLimiter: loginLimiter,
	}

	app := &httpapi.App{
		Health: &httpapi.HealthHandler{DB: d.Pool},
		Snippets: &httpapi.SnippetsHandler{
			Service: snippetsService,
		},
		Users: &httpapi.UsersHandler{Service: usersService},
		Auth: &httpapi.AuthHandler{
			Service:       authService,
			Authenticator: authService,
			Cookie:        cookie,
			CSRFCookie:    csrfCookie,
		},
		APIKeys:       &httpapi.APIKeysHandler{Service: apiKeysService},
		Authenticator: authService,
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
