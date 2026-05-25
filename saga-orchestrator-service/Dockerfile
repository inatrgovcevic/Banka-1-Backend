# syntax=docker/dockerfile:1.7
#
# Multi-stage Go build for saga-orchestrator-service.
# Context must be the workspace root (Banka-1-Backend-go/) so that
# the Go workspace (go.work / go.work.sum) and all local modules
# (saga-orchestrator-service/, shared/) are available.
#
# Build:
#   docker build -f saga-orchestrator-service/Dockerfile \
#                -t banka1-saga-orchestrator-service:go .

# ---------------------------------------------------------------------------
# Stage 1: builder
# ---------------------------------------------------------------------------
FROM golang:1.25-alpine AS builder

# git is needed for go mod download from VCS sources and for ldflags version.
RUN apk add --no-cache git

WORKDIR /workspace

# Copy workspace descriptor first — Docker layer caches dep download separately
# from source changes.
COPY go.work go.work.sum ./

# Copy each module's go.mod + go.sum before sources so that dep download is
# cached independently of source changes.
# All four modules listed in go.work must have their go.mod present, else
# `go work sync` fails — even if we only build saga in this image,
# go.work resolution needs every member module's manifest.
COPY proto/go.mod proto/go.sum proto/
COPY shared/go.mod shared/go.sum shared/
COPY interbank-service/go.mod interbank-service/go.sum interbank-service/
COPY saga-orchestrator-service/go.mod saga-orchestrator-service/go.sum saga-orchestrator-service/

# Pre-fetch dependencies needed for the saga build only. We skip `go work sync`
# (which would try to download deps for every workspace module, including
# interbank whose sources we never copy here) and instead download explicitly
# for the modules this image needs: saga + its replace-targets (shared, proto).
RUN cd saga-orchestrator-service && go mod download && cd .. && \
    cd shared && go mod download && cd .. && \
    cd proto && go mod download && cd ..

# Copy full sources now.
COPY proto/ proto/
COPY shared/ shared/
COPY saga-orchestrator-service/ saga-orchestrator-service/

# Compile the binary.
# CGO_ENABLED=0 produces a fully static binary (no glibc dependency).
# -s -w strips debug info and DWARF tables (~30% size reduction).
# -trimpath removes local file paths from the binary (reproducibility).
WORKDIR /workspace/saga-orchestrator-service
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(git rev-parse --short HEAD 2>/dev/null || echo dev)" \
    -trimpath \
    -o /out/saga-orchestrator-service \
    ./cmd/saga-orchestrator-service

# ---------------------------------------------------------------------------
# Stage 2: runtime
# ---------------------------------------------------------------------------
# Alpine gives us a shell + wget for the HEALTHCHECK without distroless
# complexity. Final image size is ~30 MB (alpine 8 MB + static binary ~20 MB
# + ca-certs + migrations).
FROM alpine:3.20

# ca-certificates: needed for outbound HTTPS calls to Java services.
# wget: used by HEALTHCHECK.
RUN apk add --no-cache ca-certificates wget && \
    addgroup -S app && adduser -S -G app app

WORKDIR /

# Static binary from builder.
COPY --from=builder /out/saga-orchestrator-service /saga-orchestrator-service

# SQL migrations — goose reads these at startup.
# The binary uses SAGA_SERVER_MIGRATIONS_PATH (default "/migrations").
COPY --from=builder /workspace/saga-orchestrator-service/migrations /migrations

EXPOSE 8095

USER app

HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=5 \
    CMD wget --quiet --tries=1 --timeout=3 -O- http://localhost:8095/health > /dev/null || exit 1

ENTRYPOINT ["/saga-orchestrator-service"]
