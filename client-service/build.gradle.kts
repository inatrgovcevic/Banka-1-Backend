plugins {
    java
    jacoco
    checkstyle
    id("org.springframework.boot") version "4.0.3"
    id("io.spring.dependency-management") version "1.1.7"
    id("org.springdoc.openapi-gradle-plugin") version "1.9.0"
}

group = "com.banka1"
version = "0.0.1-SNAPSHOT"
description = "Client management service for Banka 1"

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
    implementation(project(":security-lib"))
    implementation(project(":company-observability-starter"))
    implementation("org.springframework.boot:spring-boot-starter-actuator")
    implementation("me.paulschwarz:springboot3-dotenv:5.0.1")
    implementation("org.springframework.boot:spring-boot-starter-data-jpa")
    implementation("org.springframework.boot:spring-boot-starter-liquibase")
    implementation("org.springframework.boot:spring-boot-starter-security")
    implementation("org.bouncycastle:bcprov-jdk18on:1.78.1")
    implementation("org.springframework.boot:spring-boot-starter-web")
    implementation("com.fasterxml.jackson.core:jackson-core:2.21.1")
    implementation("com.fasterxml.jackson.core:jackson-databind:2.21.1")
    implementation("com.fasterxml.jackson.core:jackson-annotations:2.21")
    implementation("org.springdoc:springdoc-openapi-starter-webmvc-ui:3.0.2")
    implementation("org.springframework.boot:spring-boot-starter-amqp")
    implementation("org.springframework.boot:spring-boot-starter-oauth2-resource-server")
    compileOnly("org.projectlombok:lombok")
    implementation("org.mapstruct:mapstruct:1.5.5.Final")
    runtimeOnly("org.postgresql:postgresql")
    annotationProcessor("org.projectlombok:lombok")
    annotationProcessor("org.projectlombok:lombok-mapstruct-binding:0.2.0")
    annotationProcessor("org.mapstruct:mapstruct-processor:1.5.5.Final")
    // PR_16 C16.1: phantom test starter-i uklonjeni.
    testImplementation("org.springframework.boot:spring-boot-starter-test")
    testImplementation("org.springframework.security:spring-security-test")
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

checkstyle {
    configFile = rootProject.file("checkstyle.xml")
}

tasks.withType<org.gradle.api.plugins.quality.Checkstyle>().configureEach {
    ignoreFailures = false
}

openApi {
    apiDocsUrl.set("http://localhost:8083/v3/api-docs.yaml")
    outputDir.set(file("docs"))
    outputFileName.set("openapi.yml")
    waitTimeInSeconds.set(30)
}

// PR_19 library mode: konsolidovani service (user/banking-core/market/trading)
// koristi ovaj modul kao project() dep, pa nam treba klasican "jar" artifact, ne fat bootJar.
tasks.bootJar { enabled = false }
tasks.jar { enabled = true; archiveClassifier.set("") }
