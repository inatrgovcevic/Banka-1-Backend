FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY banking-core-service-go/go.mod banking-core-service-go/go.sum ./
RUN go mod download

COPY banking-core-service-go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/banking-core-service-go ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl

WORKDIR /app
COPY --from=builder /out/banking-core-service-go /app/banking-core-service-go

ARG SERVER_PORT=8084
EXPOSE ${SERVER_PORT}

ENTRYPOINT ["/app/banking-core-service-go"]
