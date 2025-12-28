package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PabloPavan/sniply_api/internal"
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

	sessionPrefix := internal.Env("SESSION_REDIS_PREFIX", "sniply:session:")
	sessionTTL := parseDurationEnv("SESSION_TTL", 7*24*time.Hour)
	sessionManager := &session.Manager{
		Store:   session.NewRedisStore(redisClient, sessionPrefix),
		TTL:     sessionTTL,
		IDBytes: 32,
	}

	cookieSecure := parseBoolEnv("SESSION_COOKIE_SECURE", true)
	cookieSameSite := parseSameSiteEnv("SESSION_COOKIE_SAMESITE", http.SameSiteLaxMode)
	cookie := session.CookieConfig{
		Name:     internal.Env("SESSION_COOKIE_NAME", "sniply_session"),
		Path:     internal.Env("SESSION_COOKIE_PATH", "/"),
		Domain:   internal.Env("SESSION_COOKIE_DOMAIN", ""),
		Secure:   cookieSecure,
		SameSite: cookieSameSite,
	}

	loginLimit := parseIntEnv("LOGIN_RATE_LIMIT", 5)
	loginWindow := parseDurationEnv("LOGIN_RATE_WINDOW", time.Minute)
	loginLimiter := &ratelimit.Limiter{
		Client: redisClient,
		Prefix: "sniply:ratelimit:",
		Limit:  loginLimit,
		Window: loginWindow,
	}

	cacheTTL := parseDurationEnv("SNIPPETS_CACHE_TTL", 2*time.Minute)
	listCacheTTL := parseDurationEnv("SNIPPETS_LIST_CACHE_TTL", 30*time.Second)
	snippetsCache := snippets.NewRedisCache(redisClient, "sniply:cache:")
	telemetry.InitAppMetrics("sniply-api", d.Pool, redisClient, sessionPrefix)

	app := &httpapi.App{
		Health: &httpapi.HealthHandler{DB: d.Pool},
		Snippets: &httpapi.SnippetsHandler{
			Repo:         snRepo,
			RepoUser:     usrRepo,
			Cache:        snippetsCache,
			CacheTTL:     cacheTTL,
			ListCacheTTL: listCacheTTL,
		},
		Users: &httpapi.UsersHandler{Repo: usrRepo},
		Auth: &httpapi.AuthHandler{
			Users:        usrRepo,
			Sessions:     sessionManager,
			Cookie:       cookie,
			LoginLimiter: loginLimiter,
		},
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

func parseDurationEnv(key string, def time.Duration) time.Duration {
	val := strings.TrimSpace(internal.Env(key, ""))
	if val == "" {
		return def
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		log.Printf("invalid %s: %q, using default", key, val)
		return def
	}
	return d
}

func parseIntEnv(key string, def int) int {
	val := strings.TrimSpace(internal.Env(key, ""))
	if val == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		log.Printf("invalid %s: %q, using default", key, val)
		return def
	}
	return n
}

func parseBoolEnv(key string, def bool) bool {
	val := strings.TrimSpace(internal.Env(key, ""))
	if val == "" {
		return def
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		log.Printf("invalid %s: %q, using default", key, val)
		return def
	}
	return b
}

func parseSameSiteEnv(key string, def http.SameSite) http.SameSite {
	val := strings.ToLower(strings.TrimSpace(internal.Env(key, "")))
	switch val {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		return http.SameSiteLaxMode
	case "":
		return def
	default:
		log.Printf("invalid %s: %q, using default", key, val)
		return def
	}
}
