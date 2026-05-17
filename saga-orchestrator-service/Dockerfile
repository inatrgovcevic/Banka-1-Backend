# ===== Build Stage =====
FROM gradle:8.14.3-jdk21-alpine AS builder

WORKDIR /app

# PR_19 C19.X: multi-module Gradle build.
COPY . .

RUN gradle :saga-orchestrator-service:bootJar --no-daemon -x test

# ===== Runtime Stage =====
FROM eclipse-temurin:21-jre-alpine

RUN apk add --no-cache curl

WORKDIR /app

COPY --from=builder /app/saga-orchestrator-service/build/libs/*.jar app.jar

ARG SERVER_PORT=8095
EXPOSE ${SERVER_PORT}

ENTRYPOINT ["java","-jar","app.jar"]
