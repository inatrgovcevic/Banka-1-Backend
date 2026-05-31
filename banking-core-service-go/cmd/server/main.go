package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"banka1/banking-core-service-go/internal/config"
	"banka1/banking-core-service-go/internal/db"
	httpapi "banka1/banking-core-service-go/internal/http"
	"banka1/banking-core-service-go/internal/service"
)

func main() {
	cfg := config.Load()
	if err := service.ValidateStartupConfig(cfg); err != nil {
		log.Fatalf("validate startup config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := db.Open(ctx, cfg)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer conn.Close()

	if cfg.MigrationsEnabled {
		if err := db.Migrate(ctx, conn, cfg); err != nil {
			log.Fatalf("run migrations: %v", err)
		}
	}

	services := service.NewContainer(cfg, conn)
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	services.StartBackground(appCtx)

	handler := httpapi.NewHandler(cfg, services)

	server := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		log.Printf("banking-core-service-go listening on :%s", cfg.ServerPort)
		errs <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-stop:
		log.Printf("shutdown signal received: %s", sig)
	case err := <-errs:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	}

	appCancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
