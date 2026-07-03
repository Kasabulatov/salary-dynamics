package main

import (
	"context"
	"log"
	"net/http"

	"dynamics-dashboard/database"
)

func main() {
	ctx := context.Background()
	cfg := LoadConfig()

	store, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer store.Pool.Close()

	if err := store.Migrate(ctx, cfg.MigrationsDir); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	log.Println("migrations applied")

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: newRouter(cfg, store),
	}
	log.Printf("listening on :%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
