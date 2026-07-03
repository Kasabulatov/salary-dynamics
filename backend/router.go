package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"dynamics-dashboard/database"
	"dynamics-dashboard/handlers"
	"dynamics-dashboard/services"
)

func newRouter(cfg Config, store *database.Store) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(secureHeaders)
	r.Use(httprate.LimitByIP(120, time.Minute)) // global rate limit

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.CORSOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	auth := &handlers.AuthHandler{
		Store:          store,
		JWTSecret:      cfg.JWTSecret,
		CookieSecure:   cfg.CookieSecure,
		CookieSameSite: cfg.CookieSameSite,
	}

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "ok"}`))
	})

	// Public auth routes with a stricter rate limit (brute-force protection).
	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitByIP(10, time.Minute))
		r.Post("/api/register", auth.Register)
		r.Post("/api/login", auth.Login)
	})
	r.Post("/api/logout", auth.Logout)

	frank := services.NewFrankfurter()
	nbk := services.NewNBK()
	converter := services.NewConverter(store, frank, nbk)

	salary := &handlers.SalaryHandler{Store: store, Converter: converter}
	series := &handlers.SeriesHandler{Salary: store, Store: store, Frank: frank, NBK: nbk}
	refresh := &handlers.RefreshHandler{Store: store, Frank: frank, NBK: nbk, Secret: cfg.RefreshSecret}

	// Internal: rate ingestion, protected by X-Refresh-Secret (cron calls this).
	r.Post("/api/internal/refresh", refresh.Refresh)

	// Protected routes: valid JWT cookie + CSRF header on writes.
	r.Group(func(r chi.Router) {
		r.Use(handlers.Auth(cfg.JWTSecret))
		r.Use(handlers.CSRF)
		r.Get("/api/me", auth.Me)

		r.Post("/api/salary", salary.Create)
		r.Get("/api/salary", salary.List)
		r.Get("/api/salary/series", series.Series)
		r.Put("/api/salary/{id}", salary.Update)
		r.Delete("/api/salary/{id}", salary.Delete)
	})

	return r
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}
