import org.gradle.api.tasks.testing.Test
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
description = "Demo project for Spring Boot"

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
    implementation(project(":company-observability-starter"))
    implementation(project(":security-lib"))

//	implementation("tools.jackson.core:jackson-core:2.21.1")
//	implementation("tools.jackson.core:jackson-databind:2.21.1")
//	implementation("tools.jackson.core:jackson-annotations:2.21.1")
    implementation("org.springframework.boot:spring-boot-starter-actuator")
    implementation("me.paulschwarz:springboot3-dotenv:5.0.1")
    implementation("org.bouncycastle:bcprov-jdk18on:1.78.1")
    implementation("org.springframework.boot:spring-boot-starter-amqp")
    implementation("org.springframework.boot:spring-boot-starter-data-jpa")
    implementation("org.springframework.boot:spring-boot-starter-liquibase")
    implementation("org.springframework.boot:spring-boot-starter-security")
    implementation("org.springframework.boot:spring-boot-starter-web")
    implementation("com.fasterxml.jackson.core:jackson-core:2.21.1")
    implementation("com.fasterxml.jackson.core:jackson-databind:2.21.1")
    implementation("com.fasterxml.jackson.core:jackson-annotations:2.21")
    implementation("org.springdoc:springdoc-openapi-starter-webmvc-ui:3.0.2")
    implementation("org.springframework.boot:spring-boot-starter-oauth2-resource-server")
    implementation("org.bouncycastle:bcprov-jdk18on:1.78.1")
    compileOnly("org.projectlombok:lombok")
    runtimeOnly("org.postgresql:postgresql")
    annotationProcessor("org.projectlombok:lombok")
    // PR_16 C16.1: phantom test starter-i uklonjeni; duplikat starter-test uklonjen.
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
        xml.required.set(true)
        html.required.set(true)
    }
}


jacoco {
    toolVersion = "0.8.12"
}

openApi {
    apiDocsUrl.set("http://localhost:8085/v3/api-docs.yaml") // todo check if this port is taken....
    outputDir.set(file("docs"))
    outputFileName.set("openapi.yml")
    waitTimeInSeconds.set(30)
}

checkstyle {
    configFile = rootProject.file("checkstyle.xml")
}

tasks.withType<Test>().configureEach {
    useJUnitPlatform()
    // Ovo osigurava da se Jacoco report pokrene odmah nakon testova
    finalizedBy(tasks.jacocoTestReport)
}

// PR_19 library mode: konsolidovani service (user/banking-core/market/trading)
// koristi ovaj modul kao project() dep, pa nam treba klasican "jar" artifact, ne fat bootJar.
tasks.bootJar { enabled = false }
tasks.jar { enabled = true; archiveClassifier.set("") }
