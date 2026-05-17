// PR_32 Phase 1: interbank-service Gradle build.
//
// Standalone Spring Boot servis koji implementira inter-bank protokol
// (profesorov "A protocol for bank-to-bank asset exchange" + Tim 2 normativni doc).
// Port 8091, baza interbank_service. Sinhronizovan 2PC koordinator preko RestClient
// + SERVICE JWT ka banking-core / trading / user-service, X-Api-Key inbound za
// partner banke.
//
// Pratimo konvencije postojecih standalone modula (credit-service,
// saga-orchestrator-service):
//   * Spring Boot 4.0.3 (NE 3.5.3 koji se pominje u planu — repo je vec na 4.0.3
//     za sve servise).
//   * security-lib + company-observability-starter kao project(":..") reference
//     (multi-module Gradle), ne Maven koordinate.
//   * jacoco / checkstyle / Java 21 toolchain auto-aplicirani iz root
//     build.gradle.kts subprojects { } bloka.

plugins {
    java
    jacoco
    id("org.springframework.boot") version "4.0.3"
    id("io.spring.dependency-management") version "1.1.7"
    id("org.springdoc.openapi-gradle-plugin") version "1.9.0"
    checkstyle
}

group = "com.banka1"
version = "0.0.1-SNAPSHOT"

java {
    toolchain {
        languageVersion = JavaLanguageVersion.of(21)
    }
}

configurations {
    compileOnly {
        extendsFrom(configurations.annotationProcessor.get())
    }
}

repositories {
    mavenLocal()
    mavenCentral()
}

dependencies {
    // Shared multi-module library-i (SecurityConfig, JmbgEncryptor,
    // ServiceJwtAuthInterceptor, GlobalNoResourceHandler) i OTEL/Micrometer auto-config.
    implementation(project(":security-lib"))
    implementation(project(":company-observability-starter"))

    // Spring Boot starter-i — istovetan set kao credit-service + amqp/validation.
    implementation("org.springframework.boot:spring-boot-starter-web")
    implementation("org.springframework.boot:spring-boot-starter-data-jpa")
    implementation("org.springframework.boot:spring-boot-starter-security")
    implementation("org.springframework.boot:spring-boot-starter-oauth2-resource-server")
    implementation("org.springframework.boot:spring-boot-starter-validation")
    implementation("org.springframework.boot:spring-boot-starter-actuator")
    implementation("org.springframework.boot:spring-boot-starter-amqp")
    implementation("org.springframework.boot:spring-boot-starter-liquibase")

    // Jackson — eksplicitne verzije za BigDecimal precision (Tim 2 §12.1).
    implementation("com.fasterxml.jackson.core:jackson-core:2.21.1")
    implementation("com.fasterxml.jackson.core:jackson-databind:2.21.1")
    implementation("com.fasterxml.jackson.core:jackson-annotations:2.21")
    implementation("com.fasterxml.jackson.datatype:jackson-datatype-jsr310:2.21.1")

    // JWT (HMAC + RSA signature support).
    implementation("com.nimbusds:nimbus-jose-jwt:9.40")

    // Bouncy Castle — pojavljuje se u svim Spring servisima (security-lib trazi).
    implementation("org.bouncycastle:bcprov-jdk18on:1.78.1")

    // dotenv za lokalni dev (kao u credit-service-u).
    implementation("me.paulschwarz:springboot3-dotenv:5.0.1")

    // Swagger UI.
    implementation("org.springdoc:springdoc-openapi-starter-webmvc-ui:3.0.2")

    // Lombok.
    compileOnly("org.projectlombok:lombok")
    annotationProcessor("org.projectlombok:lombok")

    // Runtime — PostgreSQL JDBC driver.
    runtimeOnly("org.postgresql:postgresql")

    // Test dependencies.
    testImplementation("org.springframework.boot:spring-boot-starter-test")
    testImplementation("org.springframework.security:spring-security-test")
    // Spring Boot 4 split test autoconfigure module-e u zasebne starter-e — moramo
    // eksplicitno povuci JPA i JDBC test slice-ove (DataJpaTest, AutoConfigureTestDatabase).
    testImplementation("org.springframework.boot:spring-boot-data-jpa-test:4.0.3")
    testImplementation("org.springframework.boot:spring-boot-jdbc-test:4.0.3")
    testRuntimeOnly("com.h2database:h2")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

tasks.withType<Test> {
    useJUnitPlatform()
    finalizedBy(tasks.jacocoTestReport)
}

tasks.jacocoTestReport {
    dependsOn(tasks.test)
    reports {
        xml.required = true
        html.required = true
    }
}

jacoco {
    toolVersion = "0.8.12"
}

openApi {
    apiDocsUrl.set("http://localhost:8091/v3/api-docs.yaml")
    outputDir.set(file("docs"))
    outputFileName.set("openapi.yml")
    waitTimeInSeconds.set(30)
}

// Checkstyle config + ignoreFailures je vec primenjen kroz root build.gradle.kts
// subprojects { } blok — namerno NEMA duplikata ovde.
