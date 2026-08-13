// Command server is the Studio app's single entrypoint: load config, open the MySQL pool, apply
// pending migrations, mount every module's routes, and serve.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"studio/internal/config"
	"studio/internal/db"
	"studio/internal/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := pool.PingContext(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}
	if err := db.Migrate(ctx, pool, "db/migrations"); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("database ready, migrations applied")

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page := web.Layout("Studio", web.Placeholder("Go/templ/MySQL rewrite — scaffold module complete."))
		if err := page.Render(r.Context(), w); err != nil {
			log.Printf("render error: %v", err)
		}
	})

	addr := ":" + cfg.Port
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}
