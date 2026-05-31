FROM golang:latest AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o credit-service ./cmd/credit-service

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/credit-service .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8085

CMD ["./credit-service"]