FROM golang:1.26.3-alpine AS builder

WORKDIR /app

COPY notification-service-go/go.mod notification-service-go/go.sum ./
RUN go mod download

COPY notification-service-go/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o notification-service ./cmd/server/

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates

ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

COPY --from=builder /app/notification-service .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8006

CMD ["./notification-service"]
