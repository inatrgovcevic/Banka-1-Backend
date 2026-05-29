FROM golang:1.25 AS build
WORKDIR /workspace

# Bring the shared go-platform module + this service. Build context is the repo
# root so the `replace banka1/go-platform => ../go-platform` directive in
# trading-service-go/go.mod resolves.
COPY go-platform/ ./go-platform/
COPY trading-service-go/ ./trading-service-go/

WORKDIR /workspace/trading-service-go
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/trading-service-go ./cmd/server

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/trading-service-go /app/trading-service-go
# Schema migrations run at boot (RunMigrations reads ./migrations relative to WORKDIR).
# As of the P8 cut-over this service owns the `trading` schema; on a DB Java Liquibase
# already provisioned they baseline-skip. Build context is the repo root.
COPY trading-service-go/migrations /app/migrations
EXPOSE 8088 9088 18088 19088
# distroless has no shell or curl, so probe liveness by exec'ing the binary
# itself (-healthcheck) instead of a curl command.
HEALTHCHECK --interval=15s --timeout=5s --retries=10 --start-period=20s \
  CMD ["/app/trading-service-go", "-healthcheck"]
ENTRYPOINT ["/app/trading-service-go"]
