# user-service

Konsolidovani modul uveden u **PR_02 C2.3** koji zamenjuje stari `employee-service` i
`client-service`. Servira oba REST ugovora iz iste JVM instance:

| Endpoint prefix | Pakovanje | Stari servis |
|---|---|---|
| `/employees/...` | `com.banka1.userservice.employee` | `employee-service` |
| `/clients/...`   | `com.banka1.userservice.client`   | `client-service` |

## Zašto konsolidacija

Prethodno su employee-service i client-service:

- delili isti `JWT_SECRET` i `security-lib`,
- imali istu observability/RabbitMQ infrastrukturu,
- pozivali jedan drugog kroz REST sa hardkodovanih URL-ova (`employee-service` ↔ `client-service`),
- svaki imao zasebnu PostgreSQL instancu (~150 MB RAM po DB-u + ~250 MB JVM-a).

Konsolidacija oslobađa ~400 MB RAM-a po deployu i eliminiše interni REST hop, čime
smanjuje p99 latency-ja za login flow sa 280 ms na ~120 ms (mereno u
PR_02 acceptance test-u).

## Pokretanje lokalno

```sh
# .env mora imati:
#   USER_SERVICE_DB_HOST, USER_SERVICE_DB_PORT, USER_SERVICE_DB_NAME
#   USER_SERVICE_DB_USER, USER_SERVICE_DB_PASSWORD
#   JWT_SECRET (zajednicki za sve servise)
#   RABBITMQ_HOST, _PORT, _USERNAME, _PASSWORD
#   NOTIFICATION_QUEUE, _EXCHANGE, _ROUTING_KEY
#   TOKEN_REFRESH_EXPIRATION_DAYS=7   (PR_01 C1.7)
#   SWAGGER_ENABLED=true              (false u prod)
#   LIQUIBASE_CONTEXTS=dev            (prod u prod deploy-u)
docker compose -f setup/docker-compose.yml up user-service
```

## Pakovanje (posle PR_02 C2.4 i C2.5)

```text
src/main/java/com/banka1/userservice/
├── UserServiceApplication.java
├── employee/                      # PR_02 C2.4 (preneto iz employeeService)
│   ├── advice/
│   ├── configuration/
│   ├── controller/
│   ├── domain/
│   ├── dto/
│   ├── exception/
│   ├── filter/
│   ├── mappers/
│   ├── rabbitMQ/
│   ├── repository/
│   ├── scheduled/
│   ├── security/
│   └── service/
├── client/                        # PR_02 C2.5 (preneto iz clientService)
│   ├── advice/
│   ├── configuration/
│   ├── controller/
│   ├── domain/
│   ├── dto/
│   ├── exception/
│   ├── filter/
│   ├── mappers/
│   ├── rabbitMQ/
│   ├── repository/
│   ├── security/
│   └── service/
└── common/                        # zajednicki: shared infra, audit log
    └── (TBD u kasnijim PR-ovima)
```
