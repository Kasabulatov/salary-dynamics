package main

import (
	"encoding/hex"
	"log"
	"net/http"
	"os"
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

	// v4 analytics module — optional: endpoints report "not configured"
	// until all three are set.
	TokenEncryptionKey []byte // 32 bytes from 64 hex chars, or nil
	YandexClientID     string
	YandexClientSecret string
	PublicAPIURL       string
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

		YandexClientID:     os.Getenv("YANDEX_CLIENT_ID"),
		YandexClientSecret: os.Getenv("YANDEX_CLIENT_SECRET"),
		PublicAPIURL:       envOr("PUBLIC_API_URL", "http://localhost:8080"),
	}
	if hexKey := os.Getenv("TOKEN_ENCRYPTION_KEY"); hexKey != "" {
		key, err := hex.DecodeString(hexKey)
		if err != nil || len(key) != 32 {
			log.Fatalf("TOKEN_ENCRYPTION_KEY must be 64 hex characters (32 bytes)")
		}
		cfg.TokenEncryptionKey = key
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
