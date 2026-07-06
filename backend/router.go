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
	r.Use(middleware.RequestSize(1 << 20)) // 1 MiB body cap on every endpoint
	r.Use(secureHeaders)

	// CORS must run BEFORE any rate limiter: a 429 without CORS headers is
	// blocked by the browser and surfaces as an opaque "Failed to fetch"
	// instead of a readable error (found by the e2e suite).
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.CORSOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	r.Use(httprate.LimitByIP(cfg.GlobalRatePerMin, time.Minute))

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
		r.Use(httprate.LimitByIP(cfg.AuthRatePerMin, time.Minute))
		r.Post("/api/register", auth.Register)
		r.Post("/api/login", auth.Login)
	})
	r.Post("/api/logout", auth.Logout)

	frank := services.NewFrankfurter()
	nbk := services.NewNBK()
	wb := services.NewWorldBank()
	converter := services.NewConverter(store, frank, nbk)

	salary := &handlers.SalaryHandler{Store: store, Converter: converter}
	series := &handlers.SeriesHandler{Salary: store, Store: store, Frank: frank, NBK: nbk}
	events := &handlers.EventsHandler{Store: store, Frank: frank, NBK: nbk}
	inflation := &handlers.InflationHandler{Salary: store, Store: store, WB: wb, Frank: frank, NBK: nbk}
	refresh := &handlers.RefreshHandler{Store: store, Frank: frank, NBK: nbk, WB: wb, Secret: cfg.RefreshSecret}
	news := &handlers.NewsHandler{Store: store, Secret: cfg.RefreshSecret}

	// Internal: cron-driven ingestion, protected by X-Refresh-Secret.
	// The workflow queries GDELT itself and posts headlines back (news.*).
	r.Post("/api/internal/refresh", refresh.Refresh)
	r.Get("/api/internal/news/pending", news.Pending)
	r.Post("/api/internal/news", news.Submit)

	// Public stateless compute (guest mode + landing demo): stricter limit,
	// nothing is persisted from these requests.
	public := &handlers.PublicHandler{Store: store, Converter: converter, WB: wb, Frank: frank, NBK: nbk}
	r.Group(func(r chi.Router) {
		r.Use(httprate.LimitByIP(cfg.PublicRatePerMin, time.Minute))
		r.Post("/api/public/compute", public.Compute)
		r.Post("/api/public/compare", public.Compare)
		r.Get("/api/public/compare/meta", public.CompareMeta)
	})

	// Protected routes: valid JWT cookie + CSRF header on writes.
	r.Group(func(r chi.Router) {
		r.Use(handlers.Auth(cfg.JWTSecret))
		r.Use(handlers.CSRF)
		r.Get("/api/me", auth.Me)

		r.Post("/api/salary", salary.Create)
		r.Get("/api/salary", salary.List)
		r.Get("/api/salary/series", series.Series)
		r.Get("/api/salary/inflation", inflation.Target)
		r.Get("/api/events", events.Events)
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
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
