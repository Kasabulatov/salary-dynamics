package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	DatabaseURL    string
	JWTSecret      []byte
	RefreshSecret  string
	CORSOrigin     string
	CookieSecure   bool
	CookieSameSite http.SameSite
	Port           string
	MigrationsDir  string

	// Auth endpoints rate limit per IP per minute. Production default is 10
	// (brute-force protection); local/CI compose raises it so the E2E suite
	// (which registers many users quickly) doesn't trip it.
	AuthRatePerMin int
}

func LoadConfig() Config {
	cfg := Config{
		DatabaseURL:    mustEnv("DATABASE_URL"),
		JWTSecret:      []byte(mustEnv("JWT_SECRET")),
		RefreshSecret:  mustEnv("REFRESH_SECRET"),
		CORSOrigin:     envOr("CORS_ORIGIN", "http://localhost:5173"),
		CookieSecure:   envOr("COOKIE_SECURE", "false") == "true",
		CookieSameSite: parseSameSite(envOr("COOKIE_SAMESITE", "lax")),
		Port:           envOr("PORT", "8080"),
		MigrationsDir:  envOr("MIGRATIONS_DIR", "migrations"),
		AuthRatePerMin: 10,
	}
	if s := os.Getenv("AUTH_RATE_PER_MIN"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			log.Fatalf("AUTH_RATE_PER_MIN must be a positive integer")
		}
		cfg.AuthRatePerMin = n
	}
	return cfg
}

// parseSameSite maps the COOKIE_SAMESITE env var to http.SameSite.
// Local (same-site ports): "lax". Production cross-site (e.g. Vercel frontend
// + Render backend): "none", which also requires COOKIE_SECURE=true.
func parseSameSite(v string) http.SameSite {
	switch v {
	case "none":
		return http.SameSiteNoneMode
	case "strict":
		return http.SameSiteStrictMode
	default:
		return http.SameSiteLaxMode
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
