# trading-service-go

Go re-implementation of the Java `trading-service` (including the embedded
`order-service` library) — **now the canonical `trading-service` on `:8088`**
(gRPC `:9088`), cut over from Java in P8 the same way market-service and
user-service were. It serves the full trading + order surface: `/orders` +
matching engine, `/portfolio`, `/actuaries`, `/tax`, `/funds` (+
`/funds/internal`), `/otc` (+ `/stocks/internal`), `/analytics`, and the
SERVICE-only `/internal/interbank/*` 2PC primitives — REST plus an additive
gRPC surface. The Java `trading-service` is retired from compose; the
api-gateway and interbank-service target this service unchanged.

- **Platform conventions:** [`../docs/go-platform.md`](../docs/go-platform.md)
- **Status:** P0–P8 done. All domains parity-validated against live Java (P2 6/6,
  P3 7/7, P6 6/6, P5 14/14, P7 byte-exact incl. 2PC; P4 4/7 — the 3 diffs are a
  known Java-side bug). **P8 cut-over (big-bang): promoted to the canonical
  `trading-service` on :8088** (mirrors market/user-go) — it now owns the `trading`
  schema via its own migrations, and the Java `trading-service` is retired from compose.
  Serves `/actuator/*`, `/analytics/*` (P1), the full `/portfolio` + `/actuaries`
  surface (P2), the full `/orders` surface + matching engine (P3), the `/tax`
  capital-gains surface + monthly scheduler (P4), the full `/funds` (+
  `/funds/internal`) surface + saga publisher + (gated) saga consumers +
  (gated) daily snapshot scheduler (P5), the full `/otc` + `/stocks/internal`
  surface + offer/contract state machines + premium/exercise saga publisher +
  (gated) saga consumers + (gated) expire/reminder schedulers (P6), and the
  `/internal/interbank/*` SERVICE surface — stock + option 2PC primitives,
  synchronous (no saga/scheduler) (P7). P8 promoted this onto :8088 as the canonical
  trading-service (api-gateway + interbank-service target it unchanged; the go-trading
  overlay on :18088 is now a vestigial alt-port validation instance). gRPC: `Ping` +
  `GetLatestAnalyticsRun` (additive; not extended to P2–P7 reads).

> **Coexistence gates — now ON.** Six env gates fence the scheduled + saga work
> that would otherwise double-process rows the Java service also scanned during
> coexistence: `ORDER_SCHEDULERS_ENABLED`, `TAX_SCHEDULER_ENABLED`,
> `FUND_SAGA_CONSUMERS_ENABLED`, `FUND_SNAPSHOT_SCHEDULER_ENABLED`,
> `OTC_SAGA_CONSUMERS_ENABLED`, `OTC_SCHEDULERS_ENABLED`. They default **off** in
> code, but the canonical `setup/docker-compose.yml` sets all six **on** as of the
> P8 cut-over (Java retired → nothing to double-process). The per-phase notes
> below still describe the *off-during-coexistence* rationale — that is why each
> gate exists; set any back to `false` to hand that job to another instance.

### Implemented endpoints

| Phase | Endpoints |
|---|---|
| P1 | `GET /analytics/{runs/latest,clients/segments,users/{id}/portfolio-risk,tickers/top}` |
| P2 | `GET /portfolio`, `PUT /portfolio/{id}/set-public`, `POST /portfolio/{id}/exercise-option`, `GET /actuaries/agents`, `PUT /actuaries/agents/{id}/{limit,reset-limit,need-approval}`, `GET /actuaries/profit`, `GET /actuaries/profit/bank-summary` |
| P3 | `POST /orders/{buy,sell}`, `GET /orders`, `GET /orders/my-orders`, `POST /orders/{id}/{confirm,cancel}`, `PUT /orders/{id}/{cancel,approve,decline}` |
| P4 | `POST /tax/collect`, `POST /tax/collect/current-month`, `POST /internal/tax/capital-gains/run`, `GET /tax/capital-gains/debts`, `GET /tax/capital-gains/{userId}`, `GET /tax/tracking` |
| P5 | `GET /funds`, `GET /funds/{id}`, `GET /funds/{id}/{analytics,securities,performance,positions,transactions}`, `GET /funds/{supervised,my-positions,bank-positions,my-transactions}`, `POST /funds`, `POST /funds/{id}/{invest,redeem,bank-invest,bank-redeem}`, `POST /funds/{id}/securities/{ticker}/sell`, `POST /funds/{id}/dividends`, `PATCH /funds/admin/reassign-manager`, `POST /funds/internal/{fundId}/{liquidate,holdings/add,liquidity/debit}` |
| P6 | `POST /otc/offers`, `POST /otc/offers/{offerId}/{counter,accept,reject,withdraw}`, `GET /otc/offers/{active,history}`, `GET /otc/public-stocks`, `POST /otc/contracts/{contractId}/exercise`, `GET /otc/contracts/my`, `GET /otc/my-positions`, `POST /otc/positions`, `PUT /otc/positions/{positionId}`, `DELETE /otc/positions/{positionId}`, `POST /stocks/internal/reserve`, `DELETE /stocks/internal/reservations/{id}`, `POST /stocks/internal/reservations/{id}/transfer`, `POST /stocks/internal/ownership-transfers/{id}/reverse` |
| P7 | `POST /internal/interbank/reserve-stock`, `POST /internal/interbank/reservations/{id}/commit-stock`, `DELETE /internal/interbank/reservations/{id}`, `POST /internal/interbank/options/{negotiationId}/reserve`, `POST /internal/interbank/options/{negotiationId}/exercise`, `DELETE /internal/interbank/options/{negotiationId}/release`, `GET /internal/interbank/public-stocks` (all **SERVICE**) |

P2/P3 call three sibling services with a minted **SERVICE** JWT (no caller-bearer
propagation, matching order-service): market-service (listing/price/FX/exchange
status/refresh), banking-core account-service (account details + exchangeBuy/Sell
+ transfers), user-service (employees). See `internal/clients`.

### P3 — order matching engine & schedulers

- **Execution worker** (`internal/order/worker.go` + `execution.go`): the Go
  equivalent of the Java `ThreadPoolTaskScheduler` (pool 4). A confirmed/approved
  order is scheduled `+60s`, then each portion executes under `FOR UPDATE` row
  locks (order → portfolio → actuary) in one `RunInTx`, reproducing the random
  fill size, the `Random(0, 1440/(volume/remaining))`-sec delay (`+30min`
  after-hours), STOP→MARKET / STOP_LIMIT→LIMIT mutation, the `0.14`/`0.24`
  commission with `$7`/`$12` USD caps, weighted-average buy cost, and the
  `reserved_limit`→`used_limit` move. In-memory only (like Java): pending
  attempts are lost on restart.
- **`purchaseFor` scope:** standard (own portfolio) and **BANK** orders are
  supported; **INVESTMENT_FUND** orders are rejected with 409 (deferred to P5,
  where the funds domain + `TradingServiceClient` liquidity/holdings callbacks
  live). The `purchase_for`/`fund_id` columns and DTO fields are carried so the
  schema and responses still match.
- **Event delivery:** `order.approved` / `order.declined` publish to
  `employee.events` **after** the decision transaction commits (publish-after-
  commit; no outbox table — `trading` schema stays Java-owned). A missing broker
  falls back to a Noop publisher so the service still boots.
- **Schedulers** (`ORDER_SCHEDULERS_ENABLED`, default **off**): daily actuary
  limit reset (`0 59 23 * * *`) and 15-min expired-PENDING auto-decline
  (`0 */15 * * * *`). Off during coexistence so they do not double-process the
  shared rows the Java service still scans; flip on at cut-over.

### P3 parity / verify-against-live-Java TODOs

- **Create-request bean-validation messages** (`validateCreateBuy`/`Sell` in
  `internal/http/order_handlers.go`) — the Hibernate `@NotNull`/`@Positive`
  default strings are version-sensitive; confirm and adjust.
- **Mutating endpoints** (`buy/sell/confirm/cancel/approve/decline`) write shared
  `trading` rows, so they are **not** in the auto sweep — verify manually in
  isolation against a throwaway dataset (response body, status, resulting rows).
- `GET /orders` overview ordering and the Spring `Page<>` envelope; `GET
  /orders/my-orders` needs a CLIENT token (separate from the SUPERVISOR sweep).

### P2 parity / verify-against-live-Java TODOs

Reproduce the consolidated JVM's behavior; confirm these against the running Java
`trading-service` (port 8088) during the parity sweep and adjust if they differ:

- **Spring `Page<>` JSON** (`internal/api/page.go`) — the exact PageImpl envelope
  (pageable/sort sub-objects) is Spring-version-sensitive.
- **Two error-body shapes**: order-module exceptions → `ApiErrorResponse`
  `{timestamp,status,error,message,path,fieldErrors?}`; but
  `IllegalArgumentException` (all `/actuaries` admin/role/limit validations) →
  **404** via trading-service `OtcExceptionHandler` `{status,error,message,timestamp}`
  (it is `@Order(HIGHEST_PRECEDENCE)` and global). See `internal/api/errors.go`.
- **Bean-validation messages** (`validateLimit` in `internal/http/actuary_handlers.go`)
  — the Hibernate `@DecimalMin`/`@NotNull` default strings.
- **Tax coupling (resolved in P4)**: `GET /portfolio` `yearlyTaxPaid`/`monthlyTaxDue`
  are now served by the tax service (`portfolio.TaxReporter`), no longer stubbed. As
  in Java, `monthlyTaxDue` uses strict FX, so a conversion failure makes `/portfolio`
  return 409 (OTC error shape).

### P4 — tax (capital gains)

- **FIFO cost basis** (`internal/tax/service.go`): per (user, listing), SELL
  transactions match against the oldest BUY lots (a deque); `gain = (sellPPU −
  buyPPU) × matched`, only positive gains taxed, `tax = gain × rate` (rate
  `BANKA_TAX_CAPITAL_GAINS_RATE`, default `0.15`) at scale 2 HALF_UP. A sell with no
  buy history uses `portfolio.average_purchase_price` as cost basis and writes the
  charge with `buy_transaction_id = -1`.
- **OTC capital gains**: a raw-SQL JOIN over the trading-service-owned
  `option_contracts` + `stock_ownership_transfers` + `portfolio` (Java still writes
  these during coexistence; the tax pass only reads them). `profit = (price_per_stock
  − avg) × amount`, 15%, USD→RSD; idempotent via the partial unique index on
  `otc_contract_id`.
- **Ledger + idempotency**: this service writes `tax_charges` (RESERVED→CHARGED; a
  failed debit DELETES the reservation, it is not marked FAILED). Idempotency is
  insert-then-skip-on-conflict via `uk_tax_charges_sell_buy` /
  `uk_tax_charges_otc_contract`, never check-then-insert.
- **Collection** (`POST /tax/collect`, `/tax/collect/current-month`,
  `POST /internal/tax/capital-gains/run`): reserve → account `transaction` → mark
  CHARGED, then publish `tax.collected` to `employee.events` (publish-after-charge).
  No surrounding DB transaction — each ledger write is its own commit, like the Java
  auto-commit-per-save flow.
- **Scheduler** (`TAX_SCHEDULER_ENABLED`, default **off**): monthly previous-month
  collection (`0 0 0 1 * *`). Off during coexistence so it does not double-process the
  rows the Java `TaxScheduler` still scans (idempotent either way); flip on at
  cut-over. The manual POST endpoints are always available to a supervisor.
- **Auth**: `/tax/*` is SUPERVISOR; `/internal/tax/capital-gains/run` is SERVICE.

### P4 parity / verify-against-live-Java TODOs

- **`/tax/capital-gains/debts` content ordering**: Java sums per-user debt in a
  `HashMap` and pages its arbitrary iteration order; the Go port orders by `userId`
  ascending. Values match; multi-user page ordering may differ — confirm/accept
  during the sweep.
- **`debtRsd` is a raw cross-currency sum**: `getAllDebts`/`getUserDebt` sum the
  per-trade `taxAmount` in the security's original currency with **no** FX and **no**
  OTC component (a faithful Java quirk despite the field name); the RSD figures are
  on the `/tax/tracking` rows.
- **Mutating endpoints** (`/tax/collect*`, `/internal/tax/...`) write shared rows +
  call account-service, so they are **not** in the auto sweep — verify manually in
  isolation (resulting `tax_charges` rows, the state-account debit, the
  `tax.collected` message).
- **TZ assumption**: period math uses UTC; parity with Java's
  `ZoneId.systemDefault()` holds when the containers run UTC (the default) — confirm
  the deployment TZ during the sweep.
- **Notification payload**: `tax.collected` omits `senderBalance`/`receiverBalance`
  (the Go account client discards the transaction response body); notifications are
  side-effects, not parity-gated.

`parity.endpoints.p4.json` covers only the **safe GET** P4 reads (debts, tracking,
user-debt). Run it with a **SUPERVISOR** token.

## Ports & data

| | |
|---|---|
| REST | `8088` (`SERVER_PORT`) — canonical `trading-service` since the P8 cut-over. (The optional `go-trading` validation overlay runs a *second* instance on `18088`.) |
| gRPC | `9088` (`GRPC_PORT`) — overlay instance on `19088`. |
| Database | shared `trading` Postgres (`TRADING_DB_*`). **As of P8 this service owns the schema** and runs its own migrations (`migrations/`): on a fresh DB it creates the schema (+ dev fixtures when `LIQUIBASE_CONTEXTS=dev`); on a DB Java Liquibase already provisioned it baseline-skips. |
| Module | `banka1/trading-service-go` (uses `replace banka1/go-platform => ../go-platform`) |

## Schema & migrations (P8)

As of the P8 cut-over this service **owns the `trading` schema**.
`internal/platform/migrations.go` (`RunMigrations`, ported from market-service-go) applies
`migrations/*.sql` in lexical order at boot, tracking applied files in `go_schema_migrations`:

- **`001_trading_baseline.sql`** — the full schema (24 tables + sequences + constraints +
  indexes), generated from `pg_dump --schema-only` of the live Java-Liquibase `trading` DB.
  **DO NOT hand-edit; regenerate** from `pg_dump` if the schema changes.
- **`002_devseed_trading.sql`** — the Java `context:dev` fixtures (trading-otc/003..010):
  public-stock portfolios, the Mile interbank position, intra-bank OTC offers + one active
  contract, and the RSD investment funds. **Gated**: applied only when `LIQUIBASE_CONTEXTS`
  contains `dev` (compose default); set a non-dev value for production.

**Baseline-skip:** on a DB Java Liquibase already provisioned (sentinels `portfolio` + `orders`
present and `go_schema_migrations` empty), every migration is recorded as applied *without
running it*, so the Go service never collides with the existing tables. On a fresh DB the
migrations run and create everything. The Dockerfile ships `migrations/` to `/app/migrations`.

## Build & validate

There is no requirement for a local Go toolchain — build via Docker (the build
context is the **repo root** so the sibling `go-platform/` module resolves):

```powershell
# compile + vet inside the pinned Go image (repo root mounted)
docker run --rm -v "${PWD}\..:/workspace" -w /workspace/trading-service-go `
  golang:1.25 sh -lc "go mod tidy && go build ./... && go vet ./..."

# build the runtime image (run from repo root)
docker build -t trading-service-go-local -f trading-service-go/Dockerfile ..
```

## Run (compose)

Since the P8 cut-over this **is** the `trading-service` — it comes up with the
normal stack, no profile, on `:8088`:

```powershell
docker compose --env-file .\setup\.env -f .\setup\docker-compose.yml up -d --build trading-service

curl http://localhost:8088/actuator/health/liveness   # {"status":"UP"}
curl http://localhost:8088/actuator/info              # {}
```

The api-gateway and interbank-service target `trading-service:8088` unchanged
(now Go); the Java `trading-service` is no longer built.

### Optional: side-by-side validation instance (`go-trading` overlay)

`setup/docker-compose.go-trading.yml` starts a *second* instance on `:18088`
(gRPC `:19088`) under the `go-trading` profile — vestigial now that Java is
retired, but still handy for an isolated check:

```powershell
docker compose --env-file .\setup\.env `
  -f .\setup\docker-compose.yml `
  -f .\setup\docker-compose.go-trading.yml `
  --profile go-trading up -d --build trading-service-go

curl http://localhost:18088/actuator/health/liveness   # {"status":"UP"}
```

## Healthcheck

The runtime image is `distroless` (no shell, no curl). The container healthcheck
execs the binary itself: `trading-service-go -healthcheck`, which probes
`/actuator/health/liveness` on `SERVER_PORT` and exits 0 (UP) or 1 (down).

## gRPC / proto

Stubs are committed so consumers need no `protoc`. Regenerate after editing
`proto/trading/v1/*.proto`:

```powershell
.\scripts\generate-proto.ps1   # docker-based buf generate (pinned bufbuild/buf:1.46.0)
```

## Parity

`cmd/paritycheck` diffs this service against the live Java service per endpoint
(status + normalized JSON), per phase via the `parity.endpoints*.json` files.
This was the migration-time harness — Java on `8088` and Go on `18088` running
side by side. Since the P8 cut-over Java is retired, so the commands below are
the historical record of how each phase was signed off (re-run them against a
`go-trading` overlay instance on `18088` if you ever need to re-validate).

```powershell
# P1 (analytics + actuator) — needs an ADMIN/SUPERVISOR/AGENT/SERVICE token
go run ./cmd/paritycheck -java-base http://localhost:8088 -go-base http://localhost:18088 `
  -endpoints-file .\parity.endpoints.example.json -token "<JWT>"

# P2 (portfolio + actuary safe reads) — needs a SUPERVISOR token
go run ./cmd/paritycheck -java-base http://localhost:8088 -go-base http://localhost:18088 `
  -endpoints-file .\parity.endpoints.p2.json -token "<SUPERVISOR_JWT>"
```

`parity.endpoints.p2.json` covers only the **safe GET** P2 reads. The mutating
endpoints (`set-limit`, `reset-limit`, `need-approval`, `set-public`,
`exercise-option`) write shared `trading` rows, so they are **not** in the auto
sweep — running them against both Java and Go would double-apply. Verify those
manually in isolation (or against a throwaway dataset), confirming the response
body, status code, and the resulting row state match Java.

### P5 — funds (investment funds + saga + dividends)

- **Domains** (`internal/funds/*`): the seven trading-service-owned tables
  (`investment_funds`, `client_fund_positions`, `client_fund_transactions`,
  `fund_holdings`, `fund_value_snapshots`, `fund_dividend_distributions`,
  `fund_dividend_payouts`). Java Liquibase still owns the schema; this service
  runs no migrations.
- **NAV / position math** (`service.go`): `totalValue = liquidnaSredstva +
  sum(holding.qty × livePrice USD→RSD)`; `denominator = max(totalFundInvested,
  fundValue)` for percentageOfFund (matches the "orphaned assets" guard in the
  Java code); decimals at scale 2 RSD / 4 prices / 8 ratios with HALF_UP.
- **Statistics** (`statistics.go`): `annualizedReturn`, `volatility`,
  `rewardToVariabilityRatio`, `maxDrawdown` deliberately computed in `float64`
  (Math.pow / Math.sqrt parity) behind a hard **≥12 monthly snapshots** gate;
  below the threshold the four fields render `null`.
- **Dividend split** (`dividend.go`): REINVEST buys whole shares at the live
  price (rounding remainder stays in liquidity); PAYOUT_CLIENTS proportionally
  splits with the **last-client-remainder** rule — the last position absorbs
  `grossRsd − sum(others)` so the totals reconcile to the cent. Idempotent via
  unique `(fund_id, stock_ticker, payment_date)`; duplicate call → OTC 409.
- **Liquidation** (`liquidation.go`): saga FUND_LIQUIDATION_FOR_REDEMPTION step
  1 — sell down active holdings largest-first until cumulative RSD proceeds ≥
  target; live market price (avgUnitPrice fallback per ticker); fund liquidity
  + bank account credited; new UUID-stamped liquidationId returned.
- **Saga publisher** (`saga.go`): a SECOND `go-platform/rabbitmq.Publisher`
  dedicated to `SAGA_EVENTS_EXCHANGE` (default `saga.events`) publishes
  `fund.subscribe.requested`, `fund.redeem.requested`, and
  `fund.redeem.with-liquidation.requested` **after** the local invest/redeem
  transaction commits. Mirrors Java
  `TransactionSynchronizationManager.afterCommit` — no outbox table (the
  `trading` schema stays Java-owned). Broker outage → NoopSagaPublisher; the
  client_fund_transaction row stays `PENDING` so a supervisor can act.
- **Saga consumer** (`saga.go`, gated): the FIRST Go saga *consumer* in this
  service. Bound to `SAGA_RESULTS_EXCHANGE` (default `saga.exchange`), six
  durable queues (3 sagas × {success, failure}) → `Service.CompleteInvest` /
  `CompleteRedeem` / `FailTransaction`. The complete callbacks are idempotent
  (no-op once the tx left PENDING) so a duplicate delivery is harmless. **OFF
  by default** (`FUND_SAGA_CONSUMERS_ENABLED=false`) — during coexistence the
  Java listeners own the same durable queues; binding from both sides would
  round-robin deliveries → half-processed sagas. Flip to true at cut-over.
- **Snapshot scheduler** (gated): daily `0 10 0 * * *` (00:00:10) full
  per-fund value snapshot — captured (liquidity, holdings, total) into
  `fund_value_snapshots` upserting on `(fund_id, snapshot_date)`. **OFF by
  default** (`FUND_SNAPSHOT_SCHEDULER_ENABLED=false`) during coexistence; flip
  on at cut-over. The manual snapshot writes (every fund-mutating service
  path) keep history flowing in the meantime.
- **INVESTMENT_FUND order branch re-activated**: `internal/order` now accepts
  `purchaseFor=INVESTMENT_FUND` BUY orders again (P3 deferral resolved). The
  fund's RSD account is the funding source; on execution the shares land in
  the fund's `fund_holdings` (not the user's portfolio) and the cached
  liquidity mirror is updated via the **in-process `FundCallback`** (no HTTP
  self-call — mirrors `funds.ServiceCallback` directly). Same flow as Java
  `notifyFundLiquidityDebit` + `notifyFundHolding`, but via interface.

#### P5 parity / verify-against-live-Java TODOs

- **Mutating endpoints** (`POST /funds`, `POST /funds/{id}/{invest,redeem,
  bank-invest,bank-redeem,securities/{ticker}/sell,dividends}`, `PATCH
  /funds/admin/reassign-manager`, all `/funds/internal/*`) write shared
  `trading` rows and may drive sagas, so they are **not** in the auto sweep —
  verify manually in isolation. Especially the dividend distribution payouts
  (PAYOUT_CLIENTS last-client-remainder must match Java to the cent).
- **Saga event payload shapes**: Java publishes records (`record
  FundSubscribeRequestedEvent`) via `RabbitTemplate.convertAndSend`. Confirm
  the on-wire JSON matches saga-orchestrator-service's deserializer (the Go
  port emits the same field set — camelCase, no `class`/type wrapper).
- **Statistics scale**: `annualizedReturn` etc. render at scale 4 (matches
  Java `scaleOrNull` → `BigDecimal.setScale(4, HALF_UP)`). Confirm against a
  fund with ≥12 monthly snapshots seeded.
- **AccountID extraction in FundDto**: Java derives `accountId` from
  `accountServiceClient.getByNumber(...).id()`; the Go `clients.AccountDetails`
  shape does not currently expose `id`, so `accountId` renders `null`.
  Acceptable for read parity (`accountId` is informational), but the field
  should be added if a downstream consumer depends on it.
- **TZ assumption**: snapshot dates are recorded UTC-truncated; parity with
  Java holds when the containers run UTC (the default).

`parity.endpoints.p5.json` covers only the **safe GET** P5 reads
(discovery / details / analytics / securities / performance / supervised /
my-positions / bank-positions / positions / transactions / my-transactions).
Run it with a SUPERVISOR token; `/funds/my-positions` and
`/funds/my-transactions` need a CLIENT_TRADING token (separate sweep).

### P6 — OTC (options trading between users + saga)

- **Domains** (`internal/otc/*`): the four trading-service-owned tables
  `otc_offers`, `option_contracts`, `otc_negotiation_history`,
  `otc_contract_expiry_reminders`, plus `stock_reservations` /
  `stock_ownership_transfers`, plus the **shared `portfolio`** table (read/write
  via `internal/portfolio`). Java Liquibase still owns the schema; no migrations.
- **Auth**: `/otc/*` is **authenticated** (any valid JWT — `OtcController` has no
  `@PreAuthorize`; wrapped in the JWT middleware with no role gate, so there is
  never a 403). `/stocks/internal/*` is **PUBLIC** (permit-all; saga-orchestrator
  calls it with no JWT — registered with no auth middleware, like `/actuator/*`).
- **Offer state machine** (`service.go`): `PENDING_SELLER ⇄ PENDING_BUYER`
  (counter flips the turn) → `ACCEPTED` / `REJECTED` / `WITHDRAWN` / `EXPIRED`.
  Every transition takes the offer row `FOR UPDATE` + a status guard, reproducing
  Java 1:1 — including the quirks: `counter` is allowed on a `WITHDRAWN` offer
  (only ACCEPTED/REJECTED/EXPIRED are final), `reject` has no status guard,
  `accept`'s participant message has a trailing period where `counter`'s does not.
- **Contract state machine**: `PENDING_PREMIUM` → `ACTIVE` (premium saga ok) →
  `EXERCISED` (exercise saga ok), or → `EXPIRED` (settlement-date cron) / →
  `CANCELED` (premium saga failed). The saga-completion transitions are idempotent
  (no-op once the contract left the source state) so duplicate deliveries are safe.
- **Reserved-stock invariant on accept** (parity-critical): under the offer lock,
  `sum(seller's ACTIVE+PENDING_PREMIUM contract amounts for the ticker) +
  offer.amount ≤ seller-owned`, where seller-owned counts `publicQuantity` for an
  exposed position else full `quantity` (resolved via the market client). A
  violation throws **`InsufficientPublicStockException` → 400** (the OTC body
  shape, *not* 409). On success an `OptionContract` (PENDING_PREMIUM) is created
  and the seller's `portfolio` is reserved (`reserved += amount`, `public =
  max(0, public-amount)`).
- **Saga** (`saga.go`): publishes `otc.premium.transfer.requested` (on accept) and
  `otc.exercise.requested` (on exercise) on `SAGA_EVENTS_EXCHANGE` (`saga.events`)
  **after the local transaction commits** (mirrors Java
  `TransactionSynchronizationManager.afterCommit`; no outbox — the `trading` schema
  stays Java-owned; broker outage → `NoopSagaPublisher` and the contract stays in
  its intermediate state for a supervisor to see). Reuses the **same** `saga.events`
  publisher the P5 funds saga uses (no funds code touched). The three result
  consumers (`trading.otc.{premium.completed,premium.failed,exercise.completed}` ←
  `otc.{premium.transfer.completed,premium.transfer.failed,exercise.completed}`)
  also bind `saga.events` (note: funds results bind `saga.exchange`). **OFF by
  default** (`OTC_SAGA_CONSUMERS_ENABLED=false`) — during coexistence the Java
  `@RabbitListener`s own those durable queues; binding from both sides would
  round-robin deliveries → half-processed contracts. Flip on at cut-over.
- **`/stocks/internal` reservation service** (`reservations.go`): `reserve` /
  release / `transfer` / `reverse` over `stock_reservations` +
  `stock_ownership_transfers` + `portfolio`, each in one `RunInTx`. Reservation/
  transfer ids are crypto/rand UUIDv4 strings (matches Java `UUID.randomUUID()`).
  release/reverse are idempotent (missing/terminal row → no-op or UNKNOWN). The
  `otc-exercise-`-prefixed `correlationId` consumes the accept-time reservation
  instead of double-reserving. Portfolio rows are taken `FOR UPDATE` in a
  deterministic seller→buyer order (a Go-port hardening; Java holds no portfolio
  lock here, only `@Version`-less rows — the lock changes no observable result).
- **Schedulers** (`scheduler.go`, gated `OTC_SCHEDULERS_ENABLED`, default **off**):
  `0 5 0 * * *` expires overdue ACTIVE contracts + releases the seller's stock,
  `0 30 8 * * *` sends D-`OTC_CONTRACT_EXPIRATION_NOTIFICATION_DAYS` (default 3)
  expiry reminders. Reminder idempotency is race-safe `INSERT … ON CONFLICT DO
  NOTHING` on `uk_otc_contract_expiry_reminder` (send only when the insert wins),
  replacing Java's check-then-insert. Off during coexistence — Java runs the same
  jobs on the same rows.
- **Notifications** (`notify.go`): `otc.{countered,accepted,canceled,
  expiry_reminder}` on `NOTIFICATION_EXCHANGE` (`employee.events`), recipient
  email/name resolved via the user-service customer client. Best-effort, published
  after commit (Java publishes inline within the transaction — strictly safer here;
  notifications are not parity-gated).
- **Mutations are NOT gated**: matching the P3 order / P5 funds precedent (both
  also write shared `portfolio`), the OTC mutating endpoints are live. During
  coexistence this was safe because the shared `portfolio` writes are correct
  under Postgres `FOR UPDATE` (cross-process) even while Java also served traffic;
  since the P8 cut-over this is the canonical service, so they are simply the live
  path. Only the saga consumers + schedulers were gated (now on — see the note
  near the top).

#### P6 parity / verify-against-live-Java TODOs

- **Mutating endpoints** (`/otc/offers*`, `/otc/contracts/*/exercise`,
  `/otc/positions*`, all `/stocks/internal/*`) write shared `trading` rows and may
  drive sagas, so they are **not** in the auto sweep — verify manually in isolation
  (response body, status, resulting `otc_offers` / `option_contracts` /
  `portfolio` / `stock_*` rows, and the published saga/notification messages).
  Especially confirm the `portfolio` invariant `available = quantity −
  reserved_quantity` holds after accept / exercise / expire.
- **`modifiedBy` on create/counter** comes from the JWT `name` claim, read by
  peeking the (already-verified) bearer token. If live tokens carry no `name`
  claim, `modifiedBy` renders `null` (matches Java's null claim) — confirm the
  real token shape during the sweep.
- **Bean-validation messages** (`validateOfferTerms` / `validatePublicQuantity` in
  `internal/http/otc_handlers.go`) use approximate Hibernate-default strings — the
  exact `@DecimalMin`/`@NotNull`/`@Future`/`@Min` messages are version-sensitive
  (same caveat as the P3/P5 create-request validations).
- **`myContracts` / `public-stocks` / `history` ordering**: the Go queries omit
  `ORDER BY` exactly where Java's derived queries do, so against the same Postgres
  the row order matches; `history` is `changed_at DESC`. Confirm during the sweep.
- **Saga event payload shapes**: Java publishes records
  (`OtcPremiumTransferRequestedEvent` etc.) via `RabbitTemplate.convertAndSend`.
  The Go port emits the same camelCase field set with no type wrapper — confirm
  against saga-orchestrator-service's deserializer.
- **TZ assumption**: the expire/reminder settlement-date math truncates to the UTC
  calendar date (`::date` cast); parity holds when the containers run UTC.

`parity.endpoints.p6.json` covers only the **safe GET** P6 reads
(offers/active, offers/history, public-stocks, contracts/my [+ ?status=ACTIVE],
my-positions). Run it with any authenticated token (the reads are user-scoped by
the JWT `id`, so use the same token against Java and Go).

### P7 — interbank (inter-bank 2PC stock + option primitives)

- **Domain** (`internal/interbank/*`): the SERVICE-gated `/internal/interbank/*`
  endpoints interbank-service calls — via `TradingInternalClient` — to reserve /
  commit / release this bank's stock and OTC-option holdings during the Tim 2
  inter-bank 2PC protocol. Two tables — `interbank_stock_reservations`
  (order-service Liquibase; `reservation_id` UUID) and
  `interbank_option_reservations` (trading-service Liquibase; `negotiation_id`
  PK) — plus the **shared `portfolio`** table. Mirrors
  `com.banka1.tradingservice.interbank.*`. Java Liquibase owns the schema; no
  migrations here.
- **Scope boundary**: the 2PC *protocol* (negotiations, retry scheduler, mock
  bank-2, outbound OTC wrapper, X-Api-Key auth) lives in the **separate**
  `interbank-service` microservice and is **out of scope** — not migrated. This
  package only exposes the synchronous primitives that service invokes. There is
  **no RabbitMQ, saga, or scheduler** on the trading side (the smallest phase).
- **Auth**: all seven routes are **SERVICE** only (`@PreAuthorize("hasRole('SERVICE')")`
  → `orderSecured(jwtService, serviceOnly, …)`). Unlike P6's PUBLIC
  `/stocks/internal/*`, interbank-service sends a SERVICE JWT.
- **Stock 2PC** (`service.go`, parity-critical): each is one `RunInTx`.
  - `reserve-stock` → `reserved_quantity += qty` (quantity untouched), HELD row,
    returns a new UUID. Resolves the seller position by ticker through the market
    client, takes it `FOR UPDATE`, enforces `available = quantity −
    reserved_quantity ≥ qty`.
  - `commit-stock` → `quantity −= qty` **and** `reserved_quantity −= qty` (both
    floored at 0), COMMITTED. Idempotent (repeat on COMMITTED → no-op).
  - release (`DELETE /reservations/{id}`) → `reserved_quantity −= qty` **only**
    (quantity untouched), RELEASED. Idempotent (repeat on RELEASED → no-op).
  - Wrong-terminal transitions raise **409**: commit on RELEASED ("Cannot commit
    reservation X in state RELEASED"), release on COMMITTED ("Cannot release
    reservation X — already COMMITTED"). Reserve failures (no position /
    insufficient) and unknown reservation raise **404** — all exact Java messages.
- **Option lifecycle** (thin wrapper over the stock 2PC, keyed by `negotiationId`,
  persisted in `interbank_option_reservations` — was an in-memory map before
  PR_34, so a restart orphaned shares): `reserve` is idempotent (existing
  negotiation → 204 no-op, does **not** re-reserve), `exercise` → commitStock +
  EXERCISED, release → releaseStock + RELEASED; all three terminal-guard to a
  204 no-op on a missing / already-terminal row. `sellerForeignId` strips the
  `C-`/`E-` prefix to the numeric userId (non-numeric / null → 404).
- **public-stocks** (`GET /public-stocks`): every advertised public STOCK position
  grouped by ticker (insertion order over the same Postgres rows as Java's
  `LinkedHashMap`), each seller tagged `{routingNumber, "C-"+userId}` where
  `routingNumber = BANKA1_ROUTING_NUMBER` (default `111`). Ticker resolved via the
  market client; unresolved positions are skipped.
- **`FOR UPDATE` hardening**: reserve locks the portfolio row by (user, listing);
  commit/release lock it by id (`portfolio.FindByIDForUpdate`, P7-new) — Java
  fetches by id without a lock there. Each op locks exactly one row, so there is
  no lock-ordering hazard (unlike the OTC seller→buyer transfer).
- **Mutations are NOT gated** (matches the P3 order / P5 funds / P6 OTC precedent —
  all write the shared `portfolio` under `FOR UPDATE`). Since the P8 cut-over
  interbank-service targets this service (`services.trading.url` →
  `trading-service:8088`, now Go) unchanged, so the `/internal/interbank/*`
  primitives are now the live inter-bank 2PC path.

#### P7 parity / verify-against-live-Java TODOs

- **Mutating endpoints** (`reserve-stock`, `commit-stock`, `DELETE
  /reservations/{id}`, all `/options/{negotiationId}/*`) write the shared
  `portfolio` + interbank reservation rows and drive 2PC, so they are **not** in
  the auto sweep — verify manually in isolation (response body + status, the
  resulting `interbank_stock_reservations` / `interbank_option_reservations` /
  `portfolio` rows, and the invariant `available = quantity − reserved_quantity`
  after reserve / commit / release).
- **Exact OTC error messages/codes**: IllegalArgument → **404**, IllegalState →
  **409** via the global `OtcExceptionHandler` (`{status,error,message,timestamp}`).
  The Go port reproduces the exact strings ("No portfolio position for user=X
  ticker=Y", "Insufficient stock for reservation: have=A need=B (ticker=T)",
  "Cannot commit reservation X in state S", "Cannot release reservation X —
  already COMMITTED", "Invalid sellerForeignId, expected numeric or 'C-N'/'E-N'
  format: X"). Confirm during the sweep.
- **Malformed/empty body & non-UUID path id**: the handler maps malformed JSON to
  the order 400 shape; an unparseable `{id}` fails the Postgres `::uuid` cast →
  500 (Java's `@PathVariable UUID` would 400 earlier). interbank-service always
  sends valid bodies + UUIDs, so these edges are not exercised in practice.

`parity.endpoints.p7.json` covers only the **safe GET** `/internal/interbank/public-stocks`.
It is **SERVICE-gated**, so run the sweep with a **SERVICE token** (mint via
user-service login or `GenerateServiceToken`; see `docs/go-platform.md`) against
both Java (`8088`) and Go (`18088`).
