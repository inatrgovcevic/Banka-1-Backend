FROM golang:1.25 AS build
WORKDIR /workspace

# Bring the shared go-platform module + this service. Build context is the
# repo root so the replace directive in market-service-go/go.mod resolves.
COPY go-platform/ ./go-platform/
COPY market-service-go/ ./market-service-go/

WORKDIR /workspace/market-service-go
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/market-service-go ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates curl && addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /out/market-service-go /app/market-service-go
COPY market-service-go/migrations /app/migrations
USER app
EXPOSE 8085 9085
ENTRYPOINT ["/app/market-service-go"]
