// Command saga-orchestrator-service runs the Banka 1 saga orchestrator.
// It wires config, Postgres pool, goose migrations, RabbitMQ topology,
// 5 saga listeners, 1 publisher, admin HTTP server, and cleanup scheduler
// into a single statically-linked binary with graceful shutdown.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/api"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/client"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/config"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/events"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/rabbit"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/saga"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/scheduler"
	"github.com/raf-si-2025/banka-1-go/saga-orchestrator-service/internal/store"
	"github.com/raf-si-2025/banka-1-go/shared/auth"
	sharedLog "github.com/raf-si-2025/banka-1-go/shared/log"
	pgxpoolx "github.com/raf-si-2025/banka-1-go/shared/pgxpool"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// -------------------------------------------------------------------------
	// Config
	// -------------------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// -------------------------------------------------------------------------
	// Logger
	// -------------------------------------------------------------------------
	logger := sharedLog.New(sharedLog.LogConfig{
		Level: cfg.Server.LogLevel,
		JSON:  cfg.Server.LogJSON,
	})
	logger.Info("starting saga-orchestrator-service",
		"port", cfg.Server.HTTPPort,
		"rabbitMQ", cfg.Saga.RabbitMQURL,
		"bankingCore", cfg.Saga.Services.BankingCoreURL,
		"trading", cfg.Saga.Services.TradingURL,
		"market", cfg.Saga.Services.MarketURL,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// -------------------------------------------------------------------------
	// Postgres pool (pgx/v5)
	// -------------------------------------------------------------------------
	pool, err := pgxpoolx.New(ctx, pgxpoolx.Config{
		URL:      cfg.DB.URL,
		MaxConns: cfg.DB.MaxConns,
		MinConns: cfg.DB.MinConns,
	})
	if err != nil {
		return fmt.Errorf("pgxpool: %w", err)
	}
	defer pool.Close()

	// -------------------------------------------------------------------------
	// Goose migrations (database/sql + pgx/v5/stdlib)
	// -------------------------------------------------------------------------
	sqlDB, err := sql.Open("pgx", cfg.DB.URL)
	if err != nil {
		return fmt.Errorf("sql.Open for goose: %w", err)
	}
	if err := goose.SetDialect("postgres"); err != nil {
		sqlDB.Close()
		return fmt.Errorf("goose set dialect: %w", err)
	}
	if err := goose.Up(sqlDB, cfg.Server.MigrationsPath); err != nil {
		sqlDB.Close()
		return fmt.Errorf("goose up: %w", err)
	}
	sqlDB.Close()
	logger.Info("migrations applied")

	// -------------------------------------------------------------------------
	// Store
	// -------------------------------------------------------------------------
	sagaStore := store.NewSagaInstanceStore(pool)

	// -------------------------------------------------------------------------
	// Auth: S2S JWT issuer for service-to-service calls
	// -------------------------------------------------------------------------
	s2sIssuer := auth.NewS2SIssuer(
		cfg.JWT.Issuer,
		"saga-orchestrator-service",
		[]string{"SERVICE"},
		cfg.JWT.Secret,
		cfg.JWT.TTL,
	)

	// -------------------------------------------------------------------------
	// REST clients
	// -------------------------------------------------------------------------
	bcClient := client.NewBankingCoreClient(
		cfg.Saga.Services.BankingCoreURL,
		s2sIssuer,
		cfg.Saga.Services.Timeout,
	)
	tdClient := client.NewTradingServiceClient(
		cfg.Saga.Services.TradingURL,
		s2sIssuer,
		cfg.Saga.Services.Timeout,
	)
	mkClient := client.NewMarketServiceClient(
		cfg.Saga.Services.MarketURL,
		s2sIssuer,
		cfg.Saga.Services.Timeout,
	)

	// -------------------------------------------------------------------------
	// RabbitMQ connection + topology
	// -------------------------------------------------------------------------
	rabbitConn, err := rabbit.Dial(cfg.Saga.RabbitMQURL)
	if err != nil {
		return fmt.Errorf("rabbitmq dial: %w", err)
	}
	defer rabbitConn.Close()

	// Topology channel — declare once, then discard.
	topoCh, err := rabbitConn.Channel()
	if err != nil {
		return fmt.Errorf("rabbitmq topology channel: %w", err)
	}
	if err := rabbit.DeclareTopology(topoCh); err != nil {
		topoCh.Close()
		return fmt.Errorf("rabbitmq declare topology: %w", err)
	}
	topoCh.Close()
	logger.Info("RabbitMQ topology declared")

	// Publisher channel — saga result events go here.
	pubCh, err := rabbitConn.Channel()
	if err != nil {
		return fmt.Errorf("rabbitmq publisher channel: %w", err)
	}
	defer pubCh.Close()
	publisher := rabbit.NewPublisher(pubCh)

	// -------------------------------------------------------------------------
	// Saga Orchestrator (wired with real publisher)
	// -------------------------------------------------------------------------
	orch := saga.NewOrchestrator(
		sagaStore,
		bcClient,
		tdClient,
		mkClient,
		publisher,
		logger,
	)

	// -------------------------------------------------------------------------
	// Saga listeners — one dedicated channel per queue
	// -------------------------------------------------------------------------
	type listenerSpec struct {
		queue   string
		decoder rabbit.Decoder
		handler rabbit.Handler
	}

	specs := []listenerSpec{
		{
			queue: rabbit.QueueOtcExercise,
			decoder: func(body []byte) (any, error) {
				var evt events.OtcExerciseRequested
				return evt, json.Unmarshal(body, &evt)
			},
			handler: func(ctx context.Context, payload any) error {
				return orch.HandleOtcExercise(ctx, payload.(events.OtcExerciseRequested))
			},
		},
		{
			queue: rabbit.QueueOtcPremium,
			decoder: func(body []byte) (any, error) {
				var evt events.OtcPremiumTransferRequested
				return evt, json.Unmarshal(body, &evt)
			},
			handler: func(ctx context.Context, payload any) error {
				return orch.HandleOtcPremiumTransfer(ctx, payload.(events.OtcPremiumTransferRequested))
			},
		},
		{
			queue: rabbit.QueueFundSubscribe,
			decoder: func(body []byte) (any, error) {
				var evt events.FundSubscribeRequested
				return evt, json.Unmarshal(body, &evt)
			},
			handler: func(ctx context.Context, payload any) error {
				return orch.HandleFundSubscribe(ctx, payload.(events.FundSubscribeRequested))
			},
		},
		{
			queue: rabbit.QueueFundRedeem,
			decoder: func(body []byte) (any, error) {
				var evt events.FundRedeemRequested
				return evt, json.Unmarshal(body, &evt)
			},
			handler: func(ctx context.Context, payload any) error {
				return orch.HandleFundRedeem(ctx, payload.(events.FundRedeemRequested))
			},
		},
		{
			queue: rabbit.QueueFundRedeemWithLiquidation,
			decoder: func(body []byte) (any, error) {
				var evt events.FundRedeemWithLiquidationRequested
				return evt, json.Unmarshal(body, &evt)
			},
			handler: func(ctx context.Context, payload any) error {
				return orch.HandleFundRedeemWithLiquidation(ctx, payload.(events.FundRedeemWithLiquidationRequested))
			},
		},
	}

	listenerDone := make([]chan error, len(specs))
	for i, spec := range specs {
		ch, err := rabbitConn.Channel()
		if err != nil {
			return fmt.Errorf("rabbitmq channel for %q: %w", spec.queue, err)
		}
		defer ch.Close()

		l := rabbit.NewListener(ch, spec.queue, spec.decoder, spec.handler, logger)
		done := make(chan error, 1)
		listenerDone[i] = done
		go func(l *rabbit.Listener, done chan<- error) {
			done <- l.Run(ctx)
		}(l, done)
	}
	logger.Info("RabbitMQ listeners started", "count", len(specs))

	// -------------------------------------------------------------------------
	// Cleanup scheduler
	// -------------------------------------------------------------------------
	cleanupSched := scheduler.New(
		sagaStore,
		pool,
		scheduler.CleanupConfig{
			Interval:             cfg.Saga.Cleanup.Interval,
			StuckCutoff:          cfg.Saga.Cleanup.StuckCutoff,
			IdempotencyRetention: cfg.Saga.Cleanup.IdempotencyRetention,
		},
		logger,
	)
	schedDone := make(chan error, 1)
	go func() { schedDone <- cleanupSched.Run(ctx) }()

	// -------------------------------------------------------------------------
	// Admin HTTP server
	// -------------------------------------------------------------------------
	adminHandler := api.NewAdminHandler(sagaStore, orch)
	router := api.NewRouter(adminHandler)

	httpAddr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	httpSrv := &http.Server{
		Addr:         httpAddr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
	httpDone := make(chan error, 1)
	go func() {
		logger.Info("admin HTTP server listening", "addr", httpAddr)
		if listenErr := httpSrv.ListenAndServe(); !errors.Is(listenErr, http.ErrServerClosed) {
			httpDone <- listenErr
		}
		close(httpDone)
	}()

	// -------------------------------------------------------------------------
	// Wait for OS signal or fatal error
	// -------------------------------------------------------------------------
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case s := <-sig:
		logger.Info("shutdown signal received", "signal", s)
	case err := <-httpDone:
		if err != nil {
			cancel()
			return fmt.Errorf("admin HTTP server fatal: %w", err)
		}
	case err := <-schedDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			cancel()
			return fmt.Errorf("cleanup scheduler fatal: %w", err)
		}
	}

	// -------------------------------------------------------------------------
	// Graceful shutdown
	// -------------------------------------------------------------------------
	cancel() // stop scheduler + listeners

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("admin HTTP server shutdown error", "err", err)
	}

	// Drain cleanup scheduler.
	select {
	case <-schedDone:
	case <-shutdownCtx.Done():
		logger.Warn("timed out waiting for cleanup scheduler to stop")
	}

	// Drain listeners.
	for _, done := range listenerDone {
		select {
		case <-done:
		case <-shutdownCtx.Done():
		}
	}

	logger.Info("shutdown complete")
	return nil
}
