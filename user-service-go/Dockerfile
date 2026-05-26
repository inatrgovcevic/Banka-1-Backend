FROM golang:1.23-alpine AS build

WORKDIR /src
COPY user-service-go/go.mod ./
RUN go mod download

COPY user-service-go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/user-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache curl && addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /out/user-service /app/user-service
COPY user-service-go/migrations /app/migrations
USER app

EXPOSE 8081
ENTRYPOINT ["/app/user-service"]
