# user-service-go

Go implementation of the Banka 1 user domain plus the shared platform baseline
for future Go services.

## Scope

This service preserves the public route shape of the current Spring user stack:

- `/employees/auth/*`
- `/employees/*`
- `/clients/auth/*`
- `/clients/customers/*`
- `/internal/interbank/user/{type}/{id}`
- `/actuator/health/liveness`
- `/actuator/health/readiness`

It also contains the common platform pieces other Go services should copy or
move into a shared module later:

- environment-based config
- JSON HTTP server
- panic recovery and request logging
- CORS
- standard JSON error response
- PostgreSQL pool
- JWT HS256 generation and validation compatible with the Java claims
- Argon2id password hashing and verification compatible with Spring Security
- RabbitMQ publisher compatible with notification-service routing keys
- AES-GCM JMBG encryption compatible with `security-lib`
- role middleware
- health/readiness endpoints

## Important Compatibility Notes

JWT tokens use the same core claims as Spring:

```json
{
  "iss": "banka1",
  "sub": "user@example.com",
  "id": 1,
  "roles": "ADMIN",
  "permissions": ["EMPLOYEE_MANAGE_ALL"],
  "exp": 1234567890
}
```

The service expects the same `JWT_SECRET` as the Java services.

Client JMBG encryption uses the same AES-GCM output format as `security-lib`:
`Base64(IV || ciphertext || authTag)`. Because the Java encryption uses a random
IV, exact lookup by JMBG cannot use a normal SQL equality predicate after the
plaintext column is dropped. The Go service handles this by decrypting existing
`jmbg_encrypted` values and comparing in application code, matching the current
runtime contract while keeping writes encrypted.

RabbitMQ publishing uses the existing notification exchange and routing keys:
`employee.created`, `employee.password_reset`, `employee.account_deactivated`,
`client.created`, `client.password_reset`, and `client.account_deactivated`.

## Local Run

```bash
cd user-service-go
go mod download
go run ./cmd/server
```

Required environment:

```env
SERVER_PORT=8081
JWT_SECRET=change-me-change-me-change-me-change-me
USER_SERVICE_DB_HOST=localhost
USER_SERVICE_DB_PORT=5432
USER_SERVICE_DB_NAME=user_service
USER_SERVICE_DB_USER=postgres
USER_SERVICE_DB_PASSWORD=postgres
```

## Docker

```bash
docker build -f user-service-go/Dockerfile -t banka-user-service-go .
```

The repository root is the Docker build context.
