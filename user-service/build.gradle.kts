// user-service — konsolidovani modul (employee + client) uveden u PR_02 C2.3.
//
// Zamenjuje stari `employee-service` i `client-service`. Posle migracije C2.4 (employee paketi)
// i C2.5 (client paketi), oba REST API ugovora se serviraju iz iste JVM instance:
//
//   * `/employees/auth/login`, `/employees/{id}` — pod EmployeeController.java
//   * `/clients/auth/login`,   `/clients/{id}`   — pod ClientController.java
//
// Frontend nema breaking changes; api-gateway (PR_02 C2.15) prosleđuje oba prefixa
// na isti upstream `user_service:8081`.
//
// Java toolchain (21), JaCoCo, i Checkstyle dolaze iz root build.gradle.kts (subprojects {}).

plugins {
    java
    id("org.springframework.boot") version "4.0.3"
    id("io.spring.dependency-management") version "1.1.7"
    id("org.springdoc.openapi-gradle-plugin") version "1.9.0"
}

description = "User service — konsolidovani modul za employees i clients (PR_02)."

configurations {
    compileOnly {
        extendsFrom(configurations.annotationProcessor.get())
    }
}

dependencies {
    // PR_19 C19.X: project(...) umesto Maven coord-a (multi-module subproject deps).
    implementation(project(":security-lib"))
    implementation(project(":company-observability-starter"))

    // PR_19 C19.X: legacy moduli kao library deps (jar-only, ne bootJar).
    // Sav legacy controller/service/entity kod live-uje u istom JVM-u kroz user-service.
    implementation(project(":employee-service"))
    implementation(project(":client-service"))

    implementation("org.springframework.boot:spring-boot-starter-actuator")
    implementation("org.springframework.boot:spring-boot-starter-amqp")
    implementation("org.springframework.boot:spring-boot-starter-data-jpa")
    implementation("org.springframework.boot:spring-boot-starter-liquibase")
    implementation("org.springframework.boot:spring-boot-starter-security")
    implementation("org.springframework.boot:spring-boot-starter-oauth2-resource-server")
    implementation("org.springframework.boot:spring-boot-starter-web")
    implementation("org.springdoc:springdoc-openapi-starter-webmvc-ui:3.0.2")

    implementation("me.paulschwarz:springboot3-dotenv:5.0.1")
    implementation("org.bouncycastle:bcprov-jdk18on:1.78.1")

    implementation("com.fasterxml.jackson.core:jackson-core:2.21.1")
    implementation("com.fasterxml.jackson.core:jackson-databind:2.21.1")
    implementation("com.fasterxml.jackson.core:jackson-annotations:2.21")

    compileOnly("org.projectlombok:lombok")
    annotationProcessor("org.projectlombok:lombok")
    runtimeOnly("org.postgresql:postgresql")

    // PR_16 C16.1: phantom test starter-i uklonjeni.
    testImplementation("org.springframework.boot:spring-boot-starter-test")
    testImplementation("org.springframework.security:spring-security-test")
    testRuntimeOnly("com.h2database:h2")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

openApi {
    apiDocsUrl.set("http://localhost:8081/v3/api-docs.yaml")
    outputDir.set(file("docs"))
    outputFileName.set("openapi.yml")
    waitTimeInSeconds.set(30)
}
