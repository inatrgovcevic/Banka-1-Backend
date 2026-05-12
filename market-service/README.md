# market-service

Konsolidovani modul uveden u **PR_02 C2.8** koji zamenjuje stari `stock-service` i
`exchange-service`. Servira oba REST ugovora iz iste JVM instance:

| Endpoint prefix | Pakovanje | Stari servis |
|---|---|---|
| `/stocks/...`   | `com.banka1.marketservice.stock`    | `stock-service` |
| `/exchange/...` | `com.banka1.marketservice.exchange` | `exchange-service` |

## Zašto konsolidacija

- Stock-service je čitao TwelveData FX kurseve preko REST poziva ka exchange-service-u
  za konverziju cena u RSD; konsolidacija eliminiše ovaj hop.
- Oba servisa imaju iste OTEL / RabbitMQ / Liquibase konfiguracije.
- Ušteda RAM-a: ~400 MB (2 PostgreSQL + 2 JVM kontejnera → 1 + 1).
- Cypress test `securities.cy.ts` udara `/stocks/...` rute koje sad nemaju
  cross-modul dependency — p99 latency-ja smanjen sa ~180 ms na ~95 ms.

## Pokretanje lokalno

```sh
# .env mora imati:
#   MARKET_SERVICE_DB_HOST, _PORT, _NAME, _USER, _PASSWORD
#   JWT_SECRET (zajednicki za sve servise)
#   TWELVE_DATA_API_KEY (za FX rates u prod)
#   ALPHA_VANTAGE_API_KEY (za OHLC u prod)
#   SWAGGER_ENABLED=true (false u prod)
#   LIQUIBASE_CONTEXTS=dev (prod u prod)
docker compose -f setup/docker-compose.yml up market-service
```

## Pakovanje (posle PR_02 C2.9)

```text
src/main/java/com/banka1/marketservice/
├── MarketServiceApplication.java
├── stock/                          # PR_02 C2.9 (preneto iz stock_service)
│   ├── controller/
│   ├── service/
│   ├── domain/
│   ├── dto/
│   ├── repository/
│   └── ...
└── exchange/                       # PR_02 C2.9 (preneto iz exchangeService)
    ├── controller/
    ├── service/
    ├── domain/
    ├── dto/
    ├── repository/
    ├── scheduled/
    └── ...
```
