# ===== Build Stage =====
FROM gradle:8.14.3-jdk21-alpine AS builder

WORKDIR /app

# PR_32 Phase 1: multi-module Gradle build, kao saga-orchestrator-service.
COPY . .

RUN gradle :interbank-service:bootJar --no-daemon -x test

# ===== Runtime Stage =====
FROM eclipse-temurin:21-jre-alpine

RUN apk add --no-cache curl

WORKDIR /app

COPY --from=builder /app/interbank-service/build/libs/*.jar app.jar

ARG SERVER_PORT=8091
EXPOSE ${SERVER_PORT}

ENTRYPOINT ["sh","-c","java $JAVA_OPTS -jar /app/app.jar"]
