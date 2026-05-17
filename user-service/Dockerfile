FROM gradle:8.14.3-jdk21-alpine AS builder

WORKDIR /app

# PR_19 C19.X: ceo backend repo kao multi-module Gradle (settings.gradle +
# root build.gradle.kts) — daje mavenCentral + dependency-management plugin +
# security-lib subproject. Pre PR_19 standalone Dockerfile build je padao
# sa "no repositories defined".
COPY . .

RUN gradle :user-service:bootJar --no-daemon -x test

FROM eclipse-temurin:21-jre-alpine

RUN apk add --no-cache curl

WORKDIR /app

COPY --from=builder /app/user-service/build/libs/*.jar app.jar

ARG SERVER_PORT=8081
EXPOSE ${SERVER_PORT}

ENTRYPOINT ["java","-jar","app.jar"]
