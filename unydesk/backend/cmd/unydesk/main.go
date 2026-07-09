package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"unydesk/auth"
	"unydesk/config"
	"unydesk/remote"
	"unydesk/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.Load("settings/settings.yaml")
	if err != nil {
		logger.Error("settings load failed", "err", err)
		os.Exit(1)
	}

	store := remote.NewMemoryStore()
	if err := store.ConfigurePersistence(cfg.Paths.HostsFile, cfg.Paths.TrustedHostsFile); err != nil {
		logger.Error("remote store init failed", "err", err)
		os.Exit(1)
	}
	authStore, err := auth.NewStore(cfg.Paths.UsersFile)
	if err != nil {
		logger.Error("auth store init failed", "err", err)
		os.Exit(1)
	}
	srv := server.New(cfg, store, authStore, logger)

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logger.Error("server stopped", "err", err)
		}
	}()

	logger.Info("unydesk listening",
		"addr", srv.Addr(),
		"http3_enabled", srv.NativeHTTP3Enabled(),
		"http3_addr", srv.HTTP3Addr(),
		"name", cfg.Name,
		"version", config.Version,
	)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutdown requested")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown failed", "err", err)
		os.Exit(1)
	}

	logger.Info("stopped gracefully")
}
