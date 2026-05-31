FROM golang:1.25-alpine AS build

WORKDIR /workspace

# Build context is the repo root (configured in setup/docker-compose.yml).
# We need both the shared go-platform module and this service so the replace
# directive in user-service-go/go.mod resolves.
COPY go-platform/ ./go-platform/
COPY user-service-go/ ./user-service-go/

WORKDIR /workspace/user-service-go
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/user-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache curl && addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /out/user-service /app/user-service
COPY user-service-go/migrations /app/migrations
USER app

EXPOSE 8081
ENTRYPOINT ["/app/user-service"]
