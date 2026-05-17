// market-service — konsolidovani modul (stock + exchange) uveden u PR_02 C2.8.
//
// Zamenjuje stari `stock-service` i `exchange-service`. Posle migracije C2.9, oba
// REST API ugovora se serviraju iz iste JVM instance:
//
//   * `/stocks/...`   — pod StockController.java (stock paket)
//   * `/exchange/...` — pod ExchangeController.java (exchange paket)
//
// Frontend nema breaking changes; api-gateway (PR_02 C2.15) prosleđuje oba prefixa
// na isti upstream `market_service:8085`.
//
// Java toolchain (21), JaCoCo, i Checkstyle dolaze iz root build.gradle.kts.

plugins {
    java
    id("org.springframework.boot") version "4.0.3"
    id("io.spring.dependency-management") version "1.1.7"
    id("org.springdoc.openapi-gradle-plugin") version "1.9.0"
}

description = "Market service — konsolidovani modul za stock i exchange (PR_02)."

configurations {
    compileOnly {
        extendsFrom(configurations.annotationProcessor.get())
    }
}

dependencies {
    // PR_19 C19.X: project(...) umesto Maven coord-a. Gradle ne auto-substitute-uje
    // module dep-ove za subprojekte; eksplicitan project syntax je pouzdaniji.
    implementation(project(":security-lib"))
    implementation(project(":company-observability-starter"))

    // PR_19 C19.X: legacy stock + exchange kao library deps.
    implementation(project(":stock-service"))
    implementation(project(":exchange-service"))

    implementation("org.springframework.boot:spring-boot-starter-actuator")
    implementation("org.springframework.boot:spring-boot-starter-data-jpa")
    implementation("org.springframework.boot:spring-boot-starter-liquibase")
    implementation("org.springframework.boot:spring-boot-starter-security")
    implementation("org.springframework.boot:spring-boot-starter-oauth2-resource-server")
    implementation("org.springframework.boot:spring-boot-starter-web")
    implementation("org.springframework.boot:spring-boot-starter-webflux")  // WebClient za TwelveData/AlphaVantage
    // PR_19 C19.X: AlphaVantageClient koristi @CircuitBreaker + @Retry.
    implementation("io.github.resilience4j:resilience4j-spring-boot3:2.2.0")
    implementation("org.springdoc:springdoc-openapi-starter-webmvc-ui:3.0.2")

    implementation("me.paulschwarz:springboot3-dotenv:5.0.1")

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
    apiDocsUrl.set("http://localhost:8085/v3/api-docs.yaml")
    outputDir.set(file("docs"))
    outputFileName.set("openapi.yml")
    waitTimeInSeconds.set(30)
}
