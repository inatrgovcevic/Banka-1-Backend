// trading-service — konsolidovani modul (PR_02 C2.13).
//
// Trenutno: rename starog `order-service`. Sub-paket `order/` sadrzi sve sto je
// bilo u order-service-u.
//
// Buduce ekspanzije (kasniji PR-ovi):
//   - PR_03 ce uvesti `margin/` sub-paket za marzne racune
//   - PR_04 ce uvesti `otc/` sub-paket za OTC trading
//   - PR_04 ce uvesti `funds/` sub-paket za investicione fondove
//
// Public API ugovor:
//   /orders/...    (postojeci, isti kao stari order-service)
//   /margin/...    (TBD u PR_03)
//   /otc/...       (TBD u PR_04)
//   /funds/...     (TBD u PR_04)
//
// Java toolchain (21), JaCoCo, Checkstyle dolaze iz root build.gradle.kts.

plugins {
    java
    id("org.springframework.boot") version "4.0.3"
    id("io.spring.dependency-management") version "1.1.7"
    id("org.springdoc.openapi-gradle-plugin") version "1.9.0"
}

description = "Trading service — konsolidovani modul za order/margin/otc/funds (PR_02 C2.13)."

configurations {
    compileOnly {
        extendsFrom(configurations.annotationProcessor.get())
    }
}

dependencies {
    // PR_19 C19.X: project(...) umesto Maven coord-a (multi-module subproject deps).
    implementation(project(":security-lib"))
    implementation(project(":company-observability-starter"))

    // PR_19 C19.X: legacy order-service kao library dep.
    implementation(project(":order-service"))

    implementation("org.springframework.boot:spring-boot-starter-actuator")
    implementation("org.springframework.boot:spring-boot-starter-amqp")
    implementation("org.springframework.boot:spring-boot-starter-data-jpa")
    implementation("org.springframework.boot:spring-boot-starter-liquibase")
    implementation("org.springframework.boot:spring-boot-starter-security")
    implementation("org.springframework.boot:spring-boot-starter-oauth2-resource-server")
    implementation("org.springframework.boot:spring-boot-starter-web")
    implementation("org.springframework.boot:spring-boot-starter-webflux")  // za OTC saga + market-service WebClient
    implementation("org.springdoc:springdoc-openapi-starter-webmvc-ui:3.0.2")

    implementation("me.paulschwarz:springboot3-dotenv:5.0.1")

    implementation("com.fasterxml.jackson.core:jackson-core:2.21.1")
    implementation("com.fasterxml.jackson.core:jackson-databind:2.21.1")
    implementation("com.fasterxml.jackson.core:jackson-annotations:2.21")

    // PR_15 C15.4: service-to-service JWT za pozive ka account-service iz
    // RabbitMQ listener-a (FundSubscribeListener kreira fund-account u
    // request scope-u koji nema korisnikov JWT u SecurityContext-u).
    implementation("com.nimbusds:nimbus-jose-jwt:9.40")

    compileOnly("org.projectlombok:lombok")
    annotationProcessor("org.projectlombok:lombok")
    runtimeOnly("org.postgresql:postgresql")

    // PR_16 C16.1: phantom test starter-i uklonjeni (ne postoje u Maven Central).
    testImplementation("org.springframework.boot:spring-boot-starter-test")
    testImplementation("org.springframework.security:spring-security-test")
    testRuntimeOnly("com.h2database:h2")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

openApi {
    apiDocsUrl.set("http://localhost:8088/v3/api-docs.yaml")
    outputDir.set(file("docs"))
    outputFileName.set("openapi.yml")
    waitTimeInSeconds.set(30)
}
