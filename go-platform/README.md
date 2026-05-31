# go-platform

Shared Go module: the Go equivalent of `security-lib` +
`company-observability-starter` from the Java side, plus the DB / RabbitMQ /
HTTP / gRPC scaffolding every Go service needs.

Module path: `banka1/go-platform`. Used in-tree via `replace` directives
from each Go service's `go.mod` (no publishing).

## Packages

| Package    | Purpose                                                         |
|------------|-----------------------------------------------------------------|
| `config`   | env helpers (`Env`, `EnvInt`, `EnvBool`, `EnvDuration`, `SplitCSV`, `Required`, `Redact`) |
| `log`      | `slog.JSONHandler` factory with `service` field + ctx logger    |
| `httpx`    | Correlation, request log, recover, CORS, response/error helpers, `Chain`, `Default` |
| `auth`     | JWT parse/mint (HS256, `banka1` issuer default), `Principal`, role hierarchy, `RequireRoles`, `RequirePermissions`, `PermitAll` matcher, `SERVICE` token |
| `health`   | `/actuator/health`, `/liveness`, `/readiness`, `/info` with `Checker` interface |
| `db`       | `pgxpool` open + ping `Checker` + `RunInTx` helper              |
| `rabbitmq` | Dial with backoff, durable topic publisher (persistent JSON + correlation id + eventId), consumer with manual ack/panic-recovery/idempotency hook, opt-in `NoopPublisher` |
| `grpcx`    | `NewServer(logger, opts...)` and unary interceptors: correlation, log, recover, auth |
| `otel`     | Optional OTLP HTTP exporter; HTTP middleware + gRPC interceptor that put `traceId`/`spanId` on the ctx logger so the existing `httpx` log middleware picks them up automatically. Disabled when `OTEL_EXPORTER_OTLP_ENDPOINT` is blank or `OTEL_SDK_DISABLED=true`. |

## Wiring example

```go
import (
    "banka1/go-platform/auth"
    "banka1/go-platform/config"
    "banka1/go-platform/db"
    "banka1/go-platform/health"
    "banka1/go-platform/httpx"
    "banka1/go-platform/log"
    "banka1/go-platform/otel"
    "banka1/go-platform/rabbitmq"
)

func main() {
    logger := log.New("market-service-go", log.Level(config.Env("LOG_LEVEL_APP", "INFO")))
    ctx := context.Background()

    // Tracing (no-op when OTEL_EXPORTER_OTLP_ENDPOINT is empty).
    shutdownOTEL, _ := otel.Setup(ctx, otel.LoadConfig("market-service-go"))
    defer shutdownOTEL(context.Background())

    pool, _ := db.OpenPool(ctx, db.Config{...}.URL())
    defer pool.Close()

    jwt := auth.NewService(auth.LoadConfig())
    publisher, _ := rabbitmq.NewPublisher(ctx, rabbitmq.LoadConfig(), logger)
    defer publisher.Close()

    healthH := health.NewHandler().Register(db.Checker("postgres", pool))

    mux := http.NewServeMux()
    healthH.MountStandard(mux)
    mux.Handle("GET /api/listings/", jwt.Middleware(auth.RequireRoles("ADMIN")(myHandler())))

    middleware := httpx.Chain(
        httpx.CorrelationMiddleware,
        otel.HTTPMiddleware("market-service-go", logger),
        httpx.RecoverMiddleware(logger),
        httpx.RequestLogMiddleware(logger),
        httpx.CORSMiddleware(httpx.DefaultCORS()),
    )
    server := &http.Server{Addr: ":18085", Handler: middleware(mux)}
    server.ListenAndServe()
}
```

## How services consume it

Both `user-service-go` and `market-service-go` add a `replace` directive in
their `go.mod`:

```go
replace banka1/go-platform => ../go-platform
```

and a normal `require banka1/go-platform v0.0.0-...`. The replace keeps the
module local; no Go proxy / private registry is needed.

## Test

```powershell
cd D:\Banka-1-Backend\go-platform
go mod tidy
go test ./...
```
