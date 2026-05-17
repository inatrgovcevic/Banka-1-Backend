# trading-service

Konsolidovani modul uveden u **PR_02 C2.13**. Inicijalna verzija je rename
starog `order-service`. Buduce ekspanzije:

| Sub-paket | Endpoint prefix | PR | Uloga |
|---|---|---|---|
| `order/`  | `/orders/...`  | C2.13 (preneto iz order-service)  | Postojeci stock trading orders |
| `margin/` | `/margin/...`  | PR_03                              | Marzni racuni i pozajmice |
| `otc/`    | `/otc/...`     | PR_04                              | OTC trading (counter-offer flow) |
| `funds/`  | `/funds/...`   | PR_04                              | Investicioni fondovi (subscribe/redeem) |

## Zašto trading-service je odvojen

Banking-core-service i trading-service su funkcionalno razlicita subdomena:

- **banking-core**: account balance, payments, transfers, OTP — sve u jednoj
  acid transakciji (uglavnom kratke).
- **trading-service**: stock orders, margin loans, OTC offers — duzi
  background workflow-i sa SAGA pattern-om, podlozni eventual consistency.

Zadrzavanje odvojenih JVM-ova omogucava nezavisno scaling — trading sub-system
moze imati 3 replice za matching cron ili OTC saga handler-e, dok banking-core
ima 1.

## Pokretanje lokalno

```sh
# .env mora imati:
#   TRADING_DB_HOST, _PORT, _NAME, _USER, _PASSWORD
#   JWT_SECRET, RABBITMQ_*, NOTIFICATION_*, OTEL_*
#   SERVICES_USER_URL=http://user-service:8081
#   SERVICES_BANKING_CORE_URL=http://banking-core-service:8084
#   SERVICES_MARKET_URL=http://market-service:8085
docker compose -f setup/docker-compose.yml up trading-service
```

## SAGA listeners

Trading-service objavljuje saga events na exchange `saga.events`:
- `order.placed`, `order.partial_fill`, `order.fully_filled`, `order.cancelled`
- (PR_03) `margin.position.opened`, `margin.position.closed`, `margin.margin_call`
- (PR_04) `otc.offer.placed`, `otc.offer.accepted`, `otc.offer.rejected`
- (PR_04) `fund.share.subscribed`, `fund.share.redeemed`

`saga-orchestrator-service` konzumira ove i izvrsava distribuirane sagas
(npr. order.placed → banking-core.account.lock_balance →
market-service.stock.reserve → ...).
