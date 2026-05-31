# market-service-go

Standalone Go implementation of the market service API. It replaces the former Java `market-service` module and serves the stock + exchange contracts.

The default compose `market-service` entry builds this Go implementation, so it
is wired into gateway traffic by default. It owns the `market_service` schema
through SQL migrations in `market-service-go/migrations`.

## Proto generation

Generated protobuf stubs are committed under `proto/market/v1`.

Regenerate them with Docker-based `buf`:

```powershell
cd D:\Banka-1-Backend\market-service-go
.\scripts\generate-proto.ps1
```

If you prefer local tooling instead of Docker:

1. Install `buf` or `protoc` with `protoc-gen-go` and `protoc-gen-go-grpc`.
2. Regenerate from the module root using `buf generate`.

## Local run (Go only, no Docker)

```powershell
cd D:\Banka-1-Backend\market-service-go
$env:SERVER_PORT='18085'
$env:GRPC_PORT='19085'
$env:MARKET_SERVICE_DB_HOST='localhost'
$env:MARKET_SERVICE_DB_PORT='5432'
$env:MARKET_SERVICE_DB_NAME='market_service'
$env:MARKET_SERVICE_DB_USER='postgres'
$env:MARKET_SERVICE_DB_PASSWORD='postgres'
$env:JWT_SECRET='development_market_service_secret_123456'
$env:EXCHANGE_RATES_FETCH_ON_STARTUP='false'   # set to true only with a real API key
go run ./cmd/server
```

## Docker run

Start Postgres, Redis, and the default Go-backed market service:

```powershell
cd D:\Banka-1-Backend
docker compose --env-file .\setup\.env -f .\setup\docker-compose.yml up -d postgres redis market-service
```

Wait until `banka_market_service` reports `healthy`:

```powershell
docker compose --env-file .\setup\.env -f .\setup\docker-compose.yml ps market-service
```

### Optional alternate Go instance

```powershell
docker compose --env-file .\setup\.env -f .\setup\docker-compose.yml --profile go-market up -d market-service-go
```

The alternate instance binds REST on `18085` and gRPC on `19085`, and reuses the
same Postgres database (`market_service`) and Redis container.

### 2-alt. Run the Go service directly (skip Docker build)

Useful when you do not want to rebuild the Go image every iteration:

```powershell
cd D:\Banka-1-Backend\market-service-go
$env:SERVER_PORT='18085'
$env:GRPC_PORT='19085'
$env:MARKET_SERVICE_DB_HOST='localhost'
$env:MARKET_SERVICE_DB_PORT='5432'
$env:MARKET_SERVICE_DB_NAME='market_service'
$env:MARKET_SERVICE_DB_USER='postgres'
$env:MARKET_SERVICE_DB_PASSWORD='postgres'
$env:REDIS_HOST='localhost'
$env:REDIS_PORT='6379'
$env:JWT_SECRET='development_market_service_secret_123456'
$env:EXCHANGE_RATES_FETCH_ON_STARTUP='false'
$env:STOCK_LISTING_REFRESH_ENABLED='false'
go run ./cmd/server
```

### 3. Mint a JWT for protected endpoints

Most protected endpoints require a Bearer token. The Go service uses the same `JWT_SECRET`, issuer (`banka1`), id-claim, roles-claim and permissions-claim as the rest of the stack. The fastest way to produce a token is to log in through
`user-service`:

```powershell
$payload = @{ email = 'admin@banka.rs'; password = 'admin' } | ConvertTo-Json
$token = (Invoke-RestMethod -Uri "http://localhost:8081/api/auth/login" -Method POST `
    -ContentType 'application/json' -Body $payload).token
```

Or generate one ad-hoc with the helper below (any HS256 library works; this
example uses jwt.io semantics — replace `<JWT_SECRET>`):

```text
header:  {"alg":"HS256","typ":"JWT"}
payload: {
  "iss":"banka1",
  "sub":"parity",
  "id":0,
  "roles":"ADMIN",
  "permissions":["READ"],
  "exp": <now + 3600>
}
```

`roles` is a single string (not a list) on this stack. Use `ADMIN` to satisfy
all parity endpoints in the safe list except `forex`, which also accepts
`BASIC`.

### 4. Optional response comparison

```powershell
cd D:\Banka-1-Backend\market-service-go
go run ./cmd/paritycheck `
    -baseline-base http://localhost:8085 `
    -go-base http://localhost:18085 `
    -endpoints-file .\parity.endpoints.example.json `
    -token $token
```

Exit code 0 means every endpoint matched after JSON normalisation
(`timestamp`, `lastRefresh`, `createdAt` are ignored, keys sorted,
trailing-zero scale ignored).

### 5. Optional destructive comparison endpoints

`parity.endpoints.destructive.example.json` exercises mutating endpoints
(`/admin/stocks/*/refresh-market-data`, `POST /rates/fetch`,
`PUT /api/stock-exchanges/{id}/toggle-active`, …). They hit AlphaVantage,
TwelveData, and modify DB state. Run only against a disposable database, and
prefer running them sequentially:

```powershell
go run ./cmd/paritycheck `
    -baseline-base http://localhost:8085 `
    -go-base http://localhost:18085 `
    -endpoints-file .\parity.endpoints.destructive.example.json `
    -token $token
```

### 6. Tear down

```powershell
docker compose --env-file .\setup\.env -f .\setup\docker-compose.yml --profile go-market down market-service-go
```


## Build / test / image

```powershell
cd D:\Banka-1-Backend\market-service-go
go mod tidy
go test ./...
go build ./...
docker build -t market-service-go-local .
```
