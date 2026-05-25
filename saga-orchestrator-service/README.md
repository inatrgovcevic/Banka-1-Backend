# saga-orchestrator-service

Go port of the Banka 1 saga orchestrator service (originally Spring Boot / Java 21).
Handles distributed transaction coordination across `banking-core-service`,
`trading-service`, and `market-service` using the SAGA pattern with LIFO compensation.

GH issues: **#213** (infrastructure), **#214** (SagaInstance + state machine), **#215** (RabbitMQ topology).

## Sagas implemented

| Saga | Trigger queue | Steps | Notes |
|---|---|---|---|
| **OtcExerciseSaga** | `saga.otc.exercise.queue` | 5 | ReserveFunds → ReserveStocks → Transfer → TransferOwnership → Consistency check; full LIFO compensation |
| **OtcPremiumTransferSaga** | `saga.otc.premium.queue` | 1 | InternalTransfer (USD→RSD via market-service) |
| **FundSubscribeSaga** | `saga.fund.subscribe.queue` | 1 | InternalTransfer client→fund account |
| **FundRedeemSaga** | `saga.fund.redeem.queue` | 1 | InternalTransfer fund→client account (liquid funds) |
| **FundRedeemWithLiquidationSaga** | `saga.fund.redeem.with-liquidation.queue` | 2 | LiquidateForFund → InternalTransfer |

## Architecture

```
RabbitMQ                    saga-orchestrator-service
  saga.otc.exercise.queue  ──►  OtcExerciseSaga handler
  saga.otc.premium.queue   ──►  OtcPremiumTransferSaga handler
  saga.fund.subscribe.queue──►  FundSubscribeSaga handler
  saga.fund.redeem.queue   ──►  FundRedeemSaga handler
  saga.fund.redeem.with-   ──►  FundRedeemWithLiquidationSaga handler
    liquidation.queue

  ◄── saga.exchange (result events published back)

Postgres (saga_db)
  saga_instance — optimistic-locked state for each running saga

Admin HTTP (port 8095)
  GET /health
  GET /saga/instances?state=&limit=&offset=
  GET /saga/instances/{id}
```

## Configuration

All env vars use the `SAGA_` prefix. Nested struct fields are **double-prefixed**
by `kelseyhightower/envconfig` (e.g. `Config.Saga.RabbitMQURL` → `SAGA_SAGA_RABBITMQ_URL`).

| Env var | Default | Description |
|---|---|---|
| `SAGA_DB_URL` | **required** | Postgres DSN |
| `SAGA_JWT_SECRET` | **required** | HS256 secret for S2S token issuance |
| `SAGA_SERVER_HTTP_PORT` | `8095` | Admin HTTP port |
| `SAGA_SERVER_MIGRATIONS_PATH` | `/migrations` | Goose SQL dir |
| `SAGA_SERVER_LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `SAGA_SERVER_LOG_JSON` | `true` | JSON vs text log format |
| `SAGA_JWT_ISSUER` | `banka1` | JWT issuer claim |
| `SAGA_PROFILE` | `prod` | `dev`/`test` relaxes secret validation |
| `SAGA_SAGA_RABBITMQ_URL` | `amqp://guest:guest@rabbitmq:5672/` | Broker URL |
| `SAGA_SAGA_SERVICES_BANKING_CORE_URL` | `http://banking-core-service:8084` | |
| `SAGA_SAGA_SERVICES_TRADING_URL` | `http://trading-service:8088` | |
| `SAGA_SAGA_SERVICES_MARKET_URL` | `http://market-service:8085` | |
| `SAGA_SAGA_SERVICES_TIMEOUT` | `30s` | REST call timeout |
| `SAGA_SAGA_CLEANUP_INTERVAL` | `15m` | Cleanup tick interval |
| `SAGA_SAGA_CLEANUP_STUCK_CUTOFF` | `1h` | Age above which IN_PROGRESS/COMPENSATING are stuck |
| `SAGA_SAGA_CLEANUP_IDEMPOTENCY_RETENTION` | `336h` | 14-day idempotency-log retention |

## Building

From the workspace root (`Banka-1-Backend-go/`):

```sh
# Local binary
cd saga-orchestrator-service
go build ./cmd/saga-orchestrator-service

# Docker image (build context = workspace root)
docker build -f saga-orchestrator-service/Dockerfile \
             -t banka1-saga-orchestrator-service:go .
```

## Running tests

```sh
cd saga-orchestrator-service
go test ./internal/... -v -count=1
go vet ./...
```

DB integration tests (`internal/store/`) are skipped unless `SAGA_DB_URL` is set.

## Notable Java to Go fixes

| Issue | Fix |
|---|---|
| Java published to `saga.events` queue name as exchange (routing bug) | Go uses `saga.exchange` topic exchange + routing key binding |
| `MarketServiceClient` had dead stock reservation methods (wrong service) | Removed; only `ConvertCurrencyNoCommission` retained |
| Java retry config properties were declared but never wired to the scheduler | Go `CleanupConfig` is fully consumed from env |
| No stuck-saga detection in Java | Go `scheduler.Scheduler` sweeps sagas older than `STUCK_CUTOFF` |
| `saga_idempotency_log` cleanup crashed if table missing | Go catches SQLSTATE 42P01 gracefully and logs once |

## Deferred

- Live RabbitMQ + Java cohort integration tests (requires running broker + Java services)
- Prometheus metrics exporter (currently using slog WARN + in-process counter)
- Mutual TLS for S2S calls
