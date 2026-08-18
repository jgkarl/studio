// Command server is the Studio app's single entrypoint: load config, open the SQLite pool, apply
// pending migrations, mount every module's routes, and serve.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"studio/internal/assets"
	"studio/internal/auth"
	"studio/internal/clients"
	"studio/internal/config"
	"studio/internal/dashboard"
	"studio/internal/db"
	"studio/internal/export"
	"studio/internal/mail"
	"studio/internal/media"
	"studio/internal/reporter"
	"studio/internal/seed"
	"studio/internal/session"
	"studio/internal/settings"
	"studio/internal/treatments"
	"studio/internal/web"
	"studio/internal/workflows"
)

func main() {
	cfg := config.Load()

	pool, err := db.Open(cfg.DBPath)
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

	if err := seed.BootstrapAdmin(ctx, pool, cfg.BootstrapAdminName, cfg.BootstrapAdminEmail); err != nil {
		log.Fatalf("bootstrapping admin: %v", err)
	}

	media.InitImageProcessing()
	defer media.ShutdownImageProcessing()

	storage, err := media.NewLocalDiskAdapter(cfg.MediaStorageDir)
	if err != nil {
		log.Fatalf("media storage: %v", err)
	}

	authSvc := &auth.Service{
		Pool:     pool,
		Sessions: session.New(cfg.AuthSecret),
		Mailer: mail.New(mail.Config{
			Host: cfg.SMTPHost,
			Port: cfg.SMTPPort,
			User: cfg.SMTPUser,
			Pass: cfg.SMTPPass,
			From: cfg.SMTPFrom,
		}),
		AppURL:        cfg.AppURL,
		AllowDevLogin: cfg.AllowDevLogin,
	}
	mediaSvc := &media.Service{Pool: pool, Storage: storage}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	web.MountToggles(mux)
	auth.Mount(mux, authSvc)
	settings.Mount(mux, &settings.Service{Pool: pool, Auth: authSvc})
	dashboard.Mount(mux, &dashboard.Service{Pool: pool, Auth: authSvc})
	clients.Mount(mux, &clients.Service{Pool: pool, Auth: authSvc})
	assets.Mount(mux, &assets.Service{Pool: pool, Auth: authSvc, Media: mediaSvc})
	media.Mount(mux, &media.HandlerService{Service: mediaSvc, Auth: authSvc})
	treatments.Mount(mux, &treatments.Service{Pool: pool, Auth: authSvc, Media: mediaSvc})
	workflows.Mount(mux, &workflows.Service{Pool: pool, Auth: authSvc})
	reporter.Mount(mux, &reporter.Service{Pool: pool, Auth: authSvc, Media: mediaSvc})
	export.Mount(mux, &export.Service{Pool: pool, Media: mediaSvc}, authSvc)

	addr := ":" + cfg.Port
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}
