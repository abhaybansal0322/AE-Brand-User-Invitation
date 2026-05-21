package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/admin"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/config"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/httpapi"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/service"
	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server_exit", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fileStore, err := store.NewFileStore(cfg.DataFile)
	if err != nil {
		return err
	}

	var adminClient admin.Client
	if cfg.AdminAPIBaseURL != "" {
		adminClient, err = admin.NewHTTPClient(cfg.AdminAPIBaseURL, nil)
		if err != nil {
			return err
		}
	} else {
		adminClient = admin.NewFileClient(fileStore)
	}

	invitationService := service.New(
		fileStore,
		fileStore,
		adminClient,
		service.WithAuditRetentionDays(cfg.AuditRetentionDays),
		service.WithDetailTimeout(cfg.RequestTimeout),
	)

	server := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      httpapi.NewRouter(invitationService, logger),
		ReadTimeout:  cfg.RequestTimeout,
		WriteTimeout: cfg.RequestTimeout,
		IdleTimeout:  cfg.RequestTimeout * 20,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server_started", slog.String("addr", cfg.HTTPAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server_listen_failed", slog.String("error", err.Error()))
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
