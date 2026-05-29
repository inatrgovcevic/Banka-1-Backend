package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	authpkg "banka1/trading-service-go/internal/auth"
	"banka1/trading-service-go/internal/funds"
	grpcserver "banka1/trading-service-go/internal/grpc"
	httpapi "banka1/trading-service-go/internal/http"
	"banka1/trading-service-go/internal/order"
	"banka1/trading-service-go/internal/otc"
	"banka1/trading-service-go/internal/platform"
	"banka1/trading-service-go/internal/tax"

	gplog "banka1/go-platform/log"
	"banka1/go-platform/rabbitmq"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local liveness endpoint and exit (0=UP, 1=down); used by the container HEALTHCHECK")
	flag.Parse()
	if *healthcheck {
		os.Exit(runHealthcheck())
	}

	logger := gplog.New("trading-service-go", gplog.Level(os.Getenv("LOG_LEVEL_APP")))
	cfg := platform.LoadConfig()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := platform.OpenPostgres(ctx, cfg.DatabaseURL())
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// P8 cut-over: trading-service-go now owns the `trading` schema (it is the
	// canonical trading-service). On a fresh DB the migrations create the schema
	// (+ dev fixtures when LIQUIBASE_CONTEXTS=dev); on a DB Java Liquibase already
	// provisioned, RunMigrations baseline-skips them. See internal/platform/migrations.go.
	if err := platform.RunMigrations(ctx, db, "migrations"); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	jwtService := authpkg.NewJWTService(cfg.JWT)

	// Order decisions publish order.approved / order.declined to employee.events.
	// AllowNoop: a missing broker must not stop trading — fall back to a noop
	// publisher so the service still boots (matches Java RABBITMQ_LISTENER_ENABLED).
	rmqCfg := rabbitmq.LoadConfig()
	rmqCfg.AllowNoop = true
	var notifier order.Notifier = order.NoopNotifier{}
	var taxNotifier tax.Notifier = tax.NoopNotifier{}
	// employeeEventsPub is handed to NewApp so the OTC notifier (which also needs
	// the customer client) can be built there; nil when the broker is unavailable.
	var employeeEventsPub rabbitmq.Publisher
	if publisher, perr := rabbitmq.NewPublisher(ctx, rmqCfg, logger); perr != nil {
		logger.Warn("rabbitmq publisher init failed; order/tax/otc notifications disabled", "error", perr)
	} else {
		defer publisher.Close()
		notifier = order.NewRabbitNotifier(publisher, logger)
		taxNotifier = tax.NewRabbitNotifier(publisher, logger)
		employeeEventsPub = publisher
	}

	// P5: a SECOND publisher dedicated to SAGA_EVENTS_EXCHANGE for the fund
	// saga-request keys (fund.subscribe.requested / fund.redeem.requested /
	// fund.redeem.with-liquidation.requested). saga-orchestrator-service binds.
	// AllowNoop: a broker outage downgrades to NoopSagaPublisher so the local
	// invest/redeem tx still commits PENDING — supervisor sees the stuck row.
	sagaPubCfg := rabbitmq.LoadConfig()
	sagaPubCfg.AllowNoop = true
	sagaPubCfg.Exchange = cfg.SagaEventsExchange
	var sagaPublisher funds.SagaPublisher = funds.NewNoopSagaPublisher(logger)
	var otcSagaPublisher otc.SagaPublisher = otc.NewNoopSagaPublisher(logger)
	if pub, perr := rabbitmq.NewPublisher(ctx, sagaPubCfg, logger); perr != nil {
		logger.Warn("saga publisher init failed; fund/otc saga requests will noop", "error", perr)
	} else {
		defer pub.Close()
		// Both funds and OTC publish on saga.events — share the one publisher.
		sagaPublisher = funds.NewRabbitSagaPublisher(pub, logger)
		otcSagaPublisher = otc.NewRabbitSagaPublisher(pub, logger)
	}
	// Saga-result consumer configs: same broker creds, different exchanges. Funds
	// results land on SAGA_RESULTS_EXCHANGE (saga.exchange); OTC results land on
	// SAGA_EVENTS_EXCHANGE (saga.events). The Consumer scaffold declares the
	// exchange + queue + binds, so re-declaring an existing exchange is safe.
	sagaConsCfg := rabbitmq.LoadConfig()
	sagaConsCfg.Exchange = cfg.SagaResultsExchange
	otcSagaConsCfg := rabbitmq.LoadConfig()
	otcSagaConsCfg.Exchange = cfg.SagaEventsExchange

	app := httpapi.NewApp(cfg, db, jwtService, logger, notifier, taxNotifier, sagaPublisher, sagaConsCfg, employeeEventsPub, otcSagaPublisher, otcSagaConsCfg)
	defer app.Close()
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
	grpcSrv := grpcserver.NewServer(app, logger)

	go func() {
		logger.Info("trading-service-go http started", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	go func() {
		logger.Info("trading-service-go grpc started", "addr", grpcListener.Addr().String())
		if err := grpcSrv.Serve(grpcListener); err != nil {
			logger.Error("grpc server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("trading-service-go shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	grpcSrv.GracefulStop()
}

// runHealthcheck probes the local liveness endpoint and returns a process exit
// code. The distroless runtime image has no shell or curl, so the container
// HEALTHCHECK execs this same binary with -healthcheck instead of a curl command.
func runHealthcheck() int {
	port := strings.TrimSpace(os.Getenv("SERVER_PORT"))
	if port == "" {
		port = "18088"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/actuator/health/liveness")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
