// Command server is the Studio app's single entrypoint: load config, open the SQLite pool, apply
// pending migrations, mount every module's routes, and serve. Migrations and static assets are
// embedded (db/migrations/embed.go, static/embed.go) — the compiled binary is fully
// self-contained, nothing else needs to ship alongside it (see docs/deploy.md).
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"studio/internal/assessments"
	"studio/internal/assets"
	"studio/internal/auth"
	"studio/internal/clients"
	"studio/internal/config"
	"studio/internal/dashboard"
	"studio/internal/db"
	"studio/internal/export"
	"studio/internal/httplog"
	"studio/internal/iiif"
	"studio/internal/logging"
	"studio/internal/mail"
	"studio/internal/media"
	"studio/internal/reporter"
	"studio/internal/seed"
	"studio/internal/session"
	"studio/internal/settings"
	"studio/internal/treatments"
	"studio/internal/web"
	"studio/internal/workflows"

	"studio/db/migrations"
	staticassets "studio/static"
)

func main() {
	cfg := config.Load()
	slog.SetDefault(logging.New(cfg))

	const category = "startup"

	pool, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("opening database", "err", err, "category", category, "event", "db_open_failed")
		os.Exit(1)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := pool.PingContext(ctx); err != nil {
		slog.Error("pinging database", "err", err, "category", category, "event", "db_ping_failed")
		os.Exit(1)
	}
	if err := db.Migrate(ctx, pool, migrations.Files); err != nil {
		slog.Error("running migrations", "err", err, "category", category, "event", "migrate_failed")
		os.Exit(1)
	}
	slog.Info("database ready, migrations applied", "category", category, "event", "db_ready")

	if err := seed.BootstrapAdmin(ctx, pool, cfg.BootstrapAdminName, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword); err != nil {
		slog.Error("bootstrapping admin", "err", err, "category", category, "event", "bootstrap_admin_failed")
		os.Exit(1)
	}

	media.InitImageProcessing()
	defer media.ShutdownImageProcessing()

	storage, err := media.NewLocalDiskAdapter(cfg.MediaStorageDir)
	if err != nil {
		slog.Error("initializing media storage", "err", err, "category", category, "event", "media_storage_init_failed")
		os.Exit(1)
	}
	mediaSvc := &media.Service{Pool: pool, Storage: storage}

	// Example data across every domain model (clients, assets, a project, a treatment, a report,
	// media library images) plus a second "conservator" example login — dev/docker convenience
	// only, never a production deploy (see ansible/roles/studio_app/defaults/main.yml) — a
	// production boot only ever gets the one BootstrapAdmin account above.
	if cfg.SeedExampleData {
		if err := seed.SeedDemoData(ctx, pool, mediaSvc); err != nil {
			slog.Error("seeding demo data", "err", err, "category", category, "event", "seed_demo_failed")
			os.Exit(1)
		}
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
		AppURL: cfg.AppURL,
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServerFS(staticassets.Files)))
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
	assessments.Mount(mux, &assessments.Service{Pool: pool, Auth: authSvc, Media: mediaSvc})
	workflows.Mount(mux, &workflows.Service{Pool: pool, Auth: authSvc, Media: mediaSvc})
	reporter.Mount(mux, &reporter.Service{Pool: pool, Auth: authSvc, Media: mediaSvc})
	export.Mount(mux, &export.Service{Pool: pool, Media: mediaSvc}, authSvc)
	iiif.Mount(mux, &iiif.Service{Media: mediaSvc}, authSvc)

	addr := ":" + cfg.Port
	slog.Info("listening", "addr", addr, "log_level", cfg.LogLevel, "log_format", cfg.LogFormat,
		"category", category, "event", "listening")
	if err := http.ListenAndServe(addr, httplog.Middleware(mux)); err != nil {
		slog.Error("server", "err", err, "category", category, "event", "server_failed")
		os.Exit(1)
	}
}
