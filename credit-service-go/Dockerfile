FROM golang:1.26.3-alpine AS builder

WORKDIR /app

# Build context in CI is the repo root, so copy this module explicitly.
COPY credit-service-go/go.mod credit-service-go/go.sum ./
RUN go mod download

COPY credit-service-go/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/credit-service ./cmd/credit-service

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /out/credit-service /app/credit-service
COPY credit-service-go/migrations /app/migrations

EXPOSE 8085

CMD ["./credit-service"]
