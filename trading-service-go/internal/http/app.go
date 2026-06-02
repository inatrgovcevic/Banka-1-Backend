package http

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"banka1/trading-service-go/internal/actuary"
	"banka1/trading-service-go/internal/analytics"
	"banka1/trading-service-go/internal/clients"
	"banka1/trading-service-go/internal/funds"
	"banka1/trading-service-go/internal/interbank"
	"banka1/trading-service-go/internal/order"
	"banka1/trading-service-go/internal/otc"
	"banka1/trading-service-go/internal/platform"
	"banka1/trading-service-go/internal/portfolio"
	"banka1/trading-service-go/internal/tax"

	gpauth "banka1/go-platform/auth"
	"banka1/go-platform/rabbitmq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"github.com/shopspring/decimal"
)

// App holds the wired domain services shared by the HTTP and gRPC servers, plus
// the order-execution worker and the (optional) cron schedulers. Domains are
// added per phase (P1: analytics; P2: portfolio/actuary; P3: order; P4: tax;
// P5: funds + saga consumers + snapshot scheduler).
type App struct {
	DB             *pgxpool.Pool
	Analytics      *analytics.Service
	Portfolio      *portfolio.Service
	Actuary        *actuary.Service
	Order          *order.Service
	Tax            *tax.Service
	Funds          *funds.Service
	Holding        *funds.HoldingService
	Snapshot       *funds.SnapshotService
	Liquidation    *funds.LiquidationService
	Dividend       *funds.DividendService
	Statistics     *funds.StatisticsService
	Otc            *otc.Service
	OtcReservation *otc.ReservationService
	Interbank      *interbank.Service

	cron      *cron.Cron
	consumers []*rabbitmq.Consumer
	cancel    context.CancelFunc
	closeOnce sync.Once
}

// NewApp wires the domain layer over the database pool. jwtService mints the
// SERVICE token used by the outbound clients; cfg supplies the SERVICES_* base
// URLs and the token TTL. notifier publishes order decisions (a Noop notifier is
// fine when the broker is unavailable). sagaPublisher publishes the fund saga
// requests on SAGA_EVENTS_EXCHANGE; pass a Noop when the broker is unreachable.
// The order-execution worker is started here; the cron schedulers run only
// when their respective cfg flag is true (off during coexistence so they do
// not double-process shared rows with the Java service). When
// cfg.FundSagaConsumersEnabled is true, six fund saga result consumers are
// also started.
func NewApp(cfg platform.Config, db *pgxpool.Pool, jwtService *gpauth.Service, logger *slog.Logger, notifier order.Notifier, taxNotifier tax.Notifier, sagaPublisher funds.SagaPublisher, sagaCfg rabbitmq.Config, employeeEventsPub rabbitmq.Publisher, otcSagaPublisher otc.SagaPublisher, otcSagaCfg rabbitmq.Config) *App {
	cl := clients.New(
		jwtService,
		cfg.Services.MarketURL,
		cfg.Services.BankingCoreURL,
		cfg.Services.UserURL,
		time.Duration(cfg.JWT.ExpirationMillis)*time.Millisecond,
	)
	portfolioRepo := portfolio.NewRepository(db)
	actuaryRepo := actuary.NewRepository(db)
	orderRepo := order.NewRepository(db)
	taxRepo := tax.NewRepository(db)
	fundsRepo := funds.NewRepository(db)

	taxRate, err := decimal.NewFromString(strings.TrimSpace(cfg.TaxCapitalGainsRate))
	if err != nil {
		logger.Warn("invalid BANKA_TAX_CAPITAL_GAINS_RATE; defaulting to 0.15", "value", cfg.TaxCapitalGainsRate, "error", err)
		taxRate = decimal.RequireFromString("0.15")
	}

	// funds collaborators (holding → snapshot → statistics, then service).
	holdingSvc := funds.NewHoldingService(fundsRepo, cl.Market, logger)
	snapshotSvc := funds.NewSnapshotService(fundsRepo, holdingSvc, logger)
	statsSvc := funds.NewStatisticsService(snapshotSvc)
	fundsSvc := funds.NewService(fundsRepo, snapshotSvc, statsSvc, holdingSvc,
		cl.Market, cl.Account, cl.Employee, sagaPublisher, logger)
	liquidationSvc := funds.NewLiquidationService(fundsRepo, holdingSvc,
		cl.Market, cl.Account, snapshotSvc, logger)
	dividendSvc := funds.NewDividendService(fundsRepo, holdingSvc, snapshotSvc,
		cl.Market, cl.Account, fundsSvc, logger)

	// order: the order service depends on a FundCallback that lives in the
	// funds package. In-process binding (matches our resolved P5 decision
	// over HTTP self-call) — see funds/callback.go.
	fundCallback := funds.NewOrderCallback(fundsSvc, holdingSvc)
	orderSvc := order.NewService(orderRepo, portfolioRepo, actuaryRepo, cl, notifier, fundCallback, logger)
	orderSvc.Start()

	// tax service feeds the portfolio summary (yearlyTaxPaid/monthlyTaxDue), so it
	// is built before the portfolio service and injected as its TaxReporter.
	taxSvc := tax.NewService(taxRepo, orderRepo, portfolioRepo, actuaryRepo, cl, taxNotifier, taxRate, logger)

	// OTC (P6): the offer/contract engine + the /stocks/internal reservation
	// service. Both read/write the shared `portfolio` table via portfolioRepo. The
	// notifier resolves recipient email/name via cl.Customer, so it is built here
	// (where the clients live) from the employee.events publisher; a nil publisher
	// (broker down at startup) degrades to NoopNotifier.
	var otcNotifier otc.OtcNotifier = otc.NoopNotifier{}
	if employeeEventsPub != nil {
		otcNotifier = otc.NewRabbitNotifier(employeeEventsPub, cl.Customer, logger)
	}
	otcRepo := otc.NewRepository(db)
	otcSvc := otc.NewService(otcRepo, portfolioRepo, cl.Market, cl.Customer, cl.Employee, otcSagaPublisher, otcNotifier, logger)
	otcReservationSvc := otc.NewReservationService(db, portfolioRepo, cl.Market, logger)

	// Interbank (P7): the SERVICE-gated /internal/interbank/* 2PC primitives
	// interbank-service calls. Synchronous over the shared `portfolio` table +
	// interbank_{stock,option}_reservations — NO publisher/consumer/scheduler (the
	// inter-bank protocol + retry live in the separate interbank-service).
	interbankSvc := interbank.NewService(interbank.NewRepository(db), portfolioRepo, cl.Market, cfg.RoutingNumber, logger)

	app := &App{
		DB:             db,
		Analytics:      analytics.NewService(analytics.NewRepository(db)),
		Portfolio:      portfolio.NewService(portfolioRepo, cl.Market, cl.Account, taxSvc),
		Actuary:        actuary.NewService(actuaryRepo, cl.Employee),
		Order:          orderSvc,
		Tax:            taxSvc,
		Funds:          fundsSvc,
		Holding:        holdingSvc,
		Snapshot:       snapshotSvc,
		Liquidation:    liquidationSvc,
		Dividend:       dividendSvc,
		Statistics:     statsSvc,
		Otc:            otcSvc,
		OtcReservation: otcReservationSvc,
		Interbank:      interbankSvc,
	}

	// Schedulers are OFF by default during coexistence (Java still runs them on the
	// same rows). order: daily limit reset + 15-min auto-decline. tax: monthly
	// capital-gains collection. funds: daily fund-value snapshot. Flip the env
	// flags on at cut-over.
	if cfg.OrderSchedulersEnabled || cfg.TaxSchedulerEnabled || cfg.FundSnapshotSchedulerEnabled || cfg.OtcSchedulersEnabled {
		c := cron.New(cron.WithSeconds())
		if cfg.OrderSchedulersEnabled {
			// order: reset all actuary daily limits at 23:59 (idempotent — sets to 0).
			if _, err := c.AddFunc("0 59 23 * * *", func() {
				if err := actuaryRepo.ResetAllLimits(context.Background()); err != nil {
					logger.Error("scheduled actuary limit reset failed", "error", err)
				}
			}); err != nil {
				logger.Error("failed to register actuary reset schedule", "error", err)
			}
			// order: auto-decline expired PENDING orders every 15 minutes.
			if _, err := c.AddFunc("0 */15 * * * *", func() {
				if err := orderSvc.AutoDeclineExpiredPendingOrders(context.Background()); err != nil {
					logger.Error("scheduled auto-decline failed", "error", err)
				}
			}); err != nil {
				logger.Error("failed to register auto-decline schedule", "error", err)
			}
			logger.Info("order schedulers enabled (limit reset 23:59, auto-decline /15min)")
		}
		if cfg.TaxSchedulerEnabled {
			// tax: monthly capital-gains collection at 00:00 on the 1st (previous month).
			if _, err := c.AddFunc("0 0 0 1 * *", func() {
				if err := taxSvc.CollectMonthlyTax(context.Background()); err != nil {
					logger.Error("scheduled monthly tax collection failed", "error", err)
				}
			}); err != nil {
				logger.Error("failed to register tax collection schedule", "error", err)
			}
			logger.Info("tax scheduler enabled (monthly collection 0 0 0 1 * *)")
		}
		if cfg.FundSnapshotSchedulerEnabled {
			// funds: daily value snapshot at 00:00:10 (matches Java
			// FundValueSnapshotService @Scheduled cron 0 10 0 * * *).
			if _, err := c.AddFunc("0 10 0 * * *", func() {
				snapshotSvc.CaptureDailySnapshots(context.Background())
			}); err != nil {
				logger.Error("failed to register fund snapshot schedule", "error", err)
			}
			logger.Info("fund snapshot scheduler enabled (daily 0 10 0 * * *)")
		}
		if cfg.OtcSchedulersEnabled {
			// otc: expire overdue ACTIVE contracts + release stock at 00:00:05.
			if _, err := c.AddFunc("0 5 0 * * *", func() {
				if err := otcSvc.ExpireOverdueContracts(context.Background()); err != nil {
					logger.Error("scheduled otc expire-overdue-contracts failed", "error", err)
				}
			}); err != nil {
				logger.Error("failed to register otc expire schedule", "error", err)
			}
			// otc: send D-N expiry reminders at 08:30 (idempotent per (contract, days)).
			if _, err := c.AddFunc("0 30 8 * * *", func() {
				if err := otcSvc.SendExpiryReminders(context.Background(), cfg.OtcReminderDays); err != nil {
					logger.Error("scheduled otc expiry-reminders failed", "error", err)
				}
			}); err != nil {
				logger.Error("failed to register otc reminder schedule", "error", err)
			}
			logger.Info("otc schedulers enabled (expire 0 5 0, reminders 0 30 8)")
		}
		c.Start()
		app.cron = c
	} else {
		logger.Info("schedulers disabled (coexistence: Java owns scheduled writes); set ORDER_SCHEDULERS_ENABLED / TAX_SCHEDULER_ENABLED / FUND_SNAPSHOT_SCHEDULER_ENABLED / OTC_SCHEDULERS_ENABLED=true at cut-over")
	}

	// saga consumers — OFF by default during coexistence (Java listeners own the
	// durable trading.fund.* / trading.otc.* queues; binding from both sides would
	// round-robin deliveries → half-processed sagas). Flip to true at cut-over.
	// Funds consume from saga.exchange (sagaCfg); OTC consume from saga.events
	// (otcSagaCfg) — different exchanges, one shared cancel context.
	if cfg.FundSagaConsumersEnabled || cfg.OtcSagaConsumersEnabled {
		ctx, cancel := context.WithCancel(context.Background())
		app.cancel = cancel
		if cfg.FundSagaConsumersEnabled {
			if consumers, err := funds.StartSagaConsumers(ctx, sagaCfg, fundsSvc, logger); err != nil {
				logger.Error("fund saga consumers init failed", "error", err)
			} else {
				app.consumers = append(app.consumers, consumers...)
				logger.Info("fund saga consumers started (FUND_SAGA_CONSUMERS_ENABLED=true)", "count", len(consumers))
			}
		} else {
			logger.Info("fund saga consumers disabled (coexistence: Java owns the durable queues); set FUND_SAGA_CONSUMERS_ENABLED=true at cut-over")
		}
		if cfg.OtcSagaConsumersEnabled {
			if consumers, err := otc.StartSagaConsumers(ctx, otcSagaCfg, otcSvc, logger); err != nil {
				logger.Error("otc saga consumers init failed", "error", err)
			} else {
				app.consumers = append(app.consumers, consumers...)
				logger.Info("otc saga consumers started (OTC_SAGA_CONSUMERS_ENABLED=true)", "count", len(consumers))
			}
		} else {
			logger.Info("otc saga consumers disabled (coexistence: Java owns the durable queues); set OTC_SAGA_CONSUMERS_ENABLED=true at cut-over")
		}
	} else {
		logger.Info("fund + otc saga consumers disabled (coexistence: Java owns the durable queues); set FUND_SAGA_CONSUMERS_ENABLED / OTC_SAGA_CONSUMERS_ENABLED=true at cut-over")
	}

	return app
}

// Close stops the schedulers, fund saga consumers, and drains the execution
// worker. Call before closing the database pool so in-flight execution
// attempts finish first.
func (a *App) Close() {
	a.closeOnce.Do(func() {
		if a.cancel != nil {
			a.cancel()
		}
		for _, c := range a.consumers {
			_ = c.Close()
		}
		if a.cron != nil {
			<-a.cron.Stop().Done()
		}
		if a.Order != nil {
			a.Order.Stop()
		}
	})
}
