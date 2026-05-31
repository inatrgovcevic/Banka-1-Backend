# banking-core-service-go

Go port of `banking-core-service`.

This folder is intentionally isolated from the Java service. The first migration
slice implements:

- Spring-compatible health endpoints used by Docker.
- Banking-core-specific margin account and margin transaction endpoints.
- Internal account primitives needed by banking-core flows.
- Internal transfer, fund reservation, and interbank reservation endpoints.
- Account-service account and currency endpoints for employee, client, and
  internal service flows.
- Card-service creation and lifecycle endpoints under `/api/cards`, including
  automatic creation, personal/business requests, listing, limits, and status
  transitions.
- Verification-service endpoints under `/verification`, including OTP session
  generation, validation, status polling, expiration, and attempt cancellation.
- Card utility logic covered by the Java unit tests: Luhn, brand detection,
  MasterCard FX fee, and FX fee application.

Legacy Java modules that the current `banking-core-service` loads as Gradle
dependencies (`account-service`, `card-service`, `transaction-service`,
`transfer-service`, `verification-service`) are not edited here. Their endpoint
surface should be ported into this module in follow-up slices.

## Local/Docker

The service reads the same environment variables as the Java service where
possible:

- `SERVER_PORT`
- `BANKING_CORE_DB_HOST`, `BANKING_CORE_DB_PORT`, `BANKING_CORE_DB_NAME`
- `BANKING_CORE_DB_USER`, `BANKING_CORE_DB_PASSWORD`
- `JWT_SECRET`
- `SERVICES_EXCHANGE_URL`
- `SERVICES_USER_URL`
- `SERVICES_VERIFICATION_URL`
- `SKIP_VERIFICATION`
- `CARD_CREATION_AUTOMATIC_DEFAULT_LIMIT`
- `VERIFICATION_OTP_TTL_MINUTES`
- `VERIFICATION_OTP_MAX_ATTEMPTS`

Docker compose builds this folder but keeps the public service name
`banking-core-service`, so dependent containers can continue using
`http://banking-core-service:8084`.

Example local checks:

```bash
go test ./...
go build ./cmd/server
```
