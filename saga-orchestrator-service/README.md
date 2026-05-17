# saga-orchestrator-service

SAGA pattern orchestrator za distribuirane transakcije u Banka-1 stack-u. Centralizuje OTC kupoprodaju i fondovsku kupovinu/likvidaciju hartija — koraci koji ostaju u različitim servisima posle banking-konsolidacije (#210/#211).

GH issues iz koje ovaj servis raste: **#213** (infrastruktura), **#214** (SagaInstance + state machine), **#215** (RabbitMQ topologija).

## Šta orkestrira

Po **DoD #213** i konsolidaciji iz `banking-service`:

| SAGA tip | Koraci (ukratko) |
|---|---|
| `OTC_EXERCISE` | rezervacija sredstava (banking) → transfer hartija (order) → naplata (banking) |
| `FUND_LIQUIDATION_FOR_REDEMPTION` | likvidacija pozicija fonda (order) → transfer sredstava klijentu (banking) |

OTC SAGA je posle konsolidacije svedena sa 5 servisa na 2 (`banking` i `order`); SAGA i dalje postoji jer transfer hartija i naplata sredstava ostaju u različitim servisima.

## Tehnologije

- Spring Boot 4.0.x + Java 21
- PostgreSQL (`saga_db` schema, Flyway migracije)
- RabbitMQ (Topic exchange `saga.exchange`, queues `saga.cmd.{banking,order,otc,fund}` + `saga.events` + `saga.dlq`)
- OpenAPI (`/v3/api-docs`, Swagger UI na `/swagger-ui/index.html`, statični `docs/openapi.yml`)
- Spring Actuator za infra health (`/actuator/health/liveness`)

## Endpointi

- `GET /saga/health` — domain liveness (api-gateway probe; ne treba auth)
- `GET /saga/instances` — admin listing (Issue #214 implementacija)
- `GET /saga/instances/{id}` — detalj jedne instance
- `GET /actuator/health` — Spring Actuator (DB + Rabbit)
- `GET /v3/api-docs` — OpenAPI JSON
- `GET /swagger-ui/index.html` — Swagger UI

Sve admin/internal rute idu kroz api-gateway `/saga/*` lokaciju.

## Lokalno pokretanje

Iz root-a `Banka-1-Backend-main`:

```bash
docker compose -f setup/docker-compose.yml up -d --build saga-orchestrator-service
```

Servis sluša na portu **8095** (env `SAGA_SERVER_PORT`).

Standalone (bez ostatka stack-a, samo za servis dev):

```bash
cd saga-orchestrator-service
docker compose up -d
```

Pretpostavka: postgres + rabbitmq kontejneri iz root stack-a su već pokrenuti i `banka-network` postoji.

## Health check

```bash
curl http://localhost/saga/health
# {"status":"UP","service":"saga-orchestrator-service"}

curl http://localhost:8095/actuator/health/liveness
# {"status":"UP"}
```

## Konfiguracija

Sve env varijable imaju default vrednosti — vidi [src/main/resources/application.properties](src/main/resources/application.properties).

Ključne:

| Var | Default | Opis |
|---|---|---|
| `SAGA_SERVER_PORT` | `8095` | HTTP port |
| `POSTGRES_DB` | `saga_db` | DB ime |
| `SAGA_EXCHANGE` | `saga.exchange` | Topic exchange |
| `SAGA_DLQ` | `saga.dlq` | Dead-letter queue |
| `SAGA_RETRY_MAX_ATTEMPTS` | `5` | Retry pokušaja po koraku |

## Roadmap

- **#213** (ovaj PR): infrastruktura + health + OpenAPI + skelet
- **#214**: `SagaInstance` entitet, `SagaStep` interface, `SagaOrchestrator` state machine
- **#215**: `RabbitConfig` sa exchange-om, queue-ovima, DLQ
- **#220**: prva real SAGA — OTC "Iskoristi opciju" flow
- **#231**: druga SAGA — strategija likvidacije hartija pri velikim isplatama iz fonda
