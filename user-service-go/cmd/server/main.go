package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"banka1/user-service-go/internal/platform"
	"banka1/user-service-go/internal/user"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := platform.LoadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := platform.EnsurePostgresDatabase(ctx, cfg.DB); err != nil {
		logger.Error("database ensure failed", "error", err)
		os.Exit(1)
	}
	db, err := platform.OpenPostgres(ctx, cfg.DatabaseURL())
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := platform.RunMigrations(ctx, db, "migrations"); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	auth := platform.NewJWTService(cfg.JWT)
	jmbgCrypto, err := platform.NewJMBGCrypto(cfg.JMBG)
	if err != nil {
		logger.Error("jmbg crypto setup failed", "error", err)
		os.Exit(1)
	}
	publisher, err := platform.NewRabbitPublisher(ctx, cfg, logger)
	if err != nil {
		logger.Error("rabbit publisher setup failed", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	repo := user.NewRepository(db, jmbgCrypto)
	service := user.NewService(repo, auth, publisher, cfg.User, cfg.Email)
	handlers := user.NewHandlers(service)

	router := platform.NewRouter(platform.RouterDeps{
		Config:        cfg,
		Logger:        logger,
		DB:            db,
		Authenticator: auth,
		Register:      handlers.RegisterRoutes,
	})

	server := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("user-service-go started", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
