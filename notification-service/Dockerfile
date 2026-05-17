# ===== Build Stage =====
FROM gradle:8.14.3-jdk21-alpine AS builder

# Set working directory
WORKDIR /app

# PR_19 C19.X: multi-module Gradle build.
COPY . .

RUN gradle :notification-service:bootJar --no-daemon -x test

# ===== Runtime Stage =====
FROM eclipse-temurin:21-jre-alpine

RUN apk add --no-cache curl

WORKDIR /app

# Copy the built JAR from the builder stage
COPY --from=builder /app/notification-service/build/libs/*.jar app.jar

# Expose the service port
ARG SERVER_PORT=8006
EXPOSE ${SERVER_PORT}

# Run the application
ENTRYPOINT ["java","-jar","app.jar"]