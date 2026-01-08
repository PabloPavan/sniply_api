package internal

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func Env(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func MustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing env: %s", key)
	}
	return v
}

func RandomHex(nBytes int) string {
	b := make([]byte, nBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func DefaultPasswordHasher(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ParseDurationEnv(key string, def time.Duration) time.Duration {
	val := strings.TrimSpace(Env(key, ""))
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

func ParseIntEnv(key string, def int) int {
	val := strings.TrimSpace(Env(key, ""))
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

func ParseBoolEnv(key string, def bool) bool {
	val := strings.TrimSpace(Env(key, ""))
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

func ParseSameSiteEnv(key string, def http.SameSite) http.SameSite {
	val := strings.ToLower(strings.TrimSpace(Env(key, "")))
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
