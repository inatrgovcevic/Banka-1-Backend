package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"banka1/market-service-go/internal/auth"
	grpcserver "banka1/market-service-go/internal/grpc"
	httpapi "banka1/market-service-go/internal/http"
	"banka1/market-service-go/internal/platform"

	gplog "banka1/go-platform/log"
)

func main() {
	logger := gplog.New("market-service-go", gplog.Level(os.Getenv("LOG_LEVEL_APP")))
	cfg := platform.LoadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := platform.EnsurePostgresDatabase(ctx, cfg); err != nil {
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

	app, err := httpapi.NewApp(ctx, cfg, db, logger)
	if err != nil {
		logger.Error("app setup failed", "error", err)
		os.Exit(1)
	}
	defer app.Close()

	jwtService := auth.NewJWTService(cfg.JWT)
	httpHandler := httpapi.NewRouter(cfg, logger, db, jwtService, app)
	httpServer := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           httpHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Error("grpc listen failed", "error", err)
		os.Exit(1)
	}
	grpcSrv := grpcserver.NewServer(app)

	go func() {
		logger.Info("market-service-go http started", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	go func() {
		logger.Info("market-service-go grpc started", "addr", grpcListener.Addr().String())
		if err := grpcSrv.Serve(grpcListener); err != nil {
			logger.Error("grpc server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	grpcSrv.GracefulStop()
}
