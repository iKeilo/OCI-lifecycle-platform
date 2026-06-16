package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"a-series-oracle/backend/internal/api"
	"a-series-oracle/backend/internal/auth"
	"a-series-oracle/backend/internal/config"
	"a-series-oracle/backend/internal/db"
	"a-series-oracle/backend/internal/domain"
	"a-series-oracle/backend/internal/fileprofile"
	"a-series-oracle/backend/internal/jobs"
	"a-series-oracle/backend/internal/oci"
	"a-series-oracle/backend/internal/profileconfig"
	"a-series-oracle/backend/internal/store"
)

func main() {
	cfg := config.Load()
	addr := ":" + cfg.Port
	ctx := context.Background()
	dbConn, err := db.Open(ctx, cfg.Database.URL)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	if dbConn != nil {
		defer dbConn.Close()
		if err := db.Migrate(ctx, dbConn); err != nil {
			slog.Error("database migration failed", "error", err)
			os.Exit(1)
		}
		slog.Info("database migrations ready")
	}

	appStore := store.New()
	if _, err := appStore.SetEmailSettings(domain.EmailSettings{
		Enabled:  cfg.Email.Enabled,
		Host:     cfg.Email.Host,
		Port:     cfg.Email.Port,
		Username: cfg.Email.Username,
		Password: cfg.Email.Password,
		From:     cfg.Email.From,
		To:       cfg.Email.To,
		UseTLS:   cfg.Email.UseTLS,
		StartTLS: cfg.Email.StartTLS,
	}); err != nil {
		slog.Error("email settings setup failed", "error", err)
		os.Exit(1)
	}
	if _, err := appStore.SetWebhookSettings(domain.WebhookSettings{
		Enabled: cfg.Webhook.Enabled,
		URL:     cfg.Webhook.URL,
		Secret:  cfg.Webhook.Secret,
	}); err != nil {
		slog.Error("webhook settings setup failed", "error", err)
		os.Exit(1)
	}
	var recoveredJobs []string
	if dbConn != nil {
		persistence := db.NewPostgresSink(dbConn)
		if err := persistence.SetProfileKeyEncryptionKey(cfg.Security.ProfileKeyEncryptionKey); err != nil {
			slog.Error("profile key encryption setup failed", "error", err)
			os.Exit(1)
		}
		profiles, err := persistence.ListProfiles()
		if err != nil {
			slog.Error("database profile load failed", "error", err)
			os.Exit(1)
		}
		appStore.ReplaceProfiles(profiles)
		instances, err := persistence.ListInstances("")
		if err != nil {
			slog.Error("database instance load failed", "error", err)
			os.Exit(1)
		}
		appStore.ReplaceInstances(instances)
		templates, err := persistence.ListTemplates()
		if err != nil {
			slog.Error("database template load failed", "error", err)
			os.Exit(1)
		}
		appStore.ReplaceTemplates(templates)
		appStore.SetPersistenceSink(persistence)
		if err := appStore.LoadPersistedSettings(); err != nil {
			slog.Error("database settings load failed", "error", err)
			os.Exit(1)
		}
		persistedJobs, err := persistence.ListJobs()
		if err != nil {
			slog.Error("database job load failed", "error", err)
			os.Exit(1)
		}
		appStore.ReplaceJobs(persistedJobs)
		runnableJobs, err := appStore.RecoverRunnableJobs()
		if err != nil {
			slog.Error("database job recovery failed", "error", err)
			os.Exit(1)
		}
		for _, job := range runnableJobs {
			recoveredJobs = append(recoveredJobs, job.ID)
		}
	} else if cfg.Security.ProfileStoreFile != "" {
		persistence, err := fileprofile.New(cfg.Security.ProfileStoreFile)
		if err != nil {
			slog.Error("file profile store setup failed", "error", err)
			os.Exit(1)
		}
		if err := persistence.SetProfileKeyEncryptionKey(cfg.Security.ProfileKeyEncryptionKey); err != nil {
			slog.Error("profile key encryption setup failed", "error", err)
			os.Exit(1)
		}
		profiles, err := persistence.ListProfiles()
		if err != nil {
			slog.Error("file profile load failed", "error", err)
			os.Exit(1)
		}
		appStore.ReplaceProfiles(profiles)
		templates, err := persistence.ListTemplates()
		if err != nil {
			slog.Error("file template load failed", "error", err)
			os.Exit(1)
		}
		appStore.ReplaceTemplates(templates)
		appStore.SetPersistenceSink(persistence)
		if err := appStore.LoadPersistedSettings(); err != nil {
			slog.Error("file settings load failed", "error", err)
			os.Exit(1)
		}
		if len(profiles) == 0 {
			if profile := envProfile(cfg); profile.ID != "" {
				appStore.ReplaceProfiles([]domain.Profile{profile})
			}
		}
	} else if profile := envProfile(cfg); profile.ID != "" {
		appStore.ReplaceProfiles([]domain.Profile{profile})
	}
	readiness := oci.ReadinessConfig{
		ExecutionMode:  string(cfg.ExecutionMode),
		TenancyOCID:    cfg.OCI.TenancyOCID,
		UserOCID:       cfg.OCI.UserOCID,
		Fingerprint:    cfg.OCI.Fingerprint,
		PrivateKey:     cfg.OCI.PrivateKey,
		PrivateKeyFile: cfg.OCI.PrivateKeyFile,
		Region:         cfg.OCI.Region,
	}
	profileResolver := profileconfig.NewResolver(appStore, readiness)
	executor := jobs.Executor(jobs.NewLocalExecutor(appStore))
	if cfg.ExecutionMode == config.ExecutionModeOCI {
		executor = jobs.NewOCIExecutorWithResolver(appStore, readiness, profileResolver)
	}
	runner := jobs.NewRunnerWithExecutor(appStore, executor)
	runner.Start(ctx)
	for _, jobID := range recoveredJobs {
		runner.Enqueue(jobID)
	}
	authManager, err := auth.New(auth.Config{
		PasswordHash:  cfg.Security.PanelPasswordHash,
		PlainPassword: cfg.Security.PanelPassword,
		SessionSecret: cfg.Security.PanelSessionSecret,
		AuthDisabled:  cfg.Security.PanelAuthDisabled,
	})
	if err != nil {
		slog.Error("panel auth setup failed", "error", err)
		os.Exit(1)
	}
	if persisted := appStore.GetAccountSettingsForAuth(); persisted.PasswordHash != "" {
		authManager.SetPasswordHash(persisted.PasswordHash)
	}

	handler := api.NewServerWithOptions(appStore, api.ServerOptions{
		Enqueue:         runner.Enqueue,
		ExecutionMode:   string(cfg.ExecutionMode),
		OCIReadiness:    readiness,
		ProfileResolver: profileResolver,
		Auth:            authManager,
	}).Handler()
	if strings.TrimSpace(cfg.StaticDir) != "" {
		handler = withStaticFiles(handler, cfg.StaticDir)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("starting OCI lifecycle API", "addr", addr, "executionMode", cfg.ExecutionMode)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func withStaticFiles(apiHandler http.Handler, staticDir string) http.Handler {
	staticDir = filepath.Clean(staticDir)
	fileServer := http.FileServer(http.Dir(staticDir))
	indexPath := filepath.Join(staticDir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		requestedPath := filepath.Join(staticDir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(requestedPath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, indexPath)
	})
}

func envProfile(cfg config.Config) domain.Profile {
	if cfg.OCI.TenancyOCID == "" || cfg.OCI.UserOCID == "" || cfg.OCI.Fingerprint == "" || cfg.OCI.Region == "" {
		return domain.Profile{}
	}
	return domain.Profile{
		ID:            "env-default",
		Name:          "ENV DEFAULT",
		TenancyOCID:   cfg.OCI.TenancyOCID,
		UserOCID:      cfg.OCI.UserOCID,
		Fingerprint:   cfg.OCI.Fingerprint,
		DefaultRegion: cfg.OCI.Region,
		Status:        "Configured",
		LastCheckedAt: time.Now().UTC(),
	}
}
